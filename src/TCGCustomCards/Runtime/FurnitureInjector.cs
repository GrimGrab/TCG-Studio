using System.Collections.Generic;
using System.IO;
using System.Linq;
using TCGCustomCards.Core;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// Adds custom furniture to the (shared) ShelfData ScriptableObject: for each piece a hidden copy of its base prefab (inactive, so the
    /// game's clones start active and run their own Awake) with our object type, look and spots, plus ObjectData and
    /// FurniturePurchaseData entries. The game then buys, spawns, places and saves it like any vanilla piece.
    /// Runs once per session in the "Start" scene, before saved objects are spawned.
    /// </summary>
    internal static class FurnitureInjector
    {
        private static ShelfData_ScriptableObject _injectedInto;
        private static GameObject _root;

        public static void EnsureInjected()
        {
            if (Registry.Furniture.Count == 0) return;
            // Never touch CSingleton<InventoryBase>.Instance outside the shop scene: it would create an empty manager.
            var inv = Object.FindObjectOfType<InventoryBase>();
            if (inv == null || inv.m_ObjectData_SO == null) return;
            var so = inv.m_ObjectData_SO;
            if (_injectedInto == so) return;
            _injectedInto = so;

            if (_root == null)
            {
                _root = new GameObject("TCGCC_FurnitureTemplates");
                _root.SetActive(false);
                Object.DontDestroyOnLoad(_root);
            }

            int ok = 0;
            foreach (var f in Registry.Furniture)
            {
                try
                {
                    if (Build(so, inv, f)) ok++;
                }
                catch (System.Exception e)
                {
                    Plugin.Log.LogError($"Furniture '{f.Def.Id}' failed to build: {e}");
                    if (f.Prefab != null) Object.Destroy(f.Prefab.gameObject);
                    f.Prefab = null;
                }
            }
            Plugin.Log.LogInfo($"Injected {ok} of {Registry.Furniture.Count} custom furniture piece(s)");
            FurnitureTemplateDump.Write(so);
        }

        private static bool Build(ShelfData_ScriptableObject so, InventoryBase inv, CustomFurniture f)
        {
            var def = f.Def;
            var baseData = so.m_ObjectDataList.Find(o => o != null && o.objectType == def.BaseObject);
            var basePrefab = baseData?.spawnPrefab;
            if (basePrefab == null)
            {
                Plugin.Log.LogError($"Furniture '{def.Id}': base {def.BaseObject} has no prefab — skipped");
                return false;
            }
            if (!FurnitureKinds.Matches(def.Type, basePrefab))
            {
                Plugin.Log.LogError($"Furniture '{def.Id}': base {def.BaseObject} is a {basePrefab.GetType().Name}, not a {def.Type} — skipped");
                return false;
            }
            var basePurchase = so.m_FurniturePurchaseDataList.Find(p => p != null && p.objectType == def.BaseObject);

            var tpl = Object.Instantiate(basePrefab, _root.transform);
            tpl.name = $"TCGCC_{def.Id}";
            tpl.transform.localPosition = Vector3.zero;
            tpl.transform.localRotation = Quaternion.identity;
            tpl.m_ObjectType = f.Object;
            f.Prefab = tpl;

            if (def.Spots != null) FurnitureSpots.Build(tpl, def, so, inv);
            if (def.Area != null) SetArea(tpl, def);
            if (def.Points != null) SetPoints(tpl, def);
            ApplyLook(tpl, def);

            so.m_ObjectDataList.Add(new ObjectData
            {
                name = def.Name,
                objectType = f.Object,
                spawnPrefab = tpl,
                decoBonus = def.DecoBonus ?? baseData.decoBonus,
            });
            Sprite baseIcon = basePurchase?.icon;
            Sprite icon = string.IsNullOrEmpty(def.Icon) ? null
                : ImageCache.GetPadded(Path.Combine(def.FolderPath, def.Icon), baseIcon != null ? baseIcon.rect.width / baseIcon.rect.height : 1f);
            // No own icon but a tint: the base icon in the same colour, so the shop shows what you get.
            if (icon == null && baseIcon != null && !string.IsNullOrEmpty(def.Tint) && ColorUtility.TryParseHtmlString(def.Tint, out var iconTint))
                icon = TintedIcon(baseIcon, iconTint, def.Id);
            so.m_FurniturePurchaseDataList.Add(new FurniturePurchaseData
            {
                name = def.Name,
                // Our own text is served by FurniturePatches; without one the base piece's (localized) description is used.
                description = basePurchase?.description ?? "",
                levelRequirement = def.Level ?? basePurchase?.levelRequirement ?? 1,
                price = def.Price ?? basePurchase?.price ?? 100f,
                objectType = f.Object,
                icon = icon != null ? icon : baseIcon,
            });
            return true;
        }

        /// <summary>
        /// The placement area (m_MoveStateValidArea, an OverlapBox of its lossy scale) sits in a group that also holds the snap points and
        /// the dashed guide lines and carries the scale. Move and scale the group so the area gets the def's centre and size (x/z).
        /// </summary>
        private static void SetArea(InteractableObject tpl, Core.FurnitureDef def)
        {
            var area = tpl.m_MoveStateValidArea;
            if (area == null) { Plugin.Log.LogWarning($"Furniture '{def.Id}': base {def.BaseObject} has no placement area; 'area' ignored"); return; }
            var grp = area.parent != null && area.parent != tpl.transform ? area.parent : area;
            var piece = tpl.transform;
            var cur = piece.InverseTransformPoint(area.position);
            grp.position += piece.TransformVector(new Vector3(def.Area.Pos[0] - cur.x, 0f, def.Area.Pos[1] - cur.z));
            var lossy = area.lossyScale;
            var s = grp.localScale;
            if (lossy.x > 1e-4f) s.x *= def.Area.Size[0] / lossy.x;
            if (lossy.z > 1e-4f) s.z *= def.Area.Size[1] / lossy.z;
            grp.localScale = s;
        }

        /// <summary>
        /// Moves the piece's position Transforms (seats, cashier/worker spots, customer stand points…) to the def's points, role by role
        /// in order. Resizable lists get extra points (copies of the first) or lose surplus ones; other roles keep the base's count.
        /// </summary>
        private static void SetPoints(InteractableObject tpl, Core.FurnitureDef def)
        {
            var piece = tpl.transform;
            foreach (var role in Core.FurnitureKinds.PointRoles(def.Type))
            {
                var pts = def.Points.Where(p => p.Role == role.Role).ToList();
                if (pts.Count == 0) continue;
                var field = tpl.GetType().GetField(role.Field, System.Reflection.BindingFlags.Instance | System.Reflection.BindingFlags.Public | System.Reflection.BindingFlags.NonPublic);
                if (field == null) { Plugin.Log.LogWarning($"Furniture '{def.Id}': {tpl.GetType().Name} has no {role.Field}; '{role.Role}' points ignored"); continue; }
                var list = field.GetValue(tpl) as List<Transform>;
                var single = field.FieldType == typeof(Transform) ? field.GetValue(tpl) as Transform : null;
                if (list == null && single == null) { Plugin.Log.LogWarning($"Furniture '{def.Id}': base has no '{role.Role}' point"); continue; }
                if (list == null) list = new List<Transform> { single };
                if (role.Resizable && list.Count > 0)
                {
                    while (list.Count < pts.Count)
                    {
                        var c = Object.Instantiate(list[0].gameObject, list[0].parent, true).transform;
                        c.name = $"{list[0].name}_{list.Count}";
                        list.Add(c);
                    }
                    while (list.Count > pts.Count && list.Count > 1)
                    {
                        var last = list[list.Count - 1];
                        list.RemoveAt(list.Count - 1);
                        Object.DestroyImmediate(last.gameObject);
                    }
                }
                else if (pts.Count != list.Count)
                    Plugin.Log.LogWarning($"Furniture '{def.Id}': '{role.Role}' has {list.Count} point(s) in the game, the def {pts.Count}; the first {Mathf.Min(list.Count, pts.Count)} are used");
                for (int i = 0; i < list.Count && i < pts.Count; i++)
                {
                    var t = list[i];
                    if (t == null) continue;
                    t.SetPositionAndRotation(piece.TransformPoint(new Vector3(pts[i].Pos[0], pts[i].Pos[1], pts[i].Pos[2])),
                        piece.rotation * Quaternion.Euler(pts[i].Rot[0], pts[i].Rot[1], pts[i].Rot[2]));
                }
            }
        }

        // ------------------------------------------------------------------ look

        /// <summary>
        /// Body renderers = every renderer of the piece that isn't part of a spot (compartments carry price tags, adapters and item slots).
        /// Texture: replaces the base's main texture wherever it is used. Tint: multiplies the colour. Mesh: our model (piece root
        /// space) on a new child renderer drawn with a copy of the base's body material, the base's body renderers hidden, the main
        /// collider fitted to the model.
        /// </summary>
        private static void ApplyLook(InteractableObject tpl, FurnitureDef def)
        {
            var spotRoots = new HashSet<Transform>(tpl.GetComponentsInChildren<ShelfCompartment>(true).Select(c => c.transform)
                .Concat(tpl.GetComponentsInChildren<InteractableCardCompartment>(true).Select(c => c.transform)));
            bool InSpot(Transform t)
            {
                for (var p = t; p != null && p != tpl.transform; p = p.parent) if (spotRoots.Contains(p)) return true;
                return false;
            }
            var body = tpl.GetComponentsInChildren<Renderer>(true).Where(r => !InSpot(r.transform) && r.GetType().Name != "ParticleSystemRenderer").ToList();

            Texture2D tex = string.IsNullOrEmpty(def.Texture) ? null : ImageCache.Get(Path.Combine(def.FolderPath, def.Texture))?.texture;
            Color? tint = null;
            if (!string.IsNullOrEmpty(def.Tint) && ColorUtility.TryParseHtmlString(def.Tint, out var c)) tint = c;
            Mesh mesh = string.IsNullOrEmpty(def.Mesh) ? null : MeshLoader.Get(Path.Combine(def.FolderPath, def.Mesh));

            var mainRenderer = tpl.m_Mesh;
            Material mainMat = mainRenderer != null ? mainRenderer.sharedMaterial : body.Select(r => r.sharedMaterial).FirstOrDefault(m => m != null);
            Texture baseTex = mainMat != null ? mainMat.mainTexture : null;

            if (mesh != null)
            {
                // The model is in the piece's root space (TCG Studio bakes it there), so it gets its own child at the root. The base's
                // body renderers are hidden; the outline, the move preview and the culling check (shelves hide their stock while it
                // isn't visible) use the new renderer.
                var go = new GameObject("TCGCC_Model");
                go.transform.SetParent(tpl.transform, false);
                go.layer = tpl.gameObject.layer;
                var mf = go.AddComponent<MeshFilter>();
                mf.sharedMesh = mesh;
                var mr = go.AddComponent<MeshRenderer>();
                var m = mainMat != null ? new Material(mainMat) : new Material(Shader.Find("Standard"));
                m.name = $"TCGCC_{def.Id}";
                m.hideFlags = HideFlags.DontUnloadUnusedAsset;
                m.mainTexture = tex != null ? tex : Texture2D.whiteTexture;
                m.mainTextureScale = Vector2.one;
                m.mainTextureOffset = Vector2.zero;
                if (m.HasProperty("_Color")) m.color = tint ?? Color.white;
                mr.sharedMaterial = m;
                foreach (var r in body)
                    if (!IsUnder(r.transform, tpl.m_HighlightGameObj)) r.enabled = false;
                tpl.m_Mesh = mr;
                tpl.m_PickupObjectMesh = mf;
                ReplaceHighlight(tpl, mesh);
                if (tpl.m_CullingCheckMesh != null && !tpl.m_CullingCheckMesh.enabled) tpl.m_CullingCheckMesh = mr;
                FitCollider(tpl.m_BoxCollider, go.transform, mesh.bounds);
                return;
            }

            if (tex == null && !tint.HasValue) return;
            var copies = new Dictionary<Material, Material>();
            Material Swap(Material m)
            {
                if (m == null) return null;
                if (copies.TryGetValue(m, out var copy)) return copy;
                bool usesBaseTex = baseTex != null && m.mainTexture == baseTex;
                if (!usesBaseTex && !tint.HasValue) return copies[m] = m;
                copy = new Material(m) { name = $"TCGCC_{def.Id}_{m.name}", hideFlags = HideFlags.DontUnloadUnusedAsset };
                if (tex != null && usesBaseTex)
                    foreach (var prop in copy.GetTexturePropertyNames())
                        if (copy.GetTexture(prop) == baseTex) copy.SetTexture(prop, tex);
                if (tint.HasValue && copy.HasProperty("_Color")) copy.color = copy.color * tint.Value;
                return copies[m] = copy;
            }
            foreach (var r in body)
            {
                var mats = r.sharedMaterials;
                for (int i = 0; i < mats.Length; i++) mats[i] = Swap(mats[i]);
                r.sharedMaterials = mats;
            }
        }

        /// <summary>Copy of a (non-readable) game sprite with its colour multiplied by <paramref name="tint"/>; alpha kept. Null on failure.</summary>
        private static Sprite TintedIcon(Sprite src, Color tint, string id)
        {
            var r = src.textureRect;
            var tex = TextureReadback.Read(src.texture, new RectInt((int)r.x, (int)r.y, (int)r.width, (int)r.height));
            if (tex == null) return null;
            var px = tex.GetPixels32();
            for (int i = 0; i < px.Length; i++)
            {
                var c = px[i];
                px[i] = new Color32((byte)(c.r * tint.r), (byte)(c.g * tint.g), (byte)(c.b * tint.b), c.a);
            }
            tex.SetPixels32(px);
            tex.Apply(false, true);
            tex.name = $"TCGCC_{id}_icon";
            var pivot = new Vector2(src.pivot.x / src.rect.width, src.pivot.y / src.rect.height);
            var s = Sprite.Create(tex, new Rect(0, 0, tex.width, tex.height), pivot, src.pixelsPerUnit);
            s.name = tex.name;
            return s;
        }

        /// <summary>
        /// Pieces without an outline on their main material (e.g. shelves) show m_HighlightGameObj while aimed at: a separate
        /// object shaped like the vanilla model. Give it our model's shape with the game's own highlight material, and switch the
        /// vanilla one off (Init no longer hides it once m_Mesh is set).
        /// </summary>
        private static void ReplaceHighlight(InteractableObject tpl, Mesh mesh)
        {
            var old = tpl.m_HighlightGameObj;
            if (old == null) return;
            var mats = old.GetComponentsInChildren<Renderer>(true).Select(r => r.sharedMaterial).FirstOrDefault(m => m != null);
            old.SetActive(false);
            if (mats == null) { tpl.m_HighlightGameObj = null; return; }
            var hl = new GameObject("TCGCC_Highlight");
            hl.transform.SetParent(tpl.transform, false);
            hl.layer = old.layer;
            hl.AddComponent<MeshFilter>().sharedMesh = mesh;
            var r2 = hl.AddComponent<MeshRenderer>();
            r2.sharedMaterial = mats;
            r2.shadowCastingMode = UnityEngine.Rendering.ShadowCastingMode.Off;
            r2.receiveShadows = false;
            hl.SetActive(false);
            tpl.m_HighlightGameObj = hl;
        }

        private static bool IsUnder(Transform t, GameObject root) => root != null && (t == root.transform || t.IsChildOf(root.transform));

        /// <summary>Sets a box collider to the bounds of <paramref name="localBounds"/> (in <paramref name="space"/>), converted into the collider's space.</summary>
        internal static void FitCollider(BoxCollider col, Transform space, Bounds localBounds)
        {
            if (col == null) return;
            var min = new Vector3(float.MaxValue, float.MaxValue, float.MaxValue);
            var max = new Vector3(float.MinValue, float.MinValue, float.MinValue);
            var e = localBounds.extents;
            for (int i = 0; i < 8; i++)
            {
                var corner = localBounds.center + new Vector3((i & 1) == 0 ? -e.x : e.x, (i & 2) == 0 ? -e.y : e.y, (i & 4) == 0 ? -e.z : e.z);
                var p = col.transform.InverseTransformPoint(space.TransformPoint(corner));
                min = Vector3.Min(min, p);
                max = Vector3.Max(max, p);
            }
            col.center = (min + max) * 0.5f;
            col.size = max - min;
        }
    }
}

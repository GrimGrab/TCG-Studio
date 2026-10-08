using System.Collections.Generic;
using System.IO;
using System.Linq;
using TCGCustomCards.Core;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// Adds custom decorations to the (shared) ShelfData ScriptableObject, once per session in the "Start" scene:
    ///  • surfaces: a ShopDecoData appended to the wall / floor / ceiling list (the game applies it to the shop's shared materials);
    ///  • posters and objects: a hidden copy of a vanilla decoration prefab (wall: Poster 1, floor: plant 1) with our model, plus
    ///    DecoData / DecoPurchaseData served for our EDecoObject int (DecorationPatches) and an entry on the shop's Poster / Other tab.
    /// The game then sells, places, moves and saves them like vanilla decorations.
    /// </summary>
    internal static class DecorationInjector
    {
        public const EDecoObject WallBase = (EDecoObject)1;    // Poster1: thin board on the wall, faces +z, wall plane z = 0
        public const EDecoObject FloorBase = (EDecoObject)121; // VPlant1: stands on y = 0, cuts the customer nav mesh

        private static ShelfData_ScriptableObject _injectedInto;
        private static GameObject _root;
        private static Texture2D _flatMetallic;
        /// <summary>Our ShopDecoData / DecoPurchaseData / DecoData objects → decoration (their names are plain text, not I2 terms).</summary>
        private static readonly Dictionary<object, CustomDecoration> ByData = new Dictionary<object, CustomDecoration>();

        public static CustomDecoration Of(object data) => data != null && ByData.TryGetValue(data, out var d) ? d : null;

        public static void EnsureInjected()
        {
            if (Registry.Decorations.Count == 0) return;
            // Never touch CSingleton<InventoryBase>.Instance outside the shop scene: it would create an empty manager.
            var inv = Object.FindObjectOfType<InventoryBase>();
            if (inv == null || inv.m_ObjectData_SO == null) return;
            var so = inv.m_ObjectData_SO;
            if (_injectedInto == so) return;
            _injectedInto = so;

            if (_root == null)
            {
                _root = new GameObject("TCGCC_DecorationTemplates");
                _root.SetActive(false);
                Object.DontDestroyOnLoad(_root);
            }

            int ok = 0;
            foreach (var d in Registry.Decorations)
            {
                try
                {
                    if (d.Def.IsSurface ? BuildSurface(so, d) : BuildPlaceable(so, d)) ok++;
                }
                catch (System.Exception e)
                {
                    Plugin.Log.LogError($"Decoration '{d.Def.Id}' failed to build: {e}");
                    if (d.Prefab != null) Object.Destroy(d.Prefab.gameObject);
                    d.Prefab = null;
                }
            }
            EnsurePlayerLists();
            Plugin.Log.LogInfo($"Injected {ok} of {Registry.Decorations.Count} custom decoration(s)");
        }

        /// <summary>The list a surface kind lives in.</summary>
        public static List<ShopDecoData> SurfaceList(ShelfData_ScriptableObject so, DecorationKind kind) =>
            kind == DecorationKind.Wall ? so.m_WallDecoDataList : kind == DecorationKind.Floor ? so.m_FloorDecoDataList : so.m_CeilingDecoDataList;

        private static bool BuildSurface(ShelfData_ScriptableObject so, CustomDecoration d)
        {
            var def = d.Def;
            var list = SurfaceList(so, def.Kind);
            int index = list.Count;
            // CPlayerData's unlock lists are 1001 long on a new game; EnsurePlayerLists pads them, keep a sane cap anyway.
            if (index >= 1000) { Plugin.Log.LogError($"Decoration '{def.Id}': too many {def.Kind} looks"); return false; }
            var tex = ImageCache.GetTiling(Path.Combine(def.FolderPath, def.Texture), linear: false);
            if (tex == null) { Plugin.Log.LogError($"Decoration '{def.Id}': texture '{def.Texture}' could not be loaded — skipped"); return false; }
            var normal = string.IsNullOrEmpty(def.NormalMap) ? null : ImageCache.GetTiling(Path.Combine(def.FolderPath, def.NormalMap), linear: true);
            var metal = string.IsNullOrEmpty(def.RoughnessMap) ? null : ImageCache.GetTiling(Path.Combine(def.FolderPath, def.RoughnessMap), linear: true);
            var color = Color.white;
            if (!string.IsNullOrEmpty(def.Color) && ColorUtility.TryParseHtmlString(def.Color, out var c)) color = c;

            var baseIcon = list.Count > 0 ? list[0].icon : null;
            Sprite icon = string.IsNullOrEmpty(def.Icon) ? null
                : ImageCache.GetPadded(Path.Combine(def.FolderPath, def.Icon), baseIcon != null ? baseIcon.rect.width / baseIcon.rect.height : 1f);
            if (icon == null) icon = Sprite.Create(tex, new Rect(0, 0, tex.width, tex.height), new Vector2(0.5f, 0.5f));

            var data = new ShopDecoData
            {
                name = def.Name, mainNameText = "", replaceNameXXXText = "", replaceNameYYYText = "",
                price = def.Price,
                showBar = false,
                mainTexture = tex,
                // The shop materials use a metallic/smoothness map; without one the shader's default (white) would make it chrome.
                roughnessMap = metal != null ? metal : FlatMetallic(),
                normalMap = normal,
                color = color,
                smoothness = def.Smoothness,
                icon = icon,
            };
            list.Add(data);
            if (def.Kind == DecorationKind.Wall)
            {
                // GetWallBarDecoData shares the wall index (only read for walls with a bar; ours have none). Keep the lists aligned.
                var bars = so.m_WallBarDecoDataList;
                while (bars.Count > 0 && bars.Count <= index) bars.Add(bars[bars.Count - 1]);
            }
            d.Index = index;
            d.Surface = data;
            ByData[data] = d;
            return true;
        }

        /// <summary>Metallic 0, smoothness 1 (scaled by the material's _GlossMapScale = the decoration's smoothness).</summary>
        private static Texture2D FlatMetallic()
        {
            if (_flatMetallic != null) return _flatMetallic;
            _flatMetallic = new Texture2D(4, 4, TextureFormat.RGBA32, false, true) { name = "TCGCC_FlatMetallic", hideFlags = HideFlags.DontUnloadUnusedAsset };
            var px = new Color32[16];
            for (int i = 0; i < px.Length; i++) px[i] = new Color32(0, 0, 0, 255);
            _flatMetallic.SetPixels32(px);
            _flatMetallic.Apply(false, true);
            return _flatMetallic;
        }

        private static bool BuildPlaceable(ShelfData_ScriptableObject so, CustomDecoration d)
        {
            var def = d.Def;
            bool wall = def.EffectiveMount == DecorationMount.Wall;
            int baseIdx = (int)(wall ? WallBase : FloorBase);
            if (baseIdx >= so.m_DecoDataList.Count || baseIdx >= so.m_DecoPurchaseDataList.Count || so.m_DecoDataList[baseIdx]?.spawnPrefab == null)
            {
                Plugin.Log.LogError($"Decoration '{def.Id}': base decoration {baseIdx} not found — skipped");
                return false;
            }
            var mesh = MeshLoader.Get(Path.Combine(def.FolderPath, def.Mesh));
            if (mesh == null) { Plugin.Log.LogError($"Decoration '{def.Id}': model '{def.Mesh}' could not be loaded — skipped"); return false; }
            var basePurchase = so.m_DecoPurchaseDataList[baseIdx];

            var tpl = Object.Instantiate(so.m_DecoDataList[baseIdx].spawnPrefab, _root.transform);
            tpl.name = $"TCGCC_{def.Id}";
            tpl.transform.localPosition = Vector3.zero;
            tpl.transform.localRotation = Quaternion.identity;
            tpl.transform.localScale = Vector3.one;
            tpl.m_DecoObjectType = d.Object;
            d.Prefab = tpl;

            Texture tex = string.IsNullOrEmpty(def.Texture) ? null : ImageCache.Get(Path.Combine(def.FolderPath, def.Texture))?.texture;
            var body = tpl.GetComponentsInChildren<Renderer>(true).Where(r => r.GetType().Name != "ParticleSystemRenderer");
            ModelSwap.Apply(tpl, body, mesh, tex, null, def.Id);
            SetArea(tpl, mesh.bounds, wall);

            var data = new DecoData
            {
                name = def.Name,
                decoType = def.EffectiveTab == DecorationTab.Poster ? EDecoType.Poster : wall ? EDecoType.Sign : EDecoType.Statue,
                spawnPrefab = tpl,
            };
            Sprite baseIcon = basePurchase?.icon;
            Sprite icon = string.IsNullOrEmpty(def.Icon) ? null
                : ImageCache.GetPadded(Path.Combine(def.FolderPath, def.Icon), baseIcon != null ? baseIcon.rect.width / baseIcon.rect.height : 1f);
            var purchase = new DecoPurchaseData
            {
                name = def.Name, mainNameText = "", replaceNameXXXText = "", replaceNameYYYText = "",
                levelRequirement = 0,
                price = def.Price,
                icon = icon != null ? icon : baseIcon,
            };
            d.Data = data;
            d.Purchase = purchase;
            ByData[data] = d;
            ByData[purchase] = d;
            (def.EffectiveTab == DecorationTab.Poster ? so.m_PosterDecoList : so.m_OtherDecoList).Add(d.Object);
            return true;
        }

        /// <summary>
        /// The placement area (m_MoveStateValidArea: an OverlapBox of its lossy scale, also the snap guide) sits in a group that carries
        /// the size. Wall decorations: the group is turned 90° about x (its x = width, its z = height on the wall). Floor: x/z footprint.
        /// </summary>
        private static void SetArea(InteractableObject tpl, Bounds b, bool wall)
        {
            var area = tpl.m_MoveStateValidArea;
            if (area == null) return;
            var grp = area.parent != null && area.parent != tpl.transform ? area.parent : area;
            var s = grp.localScale;
            var p = grp.localPosition;
            if (wall)
            {
                s.x = Mathf.Max(0.05f, b.size.x);
                s.z = Mathf.Max(0.05f, b.size.y);
                p.x = b.center.x;
                p.y = b.center.y;
            }
            else
            {
                s.x = Mathf.Max(0.05f, b.size.x);
                s.z = Mathf.Max(0.05f, b.size.z);
                p.x = b.center.x;
                p.z = b.center.z;
            }
            grp.localScale = s;
            grp.localPosition = p;
        }

        /// <summary>CPlayerData's owned-count list (by EDecoObject) and unlock lists (by surface index) must cover our ints.</summary>
        public static void EnsurePlayerLists()
        {
            if (_injectedInto == null) return;
            int maxObj = -1;
            var maxSurf = new Dictionary<DecorationKind, int>();
            foreach (var d in Registry.Decorations)
            {
                if (d.Prefab != null) maxObj = Mathf.Max(maxObj, (int)d.Object);
                if (d.Def.IsSurface && d.Index >= 0) maxSurf[d.Def.Kind] = Mathf.Max(maxSurf.TryGetValue(d.Def.Kind, out var m) ? m : -1, d.Index);
            }
            if (CPlayerData.m_DecorationInventoryList != null)
                while (CPlayerData.m_DecorationInventoryList.Count <= maxObj) CPlayerData.m_DecorationInventoryList.Add(0);
            foreach (var kv in maxSurf)
            {
                var list = kv.Key == DecorationKind.Wall ? CPlayerData.m_UnlockedDecoWallList
                         : kv.Key == DecorationKind.Floor ? CPlayerData.m_UnlockedDecoFloorList : CPlayerData.m_UnlockedDecoCeilingList;
                if (list == null) continue;
                while (list.Count <= kv.Value) list.Add(false);
            }
        }
    }
}

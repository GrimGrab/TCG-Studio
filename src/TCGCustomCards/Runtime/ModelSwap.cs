using System.Collections.Generic;
using System.Linq;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// Puts an own model (baked OBJ in the object's root space) on a hidden template copy of a vanilla InteractableObject: a new child
    /// renderer drawn with a copy of the base's main material, the base's own renderers hidden, the outline / move preview / culling
    /// check pointed at the new renderer and the main collider fitted to the model. Used by custom furniture and decorations.
    /// </summary>
    internal static class ModelSwap
    {
        private static readonly System.Action<InteractableObject> EvaluateMeshOverride = Core.Compat.Method<InteractableObject>(
            "Furniture models", "EvaluateMeshOverride");

        /// <summary>
        /// A fresh template copy of a vanilla piece (game 1.05+): the game swaps meshes and materials listed in
        /// m_OverrideMeshDataList in every object's Awake — on each placed copy, which would undo our paint and model swaps.
        /// Applies those swaps to the template once (its copies start from them) and empties the list.
        /// </summary>
        public static void SettleOverrides(InteractableObject tpl)
        {
            if (tpl == null || tpl.m_OverrideMeshDataList == null || tpl.m_OverrideMeshDataList.Count == 0) return;
            EvaluateMeshOverride?.Invoke(tpl);
            tpl.m_OverrideMeshDataList.Clear();
        }

        /// <param name="body">The base's renderers that make up its look (hidden unless <paramref name="keep"/> says otherwise).</param>
        /// <param name="keep">Renderers that stay visible (working parts…). The highlight shell and the placement area are always kept.</param>
        /// <param name="texPath">The texture's file, if any (a texture-array base takes its slice from it).</param>
        public static MeshRenderer Apply(InteractableObject tpl, IEnumerable<Renderer> body, Mesh mesh, Texture tex, Color? tint, string id,
            System.Func<Transform, bool> keep = null, string texPath = null)
        {
            var bodyList = body.ToList();
            var mainRenderer = tpl.m_Mesh;
            Material mainMat = mainRenderer != null ? mainRenderer.sharedMaterial : bodyList.Select(r => r.sharedMaterial).FirstOrDefault(m => m != null);

            var go = new GameObject("TCGCC_Model");
            go.transform.SetParent(tpl.transform, false);
            go.layer = tpl.gameObject.layer;
            var mf = go.AddComponent<MeshFilter>();
            mf.sharedMesh = mesh;
            var mr = go.AddComponent<MeshRenderer>();
            var m = mainMat != null ? new Material(mainMat) : new Material(Shader.Find("Standard"));
            m.name = $"TCGCC_{id}";
            m.hideFlags = HideFlags.DontUnloadUnusedAsset;
            // A base drawn from a texture array (wall decorations: the posters' array) shows a slice picked by "_TexIndex": our
            // texture becomes a slice of that array, and the game's own decoration code sets the index on every placed copy.
            int slice = Hooks.ItemVisuals.AddSlice(m, texPath, tex as Texture2D);
            if (slice < 0)
            {
                m.mainTexture = tex != null ? tex : Texture2D.whiteTexture;
                m.mainTextureScale = Vector2.one;
                m.mainTextureOffset = Vector2.zero;
            }
            if (m.HasProperty("_Color")) m.color = tint ?? Color.white;
            // One material per submesh (the OBJ's "usemtl" groups all read the same texture).
            var mats = new Material[Mathf.Max(1, mesh.subMeshCount)];
            for (int i = 0; i < mats.Length; i++) mats[i] = m;
            mr.sharedMaterials = mats;
            if (slice >= 0)
            {
                if (tpl is InteractableDecoration deco) { deco.m_SpecialMesh = mr; deco.m_SpecialIndex = slice; }
                else Plugin.Log.LogError($"Model {id}: base {tpl.name} draws from a texture array but isn't a decoration — its texture won't show");
            }

            var area = tpl.m_MoveStateValidArea != null ? (tpl.m_MoveStateValidArea.parent != null && tpl.m_MoveStateValidArea.parent != tpl.transform
                ? tpl.m_MoveStateValidArea.parent : tpl.m_MoveStateValidArea) : null;
            foreach (var r in bodyList)
            {
                if (r == null || IsUnder(r.transform, tpl.m_HighlightGameObj) || (area != null && IsUnder(r.transform, area.gameObject))) continue;
                if (keep != null && keep(r.transform)) continue;
                r.enabled = false;
            }
            tpl.m_Mesh = mr;
            tpl.m_PickupObjectMesh = mf;
            ReplaceHighlight(tpl, mesh);
            if (tpl.m_CullingCheckMesh != null && !tpl.m_CullingCheckMesh.enabled) tpl.m_CullingCheckMesh = mr;
            FitCollider(tpl.m_BoxCollider, go.transform, mesh.bounds);
            return mr;
        }

        /// <summary>
        /// Objects without an outline on their main material (e.g. shelves) show m_HighlightGameObj while aimed at: a separate
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

        public static bool IsUnder(Transform t, GameObject root) => root != null && (t == root.transform || t.IsChildOf(root.transform));

        /// <summary>Sets a box collider to the bounds of <paramref name="localBounds"/> (in <paramref name="space"/>), converted into the collider's space.</summary>
        public static void FitCollider(BoxCollider col, Transform space, Bounds localBounds)
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

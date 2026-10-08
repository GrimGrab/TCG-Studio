using System.Collections.Generic;
using System.IO;
using TCGCustomCards.Core;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// A piece painted in TCG Studio (<see cref="FurniturePaintDef"/>): each listed body renderer gets Studio's mesh (same shape, new UVs
    /// into one atlas) and copies of its materials that read the painted atlas in white — the vanilla colours are baked into it.
    /// Other texture slots (normal/detail maps) are cleared because they followed the old UVs. Everything else of the piece (colliders,
    /// highlight, spots) stays the base's. Runs on the template before spots/points are rebuilt, so the index paths are the prefab's.
    /// </summary>
    internal static class FurniturePaint
    {
        public static void Apply(InteractableObject tpl, FurnitureDef def)
        {
            var tex = ImageCache.Get(Path.Combine(def.FolderPath, def.Paint.Texture))?.texture;
            if (tex == null) { Plugin.Log.LogWarning($"Furniture '{def.Id}': painted texture failed to load — base look kept"); return; }
            var copies = new Dictionary<Material, Material>();
            int done = 0;
            foreach (var part in def.Paint.Parts)
            {
                var t = Find(tpl.transform, part.Renderer);
                var mf = t != null ? t.GetComponent<MeshFilter>() : null;
                var mr = t != null ? t.GetComponent<MeshRenderer>() : null;
                if (mf == null || mr == null)
                {
                    Plugin.Log.LogWarning($"Furniture '{def.Id}': paint part '{part.Renderer}' not found on {def.BaseObject} (game update?) — left as it is");
                    continue;
                }
                var mesh = MeshLoader.Get(Path.Combine(def.FolderPath, part.Mesh));
                if (mesh == null) continue;
                if (mesh.subMeshCount != mr.sharedMaterials.Length)
                    Plugin.Log.LogWarning($"Furniture '{def.Id}': paint part '{part.Renderer}' has {mesh.subMeshCount} submeshes for {mr.sharedMaterials.Length} materials");
                mf.sharedMesh = mesh;
                var mats = mr.sharedMaterials;
                for (int i = 0; i < mats.Length; i++) mats[i] = Painted(mats[i], tex, def.Id, copies);
                mr.sharedMaterials = mats;
                done++;
            }
            Plugin.Log.LogInfo($"Furniture '{def.Id}': painted {done} of {def.Paint.Parts.Count} part(s)");
        }

        private static Material Painted(Material m, Texture2D tex, string id, Dictionary<Material, Material> copies)
        {
            if (m == null) return null;
            if (copies.TryGetValue(m, out var copy)) return copy;
            copy = new Material(m) { name = $"TCGCC_{id}_{m.name}", hideFlags = HideFlags.DontUnloadUnusedAsset };
            foreach (var prop in copy.GetTexturePropertyNames()) copy.SetTexture(prop, null);
            copy.mainTexture = tex;
            copy.mainTextureScale = Vector2.one;
            copy.mainTextureOffset = Vector2.zero;
            if (copy.HasProperty("_Color")) copy.color = Color.white;
            if (copy.HasProperty("_BaseColor")) copy.SetColor("_BaseColor", Color.white);
            return copies[m] = copy;
        }

        /// <summary>"&lt;index&gt;:&lt;name&gt;/…" under the root: by sibling index, checked against the name ("" = the root).</summary>
        private static Transform Find(Transform root, string path)
        {
            var t = root;
            if (string.IsNullOrEmpty(path)) return t;
            foreach (var step in path.Split('/'))
            {
                int colon = step.IndexOf(':');
                if (colon < 0 || !int.TryParse(step.Substring(0, colon), out int index) || index < 0 || index >= t.childCount) return null;
                var child = t.GetChild(index);
                if (child.name != step.Substring(colon + 1)) return null;
                t = child;
            }
            return t;
        }
    }
}

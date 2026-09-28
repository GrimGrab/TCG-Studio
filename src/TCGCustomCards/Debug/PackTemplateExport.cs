using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using Newtonsoft.Json;
using UnityEngine;

namespace TCGCustomCards.Debug
{
    /// <summary>
    /// Exports the card pack / card box meshes for TCG Studio's pack editor into &lt;plugin&gt;\templates\accessories\ (next to the
    /// accessory meshes, same <c>{mesh}.obj</c> format) plus <c>packs.json</c>: per pack/box item its mesh, texture and bounds, and
    /// every other renderer in the loaded scenes that draws a pack/box texture (e.g. the pack torn open in
    /// <see cref="CardOpeningSequence"/>, a skinned mesh that gets the shelf pack's material) — their UVs must agree with the
    /// shelf mesh for one texture to work everywhere.
    /// </summary>
    internal static class PackTemplateExport
    {
        public static int Run(string templatesDir)
        {
            string dir = Path.Combine(templatesDir, "accessories");
            Directory.CreateDirectory(dir);
            var so = CSingleton<InventoryBase>.Instance.m_StockItemData_SO;
            int n = 0;
            var meshes = new Dictionary<Mesh, string>();
            var items = new List<object>();
            var textures = new HashSet<Texture>();

            for (int i = 0; i < (int)EItemType.Max && i < so.m_ItemDataList.Count; i++)
            {
                var type = (EItemType)i;
                string name = type.ToString();
                bool pack = name.Contains("CardPack") || name.EndsWith("Pack");
                bool box = name.Contains("CardBox");
                if (!pack && !box) continue;
                var md = so.m_ItemMeshDataList[i];
                if (md?.mesh == null) continue;
                var tex = md.material != null ? md.material.mainTexture : null;
                if (tex != null) textures.Add(tex);
                items.Add(new
                {
                    type = name, id = i, kind = box ? "box" : "pack",
                    mesh = AccessoryTemplateExport.ExportMesh(md.mesh, dir, meshes, ref n), meshName = md.mesh.name,
                    bounds = AccessoryTemplateExport.Bounds(md.mesh.bounds),
                    meshSecondary = md.meshSecondary?.name,
                    meshSecondaryFile = md.meshSecondary != null ? AccessoryTemplateExport.ExportMesh(md.meshSecondary, dir, meshes, ref n) : null,
                    textureName = tex?.name, texture = tex != null ? $"{name}_texture.png" : null,
                    material = AccessoryTemplateExport.Mat(md.material),
                    materialSecondary = AccessoryTemplateExport.Mat(md.materialSecondary),
                });
            }

            // Other renderers drawing a pack/box texture (opening sequence, held/opened boxes, prefabs in memory).
            var renderers = new List<object>();
            void Add(string owner, Renderer r, Mesh mesh)
            {
                if (r == null || mesh == null) return;
                var tex = r.sharedMaterial != null ? r.sharedMaterial.mainTexture : null;
                renderers.Add(new
                {
                    owner, path = AccessoryTemplateExport.PathOf(r.transform), skinned = r is SkinnedMeshRenderer,
                    mesh = AccessoryTemplateExport.ExportMesh(mesh, dir, meshes, ref n), meshName = mesh.name,
                    bounds = AccessoryTemplateExport.Bounds(mesh.bounds), textureName = tex?.name,
                    material = AccessoryTemplateExport.Mat(r.sharedMaterial),
                });
            }
            var seq = CSingleton<CardOpeningSequence>.Instance;
            if (seq != null && seq.m_CardPackMesh != null)
                Add("CardOpeningSequence.m_CardPackMesh", seq.m_CardPackMesh, seq.m_CardPackMesh.sharedMesh);
            var seen = new HashSet<Mesh>(meshes.Keys);
            foreach (var r in Resources.FindObjectsOfTypeAll<Renderer>())
            {
                var mat = r.sharedMaterial;
                if (mat == null || mat.mainTexture == null || !textures.Contains(mat.mainTexture)) continue;
                Mesh mesh = r is SkinnedMeshRenderer s ? s.sharedMesh : r.GetComponent<MeshFilter>()?.sharedMesh;
                if (mesh == null || !seen.Add(mesh)) continue;
                Add(r.GetType().Name, r, mesh);
            }

            File.WriteAllText(Path.Combine(dir, "packs.json"),
                JsonConvert.SerializeObject(new { version = 1, items, renderers }, Formatting.Indented));
            Plugin.Log.LogInfo($"Pack templates: {items.Count} items, {renderers.Count} extra renderers, {meshes.Count} meshes → {dir}");
            return n + 1;
        }
    }
}

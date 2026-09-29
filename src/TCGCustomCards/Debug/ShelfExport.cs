using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Text;
using UnityEngine;

namespace TCGCustomCards.Debug
{
    /// <summary>
    /// Template export v11: the shop item prefab (where the item mesh sits under the Item root) and every loaded shop shelf — its
    /// compartments (slot grid, start corner and axes, flags) and its model as one OBJ in the shelf's root space
    /// (<c>Shelf_{name}.obj</c>). TCG Studio rebuilds <c>ShelfCompartment.CalculatePositionList</c> from this to stand custom figurines on a real shelf.
    /// </summary>
    internal static class ShelfExport
    {
        public static object ItemPrefab()
        {
            var item = Resources.FindObjectsOfTypeAll<Item>().FirstOrDefault(i => i != null && i.m_MeshFilter != null);
            if (item == null) return null;
            var root = item.transform;
            var m = root.worldToLocalMatrix * item.m_MeshFilter.transform.localToWorldMatrix;
            return new
            {
                name = item.name, meshPath = AccessoryTemplateExport.PathOf(item.m_MeshFilter.transform),
                meshMatrix = Rows(m),
                meshLocalPosition = AccessoryTemplateExport.V(root.InverseTransformPoint(item.m_MeshFilter.transform.position)),
                meshLossyScale = AccessoryTemplateExport.V(item.m_MeshFilter.transform.lossyScale),
                rootLossyScale = AccessoryTemplateExport.V(root.lossyScale)
            };
        }

        public static List<object> Run(string dir, ref int n)
        {
            var result = new List<object>();
            var seen = new HashSet<string>();
            foreach (var shelf in Resources.FindObjectsOfTypeAll<Shelf>())
            {
                if (shelf == null || shelf.m_ShelfCompartmentGrpList == null) continue;
                string name = shelf.name.Replace("(Clone)", "").Trim();
                if (!seen.Add(name)) continue;
                var root = shelf.transform;

                var comps = new List<object>();
                foreach (var grp in shelf.m_ShelfCompartmentGrpList)
                {
                    if (grp == null) continue;
                    for (int j = 0; j < grp.childCount; j++)
                    {
                        var c = grp.GetChild(j).GetComponent<ShelfCompartment>();
                        if (c == null || c.m_StartLoc == null || c.m_EndWidthLoc == null || c.m_EndDepthLoc == null || c.m_EndHeightLoc == null) continue;
                        Vector3 P(Transform t) => root.InverseTransformPoint(t.position);
                        Vector3 D(Vector3 v) => root.InverseTransformDirection(v);
                        comps.Add(new
                        {
                            path = AccessoryTemplateExport.PathOf(c.transform),
                            start = AccessoryTemplateExport.V(P(c.m_StartLoc)),
                            endWidth = AccessoryTemplateExport.V(P(c.m_EndWidthLoc)),
                            endDepth = AccessoryTemplateExport.V(P(c.m_EndDepthLoc)),
                            endHeight = AccessoryTemplateExport.V(P(c.m_EndHeightLoc)),
                            // Slot axes as used by CalculatePositionList: width along -right, depth along forward, height along ±up.
                            right = AccessoryTemplateExport.V(D(c.m_StartLoc.right)),
                            up = AccessoryTemplateExport.V(D(c.m_StartLoc.up)),
                            forward = AccessoryTemplateExport.V(D(c.m_StartLoc.forward)),
                            // World lengths (what the game measures).
                            width = (c.m_EndWidthLoc.position - c.m_StartLoc.position).magnitude,
                            depth = (c.m_EndDepthLoc.position - c.m_StartLoc.position).magnitude,
                            height = (c.m_EndHeightLoc.position - c.m_StartLoc.position).magnitude,
                            sizeX = c.m_SizeX, sizeY = c.m_SizeY, sizeZ = c.m_SizeZ,
                            c.m_CanPutItem, c.m_CanPutBox, c.m_ApplyScaleOffset, c.m_HeightGoesUp, c.m_AffectedByTallItem, c.m_ItemNotForSale,
                            posCount = c.m_PosListGrp != null ? c.m_PosListGrp.childCount : 0,
                        });
                    }
                }
                if (comps.Count == 0) continue;

                string file = null;
                try { file = WriteModel(shelf, dir, name, ref n); }
                catch (Exception e) { Plugin.Log.LogWarning($"Shelf {name}: model export failed: {e.Message}"); }
                result.Add(new
                {
                    name, isPrefab = !shelf.gameObject.scene.IsValid(), shelf.m_ItemNotForSale,
                    rootLossyScale = AccessoryTemplateExport.V(root.lossyScale), mesh = file, compartments = comps,
                });
            }
            Plugin.Log.LogInfo($"Shelf export: {result.Count} shelf type(s)");
            return result;
        }

        /// <summary>All static renderers of the shelf (not items or boxes standing on it) merged into one OBJ in the shelf's root space.</summary>
        private static string WriteModel(Shelf shelf, string dir, string name, ref int n)
        {
            var root = shelf.transform;
            var inv = CultureInfo.InvariantCulture;
            var sb = new StringBuilder();
            sb.AppendLine($"# Shelf {name} — exported by TCGCustomCards (shelf root space, Unity: left-handed, Y up)");
            int baseIndex = 1, parts = 0, tris = 0;
            foreach (var r in shelf.GetComponentsInChildren<MeshRenderer>(true))
            {
                if (parts >= 80 || tris > 60000) break;
                if (!r.enabled || !r.gameObject.activeInHierarchy && shelf.gameObject.scene.IsValid()) continue;
                if (r.GetComponentInParent<Item>() != null || r.GetComponentInParent<InteractablePackagingBox>() != null) continue;
                var mf = r.GetComponent<MeshFilter>();
                if (mf == null || mf.sharedMesh == null) continue;
                if (!MeshReader.Read(mf.sharedMesh, out var pos, out var nrm, out var uv, out var subs)) continue;
                var m = root.worldToLocalMatrix * r.transform.localToWorldMatrix;
                bool mirrored = m.determinant < 0;
                foreach (var v in pos)
                {
                    var p = m.MultiplyPoint3x4(v);
                    sb.AppendLine(string.Format(inv, "v {0} {1} {2}", p.x, p.y, p.z));
                }
                bool hasN = nrm != null && nrm.Length == pos.Length;
                if (hasN)
                    foreach (var v in nrm)
                    {
                        var d = m.MultiplyVector(v).normalized;
                        sb.AppendLine(string.Format(inv, "vn {0} {1} {2}", d.x, d.y, d.z));
                    }
                sb.AppendLine($"g {AccessoryTemplateExport.PathOf(r.transform).Replace(' ', '_')}");
                foreach (var tri in subs)
                    for (int i = 0; i + 2 < tri.Length; i += 3)
                    {
                        int a = tri[i] + baseIndex, b = tri[i + 1] + baseIndex, c = tri[i + 2] + baseIndex;
                        if (mirrored) { int t = b; b = c; c = t; }
                        sb.AppendLine(hasN ? $"f {a}//{a} {b}//{b} {c}//{c}" : $"f {a} {b} {c}");
                        tris++;
                    }
                baseIndex += pos.Length;
                parts++;
            }
            if (parts == 0) return null;
            string file = "Shelf_" + string.Concat(name.Select(ch => char.IsLetterOrDigit(ch) || ch == '_' || ch == '-' ? ch : '_')) + ".obj";
            File.WriteAllText(Path.Combine(dir, file), sb.ToString());
            n++;
            return file;
        }

        private static float[][] Rows(Matrix4x4 m) => Enumerable.Range(0, 4).Select(r => new[] { m[r, 0], m[r, 1], m[r, 2], m[r, 3] }).ToArray();
    }
}

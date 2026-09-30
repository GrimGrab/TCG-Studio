using System.Globalization;
using System.IO;
using System.Linq;
using System.Text;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// Writes &lt;plugin&gt;\furniture-templates.txt once per session (with [Debug] DumpDiagnostics): every furniture prefab's component,
    /// shop data and spots in the accessories.json "spots" format (piece-local, as <see cref="FurnitureSpots"/> reads them), plus the
    /// hierarchy of the pieces custom furniture is built on. Starting point for writing spots and a check of the prefab layout.
    /// </summary>
    internal static class FurnitureTemplateDump
    {
        private static bool _written;
        private static readonly CultureInfo Inv = CultureInfo.InvariantCulture;

        public static void Write(ShelfData_ScriptableObject so)
        {
            if (_written || Plugin.DumpDiagnostics == null || !Plugin.DumpDiagnostics.Value) return;
            _written = true;
            try
            {
                var sb = new StringBuilder();
                sb.AppendLine("# Furniture templates (TCG Custom Cards) — spots are piece-local: metres, degrees; see docs 'furniture'");
                var bases = new System.Collections.Generic.HashSet<EObjectType>(Registry.Furniture.Select(f => f.Def.BaseObject));
                foreach (var od in so.m_ObjectDataList)
                {
                    if (od?.spawnPrefab == null) continue;
                    var p = od.spawnPrefab;
                    var buy = so.m_FurniturePurchaseDataList.Find(x => x != null && x.objectType == od.objectType);
                    sb.AppendLine();
                    sb.AppendLine($"## {od.objectType} ({(int)od.objectType}) — {p.GetType().Name}{(p.m_IsGenericObject ? " [generic]" : "")}" +
                                  (buy != null ? $" — price {F(buy.price)}, level {buy.levelRequirement}" : " — not sold") + $", decoBonus {F(od.decoBonus)}");
                    var mf = p.m_Mesh != null ? p.m_Mesh.GetComponent<MeshFilter>() : null;
                    if (mf != null && mf.sharedMesh != null) sb.AppendLine($"main mesh '{mf.sharedMesh.name}' bounds {B(mf.sharedMesh.bounds)} on '{Path(p.transform, p.m_Mesh.transform)}'");
                    if (p.m_BoxCollider != null) sb.AppendLine($"collider '{Path(p.transform, p.m_BoxCollider.transform)}' centre {V(p.m_BoxCollider.center)} size {V(p.m_BoxCollider.size)}");
                    var items = p.GetComponentsInChildren<ShelfCompartment>(true);
                    var cards = p.GetComponentsInChildren<InteractableCardCompartment>(true);
                    if (items.Length + cards.Length > 0)
                    {
                        sb.AppendLine("\"spots\": [");
                        var lines = items.Select(c => ItemSpot(p.transform, c)).Concat(cards.Select(c => CardSpot(p.transform, c))).ToList();
                        sb.AppendLine(string.Join(",\n", lines));
                        sb.AppendLine("]");
                    }
                    if (bases.Contains(od.objectType)) Hierarchy(sb, p.transform, p.transform, 0);
                }
                File.WriteAllText(System.IO.Path.Combine(Plugin.PluginDir, "furniture-templates.txt"), sb.ToString());
            }
            catch (System.Exception e)
            {
                Plugin.Log.LogWarning($"furniture-templates.txt not written: {e.Message}");
            }
        }

        private static string ItemSpot(Transform piece, ShelfCompartment c)
        {
            if (c.m_StartLoc == null || c.m_EndWidthLoc == null || c.m_EndDepthLoc == null || c.m_EndHeightLoc == null)
                return $"  {{ \"kind\": \"items\", \"note\": \"{c.name}: missing box markers\" }}";
            var s = c.m_StartLoc;
            float w = (c.m_EndWidthLoc.position - s.position).magnitude;
            float d = (c.m_EndDepthLoc.position - s.position).magnitude;
            float h = (c.m_EndHeightLoc.position - s.position).magnitude;
            var upDir = (c.m_HeightGoesUp ? 1f : -1f) * s.up;
            var centre = s.position - s.right * (w * 0.5f) + s.forward * (d * 0.5f) + upDir * (h * 0.5f);
            var sb = new StringBuilder();
            sb.Append($"  {{ \"kind\": \"items\", \"pos\": {V(piece.InverseTransformPoint(centre))}, \"rot\": {E(piece, s.rotation)}, " +
                      $"\"size\": [{F(w)}, {F(d)}, {F(h)}], \"grid\": [{c.m_SizeX}, {c.m_SizeY}, {c.m_SizeZ}]");
            if (c.m_CustomerStandLoc != null)
            {
                var cs = piece.InverseTransformPoint(c.m_CustomerStandLoc.position);
                sb.Append($", \"customer\": [{F(cs.x)}, {F(cs.z)}]");
            }
            if (c.m_InteractablePriceTagList != null && c.m_InteractablePriceTagList.Count > 0 && c.m_InteractablePriceTagList[0] != null)
                sb.Append($", \"priceTag\": {V(piece.InverseTransformPoint(c.m_InteractablePriceTagList[0].transform.position))}");
            sb.Append($", \"boxes\": {(c.m_CanPutBox ? "true" : "false")}");
            sb.Append($" }}  // {c.name}, heightGoesUp={c.m_HeightGoesUp}, slots={(c.m_PosListGrp != null ? c.m_PosListGrp.childCount : -1)}, " +
                      $"colliders={c.GetComponents<BoxCollider>().Length}, priceTags={c.m_InteractablePriceTagList?.Count ?? 0}" +
                      $"{(c.m_InteractablePriceTagList != null && c.m_InteractablePriceTagList.Any(t => t != null && !t.transform.IsChildOf(c.transform)) ? " (tag outside compartment!)" : "")}");
            return sb.ToString();
        }

        private static string CardSpot(Transform piece, InteractableCardCompartment c)
        {
            var at = c.m_PutCardLocation != null ? c.m_PutCardLocation : c.transform;
            var sb = new StringBuilder();
            sb.Append($"  {{ \"kind\": \"card\", \"pos\": {V(piece.InverseTransformPoint(at.position))}, \"rot\": {E(piece, at.rotation)}");
            if (c.m_CustomerStandLoc != null)
            {
                var cs = piece.InverseTransformPoint(c.m_CustomerStandLoc.position);
                sb.Append($", \"customer\": [{F(cs.x)}, {F(cs.z)}]");
            }
            if (c.m_InteractablePriceTagList != null && c.m_InteractablePriceTagList.Count > 0 && c.m_InteractablePriceTagList[0] != null)
                sb.Append($", \"priceTag\": {V(piece.InverseTransformPoint(c.m_InteractablePriceTagList[0].transform.position))}");
            sb.Append($" }}  // {c.name}, colliders={c.GetComponents<Collider>().Length}");
            return sb.ToString();
        }

        private static void Hierarchy(StringBuilder sb, Transform root, Transform t, int depth)
        {
            if (depth > 12) return;
            var comps = t.GetComponents<Component>().Where(c => c != null && !(c is Transform)).Select(c => c.GetType().Name);
            sb.AppendLine($"{new string(' ', depth * 2)}- {t.name} [{string.Join(", ", comps)}] local {V(t.localPosition)} {V(t.localEulerAngles)} scale {V(t.localScale)}");
            for (int i = 0; i < t.childCount; i++) Hierarchy(sb, root, t.GetChild(i), depth + 1);
        }

        private static string Path(Transform root, Transform t)
        {
            var parts = new System.Collections.Generic.List<string>();
            for (var p = t; p != null && p != root; p = p.parent) parts.Insert(0, p.name);
            return parts.Count == 0 ? "(root)" : string.Join("/", parts);
        }

        private static string E(Transform piece, Quaternion world)
        {
            var e = (Quaternion.Inverse(piece.rotation) * world).eulerAngles;
            return V(new Vector3(Norm(e.x), Norm(e.y), Norm(e.z)));
        }

        private static float Norm(float a) { a %= 360f; if (a > 180f) a -= 360f; if (a <= -180f) a += 360f; return Mathf.Abs(a) < 0.005f ? 0f : a; }
        private static string F(float v) => v.ToString("0.###", Inv);
        private static string V(Vector3 v) => $"[{F(v.x)}, {F(v.y)}, {F(v.z)}]";
        private static string B(Bounds b) => $"centre {V(b.center)} size {V(b.size)}";
    }
}

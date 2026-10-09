using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Text;
using HarmonyLib;
using UnityEngine;

namespace TCGCustomCards.Debug
{
    /// <summary>
    /// Writes &lt;plugin&gt;\pack-opening.txt once per session (with [Debug] DumpDiagnostics): how the pack-opening scene is built
    /// (the 7 card slots, their animation roots, final-reveal positions, clips) and where the 7 cards end up on screen in the
    /// final reveal. Measurements for N-card packs (cloned slots and a final layout that must fit the same screen area).
    /// </summary>
    internal static class PackOpeningDump
    {
        private static readonly CultureInfo Inv = CultureInfo.InvariantCulture;
        private static bool _done, _active;
        private static int _lastState = int.MinValue, _samples;
        private static readonly StringBuilder Sb = new StringBuilder();

        private static bool Enabled => !_done && Plugin.DumpDiagnostics != null && Plugin.DumpDiagnostics.Value;

        [HarmonyPatch(typeof(CardOpeningSequence), nameof(CardOpeningSequence.OpenScreen))]
        internal static class OpenScreenPatch
        {
            private static void Postfix(CardOpeningSequence __instance, ECollectionPackType collectionPackType)
            {
                if (!Enabled || _active) return;
                try
                {
                    _active = true;
                    _lastState = int.MinValue;
                    _samples = 0;
                    Sb.Clear();
                    Static(__instance, collectionPackType);
                }
                catch (System.Exception e)
                {
                    Plugin.Log.LogWarning($"pack-opening dump failed: {e}");
                    _active = false;
                    _done = true;
                }
            }
        }

        [HarmonyPatch(typeof(CardOpeningSequence), "Update")]
        internal static class UpdatePatch
        {
            private static void Postfix(CardOpeningSequence __instance)
            {
                if (!_active) return;
                try
                {
                    Live(__instance);
                }
                catch (System.Exception e)
                {
                    Plugin.Log.LogWarning($"pack-opening dump failed: {e}");
                    _active = false;
                    _done = true;
                }
            }
        }

        private static void Static(CardOpeningSequence s, ECollectionPackType pack)
        {
            Sb.AppendLine("# Pack opening scene (TCG Custom Cards) - positions in metres, angles in degrees");
            Sb.AppendLine($"pack {pack} ({(int)pack}); lists: card3dUI {s.m_Card3dUIList?.Count}, anim {s.m_CardAnimList?.Count}, showAll {s.m_ShowAllCardPosList?.Count}");
            Sb.AppendLine($"sequence object '{Path(s.transform)}'; rotate-to-front anim on '{Path(s.m_CardOpeningRotateToFrontAnim?.transform)}'; opening UI group '{Path(s.m_CardOpeningUIGroup?.transform)}'");
            int n = Mathf.Max(s.m_Card3dUIList?.Count ?? 0, Mathf.Max(s.m_CardAnimList?.Count ?? 0, s.m_ShowAllCardPosList?.Count ?? 0));
            for (int i = 0; i < n; i++)
            {
                Sb.AppendLine($"## slot {i}");
                var a = i < s.m_CardAnimList.Count ? s.m_CardAnimList[i] : null;
                var c = i < s.m_Card3dUIList.Count ? s.m_Card3dUIList[i] : null;
                var p = i < s.m_ShowAllCardPosList.Count ? s.m_ShowAllCardPosList[i] : null;
                if (a != null)
                {
                    Sb.AppendLine($"anim root '{Path(a.transform)}' {T(a.transform)}");
                    Sb.AppendLine("  clips: " + string.Join(", ", ClipStates(a).Select(st => $"{st.name} {F(st.length)}s wrap {st.wrapMode}")));
                }
                if (c != null)
                {
                    var canvas = c.GetComponentInParent<Canvas>();
                    Sb.AppendLine($"card3dUI '{Path(c.transform)}' {T(c.transform)} childOfAnim={(a != null && c.transform.IsChildOf(a.transform))} " +
                                  $"canvasInParents={(canvas != null ? Path(canvas.transform) : "none")} scaleGrp {(c.m_ScaleGrp != null ? V(c.m_ScaleGrp.localScale) : "-")} " +
                                  $"own anim {(c.m_Anim != null ? string.Join("/", ClipStates(c.m_Anim).Select(st => st.name)) : "-")}");
                }
                if (p != null) Sb.AppendLine($"showAll '{Path(p)}' {T(p)}");
            }
            if (s.m_CardAnimList.Count > 0 && s.m_CardAnimList[0].transform.parent != null)
            {
                Sb.AppendLine("## hierarchy under the first anim root's parent");
                Hierarchy(s.m_CardAnimList[0].transform.parent, 0, 5);
            }
            if (s.m_ShowAllCardPosList.Count > 0 && s.m_ShowAllCardPosList[0].parent != null
                && (s.m_CardAnimList.Count == 0 || s.m_ShowAllCardPosList[0].parent != s.m_CardAnimList[0].transform.parent))
            {
                Sb.AppendLine("## hierarchy under the first showAll position's parent");
                Hierarchy(s.m_ShowAllCardPosList[0].parent, 0, 2);
            }
        }

        private static void Live(CardOpeningSequence s)
        {
            int st = s.m_StateIndex;
            // Root 0 during the final reveal: does OpenCardFinalReveal move/scale the root (it's placed on showAll[0] just before)?
            if (st >= 7 && st <= 10 && s.m_CardAnimList.Count > 0 && _samples < 40 && (st != _lastState || _samples % 3 == 0 || st == 8))
            {
                var r = s.m_CardAnimList[0].transform;
                var g = s.m_Card3dUIList.Count > 0 ? s.m_Card3dUIList[0] : null;
                Sb.AppendLine($"state {st} t={F(Time.time)}: root0 world {V(r.position)} local {V(r.localPosition)} scale {V(r.localScale)}" +
                              (g != null ? $" | card0 world {V(g.transform.position)} scale {V(g.transform.lossyScale)} scaleGrp {(g.m_ScaleGrp != null ? V(g.m_ScaleGrp.localScale) : "-")}" : ""));
                _samples++;
            }
            if (st == 10 && _lastState != 10)
            {
                Final(s);
                File.WriteAllText(System.IO.Path.Combine(Plugin.PluginDir, "pack-opening.txt"), Sb.ToString());
                Plugin.Log.LogInfo("pack-opening.txt written (pack opening measurements)");
                _active = false;
                _done = true;
            }
            _lastState = st;
        }

        /// <summary>Final reveal at rest: camera and where each card is on screen (viewport 0..1, y up).</summary>
        private static void Final(CardOpeningSequence s)
        {
            // The player's camera (Camera.main is null in this scene).
            var ipc = CSingleton<InteractionPlayerController>.Instance;
            var cam = ipc != null && ipc.m_Cam != null ? ipc.m_Cam : Camera.main;
            Sb.AppendLine("## final reveal at rest (state 10)");
            if (cam == null) { Sb.AppendLine("no camera"); return; }
            Sb.AppendLine($"camera '{Path(cam.transform)}' world {V(cam.transform.position)} rot {V(cam.transform.eulerAngles)} fov {F(cam.fieldOfView)} aspect {F(cam.aspect)} screen {Screen.width}x{Screen.height}");
            float minX = 1, minY = 1, maxX = 0, maxY = 0;
            for (int i = 0; i < s.m_Card3dUIList.Count; i++)
            {
                var g = s.m_Card3dUIList[i];
                var rt = g.m_CardUI != null ? g.m_CardUI.transform as RectTransform : null;
                string rect = "-";
                if (rt != null)
                {
                    var corners = new Vector3[4];
                    rt.GetWorldCorners(corners);
                    var vp = corners.Select(w => cam.WorldToViewportPoint(w)).ToArray();
                    float x0 = vp.Min(v => v.x), x1 = vp.Max(v => v.x), y0 = vp.Min(v => v.y), y1 = vp.Max(v => v.y);
                    rect = $"x {F(x0)}..{F(x1)} y {F(y0)}..{F(y1)} (w {F(x1 - x0)} h {F(y1 - y0)}) depth {F(vp[0].z)}";
                    minX = Mathf.Min(minX, x0); maxX = Mathf.Max(maxX, x1); minY = Mathf.Min(minY, y0); maxY = Mathf.Max(maxY, y1);
                }
                var a = i < s.m_CardAnimList.Count ? s.m_CardAnimList[i].transform : null;
                Sb.AppendLine($"card {i}: active {g.gameObject.activeInHierarchy} viewport {rect}" +
                              (a != null ? $" | root world {V(a.position)} rot {V(a.eulerAngles)} scale {V(a.lossyScale)}" : ""));
                foreach (var ren in g.GetComponentsInChildren<Renderer>(false).Take(4))
                    Sb.AppendLine($"    renderer '{ren.name}' bounds centre {V(ren.bounds.center)} size {V(ren.bounds.size)}");
            }
            Sb.AppendLine($"all cards viewport: x {F(minX)}..{F(maxX)} y {F(minY)}..{F(maxY)}");
        }

        private static List<AnimationState> ClipStates(Animation a)
        {
            var r = new List<AnimationState>();
            foreach (AnimationState st in a) r.Add(st);
            return r;
        }

        private static void Hierarchy(Transform t, int depth, int maxDepth)
        {
            var comps = t.GetComponents<Component>().Where(c => c != null && !(c is Transform)).Select(c => c.GetType().Name);
            Sb.AppendLine($"{new string(' ', depth * 2)}- {t.name} [{string.Join(", ", comps)}] {T(t)}{(t.gameObject.activeSelf ? "" : " (inactive)")}");
            if (depth >= maxDepth) { if (t.childCount > 0) Sb.AppendLine($"{new string(' ', depth * 2 + 2)}... {t.childCount} children"); return; }
            for (int i = 0; i < t.childCount; i++) Hierarchy(t.GetChild(i), depth + 1, maxDepth);
        }

        private static string Path(Transform t)
        {
            if (t == null) return "null";
            var parts = new List<string>();
            for (var p = t; p != null; p = p.parent) parts.Insert(0, p.name);
            return string.Join("/", parts);
        }

        private static string T(Transform t) =>
            $"local {V(t.localPosition)} rot {V(t.localEulerAngles)} scale {V(t.localScale)} world {V(t.position)}";

        private static string F(float v) => v.ToString("0.####", Inv);
        private static string V(Vector3 v) => $"[{F(v.x)}, {F(v.y)}, {F(v.z)}]";
    }
}

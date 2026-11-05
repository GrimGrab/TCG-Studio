using System;
using System.Collections.Generic;
using HarmonyLib;
using TMPro;
using UnityEngine;

namespace TCGCustomCards.Hooks
{
    /// <summary>
    /// Single owner of the end-of-day report patches (EndOfDayReportScreen). Features add rows (<see cref="AddRow"/>): each is
    /// a clone of a vanilla money row, appended to the screen's row list, so vanilla fills it in order after the total
    /// (Update walks m_EndDayReportTextUIList, :197-212). Rows aren't part of the total profit. <see cref="Closed"/> = the day's
    /// report was closed and vanilla reset its day stats (CloseScreen :92-104).
    /// </summary>
    internal static class DayReport
    {
        private class Row
        {
            public string Label;
            public Func<float> Value;
            public Func<bool> Shown;
            public GameObject Object;
            public EndDayReportTextUI Ui;
        }

        private static readonly List<Row> Rows = new List<Row>();
        /// <summary>Our row objects per screen instance (a save load rebuilds the scene).</summary>
        private static EndOfDayReportScreen _builtFor;

        /// <summary>The day's report was closed (going to the next day); vanilla's day stats are reset now.</summary>
        public static event Action Closed;

        /// <summary>A money row after the total profit (positive green, negative red, 0 white), while <paramref name="shown"/>.</summary>
        public static void AddRow(string label, Func<float> value, Func<bool> shown) =>
            Rows.Add(new Row { Label = label, Value = value, Shown = shown });

        // Vanilla rows by index (SetAllLerpNumber :60-84): 18 = rent + bills (a money row), 19 = total profit (last).
        private const int TemplateRow = 18;
        private const int LastRow = 19;

        private static readonly AccessTools.FieldRef<EndOfDayReportScreen, Color> Green =
            AccessTools.FieldRefAccess<EndOfDayReportScreen, Color>("m_GreenColor");
        private static readonly AccessTools.FieldRef<EndOfDayReportScreen, Color> Red =
            AccessTools.FieldRefAccess<EndOfDayReportScreen, Color>("m_RedColor");
        private static readonly AccessTools.FieldRef<EndOfDayReportScreen, Color> White =
            AccessTools.FieldRefAccess<EndOfDayReportScreen, Color>("m_WhiteColor");

        private static void Build(EndOfDayReportScreen screen)
        {
            if (_builtFor == screen && Rows.TrueForAll(r => r.Ui != null)) return;
            var list = screen.m_EndDayReportTextUIList;
            foreach (var r in Rows)
            {
                if (r.Ui != null) list.Remove(r.Ui);
                r.Ui = null;
                r.Object = null;
            }
            _builtFor = screen;
            if (list.Count <= LastRow || Rows.Count == 0) return;
            var template = list[TemplateRow];
            var last = list[LastRow];
            // A row = the number's own object or, when the label sits next to it, the parent holding just that one number.
            Transform RowOf(EndDayReportTextUI t) =>
                t.transform.parent != null && t.transform.parent.GetComponentsInChildren<EndDayReportTextUI>(true).Length == 1
                    ? t.transform.parent : t.transform;
            var templateRow = RowOf(template);
            var lastRow = RowOf(last);
            var parent = lastRow.parent;
            bool laidOut = parent != null && parent.GetComponent<UnityEngine.UI.LayoutGroup>() != null;
            Vector3 pitch = lastRow.localPosition - templateRow.localPosition;
            Plugin.Log.LogInfo($"Day report: adding {Rows.Count} row(s) after '{lastRow.name}' (parent '{parent?.name}', " +
                               $"{(laidOut ? "layout group" : $"pitch {pitch}")})");
            var after = lastRow;
            foreach (var row in Rows)
            {
                var clone = UnityEngine.Object.Instantiate(templateRow.gameObject, parent, false);
                clone.name = "TCGCC_" + row.Label;
                if (laidOut) clone.transform.SetSiblingIndex(after.GetSiblingIndex() + 1);
                else clone.transform.localPosition = after.localPosition + pitch;
                after = clone.transform;
                var ui = clone.GetComponentInChildren<EndDayReportTextUI>(true);
                // The label: any other text in the row; vanilla localizes it (I2 Localize), ours is set once.
                foreach (var loc in clone.GetComponentsInChildren<I2.Loc.Localize>(true)) UnityEngine.Object.Destroy(loc);
                foreach (var text in clone.GetComponentsInChildren<TextMeshProUGUI>(true))
                    if (text != ui.m_Text) text.text = row.Label;
                row.Object = clone;
                row.Ui = ui;
            }
        }

        [HarmonyPatch(typeof(EndOfDayReportScreen), "SetAllLerpNumber")]
        private static class FillPatch
        {
            private static void Postfix(EndOfDayReportScreen __instance)
            {
                if (Rows.Count == 0) return;
                try
                {
                    Build(__instance);
                    // Shown rows join vanilla's list (it reveals them one by one); hidden ones stay out of it.
                    var list = __instance.m_EndDayReportTextUIList;
                    foreach (var r in Rows)
                    {
                        if (r.Ui == null) continue;
                        list.Remove(r.Ui);
                        bool shown = r.Shown();
                        r.Object.SetActive(shown);
                        if (!shown) continue;
                        list.Add(r.Ui);
                        r.Ui.SetNumber(0f, r.Value(), 0.3f, Green(__instance), Red(__instance), White(__instance));
                    }
                }
                catch (Exception e)
                {
                    Plugin.Log.LogError($"Day report: extra rows failed: {e}");
                }
            }
        }

        [HarmonyPatch(typeof(EndOfDayReportScreen), nameof(EndOfDayReportScreen.CloseScreen))]
        private static class ClosePatch
        {
            private static void Postfix()
            {
                if (Closed == null) return;
                foreach (Action h in Closed.GetInvocationList())
                {
                    try { h(); }
                    catch (Exception e) { Plugin.Log.LogError($"Day report close handler failed: {e}"); }
                }
            }
        }
    }
}

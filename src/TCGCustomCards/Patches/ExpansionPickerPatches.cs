using System.Collections;
using System.Collections.Generic;
using System.Linq;
using HarmonyLib;
using TCGCustomCards.Runtime;
using TCGCustomCards.UI;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.Patches
{
    // Layer 6 — CardExpansionSelectScreen: the set picker shared by Check Price, Workbench and Bulk Donation quick-fill.
    // Vanilla buttons call OnPressButton(int) via persistent (scene-wired) listeners and m_BtnHighlightList[(int)expansion]
    // holds separate highlight objects. We find each button by its persistent int argument, clone one per custom set
    // (with a cloned highlight at the same offset) into a scrollable grid, and replace OpenScreen so custom ints map correctly.

    [HarmonyPatch(typeof(CardExpansionSelectScreen), nameof(CardExpansionSelectScreen.OpenScreen))]
    internal static class ExpansionPickerOpenPatch
    {
        private static readonly AccessTools.FieldRef<CardExpansionSelectScreen, int> CurrentIndex =
            AccessTools.FieldRefAccess<CardExpansionSelectScreen, int>("m_CurrentIndex");

        private static CardExpansionSelectScreen _builtFor;
        private static ButtonGrid _grid;
        /// <summary>Expansion int → (button, highlight) for custom sets.</summary>
        private static readonly Dictionary<int, (RectTransform button, GameObject highlight)> Custom = new Dictionary<int, (RectTransform, GameObject)>();

        private static bool Prefix(ECardExpansionType initCardExpansion)
        {
            var screen = CSingleton<CardExpansionSelectScreen>.Instance;
            if (_builtFor != screen) Build(screen);
            _grid?.Relayout(item => !VanillaFilter.HideCards || item.name.StartsWith("TCGCC_"));
            if (VanillaFilter.HideCards && !Registry.IsCustom(initCardExpansion)) initCardExpansion = VanillaFilter.FirstCustomExpansion;

            int index = (int)initCardExpansion;
            CurrentIndex(screen) = index;
            foreach (var h in screen.m_BtnHighlightList) if (h != null) h.SetActive(false);
            foreach (var c in Custom.Values) if (c.highlight != null) c.highlight.SetActive(false);

            if (Custom.TryGetValue(index, out var entry))
            {
                if (entry.highlight != null) entry.highlight.SetActive(true);
                _grid?.ScrollTo(entry.button);
            }
            else if (index >= 0 && index < screen.m_BtnHighlightList.Count && screen.m_BtnHighlightList[index] != null)
            {
                screen.m_BtnHighlightList[index].SetActive(true);
            }

            SoundManager.GenericMenuOpen();
            screen.m_ScreenGrp.SetActive(true);
            ControllerScreenUIExtManager.OnOpenScreen(screen.m_ControllerScreenUIExtension);
            return false;
        }

        private static void Build(CardExpansionSelectScreen screen)
        {
            _builtFor = screen;
            _grid = null;
            Custom.Clear();
            if (Registry.Sets.Count == 0) return;

            var byIndex = FindButtons(screen);
            if (byIndex.Count == 0)
            {
                Plugin.Log.LogWarning("Expansion picker: no buttons wired to OnPressButton(int) found; custom sets are not selectable here");
                return;
            }
            // Each button sits in its own row group (background + button + label). Rows are the children of the buttons'
            // deepest common ancestor; those are what we lay out and clone.
            var rowsByIndex = ToRows(byIndex);
            var vanilla = rowsByIndex.OrderBy(kv => kv.Key).Select(kv => kv.Value).ToList();

            // Reference highlight: the one belonging to the lowest-index row that has a highlight.
            int refIndex = rowsByIndex.Keys.OrderBy(k => k).FirstOrDefault(k => k < screen.m_BtnHighlightList.Count && screen.m_BtnHighlightList[k] != null);
            var refButton = rowsByIndex[refIndex];
            var refHighlight = refIndex < screen.m_BtnHighlightList.Count ? screen.m_BtnHighlightList[refIndex]?.transform as RectTransform : null;
            bool highlightInsideButton = refHighlight != null && refHighlight.IsChildOf(refButton);
            string highlightPath = highlightInsideButton ? RelativePath(refButton, refHighlight) : null;
            Vector3 highlightOffset = refHighlight != null && !highlightInsideButton ? refHighlight.localPosition - refButton.localPosition : Vector3.zero;
            Plugin.Log.LogInfo($"Expansion picker: {byIndex.Count} buttons (indices {string.Join(",", byIndex.Keys.OrderBy(k => k))}), " +
                               $"rows under '{refButton.parent?.name}' ({string.Join(", ", vanilla.Select(r => r.name))}), " +
                               $"highlight {(refHighlight == null ? "none" : highlightInsideButton ? "inside row" : $"under '{refHighlight.parent?.name}'")}");

            var extra = new List<RectTransform>();
            if (refHighlight != null && !highlightInsideButton)
                extra.AddRange(screen.m_BtnHighlightList.Where(h => h != null).Select(h => (RectTransform)h.transform));

            _grid = ButtonGrid.Build(vanilla, Registry.Sets.Count, (k, clone) =>
            {
                var set = Registry.Sets[k];
                clone.name = $"TCGCC_PickerButton_{set.Def.Id}";
                ButtonGrid.SetLabel(clone, set.Def.Name);
                int expansion = (int)set.Expansion;
                ButtonGrid.SetOnClick(clone, () => screen.OnPressButton(expansion));

                GameObject highlight = null;
                if (highlightInsideButton)
                {
                    highlight = clone.Find(highlightPath)?.gameObject;
                }
                else if (refHighlight != null)
                {
                    var h = (RectTransform)Object.Instantiate(refHighlight.gameObject, refHighlight.parent, false).transform;
                    h.name = $"TCGCC_PickerHighlight_{set.Def.Id}";
                    if (refHighlight.parent == clone.parent) h.localPosition = clone.localPosition + highlightOffset;
                    else h.position = clone.position + (refHighlight.position - refButton.position);
                    extra.Add(h);
                    highlight = h.gameObject;
                }
                if (highlight != null) highlight.SetActive(false);
                Custom[expansion] = (clone, highlight);
            }, extra, manualWheel: true);
        }

        /// <summary>Maps each button to its row: the ancestor directly under the deepest common ancestor of all buttons.</summary>
        private static Dictionary<int, RectTransform> ToRows(Dictionary<int, Button> buttons)
        {
            var chains = buttons.ToDictionary(kv => kv.Key, kv =>
            {
                var chain = new List<Transform>();
                for (var t = kv.Value.transform; t != null; t = t.parent) chain.Insert(0, t);
                return chain; // root … button
            });
            var first = chains.Values.First();
            int depth = 0; // number of shared ancestors
            while (depth < first.Count && chains.Values.All(c => c.Count > depth && c[depth] == first[depth])) depth++;
            if (buttons.Count == 1) depth = first.Count - 1;
            return chains.ToDictionary(kv => kv.Key, kv => (RectTransform)kv.Value[Mathf.Min(depth, kv.Value.Count - 1)]);
        }

        /// <summary>Buttons under the screen whose persistent onClick calls OnPressButton(int), keyed by that int.</summary>
        private static Dictionary<int, Button> FindButtons(CardExpansionSelectScreen screen)
        {
            var result = new Dictionary<int, Button>();
            var root = screen.m_ScreenGrp != null ? screen.m_ScreenGrp.transform : screen.transform;
            foreach (var b in root.GetComponentsInChildren<Button>(true))
            {
                var calls = Traverse.Create(b.onClick).Field("m_PersistentCalls").Field("m_Calls").GetValue() as IList;
                if (calls == null) continue;
                foreach (var call in calls)
                {
                    var t = Traverse.Create(call);
                    if (t.Field("m_MethodName").GetValue<string>() != nameof(CardExpansionSelectScreen.OnPressButton)) continue;
                    int arg = t.Field("m_Arguments").Field("m_IntArgument").GetValue<int>();
                    if (!result.ContainsKey(arg)) result[arg] = b;
                }
            }
            return result;
        }

        internal static string RelativePath(Transform root, Transform t)
        {
            var parts = new List<string>();
            for (var cur = t; cur != null && cur != root; cur = cur.parent) parts.Insert(0, cur.name);
            return string.Join("/", parts);
        }
    }

    /// <summary>Phone list screens read the raw mouse wheel every frame; ignore it while a modal WheelScroller list is open.</summary>
    [HarmonyPatch(typeof(GenericSliderScreen), "EvaluateScreenDrag")]
    internal static class SliderIgnoreWheelUnderPopup
    {
        private static bool Prefix() => WheelScroller.ActiveCount == 0;
    }
}

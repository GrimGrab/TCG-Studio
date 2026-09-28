using System.Collections.Generic;
using System.Linq;
using HarmonyLib;
using TCGCustomCards.Runtime;
using TCGCustomCards.UI;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.Patches
{
    // Layer 6 — binder expansion buttons: vanilla + one clone per custom set in a scrollable grid (UI\ButtonGrid);
    // graded-cards button pushed away with a divider; highlight follows the selected button while scrolling.

    [HarmonyPatch(typeof(CollectionBinderUI), "Awake")]
    internal static class BinderAddSetButtons
    {
        /// <summary>Custom expansion int → index of its button in m_ExpansionBtnList.</summary>
        internal static readonly Dictionary<int, int> ButtonIndexByExpansion = new Dictionary<int, int>();
        private static ButtonGrid _grid;
        private static Transform _highlight;
        private static Transform _selected;

        private static void Postfix(CollectionBinderUI __instance)
        {
            ButtonIndexByExpansion.Clear();
            _grid = null;
            var buttons = __instance.m_ExpansionBtnList;
            if (Registry.Sets.Count == 0 || buttons == null || buttons.Count == 0) return;

            var vanilla = buttons.Cast<RectTransform>().ToList();
            _grid = ButtonGrid.Build(vanilla, Registry.Sets.Count, (k, clone) =>
            {
                var set = Registry.Sets[k];
                clone.name = $"TCGCC_SetButton_{set.Def.Id}";
                ButtonGrid.SetLabel(clone, set.Def.Name);
                int expansion = (int)set.Expansion;
                ButtonGrid.SetOnClick(clone, () => __instance.OnPressSwitchExpansion(expansion));
                ButtonIndexByExpansion[expansion] = buttons.Count;
                buttons.Add(clone);
            });

            _highlight = __instance.m_ExpansionSelectHighlightBtn != null ? __instance.m_ExpansionSelectHighlightBtn.transform : null;
            _grid.Scroll.onValueChanged.AddListener(_ => FollowSelected());
            SeparateGradedButton(_grid, __instance.m_GradedCardBtn);
        }

        /// <summary>After vanilla positions the highlight: scroll the selected button into view and keep the highlight on it.</summary>
        internal static void OnSelectScreenOpened(CollectionBinderUI ui, int buttonIndex, bool isGraded)
        {
            _selected = null;
            if (_grid == null || isGraded || buttonIndex < 0 || buttonIndex >= ui.m_ExpansionBtnList.Count) return;
            var item = ui.m_ExpansionBtnList[buttonIndex] as RectTransform;
            _grid.ScrollTo(item);
            _selected = item;
            FollowSelected();
        }

        private static void FollowSelected()
        {
            if (_selected != null && _highlight != null) _highlight.position = _selected.position;
        }

        /// <summary>Pushes the graded-cards button away from the set list and draws a divider line between them.</summary>
        private static void SeparateGradedButton(ButtonGrid grid, Transform graded)
        {
            if (graded == null) return;
            var parent = grid.Parent;
            var area = grid.Area;
            float gap = grid.RowStep * 0.4f;
            Vector3 g = parent.InverseTransformPoint(graded.position);
            float gH = graded is RectTransform grt ? grt.rect.height * grt.lossyScale.y / parent.lossyScale.y : grid.ButtonHeight;

            float lineY;
            if (g.y <= area.min.y) { g.y -= gap; lineY = (area.min.y + g.y + gH / 2f) / 2f; }
            else { g.y += gap; lineY = (area.max.y + g.y - gH / 2f) / 2f; }
            graded.position = parent.TransformPoint(g);

            var line = new GameObject("TCGCC_ExpansionSeparator", typeof(RectTransform), typeof(Image));
            line.layer = parent.gameObject.layer;
            var rt = (RectTransform)line.transform;
            rt.SetParent(parent, false);
            rt.anchorMin = rt.anchorMax = rt.pivot = new Vector2(0.5f, 0.5f);
            rt.sizeDelta = new Vector2(area.size.x * 0.9f, Mathf.Max(2f, grid.ButtonHeight * 0.05f));
            rt.localPosition = new Vector3(area.center.x, lineY, 0f);
            var img = line.GetComponent<Image>();
            img.color = new Color(1f, 1f, 1f, 0.35f);
            img.raycastTarget = false;
        }

        /// <summary>Hides vanilla set buttons while vanilla cards are hidden (and packs the rest together).</summary>
        internal static void RefreshVisibility() =>
            _grid?.Relayout(item => !VanillaFilter.HideCards || item.name.StartsWith("TCGCC_"));

        /// <summary>Maps a custom expansion int to its button index so highlight code doesn't index out of range.</summary>
        internal static void MapIndex(ref int currentExpansionIndex)
        {
            RefreshVisibility();
            if (currentExpansionIndex < Registry.ExpansionBase) return;
            currentExpansionIndex = ButtonIndexByExpansion.TryGetValue(currentExpansionIndex, out int idx) ? idx : 0;
        }
    }

    [HarmonyPatch(typeof(CollectionBinderUI), nameof(CollectionBinderUI.OpenExpansionSelectScreen))]
    internal static class BinderExpansionSelectIndexFix
    {
        private static void Prefix(ref int currentExpansionIndex) => BinderAddSetButtons.MapIndex(ref currentExpansionIndex);

        private static void Postfix(CollectionBinderUI __instance, int currentExpansionIndex, bool isGradedCardAlbum) =>
            BinderAddSetButtons.OnSelectScreenOpened(__instance, currentExpansionIndex, isGradedCardAlbum);
    }

    [HarmonyPatch(typeof(CollectionBinderUI), nameof(CollectionBinderUI.OpenSortAlbumScreen))]
    internal static class BinderSortScreenIndexFix
    {
        private static void Prefix(ref int currentExpansionIndex) => BinderAddSetButtons.MapIndex(ref currentExpansionIndex);

        private static void Postfix(CollectionBinderUI __instance, int currentExpansionIndex, bool isGradedCardAlbum) =>
            BinderAddSetButtons.OnSelectScreenOpened(__instance, currentExpansionIndex, isGradedCardAlbum);
    }
}

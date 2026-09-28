using System;
using System.Collections.Generic;
using System.Linq;
using TMPro;
using UnityEngine;
using UnityEngine.UI;
using Object = UnityEngine.Object;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// Extends a hand-placed grid of vanilla buttons with cloned buttons (continuing the grid) and wraps them all in a
    /// vertical ScrollRect + scrollbar over the original area. Used by the binder set list and the card-expansion picker.
    /// </summary>
    internal class ButtonGrid
    {
        public RectTransform Parent;
        public ScrollRect Scroll;
        public Bounds Area;
        public float RowStep;
        public float ButtonHeight;
        public readonly List<RectTransform> Clones = new List<RectTransform>();
        /// <summary>All grid items in order (vanilla then clones) and whether each was active when the grid was built.</summary>
        private readonly List<RectTransform> _items = new List<RectTransform>();
        private readonly List<bool> _initiallyActive = new List<bool>();
        /// <summary>Content-space positions of every visible item at build time, sorted top-to-bottom, left-to-right.</summary>
        private List<Vector3> _slots = new List<Vector3>();

        /// <summary>
        /// Shows only items for which <paramref name="visible"/> is true (items that were inactive at build time stay hidden)
        /// and packs them into the grid's slots in order, so hidden rows leave no gaps.
        /// </summary>
        public void Relayout(Func<RectTransform, bool> visible)
        {
            if (Scroll == null) return;
            int slot = 0;
            var shown = new List<RectTransform>();
            for (int i = 0; i < _items.Count; i++)
            {
                var item = _items[i];
                if (item == null) continue;
                bool show = _initiallyActive[i] && visible(item);
                item.gameObject.SetActive(show);
                if (!show || slot >= _slots.Count) continue;
                item.localPosition = _slots[slot++];
                shown.Add(item);
            }
            // The content's origin is its top edge (pivot y = 1), but the slots come from the vanilla layout and can sit above it
            // (the binder's top buttons were ~315 units above) — anything there can never be scrolled to. Shift everything so the
            // first row starts at the top, keeping the spacing.
            float lowest = 0f;
            if (shown.Count > 0)
            {
                float top = shown.Max(s => s.localPosition.y + HalfHeight(s));
                foreach (var s in shown) s.localPosition += new Vector3(0f, -top, 0f);
                lowest = shown.Min(s => s.localPosition.y - HalfHeight(s));
            }
            var content = Scroll.content;
            content.sizeDelta = new Vector2(content.sizeDelta.x, Mathf.Max(Scroll.viewport.rect.height, -lowest));
            content.anchoredPosition = new Vector2(content.anchoredPosition.x, Mathf.Clamp(content.anchoredPosition.y, 0f,
                Mathf.Max(0f, content.sizeDelta.y - Scroll.viewport.rect.height)));
        }

        private static float HalfHeight(RectTransform r) => r.rect.height * r.localScale.y / 2f;

        /// <summary>Creates the grid. <paramref name="makeClone"/> is called per new button with the clone to customise.</summary>
        /// <param name="extraContent">Other objects that live beside the buttons (e.g. highlight frames) and must scroll with them;
        /// may be appended to by <paramref name="makeClone"/>. Only items sharing the buttons' parent are moved.</param>
        /// <param name="manualWheel">Scroll with Input.mouseScrollDelta whenever the list is visible (for UIs that don't route
        /// scroll events through the EventSystem, e.g. the phone screens).</param>
        public static ButtonGrid Build(IList<RectTransform> vanilla, int count, Action<int, RectTransform> makeClone, List<RectTransform> extraContent = null, bool manualWheel = false)
        {
            var grid = new ButtonGrid();
            var parent = vanilla[0].parent as RectTransform;
            var siblings = vanilla.Where(b => b != null && b.parent == parent).ToList();
            if (parent == null || siblings.Count != vanilla.Count)
                Plugin.Log.LogWarning($"ButtonGrid: buttons do not share one RectTransform parent ({siblings.Count}/{vanilla.Count}); layout may be off");
            var active = siblings.Where(b => b.gameObject.activeSelf).ToList();
            if (active.Count == 0) active = siblings;

            var xs = Distinct(active.Select(b => b.localPosition.x));
            xs.Sort();
            var ys = Distinct(active.Select(b => b.localPosition.y)).OrderByDescending(y => y).ToList();
            grid.Parent = parent;
            grid.ButtonHeight = active[0].rect.height * active[0].localScale.y;
            grid.RowStep = ys.Count > 1 ? Mathf.Abs(ys[0] - ys[1]) : grid.ButtonHeight * 1.15f;
            grid.Area = BoundsInParent(parent, active);

            var template = active[0];
            for (int k = 0; k < count; k++)
            {
                var clone = (RectTransform)Object.Instantiate(template.gameObject, parent, worldPositionStays: false).transform;
                clone.gameObject.SetActive(true);
                int col = k % xs.Count, row = ys.Count + k / xs.Count;
                clone.localPosition = new Vector3(xs[col], ys[0] - row * grid.RowStep, template.localPosition.z);
                foreach (var loc in clone.GetComponentsInChildren<I2.Loc.Localize>(true)) Object.Destroy(loc);
                makeClone(k, clone);
                grid.Clones.Add(clone);
            }

            float extraRows = Mathf.Ceil(count / (float)xs.Count);
            var content = siblings.Concat(grid.Clones);
            if (extraContent != null) content = content.Concat(extraContent.Where(e => e != null && e.parent == parent));
            grid.Scroll = BuildScrollView(parent, content.ToList(), grid.Area, grid.Area.size.y + extraRows * grid.RowStep, grid.RowStep);
            foreach (var item in siblings.Concat(grid.Clones))
            {
                grid._items.Add(item);
                grid._initiallyActive.Add(item.gameObject.activeSelf);
            }
            grid._slots = grid._items.Where(i => i.gameObject.activeSelf).Select(i => i.localPosition)
                .OrderByDescending(p => Mathf.Round(p.y)).ThenBy(p => p.x).ToList();
            AddScrollbar(parent, grid.Scroll, grid.Area, grid.ButtonHeight);
            if (manualWheel)
            {
                grid.Scroll.scrollSensitivity = 0f;
                grid.Scroll.gameObject.AddComponent<WheelScroller>().Init(grid.Scroll, grid.RowStep);
            }
            Plugin.Log.LogInfo($"ButtonGrid '{parent.name}': {siblings.Count} vanilla + {count} custom, grid {xs.Count}×{ys.Count}, rowStep={grid.RowStep:F1}, area={grid.Area.size}");
            return grid;
        }

        public static void SetLabel(RectTransform button, string text)
        {
            foreach (var t in button.GetComponentsInChildren<TextMeshProUGUI>(true)) t.text = text;
        }

        public static void SetOnClick(RectTransform button, Action onClick)
        {
            var b = button.GetComponentInChildren<Button>(true);
            if (b == null) { Plugin.Log.LogWarning($"ButtonGrid: '{button.name}' has no Button component"); return; }
            b.onClick = new Button.ButtonClickedEvent();
            b.onClick.AddListener(() => onClick());
        }

        /// <summary>Scrolls so that <paramref name="item"/> (a child of the scroll content) is fully visible.</summary>
        public void ScrollTo(RectTransform item)
        {
            if (Scroll == null || item == null || item.parent != Scroll.content) return;
            var content = Scroll.content;
            float viewH = Scroll.viewport.rect.height, contentH = content.rect.height;
            float center = -item.localPosition.y, h = item.rect.height * item.localScale.y;
            float offset = content.anchoredPosition.y;
            if (center - h / 2f < offset || center + h / 2f > offset + viewH)
                offset = Mathf.Clamp(center - viewH / 2f, 0f, Mathf.Max(0f, contentH - viewH));
            content.anchoredPosition = new Vector2(content.anchoredPosition.x, offset);
        }

        private static ScrollRect BuildScrollView(RectTransform parent, List<RectTransform> items, Bounds area, float contentHeight, float rowStep)
        {
            int firstSibling = items.Min(i => i.GetSiblingIndex());
            var viewportGo = new GameObject("TCGCC_ScrollView", typeof(RectTransform), typeof(Image), typeof(RectMask2D), typeof(ScrollRect));
            viewportGo.layer = parent.gameObject.layer;
            var viewport = (RectTransform)viewportGo.transform;
            viewport.SetParent(parent, false);
            viewport.anchorMin = viewport.anchorMax = viewport.pivot = new Vector2(0.5f, 0.5f);
            viewport.sizeDelta = area.size;
            viewport.localPosition = area.center;
            viewport.SetSiblingIndex(firstSibling);
            viewportGo.GetComponent<Image>().color = new Color(0, 0, 0, 0); // invisible, catches mouse-wheel events between buttons

            var contentGo = new GameObject("Content", typeof(RectTransform));
            contentGo.layer = parent.gameObject.layer;
            var content = (RectTransform)contentGo.transform;
            content.SetParent(viewport, false);
            content.anchorMin = content.anchorMax = content.pivot = new Vector2(0.5f, 1f);
            content.anchoredPosition = Vector2.zero;
            content.sizeDelta = new Vector2(area.size.x, contentHeight);
            foreach (var item in items)
            {
                item.SetParent(content, worldPositionStays: true);
                // Anchor to the content's top edge (its pivot). Items anchored to the centre would be dragged by half of every
                // content height change — Relayout resizes the content, which pushed the top rows out of the scrollable area.
                var pos = item.localPosition;
                item.anchorMin = item.anchorMax = new Vector2(0.5f, 1f);
                item.localPosition = pos;
            }

            var scroll = viewportGo.GetComponent<ScrollRect>();
            scroll.viewport = viewport;
            scroll.content = content;
            scroll.horizontal = false;
            scroll.vertical = true;
            scroll.movementType = ScrollRect.MovementType.Clamped;
            scroll.inertia = true;
            scroll.scrollSensitivity = rowStep * 0.5f;
            return scroll;
        }

        private static void AddScrollbar(RectTransform parent, ScrollRect scroll, Bounds area, float btnH)
        {
            float w = Mathf.Max(6f, btnH * 0.12f);
            var go = new GameObject("TCGCC_Scrollbar", typeof(RectTransform), typeof(Image), typeof(Scrollbar));
            go.layer = parent.gameObject.layer;
            var rt = (RectTransform)go.transform;
            rt.SetParent(parent, false);
            rt.anchorMin = rt.anchorMax = rt.pivot = new Vector2(0.5f, 0.5f);
            rt.sizeDelta = new Vector2(w, area.size.y);
            rt.localPosition = new Vector3(area.max.x + w * 1.5f, area.center.y, 0f);
            go.GetComponent<Image>().color = new Color(1f, 1f, 1f, 0.15f);

            var slide = (RectTransform)new GameObject("Sliding Area", typeof(RectTransform)).transform;
            slide.gameObject.layer = go.layer;
            slide.SetParent(rt, false);
            slide.anchorMin = Vector2.zero; slide.anchorMax = Vector2.one; slide.offsetMin = slide.offsetMax = Vector2.zero;

            var handleGo = new GameObject("Handle", typeof(RectTransform), typeof(Image));
            handleGo.layer = go.layer;
            var handle = (RectTransform)handleGo.transform;
            handle.SetParent(slide, false);
            handle.offsetMin = handle.offsetMax = Vector2.zero;
            var handleImg = handleGo.GetComponent<Image>();
            handleImg.color = new Color(1f, 1f, 1f, 0.65f);

            var sb = go.GetComponent<Scrollbar>();
            sb.handleRect = handle;
            sb.targetGraphic = handleImg;
            sb.direction = Scrollbar.Direction.BottomToTop;
            scroll.verticalScrollbar = sb;
            scroll.verticalScrollbarVisibility = ScrollRect.ScrollbarVisibility.AutoHide;
        }

        internal static Bounds BoundsInParent(RectTransform parent, IEnumerable<RectTransform> items)
        {
            var corners = new Vector3[4];
            var b = new Bounds();
            bool first = true;
            foreach (var it in items)
            {
                it.GetWorldCorners(corners);
                foreach (var c in corners)
                {
                    var p = parent.InverseTransformPoint(c);
                    p.z = 0f;
                    if (first) { b = new Bounds(p, Vector3.zero); first = false; }
                    else b.Encapsulate(p);
                }
            }
            return b;
        }

        private static List<float> Distinct(IEnumerable<float> values)
        {
            var result = new List<float>();
            foreach (var v in values)
                if (!result.Any(r => Mathf.Abs(r - v) < 2f)) result.Add(v);
            return result;
        }
    }
}

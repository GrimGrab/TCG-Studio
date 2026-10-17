using System.Collections.Generic;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.Binder
{
    /// <summary>
    /// The small squares beside a binder card's count ("X 3 ▪▪▪") in the one-slot-per-card layout: one per border × foil
    /// version (top row normal, bottom row foil), lit in the border's colour when owned, dark when not, the shown version
    /// bigger with an outline. Built once per binder slot as a child of its count text, sized from that text.
    /// </summary>
    internal static class VersionPips
    {
        // Base, First edition, Silver, Gold, EX, Full art.
        private static readonly Color[] BorderColors =
        {
            new Color(0.92f, 0.92f, 0.92f), new Color(0.35f, 0.75f, 1f), new Color(0.72f, 0.76f, 0.84f),
            new Color(1f, 0.78f, 0.15f), new Color(0.78f, 0.42f, 1f), new Color(1f, 0.42f, 0.62f),
        };
        private static readonly Color Missing = new Color(0.08f, 0.08f, 0.08f, 0.55f);

        private sealed class Pips
        {
            public RectTransform Root;
            public Image[] Squares;
            public Outline[] Outlines;
            public float Gap;
            /// <summary>Red heart left of the count: a favourite card.</summary>
            public RectTransform Fav;
        }

        // Keyed by the slot; slots die with the scene (loading a save rebuilds "Start"): dead keys are dropped on use.
        private static readonly Dictionary<Card3dUIGroup, Pips> BySlot = new Dictionary<Card3dUIGroup, Pips>();

        public static void Hide(Card3dUIGroup slot)
        {
            if (slot == null || !BySlot.TryGetValue(slot, out var p) || p.Root == null) return;
            p.Root.gameObject.SetActive(false);
            if (p.Fav != null) p.Fav.gameObject.SetActive(false);
        }

        /// <summary>A version's square: its border's colour when owned, dark when not (also drawn by the MTG deck builder).</summary>
        public static Color ColorOf(BinderEntry e, int v) =>
            e.Count(v) > 0 ? BorderColors[(v % (e.Per / 2)) % BorderColors.Length] : Missing;

        public static void Show(Card3dUIGroup slot, BinderEntry e, int shown)
        {
            var p = Get(slot, e.Per);
            if (p == null) return;
            for (int v = 0; v < p.Squares.Length; v++)
            {
                var sq = p.Squares[v];
                bool listed = v < e.Per && e.Listed[v];
                sq.gameObject.SetActive(listed);
                if (!listed) continue;
                sq.color = ColorOf(e, v);
                bool isShown = v == shown;
                sq.rectTransform.localScale = Vector3.one * (isShown ? 1.3f : 1f);
                p.Outlines[v].enabled = isShown;
            }
            // On the count's line, just right of the drawn text (its width changes with the number).
            var text = slot.m_CardCountText;
            text.ForceMeshUpdate();
            var b = text.textBounds;
            p.Root.anchoredPosition = new Vector2(b.max.x + p.Gap, b.center.y);
            p.Root.gameObject.SetActive(true);
            PlaceHeart(p, e, b);
        }

        /// <summary>Every-version-separately layout: no squares, only the favourite heart.</summary>
        public static void ShowFavourite(Card3dUIGroup slot, BinderEntry e)
        {
            var p = Get(slot, e.Per);
            if (p == null) return;
            p.Root.gameObject.SetActive(false);
            var text = slot.m_CardCountText;
            text.ForceMeshUpdate();
            PlaceHeart(p, e, text.textBounds);
        }

        private static void PlaceHeart(Pips p, BinderEntry e, Bounds b)
        {
            bool fav = Favourites.Has(e);
            p.Fav.gameObject.SetActive(fav);
            if (fav) p.Fav.anchoredPosition = new Vector2(b.min.x - p.Gap - p.Fav.sizeDelta.x * 0.5f, b.center.y);
        }

        private static Pips Get(Card3dUIGroup slot, int per)
        {
            if (slot == null || slot.m_CardCountText == null) return null;
            if (BySlot.TryGetValue(slot, out var p) && p.Root != null && p.Squares.Length == per) return p;
            if (p?.Root != null) Object.Destroy(p.Root.gameObject);
            if (p?.Fav != null) Object.Destroy(p.Fav.gameObject);
            CleanDead();

            var text = slot.m_CardCountText.rectTransform;
            float fontSize = slot.m_CardCountText.fontSize;
            float sq = Mathf.Max(1f, fontSize * 0.42f);
            float gap = sq * 0.35f;
            int cols = Mathf.Max(1, per / 2);

            // Child of the count text, anchored at its pivot (= the origin of TMP's textBounds): placed right of the text in Show.
            var go = new GameObject("TCGCC_VersionPips", typeof(RectTransform));
            go.layer = text.gameObject.layer;
            var root = go.GetComponent<RectTransform>();
            root.SetParent(text, false);
            root.anchorMin = root.anchorMax = text.pivot;
            root.pivot = new Vector2(0f, 0.5f);
            root.sizeDelta = new Vector2(cols * sq + (cols - 1) * gap, 2 * sq + gap);

            p = new Pips { Root = root, Squares = new Image[per], Outlines = new Outline[per], Gap = gap * 2f };
            for (int v = 0; v < per; v++)
            {
                int col = v % cols, row = v / cols; // row 0 = normal, row 1 = foil
                var cell = new GameObject("v" + v, typeof(RectTransform), typeof(Image), typeof(Outline));
                cell.layer = go.layer;
                var rt = cell.GetComponent<RectTransform>();
                rt.SetParent(root, false);
                rt.anchorMin = rt.anchorMax = new Vector2(0f, 1f);
                rt.pivot = new Vector2(0.5f, 0.5f);
                rt.sizeDelta = new Vector2(sq, sq);
                rt.anchoredPosition = new Vector2(col * (sq + gap) + sq / 2f, -(row * (sq + gap) + sq / 2f));
                var img = cell.GetComponent<Image>();
                img.raycastTarget = false;
                var outline = cell.GetComponent<Outline>();
                outline.effectColor = Color.white;
                outline.effectDistance = new Vector2(sq * 0.18f, sq * 0.18f);
                outline.enabled = false;
                p.Squares[v] = img;
                p.Outlines[v] = outline;
            }
            // Favourite mark: own child of the count text (the squares' root sits right of the text).
            var fav = new GameObject("TCGCC_Favourite", typeof(RectTransform), typeof(Image));
            fav.layer = go.layer;
            var frt = fav.GetComponent<RectTransform>();
            frt.SetParent(text, false);
            frt.anchorMin = frt.anchorMax = text.pivot;
            frt.pivot = new Vector2(0.5f, 0.5f);
            frt.sizeDelta = new Vector2(sq * 2.2f, sq * 2.2f);
            var fimg = fav.GetComponent<Image>();
            fimg.sprite = Heart();
            fimg.color = new Color(0.92f, 0.12f, 0.2f);
            fimg.raycastTarget = false;
            fav.SetActive(false);
            p.Fav = frt;
            BySlot[slot] = p;
            return p;
        }

        private static Sprite _heart;

        /// <summary>A heart drawn once (the game has no heart sprite): (x²+y²−1)³ − x²y³ ≤ 0, 4×4 supersampled edges.</summary>
        internal static Sprite Heart()
        {
            if (_heart != null) return _heart;
            const int size = 64, ss = 4;
            var tex = new Texture2D(size, size, TextureFormat.RGBA32, false) { wrapMode = TextureWrapMode.Clamp, filterMode = FilterMode.Bilinear };
            for (int py = 0; py < size; py++)
            for (int px = 0; px < size; px++)
            {
                int hits = 0;
                for (int sy = 0; sy < ss; sy++)
                for (int sx = 0; sx < ss; sx++)
                {
                    float x = ((px + (sx + 0.5f) / ss) / size - 0.5f) * 2.6f;
                    float y = ((py + (sy + 0.5f) / ss) / size - 0.5f) * 2.6f + 0.15f;
                    float a = x * x + y * y - 1f;
                    if (a * a * a - x * x * y * y * y <= 0f) hits++;
                }
                tex.SetPixel(px, py, new Color(1f, 1f, 1f, hits / (float)(ss * ss)));
            }
            tex.Apply();
            _heart = Sprite.Create(tex, new UnityEngine.Rect(0, 0, size, size), new Vector2(0.5f, 0.5f), 100f);
            _heart.name = "TCGCC_Heart";
            return _heart;
        }

        private static void CleanDead()
        {
            List<Card3dUIGroup> dead = null;
            foreach (var kv in BySlot)
                if (kv.Key == null || kv.Value.Root == null) (dead ?? (dead = new List<Card3dUIGroup>())).Add(kv.Key);
            if (dead != null) foreach (var k in dead) BySlot.Remove(k);
        }
    }
}

using System.Collections.Generic;
using System.Linq;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// The one place that says what our on-screen panels look like: they take the look of the game's own task panel (TutorialManager): its frame colour and its text.
    /// The background is drawn here, because the game's pictures have see-through margins of their own and don't fill the box they're put in. Screens ask for a canvas, a panel, rows and text here and don't set colours, fonts or sizes of their own, so a change here reaches all of them.
    /// </summary>
    internal static class GameStyle
    {
        public enum TextKind { Title, Body, Small }

        /// <summary>A word that has to stand out on these panels, as rich text.</summary>
        public static string Warning(string text) => $"<color=#FF6A5A><b>{text}</b></color>";

        private const float Padding = 26f;  // text from the box's edge
        private const float FrameThickness = 7f, BarHeight = 12f;
        private static readonly Color BarFull = new Color(0.36f, 0.85f, 0.42f), BarEmpty = new Color(0.93f, 0.27f, 0.22f), BarProgress = new Color(1f, 0.78f, 0.28f);
        private const int CornerRadius = 22;
        private static readonly Color InsideColor = new Color(0.27f, 0.27f, 0.29f, 0.94f);
        private const float Spacing = 8f, RowGap = 14f, LineGap = 3f;

        private static float SizeOf(TextKind kind) => kind == TextKind.Title ? 24f : kind == TextKind.Body ? 17f : 15f;

        private static TutorialManager _readFrom;
        private static Color _frameColor = new Color(0.22f, 0.16f, 0.26f);
        private static readonly Dictionary<int, Sprite> Corners = new Dictionary<int, Sprite>();
        private static TMP_FontAsset _font;
        private static Material _fontMaterial;
        private static Color _textColor = Color.white;

        /// <summary>The look, read once per shop scene. Says in the log what it found: a game update may change it.</summary>
        private static void Read()
        {
            var manager = Core.GameScene.Existing<TutorialManager>();
            if (manager != null && manager == _readFrom) return;
            _readFrom = manager;
            _font = null;
            _fontMaterial = null;
            var root = manager?.m_TutorialSubGroupList?.FirstOrDefault(g => g != null && g.m_ScreenGrp != null)?.m_ScreenGrp;
            var from = root != null ? root.transform : manager?.m_CanvasGrp != null ? manager.m_CanvasGrp.transform : null;
            if (from == null)
            {
                Plugin.Log.LogWarning("Game style: the game's task panel wasn't found; plain colours");
                return;
            }
            var frame = from.GetComponentsInParent<Image>(true).Concat(from.GetComponentsInChildren<Image>(true))
                .Where(i => i.sprite != null && i.type == Image.Type.Sliced)
                .OrderByDescending(i => i.rectTransform.rect.width * i.rectTransform.rect.height).FirstOrDefault();
            if (frame != null)
            {
                _frameColor = frame.color;
                _frameColor.a = 1f;
            }
            var text = from.GetComponentsInChildren<TextMeshProUGUI>(true).FirstOrDefault(t => t.font != null);
            if (text != null)
            {
                _font = text.font;
                _fontMaterial = text.fontSharedMaterial;
                _textColor = text.color;
                _textColor.a = 1f;
            }
            Plugin.Log.LogInfo($"Game style: frame #{ColorUtility.ToHtmlStringRGB(_frameColor)}; text {_font?.name ?? "default"} #{ColorUtility.ToHtmlStringRGB(_textColor)}");
        }

        /// <summary>A canvas of our own below the game's screens, scaled like them. It's in the current scene, so a load removes it.</summary>
        public static GameObject Canvas(string name)
        {
            var go = new GameObject(name);
            var canvas = go.AddComponent<Canvas>();
            canvas.renderMode = RenderMode.ScreenSpaceOverlay;
            canvas.sortingOrder = -10;
            var scaler = go.AddComponent<CanvasScaler>();
            scaler.uiScaleMode = CanvasScaler.ScaleMode.ScaleWithScreenSize;
            scaler.referenceResolution = new Vector2(1920f, 1080f);
            scaler.matchWidthOrHeight = 1f;
            return go;
        }

        /// <summary>A box with the game's background. Add what goes in it, change its text, then call <see cref="GamePanel.Fit"/>: the box takes the size of its text.</summary>
        public static GamePanel Panel(Transform parent, string name, float width)
        {
            Read();
            var go = new GameObject(name, typeof(RectTransform));
            var rt = (RectTransform)go.transform;
            rt.SetParent(parent, false);
            rt.sizeDelta = new Vector2(width, 0f);
            Background(rt, 0f, _frameColor);
            Background(rt, FrameThickness, InsideColor);
            return new GamePanel(rt, width, Padding);
        }

        /// <summary>A rounded rectangle exactly over the panel, less the inset: nothing in its picture is see-through except the corners.</summary>
        private static void Background(RectTransform panel, float inset, Color color)
        {
            var go = new GameObject("Background", typeof(RectTransform), typeof(Image));
            var rt = (RectTransform)go.transform;
            rt.SetParent(panel, false);
            rt.anchorMin = Vector2.zero;
            rt.anchorMax = Vector2.one;
            rt.offsetMin = new Vector2(inset, inset);
            rt.offsetMax = new Vector2(-inset, -inset);
            var image = go.GetComponent<Image>();
            image.sprite = Rounded(CornerRadius);
            image.type = Image.Type.Sliced;
            image.color = color;
            image.raycastTarget = false;
        }

        /// <summary>A white rounded rectangle to stretch (9-sliced), one for each corner size.</summary>
        public static Sprite Rounded(int radius)
        {
            if (Corners.TryGetValue(radius, out var made) && made != null) return made;
            int size = radius * 2 + 4;
            var tex = new Texture2D(size, size, TextureFormat.RGBA32, false) { name = "TCGCC_Rounded", wrapMode = TextureWrapMode.Clamp, hideFlags = HideFlags.DontUnloadUnusedAsset };
            var px = new Color32[size * size];
            for (int y = 0; y < size; y++)
                for (int x = 0; x < size; x++)
                {
                    float dx = Mathf.Max(radius - x - 0.5f, x + 0.5f - (size - radius), 0f);
                    float dy = Mathf.Max(radius - y - 0.5f, y + 0.5f - (size - radius), 0f);
                    float edge = radius - Mathf.Sqrt(dx * dx + dy * dy);
                    px[y * size + x] = new Color32(255, 255, 255, (byte)(Mathf.Clamp01(edge + 0.5f) * 255f));
                }
            tex.SetPixels32(px);
            tex.Apply(false);
            float border = radius + 1;
            var sprite = Sprite.Create(tex, new Rect(0f, 0f, size, size), new Vector2(0.5f, 0.5f), 100f, 0, SpriteMeshType.FullRect, new Vector4(border, border, border, border));
            sprite.hideFlags = HideFlags.DontUnloadUnusedAsset;
            return Corners[radius] = sprite;
        }

        /// <summary>Text in the game's look. Made on an active object: TextMeshPro's first Awake puts its defaults over anything set before it.</summary>
        private static TextMeshProUGUI Text(Transform parent, TextKind kind)
        {
            var go = new GameObject("Text", typeof(RectTransform));
            go.transform.SetParent(parent, false);
            var t = go.AddComponent<TextMeshProUGUI>();
            if (_font != null) t.font = _font;
            if (_fontMaterial != null) t.fontSharedMaterial = _fontMaterial;
            t.fontSize = SizeOf(kind);
            t.color = _textColor;
            t.enableWordWrapping = true;
            t.richText = true;
            t.raycastTarget = false;
            return t;
        }

        private static void Place(RectTransform r, float x, float y, float width, float height)
        {
            r.anchorMin = r.anchorMax = r.pivot = new Vector2(0f, 1f);
            r.anchoredPosition = new Vector2(x, -y);
            r.sizeDelta = new Vector2(width, height);
        }

        /// <summary>A bar from 0 to 1: green when full, red when nearly empty.</summary>
        public sealed class GameBar
        {
            private readonly TextMeshProUGUI _label;
            private readonly Image _back, _fill;

            public GameBar(TextMeshProUGUI label, Image back)
            {
                _label = label;
                _back = back;
                back.sprite = Rounded(CornerRadius);
                back.type = Image.Type.Sliced;
                back.color = new Color(0f, 0f, 0f, 0.45f);
                back.raycastTarget = false;
                var fill = new GameObject("Fill", typeof(RectTransform), typeof(Image));
                fill.transform.SetParent(back.transform, false);
                _fill = fill.GetComponent<Image>();
                _fill.sprite = Rounded(CornerRadius);
                _fill.type = Image.Type.Sliced;
                _fill.raycastTarget = false;
                _fill.rectTransform.anchorMin = Vector2.zero;
                _fill.rectTransform.offsetMin = _fill.rectTransform.offsetMax = Vector2.zero;
                Set(1f);
            }

            /// <summary>Call <see cref="GamePanel.Fit"/> after changing this.</summary>
            public bool Visible
            {
                get => _back.gameObject.activeSelf;
                set { _back.gameObject.SetActive(value); _label.gameObject.SetActive(value); }
            }

            /// <summary>Red to green by how full it is.</summary>
            public void Set(float value)
            {
                value = Mathf.Clamp01(value);
                Set(value, Color.Lerp(BarEmpty, BarFull, value));
            }

            private void Set(float value, Color color)
            {
                _fill.rectTransform.anchorMax = new Vector2(Mathf.Clamp01(value), 1f);
                _fill.color = color;
            }

            /// <summary>Progress towards something good: one colour all the way, or the warning colour.</summary>
            public void SetProgress(float value, bool bad) => Set(value, bad ? BarEmpty : BarProgress);

            public string Label { set => _label.text = value; }
        }

        /// <summary>A box of rows, top to bottom. A row is a text, or a picture with texts stacked beside it. Text that's empty or switched off takes no room.</summary>
        public sealed class GamePanel
        {
            private class Row
            {
                public Graphic Picture;
                public float PictureWidth, PictureHeight;
                public readonly List<TextMeshProUGUI> Texts = new List<TextMeshProUGUI>();
                public RectTransform Bar;
            }

            private readonly List<Row> _rows = new List<Row>();
            private readonly float _width, _padding;

            public RectTransform Root { get; }

            public GamePanel(RectTransform root, float width, float padding)
            {
                Root = root;
                _width = width;
                _padding = padding;
            }

            /// <summary>A new row starting with a picture; <see cref="TextBeside"/> adds to it.</summary>
            public Image PictureRow(float size)
            {
                var go = new GameObject("Picture", typeof(RectTransform), typeof(Image));
                go.transform.SetParent(Root, false);
                var image = go.GetComponent<Image>();
                image.preserveAspect = true;
                image.raycastTarget = false;
                _rows.Add(new Row { Picture = image, PictureWidth = size, PictureHeight = size });
                return image;
            }

            /// <summary>The same for a texture, like a rendered card. Switch it off when there's nothing to show.</summary>
            public RawImage TextureRow(float width, float height)
            {
                var go = new GameObject("Texture", typeof(RectTransform), typeof(RawImage));
                go.transform.SetParent(Root, false);
                var image = go.GetComponent<RawImage>();
                image.raycastTarget = false;
                _rows.Add(new Row { Picture = image, PictureWidth = width, PictureHeight = height });
                return image;
            }

            public TextMeshProUGUI TextBeside(TextKind kind)
            {
                var t = Text(Root, kind);
                _rows[_rows.Count - 1].Texts.Add(t);
                return t;
            }

            /// <summary>A new row the whole width of the box.</summary>
            public TextMeshProUGUI TextRow(TextKind kind)
            {
                var row = new Row();
                row.Texts.Add(Text(Root, kind));
                _rows.Add(row);
                return row.Texts[0];
            }

            /// <summary>A labelled bar the whole width of the box.</summary>
            public GameBar BarRow(string label, TextKind kind)
            {
                var row = new Row();
                var text = Text(Root, kind);
                text.text = label;
                row.Texts.Add(text);
                var back = new GameObject("Bar", typeof(RectTransform), typeof(Image));
                back.transform.SetParent(Root, false);
                row.Bar = (RectTransform)back.transform;
                _rows.Add(row);
                return new GameBar(text, back.GetComponent<Image>());
            }

            /// <summary>Measures the text and sizes the box around it.</summary>
            public void Fit()
            {
                float y = _padding;
                foreach (var row in _rows)
                {
                    float x = _padding, textHeight = 0f;
                    bool picture = row.Picture != null && row.Picture.gameObject.activeSelf;
                    if (picture) x += row.PictureWidth + RowGap;
                    float width = _width - _padding - x;
                    foreach (var t in row.Texts)
                    {
                        if (!t.gameObject.activeSelf || string.IsNullOrEmpty(t.text)) continue;
                        float height = t.GetPreferredValues(t.text, width, Mathf.Infinity).y;
                        if (textHeight > 0f) textHeight += LineGap;
                        Place(t.rectTransform, x, y + textHeight, width, height);
                        textHeight += height;
                    }
                    if (row.Bar != null && row.Bar.gameObject.activeSelf)
                    {
                        Place(row.Bar, x, y + textHeight + LineGap, width, BarHeight);
                        textHeight += LineGap + BarHeight;
                    }
                    float rowHeight = textHeight;
                    if (picture)
                    {
                        rowHeight = Mathf.Max(textHeight, row.PictureHeight);
                        Place(row.Picture.rectTransform, _padding, y, row.PictureWidth, row.PictureHeight);
                    }
                    if (rowHeight > 0f) y += rowHeight + Spacing;
                }
                Root.sizeDelta = new Vector2(_width, y - Spacing + _padding);
            }
        }

        /// <summary>A panel's place on screen in IMGUI's coordinates (y down).</summary>
        public static Rect ScreenRect(RectTransform panel)
        {
            var c = new Vector3[4];
            panel.GetWorldCorners(c);
            return new Rect(c[0].x, Screen.height - c[1].y, c[2].x - c[0].x, c[1].y - c[0].y);
        }
    }
}

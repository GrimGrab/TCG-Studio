using System.Collections.Generic;
using System.Linq;
using TMPro;
using UnityEngine;
using UnityEngine.UI;
using Object = UnityEngine.Object;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// Controller support for the binder panel. The game's own controller screens are a fixed grid of rows, which doesn't
    /// fit three scrolling columns and chip grids, so the panel moves a focus frame itself: d-pad / left stick go to the
    /// nearest button in that direction (hold to repeat, the game's timings), A presses it (the search box opens the game's
    /// on-screen keyboard), B closes, Y applies, X resets, LB / RB jump a page in the focused list. Buttons are read through
    /// the game's InputManager (rebinding, PS / Xbox). The vanilla sort screen's controller handling is switched off while
    /// the panel shows, or A would press its hidden buttons.
    /// </summary>
    internal static partial class BinderPanel
    {
        private const float RepeatStart = 0.5f, RepeatEvery = 0.08f; // ControllerScreenUIExtension's own timings

        private static Selectable _focus;
        private static Image _focusFrame;
        private static Sprite _ring;
        private static bool _focusFirst;
        /// <summary>After a press rebuilt a list: re-find the focus near this point of that list (next frame, once laid out).</summary>
        private static RectTransform _refocusIn;
        private static Vector2 _refocusAt;
        private static bool _refocus;
        /// <summary>Last frame something else (the on-screen keyboard) had the controller.</summary>
        private static int _navBlockedFrame = -10;
        private static Vector2Int _heldDir;
        private static float _holdTime;
        private static bool _repeating;
        private static GameObject _hintKeys, _hintPad;
        private static bool? _hintPadPS;

        private static ControllerScreenUIExtension VanillaExt => _ui != null ? _ui.m_SortAlbumScreenUIExtension : null;

        private static void NavReset()
        {
            _focus = null;
            _focusFrame = null;
            _refocus = false;
            _hintPadPS = null;
        }

        private static void NavOpened()
        {
            if (VanillaExt != null) VanillaExt.SetControllerUIActive(false);
            _focus = null;
            _focusFirst = true;
            _refocus = false;
            _heldDir = Vector2Int.zero;
        }

        private static void NavClosed()
        {
            if (VanillaExt != null) VanillaExt.SetControllerUIActive(true);
            _focus = null;
            ShowFrame(false);
        }

        /// <summary>Every frame while the panel shows.</summary>
        private static void NavUpdate()
        {
            bool pad = PadInput.Active;
            ShowHints(pad);
            if (!pad) { ShowFrame(false); return; }

            var mgr = CSingleton<ControllerScreenUIExtManager>.Instance;
            if (mgr == null || mgr.m_CurrentCtrlScreenUIExt != VanillaExt)
            {
                _navBlockedFrame = Time.frameCount; // the on-screen keyboard is on top
                ShowFrame(false);
                return;
            }

            if (_refocus && (_focus == null || !_focus.gameObject.activeInHierarchy)) Focus(Nearest(_refocusIn, _refocusAt));
            _refocus = false;
            if (_focusFirst || _focus == null || !_focus.gameObject.activeInHierarchy)
            {
                _focusFirst = false;
                Focus(Items().FirstOrDefault(s => s.transform.IsChildOf(_setList)) ?? Items().FirstOrDefault());
            }
            ShowFrame(_focus != null);
            // The press that closed the keyboard (or opened the panel) must not act here too.
            if (Time.frameCount - _navBlockedFrame <= 1) return;

            if (InputManager.GetKeyDownAction(EGameAction.MenuConfirm)) { Confirm(); return; }
            if (InputManager.GetKeyDownAction(EGameAction.MenuBack))
            {
                _swallow = true;
                Close(); // plays the game's menu-close sound
                return;
            }
            if (PadInput.Down(EGamepadControlBtn.Triangle)) { _swallow = true; Apply(); return; }
            if (PadInput.Down(EGamepadControlBtn.Square)) { Reset(); return; }
            if (PadInput.Down(EGamepadControlBtn.L1)) { PageMove(-1); return; }
            if (PadInput.Down(EGamepadControlBtn.R1)) { PageMove(1); return; }
            Directions();
        }

        // ------------------------------------------------------------------ input

        private static void Directions()
        {
            var dir = PadInput.Direction(_heldDir);
            if (dir == Vector2Int.zero) { _heldDir = dir; return; }
            if (dir != _heldDir)
            {
                _heldDir = dir;
                _holdTime = 0f;
                _repeating = false;
                Move(dir);
                return;
            }
            _holdTime += Time.unscaledDeltaTime;
            if (_holdTime < (_repeating ? RepeatEvery : RepeatStart)) return;
            _holdTime = 0f;
            _repeating = true;
            Move(dir);
        }

        private static void Confirm()
        {
            if (_focus == null) return;
            if (_focus is TMP_InputField field)
            {
                SoundManager.GenericConfirm(0.5f);
                ControllerScreenUIExtManager.StartVirtualKeyboard(field.text, field); // Done writes .text → numbers refresh
                _navBlockedFrame = Time.frameCount;
                return;
            }
            if (!(_focus is Button b)) return;
            _swallow = true; // Apply / Close / Graded close the panel inside this press
            _refocusIn = ListOf(_focus);
            _refocusAt = PointIn(_refocusIn, _focus);
            _refocus = true; // set / collection rows are rebuilt by a press
            b.onClick.Invoke();
        }

        private static void Move(Vector2Int dir)
        {
            if (_focus == null) return;
            Vector2 from = PointIn(null, _focus);
            var fromRect = RectIn(_focus);
            Selectable best = null;
            float bestScore = float.MaxValue;
            foreach (var s in Items())
            {
                if (s == _focus) continue;
                Vector2 d = PointIn(null, s) - from;
                float along = dir.x != 0 ? d.x * dir.x : d.y * dir.y;
                float across = dir.x != 0 ? Mathf.Abs(d.y) : Mathf.Abs(d.x);
                if (along < _u * 0.3f) continue; // not in that direction
                // Same row (left/right) or same column (up/down) first: from a collection row, left is its +/- button,
                // not the indented set row above it, whose centre is nearer.
                var r = RectIn(s);
                bool inLine = dir.x != 0
                    ? r.yMin < fromRect.yMax - 1f && r.yMax > fromRect.yMin + 1f
                    : r.xMin < fromRect.xMax - 1f && r.xMax > fromRect.xMin + 1f;
                float score = (inLine ? 0f : 100000f) + along + across * 2f;
                if (score < bestScore) { bestScore = score; best = s; }
            }
            if (best == null) return;
            SoundManager.GenericPop(0.5f);
            Focus(best);
        }

        /// <summary>LB / RB: about one screen of the focused list up or down.</summary>
        private static void PageMove(int sign)
        {
            var list = ListOf(_focus);
            if (list == null) return;
            var view = list.parent as RectTransform;
            float step = (view != null ? view.rect.height : _u * 10f) * 0.85f;
            var target = PointIn(list, _focus) + new Vector2(0f, -sign * step);
            var next = Nearest(list, target);
            if (next == null || next == _focus) return;
            SoundManager.GenericPop(0.5f);
            Focus(next);
        }

        // ------------------------------------------------------------------ focus

        /// <summary>Buttons and the search box that can take the focus (scrollbars can't).</summary>
        private static IEnumerable<Selectable> Items() =>
            _root == null ? Enumerable.Empty<Selectable>() :
            _root.GetComponentsInChildren<Selectable>(false).Where(s => s.interactable && (s is Button || s is TMP_InputField));

        private static void Focus(Selectable s)
        {
            _focus = s;
            if (s == null) return;
            ScrollIntoView(s);
            ShowFrame(PadInput.Active);
        }

        /// <summary>The scroll list content holding an item (null = not in a list).</summary>
        private static RectTransform ListOf(Selectable s) =>
            s == null ? null : _scrolls.Where(sc => sc != null && s.transform.IsChildOf(sc.content)).Select(sc => sc.content).FirstOrDefault();

        /// <summary>Centre of an item in a list's content space (or the panel's, for null).</summary>
        private static Vector2 PointIn(RectTransform space, Selectable s)
        {
            var rt = (RectTransform)s.transform;
            var world = rt.TransformPoint(rt.rect.center);
            return (space != null ? space : (RectTransform)_root.transform).InverseTransformPoint(world);
        }

        /// <summary>An item's rectangle in the panel's space.</summary>
        private static UnityEngine.Rect RectIn(Selectable s)
        {
            var corners = new Vector3[4];
            ((RectTransform)s.transform).GetWorldCorners(corners);
            var root = (RectTransform)_root.transform;
            Vector2 a = root.InverseTransformPoint(corners[0]), b = root.InverseTransformPoint(corners[2]);
            return UnityEngine.Rect.MinMaxRect(Mathf.Min(a.x, b.x), Mathf.Min(a.y, b.y), Mathf.Max(a.x, b.x), Mathf.Max(a.y, b.y));
        }

        private static Selectable Nearest(RectTransform list, Vector2 at)
        {
            var items = Items().Where(s => list == null ? ListOf(s) == null : s.transform.IsChildOf(list)).ToList();
            if (items.Count == 0) items = Items().ToList();
            return items.OrderBy(s => (PointIn(list, s) - at).sqrMagnitude).FirstOrDefault();
        }

        private static void ScrollIntoView(Selectable s)
        {
            var scroll = _scrolls.FirstOrDefault(sc => sc != null && s.transform.IsChildOf(sc.content));
            if (scroll == null) return;
            var content = scroll.content;
            var rt = (RectTransform)s.transform;
            var corners = new Vector3[4];
            rt.GetWorldCorners(corners);
            float top = -content.InverseTransformPoint(corners[1]).y;    // distance below the content's top
            float bottom = -content.InverseTransformPoint(corners[0]).y;
            float view = scroll.viewport.rect.height, y = content.anchoredPosition.y, margin = _u * 0.4f;
            if (top - margin < y) y = top - margin;
            else if (bottom + margin > y + view) y = bottom + margin - view;
            float max = Mathf.Max(0f, content.rect.height - view);
            content.anchoredPosition = new Vector2(content.anchoredPosition.x, Mathf.Clamp(y, 0f, max));
        }

        /// <summary>Outline around the focused item, a child of it so it scrolls and clips with its list.</summary>
        private static void ShowFrame(bool on)
        {
            if (!on || _focus == null)
            {
                if (_focusFrame != null) _focusFrame.gameObject.SetActive(false);
                return;
            }
            if (_focusFrame == null)
            {
                var go = new GameObject("TCGCC_Focus", typeof(RectTransform), typeof(Image), typeof(LayoutElement));
                go.GetComponent<LayoutElement>().ignoreLayout = true;
                _focusFrame = go.GetComponent<Image>();
                _focusFrame.raycastTarget = false;
                _focusFrame.sprite = Ring();
                _focusFrame.type = Image.Type.Sliced;
                _focusFrame.pixelsPerUnitMultiplier = 1.2f;
                _focusFrame.color = new Color(1f, 0.85f, 0.35f);
            }
            var frt = _focusFrame.rectTransform;
            if (frt.parent != _focus.transform)
            {
                frt.SetParent(_focus.transform, false);
                frt.gameObject.layer = _focus.gameObject.layer;
                frt.anchorMin = Vector2.zero; frt.anchorMax = Vector2.one;
                frt.offsetMin = new Vector2(-_u * 0.15f, -_u * 0.15f);
                frt.offsetMax = new Vector2(_u * 0.15f, _u * 0.15f);
            }
            frt.SetAsLastSibling();
            _focusFrame.gameObject.SetActive(true);
        }

        /// <summary>Rounded ring sprite (9-sliced) for the focus frame, made once like the panel's rounded fill.</summary>
        private static Sprite Ring()
        {
            if (_ring != null) return _ring;
            const int size = 32, r = 10;
            const float width = 3f;
            var tex = new Texture2D(size, size, TextureFormat.RGBA32, false) { wrapMode = TextureWrapMode.Clamp, filterMode = FilterMode.Bilinear };
            for (int y = 0; y < size; y++)
            for (int x = 0; x < size; x++)
            {
                // Signed distance to the rounded rectangle's edge (negative inside).
                float qx = Mathf.Max(r - x - 0.5f, x + 0.5f - (size - r)), qy = Mathf.Max(r - y - 0.5f, y + 0.5f - (size - r));
                float outside = Mathf.Sqrt(Mathf.Max(qx, 0f) * Mathf.Max(qx, 0f) + Mathf.Max(qy, 0f) * Mathf.Max(qy, 0f));
                float dist = outside + Mathf.Min(Mathf.Max(qx, qy), 0f) - r;
                float alpha = Mathf.Clamp01(0.5f - dist) * Mathf.Clamp01(dist + width + 0.5f);
                tex.SetPixel(x, y, new Color(1f, 1f, 1f, alpha));
            }
            tex.Apply();
            _ring = Sprite.Create(tex, new UnityEngine.Rect(0, 0, size, size), new Vector2(0.5f, 0.5f), 100f, 0,
                SpriteMeshType.FullRect, new Vector4(r + 1, r + 1, r + 1, r + 1));
            _ring.name = "TCGCC_FocusRing";
            return _ring;
        }

        // ------------------------------------------------------------------ hints

        /// <summary>The hint line under the columns: keyboard/mouse text, or the game's controller glyphs.</summary>
        private static void ShowHints(bool pad)
        {
            if (_hintKeys == null || _hintPad == null) return;
            if (_hintPad.activeSelf != pad) _hintPad.SetActive(pad);
            if (pad && _hintPadPS != PadInput.IsPS) BuildPadHint(); // once active: text sizes are measured while building
            if (_hintKeys.activeSelf == pad) _hintKeys.SetActive(!pad);
        }

        private static void BuildPadHint()
        {
            _hintPadPS = PadInput.IsPS;
            var root = (RectTransform)_hintPad.transform;
            for (int i = root.childCount - 1; i >= 0; i--)
            {
                var c = root.GetChild(i).gameObject;
                c.SetActive(false);
                Object.Destroy(c);
            }
            var v = _hintPad.GetComponent<VerticalLayoutGroup>();
            if (v == null) v = _hintPad.AddComponent<VerticalLayoutGroup>();
            v.childControlWidth = true; v.childControlHeight = true;
            v.childForceExpandWidth = true; v.childForceExpandHeight = true;
            v.spacing = _u * 0.1f;

            HintRow(root, new[]
            {
                (EGamepadControlBtn.X, "Select"), (EGamepadControlBtn.O, "Close"), (EGamepadControlBtn.Triangle, "Apply"),
                (EGamepadControlBtn.Square, "Reset"),
            });
            var second = new List<(EGamepadControlBtn, string)> { (EGamepadControlBtn.L1, ""), (EGamepadControlBtn.R1, "Scroll list") };
            if (Plugin.BinderPadVersion.Value != BinderPadButton.None)
                second.Add((PadInput.ToGame(Plugin.BinderPadVersion.Value), "Switch version"));
            if (Plugin.BinderPadFavourite.Value != BinderPadButton.None)
                second.Add((PadInput.ToGame(Plugin.BinderPadFavourite.Value), "Favourite"));
            HintRow(root, second.ToArray());
        }

        private static void HintRow(Transform parent, (EGamepadControlBtn btn, string text)[] items)
        {
            var row = Row(parent, _u * 0.8f);
            row.GetComponent<HorizontalLayoutGroup>().childForceExpandHeight = true;
            float size = _u * 0.6f;
            foreach (var (btn, text) in items)
            {
                var glyph = new GameObject("Glyph", typeof(RectTransform), typeof(Image), typeof(LayoutElement));
                glyph.layer = row.gameObject.layer;
                glyph.transform.SetParent(row, false);
                var img = glyph.GetComponent<Image>();
                img.sprite = PadInput.Glyph(btn);
                img.preserveAspect = true;
                img.raycastTarget = false;
                var gle = glyph.GetComponent<LayoutElement>();
                gle.minWidth = gle.preferredWidth = _u * 0.8f;
                if (img.sprite == null) glyph.SetActive(false);
                if (string.IsNullOrEmpty(text)) continue;
                var label = Label(row, text, size, TextAlignmentOptions.Left);
                label.color = new Color(_textColor.r, _textColor.g, _textColor.b, 0.6f);
                label.overflowMode = TextOverflowModes.Overflow; // never "…"
                // Fixed width from the text (min too: the row would otherwise squeeze labels to nothing when space is short).
                float w = Mathf.Max(label.GetPreferredValues(text).x, text.Length * size * 0.55f) + _u * 0.5f;
                var le = label.gameObject.AddComponent<LayoutElement>();
                le.minWidth = le.preferredWidth = w;
            }
            var filler = new GameObject("Filler", typeof(RectTransform), typeof(LayoutElement));
            filler.transform.SetParent(row, false);
            filler.GetComponent<LayoutElement>().flexibleWidth = 1f;
        }
    }
}

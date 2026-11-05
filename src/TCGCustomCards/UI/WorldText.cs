using System.Collections.Generic;
using HarmonyLib;
using TMPro;
using UnityEngine;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// Our own text over people (customer lines, card values) in the game's popup font: UI text on our own overlay canvas, placed
    /// each frame at the person's screen position and sized as if it stood there. On a canvas the scene can't draw over it, so
    /// window frames and glass never cover a line. Lines that would overlap are moved apart, the closer speaker keeping their place,
    /// and farther lines are fainter. The runner switches itself off when none is shown.
    /// Scene objects: everything goes with the shop scene on a reload and is built again on the next Show.
    /// </summary>
    internal class WorldText : MonoBehaviour
    {
        private class Entry
        {
            public TextMeshProUGUI Text;
            public Transform Follow;
            public float OffsetUp, Scale, Until;
            public Vector3 At;           // on screen, before moving apart; z = distance
            public Color Colour;
            public float Width, Height;  // of the text at scale 1
            public float Size;           // this frame's scale
            public float Lift;           // how far it's moved up now (pixels), easing towards what the layout wants
            public bool New;
        }

        private const int MaxShown = 24; // past this the oldest goes
        private const float Gap = 2f;    // pixels between lines moved apart
        private const float LiftSpeed = 12f;
        private const float ClearUntil = 4f, FaintFrom = 14f, Faintest = 0.45f; // metres, and how see-through at the far end

        private static WorldText _runner;
        private static readonly AccessTools.FieldRef<PricePopupSpawner, bool> CanShow =
            AccessTools.FieldRefAccess<PricePopupSpawner, bool>("m_CanShowPopup");

        private readonly List<Entry> _shown = new List<Entry>();
        private readonly Stack<TextMeshProUGUI> _free = new Stack<TextMeshProUGUI>();
        private readonly List<Entry> _layout = new List<Entry>();
        private TMP_FontAsset _font;
        private Material _material;
        private float _fontSize, _unitScale; // metres per text unit: our text at scale 1 = the game's popup height

        /// <summary>
        /// Shows <paramref name="text"/> <paramref name="offsetUp"/> metres above <paramref name="follow"/> for
        /// <paramref name="seconds"/>, <paramref name="scale"/> × the game's popup size. A text already at that spot over
        /// them is replaced. Shown only when the game would show its own popups (not in a menu, UI not hidden).
        /// </summary>
        public static void Show(Transform follow, string text, float offsetUp, float seconds, float scale = 1f, Color? colour = null)
        {
            if (follow == null || !Core.GameScene.InLoadedShop) return;
            var spawner = Core.GameScene.Existing<PricePopupSpawner>();
            var player = Core.GameScene.Existing<InteractionPlayerController>();
            var gm = Core.GameScene.Existing<CGameManager>();
            if (spawner == null || !CanShow(spawner) || (player != null && player.IsInUIMode()) || (gm != null && gm.m_IsTurnOffUI)) return;
            var runner = Runner(spawner);
            if (runner == null) return;
            runner.Place(follow, text, offsetUp, seconds, scale, colour ?? Color.white);
        }

        private static WorldText Runner(PricePopupSpawner spawner)
        {
            if (_runner != null) return _runner;
            TextMeshProUGUI template = null;
            foreach (var p in spawner.m_PricePopupList)
                if (p != null && p.m_Text != null) { template = p.m_Text; break; }
            if (template == null) return null;
            var go = new GameObject("TCGCC World Text");
            var canvas = go.AddComponent<Canvas>();
            canvas.renderMode = RenderMode.ScreenSpaceOverlay;
            canvas.sortingOrder = -10; // under the game's own screens
            _runner = go.AddComponent<WorldText>();
            _runner.Learn(template);
            return _runner;
        }

        /// <summary>Takes the game popup's font and works out the scale at which our text is as tall as its text in the world.</summary>
        private void Learn(TextMeshProUGUI template)
        {
            _font = template.font;
            _material = template.fontSharedMaterial;
            _fontSize = template.fontSize;
            var probe = NewText();
            float theirs = template.GetPreferredValues("Ag").y * template.transform.lossyScale.y;
            float ours = probe.GetPreferredValues("Ag").y;
            _unitScale = ours > 0f && theirs > 0f ? theirs / ours : 0.01f;
            Plugin.Log.LogInfo($"World text: font {(_font != null ? _font.name : "none")}, size {_fontSize}, game popup line {theirs:0.000} m, scale {_unitScale:0.0000}");
            Recycle(probe);
        }

        private TextMeshProUGUI NewText()
        {
            var go = new GameObject("Text");
            go.transform.SetParent(transform, false);
            var t = go.AddComponent<TextMeshProUGUI>();
            t.font = _font;
            if (_material != null) t.fontSharedMaterial = _material;
            t.fontSize = _fontSize;
            t.alignment = TextAlignmentOptions.Center;
            t.enableWordWrapping = false;
            t.richText = false;
            t.raycastTarget = false;
            t.rectTransform.sizeDelta = new Vector2(2000f, 200f);
            return t;
        }

        private void Recycle(TextMeshProUGUI t)
        {
            if (t == null) return; // went with a destroyed customer
            t.gameObject.SetActive(false);
            _free.Push(t);
        }

        private void Place(Transform follow, string text, float offsetUp, float seconds, float scale, Color colour)
        {
            Entry e = null;
            foreach (var s in _shown)
                if (s.Follow == follow && Mathf.Approximately(s.OffsetUp, offsetUp) && s.Text != null) { e = s; break; }
            if (e == null)
            {
                if (_shown.Count >= MaxShown) { Recycle(_shown[0].Text); _shown.RemoveAt(0); }
                TextMeshProUGUI t = null;
                while (_free.Count > 0 && t == null) t = _free.Pop();
                e = new Entry { Text = t != null ? t : NewText() };
                _shown.Add(e);
            }
            e.Follow = follow;
            e.OffsetUp = offsetUp;
            e.Scale = scale;
            e.Until = Time.time + seconds;
            e.Text.text = text;
            e.Colour = colour;
            e.Text.gameObject.SetActive(true);
            var size = e.Text.GetPreferredValues(text);
            e.Width = size.x;
            e.Height = size.y;
            e.New = true; // starts where the layout wants it
            enabled = true;
            Layout(Cam(), 0f);
        }

        private static Camera Cam()
        {
            var spawner = Core.GameScene.Existing<PricePopupSpawner>();
            var cam = spawner != null && spawner.m_Cam != null ? spawner.m_Cam.GetComponent<Camera>() : null;
            return cam != null ? cam : Camera.main;
        }

        /// <summary>Where the person is on screen, and how big the text would look standing there; off while behind the camera.</summary>
        private bool Measure(Entry e, Camera cam)
        {
            e.At = cam.WorldToScreenPoint(e.Follow.position + Vector3.up * e.OffsetUp);
            bool seen = e.At.z > 0.05f;
            if (e.Text.enabled != seen) e.Text.enabled = seen;
            if (!seen) return false;
            float pixelsPerMetre = cam.pixelHeight / (2f * e.At.z * Mathf.Tan(cam.fieldOfView * 0.5f * Mathf.Deg2Rad));
            e.Size = _unitScale * e.Scale * pixelsPerMetre;
            return !float.IsNaN(e.Size) && !float.IsInfinity(e.Size);
        }

        /// <summary>Closest first: each line keeps its spot unless a closer one is there, then moves up just past it.</summary>
        private void Layout(Camera cam, float deltaTime)
        {
            if (cam == null) return;
            _layout.Clear();
            foreach (var e in _shown)
                if (e.Text != null && e.Follow != null && Measure(e, cam)) _layout.Add(e);
            _layout.Sort((a, b) => a.At.z.CompareTo(b.At.z));
            for (int n = 0; n < _layout.Count; n++)
            {
                var e = _layout[n];
                float halfW = e.Width * e.Size / 2f, halfH = e.Height * e.Size / 2f, want = e.At.y;
                // Each pass moves it past a line it still overlaps, so n passes are enough; the bound keeps a frame from ever getting stuck.
                bool moved = true;
                for (int pass = 0; moved && pass <= n; pass++)
                {
                    moved = false;
                    for (int k = 0; k < n; k++)
                    {
                        var o = _layout[k];
                        float oHalfH = o.Height * o.Size / 2f, oY = o.At.y + o.Lift;
                        if (Mathf.Abs(o.At.x - e.At.x) >= halfW + o.Width * o.Size / 2f) continue;
                        if (Mathf.Abs(oY - want) >= halfH + oHalfH) continue; // Gap is added when moving, so rounding can't count as overlap
                        want = oY + oHalfH + halfH + Gap;
                        moved = true;
                    }
                }
                float lift = want - e.At.y;
                e.Lift = e.New ? lift : Mathf.Lerp(e.Lift, lift, Mathf.Min(1f, deltaTime * LiftSpeed));
                e.New = false;
                var tr = e.Text.rectTransform;
                tr.position = new Vector3(e.At.x, e.At.y + e.Lift, 0f);
                tr.localScale = new Vector3(e.Size, e.Size, 1f);
                var colour = e.Colour;
                colour.a *= Mathf.Lerp(1f, Faintest, Mathf.InverseLerp(ClearUntil, FaintFrom, e.At.z));
                e.Text.color = colour;
            }
        }

        // After everyone has moved, so a line doesn't trail behind its person.
        private void LateUpdate()
        {
            using var measured = Debug.SpikeWatch.Measure("world text");
            float now = Time.time;
            for (int i = _shown.Count - 1; i >= 0; i--)
            {
                var e = _shown[i];
                if (e.Text == null || e.Follow == null || now >= e.Until || !e.Follow.gameObject.activeInHierarchy)
                {
                    Recycle(e.Text);
                    _shown.RemoveAt(i);
                }
            }
            if (_shown.Count == 0) { enabled = false; return; } // nothing shown: Unity stops calling LateUpdate
            Layout(Cam(), Time.deltaTime);
        }
    }
}

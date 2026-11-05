using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Runtime.Customers;
using UnityEngine;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// [Debug] CustomerDebug: a live panel under a customer's profile (CustomerNameplates) showing what they are doing inside.
    /// Sections come from a registry: a feature adds its own with <see cref="Section"/> (usually in its Init), so a new
    /// behaviour brings its own rows without touching this panel. Rows are label/value pairs; <see cref="Bar"/> draws a timer.
    /// </summary>
    internal static class CustomerDebugPanel
    {
        /// <summary>Fills a section's rows for a customer in the shop (add nothing to hide the section).</summary>
        public delegate void Rows(Customer c, Persona p, List<(string label, string value)> rows);

        private static readonly List<(string title, Rows rows)> Sections = new List<(string, Rows)>();

        static CustomerDebugPanel()
        {
            Section("State", (c, p, rows) => Hooks.CustomerLife.StateRows(c, rows));
            Section("Till & bag", (c, p, rows) => Hooks.CustomerLife.TillRows(c, rows));
            Section("Next choice", (c, p, rows) =>
            {
                float total = Hooks.CustomerChoices.VanillaWeight;
                var list = new List<(string, float)>();
                foreach (var (id, w) in Hooks.CustomerChoices.Weights(c)) { list.Add((id, w)); total += w; }
                rows.Add(("vanilla", $"{Hooks.CustomerChoices.VanillaWeight / total * 100f:0}%"));
                foreach (var (id, w) in list) if (w > 0f) rows.Add((id, $"{w / total * 100f:0}%"));
            });
            Section("Visit data", (c, p, rows) =>
            {
                var names = Cast.VisitDataNames(c).ToList();
                if (names.Count > 0) rows.Add(("kept", string.Join(", ", names)));
            });
        }

        public static void Section(string title, Rows rows) => Sections.Add((title, rows));

        /// <summary>A text progress bar: "[|||||.....] 22/45 s".</summary>
        public static string Bar(float done, float total, string unit = "s", int width = 20)
        {
            float f = total > 0f ? Mathf.Clamp01(done / total) : 0f;
            int n = Mathf.RoundToInt(f * width);
            string colour = f < 0.5f ? "7CFC7C" : f < 0.85f ? "FFD24D" : "FF6A5A";
            return $"[<color=#{colour}>{new string('|', n)}</color>{new string('.', width - n)}] {done:0}/{total:0} {unit}";
        }

        private static GUIStyle _title, _label, _value;

        /// <summary>Draws the panel under the profile box, the same width; beside it when there's no room below.</summary>
        public static void Draw(Customer c, Persona p, Rect profile)
        {
            float width = profile.width;
            Styles();
            var blocks = new List<(string title, List<(string label, string value)> rows)>();
            foreach (var (title, fill) in Sections)
            {
                var rows = new List<(string, string)>();
                try { fill(c, p, rows); }
                catch (System.Exception e) { rows.Add(("error", e.Message)); }
                if (rows.Count > 0) blocks.Add((title, rows));
            }
            float labelW = width * 0.32f, valueW = width - labelW - 30f, pad = 10f;
            float height = pad;
            foreach (var (title, rows) in blocks)
            {
                height += _title.CalcHeight(new GUIContent(title), width - 2 * pad) + 2f;
                foreach (var (label, value) in rows)
                    height += Mathf.Max(_label.CalcHeight(new GUIContent(label), labelW), _value.CalcHeight(new GUIContent(value), valueW));
                height += 6f;
            }
            var box = new Rect(profile.x, profile.yMax + 8f, width, height + pad);
            if (box.yMax > Screen.height - 8f) box.position = new Vector2(profile.xMax + 8f, profile.y);
            GUI.Box(box, GUIContent.none);
            GUI.Box(box, GUIContent.none); // twice = darker
            float y = box.y + pad;
            foreach (var (title, rows) in blocks)
            {
                float th = _title.CalcHeight(new GUIContent(title), width - 2 * pad);
                GUI.Label(new Rect(box.x + pad, y, width - 2 * pad, th), title, _title);
                y += th + 2f;
                foreach (var (label, value) in rows)
                {
                    float h = Mathf.Max(_label.CalcHeight(new GUIContent(label), labelW), _value.CalcHeight(new GUIContent(value), valueW));
                    GUI.Label(new Rect(box.x + pad, y, labelW, h), label, _label);
                    GUI.Label(new Rect(box.x + pad + labelW + 10f, y, valueW, h), value, _value);
                    y += h;
                }
                y += 6f;
            }
        }

        private static void Styles()
        {
            int size = Mathf.RoundToInt(Screen.height / 68f);
            if (_title != null && _title.fontSize == size + 1) return;
            _title = new GUIStyle(GUI.skin.label) { fontSize = size + 1, fontStyle = FontStyle.Bold, richText = true };
            _title.normal.textColor = new Color(1f, 0.82f, 0.3f);
            _label = new GUIStyle(GUI.skin.label) { fontSize = size, richText = true, wordWrap = true };
            _label.normal.textColor = new Color(0.75f, 0.75f, 0.75f);
            _value = new GUIStyle(GUI.skin.label) { fontSize = size, richText = true, wordWrap = true };
            _value.normal.textColor = Color.white;
        }
    }
}

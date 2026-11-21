using System;
using System.Collections.Generic;
using UnityEngine;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// The MTG mana symbols for IMGUI screens: embedded PNGs (<c>Assets\mana</c>, made by Studio's <c>cmd/manasymbols</c> from
    /// Scryfall's symbology; named like the symbol without braces/slashes: W, 2U, WP, …). Colours (WUBRG), whole mana costs
    /// ("{2}{U}{U}", "{1}{R} // {G}") and the deck-kind chip. Unknown symbols are drawn as their text.
    /// </summary>
    internal static class ManaSymbols
    {
        private static readonly Dictionary<string, Texture2D> Cache = new Dictionary<string, Texture2D>(StringComparer.OrdinalIgnoreCase);
        private static GUIStyle _text, _chip;

        /// <summary>The symbol's picture (code like "W", "2U", "{G/P}"), or null when there is none.</summary>
        public static Texture2D Get(string code)
        {
            code = (code ?? "").Replace("{", "").Replace("}", "").Replace("/", "").ToUpperInvariant();
            if (code.Length == 0) return null;
            if (Cache.TryGetValue(code, out var tex)) return tex;
            tex = null;
            try
            {
                using (var s = typeof(ManaSymbols).Assembly.GetManifestResourceStream("TCGCustomCards.mana." + code))
                {
                    if (s != null)
                    {
                        var bytes = new byte[s.Length];
                        int read = 0;
                        while (read < bytes.Length)
                        {
                            int n = s.Read(bytes, read, bytes.Length - read);
                            if (n <= 0) break;
                            read += n;
                        }
                        tex = new Texture2D(2, 2, TextureFormat.RGBA32, mipChain: true) { name = "TCGCC_mana_" + code, hideFlags = HideFlags.DontSave };
                        if (tex.LoadImage(bytes, markNonReadable: true))
                        {
                            tex.filterMode = FilterMode.Trilinear;
                            tex.wrapMode = TextureWrapMode.Clamp;
                        }
                        else
                        {
                            UnityEngine.Object.Destroy(tex);
                            tex = null;
                        }
                    }
                }
            }
            catch (Exception e)
            {
                Plugin.Log.LogWarning($"Mana symbol {code}: {e.Message}");
                tex = null;
            }
            Cache[code] = tex;
            return tex;
        }

        private static void Styles()
        {
            if (_text != null) return;
            _text = new GUIStyle(GUI.skin.label) { richText = true, alignment = TextAnchor.MiddleLeft, wordWrap = false, padding = new RectOffset(0, 0, 0, 0) };
            _chip = new GUIStyle(GUI.skin.label) { richText = true, alignment = TextAnchor.MiddleCenter, fontStyle = FontStyle.Bold, wordWrap = false };
        }

        /// <summary>One symbol at (x, y); returns its width (or the fallback text's width).</summary>
        public static float Draw(float x, float y, string code, float size)
        {
            var tex = Get(code);
            if (tex != null)
            {
                GUI.DrawTexture(new Rect(x, y, size, size), tex, ScaleMode.ScaleToFit, alphaBlend: true);
                return size;
            }
            return Text(x, y, "{" + (code ?? "").Trim('{', '}') + "}", size);
        }

        private static float Text(float x, float y, string text, float size)
        {
            var style = TextStyle(size);
            float w = style.CalcSize(new GUIContent(text)).x;
            GUI.Label(new Rect(x, y, w + 2, size), text, style);
            return w;
        }

        /// <summary>
        /// A deck's or card's colours as symbols in WUBRG order ("" = one colourless symbol); returns the width used.
        /// </summary>
        public static float DrawColors(float x, float y, string colors, float size, float gap = 3f)
        {
            if (string.IsNullOrEmpty(colors)) return Draw(x, y, "C", size);
            float cx = x;
            foreach (char ch in colors)
                cx += Draw(cx, y, ch.ToString(), size) + gap;
            return cx - x - gap;
        }

        /// <summary>A mana cost's parts: symbols ("2", "U", "W/P") and plain text between them (" // " of split cards).</summary>
        private static IEnumerable<(bool symbol, string text)> Tokens(string cost)
        {
            int i = 0;
            while (i < cost.Length)
            {
                if (cost[i] == '{')
                {
                    int end = cost.IndexOf('}', i + 1);
                    if (end < 0) end = cost.Length - 1;
                    yield return (true, cost.Substring(i + 1, Math.Max(0, end - i - 1)));
                    i = end + 1;
                }
                else
                {
                    int next = cost.IndexOf('{', i);
                    string plain = (next < 0 ? cost.Substring(i) : cost.Substring(i, next - i)).Trim();
                    if (plain.Length > 0) yield return (false, " " + plain + " ");
                    i = next < 0 ? cost.Length : next;
                }
            }
        }

        private static GUIStyle TextStyle(float size)
        {
            Styles();
            return new GUIStyle(_text) { fontSize = Mathf.RoundToInt(size * 0.75f) };
        }

        /// <summary>A mana cost as written in set.json ("{2}{U}{U}", split cards "{1}{R} // {2}{G}"); returns the width used.</summary>
        public static float DrawCost(float x, float y, string cost, float size, float gap = 1f)
        {
            if (string.IsNullOrEmpty(cost)) return 0f;
            float cx = x;
            foreach (var (symbol, text) in Tokens(cost))
                cx += (symbol ? Draw(cx, y, text, size) : Text(cx, y, text, size)) + gap;
            return Math.Max(0f, cx - x - gap);
        }

        /// <summary>Width <see cref="DrawCost"/> would use (for right-aligning a cost).</summary>
        public static float CostWidth(string cost, float size, float gap = 1f)
        {
            if (string.IsNullOrEmpty(cost)) return 0f;
            var style = TextStyle(size);
            float w = 0;
            int parts = 0;
            foreach (var (symbol, text) in Tokens(cost))
            {
                w += symbol && Get(text) != null ? size : style.CalcSize(new GUIContent(symbol ? "{" + text + "}" : text)).x;
                parts++;
            }
            return w + Math.Max(0, parts - 1) * gap;
        }

        /// <summary>The deck-kind chip: "Commander" (purple) or "Constructed" (blue-grey); returns its width.</summary>
        public static float KindBadge(float x, float y, bool commander, float height = 24f, int fontSize = 13)
        {
            Styles();
            string label = commander ? "Commander" : "Constructed";
            var style = new GUIStyle(_chip) { fontSize = fontSize };
            float w = style.CalcSize(new GUIContent(label)).x + 16;
            var before = GUI.color;
            GUI.color = before * (commander ? new Color(0.45f, 0.25f, 0.65f, 0.85f) : new Color(0.25f, 0.35f, 0.45f, 0.85f));
            GUI.DrawTexture(new Rect(x, y, w, height), Texture2D.whiteTexture);
            GUI.color = before;
            style.normal.textColor = commander ? new Color(0.9f, 0.82f, 1f) : new Color(0.85f, 0.92f, 1f);
            GUI.Label(new Rect(x, y, w, height), label, style);
            return w;
        }
    }
}

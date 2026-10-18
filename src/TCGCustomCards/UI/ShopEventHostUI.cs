using System.Linq;
using TCGCustomCards.Hooks;
using TCGCustomCards.Runtime;
using UnityEngine;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// Panel beside the phone's Host Tournament screen: tournament format (Tetramon = vanilla, or a registered
    /// <see cref="ShopEventFormat"/>), the sets usable in the event (1..n installed sets) and packs per player for pack formats.
    /// Edits <see cref="ShopEvents.Pending"/>; vanilla's Confirm takes it over (ShopEvents host patches). Read-only while hosting.
    /// IMGUI on a 1920×1080 virtual layout, left edge (the phone sits in the middle).
    /// </summary>
    internal class ShopEventHostUI : MonoBehaviour
    {
        private const float W = 1920f, H = 1080f;
        private static ShopEventHostUI _inst;

        private string _search = "";
        private string _note;
        /// <summary>"How it works" open (kept between openings).</summary>
        private static bool _showHelp = true;
        /// <summary>Collapsed to one line (kept between openings).</summary>
        private static bool _minimized;
        private Vector2 _scroll, _helpScroll;
        /// <summary>Height of the content drawn last pass (the background's height).</summary>
        private float _panelH = 300;
        private GUIStyle _text, _small, _title, _button, _selected;

        public static void Open()
        {
            if (_inst == null)
            {
                var go = new GameObject("TCGCC_ShopEventHost");
                DontDestroyOnLoad(go);
                _inst = go.AddComponent<ShopEventHostUI>();
            }
            _inst._search = "";
            _inst.enabled = true;
        }

        public static void Close()
        {
            if (_inst != null) _inst.enabled = false;
        }

        private void Styles()
        {
            if (_text != null) return;
            _text = new GUIStyle(GUI.skin.label) { fontSize = 18, wordWrap = true, richText = true };
            _small = new GUIStyle(_text) { fontSize = 15 };
            _title = new GUIStyle(_text) { fontSize = 24, fontStyle = FontStyle.Bold };
            _button = new GUIStyle(GUI.skin.button) { fontSize = 17, richText = true, alignment = TextAnchor.MiddleLeft, padding = new RectOffset(12, 8, 4, 4) };
            _selected = new GUIStyle(_button) { fontStyle = FontStyle.Bold };
            _selected.normal.textColor = _selected.hover.textColor = new Color(1f, 0.85f, 0.4f);
        }

        private void OnGUI()
        {
            Styles();
            GUI.depth = -100;
            float scale = Mathf.Min(Screen.width / W, Screen.height / H);
            var offset = new Vector2((Screen.width - W * scale) / 2f, (Screen.height - H * scale) / 2f);
            GUI.matrix = Matrix4x4.TRS(new Vector3(offset.x, offset.y, 0f), Quaternion.identity, new Vector3(scale, scale, 1f));

            var p = ShopEvents.Pending;
            bool locked = CPlayerData.m_TournamentData.m_IsHostingTournament || CPlayerData.m_TournamentData.m_IsTournamentDay;
            var format = ShopEvents.FormatList.FirstOrDefault(f => f.Id == p.format);
            float x = 30, y = 120, w = 460;
            string help = format?.Help;
            float helpH = _showHelp && help != null ? _small.CalcHeight(new GUIContent(help), w - 32) + 6 : 0;
            // Long help scrolls in its own box. The panel is as tall as what was drawn (measured by the Layout pass, which runs
            // before Repaint in the same frame) and never goes below the screen: the set list takes what's left.
            float helpBoxH = Mathf.Min(helpH, 200f);
            float top = y, bottom = H - 10;
            float h = _minimized ? 62 : Mathf.Min(_panelH, bottom - top);

            GUI.color = new Color(0.06f, 0.06f, 0.08f, 0.94f);
            GUI.DrawTexture(new Rect(x, y, w, h), Texture2D.whiteTexture);
            GUI.color = Color.white;
            x += 16; w -= 32; y += 12;

            // Header + minimize toggle (always clickable, also while the tournament is locked in): the open panel covers
            // the screen's "registered 1/8" text.
            if (GUI.Button(new Rect(x + w - 40, y, 40, 34), _minimized ? "+" : "−", new GUIStyle(GUI.skin.button) { fontSize = 20 }))
                _minimized = !_minimized;
            if (_minimized)
            {
                string summary = format == null ? "Tetramon" : $"{format.Name} · {p.sets.Count} set{(p.sets.Count == 1 ? "" : "s")}" +
                                 (format.OffersBestOf && p.bestOf == 3 ? " · Best of 3" : "");
                GUI.Label(new Rect(x, y, w - 50, 36), $"<b>Format:</b> {summary}", _text);
                return;
            }
            GUI.Label(new Rect(x, y, w - 50, 32), "Tournament format", _title);
            y += 38;
            if (locked)
            {
                GUI.Label(new Rect(x, y, w, 44), "<color=#aaaaaa>Set when hosting. Cancel the tournament to change it.</color>", _small);
                y += 44;
            }
            GUI.enabled = !locked;
            if (GUI.Button(new Rect(x, y, w, 38), (p.format == null ? "● " : "○ ") + "Tetramon (vanilla)", p.format == null ? _selected : _button))
                p.format = null;
            y += 42;
            foreach (var f in ShopEvents.FormatList)
            {
                bool on = p.format == f.Id;
                bool avail = f.Available();
                GUI.enabled = !locked && avail;
                if (GUI.Button(new Rect(x, y, w, 38), (on ? "● " : "○ ") + f.Name, on ? _selected : _button))
                {
                    if (p.format != f.Id && f.UsesPacks) p.packsPerPlayer = f.DefaultPacksPerPlayer;
                    p.format = f.Id;
                    p.sets.RemoveAll(id => !Registry.Sets.Any(s => s.Def.Id == id && f.SetEligible(s)));
                }
                y += 40;
                string note = avail ? f.Description : f.Unavailable;
                if (!string.IsNullOrEmpty(note))
                {
                    GUI.enabled = true;
                    GUI.Label(new Rect(x + 12, y, w - 12, 40), $"<color=#aaaaaa>{note}</color>", _small);
                    y += 42;
                }
            }
            GUI.enabled = true;
            if (format == null) { _panelH = y + 12 - top; return; }

            y += 6;
            if (format.UsesPacks)
            {
                GUI.Label(new Rect(x, y, 230, 34), "Packs per player", _text);
                GUI.enabled = !locked;
                if (GUI.Button(new Rect(x + 240, y, 40, 34), "−")) p.packsPerPlayer = Mathf.Max(1, p.packsPerPlayer - 1);
                GUI.Label(new Rect(x + 285, y, 50, 34), $"<b>{p.packsPerPlayer}</b>", new GUIStyle(_text) { alignment = TextAnchor.MiddleCenter });
                if (GUI.Button(new Rect(x + 340, y, 40, 34), "+")) p.packsPerPlayer = Mathf.Min(format.MaxPacksPerPlayer, p.packsPerPlayer + 1);
                GUI.enabled = true;
                y += 40;
            }
            if (format.OffersBestOf)
            {
                GUI.Label(new Rect(x, y, 150, 34), "Your matches", _text);
                GUI.enabled = !locked;
                if (GUI.Button(new Rect(x + 150, y, 130, 34), (p.bestOf == 3 ? "○ " : "● ") + "Best of 1", p.bestOf == 3 ? _button : _selected)) p.bestOf = 1;
                if (GUI.Button(new Rect(x + 290, y, 138, 34), (p.bestOf == 3 ? "● " : "○ ") + "Best of 3", p.bestOf == 3 ? _selected : _button)) p.bestOf = 3;
                GUI.enabled = true;
                y += 40;
            }
            if (help != null)
            {
                if (GUI.Button(new Rect(x, y, 200, 30), _showHelp ? "Hide how it works" : "Show how it works", _button)) _showHelp = !_showHelp;
                y += 34;
                if (_showHelp)
                {
                    if (helpH > helpBoxH)
                    {
                        float inner = _small.CalcHeight(new GUIContent(help), w - 20) + 6; // narrower beside the scroll bar
                        _helpScroll = GUI.BeginScrollView(new Rect(x, y, w, helpBoxH), _helpScroll, new Rect(0, 0, w - 20, inner));
                        GUI.Label(new Rect(0, 0, w - 20, inner), $"<color=#cccccc>{help}</color>", _small);
                        GUI.EndScrollView();
                    }
                    else GUI.Label(new Rect(x, y, w, helpH), $"<color=#cccccc>{help}</color>", _small);
                    y += helpBoxH;
                }
            }
            string info = format.Info?.Invoke(p, ShopEvents.Capacity);
            if (!string.IsNullOrEmpty(info))
            {
                float ih = _small.CalcHeight(new GUIContent(info), w);
                GUI.Label(new Rect(x, y, w, ih), info, _small);
                y += ih + 4;
            }
            if (format.UsesPacks && !locked && ShopEvents.StockTotal > 0)
            {
                if (GUI.Button(new Rect(x, y, 260, 32), $"Take back {ShopEvents.StockTotal} pack(s)", _button))
                    ShopEvents.ReturnStock("taken back on the host screen");
                y += 38;
            }

            if (format.SetsFromStock)
            {
                if (!locked) p.sets = ShopEvents.StockSetIds();
                var names = p.sets.Select(id => Registry.Sets.FirstOrDefault(s => s.Def.Id == id)).Where(s => s != null)
                    .Select(s => s.Def.Name ?? s.Def.Id).ToList();
                string sets = names.Count == 0 ? "<color=#aaaaaa>none yet</color>" : string.Join(", ", names);
                string text = $"Sets: {sets}  <color=#888888>(from the packs you put up)</color>";
                float th = _small.CalcHeight(new GUIContent(text), w);
                GUI.Label(new Rect(x, y, w, th), text, _small);
                y += th + 6;
                y = DrawProblem(x, y, w, _note ?? (locked ? null : ShopEvents.PendingProblem()));
                _panelH = y + 12 - top;
                return;
            }

            var eligible = Registry.Sets.Where(format.SetEligible).OrderBy(s => s.Def.Name ?? s.Def.Id).ToList();
            GUI.Label(new Rect(x, y, w, 30), $"Sets in this tournament  <color=#ffcc66>{p.sets.Count} chosen</color>", _text);
            y += 32;
            GUI.enabled = !locked;
            _search = GUI.TextField(new Rect(x, y, w - 150, 32), _search ?? "", new GUIStyle(GUI.skin.textField) { fontSize = 17 });
            if (GUI.Button(new Rect(x + w - 140, y, 65, 32), "All"))
                foreach (var s in eligible.Where(Matches)) if (!p.sets.Contains(s.Def.Id)) p.sets.Add(s.Def.Id);
            if (GUI.Button(new Rect(x + w - 70, y, 70, 32), "None")) p.sets.RemoveAll(id => !HasStock(id));
            y += 38;

            var shown = eligible.Where(Matches).ToList();
            string problem = _note ?? (locked ? null : ShopEvents.PendingProblem());
            float problemH = problem == null ? 0 : _small.CalcHeight(new GUIContent(problem), w) + 6;
            const float rowH = 34;
            float room = bottom - 12 - problemH - 6 - y; // the list fills what's left on screen, at least 3 rows
            float listH = Mathf.Max(3 * rowH, Mathf.Min(room, Mathf.Max(1, shown.Count) * rowH));
            _scroll = GUI.BeginScrollView(new Rect(x, y, w, listH), _scroll, new Rect(0, 0, w - 20, Mathf.Max(listH, shown.Count * rowH)));
            for (int i = 0; i < shown.Count; i++)
            {
                var s = shown[i];
                bool on = p.sets.Contains(s.Def.Id);
                string code = s.Def.Mtg?.SetCode;
                string label = $"{(on ? "[x]" : "[  ]")}  {s.Def.Name ?? s.Def.Id}" + (string.IsNullOrEmpty(code) ? "" : $"  <color=#888888>{code.ToUpperInvariant()}</color>");
                if (GUI.Button(new Rect(0, i * rowH, w - 20, rowH - 3), label, on ? _selected : _button))
                {
                    if (on && HasStock(s.Def.Id)) _note = "Packs of this set are in the tournament stock - take them back first";
                    else if (on) p.sets.Remove(s.Def.Id);
                    else p.sets.Add(s.Def.Id);
                }
            }
            if (shown.Count == 0) GUI.Label(new Rect(0, 0, w - 20, 60), "<color=#aaaaaa>No installed set fits this format.</color>", _small);
            GUI.EndScrollView();
            GUI.enabled = true;
            y += listH + 6;
            y = DrawProblem(x, y, w, problem);
            _panelH = y + 12 - top;
        }

        /// <summary>Red reason under the content (why it can't be hosted yet); returns the y below it.</summary>
        private float DrawProblem(float x, float y, float w, string problem)
        {
            if (_note != null && Event.current.type == EventType.MouseDown) _note = null;
            if (problem == null) return y;
            float ph = _small.CalcHeight(new GUIContent(problem), w);
            GUI.Label(new Rect(x, y, w, ph), $"<color=#ff8866>{problem}</color>", _small);
            return y + ph + 6;
        }

        /// <summary>A set whose packs are in the tournament stock can't leave the event's set list.</summary>
        private static bool HasStock(string setId) =>
            ShopEvents.Stock.Any(kv => kv.Value > 0 && ShopEvents.PackOf(kv.Key)?.Set.Def.Id == setId);

        private bool Matches(CustomSet s)
        {
            if (string.IsNullOrWhiteSpace(_search)) return true;
            string q = _search.Trim();
            return (s.Def.Name ?? "").IndexOf(q, System.StringComparison.OrdinalIgnoreCase) >= 0
                   || (s.Def.Id ?? "").IndexOf(q, System.StringComparison.OrdinalIgnoreCase) >= 0
                   || (s.Def.Mtg?.SetCode ?? "").IndexOf(q, System.StringComparison.OrdinalIgnoreCase) >= 0;
        }
    }
}

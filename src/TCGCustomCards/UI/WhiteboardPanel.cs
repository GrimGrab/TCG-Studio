using System;
using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Runtime.Customers;
using UnityEngine;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// What clicking the shop whiteboard opens: tabs of stats about the regulars, a page per regular, and renaming or forgetting them.
    /// IMGUI on the hooks runner, 1920×1080 virtual layout, in the game's UI mode while open. Everything is read live from the modules.
    /// </summary>
    internal static class WhiteboardPanel
    {
        private const float W = 1920f, H = 1080f;
        private const int MaxNameLength = 24;
        private const int RecordRows = 5, NewsRows = 25;
        private static readonly string[] Tabs = { "Legends", "Regulars", "Records", "Drama", "Wall of Shame", "Guide" };
        private static readonly Rect Board = new Rect(190f, 70f, 1540f, 940f);
        private static readonly Color Marker = new Color(0.1f, 0.14f, 0.32f);

        private static readonly HarmonyLib.AccessTools.FieldRef<InteractionPlayerController, bool> InUIMode =
            Core.Compat.Field<InteractionPlayerController, bool>(typeof(WhiteboardPanel), "m_IsInUIMode");

        private static bool _open, _armed, _confirmRemove;
        private static int _tab;
        private static string _sort = "Spent";
        private static bool _ascending;
        private static Persona _person;
        private static Whiteboard.Board _board;
        private static string _nameEdit, _nickEdit, _message;
        private static Vector2 _scroll;
        private static GUIStyle _text, _big, _small, _head, _button, _field;

        public static bool IsOpen => _open;

        public static void Open(Whiteboard.Board board)
        {
            if (_open) return;
            _board = board;
            var ipc = Core.GameScene.Existing<InteractionPlayerController>();
            if (ipc == null || ipc.IsInUIMode()) return;
            _open = true;
            _armed = false;
            _scroll = Vector2.zero;
            _person = null;
            _message = null;
            ipc.m_WalkerCtrl?.SetStopMovement(isStop: true);
            ipc.EnterUIMode();
            SoundManager.GenericConfirm();
        }

        private static void Close()
        {
            _open = false;
            _person = null;
            var ipc = Core.GameScene.Existing<InteractionPlayerController>();
            ipc?.m_WalkerCtrl?.SetStopMovement(isStop: false);
            ipc?.ExitUIMode();
            SoundManager.GenericCancel();
        }

        public static void Tick()
        {
            if (!_open) return;
            if (!Core.GameScene.InLoadedShop) { _open = false; _person = null; return; } // the scene went: nothing to give UI mode back to
            var ipc = Core.GameScene.Existing<InteractionPlayerController>();
            if (ipc != null && !InUIMode(ipc))
            {
                ipc.m_WalkerCtrl?.SetStopMovement(isStop: true);
                ipc.EnterUIMode();
            }
            if (!Input.GetKeyDown(KeyCode.Escape)) return;
            if (_person != null) _person = null;
            else Close();
        }

        private static void Styles()
        {
            if (_text != null) return;
            _text = new GUIStyle(GUI.skin.label) { fontSize = 21, wordWrap = true, richText = true };
            _text.normal.textColor = Marker;
            _small = new GUIStyle(_text) { fontSize = 18 };
            _head = new GUIStyle(_text) { fontSize = 24, fontStyle = FontStyle.Bold };
            _big = new GUIStyle(_text) { fontSize = 38, fontStyle = FontStyle.Bold };
            _button = new GUIStyle(GUI.skin.button) { fontSize = 20, fontStyle = FontStyle.Bold, richText = true };
            _field = new GUIStyle(GUI.skin.textField) { fontSize = 21 };
        }

        public static void Draw()
        {
            if (!_open) return;
            Styles();
            GUI.depth = -120;
            if (!_armed) // the click that opened us must not also press a button
            {
                var e = Event.current;
                if (e.type == EventType.MouseDown) _armed = true;
                else if (e.type == EventType.MouseUp || e.type == EventType.MouseDrag) e.Use();
            }
            float scale = Mathf.Min(Screen.width / W, Screen.height / H);
            GUI.matrix = Matrix4x4.TRS(new Vector3((Screen.width - W * scale) / 2f, (Screen.height - H * scale) / 2f, 0f), Quaternion.identity, new Vector3(scale, scale, 1f));

            Fill(Board, new Color(0.59f, 0.6f, 0.62f));
            Fill(new Rect(Board.x + 12f, Board.y + 12f, Board.width - 24f, Board.height - 24f), new Color(0.96f, 0.96f, 0.94f));
            float x = Board.x + 36f, w = Board.width - 72f;
            GUI.Label(new Rect(x, Board.y + 22f, w - 90f, 56f), "SHOP LEGENDS", _big);
            if (_board != null)
            {
                const float viewW = 330f, arrow = 44f, bx = 400f;
                GUI.Label(new Rect(x + bx, Board.y + 34f, 200f, 36f), "This board shows:", _small);
                if (GUI.Button(new Rect(x + bx + 180f, Board.y + 28f, arrow, 44f), "<", _button)) _board.ChangeView(-1);
                GUI.Label(new Rect(x + bx + 180f + arrow + 10f, Board.y + 32f, viewW - 20f, 36f), $"<b>{_board.ViewTitle}</b>", _text);
                if (GUI.Button(new Rect(x + bx + 180f + arrow + viewW, Board.y + 28f, arrow, 44f), ">", _button)) _board.ChangeView(1);
            }
            if (GUI.Button(new Rect(x + w - 80f, Board.y + 28f, 80f, 44f), "Close", _button)) { Close(); GUI.matrix = Matrix4x4.identity; return; }

            float tabW = w / Tabs.Length;
            for (int i = 0; i < Tabs.Length; i++)
            {
                GUI.backgroundColor = i == _tab ? new Color(0.35f, 0.55f, 1f) : Color.white;
                if (GUI.Button(new Rect(x + i * tabW, Board.y + 84f, tabW - 6f, 46f), Tabs[i], _button) && (_tab != i || _person != null))
                {
                    _tab = i;
                    _person = null;
                    _scroll = Vector2.zero;
                }
            }
            GUI.backgroundColor = Color.white;

            var body = new Rect(x, Board.y + 142f, w, Board.height - 170f);
            try
            {
                if (_person != null) DrawPerson(body);
                else if (_tab == 0) DrawLegends(body);
                else if (_tab == 1) DrawRegulars(body);
                else if (_tab == 2) DrawRecords(body);
                else if (_tab == 3) DrawDrama(body);
                else if (_tab == 4) DrawShame(body);
                else DrawGuide(body);
            }
            finally { GUI.matrix = Matrix4x4.identity; }
        }

        private static void Fill(Rect r, Color c)
        {
            GUI.color = c;
            GUI.DrawTexture(r, Texture2D.whiteTexture);
            GUI.color = Color.white;
        }

        /// <summary>Lines in a scrolling column.</summary>
        private static void Lines(Rect r, List<string> lines, ref Vector2 scroll)
        {
            float width = r.width - 24f;
            var heights = lines.Select(l => (l.StartsWith("<b>") ? _head : _text).CalcHeight(new GUIContent(l), width) + 4f).ToList();
            scroll = GUI.BeginScrollView(r, scroll, new Rect(0f, 0f, width, Mathf.Max(r.height, heights.Sum())));
            float y = 0f;
            for (int i = 0; i < lines.Count; i++)
            {
                GUI.Label(new Rect(0f, y, width, heights[i]), lines[i], lines[i].StartsWith("<b>") ? _head : _text);
                y += heights[i];
            }
            GUI.EndScrollView();
        }

        // ---- Legends

        private static void DrawLegends(Rect r)
        {
            var lines = new List<string> { "<b>Who's who</b>" };
            var holders = Nicknames.Holders();
            if (holders.Count == 0) lines.Add("Nobody has earned a name yet. Regulars earn them for what they do in your shop.");
            foreach (var h in holders)
                lines.Add($"<b><color=#A0522D>{h.nick}</color></b>   {h.who}" + (h.label != null ? $"   ({h.label}: {h.number})" : ""));
            if (Nicknames.Log.Count > 0) { lines.Add(""); lines.Add("<b>Latest changes</b>"); }
            foreach (var c in Nicknames.Log.Reverse()) lines.Add($"Day {c.d}: {c.t}");
            Lines(r, lines, ref _scroll);
        }

        // ---- Regulars

        private static readonly (string title, float width, Func<Persona, float> value, Func<Persona, string> show)[] Columns =
        {
            ("Visits", 120f, p => p.visits, p => p.visits.ToString()),
            ("Spent", 150f, p => p.spent, p => "$" + p.spent.ToString("0")),
            ("Packs", 120f, p => PackRipping.PacksOpened(p), p => PackRipping.PacksOpened(p).ToString()),
            ("Wins", 120f, p => Rivalries.Wins(p), p => Rivalries.Wins(p).ToString()),
            ("Last seen", 190f, p => Cast.IsInShop(p) ? float.MaxValue : p.lastDay, Seen),
        };

        private static string Seen(Persona p) => Cast.IsInShop(p) ? "<color=#1E8E3E>in the shop</color>" : p.lastDay < 0 ? "not yet" : $"day {p.lastDay}";

        private static void SortButton(Rect r, string title)
        {
            string arrow = _sort == title ? (_ascending ? " ^" : " v") : "";
            if (!GUI.Button(r, title + arrow, _button)) return;
            if (_sort == title) _ascending = !_ascending;
            else { _sort = title; _ascending = title == "Name"; }
        }

        private static void DrawRegulars(Rect r)
        {
            var column = Columns.FirstOrDefault(c => c.title == _sort);
            IEnumerable<Persona> ordered = Cast.AllRegulars;
            ordered = column.value == null
                ? ordered.OrderBy(p => p.name, StringComparer.CurrentCultureIgnoreCase)
                : ordered.OrderBy(p => column.value(p)).ThenBy(p => p.name, StringComparer.CurrentCultureIgnoreCase);
            var list = ordered.ToList();
            if (!_ascending) list.Reverse();

            GUI.Label(new Rect(r.x, r.y, r.width, 32f), $"{list.Count} of {Plugin.MaxRegulars.Value} regulars. Click someone for their page.", _small);
            float nameW = r.width - Columns.Sum(c => c.width) - 24f;
            float x = r.x;
            SortButton(new Rect(x, r.y + 34f, nameW - 4f, 38f), "Name");
            x += nameW;
            foreach (var c in Columns)
            {
                SortButton(new Rect(x, r.y + 34f, c.width - 4f, 38f), c.title);
                x += c.width;
            }

            const float rowH = 46f;
            var view = new Rect(r.x, r.y + 78f, r.width, r.height - 78f);
            if (list.Count == 0) GUI.Label(view, "No regulars yet: customers who spend well may come back.", _text);
            _scroll = GUI.BeginScrollView(view, _scroll, new Rect(0f, 0f, r.width - 24f, Mathf.Max(view.height, list.Count * rowH)));
            for (int i = 0; i < list.Count; i++)
            {
                var p = list[i];
                var row = new Rect(0f, i * rowH, r.width - 24f, rowH - 4f);
                if (i % 2 == 0) Fill(row, new Color(0.87f, 0.89f, 0.95f));
                if (GUI.Button(row, GUIContent.none, GUIStyle.none)) OpenPerson(p);
                string nick = Nicknames.Of(p);
                GUI.Label(new Rect(8f, row.y + 6f, nameW - 12f, 34f), $"<b>{p.name}</b>" + (nick != null ? $"  <color=#A0522D>\"{nick}\"</color>" : ""), _text);
                float cx = nameW;
                foreach (var c in Columns)
                {
                    GUI.Label(new Rect(cx + 6f, row.y + 6f, c.width - 10f, 34f), c.show(p), _text);
                    cx += c.width;
                }
            }
            GUI.EndScrollView();
        }

        // ---- A regular's page

        private static void OpenPerson(Persona p)
        {
            _person = p;
            _nameEdit = p.name;
            _nickEdit = Nicknames.Of(p) ?? "";
            _message = null;
            _confirmRemove = false;
            _scroll = Vector2.zero;
        }

        private static string Clean(string s) => (s ?? "").Replace("<", "").Replace(">", "").Trim();

        private static void DrawPerson(Rect r)
        {
            var p = _person;
            if (Cast.Regular(p.id) != p) { _person = null; return; } // a load replaced them
            if (GUI.Button(new Rect(r.x, r.y, 120f, 40f), "< Back", _button)) { _person = null; _scroll = Vector2.zero; return; }

            const float side = 460f;
            string nick = Nicknames.Of(p);
            var rec = Rivalries.Record(p);
            var lines = new List<string> { $"<b>{p.name}</b>" + (nick != null ? $"  <color=#A0522D>\"{nick}\"</color>" : "") };
            lines.Add((Cast.IsInShop(p) ? "<color=#1E8E3E>In the shop now.</color> " : "") + $"A regular since day {p.firstDay}. Traits: " +
                (p.traits.Count == 0 ? "none" : string.Join(", ", p.traits.Select(Traits.NameOf))) + ".");
            lines.Add("");
            lines.Add("<b>Numbers</b>");
            lines.Add($"{p.visits} visit(s), ${p.spent:0} spent, last seen {(p.lastDay < 0 ? "never" : "day " + p.lastDay)}");
            lines.Add($"{PackRipping.PacksOpened(p)} pack(s) opened, best pull ${PackRipping.BestPull(p):0}, {Nicknames.HugePulls(p)} huge pull(s)");
            lines.Add($"Table games: {rec.wins} won, {rec.losses} lost. Against you: {rec.beatYou} won, {rec.lostToYou} lost. Skill {p.skill}");
            lines.Add($"Rip-offs noticed: {Nicknames.RipOffs(p)}. Caught stealing: {Shoplifting.Caught(p)}. Got away: {Shoplifting.GotAway(p)}. Kicked out: {KickOut.Kicks(p)}");
            string rival = Rivalries.RivalName(p);
            var friends = Friends.FriendNames(p).ToList();
            if (rival != null || friends.Count > 0)
            {
                lines.Add("");
                lines.Add("<b>People</b>");
                if (rival != null) lines.Add("Rival: " + rival);
                if (friends.Count > 0) lines.Add("Friends: " + string.Join(", ", friends));
            }
            var notes = CustomerNameplates.Summaries(p).ToList();
            if (Loyalty.TierOf(p) is Traits.LoyaltyTier tier) notes.Insert(0, $"Loyalty: {tier.name}");
            if (Trading.BestCard(p) is CardData prized) notes.Insert(0, $"Prized card: {CardInfo.Name(prized)} ({CardBubble.Value(CardInfo.Price(prized))})");
            if (notes.Count > 0)
            {
                lines.Add("");
                lines.Add("<b>Notes</b>");
                lines.AddRange(notes);
            }
            var reviews = Reviews.Of(p);
            if (reviews.Count > 0)
            {
                lines.Add("");
                lines.Add("<b>Their reviews</b>");
                foreach (var (stars, day, text) in reviews) lines.Add($"{stars}/5, day {day}: {text}");
            }
            Lines(new Rect(r.x, r.y + 50f, r.width - side - 20f, r.height - 50f), lines, ref _scroll);

            float x = r.xMax - side, y = r.y + 50f;
            var icon = p.model >= 0 ? CSingleton<CustomerManager>.Instance?.GetCustomerIconSprite(p.model, p.female) : null;
            if (icon != null)
            {
                var t = icon.texture;
                var tr = icon.textureRect;
                GUI.DrawTextureWithTexCoords(new Rect(x + side - 130f, r.y, 130f, 130f), t, new Rect(tr.x / t.width, tr.y / t.height, tr.width / t.width, tr.height / t.height));
                y += 90f;
            }
            GUI.Label(new Rect(x, y, side, 32f), "Name", _head);
            _nameEdit = GUI.TextField(new Rect(x, y + 36f, side - 110f, 40f), _nameEdit ?? "", MaxNameLength, _field);
            if (GUI.Button(new Rect(x + side - 100f, y + 36f, 100f, 40f), "Save", _button))
            {
                string name = Clean(_nameEdit);
                _message = Cast.Rename(p, name) ? $"Now called {name}." : "That name is empty or another regular has it.";
                _nameEdit = p.name;
            }
            y += 92f;
            GUI.Label(new Rect(x, y, side, 32f), "Nickname", _head);
            _nickEdit = GUI.TextField(new Rect(x, y + 36f, side - 110f, 40f), _nickEdit ?? "", MaxNameLength, _field);
            if (GUI.Button(new Rect(x + side - 100f, y + 36f, 100f, 40f), "Save", _button))
            {
                string word = Clean(_nickEdit);
                Nicknames.SetByPlayer(p, word);
                _nickEdit = Nicknames.Of(p) ?? "";
                _message = word.Length == 0 ? "Nickname removed. They can earn one again." : $"Now known as {word}. They keep it until you change it.";
            }
            y += 100f;
            bool here = Cast.IsInShop(p);
            GUI.enabled = !here;
            if (!_confirmRemove)
            {
                if (GUI.Button(new Rect(x, y, side, 44f), "Remove regular", _button)) _confirmRemove = true;
            }
            else
            {
                GUI.Label(new Rect(x, y, side, 60f), "Forget them for good? Their stats and nickname go too.", _small);
                if (GUI.Button(new Rect(x, y + 62f, side / 2f - 4f, 44f), "Yes, remove", _button))
                {
                    Cast.Forget(p);
                    _person = null;
                    _scroll = Vector2.zero;
                    GUI.enabled = true;
                    return;
                }
                if (GUI.Button(new Rect(x + side / 2f + 4f, y + 62f, side / 2f - 4f, 44f), "Keep", _button)) _confirmRemove = false;
            }
            GUI.enabled = true;
            if (here) GUI.Label(new Rect(x, y + 48f, side, 32f), "They're in the shop now: remove them after they leave.", _small);
            if (_message != null) GUI.Label(new Rect(x, r.yMax - 70f, side, 66f), _message, _small);
        }

        // ---- Records

        private static void DrawRecords(Rect r)
        {
            const int across = 3;
            float cw = r.width / across, ch = r.height / 2f;
            for (int i = 0; i < Whiteboard.Records.Length; i++)
            {
                var b = Whiteboard.Records[i];
                var lines = new List<string> { $"<b>{b.PanelTitle}</b>" };
                var top = b.Top(RecordRows);
                if (top.Count == 0) lines.Add("Nobody yet");
                for (int n = 0; n < top.Count; n++) lines.Add($"{n + 1}. {top[n].name}: {b.Prefix}{b.Value(top[n]):0}");
                float y = r.y + (i / across) * ch;
                foreach (var line in lines)
                {
                    var style = line.StartsWith("<b>") ? _head : _small;
                    float h = style.CalcHeight(new GUIContent(line), cw - 24f);
                    GUI.Label(new Rect(r.x + (i % across) * cw, y, cw - 24f, h), line, style);
                    y += h + 3f;
                }
            }
        }

        // ---- Drama

        private static Vector2 _scrollRight;

        private static void DrawDrama(Rect r)
        {
            var left = new List<string> { "<b>Rivals</b>" };
            var rivals = Cast.AllRegulars.Select(p => (p, rival: Rivalries.RivalName(p))).Where(x => x.rival != null).ToList();
            if (rivals.Count == 0) left.Add("All quiet.");
            foreach (var (p, rival) in rivals) left.Add($"{p.name} has it in for {rival}");
            left.Add("");
            left.Add("<b>Friends</b>");
            var seen = new HashSet<string>();
            foreach (var p in Cast.AllRegulars)
                foreach (var f in Friends.FriendNames(p))
                    if (seen.Add(string.CompareOrdinal(p.name, f) < 0 ? p.name + "|" + f : f + "|" + p.name)) left.Add($"{p.name} and {f}");
            if (seen.Count == 0) left.Add("Nobody has hit it off yet.");
            Lines(new Rect(r.x, r.y, r.width * 0.4f, r.height), left, ref _scroll);

            var right = new List<string> { "<b>Shop gossip</b>" };
            right.AddRange(News.Items.Reverse().Select(News.Describe).Where(s => s != null).Take(NewsRows));
            if (right.Count == 1) right.Add("Nothing has happened yet.");
            Lines(new Rect(r.x + r.width * 0.42f, r.y, r.width * 0.58f, r.height), right, ref _scrollRight);
        }

        // ---- Guide: its text and both perk lists come from defaults.json

        private static int _topic;

        private static void DrawGuide(Rect r)
        {
            var topics = Traits.Guide.Select(t => t.title).Concat(new[] { "Customer perks", "Employee perks" }).ToList();
            const float side = 250f, rowH = 50f;
            for (int i = 0; i < topics.Count; i++)
            {
                GUI.backgroundColor = i == _topic ? new Color(0.35f, 0.55f, 1f) : Color.white;
                if (GUI.Button(new Rect(r.x, r.y + i * rowH, side - 12f, rowH - 6f), topics[i], _button) && _topic != i)
                {
                    _topic = i;
                    _scroll = Vector2.zero;
                }
            }
            GUI.backgroundColor = Color.white;
            _topic = Mathf.Clamp(_topic, 0, topics.Count - 1);
            var lines = new List<string> { $"<b>{topics[_topic]}</b>" };
            int guideCount = Traits.Guide.Count;
            if (_topic < guideCount)
            {
                lines.AddRange(Traits.Guide[_topic].lines.Select(GuideLine));
                if (Traits.Guide[_topic].title == "Loyalty") lines.Add(string.Join(" > ", Traits.LoyaltyTiers.Select(t => t.name)));
            }
            else
            {
                var perks = _topic == guideCount ? Traits.All : Traits.EmployeePerks;
                foreach (var perk in perks) lines.Add($"<color=#A0522D><b>{perk.name}</b></color>   {perk.description}");
            }
            Lines(new Rect(r.x + side, r.y, r.width - side, r.height), lines, ref _scroll);
        }

        private static string GuideLine(string line) => line.Replace("{kick}", Plugin.KickOutKey.Value.ToString());

        // ---- Wall of Shame

        private static void DrawShame(Rect r)
        {
            var list = Whiteboard.Offenders().ToList();
            var lines = new List<string> { "<b>The usual suspects</b>" };
            if (list.Count == 0) lines.Add("A clean record, for now.");
            foreach (var x in list)
            {
                var parts = new List<string>();
                if (x.caught > 0) parts.Add($"caught stealing {x.caught}x");
                if (x.away > 0) parts.Add($"got away {x.away}x");
                if (x.kicks > 0) parts.Add($"kicked out {x.kicks}x");
                if (x.rips > 0) parts.Add($"{x.rips} rip-off(s)");
                bool banned = x.kicks >= KickOut.BanAfter;
                lines.Add($"{x.p.name}: {string.Join(", ", parts)}" + (banned ? "  <color=#C62828><b>BANNED</b></color>" : ""));
            }
            Lines(r, lines, ref _scroll);
        }
    }
}

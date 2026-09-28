using System;
using System.Collections.Generic;
using System.Linq;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Runtime;
using TCGCustomCards.Runtime.Mtg;
using UnityEngine;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// P1 "plain" MTG client (IMGUI, 1920×1080 virtual layout): both boards, hand, stack, the current prompt with Forge's two buttons,
    /// and a generic dialog for Forge's questions. Everything is driven by the bridge messages in <see cref="MtgSession"/>; clicks
    /// go straight back to Forge, which decides what's legal. Replaced by 3D cards on the table in P2.
    /// </summary>
    internal class MtgTableUI : MonoBehaviour
    {
        private static MtgTableUI _inst;
        private const float W = 1920f, H = 1080f;
        private JObject _hover;
        private Vector2 _askScroll, _logScroll;
        private readonly Dictionary<int, JObject> _logCards = new Dictionary<int, JObject>(); // cards named in the log (kept after they leave the state, e.g. dead tokens)
        private float _logContentH;
        private int _logCount = -1;
        private bool _logMouseIn;
        // Declare blockers (Arena-style): drag one of your creatures onto an attacker, or click it and then the attacker.
        // Forge's InputBlock assigns a clicked creature to its "current attacker" (the first one unless an attacker is
        // clicked), so an assignment is sent as two clicks: the attacker, then the blocker.
        private bool _blockStep;
        private readonly HashSet<int> _blockAttackers = new HashSet<int>();
        private int _blockCurrent = -1;       // Forge's current attacker (prompt.picked)
        private int _pendingBlocker = -1;     // clicked creature waiting for its attacker
        private int _dragBlocker = -1;        // creature being dragged
        private Vector2 _dragFrom;
        private bool _dragMoved;
        private const float DragThreshold = 8f;
        private JObject _logIndexedState;
        private readonly Dictionary<string, JObject> _cardsByName = new Dictionary<string, JObject>(StringComparer.Ordinal);
        private readonly Dictionary<string, List<(string text, JObject card)>> _logParsed = new Dictionary<string, List<(string text, JObject card)>>();
        private List<string> _namesLongestFirst;
        private static readonly Color LogLink = new Color(0.62f, 0.83f, 1f);
        private static readonly System.Text.RegularExpressions.Regex CardRef = new System.Text.RegularExpressions.Regex(@" \((\d+)\)");
        private readonly HashSet<int> _askPicked = new HashSet<int>();
        private JToken _askSeen;
        private int[] _amounts;                               // "amounts" dialog values
        private readonly List<int> _order = new List<int>(); // "reorder" dialog: option indexes, top first
        private string _zoneView; // "me:graveyard" etc.
        private bool _confirmConcede;
        private GUIStyle _text, _small, _title, _tile, _button, _big;
        private float _guiScale = 1f;
        private int _lastActive = int.MinValue;
        private float _turnFlashUntil;
        private static readonly Color MyTurn = new Color(0.25f, 0.75f, 0.35f), TheirTurn = new Color(0.85f, 0.3f, 0.3f);
        private Vector2 _guiOffset;
        private static Dictionary<string, (CustomSet set, int pos)> _art;
        private static readonly Color Selectable = new Color(1f, 0.85f, 0.2f), Picked = new Color(0.3f, 1f, 0.4f), Attacking = new Color(1f, 0.35f, 0.3f), Blocking = new Color(0.35f, 0.6f, 1f);

        public static void Show()
        {
            MtgSession.EnsureRunner();
            if (_inst == null)
            {
                var go = GameObject.Find("TCGCC_Mtg");
                _inst = go.GetComponent<MtgTableUI>() ?? go.AddComponent<MtgTableUI>();
            }
            _inst._hover = null;
            _inst._logCards.Clear(); // Forge card ids restart every game
            _inst._cardsByName.Clear();
            _inst._logParsed.Clear();
            _inst._logIndexedState = null;
            _inst._logCount = -1;
            _inst._zoneView = null;
            _inst._confirmConcede = false;
            _inst.enabled = true;
        }

        private void Update()
        {
            if (Input.GetKeyDown(KeyCode.Escape)) { _pendingBlocker = -1; _dragBlocker = -1; }
            if (!Input.GetKeyDown(KeyCode.Space)) return;
            if (MtgSession.Ask != null || _confirmConcede || _zoneView != null || MtgSession.GameOver != null) return;
            var prompt = MtgSession.Prompt;
            string text = S(prompt, "text") ?? "";
            if (text.StartsWith("Waiting for") || S(prompt, "ok") == null) return;
            ForgeBridge.Act("ok"); // same as clicking the OK button
        }

        public static void Hide()
        {
            if (_inst != null) _inst.enabled = false;
        }

        // ------------------------------------------------------------------ helpers

        private static int I(JToken t, string k, int def = 0) => t?[k] != null && t[k].Type == JTokenType.Integer ? (int)t[k] : def;
        private static bool B(JToken t, string k) => t?[k] != null && t[k].Type == JTokenType.Boolean && (bool)t[k];
        private static string S(JToken t, string k) => t?[k] != null && t[k].Type != JTokenType.Null ? (string)t[k] : null;
        private static IEnumerable<JObject> Arr(JToken t, string k) => (t?[k] as JArray)?.OfType<JObject>() ?? Enumerable.Empty<JObject>();
        private static HashSet<int> Ids(JToken t, string k) => new HashSet<int>((t?[k] as JArray)?.Where(x => x.Type == JTokenType.Integer).Select(x => (int)x) ?? Enumerable.Empty<int>());

        /// <summary>Our card image for a Forge card: matched by set code + MTG name, then by name alone.</summary>
        private static Sprite Art(JObject c)
        {
            if (c == null) return null;
            if (_art == null)
            {
                _art = new Dictionary<string, (CustomSet, int)>(StringComparer.OrdinalIgnoreCase);
                foreach (var s in Registry.Sets)
                    for (int i = 0; i < s.Def.Cards.Count; i++)
                    {
                        var m = s.Def.Cards[i].Mtg;
                        if (m?.Name == null) continue;
                        string code = s.Def.Mtg?.SetCode ?? "";
                        if (!_art.ContainsKey(code + "|" + m.Name)) _art[code + "|" + m.Name] = (s, i);
                        if (!_art.ContainsKey("|" + m.Name)) _art["|" + m.Name] = (s, i);
                    }
            }
            foreach (var name in new[] { S(c, "oracleName"), S(c, "name") })
            {
                if (string.IsNullOrEmpty(name)) continue;
                if (_art.TryGetValue((S(c, "set") ?? "") + "|" + name, out var hit) || _art.TryGetValue("|" + name, out hit))
                    return hit.set.CardImage(hit.pos);
            }
            return null;
        }

        /// <summary>
        /// Draws a Forge card as the game renders it (the player's real copy: border, foil, holo) via CardRenderCache; falls back to
        /// the flat set image until that picture is ready. False when there's neither (token/unknown card).
        /// </summary>
        private static bool DrawFace(Rect r, JObject c, bool live)
        {
            if (c == null) return false;
            var data = MtgSession.CardFaces?.For(c, I(MtgSession.State, "me", -1));
            var tex = data == null ? null : live ? CardRenderCache.GetLive(data) : CardRenderCache.Get(data);
            if (tex != null)
            {
                GUI.DrawTexture(r, tex, ScaleMode.ScaleToFit);
                return true;
            }
            var art = Art(c);
            if (art == null) return false;
            var tr = art.textureRect;
            var t = art.texture;
            GUI.DrawTextureWithTexCoords(r, t, new Rect(tr.x / t.width, tr.y / t.height, tr.width / t.width, tr.height / t.height));
            return true;
        }

        private static JObject Me(JObject st) => Arr(st, "players").FirstOrDefault(p => I(p, "id") == I(st, "me"));
        private static JObject Opp(JObject st) => Arr(st, "players").FirstOrDefault(p => I(p, "id") != I(st, "me"));

        private void Styles()
        {
            if (_text != null) return;
            _text = new GUIStyle(GUI.skin.label) { fontSize = 18, wordWrap = true, richText = true };
            _small = new GUIStyle(_text) { fontSize = 14 };
            _title = new GUIStyle(_text) { fontSize = 24, fontStyle = FontStyle.Bold };
            _tile = new GUIStyle(GUI.skin.box) { fontSize = 13, wordWrap = true, alignment = TextAnchor.LowerCenter, richText = true };
            _button = new GUIStyle(GUI.skin.button) { fontSize = 18, wordWrap = true };
            _big = new GUIStyle(GUI.skin.button) { fontSize = 22, fontStyle = FontStyle.Bold };
        }

        // ------------------------------------------------------------------ drawing

        private void OnGUI()
        {
            Styles();
            GUI.depth = -100;
            float scale = Mathf.Min(Screen.width / W, Screen.height / H);
            _guiScale = scale;
            _guiOffset = new Vector2((Screen.width - W * scale) / 2f, (Screen.height - H * scale) / 2f);
            GUI.matrix = Matrix4x4.TRS(new Vector3(_guiOffset.x, _guiOffset.y, 0f), Quaternion.identity, new Vector3(scale, scale, 1f));
            bool board3d = MtgTable3d.Active;
            if (!board3d || MtgSession.State == null)
            {
                GUI.color = new Color(0f, 0f, 0f, board3d ? 0.5f : 0.82f);
                GUI.DrawTexture(new Rect(0, 0, W, H), Texture2D.whiteTexture);
                GUI.color = Color.white;
            }

            var st = MtgSession.State;
            if (st == null)
            {
                GUI.Label(new Rect(0, H / 2 - 40, W, 80), $"<b>{(MtgSession.Status.Length > 0 ? MtgSession.Status : "Loading…")}</b>",
                    new GUIStyle(_title) { alignment = TextAnchor.MiddleCenter });
                return;
            }
            _hover = null;
            int activeId = I(st, "activePlayer", -1);
            if (activeId != _lastActive)
            {
                if (_lastActive != int.MinValue || I(st, "turn") > 0) _turnFlashUntil = Time.unscaledTime + 1.6f;
                _lastActive = activeId;
            }
            var me = Me(st);
            var opp = Opp(st);
            var prompt = MtgSession.Prompt;
            var selectable = Ids(prompt, "selectable");
            var picked = Ids(prompt, "picked");
            var combat = Arr(st, "combat").ToList();

            // IMGUI gives clicks to the first control drawn: keep the board inert while a dialog is open.
            bool modal = MtgSession.Ask != null || _confirmConcede || _zoneView != null;
            UpdateBlockStep(st, prompt, picked, combat, modal);
            GUI.enabled = !modal;
            if (board3d)
            {
                // 3D table: cards are real objects; highlights/labels/click areas over them, everything else in one panel on the
                // right so the table stays clear (the hand lies along the bottom of the view).
                DrawCards3d(selectable, picked);
                DrawHud3d(new Rect(W - 380, 8, 372, H - 16), st, prompt, me, opp, selectable, picked);
            }
            else
            {
                DrawPlayerBar(new Rect(10, 8, 1450, 36), opp, st, selectable, picked);
                DrawBattlefield(new Rect(10, 48, 1450, 300), opp, selectable, picked, combat, top: true);
                DrawMiddle(new Rect(10, 352, 1450, 180), st, prompt);
                DrawBattlefield(new Rect(10, 536, 1450, 300), me, selectable, picked, combat, top: false);
                DrawPlayerBar(new Rect(10, 840, 1450, 36), me, st, selectable, picked);
                DrawRow(new Rect(10, 880, 1450, 192), Arr(me, "hand").ToList(), selectable, picked, combat, 132);
            }
            GUI.enabled = true;

            if (_zoneView != null && MtgSession.Ask == null) DrawZoneView(st, selectable, picked, combat);
            if (MtgSession.Ask != null) DrawAsk(MtgSession.Ask);
            if (_confirmConcede) DrawConcede();
            // Last, so cards hovered inside dialogs show in the preview (and it isn't dimmed); inert while a dialog is open.
            GUI.enabled = !modal;
            if (board3d) DrawFloatingPreview();
            else DrawSide(new Rect(1470, 8, 440, 1064), st);
            GUI.enabled = true;
            if (Time.unscaledTime < _turnFlashUntil && MtgSession.GameOver == null)
            {
                bool mineNow = I(st, "activePlayer") == I(st, "me");
                float a = Mathf.Clamp01((_turnFlashUntil - Time.unscaledTime) / 0.4f); // fade out at the end
                var c = mineNow ? MyTurn : TheirTurn;
                GUI.color = new Color(c.r, c.g, c.b, a);
                GUI.Label(new Rect(0, H / 2 - 70, board3d ? W - 390 : 1460, 140),
                    $"<size=72><b>{(mineNow ? "Your turn" : (S(opp, "name") ?? "Opponent") + "'s turn")}</b></size>",
                    new GUIStyle(_title) { alignment = TextAnchor.MiddleCenter });
                GUI.color = Color.white;
            }
            if (MtgSession.Status.Length > 0 && !MtgSession.Playing || MtgSession.GameOver != null)
                GUI.Label(new Rect(0, H / 2 - 60, 1460, 120), $"<size=64><b>{MtgSession.Status}</b></size>", new GUIStyle(_title) { alignment = TextAnchor.MiddleCenter });
        }

        private void DrawPlayerBar(Rect r, JObject p, JObject st, HashSet<int> selectable, HashSet<int> picked)
        {
            if (p == null) return;
            int id = I(p, "id");
            bool active = I(st, "activePlayer") == id;
            var mana = p["mana"] as JObject;
            string pool = mana == null || !mana.HasValues ? "" : "   Mana: " + string.Join(" ", mana.Properties().Select(x => $"{x.Name}{x.Value}"));
            string label = $"{(active ? "▶ " : "")}<b>{S(p, "name")}</b>   Life <b>{I(p, "life")}</b>   Hand {I(p, "handSize")}   Library {I(p, "library")}{pool}";
            var old = GUI.backgroundColor;
            if (selectable.Contains(id)) GUI.backgroundColor = picked.Contains(id) ? Picked : Selectable;
            if (GUI.Button(new Rect(r.x, r.y, 760, r.height), label, new GUIStyle(_button) { richText = true, alignment = TextAnchor.MiddleLeft }))
                ForgeBridge.Act("player", id);
            GUI.backgroundColor = old;
            float x = r.x + 770;
            foreach (var zone in new[] { "graveyard", "exile" })
            {
                int n = Arr(p, zone).Count();
                string key = (B(p, "ai") ? "opp:" : "me:") + zone;
                if (GUI.Button(new Rect(x, r.y, 170, r.height), $"{char.ToUpper(zone[0])}{zone.Substring(1)} ({n})", _button))
                    _zoneView = _zoneView == key ? null : key;
                x += 180;
            }
            if (!B(p, "ai"))
            {
                if (mana != null)
                    foreach (var prop in mana.Properties())
                        if (GUI.Button(new Rect(x, r.y, 50, r.height), prop.Name, _button)) // spend floating mana while paying
                            ForgeBridge.Send(new JObject { ["t"] = "act", ["a"] = "mana", ["color"] = ManaByte(prop.Name) });
            }
        }

        private static int ManaByte(string c)
        {
            switch (c) { case "W": return 1; case "U": return 2; case "B": return 4; case "R": return 8; case "G": return 16; default: return 32; }
        }

        private void DrawBattlefield(Rect r, JObject p, HashSet<int> selectable, HashSet<int> picked, List<JObject> combat, bool top)
        {
            if (p == null) return;
            var cards = Arr(p, "battlefield").Where(c => S(c, "attachedTo") == null || !B(c, "land")).ToList();
            var lands = cards.Where(c => B(c, "land") && !B(c, "creature")).ToList();
            var other = cards.Where(c => !(B(c, "land") && !B(c, "creature"))).ToList();
            float half = (r.height - 6) / 2f;
            var rowOther = new Rect(r.x, top ? r.y + half + 6 : r.y, r.width, half);
            var rowLands = new Rect(r.x, top ? r.y : r.y + half + 6, r.width, half);
            DrawRow(rowOther, other, selectable, picked, combat, 104);
            DrawRow(rowLands, lands, selectable, picked, combat, 104);
        }

        private void DrawRow(Rect r, List<JObject> cards, HashSet<int> selectable, HashSet<int> picked, List<JObject> combat, float maxW)
        {
            if (cards.Count == 0) return;
            float w = Mathf.Min(maxW, (r.width - 4 * (cards.Count - 1)) / cards.Count);
            float h = Mathf.Min(r.height, w * 1.4f);
            for (int i = 0; i < cards.Count; i++)
                DrawCard(new Rect(r.x + i * (w + 4), r.y + (r.height - h) / 2f, w, h), cards[i], selectable, picked, combat);
        }

        private void DrawCard(Rect r, JObject c, HashSet<int> selectable, HashSet<int> picked, List<JObject> combat)
        {
            int id = I(c, "id");
            bool tapped = B(c, "tapped");
            var art = Art(c);
            var old = GUI.color;
            if (selectable.Contains(id) || picked.Contains(id) || B(c, "attacking") || B(c, "blocking"))
            {
                GUI.color = picked.Contains(id) ? Picked : selectable.Contains(id) ? Selectable : Attacking;
                GUI.DrawTexture(new Rect(r.x - 3, r.y - 3, r.width + 6, r.height + 6), Texture2D.whiteTexture);
            }
            GUI.color = tapped ? new Color(0.55f, 0.55f, 0.55f) : Color.white;
            if (!DrawFace(r, c, live: false)) GUI.Box(r, "", _tile);
            GUI.color = old;

            var lines = new List<string>();
            if (art == null) lines.Add($"<b>{S(c, "name")}</b>");
            if (c["power"] != null) lines.Add($"<b>{I(c, "power")}/{I(c, "toughness")}</b>" + (I(c, "damage") > 0 ? $" <color=#ff6060>-{I(c, "damage")}</color>" : ""));
            if (S(c, "loyalty") != null) lines.Add($"Loyalty {S(c, "loyalty")}");
            if (c["counters"] is JObject cnt) foreach (var p in cnt.Properties()) lines.Add($"{p.Value}× {p.Name}");
            if (tapped) lines.Add("<color=#aaaaaa>tapped</color>");
            if (B(c, "sick") && B(c, "creature") && S(c, "zone") == "Battlefield") lines.Add("<color=#aaaaaa>new</color>");
            if (B(c, "attacking")) lines.Add("<color=#ff8080>attacking</color>");
            if (B(c, "blocking")) lines.Add("<color=#80c0ff>blocking</color>");
            if (lines.Count > 0)
            {
                float lh = 17 * lines.Count + 4;
                GUI.color = new Color(0, 0, 0, art != null ? 0.65f : 0f);
                GUI.DrawTexture(new Rect(r.x, r.yMax - lh, r.width, lh), Texture2D.whiteTexture);
                GUI.color = old;
                GUI.Label(new Rect(r.x + 2, r.yMax - lh, r.width - 4, lh), string.Join("\n", lines),
                    new GUIStyle(_small) { fontSize = 13, alignment = TextAnchor.LowerCenter });
            }
            if (r.Contains(Event.current.mousePosition)) _hover = c;
            if (GUI.Button(r, "", GUIStyle.none)) ForgeBridge.Act("card", id);
        }

        /// <summary>Highlights, stat labels and invisible click/hover areas over the 3D cards (screen rects from MtgTable3d).</summary>
        /// <summary>A card on the table as seen on screen, in layout space (for frames and hit tests).</summary>
        private sealed class Shown
        {
            public MtgTable3d.Entry E;
            public JObject C;
            public int Id;
            public Rect R;
            public readonly Vector2[] Quad = new Vector2[4];
            public bool Contains(Vector2 p) => E.HasQuad ? InQuad(Quad, p) : R.Contains(p);
        }

        private void DrawCards3d(HashSet<int> selectable, HashSet<int> picked)
        {
            DrawBlockLines();
            // Nearest card first: the card in front wins where cards overlap.
            var shown = new List<Shown>();
            foreach (var e in MtgTable3d.Visible.OrderBy(v => v.Depth))
            {
                if (e.Json == null) continue; // face-down: not interactive
                var sr = e.ScreenRect;
                var s = new Shown { E = e, C = e.Json, Id = I(e.Json, "id"),
                    R = new Rect((sr.x - _guiOffset.x) / _guiScale, (sr.y - _guiOffset.y) / _guiScale, sr.width / _guiScale, sr.height / _guiScale) };
                // The card's real outline on screen (tapped/tilted cards aren't boxes).
                for (int q = 0; q < 4; q++) s.Quad[q] = (e.Quad[q] - _guiOffset) / _guiScale;
                shown.Add(s);
            }
            var ev = Event.current;
            bool overHud = ev.mousePosition.x >= W - 380; // the right-hand panel sits over the table: it gets its own clicks
            var hit = overHud ? null : shown.FirstOrDefault(s => s.Contains(ev.mousePosition));
            int meId = I(MtgSession.State, "me", -1);
            bool choosing = _blockStep && (_pendingBlocker >= 0 || (_dragBlocker >= 0 && _dragMoved));

            foreach (var s in shown)
            {
                var e = s.E;
                var c = s.C;
                var r = s.R;
                var quad = s.Quad;
                int id = s.Id;
                var old = GUI.color;
                // Covered cards (e.g. an Aura under its creature) still get the frame when they can be picked, so targets stay visible.
                Color? frame = null;
                float width = 4f;
                if (_blockStep)
                {
                    if (id == _pendingBlocker || id == _dragBlocker) { frame = Color.white; width = 6f; }
                    else if (_blockAttackers.Contains(id))
                    {
                        bool target = choosing && hit == s;
                        frame = target ? Color.white : choosing ? Selectable : id == _blockCurrent ? Picked : Attacking;
                        if (target) width = 7f;
                    }
                    else if (CanBlockNow(c, meId)) frame = Selectable;
                    else if (!e.Covered && B(c, "blocking")) frame = Blocking;
                }
                else if (selectable.Contains(id) || picked.Contains(id) || (!e.Covered && (B(c, "attacking") || B(c, "blocking"))))
                    frame = picked.Contains(id) ? Picked : selectable.Contains(id) ? Selectable : B(c, "blocking") ? Blocking : Attacking;
                if (frame is Color col)
                {
                    if (e.HasQuad)
                        for (int q = 0; q < 4; q++) DrawLine(quad[q], quad[(q + 1) % 4], col, width);
                    else
                    {
                        GUI.color = col;
                        float t = width;
                        GUI.DrawTexture(new Rect(r.x - t, r.y - t, r.width + 2 * t, t), Texture2D.whiteTexture);
                        GUI.DrawTexture(new Rect(r.x - t, r.yMax, r.width + 2 * t, t), Texture2D.whiteTexture);
                        GUI.DrawTexture(new Rect(r.x - t, r.y, t, r.height), Texture2D.whiteTexture);
                        GUI.DrawTexture(new Rect(r.xMax, r.y, t, r.height), Texture2D.whiteTexture);
                        GUI.color = old;
                    }
                }
                // The attacker a plain click on one of your creatures would block (Forge's current attacker).
                if (_blockStep && !choosing && id == _blockCurrent && _blockAttackers.Count > 1 && !e.Covered)
                {
                    var tr = new Rect(r.center.x - 48f, r.y + 4f, 96f, 20f);
                    GUI.color = new Color(0, 0, 0, 0.7f);
                    GUI.DrawTexture(tr, Texture2D.whiteTexture);
                    GUI.color = old;
                    GUI.Label(tr, "<color=#7dff8c>Selected</color>", new GUIStyle(_small) { fontSize = 13, alignment = TextAnchor.MiddleCenter });
                }
                var lines = new List<string>();
                if (c["power"] != null) lines.Add($"<b>{I(c, "power")}/{I(c, "toughness")}</b>" + (I(c, "damage") > 0 ? $" <color=#ff6060>-{I(c, "damage")}</color>" : ""));
                if (S(c, "loyalty") != null) lines.Add($"Loyalty {S(c, "loyalty")}");
                if (c["counters"] is JObject cnt) foreach (var p in cnt.Properties()) lines.Add($"{p.Value}× {p.Name}");
                if (MtgTable3d.IsUnknown(e)) lines.Insert(0, $"<b>{S(c, "name")}</b>");
                // Tags only for permanents (battlefield): not when hidden behind a nearer card, not in the hand or on the stack
                // (the popped-up card being played) — their printed stats are enough.
                if (lines.Count > 0 && !e.Covered && S(c, "zone") == "Battlefield")
                {
                    float lh = 16 * lines.Count + 4;
                    // About half the card's real width (its short side on screen), centred near the bottom of the card.
                    float cardW = e.HasQuad ? Mathf.Min((quad[0] - quad[3]).magnitude, (quad[0] - quad[1]).magnitude) : Mathf.Min(r.width, r.height);
                    float lw = Mathf.Max(56f, cardW * 0.5f);
                    var lr = new Rect(r.center.x - lw / 2f, r.yMax - lh - 4f, lw, lh);
                    GUI.color = new Color(0, 0, 0, 0.65f);
                    GUI.DrawTexture(lr, Texture2D.whiteTexture);
                    GUI.color = old;
                    GUI.Label(lr, string.Join("\n", lines), new GUIStyle(_small) { fontSize = 13, alignment = TextAnchor.LowerCenter });
                }
            }

            // Drag arrow: from the creature being dragged to the mouse.
            if (_dragBlocker >= 0 && _dragMoved && ev.type == EventType.Repaint)
            {
                var from = shown.FirstOrDefault(s => s.Id == _dragBlocker);
                if (from != null)
                {
                    var a = from.R.center;
                    var b = ev.mousePosition;
                    var col = hit != null && _blockAttackers.Contains(hit.Id) ? Color.white : Selectable;
                    DrawLine(a, b, col, 6f);
                    var dir = (b - a).sqrMagnitude > 1f ? (b - a).normalized : Vector2.up;
                    var side = new Vector2(-dir.y, dir.x);
                    DrawLine(b, b - dir * 22f + side * 12f, col, 6f);
                    DrawLine(b, b - dir * 22f - side * 12f, col, 6f);
                }
            }

            // Hover/click on the card's real shape (front card wins); a handled mouse event is used up.
            if (_hover == null && hit != null) _hover = hit.C;
            if (!GUI.enabled) return;
            if (_blockStep) HandleBlockInput(ev, hit, meId);
            else if (hit != null && ev.type == EventType.MouseDown && ev.button == 0)
            {
                Click(hit.C);
                ev.Use();
            }
        }

        private static void Click(JObject c)
        {
            Plugin.Log.LogInfo($"MTG click: {S(c, "name")} ({I(c, "id")}, {S(c, "zone")})");
            ForgeBridge.Act("card", I(c, "id"));
        }

        // ------------------------------------------------------------------ declare blockers

        /// <summary>
        /// Whether you're declaring blockers now: the opponent's declare-blockers step, a combat in progress, our turn to act
        /// and no dialog open. Also tracks Forge's current attacker and drops a pending/dragged blocker when the step ends.
        /// </summary>
        private void UpdateBlockStep(JObject st, JObject prompt, HashSet<int> picked, List<JObject> combat, bool modal)
        {
            string text = S(prompt, "text") ?? "";
            _blockStep = !modal && S(st, "phase") == "COMBAT_DECLARE_BLOCKERS" && I(st, "activePlayer", -1) != I(st, "me", -1)
                         && combat.Count > 0 && prompt != null && !text.StartsWith("Waiting for");
            _blockAttackers.Clear();
            _blockCurrent = -1;
            if (!_blockStep)
            {
                _pendingBlocker = -1;
                _dragBlocker = -1;
                return;
            }
            foreach (var cb in combat) _blockAttackers.Add(I(cb, "attacker", -1));
            _blockCurrent = picked.FirstOrDefault(_blockAttackers.Contains);
            if (!_blockAttackers.Contains(_blockCurrent)) _blockCurrent = -1;
        }

        /// <summary>One of my untapped creatures on the battlefield that isn't blocking yet (Forge still checks legality).</summary>
        private static bool CanBlockNow(JObject c, int meId) =>
            c != null && I(c, "controller", -1) == meId && S(c, "zone") == "Battlefield" && B(c, "creature") &&
            !B(c, "tapped") && !B(c, "blocking") && !B(c, "attacking");

        private void HandleBlockInput(Event ev, Shown hit, int meId)
        {
            switch (ev.type)
            {
                case EventType.MouseDown when ev.button == 1:
                    _pendingBlocker = -1;
                    _dragBlocker = -1;
                    ev.Use();
                    break;
                case EventType.MouseDown when ev.button == 0 && hit != null:
                    if (CanBlockNow(hit.C, meId))
                    {
                        _dragBlocker = hit.Id; // a drag or a click: decided on release
                        _dragFrom = ev.mousePosition;
                        _dragMoved = false;
                    }
                    else BlockClick(hit.C, meId);
                    ev.Use();
                    break;
                case EventType.MouseDrag when _dragBlocker >= 0:
                    if ((ev.mousePosition - _dragFrom).magnitude > DragThreshold) _dragMoved = true;
                    ev.Use();
                    break;
                case EventType.MouseUp when ev.button == 0 && _dragBlocker >= 0:
                    int blocker = _dragBlocker;
                    _dragBlocker = -1;
                    if (_dragMoved)
                    {
                        if (hit != null && _blockAttackers.Contains(hit.Id)) AssignBlock(hit.Id, blocker);
                        // dropped anywhere else: cancelled
                    }
                    else
                    {
                        var c = MtgTable3d.Visible.FirstOrDefault(v => v.Json != null && I(v.Json, "id") == blocker)?.Json;
                        if (c != null) BlockClick(c, meId);
                    }
                    ev.Use();
                    break;
            }
        }

        /// <summary>
        /// A click while declaring blockers. My free creature: with one attacker it blocks it straight away; with several it
        /// waits for a click on the attacker (click it again to cancel). An attacker with a creature waiting: that pair is
        /// sent. Anything else goes to Forge as usual: an attacker becomes Forge's current attacker, a blocking creature
        /// is taken back.
        /// </summary>
        private void BlockClick(JObject c, int meId)
        {
            int id = I(c, "id");
            if (CanBlockNow(c, meId) && _blockAttackers.Count > 1)
            {
                _pendingBlocker = _pendingBlocker == id ? -1 : id;
                return;
            }
            if (_blockAttackers.Contains(id) && _pendingBlocker >= 0)
            {
                AssignBlock(id, _pendingBlocker);
                _pendingBlocker = -1;
                return;
            }
            _pendingBlocker = -1;
            Click(c);
        }

        /// <summary>Forge's order: make the attacker current, then click the blocker (the bridge handles messages in order).</summary>
        private static void AssignBlock(int attacker, int blocker)
        {
            Plugin.Log.LogInfo($"MTG block: {blocker} blocks {attacker}");
            ForgeBridge.Act("card", attacker);
            ForgeBridge.Act("card", blocker);
        }

        /// <summary>A line from each blocker to the attacker it blocks (under the card overlays).</summary>
        /// <summary>Point inside a convex quad (corners in order, either winding).</summary>
        private static bool InQuad(Vector2[] q, Vector2 p)
        {
            float sign = 0f;
            for (int i = 0; i < 4; i++)
            {
                var a = q[i];
                var b = q[(i + 1) % 4];
                float cross = (b.x - a.x) * (p.y - a.y) - (b.y - a.y) * (p.x - a.x);
                if (Mathf.Abs(cross) < 1e-3f) continue;
                if (sign == 0f) sign = Mathf.Sign(cross);
                else if (Mathf.Sign(cross) != sign) return false;
            }
            return true;
        }

        private void DrawBlockLines()
        {
            var st = MtgSession.State;
            var combat = Arr(st, "combat").ToList();
            if (combat.Count == 0) return;
            var rects = new Dictionary<int, Rect>();
            foreach (var e in MtgTable3d.Visible)
                if (e.Json != null)
                {
                    var sr = e.ScreenRect;
                    rects[I(e.Json, "id")] = new Rect((sr.x - _guiOffset.x) / _guiScale, (sr.y - _guiOffset.y) / _guiScale, sr.width / _guiScale, sr.height / _guiScale);
                }
            foreach (var cb in combat)
            {
                if (!rects.TryGetValue(I(cb, "attacker", -1), out var ar)) continue;
                foreach (var b in (cb["blockers"] as JArray)?.Where(t => t.Type == JTokenType.Integer) ?? Enumerable.Empty<JToken>())
                    if (rects.TryGetValue((int)b, out var br)) DrawLine(br.center, ar.center, Blocking, 5f);
            }
        }

        /// <summary>
        /// Line between two points given in the 1920×1080 layout space. Drawn in real screen pixels: RotateAroundPivot takes its
        /// pivot in screen space, so rotating under our scaled GUI.matrix put lines in the wrong place at other resolutions.
        /// </summary>
        private void DrawLine(Vector2 a, Vector2 b, Color color, float width)
        {
            var saved = GUI.matrix;
            var old = GUI.color;
            Vector2 sa = a * _guiScale + _guiOffset, sb = b * _guiScale + _guiOffset;
            float w = width * _guiScale;
            GUI.matrix = Matrix4x4.identity;
            GUI.color = color;
            float angle = Mathf.Atan2(sb.y - sa.y, sb.x - sa.x) * Mathf.Rad2Deg;
            GUIUtility.RotateAroundPivot(angle, sa);
            GUI.DrawTexture(new Rect(sa.x, sa.y - w / 2f, (sb - sa).magnitude, w), Texture2D.whiteTexture);
            GUI.matrix = saved;
            GUI.color = old;
        }

        private void DrawMiddle(Rect r, JObject st, JObject prompt, bool translucent = false)
        {
            if (translucent)
            {
                GUI.color = new Color(0f, 0f, 0f, 0.7f);
                GUI.DrawTexture(r, Texture2D.whiteTexture);
                GUI.color = Color.white;
            }
            else GUI.Box(r, "");
            // Stack (left)
            var stack = Arr(st, "stack").ToList();
            GUI.Label(new Rect(r.x + 8, r.y + 4, 420, 24), $"<b>Stack</b> ({stack.Count})", _text);
            for (int i = 0; i < Math.Min(stack.Count, 5); i++)
            {
                var e = stack[i];
                var rr = new Rect(r.x + 8, r.y + 30 + i * 29, 420, 27);
                if (GUI.Button(rr, S(e, "text") ?? "", new GUIStyle(_small) { normal = { textColor = Color.white }, wordWrap = false, clipping = TextClipping.Clip }) && e["card"] is JObject sc)
                    ForgeBridge.Act("card", I(sc, "id"));
                if (rr.Contains(Event.current.mousePosition) && e["card"] is JObject hc) _hover = hc;
            }
            // Prompt (centre)
            string phase = $"Turn {I(st, "turn")} — {Pretty(S(st, "phase"))}";
            GUI.Label(new Rect(r.x + 440, r.y + 4, 700, 26), $"<b>{phase}</b>", _text);
            string text = S(prompt, "text") ?? "";
            bool waiting = text.StartsWith("Waiting for");
            GUI.Label(new Rect(r.x + 440, r.y + 32, 700, 140), text, _text);
            if (!waiting)
            {
                string ok = S(prompt, "ok"), cancel = S(prompt, "cancel");
                if (ok != null && GUI.Button(new Rect(r.x + 1150, r.y + 20, 290, 64), ok, _big)) ForgeBridge.Act("ok");
                if (cancel != null && GUI.Button(new Rect(r.x + 1150, r.y + 96, 290, 64), cancel, _big)) ForgeBridge.Act("cancel");
            }
            foreach (var msg in MtgSession.Messages.Skip(Math.Max(0, MtgSession.Messages.Count - 1)))
                GUI.Label(new Rect(r.x + 440, r.yMax - 26, 700, 24), $"<color=#ffcc66>{msg}</color>", _small);
        }

        /// <summary>3D mode HUD: opponent, prompt + Forge's buttons, stack, log, you, concede — one column on the right.</summary>
        private void DrawHud3d(Rect r, JObject st, JObject prompt, JObject me, JObject opp, HashSet<int> selectable, HashSet<int> picked)
        {
            GUI.color = new Color(0f, 0f, 0f, 0.72f);
            GUI.DrawTexture(r, Texture2D.whiteTexture);
            GUI.color = Color.white;
            float x = r.x + 8, w = r.width - 16, y = r.y + 6;

            y = PlayerBox(new Rect(x, y, w, 96), opp, st, selectable, picked) + 8;

            // Whose turn (coloured banner) + phase
            bool myTurn = I(st, "activePlayer") == I(st, "me");
            GUI.color = myTurn ? MyTurn : TheirTurn;
            GUI.DrawTexture(new Rect(x, y, w, 34), Texture2D.whiteTexture);
            GUI.color = Color.white;
            GUI.Label(new Rect(x, y, w, 34), $"<b>{(myTurn ? "YOUR TURN" : ((S(opp, "name") ?? "Opponent") + "'s turn").ToUpper())}</b>",
                new GUIStyle(_text) { alignment = TextAnchor.MiddleCenter, fontSize = 20 });
            y += 36;
            GUI.Label(new Rect(x, y, w, 24), $"Turn {I(st, "turn")} — {Pretty(S(st, "phase"))}", _text);
            y += 26;
            string text = S(prompt, "text") ?? "";
            bool waiting = text.StartsWith("Waiting for");
            if (_blockStep)
            {
                // Forge's own text ("Select creatures to block X or select another attacker…") doesn't fit how the table works.
                string pending = _pendingBlocker < 0 ? null
                    : S(MtgTable3d.Visible.FirstOrDefault(v => v.Json != null && I(v.Json, "id") == _pendingBlocker)?.Json, "name");
                text = pending != null
                    ? $"<b>{pending}</b>: now click the attacker it should block.\n<color=#aaaaaa>Right-click or Esc to cancel.</color>"
                    : _blockAttackers.Count > 1
                        ? "Declare blockers: <b>drag</b> one of your creatures onto the attacker it should block (or click it, then the attacker)." +
                          "\n<color=#aaaaaa>Click a blocker again to take it back. OK when done.</color>"
                        : "Declare blockers: click (or drag) your creatures to block the attacker." +
                          "\n<color=#aaaaaa>Click a blocker again to take it back. OK when done.</color>";
            }
            GUI.Label(new Rect(x, y, w, 150), text, _small);
            y += 154;
            string ok = waiting ? null : S(prompt, "ok"), cancel = waiting ? null : S(prompt, "cancel");
            if (ok != null && GUI.Button(new Rect(x, y, w, 56), ok, _big)) ForgeBridge.Act("ok");
            if (cancel != null && GUI.Button(new Rect(x, y + 62, w, 56), cancel, _big)) ForgeBridge.Act("cancel");
            y += 126;
            foreach (var msg in MtgSession.Messages.Skip(Math.Max(0, MtgSession.Messages.Count - 1)))
                GUI.Label(new Rect(x, y, w, 40), $"<color=#ffcc66>{msg}</color>", _small);
            y += 42;

            // Stack
            var stack = Arr(st, "stack").ToList();
            GUI.Label(new Rect(x, y, w, 24), $"<b>Stack</b> ({stack.Count})", _text);
            y += 26;
            for (int i = 0; i < Math.Min(stack.Count, 4); i++)
            {
                var e = stack[i];
                var rr = new Rect(x, y, w, 24);
                if (GUI.Button(rr, S(e, "text") ?? "", new GUIStyle(_small) { normal = { textColor = Color.white }, wordWrap = false, clipping = TextClipping.Clip }) && e["card"] is JObject sc)
                    ForgeBridge.Act("card", I(sc, "id"));
                if (rr.Contains(Event.current.mousePosition) && e["card"] is JObject hc) _hover = hc;
                y += 25;
            }
            y = Math.Max(y, r.y + 600);

            // Log (fills the space down to the player's box)
            float meTop = r.yMax - 96 - 48;
            DrawLog(new Rect(x, y, w, meTop - y - 8), st, 14);

            PlayerBox(new Rect(x, meTop, w, 96), me, st, selectable, picked);
            if (GUI.Button(new Rect(x, r.yMax - 44, w, 38), "Concede", _button)) _confirmConcede = true;
        }

        /// <summary>Compact player box: name/life button (click to target the player), counts, graveyard/exile viewers, mana.</summary>
        private float PlayerBox(Rect r, JObject p, JObject st, HashSet<int> selectable, HashSet<int> picked)
        {
            if (p == null) return r.yMax;
            int id = I(p, "id");
            bool active = I(st, "activePlayer") == id;
            if (active)
            {
                GUI.color = id == I(st, "me") ? MyTurn : TheirTurn;
                const float t = 3f;
                GUI.DrawTexture(new Rect(r.x - t, r.y - t, r.width + 2 * t, t), Texture2D.whiteTexture);
                GUI.DrawTexture(new Rect(r.x - t, r.yMax, r.width + 2 * t, t), Texture2D.whiteTexture);
                GUI.DrawTexture(new Rect(r.x - t, r.y, t, r.height), Texture2D.whiteTexture);
                GUI.DrawTexture(new Rect(r.xMax, r.y, t, r.height), Texture2D.whiteTexture);
                GUI.color = Color.white;
            }
            var old = GUI.backgroundColor;
            if (selectable.Contains(id)) GUI.backgroundColor = picked.Contains(id) ? Picked : Selectable;
            if (GUI.Button(new Rect(r.x, r.y, r.width, 34), $"{(active ? "▶ " : "")}<b>{S(p, "name")}</b>   Life <b>{I(p, "life")}</b>",
                    new GUIStyle(_button) { richText = true, alignment = TextAnchor.MiddleLeft }))
                ForgeBridge.Act("player", id);
            GUI.backgroundColor = old;
            var mana = p["mana"] as JObject;
            string pool = mana == null || !mana.HasValues ? "" : "   Mana " + string.Join(" ", mana.Properties().Select(m => $"{m.Name}{m.Value}"));
            GUI.Label(new Rect(r.x + 4, r.y + 36, r.width - 8, 22), $"Hand {I(p, "handSize")}   Library {I(p, "library")}{pool}", _small);
            float bx = r.x;
            foreach (var zone in new[] { "graveyard", "exile" })
            {
                int n = Arr(p, zone).Count();
                string key = (B(p, "ai") ? "opp:" : "me:") + zone;
                if (GUI.Button(new Rect(bx, r.y + 60, r.width / 2 - 4, 32), $"{char.ToUpper(zone[0])}{zone.Substring(1)} ({n})", _button))
                    _zoneView = _zoneView == key ? null : key;
                bx += r.width / 2 + 4;
            }
            return r.yMax;
        }

        /// <summary>Large preview of the hovered card, beside the cursor (3D mode).</summary>
        private void DrawFloatingPreview()
        {
            var c = _hover;
            if (c == null) return;
            var art = Art(c);
            float pw = 330, ph = pw * 1.4f;
            var mouse = Event.current.mousePosition;
            float px = mouse.x + 30 + pw > W - 390 ? mouse.x - 30 - pw : mouse.x + 30;
            float py = Mathf.Clamp(mouse.y - ph / 2, 8, H - ph - 8);
            var rect = new Rect(px, py, pw, ph);
            if (DrawFace(rect, c, live: true)) { }
            else
            {
                GUI.color = new Color(0.1f, 0.1f, 0.1f, 0.95f);
                GUI.DrawTexture(rect, Texture2D.whiteTexture);
                GUI.color = Color.white;
                GUI.Label(new Rect(rect.x + 12, rect.y + 10, pw - 24, ph - 20),
                    $"<b><size=22>{S(c, "name")}</size></b>   {S(c, "cost")}\n<i>{S(c, "type")}</i>\n\n{S(c, "text")}" +
                    (c["power"] != null ? $"\n\n<b>{I(c, "power")}/{I(c, "toughness")}</b>" : ""), _text);
            }
        }

        private static string Pretty(string phase)
        {
            if (string.IsNullOrEmpty(phase)) return "";
            switch (phase)
            {
                case "MAIN1": return "Main phase 1";
                case "MAIN2": return "Main phase 2";
                case "COMBAT_DECLARE_ATTACKERS": return "Declare attackers";
                case "COMBAT_DECLARE_BLOCKERS": return "Declare blockers";
                case "END_OF_TURN": return "End step";
                default: return char.ToUpper(phase[0]) + phase.Substring(1).ToLower().Replace('_', ' ');
            }
        }

        private void DrawSide(Rect r, JObject st)
        {
            var c = _hover;
            var art = Art(c);
            float imgW = r.width, imgH = imgW * 1.4f;
            if (c != null)
            {
                if (DrawFace(new Rect(r.x, r.y, imgW, imgH), c, live: true)) { }
                else
                {
                    GUI.Box(new Rect(r.x, r.y, imgW, imgH), "");
                    GUI.Label(new Rect(r.x + 12, r.y + 10, imgW - 24, imgH - 20),
                        $"<b><size=22>{S(c, "name")}</size></b>   {S(c, "cost")}\n<i>{S(c, "type")}</i>\n\n{S(c, "text")}" +
                        (c["power"] != null ? $"\n\n<b>{I(c, "power")}/{I(c, "toughness")}</b>" : ""), _text);
                }
            }
            if (GUI.Button(new Rect(r.x, r.y + imgH + 8, imgW, 40), "Concede", _button)) _confirmConcede = true;
            DrawLog(new Rect(r.x, r.y + imgH + 56, imgW, r.height - imgH - 56), st, 14);
        }

        /// <summary>
        /// Game log, word-wrapped, newest at the bottom (follows new lines unless scrolled up). Card names (Forge writes them as
        /// "Name (id)") are drawn underlined without the id; hovering one shows the big card preview. Card data comes from the
        /// bridge's <c>logCards</c>, remembered per game so cards that have since left play still preview.
        /// </summary>
        private void DrawLog(Rect box, JObject st, int fontSize)
        {
            if (!ReferenceEquals(st, _logIndexedState))
            {
                _logIndexedState = st;
                if (st["logCards"] is JObject lc)
                    foreach (var p in lc.Properties())
                        if (int.TryParse(p.Name, out int cid) && p.Value is JObject cj) _logCards[cid] = cj;
                // Every card the table shows (mine + public zones + stack) is remembered by name too: most log lines name
                // cards without an id ("Customer cast Thallid Soothsayer").
                foreach (var pl in Arr(st, "players"))
                    foreach (var zone in new[] { "battlefield", "graveyard", "exile", "command", "hand" })
                        foreach (var c in Arr(pl, zone)) Remember(c);
                foreach (var s in Arr(st, "stack")) Remember(s["card"] as JObject);
                foreach (var c in _logCards.Values) Remember(c);
            }
            var log = (st["log"] as JArray)?.Select(t => (string)t ?? "").ToList() ?? new List<string>();
            var style = new GUIStyle(_small) { fontSize = fontSize, wordWrap = false, richText = false, padding = new RectOffset(), margin = new RectOffset() };
            float lineH = style.CalcSize(new GUIContent("Ag")).y + 1;
            float innerW = box.width - 30;

            GUI.Box(box, "");
            _logMouseIn = box.Contains(Event.current.mousePosition); // lines scrolled out of the box aren't hoverable
            if (_logCount != log.Count || _logScroll.y >= _logContentH - box.height - 4) _logScroll.y = float.MaxValue; // follow the newest line
            _logScroll = GUI.BeginScrollView(box, _logScroll, new Rect(0, 0, box.width - 20, Mathf.Max(_logContentH, box.height)));
            float y = 4;
            for (int i = 0; i < log.Count; i++)
            {
                if (i > 0)
                {
                    GUI.color = new Color(1f, 1f, 1f, 0.12f);
                    GUI.DrawTexture(new Rect(6, y - 3, innerW, 1), Texture2D.whiteTexture);
                    GUI.color = Color.white;
                }
                y = DrawLogEntry(log[i], 6, y, innerW, style, lineH) + 6;
            }
            GUI.EndScrollView();
            _logContentH = y;
            _logCount = log.Count;
        }

        private void Remember(JObject c)
        {
            string name = S(c, "name");
            if (string.IsNullOrEmpty(name) || B(c, "faceDown")) return;
            if (!_cardsByName.TryGetValue(name, out var had) || !JToken.DeepEquals(had, c))
            {
                _cardsByName[name] = c;
                _logParsed.Clear(); // names changed: re-split the lines
            }
        }

        /// <summary>
        /// Splits a log line into plain text and card links: "Name (id)" references first (exact card), then any other known
        /// card name as a whole word (longest names first, so "Island" doesn't cut into a longer name). Cached per line.
        /// </summary>
        private List<(string text, JObject card)> ParseLogLine(string msg)
        {
            if (_logParsed.TryGetValue(msg, out var cached)) return cached;
            var parts = new List<(string text, JObject card)>();
            int pos = 0;
            foreach (System.Text.RegularExpressions.Match m in CardRef.Matches(msg))
            {
                if (!_logCards.TryGetValue(int.Parse(m.Groups[1].Value), out var card)) continue;
                string name = new[] { S(card, "name"), S(card, "oracleName") }
                    .FirstOrDefault(n => !string.IsNullOrEmpty(n) && m.Index - n.Length >= pos && string.CompareOrdinal(msg, m.Index - n.Length, n, 0, n.Length) == 0);
                if (name == null) continue;
                if (m.Index - name.Length > pos) parts.AddRange(LinkNames(msg.Substring(pos, m.Index - name.Length - pos)));
                parts.Add((name, card));
                pos = m.Index + m.Length;
            }
            if (pos < msg.Length) parts.AddRange(LinkNames(msg.Substring(pos)));
            _logParsed[msg] = parts;
            return parts;
        }

        private List<(string text, JObject card)> LinkNames(string text)
        {
            var parts = new List<(string, JObject)>();
            if (_namesLongestFirst == null || _namesLongestFirst.Count != _cardsByName.Count)
                _namesLongestFirst = _cardsByName.Keys.OrderByDescending(n => n.Length).ToList();
            // Mark claimed character ranges, longest names first.
            var hits = new List<(int start, int len, JObject card)>();
            var taken = new bool[text.Length];
            foreach (var name in _namesLongestFirst)
            {
                int i = 0;
                while ((i = text.IndexOf(name, i, StringComparison.Ordinal)) >= 0)
                {
                    int end = i + name.Length;
                    bool word = (i == 0 || !char.IsLetterOrDigit(text[i - 1])) && (end == text.Length || !char.IsLetterOrDigit(text[end]));
                    bool free = word;
                    for (int k = i; free && k < end; k++) free = !taken[k];
                    if (free)
                    {
                        for (int k = i; k < end; k++) taken[k] = true;
                        hits.Add((i, name.Length, _cardsByName[name]));
                    }
                    i = end;
                }
            }
            int pos = 0;
            foreach (var h in hits.OrderBy(h => h.start))
            {
                if (h.start > pos) parts.Add((text.Substring(pos, h.start - pos), null));
                parts.Add((text.Substring(h.start, h.len), h.card));
                pos = h.start + h.len;
            }
            if (pos < text.Length) parts.Add((text.Substring(pos), null));
            return parts;
        }

        /// <summary>Draws one log line with word wrap from (x0, y); returns the y below it.</summary>
        private float DrawLogEntry(string msg, float x0, float y, float width, GUIStyle style, float lineH)
        {
            var parts = ParseLogLine(msg);
            float x = x0, right = x0 + width;
            var mouse = Event.current.mousePosition;
            foreach (var (text, card) in parts)
            {
                var tokens = card != null ? new[] { text } : System.Text.RegularExpressions.Regex.Split(text, @"(\s+)");
                foreach (var tok in tokens)
                {
                    if (tok.Length == 0) continue;
                    bool space = card == null && string.IsNullOrWhiteSpace(tok);
                    if (space && x <= x0) continue; // no leading spaces on a wrapped line
                    float w = style.CalcSize(new GUIContent(tok)).x;
                    if (!space && x > x0 && x + w > right) { x = x0; y += lineH; }
                    var rect = new Rect(x, y, Mathf.Min(w, right - x), lineH);
                    if (card == null) GUI.Label(rect, tok, style);
                    else
                    {
                        bool hot = _logMouseIn && rect.Contains(mouse);
                        if (hot) _hover = card;
                        var old = style.normal.textColor;
                        style.normal.textColor = hot ? Color.white : LogLink;
                        GUI.Label(rect, tok, style);
                        style.normal.textColor = old;
                        GUI.color = hot ? Color.white : LogLink;
                        GUI.DrawTexture(new Rect(rect.x, rect.yMax - 2, rect.width, 1), Texture2D.whiteTexture); // underline
                        GUI.color = Color.white;
                    }
                    x += w;
                }
            }
            return y + lineH;
        }

        private void DrawZoneView(JObject st, HashSet<int> selectable, HashSet<int> picked, List<JObject> combat)
        {
            var parts = _zoneView.Split(':');
            var p = parts[0] == "me" ? Me(st) : Opp(st);
            var cards = Arr(p, parts[1]).ToList();
            var r = new Rect(60, 300, 1340, 420);
            GUI.Box(r, "");
            GUI.Label(new Rect(r.x + 12, r.y + 8, 800, 30), $"<b>{S(p, "name")} — {parts[1]} ({cards.Count})</b>", _title);
            if (GUI.Button(new Rect(r.xMax - 130, r.y + 8, 120, 36), "Close", _button)) _zoneView = null;
            DrawRow(new Rect(r.x + 12, r.y + 52, r.width - 24, r.height - 64), cards, selectable, picked, combat, 150);
        }

        private void DrawAsk(JObject ask)
        {
            string kind = S(ask, "kind");
            if (!ReferenceEquals(_askSeen, ask))
            {
                _askSeen = ask;
                _askPicked.Clear();
                _askScroll = Vector2.zero;
                var opts0 = Arr(ask, "options").ToList();
                _amounts = opts0.Select(o => I(o, "suggest")).ToArray();
                _order.Clear();
                for (int i = 0; i < opts0.Count; i++) _order.Add(i);
            }
            int min = I(ask, "min"), max = I(ask, "max", 1);
            var r = new Rect(360, 150, 1100, 760);
            GUI.color = new Color(0, 0, 0, 0.6f);
            GUI.DrawTexture(new Rect(0, 0, W, H), Texture2D.whiteTexture);
            GUI.color = Color.white;
            GUI.Box(r, "");
            GUI.Box(r, "");
            GUI.Label(new Rect(r.x + 16, r.y + 12, r.width - 32, 50), $"<b>{S(ask, "title")}</b>", _title);
            // Who is asking: the resolving spell/ability (hover for its card)
            if (ask["context"] is JObject ctx)
            {
                var cr = new Rect(r.x + 16, r.y + 60, r.width - 32, 46);
                var srcCard = ctx["card"] as JObject;
                GUI.Label(cr, $"<color=#ffcc66>From: <b>{S(srcCard, "name") ?? ""}</b></color> — {S(ctx, "text")}",
                    new GUIStyle(_small) { wordWrap = true, clipping = TextClipping.Clip });
                if (srcCard != null && cr.Contains(Event.current.mousePosition)) _hover = srcCard;
            }

            if (kind == "confirm")
            {
                var opts = (ask["options"] as JArray)?.Select(x => (string)x).ToList() ?? new List<string> { "Yes", "No" };
                for (int i = 0; i < opts.Count; i++)
                    if (GUI.Button(new Rect(r.x + 16 + i * 270, r.yMax - 90, 250, 70), opts[i], _big)) MtgSession.AnswerIndex(i);
                return;
            }

            if (kind == "amounts") { DrawAmounts(ask, r); return; }
            if (kind == "reorder") { DrawReorder(ask, r); return; }

            var options = Arr(ask, "options").ToList();
            var view = new Rect(r.x + 16, r.y + 110, r.width - 32, r.height - 220);
            if (options.Any(o => OptCard(o) != null))
            {
                if (DrawCardGrid(view, options, kind, min, max)) return; // answered
            }
            else
            {
            // Text options (abilities, modes...): rows grow to fit long ability text.
            var rowStyle = new GUIStyle(_button) { alignment = TextAnchor.MiddleLeft, wordWrap = true, padding = new RectOffset(10, 10, 8, 8) };
            var heights = options.Select(o => Mathf.Max(44f, rowStyle.CalcHeight(new GUIContent(S(o, "text") ?? ""), view.width - 24))).ToList();
            _askScroll = GUI.BeginScrollView(view, _askScroll, new Rect(0, 0, view.width - 20, heights.Sum() + options.Count * 4));
            float rowY = 0;
            for (int i = 0; i < options.Count; i++)
            {
                var o = options[i];
                float rowH = heights[i];
                var rr = new Rect(0, rowY, view.width - 24, rowH);
                rowY += rowH + 4;
                if (B(o, "header")) { GUI.Label(new Rect(rr.x + 4, rr.y + 10, rr.width, rowH), $"<b>{S(o, "text")}</b>", _text); continue; }
                bool on = _askPicked.Contains(i);
                var old = GUI.backgroundColor;
                if (on) GUI.backgroundColor = Picked;
                if (GUI.Button(rr, S(o, "text") ?? "", rowStyle) && kind != "reveal")
                {
                    if (max == 1 && min <= 1) { MtgSession.Answer(new JArray(i)); GUI.backgroundColor = old; GUI.EndScrollView(); return; }
                    if (on) _askPicked.Remove(i); else if (_askPicked.Count < max) _askPicked.Add(i);
                }
                GUI.backgroundColor = old;
                if (rr.Contains(Event.current.mousePosition)) _hover = OptCard(o) ?? (o["sourceInfo"] as JObject) ?? _hover;
            }
            GUI.EndScrollView();
            }

            if (kind == "reveal")
            {
                if (GUI.Button(new Rect(r.x + 16, r.yMax - 90, 250, 70), "OK", _big)) MtgSession.Answer(new JArray());
                return;
            }
            string hint = min == max ? $"Choose {min}" : $"Choose {min}–{max}";
            GUI.Label(new Rect(r.x + 16, r.yMax - 100, 500, 30), $"{hint}  ({_askPicked.Count} selected)", _text);
            bool okEnabled = _askPicked.Count >= min && _askPicked.Count <= max;
            GUI.enabled = okEnabled;
            if (GUI.Button(new Rect(r.xMax - 280, r.yMax - 90, 260, 70), min == 0 && _askPicked.Count == 0 ? "None" : "Confirm", _big))
                MtgSession.Answer(new JArray(_askPicked.OrderBy(x => x).Cast<object>().ToArray()));
            GUI.enabled = true;
        }

        /// <summary>One number per option (combat damage split, divided damage/counters, mana combinations); must add up to the total.</summary>
        private void DrawAmounts(JObject ask, Rect r)
        {
            var options = Arr(ask, "options").ToList();
            int total = I(ask, "total");
            int sum = _amounts.Sum();
            // Combat damage: rows with "lethal" must reach it before an "afterLethal" row (trample over to the player) gets any.
            bool allLethal = true;
            for (int i = 0; i < options.Count; i++)
                if (options[i]["lethal"] != null && _amounts[i] < I(options[i], "lethal")) allLethal = false;
            bool ruleBroken = false;
            for (int i = 0; i < options.Count; i++)
                if (B(options[i], "afterLethal") && _amounts[i] > 0 && !allLethal) ruleBroken = true;
            var view = new Rect(r.x + 16, r.y + 110, r.width - 32, r.height - 220);
            float rowH = 50;
            _askScroll = GUI.BeginScrollView(view, _askScroll, new Rect(0, 0, view.width - 20, options.Count * (rowH + 4)));
            for (int i = 0; i < options.Count; i++)
            {
                var o = options[i];
                float y = i * (rowH + 4);
                var label = new Rect(0, y, view.width - 290, rowH);
                GUI.Box(label, "");
                string note = "";
                if (o["lethal"] != null)
                {
                    int lethal = I(o, "lethal");
                    note = _amounts[i] >= lethal ? $"   <color=#70ff80>lethal {lethal} ✓</color>" : $"   <color=#aaaaaa>lethal {lethal}</color>";
                }
                bool after = B(o, "afterLethal");
                if (after) note = allLethal ? "" : "   <color=#aaaaaa>(after every blocker has lethal damage)</color>";
                GUI.Label(new Rect(label.x + 10, y + 12, label.width - 20, rowH - 12), (S(o, "text") ?? "") + note, _text);
                DrawThumb(new Rect(label.x + label.width - 40, label.y + 3, 32, label.height - 6), OptCard(o));
                if (label.Contains(Event.current.mousePosition)) _hover = OptCard(o) ?? _hover;
                int min = I(o, "min"), max = I(o, "max", total);
                GUI.enabled = _amounts[i] > min;
                if (GUI.Button(new Rect(view.width - 280, y, 70, rowH), "−", _big)) _amounts[i]--;
                GUI.enabled = true;
                GUI.Label(new Rect(view.width - 205, y, 80, rowH), $"<size=28><b>{_amounts[i]}</b></size>", new GUIStyle(_text) { alignment = TextAnchor.MiddleCenter });
                GUI.enabled = _amounts[i] < max && sum < total && (!after || allLethal);
                if (GUI.Button(new Rect(view.width - 120, y, 70, rowH), "+", _big)) _amounts[i]++;
                GUI.enabled = true;
            }
            GUI.EndScrollView();

            string unit = S(ask, "unit") ?? "";
            int remaining = total - sum;
            GUI.Label(new Rect(r.x + 16, r.yMax - 100, 560, 40),
                ruleBroken ? "<color=#ff8080>Give every blocker lethal damage before trampling over</color>"
                : remaining == 0 ? $"All {total} {unit} assigned" : $"<color=#ffcc66>{remaining} {unit} left to assign</color>", _text);
            if (GUI.Button(new Rect(r.xMax - 560, r.yMax - 90, 260, 70), "Auto", _big))
                _amounts = options.Select(o => I(o, "suggest")).ToArray();
            GUI.enabled = remaining == 0 && !ruleBroken;
            if (GUI.Button(new Rect(r.xMax - 280, r.yMax - 90, 260, 70), "Confirm", _big)) MtgSession.AnswerAmounts(_amounts);
            GUI.enabled = true;
        }

        /// <summary>Full ordering (top/first first). A divider row (e.g. "rest of library") can be moved like a card.</summary>
        private void DrawReorder(JObject ask, Rect r)
        {
            var options = Arr(ask, "options").ToList();
            int divider = I(ask, "divider", -1);
            var view = new Rect(r.x + 16, r.y + 110, r.width - 32, r.height - 220);
            float rowH = 46;
            GUI.Label(new Rect(r.x + 16, r.y + 80, 700, 28), "<i>Top of the list = first / top of the library</i>", _small);
            _askScroll = GUI.BeginScrollView(view, _askScroll, new Rect(0, 0, view.width - 20, _order.Count * (rowH + 4)));
            for (int pos = 0; pos < _order.Count; pos++)
            {
                int idx = _order[pos];
                var o = options[idx];
                float y = pos * (rowH + 4);
                var label = new Rect(0, y, view.width - 190, rowH);
                var old = GUI.backgroundColor;
                if (idx == divider) GUI.backgroundColor = new Color(0.4f, 0.4f, 0.7f);
                GUI.Box(label, "");
                GUI.backgroundColor = old;
                GUI.Label(new Rect(label.x + 10, y + 10, label.width - 20, rowH - 10),
                    idx == divider ? $"<i>{S(o, "text")}</i>" : $"{pos + 1}.  {S(o, "text")}", _text);
                DrawThumb(new Rect(label.x + label.width - 40, label.y + 3, 32, label.height - 6), OptCard(o));
                if (label.Contains(Event.current.mousePosition)) _hover = OptCard(o) ?? _hover;
                GUI.enabled = pos > 0;
                if (GUI.Button(new Rect(view.width - 180, y, 75, rowH), "▲", _big)) { _order[pos] = _order[pos - 1]; _order[pos - 1] = idx; }
                GUI.enabled = pos < _order.Count - 1;
                if (GUI.Button(new Rect(view.width - 100, y, 75, rowH), "▼", _big)) { _order[pos] = _order[pos + 1]; _order[pos + 1] = idx; }
                GUI.enabled = true;
            }
            GUI.EndScrollView();
            if (GUI.Button(new Rect(r.xMax - 280, r.yMax - 90, 260, 70), "Confirm", _big))
                MtgSession.Answer(new JArray(_order.Cast<object>().ToArray()));
        }

        /// <summary>The card an option is about: the details the bridge sent with it, else the card from the current state.</summary>
        private static JObject OptCard(JObject o) =>
            o?["cardInfo"] as JObject ?? (o?["card"] != null && o["card"].Type == JTokenType.Integer ? FindCard(I(o, "card")) : null);

        private static bool DrawThumb(Rect r, JObject c) => DrawFace(r, c, live: false);

        /// <summary>
        /// Card options as a grid of card images (library searches, reveals, "choose a card"...). Click to pick (single choice
        /// answers at once); hover for the big preview. Non-card options in the same list get a text tile. True when answered.
        /// </summary>
        private bool DrawCardGrid(Rect view, List<JObject> options, string kind, int min, int max)
        {
            const float tw = 150f, th = 210f, cellW = 162f, cellH = 244f;
            int cols = Mathf.Max(1, (int)((view.width - 20) / cellW));
            int rows = (options.Count + cols - 1) / cols;
            _askScroll = GUI.BeginScrollView(view, _askScroll, new Rect(0, 0, view.width - 20, rows * cellH));
            for (int i = 0; i < options.Count; i++)
            {
                var o = options[i];
                var c = OptCard(o);
                var tile = new Rect((i % cols) * cellW + 6, (i / cols) * cellH + 6, tw, th);
                if (B(o, "header"))
                {
                    GUI.Label(new Rect(tile.x, tile.y + th / 2 - 20, tw, 40), $"<b>{S(o, "text")}</b>", new GUIStyle(_text) { alignment = TextAnchor.MiddleCenter });
                    continue;
                }
                bool on = _askPicked.Contains(i);
                if (on)
                {
                    GUI.color = Picked;
                    GUI.DrawTexture(new Rect(tile.x - 4, tile.y - 4, tile.width + 8, tile.height + 8), Texture2D.whiteTexture);
                    GUI.color = Color.white;
                }
                if (DrawThumb(tile, c)) { }
                else
                {
                    GUI.Box(tile, "");
                    GUI.Label(new Rect(tile.x + 6, tile.y + 6, tile.width - 12, tile.height - 12),
                        c != null ? $"<b>{S(c, "name")}</b>\n<size=12>{S(c, "type")}\n\n{S(c, "text")}</size>" : (S(o, "text") ?? ""), _small);
                }
                GUI.Label(new Rect(tile.x, tile.yMax + 2, tile.width, 24), c != null ? S(c, "name") : "",
                    new GUIStyle(_small) { alignment = TextAnchor.UpperCenter, wordWrap = false, clipping = TextClipping.Clip });
                if (tile.Contains(Event.current.mousePosition)) _hover = c ?? _hover;
                if (kind != "reveal" && GUI.Button(tile, "", GUIStyle.none))
                {
                    if (max == 1 && min <= 1) { GUI.EndScrollView(); MtgSession.Answer(new JArray(i)); return true; }
                    if (on) _askPicked.Remove(i); else if (_askPicked.Count < max) _askPicked.Add(i);
                }
            }
            GUI.EndScrollView();
            return false;
        }

        private static JObject FindCard(int id)
        {
            var st = MtgSession.State;
            if (st == null) return null;
            foreach (var p in Arr(st, "players"))
                foreach (var z in new[] { "hand", "battlefield", "graveyard", "exile", "command" })
                    foreach (var c in Arr(p, z))
                        if (I(c, "id") == id) return c;
            foreach (var e in Arr(st, "stack"))
                if (e["card"] is JObject c && I(c, "id") == id) return c;
            return null;
        }

        private void DrawConcede()
        {
            var r = new Rect(660, 400, 600, 220);
            GUI.Box(r, "");
            GUI.Box(r, "");
            GUI.Label(new Rect(r.x + 20, r.y + 20, r.width - 40, 60), "<b>Concede this game?</b>", _title);
            if (GUI.Button(new Rect(r.x + 20, r.yMax - 90, 260, 70), "Concede", _big)) { _confirmConcede = false; ForgeBridge.Act("concede"); }
            if (GUI.Button(new Rect(r.xMax - 280, r.yMax - 90, 260, 70), "Keep playing", _big)) _confirmConcede = false;
        }
    }
}

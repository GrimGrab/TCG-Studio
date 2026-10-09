using System;
using System.Collections.Generic;
using System.Linq;
using HarmonyLib;
using TCGCustomCards.Runtime.Mtg;
using UnityEngine;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// Booster draft screen. <b>Picking</b>: the pack in front of the player as the real rolled copies (border/foil), click to
    /// select, click again / double-click / "Pick" to take it; Forge's draft AI picks for everyone else and passes the packs.
    /// <b>Building</b>: the player's picks → a 40-card limited deck with free basics (starts empty; "Forge's suggestion" fills it); "Done"
    /// asks Forge whether the deck is legal (DeckFormat.Limited) before handing it over. IMGUI, 1920×1080 virtual layout.
    /// </summary>
    internal class MtgDraftUI : MonoBehaviour
    {
        private const float W = 1920f, H = 1080f;
        private static readonly string[] Basics = { "Plains", "Island", "Swamp", "Mountain", "Forest" };
        private static MtgDraftUI _inst;

        private enum Mode { Picking, Building, Checking }
        private Mode _mode;
        private int _selected = -1;
        private string _message = "";
        private float _messageUntil;
        private bool _confirmLeave;
        private DraftCard _hover;
        private Vector2 _picksScroll, _poolScroll, _deckScroll;
        private GUIStyle _text, _small, _title, _button, _big;

        // building
        private List<DraftCard> _pool = new List<DraftCard>();
        private readonly HashSet<int> _inDeck = new HashSet<int>();
        private readonly Dictionary<string, int> _basics = new Dictionary<string, int>();
        private MtgDeck _checking;
        private MtgDeckCheck.Result _check;

        private Action<MtgDeck, List<CardData>> _onBuilt;
        private Action<string> _onClosed;

        public static bool IsOpen => _inst != null && _inst.enabled;

        /// <summary>
        /// Opens the pick screen for the running <see cref="MtgDraft"/>. <paramref name="onBuilt"/> gets the player's deck and the
        /// copies in it; <paramref name="onClosed"/> gets why the screen closed without a deck (aborted, error).
        /// </summary>
        public static void Open(Action<MtgDeck, List<CardData>> onBuilt, Action<string> onClosed)
        {
            MtgSession.EnsureRunner();
            var go = GameObject.Find("TCGCC_Mtg");
            _inst = go.GetComponent<MtgDraftUI>() ?? go.AddComponent<MtgDraftUI>();
            _inst._mode = Mode.Picking;
            _inst._selected = -1;
            _inst._confirmLeave = false;
            _inst._onBuilt = onBuilt;
            _inst._onClosed = onClosed;
            _inst.enabled = true;
            EnsureUiMode();
        }

        /// <summary>The draft finished: switch to building (called from the draft's completion callback).</summary>
        public static void DraftFinished(string error)
        {
            if (_inst == null || !_inst.enabled) return;
            if (error != null) { _inst.Close(error); return; }
            _inst.StartBuilding();
        }

        private static readonly AccessTools.FieldRef<InteractionPlayerController, bool> InUIMode =
            AccessTools.FieldRefAccess<InteractionPlayerController, bool>("m_IsInUIMode");

        /// <summary>UI mode on while open: a vanilla screen that just closed (e.g. the choice popup) turns it off 0.05 s later.</summary>
        private static void EnsureUiMode()
        {
            var ipc = CSingleton<InteractionPlayerController>.Instance;
            if (ipc == null || InUIMode(ipc)) return;
            ipc.m_WalkerCtrl?.SetStopMovement(isStop: true);
            ipc.EnterUIMode();
        }

        private void Close(string why)
        {
            enabled = false;
            var ipc = CSingleton<InteractionPlayerController>.Instance;
            ipc?.m_WalkerCtrl?.SetStopMovement(isStop: false);
            ipc?.ExitUIMode();
            var cb = _onClosed;
            _onClosed = null;
            _onBuilt = null;
            cb?.Invoke(why);
        }

        private void Say(string msg)
        {
            _message = msg;
            _messageUntil = Time.unscaledTime + 4f;
        }

        private void Update()
        {
            EnsureUiMode();
            if (_mode == Mode.Checking && _check != null && _check.Done)
            {
                if (_check.Ok) Built();
                else { Say(string.Join("  ", _check.Problems)); _mode = Mode.Building; }
                _check = null;
            }
            if (Input.GetKeyDown(KeyCode.Escape)) _confirmLeave = !_confirmLeave;
        }

        private void Styles()
        {
            if (_text != null) return;
            _text = new GUIStyle(GUI.skin.label) { fontSize = 18, wordWrap = true, richText = true };
            _small = new GUIStyle(_text) { fontSize = 15 };
            _title = new GUIStyle(_text) { fontSize = 28, fontStyle = FontStyle.Bold };
            _button = new GUIStyle(GUI.skin.button) { fontSize = 16, richText = true };
            _big = new GUIStyle(GUI.skin.button) { fontSize = 20, fontStyle = FontStyle.Bold };
        }

        private void OnGUI()
        {
            Styles();
            GUI.depth = -115;
            float scale = Mathf.Min(Screen.width / W, Screen.height / H);
            var offset = new Vector2((Screen.width - W * scale) / 2f, (Screen.height - H * scale) / 2f);
            GUI.matrix = Matrix4x4.TRS(new Vector3(offset.x, offset.y, 0f), Quaternion.identity, new Vector3(scale, scale, 1f));
            GUI.color = new Color(0.05f, 0.05f, 0.07f, 0.97f);
            GUI.DrawTexture(new Rect(0, 0, W, H), Texture2D.whiteTexture);
            GUI.color = Color.white;
            _hover = null;

            GUI.enabled = !_confirmLeave;
            if (_mode == Mode.Picking) DrawPicking();
            else DrawBuilding();
            GUI.enabled = true;

            if (_hover != null) DrawPreview(_hover, new Rect(W - 400, 90, 360, 504));
            if (_confirmLeave) DrawLeave();
            if (Time.unscaledTime < _messageUntil)
                GUI.Label(new Rect(40, H - 56, W - 80, 44), $"<color=#ffcc66><b>{_message}</b></color>", new GUIStyle(_text) { alignment = TextAnchor.MiddleCenter });
        }

        // ------------------------------------------------------------------ picking

        private void DrawPicking()
        {
            var ask = MtgDraft.Ask;
            var picks = MtgDraft.Running || MtgDraft.Seats == null ? CurrentPicks(ask) : MtgDraft.Seats[MtgDraft.HumanSeat].Picks;
            string head = MtgDraft.IsSealed ? "Sealed" : ask == null ? "Booster draft" : $"Booster draft - Pack {(int)ask["round"]}/{(int)ask["rounds"]} · Pick {(int)ask["pick"]}";
            GUI.Label(new Rect(40, 24, 1200, 40), head, _title);
            if (GUI.Button(new Rect(W - 440 - 180, 26, 160, 38), "Leave draft", _button)) _confirmLeave = true;

            var pack = ask == null ? new List<DraftCard>() :
                ((ask["pack"] as Newtonsoft.Json.Linq.JArray) ?? new Newtonsoft.Json.Linq.JArray())
                .Select(o => MtgDraft.Cards.TryGetValue((int)o["ref"], out var c) ? c : null).ToList();
            if (ask == null)
                GUI.Label(new Rect(40, 400, 1400, 60), !MtgDraft.Running ? "" : MtgDraft.IsSealed ? "<color=#cccccc>Opening your packs…</color>" :
                    "<color=#cccccc>Waiting for the other drafters…</color>", new GUIStyle(_text) { fontSize = 24, alignment = TextAnchor.MiddleCenter });
            else
            {
                if (_selected >= pack.Count) _selected = -1;
                const float cw = 168, ch = 235, gap = 10;
                int cols = 8;
                for (int i = 0; i < pack.Count; i++)
                {
                    var r = new Rect(40 + (i % cols) * (cw + gap), 90 + (i / cols) * (ch + gap), cw, ch);
                    var c = pack[i];
                    if (c == null) { GUI.Box(r, "?"); continue; }
                    DrawCard(r, c);
                    if (i == _selected) Frame(r, new Color(1f, 0.8f, 0.2f), 4);
                    if (r.Contains(Event.current.mousePosition)) _hover = c;
                    if (GUI.Button(r, GUIContent.none, GUIStyle.none))
                    {
                        if (i == _selected || Event.current.clickCount > 1) PickNow(i);
                        else _selected = i;
                    }
                }
                GUI.enabled = !_confirmLeave && _selected >= 0;
                if (GUI.Button(new Rect(40, H - 120, 260, 52), _selected >= 0 && pack[_selected] != null ? "Pick " + pack[_selected].Mtg.Name : "Pick a card", _big)) PickNow(_selected);
                GUI.enabled = !_confirmLeave;
                GUI.Label(new Rect(320, H - 116, 900, 44), "<color=#999999>Click a card to select it, click it again (or double-click) to take it.</color>", _small);
            }
            DrawPickList(picks, new Rect(W - 400, 610, 360, H - 680));
        }

        private static List<DraftCard> CurrentPicks(Newtonsoft.Json.Linq.JObject ask) =>
            ask == null ? new List<DraftCard>() :
            ((ask["picks"] as Newtonsoft.Json.Linq.JArray) ?? new Newtonsoft.Json.Linq.JArray())
            .Select(r => MtgDraft.Cards.TryGetValue((int)r, out var c) ? c : null).Where(c => c != null).ToList();

        private List<DraftCard> _lastPicks = new List<DraftCard>();

        private void DrawPickList(List<DraftCard> picks, Rect r)
        {
            if (picks.Count > 0 || MtgDraft.Ask != null) _lastPicks = picks;
            picks = _lastPicks;
            GUI.Label(new Rect(r.x, r.y - 34, r.width, 30), $"<b>Your picks</b>  <color=#999999>{picks.Count}</color>", _text);
            var groups = picks.GroupBy(c => c.Mtg.Name).OrderBy(g => ColorKey(g.First())).ThenBy(g => g.Key).ToList();
            const float rowH = 24;
            _picksScroll = GUI.BeginScrollView(r, _picksScroll, new Rect(0, 0, r.width - 20, Mathf.Max(r.height, groups.Count * rowH)));
            for (int i = 0; i < groups.Count; i++)
            {
                var row = new Rect(0, i * rowH, r.width - 20, rowH);
                var g = groups[i];
                GUI.Label(row, $"{(g.Count() > 1 ? g.Count() + "× " : "")}{g.Key}  <color=#888888>{ColorText(g.First())}</color>", _small);
                if (row.Contains(Event.current.mousePosition)) _hover = g.First();
            }
            GUI.EndScrollView();
        }

        private void PickNow(int index)
        {
            if (index < 0 || MtgDraft.Ask == null) return;
            MtgDraft.Pick(index);
            _selected = -1;
            SoundManager.GenericPop();
        }

        private void DrawLeave()
        {
            var r = new Rect(W / 2 - 260, H / 2 - 90, 520, 180);
            GUI.color = new Color(0.1f, 0.1f, 0.13f, 1f);
            GUI.DrawTexture(r, Texture2D.whiteTexture);
            GUI.color = Color.white;
            string what = _mode == Mode.Picking ? "Leave? The draft stops for this table. At a tournament, right-click your table again to start over with the same packs." : "Leave without a deck? At a tournament, right-click your table again to build (same packs).";
            GUI.Label(new Rect(r.x + 20, r.y + 16, r.width - 40, 80), what, _text);
            if (GUI.Button(new Rect(r.x + 20, r.y + 110, 230, 48), "Leave", _big))
            {
                _confirmLeave = false;
                if (_mode == Mode.Picking) MtgDraft.Quit(); // draftdone aborted → DraftFinished → Close
                else Close("left");
            }
            if (GUI.Button(new Rect(r.xMax - 250, r.y + 110, 230, 48), "Keep going", _big)) _confirmLeave = false;
        }

        // ------------------------------------------------------------------ building

        private void StartBuilding()
        {
            _mode = Mode.Building;
            _confirmLeave = false;
            var seat = MtgDraft.Seats[MtgDraft.HumanSeat];
            _pool = MtgDeckRules.Sort(seat.Picks, c => Info(c), MtgSort.Color).ToList();
            ClearDeck(); // the player builds; "Forge's suggestion" fills it on request (user 2026-10-09)
            Say((MtgDraft.IsSealed ? "Packs opened" : "Draft over") + " - build your 40-card deck, or press \"Forge's suggestion\" for a starting point.");
        }

        private void ClearDeck()
        {
            _inDeck.Clear();
            foreach (var b in Basics) _basics[b] = 0;
        }

        private void UseSuggestion(DraftSeat seat)
        {
            ClearDeck();
            if (seat.Suggested == null) { Say("Forge made no suggestion for these picks."); return; }
            foreach (var line in seat.Suggested.Main)
            {
                if (line.Card.IsBasicLand && _basics.ContainsKey(line.Card.Name)) { _basics[line.Card.Name] += line.Count; continue; }
                int need = line.Count;
                foreach (var c in _pool)
                    if (need > 0 && !_inDeck.Contains(c.Ref) && string.Equals(c.Mtg.Name, line.Card.Name, StringComparison.OrdinalIgnoreCase))
                    {
                        _inDeck.Add(c.Ref);
                        need--;
                    }
            }
        }

        private int DeckTotal => _inDeck.Count + _basics.Values.Sum();

        private void DrawBuilding()
        {
            GUI.Label(new Rect(40, 24, 1000, 40), MtgDraft.IsSealed ? "Build your sealed deck" : "Build your draft deck", _title);
            GUI.Label(new Rect(40, 62, 1300, 26), $"<color=#999999>Click a card to move it between your {(MtgDraft.IsSealed ? "pool" : "picks")} (left) and your deck (right). Basic lands are free.</color>", _small);
            if (GUI.Button(new Rect(W - 440 - 180, 26, 160, 38), "Leave", _button)) _confirmLeave = true;

            // pool: picks not in the deck
            var pool = _pool.Where(c => !_inDeck.Contains(c.Ref)).ToList();
            const float cw = 128, ch = 179, gap = 8;
            int cols = 10;
            var area = new Rect(40, 100, cols * (cw + gap) + 20, H - 200);
            GUI.Label(new Rect(area.x, area.y - 4, 400, 26), $"<b>{(MtgDraft.IsSealed ? "Pool" : "Picks")} not in the deck</b>  <color=#999999>{pool.Count}</color>", _small);
            int rows = (pool.Count + cols - 1) / cols;
            _poolScroll = GUI.BeginScrollView(new Rect(area.x, area.y + 24, area.width, area.height - 24), _poolScroll,
                new Rect(0, 0, area.width - 20, Mathf.Max(area.height - 24, rows * (ch + gap))));
            for (int i = 0; i < pool.Count; i++)
            {
                var r = new Rect((i % cols) * (cw + gap), (i / cols) * (ch + gap), cw, ch);
                DrawCard(r, pool[i]);
                if (r.Contains(Event.current.mousePosition)) _hover = pool[i];
                if (GUI.Button(r, GUIContent.none, GUIStyle.none)) _inDeck.Add(pool[i].Ref);
            }
            GUI.EndScrollView();

            // deck column (under the preview spot)
            float x = W - 400, w = 360, y = 610;
            int total = DeckTotal;
            GUI.Label(new Rect(x, y - 34, w, 30), $"<b>Deck</b>  <color={(total >= 40 ? "#88dd88" : "#ff9977")}>{total}/40</color>", _text);
            for (int b = 0; b < Basics.Length; b++)
            {
                var name = Basics[b];
                float bx = x + (b % 3) * 120, by = y + (b / 3) * 34;
                GUI.Label(new Rect(bx, by, 70, 30), $"{name.Substring(0, Math.Min(5, name.Length))} <b>{_basics[name]}</b>", _small);
                if (GUI.Button(new Rect(bx + 70, by, 22, 26), "−")) _basics[name] = Math.Max(0, _basics[name] - 1);
                if (GUI.Button(new Rect(bx + 94, by, 22, 26), "+")) _basics[name]++;
            }
            y += 74;
            var deckCards = _pool.Where(c => _inDeck.Contains(c.Ref)).ToList();
            var groups = deckCards.GroupBy(c => c.Mtg.Name).OrderBy(g => Info(g.First())?.Cmc ?? 0).ThenBy(g => g.Key).ToList();
            const float rowH = 24;
            float listH = H - 196 - y;
            _deckScroll = GUI.BeginScrollView(new Rect(x, y, w, listH), _deckScroll, new Rect(0, 0, w - 20, Mathf.Max(listH, groups.Count * rowH)));
            for (int i = 0; i < groups.Count; i++)
            {
                var row = new Rect(0, i * rowH, w - 20, rowH);
                var g = groups[i];
                if (GUI.Button(row, $"{(g.Count() > 1 ? g.Count() + "× " : "")}{g.Key}  <color=#888888>{ColorText(g.First())}</color>", new GUIStyle(_small) { alignment = TextAnchor.MiddleLeft }))
                    _inDeck.Remove(g.Last().Ref);
                if (row.Contains(Event.current.mousePosition)) _hover = g.First();
            }
            GUI.EndScrollView();

            if (GUI.Button(new Rect(x, H - 178, 170, 40), "Forge's suggestion", _button)) UseSuggestion(MtgDraft.Seats[MtgDraft.HumanSeat]);
            if (GUI.Button(new Rect(x + 180, H - 178, 180, 40), "Clear deck", _button)) ClearDeck();
            GUI.enabled = !_confirmLeave && _mode == Mode.Building;
            if (GUI.Button(new Rect(x, H - 130, 360, 44), _mode == Mode.Checking ? "Checking…" : "Done", _big)) CheckAndFinish();
            GUI.enabled = !_confirmLeave;
        }

        private MtgDeck BuildDeck()
        {
            var deckCards = _pool.Where(c => _inDeck.Contains(c.Ref)).ToList();
            var basicsPool = new List<MtgCard>();
            var sets = new HashSet<Runtime.CustomSet>(deckCards.Select(c => c.Set));
            var land = Runtime.Registry.Sets.FirstOrDefault(s => string.Equals(s.Def.Mtg?.SetCode, MtgDraft.LandSet, StringComparison.OrdinalIgnoreCase));
            if (land != null) sets.Add(land);
            foreach (var s in sets) basicsPool.AddRange(s.Def.Cards.Select(d => MtgMode.ToMtg(s.Def, d)).Where(m => m != null && m.IsBasicLand));
            return MtgDeckBuilder.FromMtgDeck(ForgeLauncher.Safe(ForgeLauncher.DeckPrefix + "My draft"),
                deckCards.Select(c => (c.Mtg, 1)), _basics, basicsPool);
        }

        private void CheckAndFinish()
        {
            if (DeckTotal < 40) { Say($"A draft deck needs at least 40 cards ({DeckTotal} now)."); return; }
            _checking = BuildDeck();
            _check = MtgDeckCheck.Check(_checking, new List<string>(), "limited"); // Forge decides (DeckFormat.Limited)
            _mode = Mode.Checking;
        }

        private void Built()
        {
            var copies = _pool.Where(c => _inDeck.Contains(c.Ref)).Select(c => c.Data).ToList();
            var deck = _checking;
            var cb = _onBuilt;
            _onBuilt = null;
            _onClosed = null;
            enabled = false;
            var ipc = CSingleton<InteractionPlayerController>.Instance;
            ipc?.m_WalkerCtrl?.SetStopMovement(isStop: false);
            ipc?.ExitUIMode();
            SoundManager.GenericConfirm();
            try { cb?.Invoke(deck, copies); }
            catch (Exception e) { Plugin.Log.LogError($"Draft deck callback failed: {e}"); }
        }

        // ------------------------------------------------------------------ drawing helpers

        private static MtgCardInfo Info(DraftCard c) => MtgDeckStore.Info(c.Set, c.Pos);

        private static string ColorKey(DraftCard c)
        {
            var cols = c.Mtg.Colors ?? new List<string>();
            return cols.Count == 0 ? "Z" : cols.Count > 1 ? "Y" : "WUBRG".IndexOf(cols[0], StringComparison.OrdinalIgnoreCase).ToString();
        }

        private static string ColorText(DraftCard c)
        {
            var cols = c.Mtg.Colors ?? new List<string>();
            return cols.Count == 0 ? (c.Mtg.IsLand ? "land" : "colourless") : string.Concat(cols);
        }

        private static void DrawCard(Rect r, DraftCard c)
        {
            var tex = CardRenderCache.Get(c.Data);
            if (tex != null) { GUI.DrawTexture(r, tex, ScaleMode.ScaleToFit); return; }
            var art = c.Set?.CardImage(c.Pos);
            if (art != null)
            {
                var tr = art.textureRect;
                var t = art.texture;
                GUI.DrawTextureWithTexCoords(r, t, new Rect(tr.x / t.width, tr.y / t.height, tr.width / t.width, tr.height / t.height));
            }
            else
            {
                GUI.Box(r, "");
                GUI.Label(new Rect(r.x + 6, r.y + 6, r.width - 12, r.height - 12), c.Mtg.Name);
            }
        }

        private void DrawPreview(DraftCard c, Rect r)
        {
            var tex = CardRenderCache.GetLive(c.Data);
            if (tex != null) GUI.DrawTexture(r, tex, ScaleMode.ScaleToFit);
            else DrawCard(r, c);
        }

        private static void Frame(Rect r, Color color, float t)
        {
            var old = GUI.color;
            GUI.color = color;
            GUI.DrawTexture(new Rect(r.x - t, r.y - t, r.width + 2 * t, t), Texture2D.whiteTexture);
            GUI.DrawTexture(new Rect(r.x - t, r.yMax, r.width + 2 * t, t), Texture2D.whiteTexture);
            GUI.DrawTexture(new Rect(r.x - t, r.y, t, r.height), Texture2D.whiteTexture);
            GUI.DrawTexture(new Rect(r.xMax, r.y, t, r.height), Texture2D.whiteTexture);
            GUI.color = old;
        }
    }
}

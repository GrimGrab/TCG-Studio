using System;
using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Runtime;
using TCGCustomCards.Runtime.Mtg;
using UnityEngine;

namespace TCGCustomCards.UI
{
    /// <summary>Where the player sits down: which decks can be used there.</summary>
    internal sealed class MtgPickContext
    {
        public string Title = "Choose your deck";
        /// <summary>Event's Forge set codes for a tournament legality check, or null (no tournament).</summary>
        public List<string> SetCodes;
        /// <summary>The deck kind the game needs: true = Commander, false = constructed, null = either.</summary>
        public bool? Commander;
        /// <summary>A game outside tournaments: offer Best of 1 / Best of 3 ([MTG - Match] CasualBestOf3).</summary>
        public bool Casual;
    }

    /// <summary>
    /// "Choose your deck" when sitting down for MTG: every deck the player has (MTG deck-builder decks, also unfinished ones, or
    /// vanilla decks with MTG cards) with mana symbols, kind, card count and sets; search + sort (shared with the deck builder's
    /// list). Decks that can't be used here are greyed out under "Can't use here" with the reason: unfinished, wrong kind for
    /// the event, PlayInForgeWindow limits, or Forge's legality verdict (<see cref="MtgDeckCheck"/>, Commander decks always,
    /// constructed decks against a tournament's sets). Enter plays the highlighted deck, Esc clears the search / cancels.
    /// IMGUI, 1920×1080 virtual layout.
    /// </summary>
    internal class MtgDeckPickerUI : MonoBehaviour
    {
        private const float W = 1920f, H = 1080f;
        private const string SearchControl = "TCGCC_DeckSearch";
        private static MtgDeckPickerUI _inst;

        private sealed class Row
        {
            public MtgDeckChoice Choice;
            public MtgDeck Deck;
            public MtgDeckSummary Summary;
            /// <summary>Why the deck can't be used here (null = it can, maybe after Forge's check).</summary>
            public string Why;
            public MtgDeckCheck.Result Check;
        }

        private readonly List<Row> _rows = new List<Row>();
        private MtgPickContext _ctx;
        private Action<MtgDeckChoice> _onPick;
        private Row _sel;
        private string _search = "";
        private bool _focusSearch;
        private Vector2 _scroll;
        private bool _armed;
        private GUIStyle _text, _small, _title1, _button, _row, _rowSel, _reason;

        public static bool IsOpen => _inst != null && _inst.enabled;

        public static void Open(MtgPickContext ctx, List<MtgDeckChoice> choices, Action<MtgDeckChoice> onPick)
        {
            MtgSession.EnsureRunner(); // pumps Forge's replies (MtgDeckCheck)
            var go = GameObject.Find("TCGCC_Mtg");
            _inst = go.GetComponent<MtgDeckPickerUI>() ?? go.AddComponent<MtgDeckPickerUI>();
            _inst.Init(ctx, choices, onPick);
            _inst.enabled = true;
            var ipc = CSingleton<InteractionPlayerController>.Instance;
            ipc?.m_WalkerCtrl?.SetStopMovement(isStop: true);
            ipc?.EnterUIMode();
        }

        private static bool WindowMode => Plugin.MtgPlayInWindow.Value || !MtgSession.CanPlayInGame;

        private void Init(MtgPickContext ctx, List<MtgDeckChoice> choices, Action<MtgDeckChoice> onPick)
        {
            _ctx = ctx;
            _onPick = onPick;
            _armed = false;
            _scroll = Vector2.zero;
            _search = "";
            _focusSearch = true;
            _rows.Clear();
            foreach (var c in choices) _rows.Add(MakeRow(c));
            var shown = Visible();
            _sel = shown.FirstOrDefault(r => MtgMode.IsSelected(r.Choice) && r.Why == null && !Failed(r))
                   ?? shown.FirstOrDefault(r => r.Why == null && !Failed(r)) ?? shown.FirstOrDefault();
        }

        private Row MakeRow(MtgDeckChoice c)
        {
            var row = new Row { Choice = c };
            var store = c.Store && c.Index >= 0 && c.Index < MtgDeckStore.Decks.Count ? MtgDeckStore.Decks[c.Index] : null;
            if (store != null)
            {
                row.Summary = MtgDeckSummary.Of(store);
                if (!store.Valid) row.Why = store.Problems.FirstOrDefault();
            }
            else if (!c.Store)
            {
                var v = CPlayerData.m_DeckCompactCardDataList[c.Index];
                int have = v.GetTotalCardCount(), need = GameInstance.GetMaxDeckCardCount();
                if (have < need) row.Why = $"Deck incomplete ({have}/{need} cards)";
            }
            bool commander = row.Summary?.IsCommander ?? c.Commander;
            if (row.Why == null) row.Why = KindReason(commander);
            if (row.Why == null && WindowMode)
            {
                if (commander) row.Why = "Commander games are played at the table: turn off [MTG] PlayInForgeWindow";
                else if (_ctx.SetCodes != null) row.Why = "Tournament rounds are played at the table: turn off [MTG] PlayInForgeWindow";
            }
            // Builds the Forge deck (also for greyed rows: their colours and sets are still shown)
            HashSet<CustomSet> sets = null;
            string err = row.Summary == null || store?.Valid == true ? MtgMode.BuildPlayer(c, out row.Deck, out sets) : null;
            if (row.Summary == null)
            {
                row.Summary = row.Deck != null ? MtgDeckSummary.Of(c.Name, row.Deck, sets) : new MtgDeckSummary { Name = c.Name ?? "" };
                row.Summary.Current = MtgMode.IsSelected(c);
            }
            if (row.Why == null) row.Why = err;
            if (row.Why == null && row.Deck != null)
            {
                // Commander decks are always checked by Forge (DeckFormat.Commander); constructed decks at tournaments.
                if (row.Deck.IsCommander) row.Check = MtgDeckCheck.Check(row.Deck, _ctx.SetCodes ?? new List<string>(), "commander");
                else if (_ctx.SetCodes != null) row.Check = MtgDeckCheck.Check(row.Deck, _ctx.SetCodes);
            }
            return row;
        }

        private string KindReason(bool commander)
        {
            if (_ctx.Commander == true && !commander) return "Not a Commander deck - this is a Commander tournament";
            if (_ctx.Commander == false && commander) return "Commander deck - this is a Magic (constructed) tournament";
            return null;
        }

        private static bool Failed(Row r) => r.Check != null && r.Check.Done && !r.Check.Ok;
        private static bool Blocked(Row r) => r.Why != null || Failed(r);
        private static bool Playable(Row r) => r.Why == null && (r.Check == null || (r.Check.Done && r.Check.Ok));

        /// <summary>Rows matching the search: usable (or still being checked) first, then the greyed ones; each part sorted.</summary>
        private List<Row> Visible()
        {
            var match = _rows.Where(r => r.Summary.Matches(_search)).ToList();
            var sort = MtgDeckSummary.CurrentSort;
            return MtgDeckSummary.Sort(match.Where(r => !Blocked(r)), r => r.Summary, sort)
                .Concat(MtgDeckSummary.Sort(match.Where(Blocked), r => r.Summary, sort)).ToList();
        }

        private void Finish(Row pick)
        {
            enabled = false;
            var ipc = CSingleton<InteractionPlayerController>.Instance;
            ipc?.m_WalkerCtrl?.SetStopMovement(isStop: false);
            if (pick == null) { ipc?.ExitUIMode(); SoundManager.GenericCancel(); return; }
            SoundManager.GenericConfirm();
            // The pick usually sits down at the table, whose SetPlayTable enters UI mode itself. Leaving UI mode first would
            // start vanilla's DelaySetUIModeFalse, which switches it off again 0.05 s into the match (cursor gone until Esc).
            // So UI mode is only left when no table game started (window mode, refused sit, Commander/tournament toasts).
            try { _onPick?.Invoke(pick.Choice); }
            catch (Exception e) { Plugin.Log.LogError($"Deck picker action failed: {e}"); }
            finally
            {
                var table = CSingleton<PlayCardGameManager>.Instance?.m_PlayTableGame;
                if (table == null || !table.IsPlayTableGameMode()) ipc?.ExitUIMode();
            }
        }

        private static readonly HarmonyLib.AccessTools.FieldRef<InteractionPlayerController, bool> InUIMode =
            Core.Compat.Field<InteractionPlayerController, bool>(typeof(Row), "m_IsInUIMode");

        private void Update()
        {
            // The Magic/Tetramon popup that opened us leaves UI mode with a 0.05 s delay (ExitUIMode → DelaySetUIModeFalse),
            // which would switch it off after we switched it on: keep it on while we're open, or clicks go to the game.
            var ipc = CSingleton<InteractionPlayerController>.Instance;
            if (ipc != null && !InUIMode(ipc))
            {
                ipc.m_WalkerCtrl?.SetStopMovement(isStop: true);
                ipc.EnterUIMode();
            }
            var shown = Visible();
            int i = shown.IndexOf(_sel);
            if (Input.GetKeyDown(KeyCode.Escape))
            {
                if (!string.IsNullOrEmpty(_search)) _search = "";
                else Finish(null);
            }
            else if (Input.GetKeyDown(KeyCode.Return) || Input.GetKeyDown(KeyCode.KeypadEnter))
            {
                if (_sel != null && i >= 0 && Playable(_sel)) Finish(_sel);
            }
            else if (Input.GetKeyDown(KeyCode.DownArrow) && shown.Count > 0) _sel = shown[Math.Min(shown.Count - 1, i + 1)];
            else if (Input.GetKeyDown(KeyCode.UpArrow) && shown.Count > 0) _sel = shown[Math.Max(0, i - 1)];
        }

        private void Styles()
        {
            if (_text != null) return;
            _text = new GUIStyle(GUI.skin.label) { fontSize = 19, wordWrap = true, richText = true };
            _small = new GUIStyle(_text) { fontSize = 15 };
            _title1 = new GUIStyle(_text) { fontSize = 28, fontStyle = FontStyle.Bold };
            _button = new GUIStyle(GUI.skin.button) { fontSize = 20, fontStyle = FontStyle.Bold, richText = true };
            _row = new GUIStyle(GUI.skin.button);
            _rowSel = new GUIStyle(_row);
            _rowSel.normal.background = _rowSel.active.background;
            _reason = new GUIStyle(_small) { wordWrap = true };
            _reason.normal.textColor = new Color(1f, 0.6f, 0.47f);
        }

        private const float RowTop = 66f; // name line + info line

        private float RowHeight(Row r, float width) =>
            RowTop + Reasons(r).Sum(t => _reason.CalcHeight(new GUIContent(t), width - 28)) + 8;

        private void OnGUI()
        {
            Styles();
            GUI.depth = -120;
            if (!_armed) // the click that opened us must not also pick a deck
            {
                var e = Event.current;
                if (e.type == EventType.MouseDown) _armed = true;
                else if (e.type == EventType.MouseUp || e.type == EventType.MouseDrag) e.Use();
            }
            float scale = Mathf.Min(Screen.width / W, Screen.height / H);
            var offset = new Vector2((Screen.width - W * scale) / 2f, (Screen.height - H * scale) / 2f);
            GUI.matrix = Matrix4x4.TRS(new Vector3(offset.x, offset.y, 0f), Quaternion.identity, new Vector3(scale, scale, 1f));

            var panel = new Rect(W / 2 - 470, 110, 940, 860);
            GUI.color = new Color(0.06f, 0.06f, 0.08f, 0.96f);
            GUI.DrawTexture(panel, Texture2D.whiteTexture);
            GUI.color = Color.white;
            float x = panel.x + 24, y = panel.y + 18, w = panel.width - 48;
            GUI.Label(new Rect(x, y, w, 36), _ctx.Title, _title1);
            y += 40;
            if (_ctx.SetCodes != null)
            {
                GUI.Label(new Rect(x, y, w, 30), $"Tournament sets: <b>{string.Join(" ", _ctx.SetCodes)}</b>  <color=#aaaaaa>(checked by Forge)</color>", _small);
                y += 32;
            }

            // Search + sort
            GUI.Label(new Rect(x, y + 6, 70, 30), "Search", _small);
            GUI.SetNextControlName(SearchControl);
            _search = GUI.TextField(new Rect(x + 64, y, 330, 36), _search ?? "", 40, new GUIStyle(GUI.skin.textField) { fontSize = 17 });
            if (_focusSearch && Event.current.type == EventType.Repaint) { GUI.FocusControl(SearchControl); _focusSearch = false; }
            if (GUI.Button(new Rect(x + 398, y, 36, 36), "✕", new GUIStyle(GUI.skin.button) { fontSize = 15 })) _search = "";
            float sx = x + 460;
            GUI.Label(new Rect(sx, y + 6, 50, 30), "Sort", _small);
            sx += 44;
            foreach (MtgDeckSort s in Enum.GetValues(typeof(MtgDeckSort)))
            {
                var old = GUI.backgroundColor;
                if (MtgDeckSummary.CurrentSort == s) GUI.backgroundColor = new Color(0.35f, 0.75f, 1f);
                if (GUI.Button(new Rect(sx, y, 96, 36), MtgDeckSummary.SortLabel(s), new GUIStyle(GUI.skin.button) { fontSize = 15 })) MtgDeckSummary.CurrentSort = s;
                GUI.backgroundColor = old;
                sx += 100;
            }
            y += 46;

            // Deck list
            var shown = Visible();
            float listH = panel.yMax - y - 80 - (_ctx.Casual ? 46 : 0);
            float rw = w - 20;
            int firstBlocked = shown.FindIndex(Blocked);
            const float dividerH = 34f;
            var heights = shown.Select(r => RowHeight(r, rw)).ToList();
            float contentH = heights.Sum() + 6 * heights.Count + (firstBlocked >= 0 ? dividerH : 0);
            _scroll = GUI.BeginScrollView(new Rect(x, y, w, listH), _scroll, new Rect(0, 0, rw, Mathf.Max(listH, contentH)));
            float ry = 0;
            for (int i = 0; i < shown.Count; i++)
            {
                var r = shown[i];
                if (i == firstBlocked)
                {
                    int n = shown.Count - firstBlocked;
                    GUI.Label(new Rect(4, ry + 6, rw, 26), $"<color=#999999><b>Can't use here</b> ({n})</color>", _small);
                    ry += dividerH;
                }
                var rect = new Rect(0, ry, rw, heights[i]);
                if (GUI.Button(rect, "", r == _sel ? _rowSel : _row))
                {
                    if (r == _sel && Event.current.clickCount > 1 && Playable(r)) Finish(r);
                    _sel = r;
                }
                DrawRow(r, rect, r == _sel);
                ry += heights[i] + 6;
            }
            if (shown.Count == 0)
                GUI.Label(new Rect(4, 8, rw, 60), _rows.Count == 0 ? "<i>You have no MTG decks yet.</i>" : $"<i>No deck matches \"{_search}\".</i>", _text);
            GUI.EndScrollView();

            if (_ctx.Casual && Plugin.MtgCasualBestOf3 != null)
            {
                float by = panel.yMax - 66 - 46;
                bool bo3 = Plugin.MtgCasualBestOf3.Value;
                GUI.Label(new Rect(x, by, 120, 38), "Games", _text);
                if (GUI.Button(new Rect(x + 120, by, 160, 38), (bo3 ? "○ " : "● ") + "Best of 1", _button)) Plugin.MtgCasualBestOf3.Value = false;
                if (GUI.Button(new Rect(x + 290, by, 160, 38), (bo3 ? "● " : "○ ") + "Best of 3", _button)) Plugin.MtgCasualBestOf3.Value = true;
                GUI.Label(new Rect(x + 465, by, w - 465, 38),
                    $"<color=#888888>{(bo3 ? "first to win 2 games" : "one game")}</color>", _small);
            }

            var cur = _sel != null && shown.Contains(_sel) ? _sel : null;
            GUI.enabled = cur != null && Playable(cur);
            if (GUI.Button(new Rect(panel.xMax - 24 - 220, panel.yMax - 66, 220, 48), "Play this deck", _button)) Finish(cur);
            GUI.enabled = true;
            if (GUI.Button(new Rect(panel.xMax - 24 - 220 - 160, panel.yMax - 66, 150, 48), "Cancel", _button)) Finish(null);
            GUI.Label(new Rect(x, panel.yMax - 60, 480, 40), "<color=#888888>Enter = play · ↑↓ = choose · Esc = clear search / cancel</color>", _small);
        }

        private void DrawRow(Row r, Rect rect, bool selected)
        {
            var s = r.Summary;
            bool blocked = Blocked(r);
            var before = GUI.color;
            if (blocked) GUI.color = new Color(1f, 1f, 1f, 0.5f);
            float x = rect.x + 14, y = rect.y + 8;
            var nameStyle = new GUIStyle(_text) { wordWrap = false };
            if (selected) nameStyle.normal.textColor = new Color(1f, 0.85f, 0.4f);
            string name = $"<b>{s.Name}</b>";
            float nw = Mathf.Min(rect.width * 0.5f, nameStyle.CalcSize(new GUIContent(name)).x);
            GUI.Label(new Rect(x, y, nw + 4, 28), name, nameStyle);
            float cx = x + nw + 12;
            cx += ManaSymbols.KindBadge(cx, y + 2, s.IsCommander) + 10;
            ManaSymbols.DrawColors(cx, y + 1, s.Colors, 24f);
            var status = new GUIStyle(_small) { alignment = TextAnchor.UpperRight, wordWrap = false };
            GUI.Label(new Rect(rect.xMax - 300, y + 4, 286, 24), Status(r), status);

            string info = $"{s.Total}/{s.Target} cards";
            if (s.CommanderName != null) info += $"   Commander: {s.CommanderName}";
            if (s.SetCodes.Count > 0) info += $"   <color=#999999>{string.Join(" ", s.SetCodes)}</color>";
            string ago = MtgDeckSummary.Ago(s.LastUsed);
            if (ago.Length > 0) info += $"   <color=#888888>used {ago}</color>";
            GUI.Label(new Rect(x, y + 32, rect.width - 28, 24), info, new GUIStyle(_small) { wordWrap = false, clipping = TextClipping.Clip });
            GUI.color = before;

            float ry = rect.y + RowTop;
            foreach (var t in Reasons(r))
            {
                float h = _reason.CalcHeight(new GUIContent(t), rect.width - 28);
                GUI.Label(new Rect(x, ry, rect.width - 28, h), t, _reason);
                ry += h;
            }
        }

        private static string Status(Row r)
        {
            if (r.Why != null) return "<color=#ff8866>can't use here</color>";
            if (r.Check != null && !r.Check.Done) return "<color=#cccccc>Checking with Forge…</color>";
            if (Failed(r)) return "<color=#ff8866>not legal here</color>";
            if (r.Check != null) return "<color=#88dd88>legal</color>";
            return MtgMode.IsSelected(r.Choice) ? "<color=#88cc88>current deck</color>" : "";
        }

        /// <summary>Why a greyed deck can't be used: our reason, or Forge's problems word for word.</summary>
        private static List<string> Reasons(Row r)
        {
            if (r.Why != null) return new List<string> { r.Why };
            if (Failed(r)) return r.Check.Problems.Select(p => p.Replace("\n\n", ": ").Replace("\n", ", ")).ToList();
            return new List<string>();
        }
    }
}

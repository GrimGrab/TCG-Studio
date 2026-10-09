using System;
using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Runtime.Mtg;
using UnityEngine;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// "Choose your deck" when sitting down for MTG: every deck the player could play (MTG deck-builder decks or vanilla decks
    /// with MTG cards) with card count, colours and sets. At MTG tournaments each deck is checked by Forge against the event's
    /// sets (<see cref="MtgDeckCheck"/>): legal / not legal with Forge's reasons; illegal decks can't be played there.
    /// The current deck is preselected; Enter plays the highlighted deck, Esc cancels. IMGUI, 1920×1080 virtual layout.
    /// </summary>
    internal class MtgDeckPickerUI : MonoBehaviour
    {
        private const float W = 1920f, H = 1080f;
        private static MtgDeckPickerUI _inst;

        private sealed class Row
        {
            public MtgDeckChoice Choice;
            public MtgDeck Deck;
            public string Error;      // can't be played at all (incomplete vanilla deck, build error)
            public string Colors = "", Sets = "";
            public MtgDeckCheck.Result Check;
        }

        private readonly List<Row> _rows = new List<Row>();
        private string _title;
        private List<string> _setCodes; // null = no tournament check
        private Action<MtgDeckChoice> _onPick;
        private int _sel;
        private Vector2 _scroll;
        private bool _armed;
        private GUIStyle _text, _small, _title1, _button, _row, _rowSel;

        public static bool IsOpen => _inst != null && _inst.enabled;

        /// <param name="setCodes">Event's Forge set codes for a tournament legality check, or null.</param>
        public static void Open(string title, List<MtgDeckChoice> choices, List<string> setCodes, Action<MtgDeckChoice> onPick)
        {
            MtgSession.EnsureRunner(); // pumps Forge's replies (MtgDeckCheck)
            var go = GameObject.Find("TCGCC_Mtg");
            _inst = go.GetComponent<MtgDeckPickerUI>() ?? go.AddComponent<MtgDeckPickerUI>();
            _inst.Init(title, choices, setCodes, onPick);
            _inst.enabled = true;
            var ipc = CSingleton<InteractionPlayerController>.Instance;
            ipc?.m_WalkerCtrl?.SetStopMovement(isStop: true);
            ipc?.EnterUIMode();
        }

        private void Init(string title, List<MtgDeckChoice> choices, List<string> setCodes, Action<MtgDeckChoice> onPick)
        {
            _title = title;
            _setCodes = setCodes;
            _onPick = onPick;
            _armed = false;
            _scroll = Vector2.zero;
            _rows.Clear();
            foreach (var c in choices)
            {
                var row = new Row { Choice = c };
                if (!c.Store)
                {
                    var v = CPlayerData.m_DeckCompactCardDataList[c.Index];
                    int have = v.GetTotalCardCount(), need = GameInstance.GetMaxDeckCardCount();
                    if (have < need) row.Error = $"Deck incomplete ({have}/{need} cards)";
                }
                string err = MtgMode.BuildPlayer(c, out row.Deck, out var sets);
                if (row.Error == null) row.Error = err;
                if (row.Deck != null)
                {
                    var colors = new HashSet<string>(row.Deck.Main.SelectMany(l => l.Card.Colors ?? new List<string>()).Select(x => x.ToUpperInvariant()));
                    row.Colors = colors.Count == 0 ? "Colourless" : string.Concat("WUBRG".Where(ch => colors.Contains(ch.ToString())));
                    row.Sets = string.Join(" ", sets.Select(s => (s.Def.Mtg?.SetCode ?? s.Def.Id).ToUpperInvariant()).Distinct().OrderBy(x => x));
                    if (_setCodes != null && row.Error == null) row.Check = MtgDeckCheck.Check(row.Deck, _setCodes);
                }
                _rows.Add(row);
            }
            _sel = Math.Max(0, _rows.FindIndex(r => MtgMode.IsSelected(r.Choice)));
        }

        private bool Playable(Row r) => r.Error == null && (r.Check == null || (r.Check.Done && r.Check.Ok));

        private void Finish(Row pick)
        {
            enabled = false;
            var ipc = CSingleton<InteractionPlayerController>.Instance;
            ipc?.m_WalkerCtrl?.SetStopMovement(isStop: false);
            ipc?.ExitUIMode();
            if (pick == null) { SoundManager.GenericCancel(); return; }
            SoundManager.GenericConfirm();
            try { _onPick?.Invoke(pick.Choice); }
            catch (Exception e) { Plugin.Log.LogError($"Deck picker action failed: {e}"); }
        }

        private static readonly HarmonyLib.AccessTools.FieldRef<InteractionPlayerController, bool> InUIMode =
            HarmonyLib.AccessTools.FieldRefAccess<InteractionPlayerController, bool>("m_IsInUIMode");

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
            if (Input.GetKeyDown(KeyCode.Escape)) Finish(null);
            else if (Input.GetKeyDown(KeyCode.Return) || Input.GetKeyDown(KeyCode.KeypadEnter))
            {
                if (_sel >= 0 && _sel < _rows.Count && Playable(_rows[_sel])) Finish(_rows[_sel]);
            }
            else if (Input.GetKeyDown(KeyCode.DownArrow)) _sel = Math.Min(_rows.Count - 1, _sel + 1);
            else if (Input.GetKeyDown(KeyCode.UpArrow)) _sel = Math.Max(0, _sel - 1);
        }

        private void Styles()
        {
            if (_text != null) return;
            _text = new GUIStyle(GUI.skin.label) { fontSize = 19, wordWrap = true, richText = true };
            _small = new GUIStyle(_text) { fontSize = 15 };
            _title1 = new GUIStyle(_text) { fontSize = 28, fontStyle = FontStyle.Bold };
            _button = new GUIStyle(GUI.skin.button) { fontSize = 20, fontStyle = FontStyle.Bold, richText = true };
            _row = new GUIStyle(GUI.skin.button) { fontSize = 18, richText = true, alignment = TextAnchor.UpperLeft, wordWrap = true, padding = new RectOffset(14, 10, 8, 8) };
            _rowSel = new GUIStyle(_row);
            _rowSel.normal.background = _rowSel.active.background;
            _rowSel.normal.textColor = _rowSel.hover.textColor = new Color(1f, 0.85f, 0.4f);
        }

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

            var panel = new Rect(W / 2 - 420, 140, 840, 800);
            GUI.color = new Color(0.06f, 0.06f, 0.08f, 0.96f);
            GUI.DrawTexture(panel, Texture2D.whiteTexture);
            GUI.color = Color.white;
            float x = panel.x + 24, y = panel.y + 18, w = panel.width - 48;
            GUI.Label(new Rect(x, y, w, 36), _title, _title1);
            y += 40;
            if (_setCodes != null)
            {
                GUI.Label(new Rect(x, y, w, 30), $"Tournament sets: <b>{string.Join(" ", _setCodes)}</b>  <color=#aaaaaa>(checked by Forge)</color>", _small);
                y += 32;
            }

            float listH = panel.yMax - y - 80;
            float rw = w - 20;
            var labels = _rows.Select((r, i) => Label(r, i == _sel)).ToList();
            var heights = labels.Select((l, i) => Mathf.Max(86f, (i == _sel ? _rowSel : _row).CalcHeight(new GUIContent(l), rw))).ToList();
            float contentH = heights.Sum() + 6 * heights.Count;
            _scroll = GUI.BeginScrollView(new Rect(x, y, w, listH), _scroll, new Rect(0, 0, rw, Mathf.Max(listH, contentH)));
            float ry = 0;
            for (int i = 0; i < _rows.Count; i++)
            {
                var r = _rows[i];
                if (GUI.Button(new Rect(0, ry, rw, heights[i]), labels[i], i == _sel ? _rowSel : _row))
                {
                    if (i == _sel && Event.current.clickCount > 1 && Playable(r)) Finish(r);
                    _sel = i;
                }
                ry += heights[i] + 6;
            }
            GUI.EndScrollView();

            var cur = _sel >= 0 && _sel < _rows.Count ? _rows[_sel] : null;
            GUI.enabled = cur != null && Playable(cur);
            if (GUI.Button(new Rect(panel.xMax - 24 - 220, panel.yMax - 66, 220, 48), "Play this deck", _button)) Finish(cur);
            GUI.enabled = true;
            if (GUI.Button(new Rect(panel.xMax - 24 - 220 - 160, panel.yMax - 66, 150, 48), "Cancel", _button)) Finish(null);
            GUI.Label(new Rect(x, panel.yMax - 60, 380, 40), "<color=#888888>Enter = play · Esc = cancel</color>", _small);
        }

        private string Label(Row r, bool selected)
        {
            string label = $"<b>{r.Choice.Name}</b>   {Status(r)}\n<size=15>{(r.Deck != null ? r.Deck.Total + " cards" : "")}" +
                           $"   {r.Colors}   <color=#999999>{r.Sets}</color></size>";
            if (selected && HasDetails(r)) label += "\n<size=15><color=#ff9977>" + string.Join("\n", Details(r)) + "</color></size>";
            return label;
        }

        private string Status(Row r)
        {
            if (r.Error != null) return "<color=#ff8866>can't play</color>";
            if (r.Check == null) return MtgMode.IsSelected(r.Choice) ? "<color=#88cc88>(current)</color>" : "";
            if (!r.Check.Done) return "<color=#cccccc>Checking with Forge…</color>";
            return r.Check.Ok ? "<color=#88dd88>legal</color>" : "<color=#ff8866>not legal here</color>";
        }

        private static bool HasDetails(Row r) => Details(r).Count > 0;

        private static List<string> Details(Row r)
        {
            if (r.Error != null) return new List<string> { r.Error };
            if (r.Check != null && r.Check.Done && !r.Check.Ok)
                return r.Check.Problems.Select(p => p.Replace("\n\n", ": ").Replace("\n", ", ")).ToList();
            return new List<string>();
        }
    }
}

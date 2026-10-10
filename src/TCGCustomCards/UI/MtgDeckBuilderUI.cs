using System;
using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Runtime;
using TCGCustomCards.Runtime.Mtg;
using UnityEngine;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// MTG deck builder ([MTG] MtgDeckBuilder): replaces the workbench's deck screen. Deck list (new/edit/delete/set active) and an
    /// editor with a searchable/filterable grid of the player's owned MTG cards, the deck by type, free basic lands and a mana curve.
    /// IMGUI on a 1920×1080 virtual layout, like MtgTableUI. Rules are enforced here: no 5th copy of a name (basics exempt), a deck
    /// can only be set active with 60+ cards. Cards in decks are out of the collection (MtgDeckStore).
    /// </summary>
    internal class MtgDeckBuilderUI : MonoBehaviour
    {
        private const float W = 1920f, H = 1080f;
        private static MtgDeckBuilderUI _inst;

        private MtgDeckStore.Deck _editing;          // null = deck list
        private MtgDeckStore.Deck _confirmDelete;
        private readonly MtgCardFilter _filter = new MtgCardFilter();
        private MtgSort _sort = MtgSort.ManaValue;
        private string _lastSearch = "";
        private bool _dirty = true;
        private List<(CustomSet set, int pos, MtgCardInfo info)> _all;   // every MTG card of installed sets
        /// <summary>One tile per owned version (border + foil) of each matching card.</summary>
        private List<Tile> _shown = new List<Tile>();

        private sealed class Tile
        {
            public CustomSet Set;
            public int Pos;
            public MtgCardInfo Info;
            public ECardBorderType Border;
            public bool Foil;
            public int Owned;
            public CardData Data => new CardData { expansionType = Set.Expansion, monsterType = Set.Monsters[Pos].MonsterType, borderType = Border, isFoil = Foil };
        }
        private Vector2 _gridScroll, _deckScroll, _listScroll;
        private string _message = "";
        private float _messageUntil;
        private (CustomSet set, int pos, CardData data)? _hover;
        private float _scale = 1f;
        private Vector2 _offset;
        private GUIStyle _text, _small, _title, _button, _big, _toggle;
        // Sets dropdown (multi-select popup)
        private bool _setsOpen;
        private Rect _setsButton, _setsPopup;
        private string _setSearch = "";
        private Vector2 _setScroll;

        public static bool IsOpen => _inst != null && _inst.enabled;
        private bool _armed;

        public static void Open()
        {
            MtgSession.EnsureRunner();
            var go = GameObject.Find("TCGCC_Mtg");
            _inst = go.GetComponent<MtgDeckBuilderUI>() ?? go.AddComponent<MtgDeckBuilderUI>();
            _inst._editing = null;
            _inst._confirmDelete = null;
            _inst._all = null; // sets may have changed since last time
            _inst._armed = false; // ignore the click that opened us (it could land on "+ New deck")
            _inst._dirty = true;
            _inst.enabled = true;
        }

        private void Close()
        {
            enabled = false;
            PlayCardGameManager.OnCloseDeckListScreen(); // vanilla: back to the workbench, resets its editing state
        }

        /// <summary>A right-clicked card waiting for Forge's "can it be a commander?" answer.</summary>
        private (MtgDeckStore.Deck deck, CustomSet set, int pos, ECardBorderType border, bool foil)? _pendingCommander;

        private void ResolvePendingCommander()
        {
            if (_pendingCommander == null) return;
            var p = _pendingCommander.Value;
            var card = MtgMode.ToMtg(p.set.Def, p.set.Card(p.pos));
            if (card == null) { _pendingCommander = null; return; }
            var answer = MtgDeckCheck.CanBeCommander(card);
            if (!answer.Done) return;
            _pendingCommander = null;
            if (!answer.Ok) { Say(answer.Reason ?? $"{card.Name} can't be a commander"); return; }
            string why = MtgDeckStore.SetCommander(p.deck, p.set, p.pos, p.border, p.foil);
            Say(why ?? $"{card.Name} is your commander");
            _dirty = true;
        }

        private void Update()
        {
            ResolvePendingCommander();
            if (!Input.GetKeyDown(KeyCode.Escape)) return;
            if (_setsOpen) _setsOpen = false;
            else if (_confirmDelete != null) _confirmDelete = null;
            else if (_editing != null) _editing = null;
            else Close();
        }

        private void Say(string msg)
        {
            _message = msg;
            _messageUntil = Time.unscaledTime + 3f;
        }

        // ------------------------------------------------------------------ data

        private void EnsureCards()
        {
            if (_all != null) return;
            _all = new List<(CustomSet, int, MtgCardInfo)>();
            foreach (var set in Registry.Sets)
                for (int pos = 0; pos < set.Def.Cards.Count; pos++)
                {
                    var info = MtgDeckStore.Info(set, pos);
                    if (info != null && !info.IsBasicLand) _all.Add((set, pos, info)); // basics are free: land row
                }
        }

        private void Refresh()
        {
            EnsureCards();
            if (!_dirty && _filter.Search == _lastSearch) return;
            _lastSearch = _filter.Search;
            _dirty = false;
            var tiles = new List<Tile>();
            foreach (var c in _all)
            {
                if (MtgDeckStore.Owned(c.set, c.pos) <= 0 || !MtgDeckRules.Matches(c.info, _filter)) continue;
                for (int f = 0; f < 2; f++)                             // non-foil versions first, then foils
                    for (int b = 0; b < CardStore.BordersPerCard; b++)  // Base, 1st Edition, Silver, Gold, EX, Full Art
                    {
                        int n = MtgDeckStore.OwnedVariant(c.set, c.pos, (ECardBorderType)b, f == 1);
                        if (n > 0) tiles.Add(new Tile { Set = c.set, Pos = c.pos, Info = c.info, Border = (ECardBorderType)b, Foil = f == 1, Owned = n });
                    }
            }
            // Sort keeps each card's versions together (stable sort by the card's info)
            _shown = MtgDeckRules.Sort(tiles, t => t.Info, _sort).ToList();
        }

        private static Sprite ArtOf(CustomSet set, int pos) => set?.CardImage(pos);

        /// <summary>The card as the game draws it (border, foil, holo); flat art until that picture is ready.</summary>
        private static void DrawCard(Rect r, CardData data, Sprite art, string fallback, GUIStyle style, bool live = false)
        {
            var tex = live ? CardRenderCache.GetLive(data) : CardRenderCache.Get(data);
            if (tex != null) GUI.DrawTexture(r, tex, ScaleMode.ScaleToFit);
            else DrawCardImage(r, art, fallback, style);
        }

        private static string VersionLabel(ECardBorderType border, bool foil)
        {
            string b = border == ECardBorderType.FirstEdition ? "1st Edition" : border == ECardBorderType.FullArt ? "Full Art" : border.ToString();
            return foil ? (border == ECardBorderType.Base ? "Foil" : b + " Foil") : b;
        }

        private static void DrawCardImage(Rect r, Sprite art, string fallback, GUIStyle style)
        {
            if (art != null)
            {
                var tr = art.textureRect;
                var tex = art.texture;
                GUI.DrawTextureWithTexCoords(r, tex, new Rect(tr.x / tex.width, tr.y / tex.height, tr.width / tex.width, tr.height / tex.height));
            }
            else
            {
                GUI.Box(r, "");
                GUI.Label(new Rect(r.x + 6, r.y + 6, r.width - 12, r.height - 12), fallback, style);
            }
        }

        // ------------------------------------------------------------------ drawing

        private void Styles()
        {
            if (_text != null) return;
            _text = new GUIStyle(GUI.skin.label) { fontSize = 18, wordWrap = true, richText = true };
            _small = new GUIStyle(_text) { fontSize = 14 };
            _title = new GUIStyle(_text) { fontSize = 28, fontStyle = FontStyle.Bold };
            _button = new GUIStyle(GUI.skin.button) { fontSize = 16, richText = true };
            _big = new GUIStyle(GUI.skin.button) { fontSize = 20, fontStyle = FontStyle.Bold };
            _toggle = new GUIStyle(GUI.skin.button) { fontSize = 15, richText = true };
        }

        private void OnGUI()
        {
            Styles();
            GUI.depth = -110;
            // Until the mouse is pressed inside the builder, swallow mouse events: the workbench click that opened it
            // must not also press a builder button (two empty "MTG Deck" decks appeared that way).
            if (!_armed)
            {
                var e = Event.current;
                if (e.type == EventType.MouseDown && !Input.GetMouseButton(1)) _armed = true;
                else if (e.type == EventType.MouseUp || e.type == EventType.MouseDrag) e.Use();
            }
            _scale = Mathf.Min(Screen.width / W, Screen.height / H);
            _offset = new Vector2((Screen.width - W * _scale) / 2f, (Screen.height - H * _scale) / 2f);
            GUI.matrix = Matrix4x4.TRS(new Vector3(_offset.x, _offset.y, 0f), Quaternion.identity, new Vector3(_scale, _scale, 1f));
            GUI.color = new Color(0.06f, 0.06f, 0.08f, 0.96f);
            GUI.DrawTexture(new Rect(0, 0, W, H), Texture2D.whiteTexture);
            GUI.color = Color.white;
            _hover = null;

            // Sets dropdown open: a click outside it closes it (and is used up so nothing underneath reacts).
            if (_setsOpen && _editing != null && Event.current.type == EventType.MouseDown &&
                !_setsPopup.Contains(Event.current.mousePosition) && !_setsButton.Contains(Event.current.mousePosition))
            {
                _setsOpen = false;
                Event.current.Use();
            }
            if (_editing == null) { _setsOpen = false; DrawDeckList(); }
            else
            {
                GUI.enabled = !_setsOpen; // IMGUI gives clicks to the first control drawn: keep the editor inert under the popup
                DrawEditor(_editing);
                GUI.enabled = true;
                if (_setsOpen) DrawSetsPopup();
            }

            if (_confirmDelete != null) DrawConfirmDelete();
            if (_hover.HasValue) DrawPreview(_hover.Value.set, _hover.Value.pos, _hover.Value.data);
            if (Time.unscaledTime < _messageUntil)
                GUI.Label(new Rect(0, H - 60, W, 40), $"<color=#ffcc66><b>{_message}</b></color>", new GUIStyle(_text) { alignment = TextAnchor.MiddleCenter, fontSize = 22 });
        }

        // ---- deck list

        private void DrawDeckList()
        {
            bool modal = _confirmDelete != null;
            GUI.enabled = !modal;
            GUI.Label(new Rect(40, 24, 900, 44), "MTG Decks", _title);
            GUI.Label(new Rect(40, 70, 1400, 30), "<i>Build decks from the MTG cards in your collection. Cards in a deck leave your binder until you remove them. " +
                                                  "A deck needs at least 60 cards; a Commander deck is a commander + 99 different cards (Forge checks it).</i>", _small);
            if (GUI.Button(new Rect(W - 260, 24, 220, 50), "Close", _big)) Close();

            var decks = MtgDeckStore.Decks;
            var view = new Rect(40, 120, W - 80, H - 240);
            _listScroll = GUI.BeginScrollView(view, _listScroll, new Rect(0, 0, view.width - 20, (decks.Count + 1) * 78));
            for (int i = 0; i < decks.Count; i++)
            {
                var d = decks[i];
                var row = new Rect(0, i * 78, view.width - 24, 70);
                GUI.Box(row, "");
                bool active = MtgDeckStore.Active == i;
                string state = Status(d, short_: true);
                string kind = d.IsCommander ? " <color=#c9a0ff>Commander</color>" : "";
                GUI.Label(new Rect(row.x + 16, row.y + 8, 900, 30), $"{(active ? "<color=#ffd24a>★</color> " : "")}<b>{d.Name}</b>{kind}", _text);
                GUI.Label(new Rect(row.x + 16, row.y + 38, 900, 26), $"{d.Total}/{d.Target} cards   {state}", _small);
                if (GUI.Button(new Rect(row.xMax - 560, row.y + 12, 170, 46), "Edit", _button)) { _editing = d; _dirty = true; }
                GUI.enabled = !modal && CanPlay(d) && !active;
                if (GUI.Button(new Rect(row.xMax - 380, row.y + 12, 190, 46), active ? "Active" : "Set active", _button))
                    Say(MtgDeckStore.SetActive(d) ? $"{d.Name} is now your MTG deck" : StripTags(Status(d, short_: false)));
                GUI.enabled = !modal;
                if (GUI.Button(new Rect(row.xMax - 180, row.y + 12, 170, 46), "Delete", _button)) _confirmDelete = d;
            }
            if (GUI.Button(new Rect(0, decks.Count * 78, 300, 60), "+ New deck", _big))
            {
                _editing = MtgDeckStore.Create();
                _dirty = true;
            }
            if (GUI.Button(new Rect(320, decks.Count * 78, 380, 60), "+ New Commander deck", _big))
            {
                _editing = MtgDeckStore.Create(commander: true);
                _dirty = true;
                Say("Right-click a legendary creature to make it your commander, then add 99 different cards");
            }
            GUI.EndScrollView();
            GUI.enabled = true;
        }

        private void DrawConfirmDelete()
        {
            GUI.color = new Color(0, 0, 0, 0.6f);
            GUI.DrawTexture(new Rect(0, 0, W, H), Texture2D.whiteTexture);
            GUI.color = Color.white;
            var r = new Rect(W / 2 - 330, H / 2 - 120, 660, 240);
            GUI.Box(r, "");
            GUI.Box(r, "");
            GUI.Label(new Rect(r.x + 20, r.y + 20, r.width - 40, 90),
                $"<b>Delete {_confirmDelete.Name}?</b>\n<size=16>Its cards go back to your binder.</size>", new GUIStyle(_text) { fontSize = 24 });
            if (GUI.Button(new Rect(r.x + 20, r.yMax - 90, 290, 70), "Delete", _big))
            {
                MtgDeckStore.Delete(_confirmDelete);
                if (_editing == _confirmDelete) _editing = null;
                _confirmDelete = null;
                _dirty = true;
            }
            if (GUI.Button(new Rect(r.xMax - 310, r.yMax - 90, 290, 70), "Cancel", _big)) _confirmDelete = null;
        }

        // ---- editor

        private void DrawEditor(MtgDeckStore.Deck deck)
        {
            Refresh();
            // Top bar
            if (GUI.Button(new Rect(20, 16, 160, 48), "◀ Decks", _big)) { _editing = null; return; }
            GUI.Label(new Rect(200, 24, 80, 32), "Name", _text);
            deck.Name = GUI.TextField(new Rect(270, 18, 420, 44), deck.Name ?? "", 40, new GUIStyle(GUI.skin.textField) { fontSize = 22 });
            int total = deck.Total;
            bool countOk = deck.IsCommander ? total == deck.Target : total >= deck.Target;
            string countCol = countOk ? "#70ff80" : "#ffaa55";
            GUI.Label(new Rect(710, 20, 260, 44), $"<size=28><b><color={countCol}>{total}</color>/{deck.Target}</b></size>", _text);
            GUI.Label(new Rect(900, 18, W - 260 - 920, 52), Status(deck, short_: false), new GUIStyle(_text) { fontSize = 16 });
            bool active = MtgDeckStore.ActiveDeck == deck;
            bool enabledBefore = GUI.enabled; // may be off: the Sets popup keeps the editor inert (restore, never force on)
            GUI.enabled = enabledBefore && CanPlay(deck) && !active;
            if (GUI.Button(new Rect(W - 260, 16, 240, 48), active ? "★ Active deck" : "Set active", _big))
                Say(MtgDeckStore.SetActive(deck) ? $"{deck.Name} is now your MTG deck" : StripTags(Status(deck, short_: false)));
            GUI.enabled = enabledBefore;

            DrawFilters(new Rect(20, 76, 1330, 150));
            DrawGrid(deck, new Rect(20, 232, 1330, H - 250));
            DrawDeckPanel(deck, new Rect(1370, 76, W - 1390, H - 94));
        }

        private void DrawFilters(Rect r)
        {
            GUI.color = new Color(1, 1, 1, 0.06f);
            GUI.DrawTexture(r, Texture2D.whiteTexture);
            GUI.color = Color.white;
            float x = r.x + 10, y = r.y + 8;
            GUI.Label(new Rect(x, y + 6, 80, 30), "Search", _text);
            _filter.Search = GUI.TextField(new Rect(x + 80, y, 420, 38), _filter.Search ?? "", 60, new GUIStyle(GUI.skin.textField) { fontSize = 18 });
            if (GUI.Button(new Rect(x + 506, y, 40, 38), "✕", _toggle)) _filter.Search = "";
            float sx = x + 570;
            GUI.Label(new Rect(sx, y + 6, 60, 30), "Sort", _text);
            sx += 56;
            foreach (MtgSort s in Enum.GetValues(typeof(MtgSort)))
            {
                string label = s == MtgSort.ManaValue ? "Mana value" : s.ToString();
                if (Toggle(new Rect(sx, y, 120, 38), label, _sort == s)) { _sort = s; _dirty = true; }
                sx += 124;
            }
            GUI.Label(new Rect(r.xMax - 250, y + 8, 240, 30), $"{_shown.Count} cards", new GUIStyle(_small) { alignment = TextAnchor.UpperRight });

            // Colours + mana value
            y += 46;
            float cx = x;
            foreach (var (key, label) in new[] { ("W", "W"), ("U", "U"), ("B", "B"), ("R", "R"), ("G", "G"), ("C", "Colourless"), ("M", "Multi") })
            {
                float w = key.Length == 1 && label.Length == 1 ? 46 : 110;
                if (Toggle(new Rect(cx, y, w, 38), ColorLabel(key, label), _filter.Colors.Contains(key))) Flip(_filter.Colors, key);
                cx += w + 4;
            }
            cx += 20;
            GUI.Label(new Rect(cx, y + 6, 60, 30), "Cost", _text);
            cx += 54;
            for (int mv = 0; mv <= 6; mv++)
            {
                if (Toggle(new Rect(cx, y, 46, 38), mv == 6 ? "6+" : mv.ToString(), _filter.ManaValues.Contains(mv))) Flip(_filter.ManaValues, mv);
                cx += 50;
            }
            cx += 20;
            foreach (var rar in MtgDeckRules.RarityOrder)
            {
                if (Toggle(new Rect(cx, y, 100, 38), char.ToUpper(rar[0]) + rar.Substring(1), _filter.Rarities.Contains(rar))) Flip(_filter.Rarities, rar);
                cx += 104;
            }

            // Types + sets
            y += 46;
            float tx = x;
            foreach (var t in MtgDeckRules.CardTypes)
            {
                if (Toggle(new Rect(tx, y, 120, 38), t, _filter.Types.Contains(t))) Flip(_filter.Types, t);
                tx += 124;
            }
            tx += 16;
            // Sets: a dropdown multi-select (players can have many sets installed)
            int nSets = _filter.Sets.Count;
            string setsLabel = nSets == 0 ? "Sets: All ▾" : nSets == 1 ? $"Sets: {SetCode(_filter.Sets.First())} ▾" : $"Sets: {nSets} selected ▾";
            _setsButton = new Rect(tx, y, 190, 38);
            bool wasEnabled = GUI.enabled;
            GUI.enabled = true; // the button itself also closes the popup
            if (Toggle(_setsButton, setsLabel, nSets > 0 || _setsOpen))
            {
                _setsOpen = !_setsOpen;
                _setSearch = "";
                _setScroll = Vector2.zero;
            }
            GUI.enabled = wasEnabled;
            if (GUI.Button(new Rect(r.xMax - 110, y, 100, 38), "Clear", _toggle))
            {
                _filter.Search = "";
                _filter.Colors.Clear(); _filter.Types.Clear(); _filter.ManaValues.Clear(); _filter.Rarities.Clear(); _filter.Sets.Clear();
                _dirty = true;
            }
        }

        private static string ColorLabel(string key, string label)
        {
            switch (key)
            {
                case "W": return "<color=#fff6c8><b>W</b></color>";
                case "U": return "<color=#7fb7ff><b>U</b></color>";
                case "B": return "<color=#b9a3c9><b>B</b></color>";
                case "R": return "<color=#ff7f6a><b>R</b></color>";
                case "G": return "<color=#7fd98a><b>G</b></color>";
                default: return label;
            }
        }

        private bool Toggle(Rect r, string label, bool on)
        {
            var old = GUI.backgroundColor;
            if (on) GUI.backgroundColor = new Color(0.35f, 0.75f, 1f);
            bool clicked = GUI.Button(r, label, _toggle);
            GUI.backgroundColor = old;
            return clicked;
        }

        private void Flip<T>(HashSet<T> set, T v)
        {
            if (!set.Remove(v)) set.Add(v);
            _dirty = true;
        }

        private static string SetCode(string setId)
        {
            var set = Registry.Get(setId);
            return set?.Def.Mtg?.SetCode ?? setId;
        }

        /// <summary>The Sets dropdown: search, All/None, and a checkbox per MTG set (code, name, owned cards).</summary>
        private void DrawSetsPopup()
        {
            EnsureCards();
            var sets = Registry.Sets.Where(st => st.Def.Mtg != null).ToList();
            var ownedBySet = _all.GroupBy(c => c.set).ToDictionary(g => g.Key, g => g.Count(c => MtgDeckStore.Owned(c.set, c.pos) > 0));
            var shown = sets.Where(st => string.IsNullOrWhiteSpace(_setSearch)
                                         || (st.Def.Name ?? "").IndexOf(_setSearch, StringComparison.OrdinalIgnoreCase) >= 0
                                         || (st.Def.Mtg.SetCode ?? "").IndexOf(_setSearch, StringComparison.OrdinalIgnoreCase) >= 0)
                            .OrderByDescending(st => ownedBySet.TryGetValue(st, out int n) && n > 0)
                            .ThenBy(st => st.Def.Name, StringComparer.OrdinalIgnoreCase).ToList();

            const float rowH = 36f;
            float h = Mathf.Min(560f, 110f + Mathf.Max(1, shown.Count) * rowH);
            _setsPopup = new Rect(_setsButton.x, _setsButton.yMax + 4, 520, h);
            GUI.color = new Color(0.1f, 0.1f, 0.13f, 0.98f);
            GUI.DrawTexture(_setsPopup, Texture2D.whiteTexture);
            GUI.color = new Color(0.35f, 0.75f, 1f);
            GUI.DrawTexture(new Rect(_setsPopup.x, _setsPopup.y, _setsPopup.width, 2), Texture2D.whiteTexture);
            GUI.color = Color.white;

            float x = _setsPopup.x + 10, y = _setsPopup.y + 10, w = _setsPopup.width - 20;
            GUI.SetNextControlName("tcgcc_setsearch");
            _setSearch = GUI.TextField(new Rect(x, y, w - 150, 36), _setSearch ?? "", 40, new GUIStyle(GUI.skin.textField) { fontSize = 17 });
            if (GUI.Button(new Rect(x + w - 140, y, 140, 36), "All sets", _toggle)) { _filter.Sets.Clear(); _dirty = true; } // no ticks = no set filter
            y += 44;
            GUI.Label(new Rect(x, y, w, 24), _filter.Sets.Count == 0 ? "<i>Showing all sets — tick sets to narrow it down</i>"
                                                                      : $"<i>{_filter.Sets.Count(id => Registry.Get(id) != null)} of {sets.Count} sets ticked</i>", _small);
            y += 28;

            var view = new Rect(x, y, w, _setsPopup.yMax - y - 8);
            _setScroll = GUI.BeginScrollView(view, _setScroll, new Rect(0, 0, w - 20, shown.Count * rowH));
            for (int i = 0; i < shown.Count; i++)
            {
                var st = shown[i];
                bool on = _filter.Sets.Contains(st.Def.Id);
                int owned = ownedBySet.TryGetValue(st, out int n) ? n : 0;
                var row = new Rect(0, i * rowH, w - 24, rowH - 3);
                var old = GUI.backgroundColor;
                if (on) GUI.backgroundColor = new Color(0.35f, 0.75f, 1f);
                string label = $"{(on ? "☑" : "☐")}  <b>{st.Def.Mtg.SetCode}</b>  {st.Def.Name}   <color=#aaaaaa>{(owned > 0 ? owned + " owned" : "none owned")}</color>";
                if (GUI.Button(row, label, new GUIStyle(_toggle) { alignment = TextAnchor.MiddleLeft, clipping = TextClipping.Clip }))
                {
                    if (!_filter.Sets.Remove(st.Def.Id)) _filter.Sets.Add(st.Def.Id);
                    _dirty = true;
                }
                GUI.backgroundColor = old;
            }
            if (shown.Count == 0) GUI.Label(new Rect(4, 4, w - 30, 30), "<i>No set matches that search.</i>", _small);
            GUI.EndScrollView();
        }

        private void DrawGrid(MtgDeckStore.Deck deck, Rect view)
        {
            const float tw = 150f, th = 210f, cellW = 160f, cellH = 244f; // card pictures are 300×420 (5:7)
            int cols = Mathf.Max(1, (int)((view.width - 20) / cellW));
            int rows = (_shown.Count + cols - 1) / cols;
            _gridScroll = GUI.BeginScrollView(view, _gridScroll, new Rect(0, 0, view.width - 20, Mathf.Max(rows * cellH, view.height)));
            int firstRow = Mathf.Max(0, (int)(_gridScroll.y / cellH) - 1), lastRow = Mathf.Min(rows - 1, (int)((_gridScroll.y + view.height) / cellH) + 1);
            for (int row = firstRow; row <= lastRow; row++)
                for (int col = 0; col < cols; col++)
                {
                    int i = row * cols + col;
                    if (i >= _shown.Count) break;
                    var t = _shown[i];
                    var (set, pos, info, owned) = (t.Set, t.Pos, t.Info, t.Owned);
                    var data = t.Data;
                    var tile = new Rect(col * cellW + 4, row * cellH + 4, tw, th);
                    int inDeck = MtgDeckStore.CopiesOf(deck, info.Name);
                    bool full = MtgDeckRules.CanAddCopy(info, inDeck, deck.IsCommander) != null;
                    GUI.color = full ? new Color(0.55f, 0.55f, 0.55f) : Color.white;
                    DrawCard(tile, data, ArtOf(set, pos), $"<b>{info.Name}</b>\n{info.ManaCost}\n{info.TypeLine}", _small);
                    GUI.color = Color.white;
                    // badges: owned of this version (left), copies of the card in the deck (right)
                    Badge(new Rect(tile.x + 4, tile.yMax - 30, 64, 26), $"×{owned}", new Color(0, 0, 0, 0.75f));
                    bool isCmd = deck.Commander?.Live == true && deck.Commander.Set == set && deck.Commander.Pos == pos;
                    if (isCmd) Badge(new Rect(tile.xMax - 100, tile.y + 4, 96, 26), "Commander", new Color(0.45f, 0.25f, 0.65f, 0.95f));
                    else if (inDeck > 0) Badge(new Rect(tile.xMax - 68, tile.y + 4, 64, 26), $"{inDeck} in deck", new Color(0.1f, 0.35f, 0.6f, 0.9f));
                    string version = t.Border == ECardBorderType.Base && !t.Foil ? "" : $" <color=#ffd24a>{VersionLabel(t.Border, t.Foil)}</color>";
                    GUI.Label(new Rect(tile.x, tile.yMax + 2, tile.width, 24), info.Name + version,
                        new GUIStyle(_small) { alignment = TextAnchor.UpperCenter, wordWrap = false, clipping = TextClipping.Clip });
                    if (tile.Contains(Event.current.mousePosition)) _hover = (set, pos, data);
                    // Commander decks: right-click a card = make it the commander (Forge decides whether it can be one)
                    if (deck.IsCommander && GUI.enabled && Event.current.type == EventType.MouseDown && Event.current.button == 1 &&
                        tile.Contains(Event.current.mousePosition))
                    {
                        Event.current.Use();
                        _pendingCommander = (deck, set, pos, t.Border, t.Foil); // Forge says whether it can be one (Update)
                        Say($"Asking Forge whether {info.Name} can be a commander…");
                    }
                    if (GUI.Button(tile, "", GUIStyle.none))
                    {
                        string why = MtgDeckStore.AddVariant(deck, set, pos, t.Border, t.Foil, info);
                        if (why != null) Say(why);
                        _dirty = true;
                    }
                }
            if (_shown.Count == 0)
                GUI.Label(new Rect(20, 20, view.width - 60, 60), "<i>No owned MTG cards match these filters.</i>", _text);
            GUI.EndScrollView();
        }

        private void Badge(Rect r, string text, Color bg)
        {
            GUI.color = bg;
            GUI.DrawTexture(r, Texture2D.whiteTexture);
            GUI.color = Color.white;
            GUI.Label(r, text, new GUIStyle(_small) { alignment = TextAnchor.MiddleCenter, fontSize = 13 });
        }

        private void DrawDeckPanel(MtgDeckStore.Deck deck, Rect r)
        {
            GUI.color = new Color(1, 1, 1, 0.06f);
            GUI.DrawTexture(r, Texture2D.whiteTexture);
            GUI.color = Color.white;
            float x = r.x + 10, w = r.width - 20;

            // Mana curve (non-land cards) + colours
            var live = deck.Entries.Where(e => e.Live && e.Save.Count > 0)
                .Select(e => (e, info: MtgDeckStore.Info(e.Set, e.Pos))).Where(t => t.info != null).ToList();
            var curve = new int[7];
            foreach (var (e, info) in live)
                if (!info.FrontType.Contains("Land")) curve[Mathf.Min(6, (int)info.Cmc)] += e.Save.Count;
            int maxBar = Mathf.Max(1, curve.Max());
            float bx = x, by = r.y + 10, barW = (w - 6 * 6) / 7f, barH = 80;
            for (int i = 0; i < 7; i++)
            {
                float h = barH * curve[i] / maxBar;
                GUI.color = new Color(0.35f, 0.75f, 1f, 0.85f);
                GUI.DrawTexture(new Rect(bx, by + barH - h, barW, h), Texture2D.whiteTexture);
                GUI.color = Color.white;
                GUI.Label(new Rect(bx, by + barH, barW, 20), i == 6 ? "6+" : i.ToString(), new GUIStyle(_small) { alignment = TextAnchor.UpperCenter, fontSize = 12 });
                if (curve[i] > 0) GUI.Label(new Rect(bx, by + barH - h - 18, barW, 18), curve[i].ToString(), new GUIStyle(_small) { alignment = TextAnchor.UpperCenter, fontSize = 12 });
                bx += barW + 6;
            }

            // Free basic lands
            float y = by + barH + 30;
            if (deck.IsCommander)
            {
                var slot = new Rect(x, y - 4, w, 50);
                GUI.color = new Color(0.45f, 0.25f, 0.65f, 0.35f);
                GUI.DrawTexture(slot, Texture2D.whiteTexture);
                GUI.color = Color.white;
                var c = deck.Commander;
                if (c?.Live == true)
                {
                    var cinfo = MtgDeckStore.Info(c.Set, c.Pos);
                    if (GUI.Button(new Rect(slot.x + 4, slot.y + 4, slot.width - 8, slot.height - 8),
                            $"<b>Commander:</b> {cinfo?.Name}  <color=#aaaaaa>{cinfo?.ManaCost}</color>  <size=12>(click to put it back)</size>",
                            new GUIStyle(_button) { alignment = TextAnchor.MiddleLeft, fontSize = 15 }))
                    {
                        MtgDeckStore.ClearCommander(deck);
                        _dirty = true;
                    }
                    if (slot.Contains(Event.current.mousePosition)) _hover = (c.Set, c.Pos, c.Data);
                }
                else GUI.Label(new Rect(slot.x + 10, slot.y + 4, slot.width - 20, slot.height - 8),
                    "<b>Commander:</b> <i>none - right-click a legendary creature on the left</i>", _small);
                y += 58;
            }
            GUI.Label(new Rect(x, y, w, 26), "<b>Basic lands</b> <size=13>(free)</size>", _text);
            y += 28;
            float lw = (w - 10) / 3f;
            for (int i = 0; i < MtgDeckRules.Basics.Length; i++)
            {
                var (name, color) = MtgDeckRules.Basics[i];
                var cell = new Rect(x + (i % 3) * (lw + 5), y + (i / 3) * 44, lw, 40);
                GUI.Box(cell, "");
                GUI.Label(new Rect(cell.x + 6, cell.y + 8, lw - 100, 26), $"{ColorLabel(color, name.Substring(0, 1))} {name}", _small);
                bool enabledBefore = GUI.enabled;
                GUI.enabled = enabledBefore && deck.Basics(name) > 0;
                if (GUI.Button(new Rect(cell.xMax - 94, cell.y + 5, 30, 30), "−", _toggle)) { MtgDeckStore.ChangeBasic(deck, name, -1); }
                GUI.enabled = enabledBefore;
                GUI.Label(new Rect(cell.xMax - 64, cell.y + 5, 30, 30), $"<b>{deck.Basics(name)}</b>", new GUIStyle(_small) { alignment = TextAnchor.MiddleCenter });
                if (GUI.Button(new Rect(cell.xMax - 34, cell.y + 5, 30, 30), "+", _toggle)) { MtgDeckStore.ChangeBasic(deck, name, +1); }
            }
            y += 96;

            // Deck list by type
            var groups = new[] { "Creatures", "Spells", "Lands" };
            string GroupOf(MtgCardInfo i) => i.FrontType.Contains("Creature") ? "Creatures" : i.FrontType.Contains("Land") ? "Lands" : "Spells";
            var ordered = live.OrderBy(t => Array.IndexOf(groups, GroupOf(t.info))).ThenBy(t => t.info.Cmc).ThenBy(t => t.info.Name).ToList();
            var listView = new Rect(x, y, w, r.yMax - y - 10);
            if (listView.height < 60) listView.height = 60;
            float rowH = 30;
            int headerCount = groups.Count(g => ordered.Any(t => GroupOf(t.info) == g));
            _deckScroll = GUI.BeginScrollView(listView, _deckScroll, new Rect(0, 0, w - 20, (ordered.Count + headerCount) * rowH + 10));
            float ry = 0;
            foreach (var g in groups)
            {
                var items = ordered.Where(t => GroupOf(t.info) == g).ToList();
                if (items.Count == 0) continue;
                GUI.Label(new Rect(0, ry, w - 24, rowH), $"<b>{g}</b> ({items.Sum(t => t.e.Save.Count)})", _text);
                ry += rowH;
                foreach (var (e, info) in items)
                {
                    var row = new Rect(0, ry, w - 24, rowH - 2);
                    string variant = e.Foil || e.Border != ECardBorderType.Base ? $" <size=12><color=#ffd24a>{VersionLabel(e.Border, e.Foil)}</color></size>" : "";
                    if (GUI.Button(row, $"{e.Save.Count}×  {info.Name}{variant}   <color=#aaaaaa>{info.ManaCost}</color>",
                            new GUIStyle(_button) { alignment = TextAnchor.MiddleLeft, fontSize = 14 }))
                    {
                        MtgDeckStore.RemoveOne(deck, e); // back to the binder
                        _dirty = true;
                        GUI.EndScrollView();
                        return;
                    }
                    if (row.Contains(Event.current.mousePosition)) _hover = (e.Set, e.Pos, e.Data);
                    ry += rowH;
                }
            }
            if (ordered.Count == 0)
                GUI.Label(new Rect(0, 0, w - 24, 80), "<i>Click cards on the left to add them. Click a card here to take it out again.</i>", _small);
            GUI.EndScrollView();
        }

        /// <summary>Playable: the builder's counting rules, and for Commander decks Forge's verdict (DeckFormat.Commander).</summary>
        private static bool CanPlay(MtgDeckStore.Deck d)
        {
            if (!d.Valid) return false;
            if (!d.IsCommander) return true;
            var r = MtgDeckStore.ForgeCheck(d);
            return r != null && r.Done && r.Ok;
        }

        private static string Status(MtgDeckStore.Deck d, bool short_)
        {
            if (!d.Valid) return $"<color=#ffaa55>{d.Problems.FirstOrDefault()}</color>";
            if (!d.IsCommander) return "<color=#70ff80>✓ Ready to play</color>";
            var r = MtgDeckStore.ForgeCheck(d);
            if (r == null || !r.Done) return "<color=#cccccc>Checking with Forge…</color>";
            if (r.Ok) return "<color=#70ff80>✓ Forge: legal Commander deck</color>";
            string why = string.Join(" ", r.Problems).Replace("\n\n", ": ").Replace("\n", ", ");
            if (short_ && why.Length > 90) why = why.Substring(0, 90) + "…";
            return $"<color=#ffaa55>Forge: {why}</color>";
        }

        private static string StripTags(string s) => System.Text.RegularExpressions.Regex.Replace(s ?? "", "<[^>]+>", "");

        /// <summary>Hover preview; double-faced cards show the back face beside the front.</summary>
        private void DrawPreview(CustomSet set, int pos, CardData data)
        {
            float pw = 360, ph = pw * 1.4f, gap = 12;
            var back = MtgCardFaces.BackFaceOf(data);
            float total = back != null ? pw * 2 + gap : pw;
            var mouse = Event.current.mousePosition;
            float px = mouse.x + 30 + total > W ? mouse.x - 30 - total : mouse.x + 30;
            px = Mathf.Clamp(px, 8, W - total - 8);
            float py = Mathf.Clamp(mouse.y - ph / 2, 8, H - ph - 8);
            var info = MtgDeckStore.Info(set, pos);
            DrawCard(new Rect(px, py, pw, ph), data, ArtOf(set, pos),
                $"<b><size=20>{info?.Name}</size></b>  {info?.ManaCost}\n<i>{info?.TypeLine}</i>\n\n{info?.Text}", _text, live: true);
            if (back == null) return;
            var r = new Rect(px + pw + gap, py, pw, ph);
            var tex = CardRenderCache.GetLive(back, lane: 1); // preview size + animated foil, like the front
            if (tex != null) GUI.DrawTexture(r, tex, ScaleMode.ScaleToFit);
            else DrawCard(r, back, null, $"<b><size=20>{set.Card(pos).Mtg?.BackName}</size></b>", _text);
            GUI.Label(new Rect(r.x, r.yMax - 30, r.width, 26), $"<color=#ffffff><b>Back: {set.Card(pos).Mtg?.BackName}</b></color>",
                new GUIStyle(_small) { alignment = TextAnchor.MiddleCenter });
        }
    }
}

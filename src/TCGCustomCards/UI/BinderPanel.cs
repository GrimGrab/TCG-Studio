using System;
using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Binder;
using TCGCustomCards.Patches;
using TCGCustomCards.Runtime;
using TMPro;
using UnityEngine;
using UnityEngine.UI;
using Object = UnityEngine.Object;

namespace TCGCustomCards.UI
{
    /// <summary>
    /// Enhanced binder panel: opens on Sort Album in place of the vanilla sort screen's content (one-slot-per-card layout,
    /// not graded). It lives inside m_SortAlbumScreen, so the game still treats it as the sort screen (cursor, page flips
    /// blocked, closed with the binder). Sets, sort, search and filters are picked first; Apply changes the binder once.
    /// Built in code from the screen's own background, button sprite and font. Controller: BinderPanel.Nav.cs.
    /// </summary>
    internal static partial class BinderPanel
    {
        private static readonly Color Selected = new Color(0.95f, 0.66f, 0.12f);
        private static readonly Color Normal = new Color(0.25f, 0.28f, 0.36f);
        private static readonly Color Dim = new Color(0.17f, 0.19f, 0.24f);
        private static readonly Color Partial = new Color(0.55f, 0.4f, 0.12f);
        private static readonly Color PanelColor = new Color(0.09f, 0.1f, 0.13f, 0.97f);
        private static readonly Color FieldColor = new Color(0.04f, 0.05f, 0.07f, 1f);

        private static CollectionBinderUI _ui;
        private static GameObject _root;
        private static readonly List<GameObject> _hiddenVanilla = new List<GameObject>();

        // Template look
        private static TMP_FontAsset _font;
        private static Color _textColor = Color.white;
        private static Sprite _rounded;
        private static float _u; // layout unit (≈ a text line)

        // State being edited
        private static BinderQuery _pending;
        private static ECardExpansionType _openExp;
        /// <summary>The panel is sorting the graded album (sets = a filter there; none picked = every set).</summary>
        private static bool _graded;
        /// <summary>Collections whose set list is folded away in the panel (kept while the game runs).</summary>
        private static readonly HashSet<string> _folded = new HashSet<string>();
        private static bool _foldedInit;
        private static Dictionary<ECardExpansionType, string> _names = new Dictionary<ECardExpansionType, string>();

        // Widgets
        private static TextMeshProUGUI _stats, _searchIn;
        private static TMP_InputField _search;
        private static RectTransform _setList, _sortList, _filterList;
        private static Button _apply;
        private static readonly List<(Button b, Func<bool> on)> _selectables = new List<(Button, Func<bool>)>();
        private static readonly List<(Image img, Func<Color> color)> _tinted = new List<(Image, Func<Color>)>();
        private static readonly List<ScrollRect> _scrolls = new List<ScrollRect>();
        private static readonly List<GameObject> _pageButtons = new List<GameObject>();

        public static bool IsOpen => _root != null && _root.activeInHierarchy;

        /// <summary>The search box has the keyboard: the binder's own keys must not act.</summary>
        public static bool Typing => IsOpen && _search != null && _search.isFocused;

        /// <summary>Frame of the last click on a panel button.</summary>
        private static int _clickFrame = -10;

        /// <summary>
        /// The binder must ignore its keys/clicks: while typing, on the frame a panel button was clicked (+1), and while the
        /// panel is driven by a controller. Apply/Close close the panel inside that click, and the binder then read the same
        /// mouse release as a click on the card below.
        /// </summary>
        public static bool BlockBinderInput
        {
            get
            {
                if (_swallow)
                {
                    // The Esc / controller button that closed the panel: its release must not also act in the binder
                    // (B = take card, A = close-up, X = close the binder, Y = open the panel again).
                    if (Input.GetKey(KeyCode.Escape) || Input.GetKeyUp(KeyCode.Escape) || PadInput.FaceButtonBusy()) return true;
                    _swallow = false;
                }
                return Typing || Time.frameCount - _clickFrame <= 1 || IsOpen && PadInput.Active;
            }
        }

        private static bool _swallow;

        /// <summary>
        /// Esc (the pause key) while the panel is open: leave the search box, or close the panel. True = handled, the pause
        /// menu must not open.
        /// </summary>
        public static bool HandleEscape()
        {
            if (!IsOpen) return false;
            if (Typing) _search.DeactivateInputField();
            else Close(); // plays the game's menu-close sound
            _swallow = true;
            return true;
        }

        // ------------------------------------------------------------------ open / close

        /// <summary>After vanilla opened its sort screen: show our panel instead of its content when the enhanced layout is on.</summary>
        public static void OnSortScreenOpened(CollectionBinderUI ui, bool graded)
        {
            if (ui == null || !ui.m_SortAlbumScreen.activeSelf) return;
            var ctrl = ui.m_CollectionAlbum;
            bool ours = ctrl != null && (graded ? EnhancedBinderPatches.GradedActive(ctrl) : EnhancedBinderPatches.Active(ctrl));
            if (!ours)
            {
                RestoreVanilla();
                if (_root != null) _root.SetActive(false);
                return;
            }
            try
            {
                if (_root == null || _ui != ui) Build(ui);
                HideVanilla();
                _openExp = EnhancedBinderPatches.Expansion(ctrl);
                _graded = graded;
                if (graded) _pending = BinderIndex.GradedQuery.Clone();
                else
                {
                    _pending = BinderIndex.Query.Clone();
                    _pending.Sets.Clear();
                    _pending.Sets.AddRange(BinderIndex.SetsFor(_openExp));
                }
                _root.SetActive(true);
                _search.text = _pending.Search;
                ReadSets();
                if (!_foldedInit)
                {
                    // First open: fold every collection except the ones being shown.
                    _foldedInit = true;
                    foreach (var (col, _) in Grouped())
                        if (!_pending.Sets.Any(e => Collections.Of(e) == col)) _folded.Add(col);
                    if (_pending.Sets.Count == 0) _folded.Clear();
                }
                RebuildSets();
                RebuildSorts();
                RebuildFilters();
                foreach (var b in _pageButtons) if (b != null) b.SetActive(!graded); // graded slabs: vanilla pages
                NavOpened();
            }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Binder panel failed, showing the game's sort screen: {e}");
                if (_root != null) _root.SetActive(false);
                RestoreVanilla();
            }
        }

        private static void HideVanilla()
        {
            var screen = _ui.m_SortAlbumScreen.transform;
            for (int i = 0; i < screen.childCount; i++)
            {
                var c = screen.GetChild(i).gameObject;
                if (c == _root || !c.activeSelf) continue;
                c.SetActive(false);
                _hiddenVanilla.Add(c);
            }
        }

        private static void RestoreVanilla()
        {
            foreach (var g in _hiddenVanilla) if (g != null) g.SetActive(true);
            _hiddenVanilla.Clear();
            NavClosed();
        }

        private static void Close()
        {
            _ui.CloseSortAlbumScreen();
            RestoreVanilla();
        }

        private static bool Changed() => _graded
            ? !_pending.SameAs(BinderIndex.GradedQuery)
            : !_pending.SameAs(BinderIndex.Query) || !_pending.Sets.SequenceEqual(BinderIndex.SetsFor(_openExp));

        private static void Apply() => Apply(null);

        /// <summary>Apply; then <paramref name="after"/> once the binder shows the result in place (not after a set switch,
        /// where the binder flips over to the other set).</summary>
        private static void Apply(Action<CollectionBinderFlipAnimCtrl> after)
        {
            var ctrl = _ui.m_CollectionAlbum;
            _pending.Search = _search.text ?? "";
            if (!Changed()) { Close(); after?.Invoke(ctrl); return; }
            if (_graded)
            {
                BinderIndex.GradedQuery = _pending.Clone();
                RestoreVanilla();
                SoundManager.GenericConfirm();
                _ui.CloseSortAlbumScreen();
                EnhancedBinderPatches.Rebuild(ctrl);
                after?.Invoke(ctrl);
                return;
            }
            if (_pending.Sets.Count == 0) return;
            BinderIndex.Query = _pending.Clone();
            RestoreVanilla();
            // The binder's own set must be one of the shown ones; keep it when it is (no flip-over animation).
            if (!_pending.Sets.Contains(_openExp))
            {
                _ui.OnPressSwitchExpansion((int)_pending.Sets[0]); // closes the screen; the binder flips over
            }
            else
            {
                SoundManager.GenericConfirm();
                _ui.CloseSortAlbumScreen();
                EnhancedBinderPatches.Rebuild(ctrl);
                after?.Invoke(ctrl);
            }
        }

        private static void Reset()
        {
            var sets = _graded ? new List<ECardExpansionType>() : _pending.Sets.ToList();
            _pending = new BinderQuery { Sort = _graded ? BinderSort.Grade : BinderSort.SetOrder };
            _pending.Sets.AddRange(sets); // Reset clears sort/search/filters, not the sets picked (graded: back to every set)
            _search.text = "";
            SoundManager.GenericPop(0.7f, 1.1f);
            RebuildSorts();
            RebuildFilters();
        }

        // ------------------------------------------------------------------ sets and collections

        private static bool IsMtg(ECardExpansionType exp) =>
            Registry.IsCustom(exp) && (Registry.Get(exp)?.Def.Cards.Any(c => c.Mtg != null) ?? false);

        private static string SetName(ECardExpansionType exp) =>
            _names.TryGetValue(exp, out var n) && !string.IsNullOrEmpty(n) ? n : exp.ToString();

        /// <summary>Vanilla sets (button index = expansion int, label from the game's own button) + every custom set.</summary>
        private static void ReadSets()
        {
            _names = new Dictionary<ECardExpansionType, string>();
            if (!VanillaFilter.HideCards)
                for (int i = 0; i < _ui.m_ExpansionBtnList.Count; i++)
                {
                    var b = _ui.m_ExpansionBtnList[i];
                    if (b == null || b.name.StartsWith("TCGCC_")) continue;
                    var label = b.GetComponentInChildren<TextMeshProUGUI>(true);
                    _names[(ECardExpansionType)i] = label != null ? label.text : null;
                }
            foreach (var s in Registry.Sets) _names[s.Expansion] = s.Def.Name;
        }

        private static List<(string name, List<(ECardExpansionType exp, string name)> sets)> Grouped() =>
            Collections.Group(_names.Keys.Select(e => (e, SetName(e))));

        /// <summary>Keeps the picked sets in display order (collection, release, name): Set order follows it.</summary>
        private static void OrderSets()
        {
            var order = Grouped().SelectMany(g => g.sets).Select(s => s.exp).ToList();
            var picked = new HashSet<ECardExpansionType>(_pending.Sets);
            _pending.Sets.Clear();
            _pending.Sets.AddRange(order.Where(picked.Contains));
        }

        private static void SetsChanged()
        {
            OrderSets();
            RebuildSets();
            RebuildSorts();
            RebuildFilters();
        }

        /// <summary>A set is in the selection (graded album: no pick = every set).</summary>
        private static bool InSel(ECardExpansionType e) => _graded && _pending.Sets.Count == 0 || _pending.Sets.Contains(e);

        /// <summary>The sets the panel works on: the picked ones, or every set (graded album, none picked).</summary>
        private static List<ECardExpansionType> Selection() => _graded && _pending.Sets.Count == 0 ? _names.Keys.ToList() : _pending.Sets;

        private static void ToggleSet(ECardExpansionType exp)
        {
            if (_graded)
            {
                // Graded: a filter. From "every set", a click shows only that set; un-picking the last goes back to every set.
                if (_pending.Sets.Count == 0) _pending.Sets.Add(exp);
                else if (!_pending.Sets.Remove(exp)) _pending.Sets.Add(exp);
                SoundManager.GenericPop(0.6f, 1.3f);
                SetsChanged();
                return;
            }
            // None picked is allowed while choosing (Apply waits for at least one).
            if (!_pending.Sets.Remove(exp)) _pending.Sets.Add(exp);
            SoundManager.GenericPop(0.6f, 1.3f);
            SetsChanged();
        }

        /// <summary>Collection row: all of its sets on, or (when they all are) off again.</summary>
        private static void ToggleCollection(List<(ECardExpansionType exp, string name)> sets)
        {
            var exps = sets.Select(s => s.exp).ToList();
            if (_graded && _pending.Sets.Count == 0)
            {
                _pending.Sets.AddRange(exps); // from every set: just this collection
                SoundManager.GenericPop(0.6f, 1.3f);
                SetsChanged();
                return;
            }
            if (exps.All(_pending.Sets.Contains))
            {
                var rest = _pending.Sets.Where(e => !exps.Contains(e)).ToList(); // may be none (graded: none = every set)
                _pending.Sets.Clear();
                _pending.Sets.AddRange(rest);
            }
            else
                foreach (var e in exps) if (!_pending.Sets.Contains(e)) _pending.Sets.Add(e);
            SoundManager.GenericPop(0.6f, 1.3f);
            SetsChanged();
        }

        private static void ToggleAll()
        {
            var all = _names.Keys.ToList();
            if (_graded)
            {
                _pending.Sets.Clear(); // every set
                SoundManager.GenericPop(0.6f, 1.3f);
                SetsChanged();
                return;
            }
            bool everything = all.All(_pending.Sets.Contains);
            _pending.Sets.Clear();
            if (!everything) _pending.Sets.AddRange(all); // all picked: All sets clears them
            SoundManager.GenericPop(0.6f, 1.3f);
            SetsChanged();
        }

        private static void RebuildSets()
        {
            Clear(_setList);
            var allBtn = MakeButton(_setList, "All sets", _u * 1.7f, ToggleAll);
            Tint(allBtn, () => _names.Keys.All(InSel) ? Selected : Normal);
            foreach (var (col, sets) in Grouped())
            {
                string c = col;
                var list = sets;
                int on = sets.Count(s => InSel(s.exp));
                bool folded = _folded.Contains(c);

                var row = Row(_setList, _u * 1.7f);
                var fold = MakeButton(row, folded ? "+" : "-", 0f, () =>
                {
                    if (!_folded.Remove(c)) _folded.Add(c);
                    SoundManager.GenericPop(0.5f, 1.4f);
                    RebuildSets();
                    Refresh();
                });
                fold.gameObject.AddComponent<LayoutElement>().preferredWidth = _u * 1.7f;
                Tint(fold, () => Dim);
                var head = MakeButton(row, $"{c}  ({on}/{sets.Count})", 0f, () => ToggleCollection(list));
                head.gameObject.AddComponent<LayoutElement>().flexibleWidth = 1f;
                Tint(head, () => on == list.Count ? Selected : on > 0 ? Partial : Dim);

                if (folded) continue;
                foreach (var (exp, name) in sets)
                {
                    var e = exp;
                    var setRow = Row(_setList, _u * 1.5f, indent: _u * 1.2f);
                    var b = MakeButton(setRow, name, 0f, () => ToggleSet(e));
                    b.gameObject.AddComponent<LayoutElement>().flexibleWidth = 1f;
                    Tint(b, () => InSel(e) ? Selected : Normal);
                }
            }
            var other = MakeButton(_setList, _graded ? "Back to your collection" : "Graded cards", _u * 1.7f, () =>
            {
                RestoreVanilla();
                if (_graded) _ui.OnPressSwitchExpansion((int)_openExp);
                else _ui.OnPressSwitchToGradedAlbum();
            });
            Tint(other, () => Dim);
        }

        // ------------------------------------------------------------------ sort and filters

        private static readonly (BinderSort sort, string label)[] SortLabels =
        {
            (BinderSort.SetOrder, "Set order"), (BinderSort.Name, "Name"), (BinderSort.Price, "Price"),
            (BinderSort.TotalValue, "Total value"), (BinderSort.Amount, "Copies owned"), (BinderSort.Rarity, "Rarity"),
            (BinderSort.Element, "Element"), (BinderSort.ManaValue, "Mana value"), (BinderSort.Colour, "Colour"),
        };

        private static readonly (BinderSort sort, string label)[] GradedSortLabels =
        {
            (BinderSort.Grade, "Grade"), (BinderSort.Newest, "Newest"), (BinderSort.Price, "Price"), (BinderSort.Name, "Name"),
            (BinderSort.SetOrder, "Set order"), (BinderSort.Rarity, "Rarity"),
            (BinderSort.Element, "Element"), (BinderSort.ManaValue, "Mana value"), (BinderSort.Colour, "Colour"),
        };

        private static bool AnyMtg => Selection().Any(IsMtg);
        private static bool AnyOther => Selection().Any(e => !IsMtg(e));

        private static void RebuildSorts()
        {
            Clear(_sortList);
            bool mtg = AnyMtg, other = AnyOther;
            if ((_pending.Sort == BinderSort.ManaValue || _pending.Sort == BinderSort.Colour) && !mtg ||
                _pending.Sort == BinderSort.Element && !other) _pending.Sort = _graded ? BinderSort.Grade : BinderSort.SetOrder;
            foreach (var (sort, label) in _graded ? GradedSortLabels : SortLabels)
            {
                if ((sort == BinderSort.ManaValue || sort == BinderSort.Colour) && !mtg) continue;
                if (sort == BinderSort.Element && !other) continue;
                var s = sort;
                Selectable(_sortList, label, _u * 1.7f, () => _pending.Sort == s, () => _pending.Sort = s);
            }
        }

        private static void RebuildFilters()
        {
            Clear(_filterList);
            if (_graded)
            {
                Group("Show", new[] { ("Favourites", (Action)(() => _pending.FavouritesOnly = !_pending.FavouritesOnly), (Func<bool>)(() => _pending.FavouritesOnly)) });
                Group("Grade", Enumerable.Range(1, 10).Reverse().Select(g => Toggle(g.ToString(), _pending.Grades, g)).ToArray());
            }
            else
            Group("Show", new[] { ("All cards", (Action)(() => _pending.Show = BinderShow.All), (Func<bool>)(() => _pending.Show == BinderShow.All)),
                                  ("Duplicates", () => _pending.Show = BinderShow.Duplicates, () => _pending.Show == BinderShow.Duplicates),
                                  ("Favourites", () => _pending.FavouritesOnly = !_pending.FavouritesOnly, () => _pending.FavouritesOnly),
                                  ("Gaps for missing", () => _pending.Gaps = !_pending.Gaps, () => _pending.Gaps),
                                  ("Every version separately", () => _pending.Separate = !_pending.Separate, () => _pending.Separate) });
            string[] borders = { "Base", "1st edition", "Silver", "Gold", "EX", "Full art" };
            Group("Border", borders.Select((b, i) => Toggle(b, _pending.Borders, i)).ToArray());
            Group("Finish", new[] { Toggle("Normal", _pending.Foil, false), Toggle("Foil", _pending.Foil, true) });

            var cards = _graded
                ? BinderIndex.GradedCards().Where(c => InSel(c.exp)).Select(c => c.info).ToList()
                : BinderIndex.List(_pending.Sets, new BinderQuery(), out _).Select(e => e.Info).ToList();
            var rarities = cards.GroupBy(c => c.RarityKey).Select(g => g.First()).OrderBy(c => c.RarityRank).ToList();
            _pending.Rarities.IntersectWith(rarities.Select(r => r.RarityKey)); // drop picks the sets no longer have
            if (rarities.Count > 1)
                Group("Rarity", rarities.Select(c => Toggle(c.RarityName, _pending.Rarities, c.RarityKey)).ToArray());

            if (AnyMtg)
            {
                var colours = new[] { ("White", "W"), ("Blue", "U"), ("Black", "B"), ("Red", "R"), ("Green", "G"), ("Multicolour", "M"), ("Colourless", "C") };
                Group("Colour", colours.Select(c => Toggle(c.Item1, _pending.Colours, c.Item2)).ToArray());
                Group("Type", Runtime.Mtg.MtgDeckRules.CardTypes.Select(t => Toggle(t, _pending.Types, t)).ToArray());
                Group("Mana value", Enumerable.Range(0, 7).Select(n => Toggle(n == 6 ? "6+" : n.ToString(), _pending.ManaValues, n)).ToArray());
            }
            else { _pending.Colours.Clear(); _pending.Types.Clear(); _pending.ManaValues.Clear(); }
            var elements = cards.Where(c => c.Mtg == null).Select(c => c.Element).Distinct().OrderBy(x => (int)x).ToList();
            _pending.Elements.IntersectWith(elements);
            if (elements.Count > 1)
                Group("Element", elements.Select(el => Toggle(el.ToString(), _pending.Elements, el)).ToArray());
            Refresh();
        }

        private static (string, Action, Func<bool>) Toggle<T>(string label, HashSet<T> set, T value) =>
            (label, () => { if (!set.Remove(value)) set.Add(value); }, () => set.Contains(value));

        private static readonly string[] SearchInLabels = { "Everything", "Name", "Artist", "Number", "Type line" };

        private static void CycleSearchIn()
        {
            int n = Enum.GetValues(typeof(SearchIn)).Length;
            _pending.In = (SearchIn)(((int)_pending.In + 1) % n);
            SoundManager.GenericPop(0.6f, 1.3f);
            Refresh();
        }

        /// <summary>What the header calls the picked sets.</summary>
        private static string SelectionName()
        {
            var sets = _pending.Sets;
            if (_graded && sets.Count == 0) return "Graded cards · all sets";
            if (_graded) return "Graded cards · " + (sets.Count == 1 ? SetName(sets[0]) : $"{sets.Count} sets");
            if (sets.Count == 0) return "No set picked — pick at least one";
            if (sets.Count == 1) return SetName(sets[0]);
            if (_names.Keys.All(sets.Contains)) return $"All sets ({sets.Count})";
            foreach (var (col, list) in Grouped())
                if (list.Count == sets.Count && list.All(s => sets.Contains(s.exp))) return $"{col} ({sets.Count} sets)";
            return $"{sets.Count} sets";
        }

        /// <summary>Colours of every button and the live numbers for the pending choices.</summary>
        private static void Refresh()
        {
            foreach (var (b, on) in _selectables)
                if (b != null) b.GetComponent<Image>().color = on() ? Selected : Normal;
            foreach (var (img, color) in _tinted)
                if (img != null) img.color = color();
            _searchIn.text = "in: " + SearchInLabels[(int)_pending.In];
            _pending.Search = _search.text ?? "";
            BinderStats st;
            if (_graded) BinderIndex.GradedList(_pending, out st);
            else BinderIndex.List(_pending.Sets, _pending, out st);
            string value = GameInstance.GetPriceString(st.Value);
            _stats.text = _graded
                ? $"<b>{SelectionName()}</b>   Slabs {st.Cards}   Showing {st.Matching}   Value {value}"
                : _pending.Separate
                ? $"<b>{SelectionName()}</b>   Collected {st.VersionsOwned} / {st.Versions}   Value {value}   " +
                  $"Showing {st.Matching} version{(st.Matching == 1 ? "" : "s")}"
                : $"<b>{SelectionName()}</b>   " +
                  $"Collected {st.Owned} / {st.Cards}   Versions {st.VersionsOwned} / {st.Versions}   Value {value}   " +
                  $"Showing {st.Matching} card{(st.Matching == 1 ? "" : "s")}";
            bool canApply = _graded || _pending.Sets.Count > 0;
            _apply.GetComponent<Image>().color = !canApply ? Dim : Changed() ? Selected : Normal;
        }

        /// <summary>A button whose colour comes from a rule (not on/off), re-applied on every refresh.</summary>
        private static void Tint(Button b, Func<Color> color)
        {
            _tinted.Add((b.GetComponent<Image>(), color));
            b.GetComponent<Image>().color = color();
        }

        private static RectTransform Row(Transform parent, float height, float indent = 0f)
        {
            var go = new GameObject("Row", typeof(RectTransform), typeof(HorizontalLayoutGroup), typeof(LayoutElement));
            go.layer = parent.gameObject.layer;
            go.transform.SetParent(parent, false);
            go.GetComponent<LayoutElement>().preferredHeight = height;
            var h = go.GetComponent<HorizontalLayoutGroup>();
            h.spacing = _u * 0.3f;
            h.padding = new RectOffset((int)indent, 0, 0, 0);
            h.childControlWidth = true; h.childControlHeight = true;
            h.childForceExpandWidth = false; h.childForceExpandHeight = true;
            return (RectTransform)go.transform;
        }

        // ------------------------------------------------------------------ building

        private static void Build(CollectionBinderUI ui)
        {
            if (_root != null) Object.Destroy(_root);
            _ui = ui;
            _hiddenVanilla.Clear();
            _selectables.Clear();
            _tinted.Clear();
            _scrolls.Clear();
            _pageButtons.Clear();
            NavReset();
            ReadTemplate(ui);

            var screen = (RectTransform)ui.m_SortAlbumScreen.transform;
            _root = new GameObject("TCGCC_BinderPanel", typeof(RectTransform));
            _root.layer = screen.gameObject.layer;
            var root = Place(_root, screen, 0, 0, 1, 1);
            FitToCanvas(root, screen); // the sort screen's own rect is wider than the visible screen
            _root.AddComponent<PanelInput>();

            var backdrop = root.gameObject.AddComponent<Image>();
            backdrop.color = new Color(0f, 0f, 0f, 0.5f); // also catches clicks around the panel

            // Left margin: the game's key hints (Close Album / Back) sit at the left edge.
            var panel = Place(new GameObject("Panel", typeof(RectTransform), typeof(Image)), root, 0.12f, 0.05f, 0.97f, 0.95f);
            Round(panel.GetComponent<Image>(), PanelColor);
            _u = Mathf.Max(12f, panel.rect.height / 34f);

            // Header
            var header = Place(new GameObject("Header", typeof(RectTransform)), panel, 0.02f, 0.89f, 0.98f, 0.98f);
            var title = Label(header, "Binder", _u * 1.5f, TextAlignmentOptions.Left);
            Place(title.gameObject, header, 0f, 0f, 0.18f, 1f);
            _stats = Label(header, "", _u * 0.95f, TextAlignmentOptions.Right);
            _stats.enableAutoSizing = true;
            _stats.fontSizeMax = _u * 0.95f;
            _stats.fontSizeMin = _u * 0.55f;
            Place(_stats.gameObject, header, 0.18f, 0f, 1f, 1f);

            // Columns
            Column(panel, "Sets", 0.02f, 0.25f, out _setList);
            Column(panel, "Sort", 0.27f, 0.44f, out _sortList);

            var right = Place(new GameObject("Filters", typeof(RectTransform)), panel, 0.46f, 0.12f, 0.98f, 0.87f);
            var searchRow = Place(new GameObject("Search", typeof(RectTransform)), right, 0f, 0.9f, 1f, 1f);
            _search = MakeInput(searchRow, "Search cards…");
            Place(_search.gameObject, searchRow, 0f, 0f, 0.68f, 1f);
            var inBtn = MakeButton(searchRow, "in: Everything", 0f, CycleSearchIn);
            Place(inBtn.gameObject, searchRow, 0.7f, 0f, 1f, 1f);
            _searchIn = inBtn.GetComponentInChildren<TextMeshProUGUI>();
            _search.onValueChanged.AddListener(_ => Refresh());
            _search.onSubmit.AddListener(_ => Apply());
            _filterList = Scroll(right, 0f, 0f, 1f, 0.87f);

            // Controls hint under the left columns
            var hint = Label(panel, "Mouse wheel over a card: switch version   ·   " + Plugin.BinderFavouriteKey.Value + " over a card: favourite   ·   Home / End: first / last page",
                _u * 0.7f, TextAlignmentOptions.Left);
            hint.enableAutoSizing = true; hint.fontSizeMax = _u * 0.7f; hint.fontSizeMin = _u * 0.45f;
            hint.color = new Color(_textColor.r, _textColor.g, _textColor.b, 0.6f);
            Place(hint.gameObject, panel, 0.02f, 0.02f, 0.44f, 0.07f);
            _hintKeys = hint.gameObject;
            _hintPad = Place(new GameObject("PadHint", typeof(RectTransform)), panel, 0.02f, 0.01f, 0.44f, 0.075f).gameObject;
            _hintPad.SetActive(false);

            // Footer: page jumps (also the controller's way to the first / last page), then the panel's own buttons
            var footer = Place(new GameObject("Footer", typeof(RectTransform)), panel, 0.46f, 0.02f, 0.98f, 0.1f);
            var first = MakeButton(footer, "First page", 0f, () => Apply(EnhancedBinderPatches.JumpToFirst));
            Place(first.gameObject, footer, 0f, 0f, 0.18f, 1f);
            var last = MakeButton(footer, "Last page", 0f, () => Apply(EnhancedBinderPatches.JumpToLast));
            Place(last.gameObject, footer, 0.2f, 0f, 0.38f, 1f);
            Tint(first, () => Dim);
            Tint(last, () => Dim);
            _pageButtons.Add(first.gameObject);
            _pageButtons.Add(last.gameObject);
            var reset = MakeButton(footer, "Reset", 0f, Reset);
            Place(reset.gameObject, footer, 0.42f, 0f, 0.6f, 1f);
            var close = MakeButton(footer, "Close", 0f, Close);
            Place(close.gameObject, footer, 0.62f, 0f, 0.8f, 1f);
            _apply = MakeButton(footer, "Apply", 0f, Apply);
            Place(_apply.gameObject, footer, 0.82f, 0f, 1f, 1f);

            _root.SetActive(false);
            Plugin.Log.LogInfo($"Binder panel built (unit {_u:0.#}, panel {panel.rect.width:0}x{panel.rect.height:0}, font {(_font != null ? _font.name : "default")})");
        }

        private static void ReadTemplate(CollectionBinderUI ui)
        {
            var btn = ui.m_SortAlbumBtnList.FirstOrDefault(b => b != null);
            var text = btn != null ? btn.GetComponentInChildren<TextMeshProUGUI>(true) : null;
            _font = text != null ? text.font : ui.m_PageText != null ? ui.m_PageText.font : null;
            if (text != null) _textColor = text.color;
        }

        /// <summary>Makes the root cover exactly the visible canvas (in the sort screen's local space).</summary>
        private static void FitToCanvas(RectTransform root, RectTransform screen)
        {
            var canvas = screen.GetComponentInParent<Canvas>();
            if (canvas == null) return;
            var c = new Vector3[4];
            ((RectTransform)canvas.rootCanvas.transform).GetWorldCorners(c);
            Vector3 a = screen.InverseTransformPoint(c[0]), b = screen.InverseTransformPoint(c[2]);
            root.anchorMin = root.anchorMax = root.pivot = new Vector2(0.5f, 0.5f);
            root.sizeDelta = new Vector2(Mathf.Abs(b.x - a.x), Mathf.Abs(b.y - a.y));
            root.localPosition = (a + b) / 2f;
        }

        /// <summary>Rounded-rectangle look for panels, buttons and fields (a small 9-sliced sprite made once).</summary>
        private static void Round(Image img, Color color)
        {
            if (_rounded == null)
            {
                const int size = 32, r = 10;
                var tex = new Texture2D(size, size, TextureFormat.RGBA32, false) { wrapMode = TextureWrapMode.Clamp, filterMode = FilterMode.Bilinear };
                for (int y = 0; y < size; y++)
                for (int x = 0; x < size; x++)
                {
                    float dx = Mathf.Max(0f, Mathf.Max(r - x - 0.5f, x + 0.5f - (size - r))),
                          dy = Mathf.Max(0f, Mathf.Max(r - y - 0.5f, y + 0.5f - (size - r)));
                    float alpha = Mathf.Clamp01(r - Mathf.Sqrt(dx * dx + dy * dy) + 0.5f);
                    tex.SetPixel(x, y, new Color(1f, 1f, 1f, alpha));
                }
                tex.Apply();
                _rounded = Sprite.Create(tex, new UnityEngine.Rect(0, 0, size, size), new Vector2(0.5f, 0.5f), 100f, 0,
                    SpriteMeshType.FullRect, new Vector4(r + 1, r + 1, r + 1, r + 1));
                _rounded.name = "TCGCC_Rounded";
            }
            img.sprite = _rounded;
            img.type = Image.Type.Sliced;
            img.pixelsPerUnitMultiplier = 1.2f;
            img.color = color;
        }

        private static void Column(RectTransform panel, string title, float x0, float x1, out RectTransform list)
        {
            var col = Place(new GameObject(title, typeof(RectTransform)), panel, x0, 0.08f, x1, 0.87f);
            var label = Label(col, title, _u * 1.1f, TextAlignmentOptions.Left);
            Place(label.gameObject, col, 0f, 0.93f, 1f, 1f);
            list = Scroll(col, 0f, 0f, 1f, 0.92f);
        }

        private static RectTransform Place(GameObject go, Transform parent, float x0, float y0, float x1, float y1)
        {
            var rt = (RectTransform)go.transform;
            rt.SetParent(parent, false);
            go.layer = parent.gameObject.layer;
            rt.anchorMin = new Vector2(x0, y0);
            rt.anchorMax = new Vector2(x1, y1);
            rt.offsetMin = rt.offsetMax = Vector2.zero;
            return rt;
        }

        private static TextMeshProUGUI Label(Transform parent, string text, float size, TextAlignmentOptions align)
        {
            var go = new GameObject("Text", typeof(RectTransform));
            go.layer = parent.gameObject.layer;
            go.transform.SetParent(parent, false);
            var t = go.AddComponent<TextMeshProUGUI>();
            if (_font != null) t.font = _font;
            t.text = text;
            t.fontSize = size;
            t.color = _textColor;
            t.alignment = align;
            t.enableWordWrapping = false;
            t.overflowMode = TextOverflowModes.Ellipsis;
            t.raycastTarget = false;
            return t;
        }

        /// <summary>A button; height 0 = sized by its parent (anchors), else a layout row of that height.</summary>
        private static Button MakeButton(Transform parent, string text, float height, Action onClick)
        {
            var go = new GameObject("Button", typeof(RectTransform), typeof(Image), typeof(Button));
            go.layer = parent.gameObject.layer;
            go.transform.SetParent(parent, false);
            var img = go.GetComponent<Image>();
            Round(img, Normal);
            if (height > 0f) go.AddComponent<LayoutElement>().preferredHeight = height;
            var t = Label(go.transform, text, _u * 0.9f, TextAlignmentOptions.Center);
            t.enableAutoSizing = true; // long set names / chips shrink instead of being cut
            t.fontSizeMax = _u * 0.9f;
            t.fontSizeMin = _u * 0.5f;
            var trt = t.rectTransform;
            trt.anchorMin = Vector2.zero; trt.anchorMax = Vector2.one;
            trt.offsetMin = new Vector2(_u * 0.4f, 0f); trt.offsetMax = new Vector2(-_u * 0.4f, 0f);
            var b = go.GetComponent<Button>();
            b.targetGraphic = img;
            b.onClick.AddListener(() =>
            {
                _clickFrame = Time.frameCount;
                try { onClick(); }
                catch (Exception e) { Plugin.Log.LogError($"Binder panel: {e}"); }
            });
            return b;
        }

        /// <summary>A button that shows on/off; any click refreshes every button and the numbers.</summary>
        private static Button Selectable(Transform parent, string text, float height, Func<bool> on, Action toggle)
        {
            var b = MakeButton(parent, text, height, () =>
            {
                toggle();
                SoundManager.GenericPop(0.6f, 1.3f);
                Refresh();
            });
            _selectables.Add((b, on));
            return b;
        }

        private static void Group(string title, (string label, Action toggle, Func<bool> on)[] chips)
        {
            var label = Label(_filterList, title, _u * 0.95f, TextAlignmentOptions.BottomLeft);
            label.gameObject.AddComponent<LayoutElement>().preferredHeight = _u * 1.5f;

            var grid = new GameObject(title + " chips", typeof(RectTransform), typeof(GridLayoutGroup)); // height from the list's layout
            grid.layer = _filterList.gameObject.layer;
            grid.transform.SetParent(_filterList, false);
            var g = grid.GetComponent<GridLayoutGroup>();
            g.cellSize = new Vector2(_u * 6.2f, _u * 1.6f);
            g.spacing = new Vector2(_u * 0.35f, _u * 0.35f);
            g.constraint = GridLayoutGroup.Constraint.Flexible;
            foreach (var (text, toggle, on) in chips) Selectable(grid.transform, text, 0f, on, toggle);
        }

        private static RectTransform Scroll(Transform parent, float x0, float y0, float x1, float y1)
        {
            var view = Place(new GameObject("Scroll", typeof(RectTransform), typeof(RectMask2D), typeof(ScrollRect)), parent, x0, y0, x1, y1);
            var content = new GameObject("Content", typeof(RectTransform), typeof(VerticalLayoutGroup), typeof(ContentSizeFitter));
            content.layer = view.gameObject.layer;
            var crt = (RectTransform)content.transform;
            crt.SetParent(view, false);
            crt.anchorMin = new Vector2(0f, 1f); crt.anchorMax = new Vector2(1f, 1f);
            crt.pivot = new Vector2(0.5f, 1f);
            crt.offsetMin = crt.offsetMax = Vector2.zero;
            var v = content.GetComponent<VerticalLayoutGroup>();
            v.spacing = _u * 0.35f;
            v.childControlWidth = true; v.childControlHeight = true;
            v.childForceExpandWidth = true; v.childForceExpandHeight = false;
            v.padding = new RectOffset(0, (int)(_u * 1.1f), 0, 0); // room for the scrollbar
            content.GetComponent<ContentSizeFitter>().verticalFit = ContentSizeFitter.FitMode.PreferredSize;
            var s = view.GetComponent<ScrollRect>();
            s.content = crt; s.viewport = view;
            s.horizontal = false; s.vertical = true;
            s.movementType = ScrollRect.MovementType.Clamped;
            s.scrollSensitivity = 0f; // the wheel is read by PanelInput (only the list under the mouse scrolls)
            s.verticalScrollbar = Scrollbar(view);
            s.verticalScrollbarVisibility = ScrollRect.ScrollbarVisibility.AutoHide; // only when the list doesn't fit
            _scrolls.Add(s);
            return crt;
        }

        /// <summary>Slim vertical scrollbar along the right edge of a list (drag the handle; the wheel works too).</summary>
        private static Scrollbar Scrollbar(RectTransform view)
        {
            var bar = Place(new GameObject("Scrollbar", typeof(RectTransform), typeof(Image), typeof(Scrollbar)), view, 1f, 0f, 1f, 1f);
            bar.pivot = new Vector2(1f, 0.5f);
            bar.sizeDelta = new Vector2(_u * 0.55f, 0f);
            Round(bar.GetComponent<Image>(), new Color(1f, 1f, 1f, 0.08f));
            var area = Place(new GameObject("Sliding Area", typeof(RectTransform)), bar, 0f, 0f, 1f, 1f);
            var handle = Place(new GameObject("Handle", typeof(RectTransform), typeof(Image)), area, 0f, 0f, 1f, 1f);
            var img = handle.GetComponent<Image>();
            Round(img, new Color(1f, 1f, 1f, 0.45f));
            var sb = bar.GetComponent<Scrollbar>();
            sb.handleRect = handle;
            sb.targetGraphic = img;
            sb.direction = UnityEngine.UI.Scrollbar.Direction.BottomToTop;
            return sb;
        }

        private static TMP_InputField MakeInput(Transform parent, string placeholder)
        {
            var go = new GameObject("Input", typeof(RectTransform), typeof(Image), typeof(TMP_InputField));
            go.layer = parent.gameObject.layer;
            go.transform.SetParent(parent, false);
            var img = go.GetComponent<Image>();
            Round(img, FieldColor);
            var area = Place(new GameObject("Text Area", typeof(RectTransform), typeof(RectMask2D)), go.transform, 0f, 0f, 1f, 1f);
            area.offsetMin = new Vector2(_u * 0.6f, _u * 0.2f); area.offsetMax = new Vector2(-_u * 0.6f, -_u * 0.2f);
            var ph = Label(area, placeholder, _u * 0.95f, TextAlignmentOptions.Left);
            ph.fontStyle = FontStyles.Italic;
            ph.color = new Color(_textColor.r, _textColor.g, _textColor.b, 0.5f);
            Stretch(ph.rectTransform);
            var txt = Label(area, "", _u * 0.95f, TextAlignmentOptions.Left);
            Stretch(txt.rectTransform);
            var f = go.GetComponent<TMP_InputField>();
            f.textViewport = area;
            f.textComponent = txt;
            f.placeholder = ph;
            if (_font != null) f.fontAsset = _font;
            f.pointSize = _u * 0.95f;
            f.lineType = TMP_InputField.LineType.SingleLine;
            f.targetGraphic = img;
            return f;
        }

        private static void Stretch(RectTransform rt)
        {
            rt.anchorMin = Vector2.zero; rt.anchorMax = Vector2.one;
            rt.offsetMin = rt.offsetMax = Vector2.zero;
        }

        private static void Clear(RectTransform list)
        {
            _selectables.RemoveAll(s => s.b == null || s.b.transform.IsChildOf(list));
            _tinted.RemoveAll(t => t.img == null || t.img.transform.IsChildOf(list));
            for (int i = list.childCount - 1; i >= 0; i--)
            {
                var c = list.GetChild(i).gameObject;
                c.SetActive(false);
                Object.Destroy(c);
            }
        }

        /// <summary>Mouse wheel scrolls the list under the cursor (Esc: <see cref="HandleEscape"/>); controller: <see cref="NavUpdate"/>.</summary>
        private sealed class PanelInput : MonoBehaviour
        {
            private void Update()
            {
                try { NavUpdate(); }
                catch (Exception e) { Plugin.Log.LogError($"Binder panel controller: {e}"); }
                float wheel = UnityEngine.Input.mouseScrollDelta.y;
                if (Mathf.Abs(wheel) < 0.01f) return;
                var canvas = GetComponentInParent<Canvas>();
                Camera cam = canvas != null && canvas.renderMode != RenderMode.ScreenSpaceOverlay ? canvas.worldCamera : null;
                foreach (var s in _scrolls)
                {
                    if (s == null || !s.gameObject.activeInHierarchy) continue;
                    if (!RectTransformUtility.RectangleContainsScreenPoint(s.viewport, UnityEngine.Input.mousePosition, cam)) continue;
                    var content = s.content;
                    float max = Mathf.Max(0f, content.rect.height - s.viewport.rect.height);
                    float y = Mathf.Clamp(content.anchoredPosition.y - wheel * _u * 3f, 0f, max);
                    content.anchoredPosition = new Vector2(content.anchoredPosition.x, y);
                    break;
                }
            }
        }
    }
}

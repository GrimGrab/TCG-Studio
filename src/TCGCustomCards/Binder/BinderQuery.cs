using System;
using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Core;
using TCGCustomCards.Runtime;
using TCGCustomCards.Runtime.Mtg;

namespace TCGCustomCards.Binder
{
    internal enum BinderSort { SetOrder, Name, Price, TotalValue, Amount, Rarity, Element, ManaValue, Colour, Grade, Newest }

    internal enum SearchIn { Any, Name, Artist, Number, Type }

    internal enum BinderShow { All, Duplicates }

    /// <summary>
    /// What the enhanced binder lists: sort, search and filters. The panel edits a copy and Apply makes it the binder's.
    /// Empty filter sets mean "any". Rarity and MTG filters are per set: cleared when the set changes.
    /// </summary>
    internal sealed class BinderQuery
    {
        /// <summary>Sets shown together, in set order. Empty = only the binder's current set.</summary>
        public readonly List<ECardExpansionType> Sets = new List<ECardExpansionType>();
        public BinderSort Sort = BinderSort.SetOrder;
        public string Search = "";
        public SearchIn In = SearchIn.Any;
        public BinderShow Show = BinderShow.All;
        /// <summary>Set order without filters: an empty spot for every card not owned (full-set binder). Off = owned cards packed together.</summary>
        public bool Gaps;
        public bool FavouritesOnly;
        /// <summary>Every version its own slot instead of one slot per card.</summary>
        public bool Separate;
        /// <summary>Borders 0..5 (Base … Full art); any owned version with one of these.</summary>
        public readonly HashSet<int> Borders = new HashSet<int>();
        /// <summary>false = normal, true = foil.</summary>
        public readonly HashSet<bool> Foil = new HashSet<bool>();
        /// <summary>Rarity keys (<see cref="CardInfo.RarityKey"/>).</summary>
        public readonly HashSet<string> Rarities = new HashSet<string>();
        public readonly HashSet<EElementIndex> Elements = new HashSet<EElementIndex>();
        /// <summary>W U B R G, C = colourless, M = multicolour.</summary>
        public readonly HashSet<string> Colours = new HashSet<string>();
        public readonly HashSet<string> Types = new HashSet<string>();
        /// <summary>0..6 (6 = 6 or more).</summary>
        public readonly HashSet<int> ManaValues = new HashSet<int>();
        /// <summary>Graded album: grades 1..10.</summary>
        public readonly HashSet<int> Grades = new HashSet<int>();

        public BinderQuery Clone()
        {
            var q = new BinderQuery { Sort = Sort, Search = Search, In = In, Show = Show, Gaps = Gaps, FavouritesOnly = FavouritesOnly, Separate = Separate };
            q.Sets.AddRange(Sets);
            q.Borders.UnionWith(Borders); q.Foil.UnionWith(Foil); q.Rarities.UnionWith(Rarities);
            q.Elements.UnionWith(Elements); q.Colours.UnionWith(Colours); q.Types.UnionWith(Types);
            q.ManaValues.UnionWith(ManaValues);
            q.Grades.UnionWith(Grades);
            return q;
        }

        public bool SameAs(BinderQuery o) =>
            Sets.SequenceEqual(o.Sets) && Sort == o.Sort && Search == o.Search && In == o.In && Show == o.Show && Gaps == o.Gaps && FavouritesOnly == o.FavouritesOnly && Separate == o.Separate && Borders.SetEquals(o.Borders) &&
            Foil.SetEquals(o.Foil) && Rarities.SetEquals(o.Rarities) && Elements.SetEquals(o.Elements) &&
            Colours.SetEquals(o.Colours) && Types.SetEquals(o.Types) && ManaValues.SetEquals(o.ManaValues) && Grades.SetEquals(o.Grades);

        /// <summary>Filters that only make sense for one set.</summary>
        public void ClearSetFilters()
        {
            Rarities.Clear(); Elements.Clear(); Colours.Clear(); Types.Clear(); ManaValues.Clear();
            if (Sort == BinderSort.ManaValue || Sort == BinderSort.Colour || Sort == BinderSort.Element) Sort = BinderSort.SetOrder;
        }

        /// <summary>No search and no filter: with <see cref="Gaps"/>, Set order then keeps every card in its place.</summary>
        public bool Unfiltered =>
            string.IsNullOrWhiteSpace(Search) && Show == BinderShow.All && !FavouritesOnly && Borders.Count == 0 && Foil.Count == 0 &&
            Rarities.Count == 0 && Elements.Count == 0 && Colours.Count == 0 && Types.Count == 0 && ManaValues.Count == 0;

        /// <summary>A version passes the border/foil filter (and the duplicates filter by its count).</summary>
        public bool VersionOk(BinderEntry e, int v)
        {
            int plain = e.Per / 2;
            if (Borders.Count > 0 && !Borders.Contains(v % plain)) return false;
            if (Foil.Count > 0 && !Foil.Contains(v >= plain)) return false;
            return true;
        }

        public int MinCount => Show == BinderShow.Duplicates ? 2 : 1;

        /// <summary>Card-level filters (search, rarity, element, MTG).</summary>
        public bool CardOk(CardInfo c)
        {
            if (Rarities.Count > 0 && !Rarities.Contains(c.RarityKey)) return false;
            if (Elements.Count > 0 && !Elements.Contains(c.Element)) return false;
            if (Colours.Count > 0 || Types.Count > 0 || ManaValues.Count > 0)
            {
                if (c.Mtg == null) return false;
                if (Colours.Count > 0 && !c.MtgColours.Any(Colours.Contains)) return false;
                if (Types.Count > 0 && !Types.Any(t => c.Mtg.TypeLine.IndexOf(t, StringComparison.OrdinalIgnoreCase) >= 0)) return false;
                if (ManaValues.Count > 0 && !ManaValues.Contains(Math.Min(6, (int)c.ManaValue))) return false;
            }
            if (!string.IsNullOrWhiteSpace(Search))
            {
                string s = Search.Trim();
                bool Has(string f) => !string.IsNullOrEmpty(f) && f.IndexOf(s, StringComparison.OrdinalIgnoreCase) >= 0;
                switch (In)
                {
                    case SearchIn.Name: return Has(c.Name);
                    case SearchIn.Artist: return Has(c.Artist);
                    case SearchIn.Number: return Has(c.Number);
                    case SearchIn.Type: return Has(c.Mtg?.TypeLine);
                    default: return Has(c.Name) || Has(c.Artist) || Has(c.Number) || Has(c.Mtg?.TypeLine) || Has(c.Text);
                }
            }
            return true;
        }
    }

    /// <summary>What a card is, for search/filters/sorts: custom cards from set.json, vanilla from MonsterData.</summary>
    internal sealed class CardInfo
    {
        public string Name = "", Artist = "", Number = "", Text = "";
        public string RarityKey = "", RarityName = "";
        public int RarityRank;
        public EElementIndex Element;
        public CardMtgDef Mtg;
        public string[] MtgColours = new string[0];
        public float ManaValue;

        private static readonly Dictionary<EMonsterType, CardInfo> Cache = new Dictionary<EMonsterType, CardInfo>();

        public static void ClearCache() => Cache.Clear();

        public static CardInfo Of(EMonsterType m)
        {
            if (Cache.TryGetValue(m, out var c)) return c;
            c = new CardInfo();
            var data = InventoryBase.GetMonsterData(m);
            if (data != null)
            {
                c.Name = data.GetName() ?? data.Name ?? "";
                c.Element = data.ElementIndex;
                c.RarityKey = data.Rarity.ToString();
                c.RarityName = c.RarityKey;
                c.RarityRank = (int)data.Rarity;
            }
            if (Registry.TryGetCard(m, out var set, out int pos))
            {
                var card = set.Card(pos);
                c.Name = card.Name ?? c.Name;
                c.Artist = card.Artist ?? "";
                c.Number = card.Number ?? "";
                c.Text = card.Description ?? "";
                var r = set.Rarity(pos);
                if (r != null) { c.RarityKey = r.Id; c.RarityName = r.Name ?? r.Id; c.RarityRank = r.Rank; }
                c.Mtg = card.Mtg;
                if (card.Mtg != null)
                {
                    var cols = (card.Mtg.Colors ?? new List<string>()).Where(x => Array.IndexOf(MtgDeckRules.Wubrg, x) >= 0).ToList();
                    if (cols.Count == 0)
                        cols = MtgDeckRules.Wubrg.Where(w => (card.Mtg.ManaCost ?? "").Contains("{" + w)).ToList();
                    if (cols.Count == 0) cols.Add("C");
                    else if (cols.Count > 1) cols.Add("M");
                    c.MtgColours = cols.ToArray();
                    c.ManaValue = card.Mtg.Cmc > 0 ? card.Mtg.Cmc : MtgDeckRules.CmcFromCost(card.Mtg.ManaCost);
                }
            }
            Cache[m] = c;
            return c;
        }

        /// <summary>Colour sort key: W U B R G one colour, then multicolour, then colourless.</summary>
        public int ColourKey
        {
            get
            {
                if (MtgColours.Contains("M")) return 5;
                if (MtgColours.Contains("C")) return 6;
                int i = MtgColours.Length > 0 ? Array.IndexOf(MtgDeckRules.Wubrg, MtgColours[0]) : -1;
                return i < 0 ? 7 : i;
            }
        }
    }
}

using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Patches;

namespace TCGCustomCards.Binder
{
    /// <summary>
    /// One card of the binder in the one-slot-per-card layout: all its border × foil versions (save slots) together.
    /// Counts and prices are read live, so cards taken out or pulled while the binder is built stay right.
    /// </summary>
    internal sealed class BinderEntry
    {
        public ECardExpansionType Expansion;
        /// <summary>Ghost's dimension half (CPlayerData "isDestiny" lists).</summary>
        public bool Dimension;
        /// <summary>Position in the expansion's shown monster list.</summary>
        public int Pos;
        public EMonsterType Monster;
        /// <summary>Save slots per card (12; Ghost 2).</summary>
        public int Per;
        /// <summary>Versions shown as squares: the set has it, or the player owns one.</summary>
        public bool[] Listed;
        public int Order;
        /// <summary>Every-version-separately layout: the one version this slot shows (-1 = the card with all its versions).</summary>
        public int Fixed = -1;

        public CardInfo Info => CardInfo.Of(Monster);
        public int SaveIndex(int v) => Pos * Per + v;
        public int Count(int v) => CPlayerData.GetCardAmountByIndex(SaveIndex(v), Expansion, Dimension);

        public int Total()
        {
            if (Fixed >= 0) return Count(Fixed);
            int n = 0;
            for (int v = 0; v < Per; v++) n += Count(v);
            return n;
        }

        public float TotalValue()
        {
            if (Fixed >= 0) return Count(Fixed) * Price(Fixed);
            float sum = 0f;
            for (int v = 0; v < Per; v++)
            {
                int n = Count(v);
                if (n > 0) sum += n * Price(v);
            }
            return sum;
        }

        public CardData Card(int v)
        {
            int plain = Per / 2;
            return new CardData
            {
                monsterType = Monster,
                expansionType = Expansion,
                isDestiny = Dimension,
                borderType = (ECardBorderType)(v % plain),
                isFoil = v >= plain,
            };
        }

        public float Price(int v) => CPlayerData.GetCardMarketPrice(Card(v));

        /// <summary>Same card, all fields copied (the key is shared).</summary>
        public BinderEntry MemberwiseCopy() => (BinderEntry)MemberwiseClone();

        private string _key;
        /// <summary>Stable card key (<see cref="BinderKeys.Card(BinderEntry)"/>): favourites and picked versions.</summary>
        public string Key => _key ?? (_key = BinderKeys.Card(this));
    }

    /// <summary>Numbers for the binder's header and the panel's preview.</summary>
    internal struct BinderStats
    {
        public int Cards, Owned, Matching;
        /// <summary>Card versions (border × foil) the shown sets have, and how many of them are owned.</summary>
        public int Versions, VersionsOwned;
        public float Value;
    }

    /// <summary>
    /// Builds the binder's card list for the one-slot-per-card layout and picks the version each slot shows.
    /// Sorting is one stable sort (the game's own sorts are O(n²) over 12 slots per card).
    /// </summary>
    internal static class BinderIndex
    {
        public static readonly List<BinderEntry> Entries = new List<BinderEntry>();
        public static BinderStats Stats;
        /// <summary>The query the binder shows (the panel's Apply sets it).</summary>
        public static BinderQuery Query = new BinderQuery();

        /// <summary>Version the player picked with the wheel, per card (until another save loads).</summary>
        private static readonly Dictionary<string, int> Picked = new Dictionary<string, int>();

        internal static IReadOnlyDictionary<string, int> PickedByKey() => Picked;
        internal static void LoadPick(string key, int v) => Picked[key] = v;

        /// <summary>Sets the binder shows: the query's selection when it includes the binder's set, else just that set.</summary>
        public static List<ECardExpansionType> SetsFor(ECardExpansionType exp) =>
            Query.Sets.Count > 0 && Query.Sets.Contains(exp) ? Query.Sets.ToList() : new List<ECardExpansionType> { exp };

        /// <summary>Every version (border × foil) gets its own slot, like the game's binder (per save, binder panel).</summary>
        public static bool Separate => Query.Separate;

        /// <summary>Fills <see cref="Entries"/> for the binder.</summary>
        public static void Build(ECardExpansionType exp)
        {
            Entries.Clear();
            Entries.AddRange(List(SetsFor(exp), Query, out Stats));
        }

        /// <summary>The cards some sets show under a query (also the panel's live preview).</summary>
        public static List<BinderEntry> List(IList<ECardExpansionType> sets, BinderQuery q, out BinderStats stats)
        {
            var all = new List<BinderEntry>();
            foreach (var exp in sets)
            {
                AddHalf(all, exp, dimension: false);
                if (exp == ECardExpansionType.Ghost) AddHalf(all, exp, dimension: true);
            }
            stats = new BinderStats { Cards = all.Count };
            foreach (var e in all)
            {
                int owned = 0;
                for (int v = 0; v < e.Per; v++)
                {
                    if (e.Listed[v]) stats.Versions++; // listed = the set has it, or it's owned anyway
                    int n = e.Count(v);
                    if (n <= 0) continue;
                    owned++;
                    stats.Value += n * e.Price(v);
                }
                if (owned == 0) continue;
                stats.Owned++;
                stats.VersionsOwned += owned;
            }

            if (q.Gaps && q.Sort == BinderSort.SetOrder && q.Unfiltered)
            {
                // every card keeps its place: missing cards are gaps, like a real binder
                if (!q.Separate)
                {
                    stats.Matching = stats.Owned;
                    return all;
                }
                stats.Matching = stats.VersionsOwned;
                return Versions(all, (e, v) => e.Listed[v]);
            }
            if (q.Separate) all = Versions(all, (e, v) => Usable(e, q, v));
            var shown = all.Where(e => Shown(e, q) >= 0 && q.CardOk(e.Info) && (!q.FavouritesOnly || Favourites.Has(e))).ToList();
            stats.Matching = shown.Count;
            IEnumerable<BinderEntry> sorted;
            switch (q.Sort)
            {
                case BinderSort.Name: sorted = shown.OrderBy(e => e.Info.Name, System.StringComparer.OrdinalIgnoreCase); break;
                case BinderSort.Price: sorted = shown.OrderByDescending(e => e.Price(Shown(e, q))); break;
                case BinderSort.TotalValue: sorted = shown.OrderByDescending(e => e.TotalValue()); break;
                case BinderSort.Amount: sorted = shown.OrderByDescending(e => e.Total()); break;
                case BinderSort.Rarity: sorted = shown.OrderByDescending(e => e.Info.RarityRank); break;
                case BinderSort.Element: sorted = shown.OrderBy(e => (int)e.Info.Element); break;
                case BinderSort.ManaValue: sorted = shown.OrderBy(e => e.Info.Mtg == null ? 99f : e.Info.ManaValue); break;
                case BinderSort.Colour: sorted = shown.OrderBy(e => e.Info.ColourKey).ThenBy(e => e.Info.ManaValue); break;
                default: sorted = shown; break;
            }
            return sorted is IOrderedEnumerable<BinderEntry> o ? o.ThenBy(e => e.Order).ToList() : sorted.ToList();
        }

        /// <summary>One entry per version of every card that passes <paramref name="keep"/>, in the game's order (card, then version).</summary>
        private static List<BinderEntry> Versions(List<BinderEntry> cards, System.Func<BinderEntry, int, bool> keep)
        {
            var list = new List<BinderEntry>();
            foreach (var e in cards)
                for (int v = 0; v < e.Per; v++)
                {
                    if (!keep(e, v)) continue;
                    var copy = e.MemberwiseCopy();
                    copy.Fixed = v;
                    copy.Order = list.Count;
                    list.Add(copy);
                }
            return list;
        }

        /// <summary>The graded album's query (sets empty = every set).</summary>
        public static BinderQuery GradedQuery = new BinderQuery { Sort = BinderSort.Grade };

        /// <summary>
        /// Graded album: indices into CPlayerData.m_GradedCardInventoryList that pass the query, sorted. The game's own slab
        /// display fills the pages from these (m_SortedIndexList). Stats: Cards = all slabs, Value/Matching = the listed ones.
        /// </summary>
        public static List<int> GradedList(BinderQuery q, out BinderStats stats)
        {
            var inv = CPlayerData.m_GradedCardInventoryList;
            var rows = new List<(int i, CardData d, CardInfo info, float price)>();
            stats = new BinderStats { Cards = inv.Count, Owned = inv.Count };
            for (int i = 0; i < inv.Count; i++)
            {
                var c = inv[i];
                if (c == null || q.Sets.Count > 0 && !q.Sets.Contains(c.expansionType)) continue;
                var d = CPlayerData.GetGradedCardData(c);
                if (q.Grades.Count > 0 && !q.Grades.Contains(d.cardGrade)) continue;
                if (q.Borders.Count > 0 && !q.Borders.Contains((int)d.borderType)) continue;
                if (q.Foil.Count > 0 && !q.Foil.Contains(d.isFoil)) continue;
                if (q.FavouritesOnly && !Favourites.Has(d)) continue;
                var info = CardInfo.Of(d.monsterType);
                if (!q.CardOk(info)) continue;
                float price = CPlayerData.GetCardMarketPrice(d);
                rows.Add((i, d, info, price));
                stats.Value += price;
            }
            stats.Matching = rows.Count;
            IEnumerable<(int i, CardData d, CardInfo info, float price)> sorted;
            switch (q.Sort)
            {
                case BinderSort.Grade: sorted = rows.OrderByDescending(r => r.d.cardGrade).ThenByDescending(r => r.price); break;
                case BinderSort.Newest: sorted = rows.OrderByDescending(r => r.i); break;
                case BinderSort.Price: sorted = rows.OrderByDescending(r => r.price); break;
                case BinderSort.Name: sorted = rows.OrderBy(r => r.info.Name, System.StringComparer.OrdinalIgnoreCase); break;
                case BinderSort.Rarity: sorted = rows.OrderByDescending(r => r.info.RarityRank); break;
                case BinderSort.Element: sorted = rows.OrderBy(r => (int)r.info.Element); break;
                case BinderSort.ManaValue: sorted = rows.OrderBy(r => r.info.Mtg == null ? 99f : r.info.ManaValue); break;
                case BinderSort.Colour: sorted = rows.OrderBy(r => r.info.ColourKey).ThenBy(r => r.info.ManaValue); break;
                default: // Set order: collection, release, set, card
                    sorted = rows.OrderBy(r => Collections.Of(r.d.expansionType), System.StringComparer.OrdinalIgnoreCase)
                        .ThenBy(r => Collections.Released(r.d.expansionType)).ThenBy(r => (int)r.d.expansionType)
                        .ThenBy(r => inv[r.i].cardSaveIndex);
                    break;
            }
            return sorted.Select(r => r.i).ToList();
        }

        /// <summary>Card info of every graded slab (the panel's filter choices).</summary>
        public static IEnumerable<(ECardExpansionType exp, CardInfo info)> GradedCards() =>
            CPlayerData.m_GradedCardInventoryList.Where(c => c != null)
                .Select(c => (c.expansionType, CardInfo.Of(CPlayerData.GetGradedCardData(c).monsterType)));

        private static void AddHalf(List<BinderEntry> into, ECardExpansionType exp, bool dimension)
        {
            var shown = InventoryBase.GetShownMonsterList(exp);
            int per = CPlayerData.GetCardAmountPerMonsterType(exp);
            var limited = VariantPatches.Limited(exp);
            for (int pos = 0; pos < shown.Count; pos++)
            {
                var e = new BinderEntry
                {
                    Expansion = exp, Dimension = dimension, Pos = pos, Monster = shown[pos], Per = per,
                    Listed = new bool[per], Order = into.Count,
                };
                for (int v = 0; v < per; v++)
                    e.Listed[v] = limited == null || VariantPatches.Listed(limited, exp, e.SaveIndex(v));
                into.Add(e);
            }
        }

        private static bool Usable(BinderEntry e, BinderQuery q, int v) => e.Count(v) >= q.MinCount && q.VersionOk(e, v);

        /// <summary>
        /// The version a slot shows: the one the player picked while it still fits, else the most valuable owned one that
        /// passes the version/duplicates filters. -1 = none (empty slot).
        /// </summary>
        public static int Shown(BinderEntry e, BinderQuery q)
        {
            if (e.Fixed >= 0) return Usable(e, q, e.Fixed) ? e.Fixed : -1;
            if (Picked.TryGetValue(e.Key, out int p) && p < e.Per && Usable(e, q, p)) return p;
            int best = -1;
            float bestPrice = -1f;
            for (int v = 0; v < e.Per; v++)
            {
                if (!Usable(e, q, v)) continue;
                float price = e.Price(v);
                if (price > bestPrice) { best = v; bestPrice = price; }
            }
            return best;
        }

        public static int Shown(BinderEntry e) => Shown(e, Query);

        /// <summary>Next (step +1) or previous (-1) usable version after the shown one; remembers the pick. False if there is no other.</summary>
        public static bool Switch(BinderEntry e, int step)
        {
            if (e.Fixed >= 0) return false;
            int cur = Shown(e);
            if (cur < 0) return false;
            for (int i = 1; i < e.Per; i++)
            {
                int v = ((cur + step * i) % e.Per + e.Per) % e.Per;
                if (!Usable(e, Query, v)) continue;
                Picked[e.Key] = v;
                return true;
            }
            return false;
        }

        /// <summary>New game or another save: forget picks and cached card info.</summary>
        public static void Reset()
        {
            Picked.Clear();
            Entries.Clear();
            CardInfo.ClearCache();
        }
    }
}

using System;
using System.Collections.Generic;
using System.Linq;

namespace TCGCustomCards.Runtime.Mtg
{
    /// <summary>How deck lists (deck builder, sit-down picker) are ordered.</summary>
    internal enum MtgDeckSort { Recent, Name, Colours, Kind }

    /// <summary>
    /// What a deck is, for showing, searching and sorting deck lists: colours (WUBRG order), kind (Constructed / Commander),
    /// counts, sets, card names, last used. Built from an MTG deck-builder deck or from the Forge deck of any deck choice.
    /// </summary>
    internal sealed class MtgDeckSummary
    {
        public const string Wubrg = "WUBRG";

        public string Name = "";
        /// <summary>Colour letters in WUBRG order ("" = colourless).</summary>
        public string Colors = "";
        public bool IsCommander;
        public string CommanderName;
        public int Total, Target;
        public List<string> SetCodes = new List<string>();
        public HashSet<string> CardNames = new HashSet<string>(StringComparer.OrdinalIgnoreCase);
        /// <summary>Unix seconds of the last edit / game (MTG deck-builder decks only; null = unknown).</summary>
        public long? LastUsed;
        /// <summary>The deck the player sits down with now (vanilla decks have no <see cref="LastUsed"/>: Recent puts it first).</summary>
        public bool Current;

        /// <summary>Shared by the deck builder's list and the picker (this session only).</summary>
        public static MtgDeckSort CurrentSort = MtgDeckSort.Recent;

        public static MtgDeckSummary Of(MtgDeckStore.Deck deck)
        {
            var s = new MtgDeckSummary
            {
                Name = deck.Name ?? "", IsCommander = deck.IsCommander, Total = deck.Total, Target = deck.Target, LastUsed = deck.Save.Used,
            };
            var colors = new HashSet<char>();
            var sets = new HashSet<string>();
            void Add(MtgDeckStore.Entry e)
            {
                if (e == null || !e.Live) return;
                sets.Add(SetCode(e.Set));
                var info = MtgDeckStore.Info(e.Set, e.Pos);
                if (info == null) return;
                s.CardNames.Add(info.Name);
                AddColors(colors, info.Colors);
            }
            foreach (var e in deck.Entries.Where(e => e.Save.Count > 0)) Add(e);
            if (deck.Commander?.Live == true)
            {
                Add(deck.Commander);
                s.CommanderName = MtgDeckStore.Info(deck.Commander.Set, deck.Commander.Pos)?.Name;
            }
            foreach (var b in deck.Save.Basics.Where(b => b.Value > 0)) s.CardNames.Add(b.Key);
            s.Colors = Order(colors);
            s.SetCodes = sets.OrderBy(x => x).ToList();
            return s;
        }

        public static MtgDeckSummary Of(string name, MtgDeck deck, IEnumerable<CustomSet> sets)
        {
            var s = new MtgDeckSummary { Name = name ?? "", IsCommander = deck.IsCommander, Total = deck.Total };
            s.Target = s.IsCommander ? MtgDeckRules.CommanderDeckSize : MtgDeckRules.MinDeckSize;
            var colors = new HashSet<char>();
            foreach (var l in deck.Main.Concat(deck.Commander))
            {
                s.CardNames.Add(l.Card.Name);
                AddColors(colors, l.Card.Colors);
            }
            if (deck.IsCommander) s.CommanderName = deck.Commander[0].Card.Name;
            s.Colors = Order(colors);
            s.SetCodes = (sets ?? Enumerable.Empty<CustomSet>()).Select(SetCode).Distinct().OrderBy(x => x).ToList();
            return s;
        }

        private static string SetCode(CustomSet set) => (set.Def.Mtg?.SetCode ?? set.Def.Id).ToUpperInvariant();

        private static void AddColors(HashSet<char> into, IEnumerable<string> colors)
        {
            foreach (var c in colors ?? Enumerable.Empty<string>())
                if (!string.IsNullOrEmpty(c)) into.Add(char.ToUpperInvariant(c[0]));
        }

        private static string Order(HashSet<char> colors) => string.Concat(Wubrg.Where(colors.Contains));

        /// <summary>
        /// Name, commander, set code or card name contains the text. A text of only colour letters (e.g. "ur", "c" =
        /// colourless) means colours: decks of exactly those colours (and, from 3 letters on, deck names containing it).
        /// </summary>
        public bool Matches(string query)
        {
            query = (query ?? "").Trim();
            if (query.Length == 0) return true;
            bool Has(string s) => s != null && s.IndexOf(query, StringComparison.OrdinalIgnoreCase) >= 0;
            string q = query.ToUpperInvariant();
            if (q.Length <= 5 && (q == "C" || q.All(ch => Wubrg.IndexOf(ch) >= 0)))
            {
                string wanted = q == "C" ? "" : Order(new HashSet<char>(q));
                return wanted == Colors || (q.Length >= 3 && Has(Name));
            }
            return Has(Name) || Has(CommanderName) || SetCodes.Any(Has) || CardNames.Any(Has);
        }

        /// <summary>Mono W..G, then pairs, triples … in WUBRG order, colourless last.</summary>
        public int ColorSortKey
        {
            get
            {
                if (Colors.Length == 0) return int.MaxValue;
                int combo = 0; // base 6 over WUBRG positions: W < WU < WB … within the same colour count
                foreach (char ch in Colors) combo = combo * 6 + Wubrg.IndexOf(ch) + 1;
                return Colors.Length * 100000 + combo;
            }
        }

        public static IEnumerable<T> Sort<T>(IEnumerable<T> items, Func<T, MtgDeckSummary> summary, MtgDeckSort sort)
        {
            var list = items.Select((it, i) => (it, s: summary(it), i)).ToList();
            IOrderedEnumerable<(T it, MtgDeckSummary s, int i)> ordered;
            switch (sort)
            {
                case MtgDeckSort.Name:
                    ordered = list.OrderBy(t => t.s.Name, StringComparer.OrdinalIgnoreCase);
                    break;
                case MtgDeckSort.Colours:
                    ordered = list.OrderBy(t => t.s.ColorSortKey);
                    break;
                case MtgDeckSort.Kind:
                    ordered = list.OrderBy(t => t.s.IsCommander ? 1 : 0);
                    break;
                default: // Recent: newest first; unknown (old saves, vanilla decks) after them, the current deck first
                    ordered = list.OrderBy(t => t.s.LastUsed.HasValue ? 0 : 1).ThenByDescending(t => t.s.LastUsed ?? 0)
                        .ThenBy(t => t.s.Current ? 0 : 1);
                    break;
            }
            return ordered.ThenBy(t => sort == MtgDeckSort.Recent ? "" : t.s.Name, StringComparer.OrdinalIgnoreCase)
                .ThenBy(t => t.i).Select(t => t.it);
        }

        /// <summary>"today", "yesterday", "3 days ago" …; "" when unknown.</summary>
        public static string Ago(long? unixSeconds)
        {
            if (!unixSeconds.HasValue) return "";
            var then = DateTimeOffset.FromUnixTimeSeconds(unixSeconds.Value).LocalDateTime.Date;
            int days = (int)(DateTime.Now.Date - then).TotalDays;
            if (days <= 0) return "today";
            if (days == 1) return "yesterday";
            if (days < 60) return $"{days} days ago";
            return then.ToString("d MMM yyyy");
        }

        public static string SortLabel(MtgDeckSort s) => s == MtgDeckSort.Recent ? "Recent" : s.ToString();
    }
}

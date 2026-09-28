using System;
using System.Collections.Generic;
using System.Linq;

namespace TCGCustomCards.Runtime.Mtg
{
    /// <summary>What the deck builder knows about a card, for search/filter/sort (no game types — testable outside Unity).</summary>
    internal sealed class MtgCardInfo
    {
        public string Name = "";      // Forge/MTG name
        public string TypeLine = "";
        public string ManaCost = "";
        public List<string> Colors = new List<string>();
        public string Rarity = "";    // common, uncommon, rare, mythic (special, bonus)
        public float Cmc;
        public string Text = "";      // rules text
        public string SetId = "";
        public string SetName = "";

        public string FrontType => (TypeLine ?? "").Split(new[] { " // " }, StringSplitOptions.None)[0];
        public bool IsBasicLand => MtgDeckRules.IsBasicLand(TypeLine);
    }

    /// <summary>The builder's filter bar. Empty sets = no restriction. AND across groups, OR within a group.</summary>
    internal sealed class MtgCardFilter
    {
        public string Search = "";
        public readonly HashSet<string> Colors = new HashSet<string>(); // W U B R G, "C" colourless, "M" multicolour
        public readonly HashSet<string> Types = new HashSet<string>();  // Creature, Instant, Sorcery, Enchantment, Artifact, Planeswalker, Land, Battle
        public readonly HashSet<int> ManaValues = new HashSet<int>();   // 0..6 (6 = 6 or more)
        public readonly HashSet<string> Rarities = new HashSet<string>();
        public readonly HashSet<string> Sets = new HashSet<string>();
    }

    internal enum MtgSort { ManaValue, Name, Color, Rarity }

    internal static class MtgDeckRules
    {
        public const int MinDeckSize = 60;
        public const int MaxCopies = 4;
        public static readonly string[] Wubrg = { "W", "U", "B", "R", "G" };
        public static readonly string[] CardTypes = { "Creature", "Instant", "Sorcery", "Enchantment", "Artifact", "Planeswalker", "Land", "Battle" };
        public static readonly string[] RarityOrder = { "common", "uncommon", "rare", "mythic" };
        /// <summary>The free basic lands of the builder, with the colour they make.</summary>
        public static readonly (string name, string color)[] Basics =
            { ("Plains", "W"), ("Island", "U"), ("Swamp", "B"), ("Mountain", "R"), ("Forest", "G"), ("Wastes", "C") };

        /// <summary>Basic lands (incl. snow basics) have no copy limit.</summary>
        public static bool IsBasicLand(string typeLine)
        {
            string front = (typeLine ?? "").Split(new[] { " // " }, StringSplitOptions.None)[0];
            return front.Contains("Basic") && front.Contains("Land");
        }

        /// <summary>Mana value from a cost like {2}{U}{U} (front face; X = 0; hybrid/phyrexian = 1).</summary>
        public static float CmcFromCost(string cost)
        {
            float n = 0;
            cost = cost ?? "";
            int i = 0;
            while ((i = cost.IndexOf('{', i)) >= 0)
            {
                int j = cost.IndexOf('}', i);
                if (j < 0) break;
                string s = cost.Substring(i + 1, j - i - 1);
                if (int.TryParse(s, out int v)) n += v;
                else if (s != "X" && s != "Y" && s != "Z") n += 1;
                i = j + 1;
                if (cost.Substring(i).StartsWith(" // ")) break;
            }
            return n;
        }

        /// <summary>Colours a card counts as (its colours, else the coloured symbols in its cost).</summary>
        public static HashSet<string> ColorsOf(MtgCardInfo c)
        {
            var set = new HashSet<string>((c.Colors ?? new List<string>()).Where(x => Array.IndexOf(Wubrg, x) >= 0));
            if (set.Count == 0)
                foreach (var w in Wubrg)
                    if ((c.ManaCost ?? "").Contains(w)) set.Add(w);
            return set;
        }

        public static bool Matches(MtgCardInfo c, MtgCardFilter f)
        {
            if (!string.IsNullOrWhiteSpace(f.Search))
            {
                foreach (var word in f.Search.Split(new[] { ' ' }, StringSplitOptions.RemoveEmptyEntries))
                {
                    bool hit = Has(c.Name, word) || Has(c.TypeLine, word) || Has(c.Text, word);
                    if (!hit) return false;
                }
            }
            if (f.Colors.Count > 0)
            {
                var cols = ColorsOf(c);
                bool hit = cols.Any(f.Colors.Contains)
                           || (f.Colors.Contains("C") && cols.Count == 0)
                           || (f.Colors.Contains("M") && cols.Count > 1);
                if (!hit) return false;
            }
            if (f.Types.Count > 0 && !f.Types.Any(t => c.FrontType.IndexOf(t, StringComparison.OrdinalIgnoreCase) >= 0)) return false;
            if (f.ManaValues.Count > 0 && !f.ManaValues.Contains(Math.Min(6, (int)c.Cmc))) return false;
            if (f.Rarities.Count > 0 && !f.Rarities.Contains((c.Rarity ?? "").ToLowerInvariant())) return false;
            if (f.Sets.Count > 0 && !f.Sets.Contains(c.SetId)) return false;
            return true;
        }

        private static bool Has(string s, string word) => (s ?? "").IndexOf(word, StringComparison.OrdinalIgnoreCase) >= 0;

        public static IEnumerable<T> Sort<T>(IEnumerable<T> items, Func<T, MtgCardInfo> info, MtgSort sort)
        {
            switch (sort)
            {
                case MtgSort.Name: return items.OrderBy(x => info(x).Name, StringComparer.OrdinalIgnoreCase);
                case MtgSort.Color:
                    return items.OrderBy(x => ColorKey(info(x))).ThenBy(x => info(x).Cmc).ThenBy(x => info(x).Name, StringComparer.OrdinalIgnoreCase);
                case MtgSort.Rarity:
                    return items.OrderByDescending(x => Array.IndexOf(RarityOrder, (info(x).Rarity ?? "").ToLowerInvariant()))
                                .ThenBy(x => info(x).Cmc).ThenBy(x => info(x).Name, StringComparer.OrdinalIgnoreCase);
                default:
                    return items.OrderBy(x => info(x).IsBasicLand || info(x).FrontType.Contains("Land") ? 99 : info(x).Cmc)
                                .ThenBy(x => info(x).Name, StringComparer.OrdinalIgnoreCase);
            }
        }

        /// <summary>W, U, B, R, G, then multicolour, colourless, lands.</summary>
        private static int ColorKey(MtgCardInfo c)
        {
            if (c.FrontType.Contains("Land")) return 9;
            var cols = ColorsOf(c);
            if (cols.Count == 0) return 7;
            if (cols.Count > 1) return 6;
            return Array.IndexOf(Wubrg, cols.First());
        }

        /// <summary>Why another copy of a card can't be added (null = allowed). copiesInDeck = copies of that MTG name.</summary>
        public static string CanAddCopy(MtgCardInfo c, int copiesInDeck)
        {
            if (c.IsBasicLand) return null;
            return copiesInDeck >= MaxCopies ? $"You already have {MaxCopies} {c.Name} (max {MaxCopies} copies)" : null;
        }

        /// <summary>Problems that stop a deck from being played (empty = valid).</summary>
        public static List<string> Problems(int totalCards)
        {
            var p = new List<string>();
            if (totalCards < MinDeckSize) p.Add($"{MinDeckSize - totalCards} more card(s) needed (minimum {MinDeckSize})");
            return p;
        }
    }
}

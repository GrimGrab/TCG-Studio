using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using TCGCustomCards.Runtime;

namespace TCGCustomCards.Binder
{
    /// <summary>
    /// Groups the binder's sets by game or series: set.json "collection", else "Magic: The Gathering" for MTG sets, else
    /// "Other sets"; the game's own sets form one collection. Inside a collection sets go by release date, then name.
    /// </summary>
    internal static class Collections
    {
        public const string Vanilla = "Card Shop Simulator";
        public const string Mtg = "Magic: The Gathering";
        public const string Other = "Other sets";

        public static string Of(ECardExpansionType exp)
        {
            if (!Registry.IsCustom(exp)) return Vanilla;
            var def = Registry.Get(exp)?.Def;
            if (def == null) return Other;
            if (!string.IsNullOrWhiteSpace(def.Collection)) return def.Collection.Trim();
            return def.Mtg != null || def.Cards.Any(c => c.Mtg != null) ? Mtg : Other;
        }

        public static DateTime Released(ECardExpansionType exp)
        {
            if (!Registry.IsCustom(exp)) return DateTime.MinValue.AddDays((int)exp); // the game's own order
            var s = Registry.Get(exp)?.Def.Released;
            return DateTime.TryParse(s, CultureInfo.InvariantCulture, DateTimeStyles.None, out var d) ? d : DateTime.MaxValue;
        }

        /// <summary>Sets grouped into collections (vanilla first, then by name; "Other sets" last), each set list ordered.</summary>
        public static List<(string name, List<(ECardExpansionType exp, string name)> sets)> Group(
            IEnumerable<(ECardExpansionType exp, string name)> sets)
        {
            return sets
                .GroupBy(s => Of(s.exp))
                .OrderBy(g => g.Key == Vanilla ? 0 : g.Key == Other ? 2 : 1)
                .ThenBy(g => g.Key, StringComparer.OrdinalIgnoreCase)
                .Select(g => (g.Key, g.OrderBy(s => Released(s.exp)).ThenBy(s => s.name, StringComparer.OrdinalIgnoreCase).ToList()))
                .ToList();
        }
    }
}

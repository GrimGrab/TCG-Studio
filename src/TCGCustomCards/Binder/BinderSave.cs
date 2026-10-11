using System;
using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Runtime;

namespace TCGCustomCards.Binder
{
    /// <summary>
    /// The binder's part of the side-car (per save slot): last panel choices, favourite cards and the version picked per
    /// card. Keys are stable: custom sets by id ("custom:&lt;set&gt;", cards "custom:&lt;set&gt;:&lt;card&gt;"), vanilla by
    /// enum name ("Tetramon", cards "Tetramon:&lt;monster&gt;", Ghost's dimension half with ":d").
    /// </summary>
    internal sealed class BinderSave
    {
        public List<string> sets;
        public string sort, search, searchIn, show;
        public bool gaps, favouritesOnly;
        public List<int> borders;
        public List<bool> foil;
        public List<string> rarities, elements, colours, types;
        public List<int> manaValues;
        public List<int> grades;
        /// <summary>The graded album's own choices (sets there = a filter; none = every set).</summary>
        public BinderSave graded;
        public List<string> favourites;
        public Dictionary<string, int> picked;
    }

    internal static class Favourites
    {
        private static readonly HashSet<string> Keys = new HashSet<string>();

        public static bool Has(BinderEntry e) => Keys.Contains(BinderKeys.Card(e.Expansion, e.Dimension, e.Monster));
        public static bool Has(CardData d) => Keys.Contains(BinderKeys.Card(d.expansionType, d.isDestiny, d.monsterType));

        /// <summary>Toggles a card; returns whether it is a favourite now.</summary>
        public static bool Toggle(CardData d)
        {
            string k = BinderKeys.Card(d.expansionType, d.isDestiny, d.monsterType);
            if (Keys.Remove(k)) return false;
            Keys.Add(k);
            return true;
        }

        public static int Count => Keys.Count;
        internal static IEnumerable<string> All => Keys;
        internal static void Load(IEnumerable<string> keys) { Keys.Clear(); if (keys != null) Keys.UnionWith(keys); }
    }

    internal static class BinderKeys
    {
        public static string Set(ECardExpansionType exp) =>
            Registry.IsCustom(exp) ? "custom:" + Registry.Get(exp)?.Def.Id : exp.ToString();

        public static bool TryParseSet(string key, out ECardExpansionType exp)
        {
            exp = ECardExpansionType.None;
            if (string.IsNullOrEmpty(key)) return false;
            if (key.StartsWith("custom:"))
            {
                var set = Registry.Get(key.Substring(7));
                if (set == null) return false;
                exp = set.Expansion;
                return true;
            }
            return Enum.TryParse(key, out exp) && !Registry.IsCustom(exp);
        }

        public static string Card(ECardExpansionType exp, bool dimension, EMonsterType m)
        {
            if (Registry.TryGetCard(m, out var set, out int pos)) return "custom:" + set.Def.Id + ":" + set.Card(pos).Id;
            return exp + ":" + (int)m + (dimension ? ":d" : "");
        }

        public static string Card(BinderEntry e) => Card(e.Expansion, e.Dimension, e.Monster);
    }

    internal static class BinderStore
    {
        /// <summary>Unknown entries (sets removed for now) are kept and written back.</summary>
        private static BinderSave _orphans = new BinderSave();

        private static BinderSave Save(BinderQuery q) => new BinderSave
        {
            sets = q.Sets.Select(BinderKeys.Set).ToList(),
            sort = q.Sort.ToString(), search = q.Search, searchIn = q.In.ToString(), show = q.Show.ToString(),
            gaps = q.Gaps, favouritesOnly = q.FavouritesOnly,
            borders = q.Borders.ToList(), foil = q.Foil.ToList(), rarities = q.Rarities.ToList(),
            elements = q.Elements.Select(e => e.ToString()).ToList(), colours = q.Colours.ToList(), types = q.Types.ToList(),
            manaValues = q.ManaValues.ToList(), grades = q.Grades.ToList(),
        };

        private static BinderQuery Query(BinderSave s, BinderSort defaultSort)
        {
            var q = new BinderQuery { Sort = defaultSort };
            if (s == null) return q;
            foreach (var k in s.sets ?? new List<string>())
                if (BinderKeys.TryParseSet(k, out var exp) && !q.Sets.Contains(exp)) q.Sets.Add(exp);
            if (Enum.TryParse(s.sort, out BinderSort sort)) q.Sort = sort;
            q.Search = s.search ?? "";
            if (Enum.TryParse(s.searchIn, out SearchIn si)) q.In = si;
            if (Enum.TryParse(s.show, out BinderShow show)) q.Show = show;
            q.Gaps = s.gaps;
            q.FavouritesOnly = s.favouritesOnly;
            if (s.borders != null) q.Borders.UnionWith(s.borders);
            if (s.foil != null) q.Foil.UnionWith(s.foil);
            if (s.rarities != null) q.Rarities.UnionWith(s.rarities);
            foreach (var e in s.elements ?? new List<string>()) if (Enum.TryParse(e, out EElementIndex el)) q.Elements.Add(el);
            if (s.colours != null) q.Colours.UnionWith(s.colours);
            if (s.types != null) q.Types.UnionWith(s.types);
            if (s.manaValues != null) q.ManaValues.UnionWith(s.manaValues);
            if (s.grades != null) q.Grades.UnionWith(s.grades);
            return q;
        }

        public static BinderSave Export()
        {
            var s = Save(BinderIndex.Query);
            s.graded = Save(BinderIndex.GradedQuery);
            s.favourites = Favourites.All.ToList();
            s.picked = new Dictionary<string, int>(BinderIndex.PickedByKey());
            foreach (var f in _orphans.favourites ?? new List<string>()) if (!s.favourites.Contains(f)) s.favourites.Add(f);
            foreach (var kv in _orphans.picked ?? new Dictionary<string, int>()) if (!s.picked.ContainsKey(kv.Key)) s.picked[kv.Key] = kv.Value;
            return s;
        }

        public static void Load(BinderSave s)
        {
            BinderIndex.Reset();
            _orphans = new BinderSave { favourites = new List<string>(), picked = new Dictionary<string, int>() };
            var q = Query(s, BinderSort.SetOrder);
            BinderIndex.GradedQuery = Query(s?.graded, BinderSort.Grade);
            if (s != null)
            {
                // Favourites/picks of cards not loaded now (set removed) stay in the file.
                var known = new List<string>();
                foreach (var f in s.favourites ?? new List<string>())
                    if (KnownCard(f)) known.Add(f); else _orphans.favourites.Add(f);
                Favourites.Load(known);
                foreach (var kv in s.picked ?? new Dictionary<string, int>())
                    if (KnownCard(kv.Key)) BinderIndex.LoadPick(kv.Key, kv.Value); else _orphans.picked[kv.Key] = kv.Value;
            }
            else Favourites.Load(null);
            BinderIndex.Query = q;
            Plugin.Log.LogInfo($"Binder: {q.Sets.Count} set(s) picked, {Favourites.Count} favourite(s)");
        }

        /// <summary>A custom card key whose set is installed, or any vanilla key.</summary>
        private static bool KnownCard(string key)
        {
            if (!key.StartsWith("custom:")) return true;
            var parts = key.Split(':');
            return parts.Length >= 3 && Registry.Get(parts[1]) != null;
        }
    }
}

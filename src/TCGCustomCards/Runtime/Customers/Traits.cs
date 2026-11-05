using System;
using System.Collections.Generic;
using System.IO;
using Newtonsoft.Json;

namespace TCGCustomCards.Runtime.Customers
{
    /// <summary>
    /// Traits, their effects and the lines customers say: the embedded customer-defaults.json (the same file TCG Studio uses,
    /// studio\internal\customers\defaults.json), plus the setup's own lines. Data only: a new trait or line needs no code.
    /// </summary>
    internal static class Traits
    {
#pragma warning disable CS0649 // filled from JSON
        public class Trait
        {
            public string id, name, opposite, description;
            /// <summary>Only rolled while this module is on (e.g. the thief needs shoplifting).</summary>
            public string requires;
            /// <summary>How often it is rolled compared to other traits (default 1).</summary>
            public float weight = 1f;
            public Dictionary<string, float> effects = new Dictionary<string, float>();
            public Dictionary<string, List<string>> lines = new Dictionary<string, List<string>>();
        }

#pragma warning restore CS0649

        private class SituationDef
        {
            public string id, mood;
        }

        /// <summary>A page of the whiteboard's guide.</summary>
        public class GuideTopic
        {
            public string title;
            public List<string> lines = new List<string>();
        }

        /// <summary>A step of <see cref="Loyalty"/>: from this many points. The first has no floor.</summary>
        public class LoyaltyTier
        {
            public string name, greeting;
            public int at;
            public bool notify;
        }

        /// <summary>What earns a nickname (<see cref="Nicknames"/>).</summary>
        public class NickDeed
        {
            public string id, label, stat, style;
            public float at;
            public bool bad, fadeOnHit;
            public int fadeDays;
            public List<string> words = new List<string>();
        }

        private class DefaultsFile
        {
            public List<NickDeed> nicknames = new List<NickDeed>();
            public List<LoyaltyTier> loyaltyTiers = new List<LoyaltyTier>();
            public List<GuideTopic> guide = new List<GuideTopic>();
            [JsonProperty("base")]
            public Dictionary<string, float> Base = new Dictionary<string, float>();
            public List<Trait> traits = new List<Trait>();
            public List<SituationDef> situations = new List<SituationDef>();
            public Dictionary<string, List<string>> lines = new Dictionary<string, List<string>>();
            public Dictionary<string, List<string>> names = new Dictionary<string, List<string>>();
            public List<Trait> employeePerks = new List<Trait>();
            public List<SituationDef> employeeSituations = new List<SituationDef>();
            public Dictionary<string, List<string>> employeeLines = new Dictionary<string, List<string>>();
        }

        // Effects that add up across traits; every other effect multiplies (1 = no change).
        private static readonly HashSet<string> Additive = new HashSet<string> { "itemChance", "cardChance", "rip", "wish", "skill", "steal", "showOff", "sell", "crowd", "shopExp", "chase", "lostToYou", "beatYou", "bond", "followers", "browse", "hustle", "table" };

        private static DefaultsFile _data = new DefaultsFile();
        private static readonly Dictionary<string, Trait> ById = new Dictionary<string, Trait>();
        private static Dictionary<string, List<string>> _setupLines, _setupRemoved, _setupNames, _setupNamesOff, _ownEmployeeLines, _ownEmployeeRemoved;
        private static readonly Dictionary<string, List<string>> NameLists = new Dictionary<string, List<string>>();

        public static IReadOnlyList<Trait> All => _data.traits;
        public static IReadOnlyList<NickDeed> Deeds => _data.nicknames;
        public static List<LoyaltyTier> LoyaltyTiers => _data.loyaltyTiers;
        public static List<GuideTopic> Guide => _data.guide;
        public static List<Trait> EmployeePerks => _data.employeePerks;

        public static NickDeed Deed(string id)
        {
            if (id != null)
                foreach (var d in _data.nicknames) if (d.id == id) return d;
            return null;
        }

        /// <summary>A deed's words: the built-in ones minus those the setup switched off, plus the setup's.</summary>
        public static List<string> DeedWords(NickDeed deed)
        {
            var words = new List<string>(deed.words);
            var off = Of(_setupNamesOff, "nick:" + deed.id);
            if (off != null) words.RemoveAll(off.Contains);
            var own = Of(_setupNames, "nick:" + deed.id);
            if (own != null) words.AddRange(own);
            return words;
        }

        public static void Load(CustomerLinesFile own)
        {
            _ownEmployeeLines = own?.employeeLines;
            _ownEmployeeRemoved = own?.employeeRemoved;
            _setupLines = own?.lines;
            _setupRemoved = own?.removed;
            _setupNames = own?.names;
            _setupNamesOff = own?.namesOff;
            NameLists.Clear();
            try
            {
                using (var s = typeof(Traits).Assembly.GetManifestResourceStream("TCGCustomCards.customer-defaults.json"))
                using (var r = new StreamReader(s))
                    _data = JsonConvert.DeserializeObject<DefaultsFile>(r.ReadToEnd()) ?? new DefaultsFile();
            }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Customer traits/lines could not be read: {e.Message}");
                _data = new DefaultsFile();
            }
            ById.Clear();
            foreach (var t in _data.traits) if (t.id != null) ById[t.id] = t;
            MoodBySituation.Clear();
            foreach (var s in _data.situations) if (s.id != null) MoodBySituation[s.id] = s.mood;
        }

        /// <summary>
        /// A list of names ("female", "male", "last"): the built-in ones minus those the setup switched off, plus the setup's. With all of
        /// them switched off and none added, the built-in ones are used.
        /// </summary>
        public static List<string> NameList(string list)
        {
            if (NameLists.TryGetValue(list, out var ready)) return ready;
            _data.names.TryGetValue(list, out var builtIn);
            builtIn = builtIn ?? new List<string>();
            var names = new List<string>(builtIn);
            var off = Of(_setupNamesOff, list);
            if (off != null) names.RemoveAll(off.Contains);
            var own = Of(_setupNames, list);
            if (own != null) names.AddRange(own);
            return NameLists[list] = names.Count > 0 ? names : builtIn;
        }

        private static readonly Dictionary<string, string> MoodBySituation = new Dictionary<string, string>();

        /// <summary>How a situation's lines sound (the talking sounds); null = neutral.</summary>
        public static string MoodOf(string situation) => situation != null && MoodBySituation.TryGetValue(situation, out var m) ? m : null;

        public static Trait Get(string id) => id != null && ById.TryGetValue(id, out var t) ? t : null;

        public static string NameOf(string id) => Get(id)?.name ?? id;

        /// <summary>The persona's effect: base + traits for additive effects, base × traits for the others.</summary>
        public static float Effect(Persona p, string key)
        {
            bool add = Additive.Contains(key);
            float v = _data.Base.TryGetValue(key, out var b) ? b : add ? 0f : 1f;
            if (p?.traits == null) return v;
            foreach (var id in p.traits)
            {
                var t = Get(id);
                if (t?.effects == null || !t.effects.TryGetValue(key, out var e)) continue;
                v = add ? v + e : v * e;
            }
            return v;
        }

        /// <summary>
        /// Lines for a situation, all added up: the built-in lines minus those the setup switched off, the setup's own, the
        /// persona's traits' lines; then the persona's switched-off lines go and its own lines come in. Null when none are left.
        /// </summary>
        /// <summary>An employee perk by id, or null.</summary>
        public static Trait EmployeePerk(string id) => _data.employeePerks.Find(t => t.id == id);

        public static string EmployeeMood(string situation) => _data.employeeSituations.Find(s => s.id == situation)?.mood;

        /// <summary>An employee's lines, adding up like <see cref="LinesFor"/>; def = null for one of the game's.</summary>
        public static List<string> EmployeeLines(Core.EmployeeDef def, string situation)
        {
            var list = new List<string>();
            if (_data.employeeLines.TryGetValue(situation, out var dl) && dl != null)
            {
                var off = Of(_ownEmployeeRemoved, situation);
                foreach (var l in dl) if (off == null || !off.Contains(l)) list.Add(l);
            }
            var sl = Of(_ownEmployeeLines, situation);
            if (sl != null) list.AddRange(sl);
            if (def?.Perks != null)
                foreach (var id in def.Perks)
                {
                    var t = EmployeePerk(id);
                    if (t?.lines != null && t.lines.TryGetValue(situation, out var tl) && tl != null) list.AddRange(tl);
                }
            var mine = Of(def?.Removed, situation);
            if (mine != null) list.RemoveAll(mine.Contains);
            var own = Of(def?.Lines, situation);
            if (own != null) list.AddRange(own);
            return list.Count > 0 ? list : null;
        }

        public static List<string> LinesFor(Persona p, string situation)
        {
            var list = new List<string>();
            if (_data.lines.TryGetValue(situation, out var dl) && dl != null)
            {
                var off = Of(_setupRemoved, situation);
                foreach (var l in dl) if (off == null || !off.Contains(l)) list.Add(l);
            }
            var sl = Of(_setupLines, situation);
            if (sl != null) list.AddRange(sl);
            if (p?.traits != null)
                foreach (var id in p.traits)
                {
                    var t = Get(id);
                    if (t?.lines != null && t.lines.TryGetValue(situation, out var tl) && tl != null) list.AddRange(tl);
                }
            var mine = Of(p?.removed, situation);
            if (mine != null) list.RemoveAll(mine.Contains);
            var own = Of(p?.lines, situation);
            if (own != null) list.AddRange(own);
            return list.Count > 0 ? list : null;
        }

        private static List<string> Of(Dictionary<string, List<string>> map, string situation) =>
            map != null && map.TryGetValue(situation, out var l) && l != null && l.Count > 0 ? l : null;

        /// <summary>1–2 random traits that don't contradict each other.</summary>
        public static List<string> Roll(Random rng)
        {
            var picked = new List<string>();
            if (All.Count == 0) return picked;
            int roll100 = rng.Next(100);
            int want = roll100 < 25 ? 0 : roll100 < 75 ? 1 : 2; // plenty of people have nothing special about them
            var usable = new List<Trait>();
            float total = 0f;
            foreach (var t in All)
            {
                if (t.weight <= 0f || (t.requires != null && !Cast.ModuleOn(t.requires))) continue;
                usable.Add(t);
                total += t.weight;
            }
            for (int tries = 0; picked.Count < want && tries < 20 && usable.Count > 0; tries++)
            {
                double roll = rng.NextDouble() * total;
                var t = usable[usable.Count - 1];
                foreach (var u in usable)
                    if ((roll -= u.weight) < 0) { t = u; break; }
                if (picked.Contains(t.id)) continue;
                if (picked.Exists(id => Get(id)?.opposite == t.id || t.opposite == id)) continue;
                picked.Add(t.id);
            }
            return picked;
        }
    }
}

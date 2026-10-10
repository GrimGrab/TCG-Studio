using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;

namespace TCGCustomCards.Runtime.Mtg
{
    /// <summary>One MTG card as the deck builder sees it (no game types, so it can be tested outside Unity).</summary>
    internal sealed class MtgCard
    {
        public string Name;      // Forge name
        public string SetCode;   // DOM (may be null)
        public string TypeLine = "";
        public string ManaCost = "";
        public List<string> Colors = new List<string>();

        public string FrontType => (TypeLine ?? "").Split(new[] { " // " }, StringSplitOptions.None)[0];
        public bool IsBasicLand => FrontType.Contains("Basic") && FrontType.Contains("Land");
        public bool IsLand => FrontType.Contains("Land");
        public bool IsCreature => FrontType.Contains("Creature");
    }

    internal sealed class MtgDeckLine
    {
        public int Count;
        public MtgCard Card;
    }

    internal sealed class MtgDeck
    {
        public string Name;
        public readonly List<MtgDeckLine> Main = new List<MtgDeckLine>();
        /// <summary>Commander decks: the commander(s), written as the .dck [Commander] section (Forge: DeckSection.Commander).</summary>
        public readonly List<MtgDeckLine> Commander = new List<MtgDeckLine>();
        public bool IsCommander => Commander.Count > 0;
        public int Total => Main.Sum(l => l.Count) + Commander.Sum(l => l.Count);
        /// <summary>Notes for the player (dropped basics, copy limit, added lands).</summary>
        public readonly List<string> Notes = new List<string>();

        /// <summary>Forge .dck text (forge-core DeckSerializer format).</summary>
        public string ToDck()
        {
            var sb = new StringBuilder();
            sb.Append("[metadata]\r\nName=").Append(Name).Append("\r\n");
            if (Commander.Count > 0)
            {
                sb.Append("[Commander]\r\n");
                foreach (var l in Commander) Line(sb, l);
            }
            sb.Append("[Main]\r\n");
            foreach (var l in Main) Line(sb, l);
            return sb.ToString();
        }

        private static void Line(StringBuilder sb, MtgDeckLine l)
        {
            sb.Append(l.Count).Append(' ').Append(l.Card.Name);
            if (!string.IsNullOrEmpty(l.Card.SetCode)) sb.Append('|').Append(l.Card.SetCode);
            sb.Append("\r\n");
        }
    }

    /// <summary>
    /// Turns an in-game deck (50 cards, max 4 per card, basics count against the limit) into a legal 60+ card Forge
    /// constructed deck: basic lands the player put in are dropped and replaced by free basics in the deck's colours,
    /// copies are merged by Forge name (different printings) and capped at 4. Also builds AI opponent decks.
    /// </summary>
    internal static class MtgDeckBuilder
    {
        public const int MinDeck = 60;
        public const int MaxCopies = 4;
        private static readonly string[] Wubrg = { "W", "U", "B", "R", "G" };
        private static readonly Dictionary<string, string> BasicFor = new Dictionary<string, string>
        {
            { "W", "Plains" }, { "U", "Island" }, { "B", "Swamp" }, { "R", "Mountain" }, { "G", "Forest" }, { "C", "Wastes" }
        };

        /// <param name="cards">Cards and counts from the in-game deck (only MTG cards).</param>
        /// <param name="basicsPool">MTG cards from the sets in play — basics found here are used for the added lands' printing.</param>
        public static MtgDeck FromPlayerDeck(string name, IEnumerable<(MtgCard card, int count)> cards, IEnumerable<MtgCard> basicsPool)
        {
            var deck = new MtgDeck { Name = name };
            int droppedBasics = 0, capped = 0;
            var byName = new Dictionary<string, MtgDeckLine>(StringComparer.OrdinalIgnoreCase);
            foreach (var (card, count) in cards)
            {
                if (card == null || string.IsNullOrEmpty(card.Name) || count <= 0) continue;
                if (card.IsBasicLand) { droppedBasics += count; continue; }
                if (!byName.TryGetValue(card.Name, out var line))
                {
                    line = new MtgDeckLine { Card = card };
                    byName[card.Name] = line;
                    deck.Main.Add(line);
                }
                int room = MaxCopies - line.Count;
                if (count > room) capped += count - Math.Max(room, 0);
                line.Count += Math.Min(count, Math.Max(room, 0));
            }
            if (droppedBasics > 0) deck.Notes.Add($"{droppedBasics} basic land(s) from your deck replaced by free lands");
            if (capped > 0) deck.Notes.Add($"{capped} card(s) over the {MaxCopies}-copy limit left out");
            AddLands(deck, basicsPool);
            return deck;
        }

        /// <summary>
        /// The MTG deck builder's deck as is: its cards (printings merged by name) plus the chosen free basics, printed from the
        /// deck's sets when they have that basic. No lands added, nothing dropped — the builder already enforced the rules.
        /// </summary>
        public static MtgDeck FromMtgDeck(string name, IEnumerable<(MtgCard card, int count)> cards, IDictionary<string, int> basics, IEnumerable<MtgCard> basicsPool)
        {
            var deck = new MtgDeck { Name = name };
            var byName = new Dictionary<string, MtgDeckLine>(StringComparer.OrdinalIgnoreCase);
            foreach (var (card, count) in cards)
            {
                if (card == null || string.IsNullOrEmpty(card.Name) || count <= 0) continue;
                if (!byName.TryGetValue(card.Name, out var line))
                {
                    line = new MtgDeckLine { Card = card };
                    byName[card.Name] = line;
                    deck.Main.Add(line);
                }
                line.Count += count;
            }
            var printings = (basicsPool ?? Enumerable.Empty<MtgCard>()).Where(c => c != null && c.IsBasicLand && !string.IsNullOrEmpty(c.Name))
                .GroupBy(c => c.Name, StringComparer.OrdinalIgnoreCase).ToDictionary(g => g.Key, g => g.First(), StringComparer.OrdinalIgnoreCase);
            foreach (var kv in basics ?? new Dictionary<string, int>())
            {
                if (kv.Value <= 0) continue;
                var card = printings.TryGetValue(kv.Key, out var p) ? p : new MtgCard { Name = kv.Key, TypeLine = "Basic Land — " + kv.Key };
                deck.Main.Add(new MtgDeckLine { Card = card, Count = kv.Value });
            }
            return deck;
        }

        /// <summary>
        /// A random two-colour deck for the AI from <paramref name="pool"/> (the sets the player's deck uses): ~36 spells
        /// (creature-heavy, 1–3 copies each) plus basics.
        /// </summary>
        public static MtgDeck Opponent(string name, IList<MtgCard> pool, Random rng)
        {
            var deck = new MtgDeck { Name = name };
            var spells = pool.Where(c => c != null && !string.IsNullOrEmpty(c.Name) && !c.IsLand)
                .GroupBy(c => c.Name, StringComparer.OrdinalIgnoreCase).Select(g => g.First()).ToList();
            if (spells.Count == 0) return null;

            // Colours weighted by how many spells use them; pick two different ones.
            var weight = Wubrg.ToDictionary(c => c, c => spells.Count(s => ColorsOf(s).Contains(c)));
            var colors = new List<string>();
            for (int i = 0; i < 2; i++)
            {
                var left = Wubrg.Where(c => !colors.Contains(c) && weight[c] > 0).ToList();
                if (left.Count == 0) break;
                int roll = rng.Next(left.Sum(c => weight[c]));
                foreach (var c in left) { roll -= weight[c]; if (roll < 0) { colors.Add(c); break; } }
            }
            var fits = spells.Where(s => ColorsOf(s).All(colors.Contains)).OrderBy(_ => rng.Next()).ToList();
            var creatures = fits.Where(s => s.IsCreature).ToList();
            var others = fits.Where(s => !s.IsCreature).ToList();

            int want = 36;
            void Take(List<MtgCard> from, int target)
            {
                foreach (var c in from)
                {
                    if (deck.Total >= target) break;
                    if (deck.Main.Any(l => l.Card.Name == c.Name)) continue;
                    int copies = Math.Min(Cmc(c) >= 5 ? 1 + rng.Next(2) : 2 + rng.Next(2), target - deck.Total);
                    deck.Main.Add(new MtgDeckLine { Card = c, Count = copies });
                }
            }
            Take(creatures, 22);
            Take(others, want);
            Take(creatures, want); // not enough non-creatures: more creatures
            if (deck.Total == 0) return null;
            AddLands(deck, pool);
            return deck;
        }

        /// <summary>Adds free basics so lands ≈ 40% (at least <see cref="MinDeck"/> cards), split by the spells' colour pips.</summary>
        private static void AddLands(MtgDeck deck, IEnumerable<MtgCard> basicsPool)
        {
            int lands = deck.Main.Where(l => l.Card.IsLand).Sum(l => l.Count);
            int spells = deck.Total - lands;
            int add = Math.Max(MinDeck - deck.Total, (int)Math.Round(spells * 2 / 3.0) - lands);
            if (add <= 0) return;

            var pips = Wubrg.ToDictionary(c => c, c => 0.0);
            foreach (var l in deck.Main.Where(l => !l.Card.IsLand))
                foreach (var kv in Pips(l.Card)) pips[kv.Key] += kv.Value * l.Count;
            double totalPips = pips.Values.Sum();
            var split = new Dictionary<string, int>();
            if (totalPips <= 0) split["C"] = add;
            else
            {
                // Largest remainder so the counts add up exactly.
                var exact = pips.Where(kv => kv.Value > 0).ToDictionary(kv => kv.Key, kv => add * kv.Value / totalPips);
                foreach (var kv in exact) split[kv.Key] = (int)Math.Floor(kv.Value);
                int rest = add - split.Values.Sum();
                foreach (var kv in exact.OrderByDescending(kv => kv.Value - Math.Floor(kv.Value)).Take(rest)) split[kv.Key]++;
            }

            var printings = (basicsPool ?? Enumerable.Empty<MtgCard>()).Where(c => c != null && c.IsBasicLand && !string.IsNullOrEmpty(c.Name))
                .GroupBy(c => c.Name, StringComparer.OrdinalIgnoreCase).ToDictionary(g => g.Key, g => g.First(), StringComparer.OrdinalIgnoreCase);
            foreach (var kv in split.Where(kv => kv.Value > 0))
            {
                string basic = BasicFor[kv.Key];
                var card = printings.TryGetValue(basic, out var p) ? p : new MtgCard { Name = basic, TypeLine = "Basic Land — " + basic };
                deck.Main.Add(new MtgDeckLine { Card = card, Count = kv.Value });
            }
            deck.Notes.Add($"{add} free basic land(s) added");
        }

        /// <summary>Coloured mana symbols in the cost: {W} = 1, hybrid {W/U} = ½ each, phyrexian {W/P} = 1. Falls back on colours.</summary>
        internal static Dictionary<string, double> Pips(MtgCard c)
        {
            var d = new Dictionary<string, double>();
            string cost = c.ManaCost ?? "";
            int i = 0;
            while ((i = cost.IndexOf('{', i)) >= 0)
            {
                int j = cost.IndexOf('}', i);
                if (j < 0) break;
                var parts = cost.Substring(i + 1, j - i - 1).Split('/').Where(p => Array.IndexOf(Wubrg, p) >= 0).ToList();
                foreach (var p in parts) d[p] = (d.TryGetValue(p, out var v) ? v : 0) + 1.0 / parts.Count;
                i = j + 1;
                if (cost.Substring(i).StartsWith(" // ")) break; // front face only
            }
            if (d.Count == 0)
                foreach (var col in c.Colors ?? new List<string>())
                    if (Array.IndexOf(Wubrg, col) >= 0) d[col] = 1;
            return d;
        }

        private static HashSet<string> ColorsOf(MtgCard c)
        {
            var set = new HashSet<string>(Pips(c).Keys);
            foreach (var col in c.Colors ?? new List<string>()) set.Add(col);
            return set;
        }

        private static int Cmc(MtgCard c)
        {
            int n = 0;
            string cost = c.ManaCost ?? "";
            int i = 0;
            while ((i = cost.IndexOf('{', i)) >= 0)
            {
                int j = cost.IndexOf('}', i);
                if (j < 0) break;
                string s = cost.Substring(i + 1, j - i - 1);
                n += int.TryParse(s, out int v) ? v : s == "X" ? 0 : 1;
                i = j + 1;
                if (cost.Substring(i).StartsWith(" // ")) break;
            }
            return n;
        }
    }
}

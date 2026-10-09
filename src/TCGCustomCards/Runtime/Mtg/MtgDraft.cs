using System;
using System.Collections.Generic;
using System.Linq;
using Newtonsoft.Json.Linq;

namespace TCGCustomCards.Runtime.Mtg
{
    /// <summary>One physical card in a draft: the exact in-game copy rolled from a real pack (border/foil included).</summary>
    internal sealed class DraftCard
    {
        public int Ref;
        public CardData Data;
        public CustomSet Set;
        public int Pos;
        public MtgCard Mtg;
    }

    /// <summary>A seat's result: its picks; AI seats also Forge's 40-card deck and rating, the human seat Forge's suggestion.</summary>
    internal sealed class DraftSeat
    {
        public readonly List<DraftCard> Picks = new List<DraftCard>();
        public MtgDeck Deck;       // AI seats
        public MtgDeck Suggested;  // human seat
        public double Rating;
        public string Colors = "";
    }

    /// <summary>
    /// Booster draft over real in-game packs: rolls each pack item with <see cref="PackRoller"/> (the same roll as opening it),
    /// sends the pod to the Forge bridge (<c>draft</c>, docs/mtg-forge.md "Booster draft engine"), relays the human's picks
    /// (<c>ask kind=draftpick</c>) and turns <c>draftdone</c> into decks. Forge's draft AI picks and builds for every other
    /// seat. One draft at a time.
    /// </summary>
    internal static class MtgDraft
    {
        public static readonly Dictionary<int, DraftCard> Cards = new Dictionary<int, DraftCard>();
        /// <summary>The human's current pick question (null while waiting), from the bridge.</summary>
        public static JObject Ask;
        public static bool Running { get; private set; }
        /// <summary>The running/last job is Sealed (no picking: every seat builds from its own packs).</summary>
        public static bool IsSealed { get; private set; }
        public static int HumanSeat { get; private set; } = -1;
        public static string LandSet { get; private set; }
        public static List<DraftSeat> Seats;
        public static readonly List<string> Lines = new List<string>();

        private static int _id, _nextRef = 1;
        private static JObject _outbox;
        private static Action<string> _onDone; // null error = finished, else why it stopped

        /// <summary>Rolls one pack item exactly like opening it; cards without MTG data are left out. Null if not a custom pack.</summary>
        public static List<DraftCard> Roll(EItemType packItem)
        {
            var pack = Registry.GetByItem(packItem);
            if (pack == null) return null;
            int n = pack.Def.Slots.Count > 0 ? pack.Def.Slots.Sum(s => s.Count) : pack.Def.CardsPerPack;
            var pool = Enumerable.Range(0, Math.Max(1, n)).Select(_ => new CardData()).ToList();
            var rolled = new List<CardData>();
            PackRoller.Roll(pack, pool, rolled);
            var cards = new List<DraftCard>();
            foreach (var c in rolled)
            {
                if (!pack.Set.PosByMonster.TryGetValue(c.monsterType, out int pos)) continue;
                var mtg = MtgMode.ToMtg(pack.Set.Def, pack.Set.Card(pos));
                if (mtg == null) continue;
                var card = new DraftCard { Ref = _nextRef++, Data = c, Set = pack.Set, Pos = pos, Mtg = mtg };
                Cards[card.Ref] = card;
                cards.Add(card);
            }
            return cards;
        }

        /// <summary>A known in-game copy (e.g. one saved when the packs were rolled) as a draft card; null when not an MTG card.</summary>
        public static DraftCard FromData(CardData c)
        {
            if (c == null || !Registry.IsCustom(c.expansionType)) return null;
            var set = Registry.Get(c.expansionType);
            if (set == null || !set.PosByMonster.TryGetValue(c.monsterType, out int pos)) return null;
            var mtg = MtgMode.ToMtg(set.Def, set.Card(pos));
            if (mtg == null) return null;
            var card = new DraftCard { Ref = _nextRef++, Data = c, Set = set, Pos = pos, Mtg = mtg };
            Cards[card.Ref] = card;
            return card;
        }

        /// <summary>The seat's own copies that make up its deck (by name, from its picks/pool), for the table's card faces.</summary>
        public static List<CardData> DeckCopies(DraftSeat seat, MtgDeck deck)
        {
            var copies = new List<CardData>();
            var left = new List<DraftCard>(seat?.Picks ?? new List<DraftCard>());
            foreach (var line in deck?.Main ?? new List<MtgDeckLine>())
                for (int i = 0; i < line.Count; i++)
                {
                    var c = left.FirstOrDefault(x => string.Equals(x.Mtg.Name, line.Card.Name, StringComparison.OrdinalIgnoreCase));
                    if (c == null) break; // free basics etc.
                    left.Remove(c);
                    copies.Add(c.Data);
                }
            return copies;
        }

        /// <summary>The first of these sets with basic lands (Forge prints the AI decks' basics from it), else null.</summary>
        public static string LandSetOf(IEnumerable<CustomSet> sets) =>
            sets.Where(s => s.Def.Cards.Any(c => MtgMode.ToMtg(s.Def, c)?.IsBasicLand == true))
                .Select(s => s.Def.Mtg?.SetCode).FirstOrDefault(c => !string.IsNullOrWhiteSpace(c));

        /// <summary>
        /// Starts a draft. <paramref name="packs"/>[round][seat] = cards from <see cref="Roll"/>; <paramref name="humanSeat"/> −1
        /// = all AI. <paramref name="onDone"/> gets null when <see cref="Seats"/> is ready, or an error / "aborted".
        /// </summary>
        public static string Start(List<List<List<DraftCard>>> packs, int humanSeat, string landSet, Action<string> onDone)
        {
            if (Running) return "Forge is busy with another table";
            string err = ForgeBridge.EnsureStarted();
            if (err != null) return err;
            MtgSession.EnsureRunner();
            Running = true;
            Ask = null;
            Seats = null;
            Lines.Clear();
            HumanSeat = humanSeat;
            LandSet = landSet;
            IsSealed = false;
            _onDone = onDone;
            _id++;
            _outbox = new JObject
            {
                ["t"] = "draft", ["id"] = _id, ["humanSeat"] = humanSeat, ["landSet"] = landSet,
                ["packs"] = new JArray(packs.Select(round => new JArray(round.Select(pack => new JArray(pack.Select(c => new JObject
                {
                    ["ref"] = c.Ref, ["name"] = c.Mtg.Name, ["set"] = c.Mtg.SetCode,
                })))))),
            };
            Plugin.Log.LogInfo($"MTG draft {_id}: {packs.FirstOrDefault()?.Count ?? 0} seats × {packs.Count} packs, human seat {humanSeat}, land set {landSet ?? "Forge's"}");
            return null;
        }

        /// <summary>
        /// Starts a Sealed build: <paramref name="pools"/>[seat] = every card that seat opened; Forge builds each customer's deck
        /// (and a suggestion for <paramref name="humanSeat"/>). Same completion as a draft (<see cref="Seats"/>, Picks = the pool).
        /// </summary>
        public static string StartSealed(List<List<DraftCard>> pools, int humanSeat, string landSet, Action<string> onDone)
        {
            if (Running) return "Forge is busy with another table";
            string err = ForgeBridge.EnsureStarted();
            if (err != null) return err;
            MtgSession.EnsureRunner();
            Running = true;
            Ask = null;
            Seats = null;
            Lines.Clear();
            HumanSeat = humanSeat;
            LandSet = landSet;
            IsSealed = true;
            _onDone = onDone;
            _id++;
            _outbox = new JObject
            {
                ["t"] = "sealed", ["id"] = _id, ["humanSeat"] = humanSeat, ["landSet"] = landSet,
                ["seats"] = new JArray(pools.Select(pool => new JArray(pool.Select(c => new JObject
                {
                    ["ref"] = c.Ref, ["name"] = c.Mtg.Name, ["set"] = c.Mtg.SetCode,
                })))),
            };
            Plugin.Log.LogInfo($"MTG sealed {_id}: {pools.Count} seats, human seat {humanSeat}, land set {landSet ?? "Forge's"}");
            return null;
        }

        /// <summary>Every frame (MtgSession.Pump): send the queued draft once Forge is ready; stop if Forge died.</summary>
        public static void Tick()
        {
            if (!Running) return;
            if (!ForgeBridge.Running)
            {
                Finish("Forge stopped (see TCGForge\\userdata\\tcgcc-bridge.log)");
                return;
            }
            if (_outbox != null && ForgeBridge.Ready)
            {
                ForgeBridge.Send(_outbox);
                _outbox = null;
            }
        }

        /// <summary>The human picks card <paramref name="index"/> of the current pack.</summary>
        public static void Pick(int index)
        {
            if (Ask == null) return;
            ForgeBridge.Send(new JObject { ["t"] = "answer", ["id"] = Ask["id"], ["picked"] = new JArray(index) });
            Ask = null;
        }

        /// <summary>The player left the draft: Forge stops it (draftdone aborted follows).</summary>
        public static void Quit()
        {
            if (!Running) return;
            if (_outbox != null) { _outbox = null; Finish("aborted"); return; }
            ForgeBridge.Send(new JObject { ["t"] = "draftquit" });
        }

        public static void OnAsk(JObject m) => Ask = m;

        public static void OnDone(JObject m)
        {
            if ((int?)m["id"] != _id) return;
            foreach (var l in (m["lines"] as JArray) ?? new JArray()) Lines.Add((string)l);
            if ((bool?)m["aborted"] == true)
            {
                Finish((string)m["error"] is string e && e.Length > 0 ? "Forge's draft failed: " + e : "aborted");
                return;
            }
            Seats = new List<DraftSeat>();
            foreach (JObject s in (m["seats"] as JArray) ?? new JArray())
            {
                var seat = new DraftSeat { Rating = (double?)s["rating"] ?? 0, Colors = (string)s["colors"] ?? "" };
                foreach (var r in (s["picks"] as JArray) ?? new JArray())
                    if (Cards.TryGetValue((int)r, out var c)) seat.Picks.Add(c);
                if (s["deck"] is JArray d) seat.Deck = ToDeck(d, $"{ForgeLauncher.DeckPrefix}Draft {Seats.Count + 1}");
                if (s["suggested"] is JArray sg) seat.Suggested = ToDeck(sg, ForgeLauncher.DeckPrefix + "My draft");
                Seats.Add(seat);
            }
            Plugin.Log.LogInfo($"MTG draft {_id} done: {Seats.Count} seats; " +
                               string.Join(", ", Seats.Select((s, i) => i == HumanSeat ? "you" : $"{s.Colors} {s.Rating:0.0}")) +
                               (Lines.Count > 0 ? "; " + string.Join(" | ", Lines) : ""));
            Finish(null);
        }

        private static void Finish(string error)
        {
            Running = false;
            Ask = null;
            _outbox = null;
            if (error != null) Plugin.Log.LogWarning($"MTG draft {_id}: {error}");
            var cb = _onDone;
            _onDone = null;
            try { cb?.Invoke(error); }
            catch (Exception e) { Plugin.Log.LogError($"MTG draft callback failed: {e}"); }
        }

        /// <summary>Forge's deck rows {n, name, set} → our deck (type line from our set's card when we have it).</summary>
        private static MtgDeck ToDeck(JArray rows, string name)
        {
            var deck = new MtgDeck { Name = ForgeLauncher.Safe(name) };
            foreach (JObject r in rows)
            {
                string n = (string)r["name"], set = (string)r["set"];
                var card = new MtgCard { Name = n, SetCode = set };
                if (MtgCardFaces.Find(set, n, out var cs, out int pos) && cs.Card(pos).Mtg != null)
                {
                    var m = cs.Card(pos).Mtg;
                    card.TypeLine = m.TypeLine ?? "";
                    card.ManaCost = m.ManaCost ?? "";
                    card.Colors = m.Colors ?? new List<string>();
                }
                deck.Main.Add(new MtgDeckLine { Card = card, Count = (int?)r["n"] ?? 1 });
            }
            return deck;
        }

        /// <summary>
        /// The <c>aideck</c>-style info for the table's "Customer's deck" chip and the reveal after the match (kind per card from
        /// its type line).
        /// </summary>
        public static JObject DeckInfo(DraftSeat seat) => DeckInfo(seat.Deck, seat.Rating, seat.Colors);

        public static JObject DeckInfo(MtgDeck deck, double rating, string colors) => new JObject
        {
            ["style"] = "draft", ["source"] = "forge", ["colors"] = colors, ["cards"] = deck?.Total ?? 0,
            ["profile"] = "", ["lines"] = new JArray($"Drafted by Forge's draft AI (rating {rating:0.0})"),
            ["list"] = new JArray((deck?.Main ?? new List<MtgDeckLine>()).Select(l => new JObject
            {
                ["n"] = l.Count, ["name"] = l.Card.Name, ["set"] = l.Card.SetCode,
                ["kind"] = l.Card.IsLand ? "land" : l.Card.IsCreature ? "creature" : "spell",
            })),
        };
    }
}

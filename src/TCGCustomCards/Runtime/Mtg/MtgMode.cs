using System;
using System.Collections.Generic;
using System.Linq;
using Newtonsoft.Json.Linq;
using TCGCustomCards.Core;

namespace TCGCustomCards.Runtime.Mtg
{
    /// <summary>[MTG] AiDeckStyle: which Forge generator builds the customer's deck (bridge mode).</summary>
    public enum AiDeckStyle { Random, Sealed }

    /// <summary>[MTG - AI deck] AiDeckSets: which sets the customer's deck comes from.</summary>
    public enum AiDeckSets { RandomInstalled, RandomLicensed, MatchMyDeck }

    /// <summary>[MTG - AI deck] AiDeckPower: deck strength by Forge's card ratings.</summary>
    public enum AiDeckPower { Weak, Normal, Strong, Random }

    /// <summary>[MTG - AI opponent] AiPlayStyle: Forge's AI profiles (res/ai/&lt;name&gt;.ai) or one of them at random.</summary>
    public enum AiPlayStyle { Default, Cautious, Reckless, Experimental, Random }

    /// <summary>MTG mode entry point: reads the selected in-game deck, builds Forge decks and launches Forge.</summary>
    internal static class MtgMode
    {
        private static readonly Random Rng = new Random();

        public static bool Enabled => Plugin.MtgModeEnabled?.Value ?? true;

        /// <summary>[MTG] MtgDeckBuilder: MTG games use the active deck of our MTG deck builder instead of a vanilla deck.</summary>
        public static bool UseMtgDecks => Plugin.MtgDeckBuilder?.Value ?? false;

        /// <summary>The deck the player would sit down with (vanilla: <c>m_CurrentSelectedDeckIndex</c>), or null.</summary>
        public static DeckCompactCardDataList SelectedDeck
        {
            get
            {
                var decks = CPlayerData.m_DeckCompactCardDataList;
                int i = CPlayerData.m_CurrentSelectedDeckIndex;
                return decks != null && i >= 0 && i < decks.Count ? decks[i] : null;
            }
        }

        /// <summary>MTG cards (with counts) in a deck, plus the sets they come from.</summary>
        public static List<(MtgCard card, int count)> MtgCards(DeckCompactCardDataList deck, HashSet<CustomSet> sets, out int nonMtg)
        {
            var list = new List<(MtgCard, int)>();
            nonMtg = 0;
            if (deck?.compactCardDataAmountList == null) return list;
            foreach (var c in deck.compactCardDataAmountList)
            {
                if (c == null || c.amount <= 0) continue;
                var set = Registry.IsCustom(c.expansionType) ? Registry.Get(c.expansionType) : null;
                int pos = CardStore.CardPos(c.cardSaveIndex);
                var def = set != null && pos >= 0 && pos < set.Def.Cards.Count ? set.Card(pos) : null;
                var card = ToMtg(set?.Def, def);
                if (card == null) { nonMtg += c.amount; continue; }
                sets?.Add(set);
                list.Add((card, c.amount));
            }
            return list;
        }

        /// <summary>
        /// True when the table should offer MTG mode: deck builder on = a valid active MTG deck; off = the selected vanilla deck has
        /// MTG cards.
        /// </summary>
        public static bool SelectedDeckHasMtg() =>
            UseMtgDecks ? MtgDeckStore.ActiveDeck?.Valid == true : MtgCards(SelectedDeck, null, out _).Count > 0;

        /// <summary>Which in-game copies show the player's cards on the 3D table.</summary>
        public static MtgCardFaces Faces() =>
            UseMtgDecks ? new MtgCardFaces(MtgDeckStore.ActiveDeck) : new MtgCardFaces(SelectedDeck);

        public static MtgCard ToMtg(SetDef set, CardDef def)
        {
            if (set == null || def?.Mtg == null || string.IsNullOrWhiteSpace(def.Mtg.Name)) return null;
            return new MtgCard
            {
                Name = def.Mtg.Name.Trim(),
                SetCode = set.Mtg?.SetCode,
                TypeLine = def.Mtg.TypeLine ?? "",
                ManaCost = def.Mtg.ManaCost ?? "",
                Colors = def.Mtg.Colors ?? new List<string>(),
            };
        }

        /// <summary>
        /// Player deck (from the selected in-game deck) and an AI deck from the same sets; returns an error or null.
        /// <paramref name="aiDeck"/> = what the bridge needs to build the AI deck with Forge (style, set codes, card names);
        /// <paramref name="opponent"/> is our own simple deck, used when Forge can't (and in PlayInForgeWindow mode).
        /// </summary>
        public static string BuildDecks(out MtgDeck player, out MtgDeck opponent, out JObject aiDeck)
        {
            if (UseMtgDecks) return BuildFromMtgDeck(out player, out opponent, out aiDeck);
            player = opponent = null;
            aiDeck = null;
            var deck = SelectedDeck;
            var sets = new HashSet<CustomSet>();
            var cards = MtgCards(deck, sets, out int nonMtg);
            if (cards.Count == 0) return "This deck has no MTG cards";

            var pool = sets.SelectMany(s => s.Def.Cards.Select(c => ToMtg(s.Def, c))).Where(c => c != null).ToList();
            string name = ForgeLauncher.Safe(ForgeLauncher.DeckPrefix + (string.IsNullOrWhiteSpace(deck.deckName) ? "Deck" : deck.deckName));
            player = MtgDeckBuilder.FromPlayerDeck(name, cards, pool);
            if (nonMtg > 0) player.Notes.Add($"{nonMtg} non-MTG card(s) left out");
            var aiPool = AiPool(sets);
            opponent = MtgDeckBuilder.Opponent(ForgeLauncher.OpponentDeckName, aiPool, Rng);
            aiDeck = AiDeckSpec(aiPool);
            Plugin.Log.LogInfo($"MTG deck '{name}': {player.Total} cards from {string.Join(", ", sets.Select(s => s.Def.Id))}. " +
                               string.Join("; ", player.Notes));
            return opponent == null ? "Couldn't build an opponent deck from these sets" : null;
        }

        /// <summary>
        /// Player deck = the active MTG deck exactly as built (its cards + the basics the player chose; nothing added or dropped),
        /// AI deck from the sets it uses.
        /// </summary>
        private static string BuildFromMtgDeck(out MtgDeck player, out MtgDeck opponent, out JObject aiDeck)
        {
            player = opponent = null;
            aiDeck = null;
            var deck = MtgDeckStore.ActiveDeck;
            if (deck == null) return "No active MTG deck - build one at the workbench and set it active";
            if (!deck.Valid) return deck.Problems[0];
            var sets = new HashSet<CustomSet>();
            var cards = new List<(MtgCard, int)>();
            foreach (var e in deck.Entries.Where(e => e.Live && e.Save.Count > 0))
            {
                var card = ToMtg(e.Set.Def, e.Set.Card(e.Pos));
                if (card == null) continue;
                sets.Add(e.Set);
                cards.Add((card, e.Save.Count));
            }
            var pool = sets.SelectMany(s => s.Def.Cards.Select(c => ToMtg(s.Def, c))).Where(c => c != null).ToList();
            string name = ForgeLauncher.Safe(ForgeLauncher.DeckPrefix + (string.IsNullOrWhiteSpace(deck.Name) ? "Deck" : deck.Name));
            player = MtgDeckBuilder.FromMtgDeck(name, cards, deck.Save.Basics, pool);
            var aiPool = AiPool(sets);
            opponent = MtgDeckBuilder.Opponent(ForgeLauncher.OpponentDeckName, aiPool, Rng);
            aiDeck = AiDeckSpec(aiPool);
            Plugin.Log.LogInfo($"MTG deck '{name}' (MTG deck builder): {player.Total} cards from {string.Join(", ", sets.Select(s => s.Def.Id))}");
            return opponent == null ? "Couldn't build an opponent deck from these sets" : null;
        }

        /// <summary>MTG cards of the sets the customer plays (see <see cref="ChooseAiSets"/>).</summary>
        private static List<MtgCard> AiPool(HashSet<CustomSet> deckSets)
        {
            var chosen = ChooseAiSets(deckSets);
            Plugin.Log.LogInfo($"MTG AI sets: {string.Join(", ", chosen.Select(s => s.Def.Mtg?.SetCode ?? s.Def.Id))}");
            return chosen.SelectMany(s => s.Def.Cards.Select(c => ToMtg(s.Def, c))).Where(c => c != null).ToList();
        }

        private static bool HasMtg(CustomSet s) => s.Def.Cards.Any(c => ToMtg(s.Def, c) != null);

        /// <summary>A set counts as licensed when any restock row of one of its packs is unlocked in the shop.</summary>
        private static bool IsLicensed(CustomSet s)
        {
            var list = CPlayerData.m_IsItemLicenseUnlocked;
            return list != null && Registry.Packs.Any(p => p.Set == s && p.RestockRows.Any(i => i >= 0 && i < list.Count && list[i]));
        }

        /// <summary>
        /// [MTG - AI deck] AiDeckSets: MatchMyDeck = the deck's sets; RandomInstalled / RandomLicensed = AiDeckSetsMin..Max sets
        /// picked at random among installed MTG sets (licensed ones only, or all installed while none is licensed).
        /// </summary>
        private static List<CustomSet> ChooseAiSets(HashSet<CustomSet> deckSets)
        {
            var mode = Plugin.MtgAiDeckSets?.Value ?? AiDeckSets.RandomLicensed;
            if (mode == AiDeckSets.MatchMyDeck) return deckSets.ToList();
            var eligible = Registry.Sets.Where(HasMtg).ToList();
            if (mode == AiDeckSets.RandomLicensed)
            {
                var licensed = eligible.Where(IsLicensed).ToList();
                if (licensed.Count > 0) eligible = licensed;
                else Plugin.Log.LogInfo("MTG AI sets: no MTG set licensed yet - using every installed set");
            }
            if (eligible.Count == 0) return deckSets.ToList();
            int min = Plugin.MtgAiDeckSetsMin?.Value ?? 1, max = Plugin.MtgAiDeckSetsMax?.Value ?? 10;
            if (min > max) (min, max) = (max, min);
            int count = Math.Min(eligible.Count, Rng.Next(min, max + 1));
            return eligible.OrderBy(_ => Rng.Next()).Take(Math.Max(1, count)).ToList();
        }

        /// <summary>Deck power -1 (weak) .. 1 (strong): AiDeckPower, or rising with the shop level.</summary>
        private static double Power()
        {
            if (Plugin.MtgAiPowerFollowsShopLevel?.Value ?? true)
            {
                int full = Math.Max(1, Plugin.MtgAiFullPowerShopLevel?.Value ?? 35);
                return -1 + 2 * Math.Min(1.0, CPlayerData.m_ShopLevel / (double)full);
            }
            switch (Plugin.MtgAiDeckPower?.Value ?? AiDeckPower.Normal)
            {
                case AiDeckPower.Weak: return -1;
                case AiDeckPower.Strong: return 1;
                case AiDeckPower.Random: return Rng.NextDouble() * 2 - 1;
                default: return 0;
            }
        }

        /// <summary>Forge AI profile name for [MTG - AI opponent] AiPlayStyle.</summary>
        private static string Profile()
        {
            var style = Plugin.MtgAiPlayStyle?.Value ?? AiPlayStyle.Default;
            if (style == AiPlayStyle.Random) style = (AiPlayStyle)Rng.Next(0, (int)AiPlayStyle.Random);
            return style.ToString();
        }

        /// <summary>
        /// The bridge's <c>aiDeck</c>: style, size, boosters, power, play style, colours + per set its code and every card name
        /// (basics too: booster land slots), so Forge opens each set's boosters from that set's cards only. MtgSession adds
        /// <c>reuse</c> on a rematch.
        /// </summary>
        private static JObject AiDeckSpec(List<MtgCard> pool) => new JObject
        {
            ["style"] = (Plugin.MtgAiDeckStyle?.Value ?? AiDeckStyle.Random) == AiDeckStyle.Sealed ? "sealed" : "random",
            ["size"] = Plugin.MtgAiDeckSize?.Value ?? 60,
            ["boosters"] = Plugin.MtgAiSealedBoosters?.Value ?? 6,
            ["power"] = Power(),
            ["profile"] = Profile(),
            ["colors"] = ColorSpec(),
            ["pools"] = new JArray(pool.GroupBy(c => c.SetCode ?? "", StringComparer.OrdinalIgnoreCase).Select(g => new JObject
            {
                ["set"] = g.Key,
                ["names"] = new JArray(g.Select(c => c.Name).Distinct(StringComparer.OrdinalIgnoreCase)),
            })),
        };

        /// <summary>Random-deck colour count: <c>{min, max, weights[5]}</c> (weights for 1..5 colours; min ≤ max).</summary>
        private static JObject ColorSpec()
        {
            int min = Plugin.MtgAiDeckColorsMin?.Value ?? 1, max = Plugin.MtgAiDeckColorsMax?.Value ?? 5;
            if (min > max) (min, max) = (max, min);
            var w = Plugin.MtgAiDeckColorWeights.Select((e, i) => e?.Value ?? new[] { 15, 45, 25, 10, 5 }[i]);
            return new JObject { ["min"] = min, ["max"] = max, ["weights"] = new JArray(w) };
        }

        /// <summary>Builds both decks from the selected deck and starts Forge in its own window. Returns a message for the player.</summary>
        public static string PlaySelectedDeck()
        {
            string err = BuildDecks(out var player, out var opponent, out _);
            if (err != null) return err;
            string error = ForgeLauncher.Launch(player, opponent);
            if (error != null) return error;
            return $"Starting Forge - {player.Total}-card deck ready (loading takes a moment)";
        }
    }
}

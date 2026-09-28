using System;
using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Core;

namespace TCGCustomCards.Runtime.Mtg
{
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

        /// <summary>Player deck (from the selected in-game deck) and an AI deck from the same sets; returns an error or null.</summary>
        public static string BuildDecks(out MtgDeck player, out MtgDeck opponent)
        {
            if (UseMtgDecks) return BuildFromMtgDeck(out player, out opponent);
            player = opponent = null;
            var deck = SelectedDeck;
            var sets = new HashSet<CustomSet>();
            var cards = MtgCards(deck, sets, out int nonMtg);
            if (cards.Count == 0) return "This deck has no MTG cards";

            var pool = sets.SelectMany(s => s.Def.Cards.Select(c => ToMtg(s.Def, c))).Where(c => c != null).ToList();
            string name = ForgeLauncher.Safe(ForgeLauncher.DeckPrefix + (string.IsNullOrWhiteSpace(deck.deckName) ? "Deck" : deck.deckName));
            player = MtgDeckBuilder.FromPlayerDeck(name, cards, pool);
            if (nonMtg > 0) player.Notes.Add($"{nonMtg} non-MTG card(s) left out");
            opponent = MtgDeckBuilder.Opponent(ForgeLauncher.OpponentDeckName, pool, Rng);
            Plugin.Log.LogInfo($"MTG deck '{name}': {player.Total} cards from {string.Join(", ", sets.Select(s => s.Def.Id))}. " +
                               string.Join("; ", player.Notes));
            return opponent == null ? "Couldn't build an opponent deck from these sets" : null;
        }

        /// <summary>
        /// Player deck = the active MTG deck exactly as built (its cards + the basics the player chose; nothing added or dropped),
        /// AI deck from the sets it uses.
        /// </summary>
        private static string BuildFromMtgDeck(out MtgDeck player, out MtgDeck opponent)
        {
            player = opponent = null;
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
            opponent = MtgDeckBuilder.Opponent(ForgeLauncher.OpponentDeckName, pool, Rng);
            Plugin.Log.LogInfo($"MTG deck '{name}' (MTG deck builder): {player.Total} cards from {string.Join(", ", sets.Select(s => s.Def.Id))}");
            return opponent == null ? "Couldn't build an opponent deck from these sets" : null;
        }

        /// <summary>Builds both decks from the selected deck and starts Forge in its own window. Returns a message for the player.</summary>
        public static string PlaySelectedDeck()
        {
            string err = BuildDecks(out var player, out var opponent);
            if (err != null) return err;
            string error = ForgeLauncher.Launch(player, opponent);
            if (error != null) return error;
            return $"Starting Forge - {player.Total}-card deck ready (loading takes a moment)";
        }
    }
}

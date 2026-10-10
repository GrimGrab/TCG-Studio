using System.Collections;
using System.Linq;
using TCGCustomCards.Hooks;
using TCGCustomCards.Patches;
using TCGCustomCards.UI;

namespace TCGCustomCards.Runtime.Mtg
{
    /// <summary>
    /// MTG mode's <see cref="TableSession.OnSit"/>. Right-clicking a play table when the player has a deck with MTG cards asks
    /// which game to play; "Magic (Forge)" opens the deck picker (<see cref="MtgDeckPickerUI"/>), then runs the vanilla sit through
    /// <see cref="TableSession.Sit"/> with <see cref="MtgTableGame"/> (an MTG game against Forge's AI that ends in the vanilla
    /// win/lose flow); [MTG] PlayInForgeWindow instead exports the deck and opens Forge's own window (P0). "Tetramon" runs the
    /// vanilla sit. A missing Forge install or [MTG] MtgMode off = vanilla only.
    /// MTG tournaments (<see cref="MtgEvents"/>): the player's tournament table goes straight to the picker, which shows Forge's
    /// legality check against the event's sets; the vanilla sit does the tournament checks and StopTableGame records the round.
    /// Tetramon tournaments keep the Magic/Tetramon choice.
    /// </summary>
    internal static class MtgTableChoice
    {
        public static bool OnSit(InteractablePlayTable table)
        {
            if (ChoicePopup.IsOpen || MtgDeckPickerUI.IsOpen || MtgDraftUI.IsOpen) return true; // held right mouse button repeats the call
            if (!MtgMode.Enabled || !ForgeLauncher.IsInstalled) return false;

            if (MtgDraftGame.DevPending != null) // Debug TestDraft: the next table plays the drafted decks
            {
                TableSession.Sit(table, new MtgDraftGame(MtgDraftGame.DevPending, dev: true), sit => MtgVanillaDeckStandIn.Run(sit, always: true));
                return true;
            }

            if (MtgDraftEvent.PlayerInEvent)
            {
                if (!MtgEvents.TournamentTableOk(table)) return false; // vanilla says why (wrong table, round reported…)
                if (!MtgDraftEvent.PlayerDeckReady) { MtgDraftEvent.StartPlayerDeck(); return true; }
                var opponent = TableSession.WaitingCustomer(table);
                if (opponent == null) { Toast.Show("Your deck is ready - wait for your opponent to sit down"); return true; }
                var pair = MtgDraftEvent.PairAgainst(opponent);
                if (pair == null) { Toast.Show("This customer has no deck (Forge couldn't build it) - playing Tetramon"); return false; }
                TableSession.Sit(table, new MtgDraftGame(pair), sit => MtgVanillaDeckStandIn.Run(sit, always: true));
                return true;
            }

            if (MtgEvents.PlayerInMtgTournament)
            {
                if (!MtgEvents.TournamentTableOk(table)) return false; // vanilla says why (wrong table, round reported…)
                bool cmdr = MtgEvents.IsCommanderEvent;
                var decks = MtgMode.DeckChoices().Where(c => c.Commander == cmdr).ToList();
                if (decks.Count == 0)
                {
                    Toast.Show(cmdr ? "Commander tournament: build a Commander deck in the MTG deck builder ([MTG] MtgDeckBuilder) first"
                        : MtgMode.UseMtgDecks ? "Magic tournament: build an MTG deck (60+ cards) at the workbench first"
                        : "Magic tournament: you need a deck with MTG cards");
                    return true;
                }
                MtgDeckPickerUI.Open(cmdr ? "Choose your deck - Commander tournament" : "Choose your deck - Magic tournament",
                    decks, MtgEvents.EventSetCodes(), c => PlayMagic(table, c));
                return true;
            }

            var choices = MtgMode.DeckChoices();
            if (choices.Count == 0) return false;
            ChoicePopup.Show("Which game do you want to play?",
                "Magic (Forge)", () => MtgDeckPickerUI.Open("Choose your deck", MtgMode.DeckChoices(), null, c => PlayMagic(table, c)),
                "Tetramon", () => TableSession.SitVanilla(table));
            return true;
        }

        private static void PlayMagic(InteractablePlayTable table, MtgDeckChoice deck)
        {
            MtgMode.Select(deck);
            if (Plugin.MtgPlayInWindow.Value || !MtgSession.CanPlayInGame)
            {
                if (deck.Commander)
                {
                    Toast.Show("Commander games are played at the table: turn off [MTG] PlayInForgeWindow");
                    return;
                }
                if (MtgEvents.PlayerInMtgTournament)
                {
                    Toast.Show("Tournament rounds are played at the table: turn off [MTG] PlayInForgeWindow");
                    return;
                }
                Toast.Show(MtgMode.PlaySelectedDeck());
                return;
            }
            TableSession.Sit(table, MtgTableGame.Instance, MtgVanillaDeckStandIn.Run); // MTG deck builder decks aren't vanilla decks
        }
    }

    /// <summary>The constructed MTG match (<see cref="MtgSession"/>) as a table game.</summary>
    internal sealed class MtgTableGame : ITableGame
    {
        public static readonly MtgTableGame Instance = new MtgTableGame();

        public IEnumerator Run(PlayTableGame game, bool rematch)
        {
            MtgSession.Override = null; // constructed: decks from the selected deck
            return MtgSession.Run(game, rematch);
        }
        public bool Playing => MtgSession.Playing;

        public void Concede()
        {
            if (MtgSession.GameOver == null) ForgeBridge.Act("concede");
        }

        public void LeaveMidGame() => MtgSession.LeaveMidGame();
    }

    /// <summary>A match with drafted decks: the player's built deck vs one customer's Forge-drafted deck (faces = the drafted copies).</summary>
    internal sealed class MtgDraftGame : ITableGame
    {
        /// <summary>Debug TestDraft: decks waiting for the next table sit.</summary>
        public static MtgSession.DeckPair DevPending;

        private readonly MtgSession.DeckPair _pair;
        private readonly bool _dev;

        public MtgDraftGame(MtgSession.DeckPair pair, bool dev = false)
        {
            _pair = pair;
            _dev = dev;
        }

        public IEnumerator Run(PlayTableGame game, bool rematch)
        {
            MtgSession.Override = _pair;
            return MtgSession.Run(game, rematch);
        }

        public bool Playing => MtgSession.Playing;

        public void Concede()
        {
            if (MtgSession.GameOver == null) ForgeBridge.Act("concede");
        }

        public void LeaveMidGame()
        {
            MtgSession.LeaveMidGame();
            MtgSession.Override = null;
            if (_dev) DevPending = null;
        }
    }
}

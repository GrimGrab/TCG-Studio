using System.Collections;
using HarmonyLib;
using TCGCustomCards.Runtime.Mtg;
using TCGCustomCards.UI;

namespace TCGCustomCards.Patches
{
    /// <summary>
    /// MTG mode: right-clicking a play table with a deck that contains MTG cards asks which game to play.
    /// "Magic (Forge)" in-game mode runs the vanilla sit (customer, deck and seat checks) and <see cref="MtgDelayStart"/> swaps the
    /// Tetramon board for an MTG game against Forge's AI that ends in the vanilla win/lose flow. [MTG] PlayMode = ForgeWindow instead
    /// exports the deck and opens Forge's own window (P0). "Tetramon" re-runs the vanilla sit. A missing Forge install or
    /// [MTG] MtgMode off = vanilla only. Tournament days take the same path: the vanilla sit does the tournament checks (registered,
    /// right table, round not reported) and vanilla StopTableGame records the round result that ReportWinner set.
    /// </summary>
    [HarmonyPatch(typeof(InteractablePlayTable), nameof(InteractablePlayTable.OnRightMouseButtonUp))]
    internal static class MtgTableChoice
    {
        private static bool _vanilla;

        private static bool Prefix(InteractablePlayTable __instance)
        {
            if (_vanilla) return true;
            if (ChoicePopup.IsOpen) return false; // held right mouse button repeats the call
            if (!MtgMode.Enabled || !ForgeLauncher.IsInstalled) return true;
            if (!MtgMode.SelectedDeckHasMtg()) return true;

            var table = __instance;
            ChoicePopup.Show("Which game do you want to play with this deck?",
                "Magic (Forge)", () =>
                {
                    if (Plugin.MtgPlayInWindow.Value || !MtgSession.CanPlayInGame)
                    {
                        Toast.Show(MtgMode.PlaySelectedDeck());
                        return;
                    }
                    MtgSession.Pending = true;
                    MtgVanillaDeckStandIn.Run(() => SitVanilla(table)); // MTG deck builder decks aren't vanilla decks
                    if (!CSingleton<PlayCardGameManager>.Instance.m_PlayTableGame.IsPlayTableGameMode())
                        MtgSession.Pending = false; // vanilla refused (no opponent seated, deck incomplete…) and said why
                },
                "Tetramon", () => SitVanilla(table));
            return false;
        }

        private static void SitVanilla(InteractablePlayTable table)
        {
            _vanilla = true;
            try { table.OnRightMouseButtonUp(); }
            finally { _vanilla = false; }
        }
    }

    /// <summary>Replaces the Tetramon board setup with an MTG game while an MTG table session is on (first game and rematches).</summary>
    [HarmonyPatch(typeof(PlayTableGame), "DelayStart")]
    internal static class MtgDelayStart
    {
        private static bool Prefix(PlayTableGame __instance, ref IEnumerator __result)
        {
            if (!MtgSession.Pending && !MtgSession.InSession) return true;
            bool rematch = MtgSession.InSession && !MtgSession.Pending; // vanilla rematch with the same customer
            MtgSession.Pending = false;
            MtgSession.InSession = true;
            __result = MtgSession.Run(__instance, rematch);
            return false;
        }
    }

    /// <summary>
    /// Leaving the table ends the MTG session (the next sit asks again). Leaving mid-game (Esc → vanilla quit screen, which calls
    /// FinishLeaveGame directly) concedes in Forge and cleans up the MTG screen/cards.
    /// </summary>
    [HarmonyPatch(typeof(PlayTableGame), nameof(PlayTableGame.FinishLeaveGame))]
    internal static class MtgLeaveTable
    {
        private static void Prefix()
        {
            MtgSession.LeaveMidGame();
            MtgSession.InSession = false;
        }
    }

    /// <summary>ConfirmQuitGame (a vanilla "forfeit" that reports a loss) concedes the MTG game instead; Forge's game over then reports it.</summary>
    [HarmonyPatch(typeof(PlayTableGame), nameof(PlayTableGame.ConfirmQuitGame))]
    internal static class MtgQuitBattle
    {
        private static bool Prefix()
        {
            if (!MtgSession.Playing) return true;
            if (MtgSession.GameOver == null) ForgeBridge.Act("concede");
            return false;
        }
    }
}

using System;
using System.Collections;
using System.Linq;
using HarmonyLib;

namespace TCGCustomCards.Hooks
{
    /// <summary>A game of ours played inside a vanilla play-table session instead of the Tetramon duel.</summary>
    internal interface ITableGame
    {
        /// <summary>
        /// Replaces PlayTableGame.DelayStart (first game and every vanilla rematch). Must end with
        /// <c>game.ReportWinner(won, draw)</c> (vanilla win/lose screen → rematch/leave) or <c>game.FinishLeaveGame(false)</c> (abort).
        /// </summary>
        IEnumerator Run(PlayTableGame game, bool rematch);
        /// <summary>A game is running (Esc's "forfeit" goes to <see cref="Concede"/> instead of vanilla's ConfirmQuitGame).</summary>
        bool Playing { get; }
        void Concede();
        /// <summary>The player left the table (vanilla quit screen calls FinishLeaveGame directly): clean up, no result.</summary>
        void LeaveMidGame();
    }

    /// <summary>
    /// Single owner of the play-table session patches (InteractablePlayTable.OnRightMouseButtonUp, PlayTableGame.DelayStart /
    /// FinishLeaveGame / ConfirmQuitGame). <see cref="OnSit"/> decides what a right-click on a table does; <see cref="Sit"/> runs
    /// the vanilla sit (customer, seat, deck and tournament checks, camera) and plays an <see cref="ITableGame"/> in place of the
    /// Tetramon board. Tournament rounds are recorded by vanilla StopTableGame from the ReportWinner result. See docs/hooks.md.
    /// </summary>
    internal static class TableSession
    {
        /// <summary>Asked first when the player right-clicks a play table; true = handled (the vanilla sit is skipped).</summary>
        public static Func<InteractablePlayTable, bool> OnSit;

        private static ITableGame _game;
        private static bool _pending; // set between Sit and the DelayStart it triggers
        private static bool _vanilla; // re-entrant vanilla call from SitVanilla

        /// <summary>A session of ours is on (from Sit until the player leaves the table).</summary>
        public static bool Active => _game != null;

        /// <summary>
        /// Runs the vanilla sit and plays <paramref name="game"/> there. <paramref name="wrap"/> may wrap the synchronous vanilla
        /// call (e.g. a stand-in deck for its 50-card check). False = vanilla refused (it already said why).
        /// </summary>
        public static bool Sit(InteractablePlayTable table, ITableGame game, Action<Action> wrap = null)
        {
            _game = game;
            _pending = true;
            if (wrap != null) wrap(() => SitVanilla(table));
            else SitVanilla(table);
            if (CSingleton<PlayCardGameManager>.Instance.m_PlayTableGame.IsPlayTableGameMode()) return true;
            _game = null; // no opponent seated, deck incomplete, wrong tournament table…
            _pending = false;
            return false;
        }

        private static readonly AccessTools.FieldRef<InteractablePlayTable, System.Collections.Generic.List<Customer>> Occupied =
            AccessTools.FieldRefAccess<InteractablePlayTable, System.Collections.Generic.List<Customer>>("m_OccupiedCustomer");

        /// <summary>The customer seated at <paramref name="table"/> waiting for an opponent (the player's opponent), or null.</summary>
        public static Customer WaitingCustomer(InteractablePlayTable table)
        {
            var seats = Occupied(table);
            return seats?.FirstOrDefault(c => c != null);
        }

        /// <summary>The unmodified vanilla right-click (Tetramon duel, or the sit that <see cref="Sit"/> takes over).</summary>
        public static void SitVanilla(InteractablePlayTable table)
        {
            _vanilla = true;
            try { table.OnRightMouseButtonUp(); }
            finally { _vanilla = false; }
        }

        [HarmonyPatch(typeof(InteractablePlayTable), nameof(InteractablePlayTable.OnRightMouseButtonUp))]
        private static class SitPatch
        {
            private static bool Prefix(InteractablePlayTable __instance)
            {
                if (_vanilla || OnSit == null) return true;
                return !OnSit(__instance);
            }
        }

        [HarmonyPatch(typeof(PlayTableGame), "DelayStart")]
        private static class DelayStartPatch
        {
            private static bool Prefix(PlayTableGame __instance, ref IEnumerator __result)
            {
                if (_game == null) return true;
                bool rematch = !_pending; // vanilla rematch with the same customer
                _pending = false;
                __result = _game.Run(__instance, rematch);
                return false;
            }
        }

        /// <summary>Leaving the table ends the session (the next sit asks again); mid-game it cleans up first.</summary>
        [HarmonyPatch(typeof(PlayTableGame), nameof(PlayTableGame.FinishLeaveGame))]
        private static class LeavePatch
        {
            private static void Prefix()
            {
                var g = _game;
                _game = null;
                _pending = false;
                g?.LeaveMidGame();
            }
        }

        /// <summary>ConfirmQuitGame (vanilla forfeit = a loss) concedes our game instead; the game's own end reports it.</summary>
        [HarmonyPatch(typeof(PlayTableGame), nameof(PlayTableGame.ConfirmQuitGame))]
        private static class QuitPatch
        {
            private static bool Prefix()
            {
                if (_game == null || !_game.Playing) return true;
                _game.Concede();
                return false;
            }
        }
    }
}

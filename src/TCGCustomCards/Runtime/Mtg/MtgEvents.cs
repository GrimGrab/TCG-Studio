using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Hooks;

namespace TCGCustomCards.Runtime.Mtg
{
    /// <summary>
    /// MTG tournament formats on the hooks layer (<see cref="ShopEvents"/>). Every MTG format carries the event's set list
    /// (1..n installed sets, host screen); decks are checked against it by Forge (<see cref="MtgDeckCheck"/>), never by our code.
    /// Constructed = vanilla tournament flow (Swiss, tables, prizes) with MTG matches at the player's table.
    /// </summary>
    internal static class MtgEvents
    {
        public const string ConstructedId = "mtg-constructed";
        public const string CommanderId = "mtg-commander";

        public static void Register()
        {
            ShopEvents.Register(new ShopEventFormat
            {
                Id = ConstructedId,
                Name = "Magic: Constructed",
                Description = "Your own 60-card deck, only cards from the chosen sets (checked by Forge).",
                Available = () => MtgMode.Enabled && MtgSession.CanPlayInGame,
                Unavailable = "Needs MTG mode: TCG Studio > Setup > Install MTG mode, and [MTG] MtgMode on.",
                SetEligible = IsMtgSet,
            });
            ShopEvents.Register(new ShopEventFormat
            {
                Id = CommanderId,
                Name = "Magic: Commander",
                Description = "Your Commander deck (commander + 99), only cards from the chosen sets (checked by Forge). Customers bring Forge-built Commander decks from those sets.",
                Available = () => MtgMode.Enabled && MtgSession.CanPlayInGame,
                Unavailable = "Needs MTG mode: TCG Studio > Setup > Install MTG mode, and [MTG] MtgMode on.",
                SetEligible = IsMtgSet,
            });
        }

        /// <summary>Installed set with MTG cards and a Forge set code.</summary>
        public static bool IsMtgSet(CustomSet s) => !string.IsNullOrWhiteSpace(s.Def.Mtg?.SetCode) && MtgMode.HasMtg(s);

        /// <summary>The hosted event is an MTG format (constructed now; draft/sealed later).</summary>
        public static bool IsMtgEvent => ShopEvents.Is(ConstructedId) || ShopEvents.Is(CommanderId);

        /// <summary>The hosted MTG tournament is Commander (players use Commander decks).</summary>
        public static bool IsCommanderEvent => ShopEvents.Is(CommanderId);

        /// <summary>Today's MTG tournament is running and the player plays in it.</summary>
        public static bool PlayerInMtgTournament =>
            IsMtgEvent && ShopEvents.EventRunning && CPlayerData.m_IsPlayerRegisteredForTournament;

        public static List<CustomSet> EventSets() =>
            (ShopEvents.Config?.sets ?? new List<string>())
            .Select(id => Registry.Sets.FirstOrDefault(s => s.Def.Id == id)).Where(s => s != null).ToList();

        /// <summary>Forge set codes of the event's sets (what Forge's tournament format allows).</summary>
        public static List<string> EventSetCodes() =>
            EventSets().Select(s => s.Def.Mtg?.SetCode).Where(c => !string.IsNullOrWhiteSpace(c))
                .Select(c => c.Trim().ToUpperInvariant()).Distinct().ToList();

        /// <summary>The customer's sets in the player's tournament match (null = not a tournament match: settings decide).</summary>
        public static List<CustomSet> TournamentAiSets()
        {
            if (!PlayerInMtgTournament) return null;
            var sets = EventSets();
            return sets.Count > 0 ? sets : null;
        }

        /// <summary>
        /// Would the vanilla sit accept this table for the player's round (registered, round not reported, right table number)?
        /// Display gating only, so the deck picker doesn't open where vanilla will refuse; vanilla still checks and says why.
        /// </summary>
        public static bool TournamentTableOk(InteractablePlayTable table) =>
            CPlayerData.m_IsPlayerRegisteredForTournament &&
            !CPlayerData.m_PlayerTournamentData.m_HasRegisteredTournamentResult &&
            CPlayerData.m_PlayerTournamentData.m_TournamentCustomerPlayTableIndex == table.GetTournamentPlayTableNumber();
    }
}

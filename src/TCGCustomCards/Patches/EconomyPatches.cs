using System;
using HarmonyLib;
using TCGCustomCards.Runtime;

namespace TCGCustomCards.Patches
{
    // Layer 4 — economy. Base card prices come from the set definition (Registry.ApplyGeneratedPrices).
    // Daily drift: vanilla only moves the game-event expansion (+Destiny/Ghost/Ascension); custom sets get the same
    // treatment through the game's own private routines, so the rules (rarity/border/element matching, crashes) stay identical.

    [HarmonyPatch(typeof(PriceChangeManager), "EvaluatePriceChange")]
    internal static class CustomCardPriceDrift
    {
        private static readonly Func<PriceChangeManager, EPriceChangeType, EPriceChangeType> RandomType =
            AccessTools.MethodDelegate<Func<PriceChangeManager, EPriceChangeType, EPriceChangeType>>(
                AccessTools.Method(typeof(PriceChangeManager), "GetRandomPriceChangeType"));

        internal static readonly Action<PriceChangeManager, ECardExpansionType, EPriceChangeType, bool> UpdateCardMarketPrice =
            AccessTools.MethodDelegate<Action<PriceChangeManager, ECardExpansionType, EPriceChangeType, bool>>(
                AccessTools.Method(typeof(PriceChangeManager), "UpdateCardMarketPrice"));

        private static void Postfix(PriceChangeManager __instance)
        {
            if (Registry.Sets.Count == 0) return;
            var evt = InventoryBase.GetGameEventData(CPlayerData.m_GameEventFormat);
            var up = RandomType(__instance, evt.positivePriceChangeType);
            var down = RandomType(__instance, evt.negativePriceChangeType);
            if (up == down) return; // vanilla also skips the day's card changes in this case
            foreach (var set in Registry.Sets)
            {
                UpdateCardMarketPrice(__instance, set.Expansion, up, true);
                UpdateCardMarketPrice(__instance, set.Expansion, down, false);
            }
        }
    }

    [HarmonyPatch(typeof(PriceChangeManager), "EvaluatePriceCrash")]
    internal static class CustomCardPriceCrash
    {
        private static readonly Action<PriceChangeManager, ECardExpansionType> EvaluateCardPriceCrash =
            AccessTools.MethodDelegate<Action<PriceChangeManager, ECardExpansionType>>(
                AccessTools.Method(typeof(PriceChangeManager), "EvaluateCardPriceCrash"));

        private static void Postfix(PriceChangeManager __instance)
        {
            foreach (var set in Registry.Sets) EvaluateCardPriceCrash(__instance, set.Expansion);
        }
    }
}

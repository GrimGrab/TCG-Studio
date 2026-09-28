using System;
using System.Collections.Generic;
using HarmonyLib;
using TCGCustomCards.Runtime;

namespace TCGCustomCards.Patches
{
    // Layer 2 — storage & price accessors. CPlayerData switches over the 8 vanilla expansions in each of these;
    // for custom expansions we answer from the set's CardStore instead. Vanilla lists are never touched.

    internal static class Store
    {
        public static CardStore For(ECardExpansionType e) => Registry.Get(e)?.Store;

        /// <summary>Resolves a custom card to (store, slot). False if the card/set is unknown or out of range.</summary>
        public static bool Resolve(CardData c, out CardStore store, out int slot)
        {
            store = For(c.expansionType);
            slot = CPlayerData.GetCardSaveIndex(c);
            return store != null && store.InRange(slot);
        }

        public static bool Resolve(ECardExpansionType e, int index, out CardStore store)
        {
            store = For(e);
            return store != null && store.InRange(index);
        }

        /// <summary>Vanilla rounding for displayed card prices.</summary>
        public static float RoundPrice(float v)
        {
            double r = Math.Round(v, 2, MidpointRounding.AwayFromZero);
            if (GameInstance.GetCurrencyConversionRate() > 1f) r = Math.Round(v, 3, MidpointRounding.AwayFromZero);
            return (float)r;
        }

        public static float PlayerPrice(CardStore s, int slot, int grade) =>
            RoundPrice(grade <= 0 ? s.PriceSet[slot] : s.GradedPriceSet[slot].floatDataList[Math.Min(grade, 10) - 1]);
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.GetCardCollectedList))]
    internal static class GetCardCollectedListPatch
    {
        private static bool Prefix(ECardExpansionType expansionType, ref List<int> __result)
        {
            if (!Registry.IsCustom(expansionType)) return true;
            __result = Store.For(expansionType)?.Counts ?? new List<int>();
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.GetIsCardCollectedList))]
    internal static class GetIsCardCollectedListPatch
    {
        private static bool Prefix(ECardExpansionType expansionType, ref List<bool> __result)
        {
            if (!Registry.IsCustom(expansionType)) return true;
            __result = Store.For(expansionType)?.Collected ?? new List<bool>();
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.AddCard))]
    internal static class AddCardPatch
    {
        private static bool Prefix(CardData cardData, int addAmount)
        {
            if (!Registry.IsCustom(cardData.expansionType)) return true;
            if (cardData.cardGrade > 0) return true; // graded cards go to m_GradedCardInventoryList (stripped from the vanilla save in M4)
            if (Store.Resolve(cardData, out var s, out int slot))
            {
                s.Counts[slot] += addAmount;
                s.Collected[slot] = true;
            }
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.ReduceCard))]
    internal static class ReduceCardPatch
    {
        private static bool Prefix(CardData cardData, int reduceAmount)
        {
            if (!Registry.IsCustom(cardData.expansionType)) return true;
            if (Store.Resolve(cardData, out var s, out int slot)) s.Counts[slot] -= reduceAmount;
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.ReduceCardUsingIndex))]
    internal static class ReduceCardUsingIndexPatch
    {
        private static bool Prefix(int index, ECardExpansionType expansionType, int reduceAmount)
        {
            if (!Registry.IsCustom(expansionType)) return true;
            if (Store.Resolve(expansionType, index, out var s)) s.Counts[index] -= reduceAmount;
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.SetCard))]
    internal static class SetCardPatch
    {
        private static bool Prefix(CardData cardData, int amount)
        {
            if (!Registry.IsCustom(cardData.expansionType)) return true;
            if (Store.Resolve(cardData, out var s, out int slot)) s.Counts[slot] = amount;
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.GetCardPrice))]
    internal static class GetCardPricePatch
    {
        private static bool Prefix(CardData cardData, ref float __result)
        {
            if (!Registry.IsCustom(cardData.expansionType)) return true;
            __result = Store.Resolve(cardData, out var s, out int slot) ? Store.PlayerPrice(s, slot, cardData.cardGrade) : 0f;
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.GetCardPriceUsingIndex))]
    internal static class GetCardPriceUsingIndexPatch
    {
        private static bool Prefix(int index, ECardExpansionType expansionType, int cardGrade, ref float __result)
        {
            if (!Registry.IsCustom(expansionType)) return true;
            __result = Store.Resolve(expansionType, index, out var s) ? Store.PlayerPrice(s, index, cardGrade) : 0f;
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.SetCardPrice))]
    internal static class SetCardPricePatch
    {
        private static bool Prefix(CardData cardData, float priceSet)
        {
            if (!Registry.IsCustom(cardData.expansionType)) return true;
            if (Store.Resolve(cardData, out var s, out int slot))
            {
                if (cardData.cardGrade > 0) s.GradedPriceSet[slot].floatDataList[Math.Min(cardData.cardGrade, 10) - 1] = priceSet;
                else s.PriceSet[slot] = priceSet;
            }
            CEventManager.QueueEvent(new CEventPlayer_CardPriceChanged(cardData, priceSet)); // price tags listen for this
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.SetCardGeneratedMarketPrice))]
    internal static class SetCardGeneratedMarketPricePatch
    {
        private static bool Prefix(int cardIndex, ECardExpansionType expansionType, float price)
        {
            if (!Registry.IsCustom(expansionType)) return true;
            if (Store.Resolve(expansionType, cardIndex, out var s)) s.Market[cardIndex].generatedMarketPrice = price;
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.AddCardPricePercentChange))]
    internal static class AddCardPricePercentChangePatch
    {
        private static bool Prefix(int cardIndex, ECardExpansionType expansionType, float percentChange)
        {
            if (!Registry.IsCustom(expansionType)) return true;
            if (Store.Resolve(expansionType, cardIndex, out var s))
            {
                var m = s.Market[cardIndex];
                m.pricePercentChangeList = UnityEngine.Mathf.Clamp(m.pricePercentChangeList + percentChange, -80f, 200f);
            }
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.GetCardPricePercentChange))]
    internal static class GetCardPricePercentChangePatch
    {
        private static bool Prefix(int cardIndex, ECardExpansionType expansionType, ref float __result)
        {
            if (!Registry.IsCustom(expansionType)) return true;
            __result = Store.Resolve(expansionType, cardIndex, out var s) ? s.Market[cardIndex].pricePercentChangeList : 0f;
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.GetPastCardPricePercentChange))]
    internal static class GetPastCardPricePercentChangePatch
    {
        private static bool Prefix(int cardIndex, ECardExpansionType expansionType, ref List<float> __result)
        {
            if (!Registry.IsCustom(expansionType)) return true;
            if (!Store.Resolve(expansionType, cardIndex, out var s))
            {
                __result = new List<float> { 0f };
                return false;
            }
            var m = s.Market[cardIndex];
            if (m.pastPricePercentChangeList == null) m.pastPricePercentChangeList = new List<float>();
            // Price graph reads list[Count-1]; never hand back an empty list.
            if (m.pastPricePercentChangeList.Count == 0) m.pastPricePercentChangeList.Add(m.pricePercentChangeList);
            __result = m.pastPricePercentChangeList;
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.UpdatePastCardPricePercentChange))]
    internal static class UpdatePastCardPricePercentChangePatch
    {
        private static void Postfix()
        {
            foreach (var set in Registry.Sets)
                foreach (var m in set.Store.Market)
                {
                    if (m.pastPricePercentChangeList == null) m.pastPricePercentChangeList = new List<float>();
                    m.pastPricePercentChangeList.Add(m.pricePercentChangeList);
                    if (m.pastPricePercentChangeList.Count > CardStore.MaxHistory) m.pastPricePercentChangeList.RemoveAt(0);
                }
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.GetCardMarketPrice), typeof(CardData))]
    internal static class GetCardMarketPriceDataPatch
    {
        private static bool Prefix(CardData cardData, ref float __result)
        {
            if (!Registry.IsCustom(cardData.expansionType)) return true;
            __result = Store.Resolve(cardData, out var s, out int slot) ? s.Market[slot].GetMarketPrice(slot, cardData.cardGrade) : 0f;
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.GetCardMarketPrice), typeof(int), typeof(ECardExpansionType), typeof(bool), typeof(int))]
    internal static class GetCardMarketPriceIndexPatch
    {
        private static bool Prefix(int index, ECardExpansionType expansionType, int cardGrade, ref float __result)
        {
            if (!Registry.IsCustom(expansionType)) return true;
            __result = Store.Resolve(expansionType, index, out var s) ? s.Market[index].GetMarketPrice(index, cardGrade) : 0f;
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.GetCardMarketPriceCustomPercent))]
    internal static class GetCardMarketPriceCustomPercentPatch
    {
        private static bool Prefix(int index, ECardExpansionType expansionType, float percent, int cardGrade, ref float __result)
        {
            if (!Registry.IsCustom(expansionType)) return true;
            __result = Store.Resolve(expansionType, index, out var s) ? s.Market[index].GetMarketPriceCustomPercent(percent, index, cardGrade) : 0f;
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.GetTotalCardCollectedAmount))]
    internal static class GetTotalCardCollectedAmountPatch
    {
        private static void Postfix(ref int __result)
        {
            foreach (var set in Registry.Sets) __result += CPlayerData.GetCardCollectedAmount(set.Expansion, isDimensionCard: false);
        }
    }
}

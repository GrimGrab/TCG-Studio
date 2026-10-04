using HarmonyLib;
using TCGCustomCards.UI;

namespace TCGCustomCards.Patches
{
    // Phone list screens (check price, checkout) come with scene-authored UI pools sized for vanilla's item count; grown with UiPool.

    /// <summary>Check Price item tabs stop at the scene's panel count (CheckPriceScreen.cs:126).</summary>
    [HarmonyPatch(typeof(CheckPriceScreen), "EvaluateItemPanelUI")]
    internal static class CheckPriceItemPoolPatch
    {
        private static void Prefix(CheckPriceScreen __instance)
        {
            var so = CSingleton<InventoryBase>.Instance.m_StockItemData_SO;
            int rows = so.m_ShownItemType.Count + so.m_ShownAccessoryItemType.Count + so.m_ShownFigurineItemType.Count + so.m_ShownBoardGameItemType.Count;
            UiPool.Grow(__instance.m_CheckPricePanelUIList, rows, "Check Price items");
        }
    }

    /// <summary>
    /// The cart holds as many different products as the checkout has bars (RestockItemScreen.cs:305). Grow the bars to one per
    /// restock row; RestockItemCheckoutScreen.UpdateData re-inits every bar before use.
    /// </summary>
    [HarmonyPatch(typeof(RestockItemScreen), nameof(RestockItemScreen.HasEnoughCartSlot))]
    internal static class RestockCartSlotsPatch
    {
        private static void Prefix(RestockItemScreen __instance)
        {
            var checkout = __instance.m_RestockItemCheckoutScreen;
            if (checkout == null) return;
            int rows = CSingleton<InventoryBase>.Instance.m_StockItemData_SO.m_RestockDataList.Count;
            UiPool.Grow(checkout.m_RestockCheckoutItemBarUIList, rows, "Restock checkout bars");
        }
    }
}

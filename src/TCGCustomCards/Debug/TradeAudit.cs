using HarmonyLib;

namespace TCGCustomCards.Debug
{
    /// <summary>
    /// Logs every accepted customer card trade/sale (CustomerTradeCardScreen.OnPressAccept): which card, grade, and the player's
    /// count of it before/after — and warns when a purchase didn't add the card. Investigating a report (2026-09-25) that a
    /// bought card sometimes doesn't arrive.
    /// </summary>
    [HarmonyPatch(typeof(CustomerTradeCardScreen), nameof(CustomerTradeCardScreen.OnPressAccept))]
    internal static class TradeAudit
    {
        private static readonly AccessTools.FieldRef<CustomerTradeCardScreen, bool> HasAccepted =
            AccessTools.FieldRefAccess<CustomerTradeCardScreen, bool>("m_HasAccepted");
        private static readonly AccessTools.FieldRef<CustomerTradeCardScreen, CardData> CardL =
            AccessTools.FieldRefAccess<CustomerTradeCardScreen, CardData>("m_CardData_L");

        private struct Before
        {
            public bool Accepted;
            public int Amount;
            public int Graded;
            public double Coins;
        }

        private static void Prefix(CustomerTradeCardScreen __instance, out Before __state)
        {
            __state = new Before { Accepted = HasAccepted(__instance), Graded = CPlayerData.m_GradedCardInventoryList?.Count ?? 0, Coins = CPlayerData.m_CoinAmountDouble };
            var card = CardL(__instance);
            try { __state.Amount = card != null && card.cardGrade <= 0 ? CPlayerData.GetCardAmount(card) : 0; } catch { __state.Amount = -1; }
        }

        private static void Postfix(CustomerTradeCardScreen __instance, Before __state)
        {
            if (__state.Accepted || !HasAccepted(__instance)) return; // only the press that completed the deal
            var card = CardL(__instance);
            if (card == null) { Plugin.Log.LogWarning("Trade: accepted but the card is null"); return; }
            int after;
            try { after = card.cardGrade <= 0 ? CPlayerData.GetCardAmount(card) : CPlayerData.m_GradedCardInventoryList.Count; }
            catch (System.Exception e) { Plugin.Log.LogWarning($"Trade: couldn't read the card amount: {e.Message}"); return; }
            int before = card.cardGrade <= 0 ? __state.Amount : __state.Graded;
            string name;
            try { name = InventoryBase.GetMonsterData(card.monsterType)?.GetName(); } catch { name = "?"; }
            string what = $"{name} [{card.expansionType}/{(int)card.monsterType}] {card.borderType}{(card.isFoil ? " foil" : "")}" +
                          $"{(card.cardGrade > 0 ? $" graded {card.cardGrade}" : "")} slot={CPlayerData.GetCardSaveIndex(card)}";
            double paid = __state.Coins - CPlayerData.m_CoinAmountDouble; // coins drop next frame via event, so usually 0 here
            if (after > before) Plugin.Log.LogInfo($"Trade: got {what} (count {before} → {after})");
            else Plugin.Log.LogWarning($"Trade: accepted but the card was NOT added — {what} (count {before} → {after}, coins Δ {paid:0.##})");
        }
    }
}

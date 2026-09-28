using System.Collections.Generic;
using HarmonyLib;
using TCGCustomCards.Runtime.Mtg;
using TCGCustomCards.UI;

namespace TCGCustomCards.Patches
{
    /// <summary>
    /// [MTG] MtgDeckBuilder on: the workbench's "Edit deck" opens our MTG deck builder instead of the vanilla deck list
    /// (PlayCardGameManager.OpenDeckListScreen is the only way in, from WorkbenchUIScreen.OnPressEditDeckButton).
    /// </summary>
    [HarmonyPatch(typeof(PlayCardGameManager), nameof(PlayCardGameManager.OpenDeckListScreen))]
    internal static class MtgDeckBuilderOpen
    {
        private static bool Prefix()
        {
            if (!MtgMode.Enabled || !MtgMode.UseMtgDecks) return true;
            CSingleton<PlayCardGameManager>.Instance.m_WorkbenchUIScreen.m_ScreenGrp.SetActive(false); // like vanilla
            MtgDeckBuilderUI.Open();
            return false;
        }
    }

    /// <summary>
    /// Sitting down for an MTG game with an MTG deck-builder deck: the vanilla sit (InteractablePlayTable.OnRightMouseButtonUp :168,
    /// PlayTableGame.SetPlayTable :209) refuses unless the selected *vanilla* deck has 50 cards. For the duration of that call only,
    /// show it a stand-in "complete" deck; the real MTG deck is read by MtgSession. Restored immediately afterwards.
    /// </summary>
    internal static class MtgVanillaDeckStandIn
    {
        public static void Run(System.Action sit)
        {
            if (!MtgMode.UseMtgDecks) { sit(); return; }
            var decks = CPlayerData.m_DeckCompactCardDataList;
            int index = CPlayerData.m_CurrentSelectedDeckIndex;
            var standIn = new DeckCompactCardDataList { deckName = "MTG" };
            standIn.compactCardDataAmountList.Add(new CompactCardDataAmount { amount = GameInstance.GetMaxDeckCardCount() });
            // Keep the vanilla deck's deck box/playmat for the table visuals when there is one.
            if (decks != null && index >= 0 && index < decks.Count)
            {
                standIn.deckBoxIndex = decks[index].deckBoxIndex;
                standIn.playmatIndex = decks[index].playmatIndex;
            }
            CPlayerData.m_DeckCompactCardDataList = new List<DeckCompactCardDataList> { standIn };
            CPlayerData.m_CurrentSelectedDeckIndex = 0;
            try { sit(); }
            finally
            {
                CPlayerData.m_DeckCompactCardDataList = decks;
                CPlayerData.m_CurrentSelectedDeckIndex = index;
            }
        }
    }
}

using BepInEx.Configuration;
using TCGCustomCards.Runtime;
using UnityEngine;
using UnityEngine.SceneManagement;

namespace TCGCustomCards.Debug
{
    /// <summary>Own DontDestroyOnLoad runner (BepInEx's manager object is hidden by this install's config).</summary>
    internal class DevTools : MonoBehaviour
    {
        internal static ConfigEntry<KeyboardShortcut> GrantKey;
        internal static ConfigEntry<KeyboardShortcut> GrantAllVariantsKey;
        internal static ConfigEntry<KeyboardShortcut> TestDraftKey;
        internal static ConfigEntry<KeyboardShortcut> ShopLevelKey;
        internal static ConfigEntry<KeyboardShortcut> MoneyKey;

        private const int DebugShopLevel = 100;   // as the game shows it (m_ShopLevel + 1)
        private const float DebugMoney = 1000000f;

        public static void Init(ConfigFile config)
        {
            GrantKey = config.Bind("Debug", "GrantTestCards", new KeyboardShortcut(KeyCode.F9),
                "Adds one Base copy and one random variant of every custom card to the collection.");
            GrantAllVariantsKey = config.Bind("Debug", "GrantAllVariants", new KeyboardShortcut(KeyCode.F9, KeyCode.LeftShift),
                "Adds one copy of every border/foil variant of every custom card.");
            TestDraftKey = config.Bind("Debug", "TestDraft", new KeyboardShortcut(KeyCode.F10, KeyCode.LeftShift),
                "MTG booster draft test without a tournament: drafts the MTG packs in your hands (or 3 test packs) against 7 Forge seats, then the next table plays your deck. Nothing is used up; your picks go into the collection.");
            ShopLevelKey = config.Bind("Debug", "SetShopLevel100", new KeyboardShortcut(KeyCode.F7, KeyCode.LeftShift),
                "Sets the shop to level 100 (no level-up popup and no Steam achievements). Saved with the game like a normal level.");
            MoneyKey = config.Bind("Debug", "AddMillion", new KeyboardShortcut(KeyCode.F8, KeyCode.LeftShift),
                "Adds $1,000,000 to your money.");

            var go = new GameObject("TCGCustomCards_Runner");
            DontDestroyOnLoad(go);
            go.hideFlags = HideFlags.HideAndDontSave;
            go.AddComponent<DevTools>();
        }

        private void Update()
        {
            if (GrantAllVariantsKey.Value.IsDown()) Grant(allVariants: true);
            else if (GrantKey.Value.IsDown()) Grant(allVariants: false);
            else if (TestDraftKey.Value.IsDown()) DraftTest.Run();
            else if (ShopLevelKey.Value.IsDown()) SetShopLevel();
            else if (MoneyKey.Value.IsDown()) AddMoney();
        }

        private static bool InShop(string what)
        {
            if (SceneManager.GetActiveScene().name == "Start" && GameInstance.m_FinishedSavefileLoading) return true;
            Plugin.Log.LogInfo(what + " ignored: load a save first");
            return false;
        }

        /// <summary>
        /// Sets the level directly instead of adding XP: the game's level-up event would unlock the Steam shop-level
        /// achievements. SetShopExp(0) refreshes the HUD without levelling up; the customer cap follows like after a level-up.
        /// </summary>
        private static void SetShopLevel()
        {
            if (!InShop("Shop level")) return;
            CPlayerData.m_ShopLevel = DebugShopLevel - 1;
            CPlayerData.m_ShopExpPoint = 0;
            CEventManager.QueueEvent(new CEventPlayer_SetShopExp(0));
            var customers = CSingleton<CustomerManager>.Instance;
            if (customers != null) HarmonyLib.AccessTools.Method(typeof(CustomerManager), "EvaluateMaxCustomerCount")?.Invoke(customers, null);
            Plugin.Log.LogInfo($"Shop level set to {DebugShopLevel}");
        }

        private static void AddMoney()
        {
            if (!InShop("Add money")) return;
            CEventManager.QueueEvent(new CEventPlayer_AddCoin(DebugMoney));
            Plugin.Log.LogInfo($"Added {DebugMoney:N0} money");
        }

        private static void Grant(bool allVariants)
        {
            if (!InShop("Grant")) return;
            int added = 0;
            foreach (var set in Registry.Sets)
                for (int pos = 0; pos < set.Def.Cards.Count; pos++)
                {
                    if (allVariants)
                    {
                        for (int b = 0; b < CardStore.BordersPerCard; b++)
                        {
                            Add(set, pos, (ECardBorderType)b, false); Add(set, pos, (ECardBorderType)b, true); added += 2;
                        }
                    }
                    else
                    {
                        Add(set, pos, ECardBorderType.Base, false);
                        Add(set, pos, (ECardBorderType)Random.Range(0, CardStore.BordersPerCard), Random.value < 0.5f);
                        added += 2;
                    }
                }
            Plugin.Log.LogInfo($"Granted {added} custom cards");
        }

        private static void Add(CustomSet set, int pos, ECardBorderType border, bool foil) =>
            CPlayerData.AddCard(new CardData
            {
                expansionType = set.Expansion,
                monsterType = set.Shown[pos],
                borderType = border,
                isFoil = foil
            }, 1);
    }
}

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

        public static void Init(ConfigFile config)
        {
            GrantKey = config.Bind("Debug", "GrantTestCards", new KeyboardShortcut(KeyCode.F9),
                "Adds one Base copy and one random variant of every custom card to the collection.");
            GrantAllVariantsKey = config.Bind("Debug", "GrantAllVariants", new KeyboardShortcut(KeyCode.F9, KeyCode.LeftShift),
                "Adds one copy of every border/foil variant of every custom card.");

            var go = new GameObject("TCGCustomCards_Runner");
            DontDestroyOnLoad(go);
            go.hideFlags = HideFlags.HideAndDontSave;
            go.AddComponent<DevTools>();
        }

        private void Update()
        {
            if (GrantAllVariantsKey.Value.IsDown()) Grant(allVariants: true);
            else if (GrantKey.Value.IsDown()) Grant(allVariants: false);
        }

        private static void Grant(bool allVariants)
        {
            if (SceneManager.GetActiveScene().name != "Start" || !GameInstance.m_FinishedSavefileLoading)
            {
                Plugin.Log.LogInfo("Grant ignored: load a save first");
                return;
            }
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

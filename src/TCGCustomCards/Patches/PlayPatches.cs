using System;
using System.Collections.Generic;
using HarmonyLib;
using TCGCustomCards.Runtime;
using UnityEngine;

namespace TCGCustomCards.Patches
{
    // Layer 7 — play table. Stats/element/evolution come from MonsterData built in Registry; effects from Runtime\PlayEffects.

    [HarmonyPatch(typeof(PlayCardEffect_ScriptableObject), nameof(PlayCardEffect_ScriptableObject.GetPlayEffectData))]
    internal static class CustomPlayEffect
    {
        private static bool Prefix(EMonsterType monsterType, ref PlayEffectData __result)
        {
            if (!Registry.IsCustom(monsterType)) return true;
            __result = PlayEffects.Get(monsterType); // null = no effect (safe for the player's cards)
            return false;
        }
    }

    /// <summary>
    /// Printed stats: CardUI shows Str/Vit/Spi/Mag when FireElement is 0; custom cards carry lane attack in both sets of
    /// stats (Str.. = 2× lane so play math matches), so print the lane values instead.
    /// </summary>
    [HarmonyPatch(typeof(CardUI), nameof(CardUI.SetCardUI))]
    internal static class CustomCardStatText
    {
        private static void Postfix(CardUI __instance, CardData cardData)
        {
            if (!Registry.TryGetCard(cardData.monsterType, out var set, out int pos)) return;
            var s = set.Monsters[pos].BaseStats;
            if (s.FireElement != 0) return;
            __instance.m_Stat1Text.text = s.FireElement.ToString();
            __instance.m_Stat2Text.text = s.EarthElement.ToString();
            __instance.m_Stat3Text.text = s.WaterElement.ToString();
            __instance.m_Stat4Text.text = s.WindElement.ToString();
        }
    }

    /// <summary>
    /// Deck text export: vanilla lines are "{n}X {Name} {Expansion} {N|B} {saveIndex+1}". Custom expansion ints change every
    /// launch, so custom cards export as "{n}X {Name_with_underscores} custom:{setId}:{cardId} N {variant+1}".
    /// </summary>
    [HarmonyPatch(typeof(PlayCardGameManager), nameof(PlayCardGameManager.CopyDeckDataToClipboard))]
    internal static class DeckExport
    {
        private static void Postfix(List<CompactCardDataAmount> compactCardDataAmountList)
        {
            bool anyCustom = false;
            var lines = new List<string>();
            foreach (var c in compactCardDataAmountList)
            {
                if (Registry.IsCustom(c.expansionType) && Registry.Get(c.expansionType) is CustomSet set)
                {
                    anyCustom = true;
                    int pos = CardStore.CardPos(c.cardSaveIndex);
                    var card = set.Card(Math.Min(pos, set.Def.Cards.Count - 1));
                    lines.Add($"{c.amount}X {card.Name.Replace(' ', '_')} custom:{set.Def.Id}:{card.Id} N {(c.cardSaveIndex % CardStore.SlotsPerCard) + 1:000}");
                }
                else
                {
                    var monster = CPlayerData.GetMonsterTypeFromCardSaveIndex(c.cardSaveIndex, c.expansionType);
                    lines.Add($"{c.amount}X {InventoryBase.GetMonsterData(monster).GetName()} {c.expansionType} {(c.isDestiny ? "B" : "N")} {c.cardSaveIndex + 1:000}");
                }
            }
            if (anyCustom) GUIUtility.systemCopyBuffer = string.Join("\n", lines);
        }
    }

    [HarmonyPatch(typeof(PlayCardGameManager), "GetCompactCardDataAmountFromCardDataString")]
    internal static class DeckImport
    {
        private static bool Prefix(string deckPasteString, ref CompactCardDataAmount __result)
        {
            var parts = deckPasteString.Split(new[] { ' ' }, StringSplitOptions.RemoveEmptyEntries);
            if (parts.Length < 5 || !parts[2].StartsWith("custom:")) return true;
            var ids = parts[2].Split(new[] { ':' }, 3);
            var set = ids.Length == 3 ? Registry.Get(ids[1]) : null;
            if (set == null || !set.PosByCardId.TryGetValue(ids[2], out int pos))
                throw new FormatException($"Unknown custom card '{parts[2]}'"); // vanilla catches and rejects the paste
            __result = new CompactCardDataAmount
            {
                amount = int.Parse(parts[0].Replace("X", "")),
                expansionType = set.Expansion,
                isDestiny = false,
                cardSaveIndex = pos * CardStore.SlotsPerCard + Mathf.Clamp(int.Parse(parts[4]) - 1, 0, CardStore.SlotsPerCard - 1)
            };
            return false;
        }
    }
}

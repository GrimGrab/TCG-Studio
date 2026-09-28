using HarmonyLib;
using I2.Loc;
using TCGCustomCards.Runtime;
using UnityEngine;

namespace TCGCustomCards.Patches
{
    // Layer 1 — identity: route custom expansion/monster ints to Registry data. Every patch no-ops for vanilla values.

    [HarmonyPatch(typeof(InventoryBase), nameof(InventoryBase.GetMonsterData))]
    internal static class GetMonsterDataPatch
    {
        private static bool Prefix(EMonsterType monsterType, ref MonsterData __result)
        {
            if (!Registry.IsCustom(monsterType)) return true;
            __result = Registry.TryGetCard(monsterType, out var set, out int pos) ? set.Monsters[pos] : Registry.MissingMonster;
            return false;
        }
    }

    [HarmonyPatch(typeof(InventoryBase), nameof(InventoryBase.GetShownMonsterList))]
    internal static class GetShownMonsterListPatch
    {
        private static bool Prefix(ECardExpansionType expansionType, ref System.Collections.Generic.List<EMonsterType> __result)
        {
            if (!Registry.IsCustom(expansionType)) return true;
            __result = Registry.Get(expansionType)?.Shown ?? new System.Collections.Generic.List<EMonsterType>();
            return false;
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.GetCardSaveIndex))]
    internal static class GetCardSaveIndexPatch
    {
        private static bool Prefix(CardData cardData, ref int __result)
        {
            if (!Registry.IsCustom(cardData.expansionType)) return true;
            var set = Registry.Get(cardData.expansionType);
            // O(1) lookup instead of vanilla's linear scan. Unknown monster → 0, matching vanilla behaviour.
            __result = set != null && set.PosByMonster.TryGetValue(cardData.monsterType, out int pos)
                ? CardStore.Slot(pos, cardData.borderType, cardData.isFoil)
                : 0;
            return false;
        }
    }

    [HarmonyPatch(typeof(InventoryBase), nameof(InventoryBase.GetCardUISetting))]
    internal static class GetCardUISettingPatch
    {
        private static bool Prefix(ECardExpansionType expansionType, ref CardUISetting __result)
        {
            if (!Registry.IsCustom(expansionType)) return true;
            var set = Registry.Get(expansionType);
            __result = set != null ? set.UISetting : CSingleton<InventoryBase>.Instance.m_MonsterData_SO.m_CardUISettingList[0];
            return false;
        }
    }

    [HarmonyPatch(typeof(InventoryBase), nameof(InventoryBase.GetCardExpansionName))]
    internal static class GetCardExpansionNamePatch
    {
        private static bool Prefix(ECardExpansionType cardExpansion, ref string __result)
        {
            if (!Registry.IsCustom(cardExpansion)) return true;
            __result = Registry.Get(cardExpansion)?.Def.Name ?? "Unknown Set";
            return false;
        }
    }

    [HarmonyPatch(typeof(MonsterData_ScriptableObject), nameof(MonsterData_ScriptableObject.GetCardBackSprite))]
    internal static class GetCardBackSpritePatch
    {
        private static bool Prefix(MonsterData_ScriptableObject __instance, ECardExpansionType cardExpansionType, ref Sprite __result)
        {
            if (!Registry.IsCustom(cardExpansionType)) return true;
            var set = Registry.Get(cardExpansionType);
            __result = set != null ? set.CardBack : __instance.m_CardBackImageList[0];
            return false;
        }
    }

    [HarmonyPatch(typeof(MonsterData_ScriptableObject), nameof(MonsterData_ScriptableObject.GetCardFoilMaskSprite))]
    internal static class GetCardFoilMaskSpritePatch
    {
        private static bool Prefix(MonsterData_ScriptableObject __instance, ECardExpansionType cardExpansionType, ref Sprite __result)
        {
            if (!Registry.IsCustom(cardExpansionType)) return true;
            var set = Registry.Get(cardExpansionType);
            __result = __instance.m_CardFoilMaskImageList[set != null ? (int)set.Def.FrameTemplate : 0];
            return false;
        }
    }

    // ---- Text: custom cards carry plain strings, not I2 terms ----

    [HarmonyPatch(typeof(MonsterData), nameof(MonsterData.GetName))]
    internal static class MonsterGetNamePatch
    {
        private static bool Prefix(MonsterData __instance, ref string __result)
        {
            if (!Registry.IsCustom(__instance.MonsterType)) return true;
            __result = __instance.Name;
            return false;
        }
    }

    [HarmonyPatch(typeof(MonsterData), nameof(MonsterData.GetDescription))]
    internal static class MonsterGetDescriptionPatch
    {
        private static bool Prefix(MonsterData __instance, ref string __result)
        {
            if (!Registry.IsCustom(__instance.MonsterType)) return true;
            __result = __instance.Description ?? "";
            return false;
        }
    }

    [HarmonyPatch(typeof(MonsterData), nameof(MonsterData.GetIcon))]
    internal static class MonsterGetIconPatch
    {
        private static bool Prefix(MonsterData __instance, ref Sprite __result)
        {
            if (!Registry.IsCustom(__instance.MonsterType)) return true;
            __result = Registry.TryGetCard(__instance.MonsterType, out var set, out int pos) ? set.CardImage(pos) : null;
            return false;
        }
    }

    /// <summary>Graded slabs translate expansionType.ToString() ("100" for a custom set) — answer with the set name.</summary>
    [HarmonyPatch(typeof(LocalizationManager), nameof(LocalizationManager.GetTranslation))]
    internal static class GetTranslationPatch
    {
        private static void Postfix(string Term, ref string __result)
        {
            if (!string.IsNullOrEmpty(__result) || string.IsNullOrEmpty(Term) || Term.Length > 6) return;
            if (int.TryParse(Term, out int n) && n >= Registry.ExpansionBase)
            {
                var set = Registry.Get((ECardExpansionType)n);
                if (set != null) __result = set.Def.Name;
            }
        }
    }
}

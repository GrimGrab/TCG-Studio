using System.Collections.Generic;
using System.Linq;
using System.Reflection.Emit;
using HarmonyLib;
using TCGCustomCards.Runtime;

namespace TCGCustomCards.Patches
{
    // Layer 4b — custom-only mode (see Runtime\VanillaFilter). Shop lists are filtered in VanillaFilter.Apply;
    // these patches cover the places that don't read those lists.

    [HarmonyPatch(typeof(InventoryBase), nameof(InventoryBase.GetUnlockableItemTypeAtShopLevel))]
    internal static class CustomerDemandFilter
    {
        private static void Postfix(List<EItemType> __result)
        {
            if (VanillaFilter.HidePacks || VanillaFilter.HideAccessories) __result.RemoveAll(VanillaFilter.IsHidden);
        }
    }

    /// <summary>
    /// Deck box / playmat picker, play-table setups (incl. comics) and NPC match boards list items by category: drop hidden vanilla accessories.
    /// </summary>
    [HarmonyPatch(typeof(InventoryBase), nameof(InventoryBase.GetItemTypeListFromCategory))]
    internal static class CategoryListFilter
    {
        private static void Postfix(EItemCategory category, List<EItemType> __result)
        {
            if (VanillaFilter.HideAccessories && (category == EItemCategory.Deckbox || category == EItemCategory.Playmat || category == EItemCategory.Manga))
                __result.RemoveAll(VanillaFilter.IsHidden);
        }
    }

    /// <summary>Random pick by category (customer tables, NPC board): re-pick from the filtered list when a hidden vanilla item came up.</summary>
    [HarmonyPatch(typeof(InventoryBase), nameof(InventoryBase.GetRandomItemTypeFromCategory))]
    internal static class CategoryRandomFilter
    {
        private static void Postfix(EItemCategory category, bool unlockedOnly, ref EItemType __result)
        {
            if (!VanillaFilter.HideAccessories || __result == EItemType.None || !VanillaFilter.IsHidden(__result)) return;
            var list = InventoryBase.GetItemTypeListFromCategory(category, unlockedOnly);
            __result = list.Count == 0 ? EItemType.None : list[UnityEngine.Random.Range(0, list.Count)];
        }
    }

    /// <summary>
    /// A deck whose deck box / playmat is a hidden vanilla item (or an item no longer offered) opens the picker at "none" instead of an
    /// index of -1, which would keep the stale item when Done is pressed.
    /// </summary>
    [HarmonyPatch(typeof(DeckboxPlaymatNameEditScreen), nameof(DeckboxPlaymatNameEditScreen.OpenDeckboxPlaymatNameEditScreen))]
    internal static class DeckPickerIndexClamp
    {
        private static void Postfix(DeckboxPlaymatNameEditScreen __instance)
        {
            var t = Traverse.Create(__instance);
            foreach (var f in new[] { "m_CurrentDeckboxListIndex", "m_CurrentPlaymatListIndex" })
                if (t.Field(f).GetValue<int>() < 0) t.Field(f).SetValue(0);
        }
    }

    /// <summary>The tutorial's "unlock the basic card box" step also completes with a custom box license.</summary>
    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.SetUnlockItemLicense))]
    internal static class TutorialCustomBox
    {
        private static void Postfix(int index)
        {
            foreach (var pack in Registry.Packs)
                if (index == pack.RestockRows[(int)CustomPack.Row.Box] || index == pack.RestockRows[(int)CustomPack.Row.BoxBig])
                {
                    TutorialManager.AddTaskValue(ETutorialTaskCondition.UnlockBasicCardBox, 1f);
                    return;
                }
        }
    }

    /// <summary>Play-table prizes add a hardcoded AscensionCardPack; swap it for a custom pack when vanilla packs are hidden.</summary>
    [HarmonyPatch(typeof(PlayTableGame), nameof(PlayTableGame.EvaluateEndGameGift))]
    internal static class PlayTableGiftPack
    {
        private static IEnumerable<CodeInstruction> Transpiler(IEnumerable<CodeInstruction> instructions)
        {
            var gift = AccessTools.Method(typeof(VanillaFilter), nameof(VanillaFilter.GiftPack));
            int replaced = 0;
            foreach (var ins in instructions)
            {
                if (ins.LoadsConstant((int)EItemType.AscensionCardPack))
                {
                    replaced++;
                    yield return new CodeInstruction(OpCodes.Call, gift).MoveLabelsFrom(ins);
                    continue;
                }
                yield return ins;
            }
            if (replaced == 0) Plugin.Log.LogWarning("PlayTableGiftPack: AscensionCardPack constant not found (game updated?)");
        }
    }

    /// <summary>
    /// Packs hidden and no custom pack: the card-pack list is empty and vanilla would index list[0] of an empty prize list. No prize then.
    /// </summary>
    [HarmonyPatch(typeof(PlayTableGame), nameof(PlayTableGame.EvaluateEndGameGift))]
    internal static class PlayTableNoGiftWithoutPacks
    {
        private static readonly AccessTools.FieldRef<PlayTableGame, List<EItemType>> Gifts =
            AccessTools.FieldRefAccess<PlayTableGame, List<EItemType>>("m_EndGameGiftItemTypeList");

        private static bool Prefix(PlayTableGame __instance)
        {
            if (CSingleton<InventoryBase>.Instance.m_StockItemData_SO.m_CardPackItemTypeList.Count > 0) return true;
            Gifts(__instance).Clear();
            return false;
        }
    }

    /// <summary>Vanilla cards hidden and no custom set: customers don't come to trade/sell cards.</summary>
    [HarmonyPatch(typeof(ShelfManager), nameof(ShelfManager.GetCashierCounterToTradeCard))]
    internal static class NoTradeCustomersWithoutCards
    {
        private static bool Prefix(ref InteractableCashierCounter __result)
        {
            if (!VanillaFilter.NoTradeCards) return true;
            __result = null;
            return false;
        }
    }

    /// <summary>Trade/sell customers pick Tetramon/Destiny/Ghost/Ascension; remap to a custom set just before it is used.</summary>
    [HarmonyPatch(typeof(CustomerTradeCardScreen), nameof(CustomerTradeCardScreen.SetCustomer))]
    internal static class TradeCustomerExpansion
    {
        private static IEnumerable<CodeInstruction> Transpiler(IEnumerable<CodeInstruction> instructions)
        {
            var codes = instructions.ToList();
            var shown = AccessTools.Method(typeof(InventoryBase), nameof(InventoryBase.GetShownMonsterList));
            var remap = AccessTools.Method(typeof(VanillaFilter), nameof(VanillaFilter.TradeExpansion));
            int call = codes.FindIndex(c => c.Calls(shown));
            if (call < 1 || !IsLdloc(codes[call - 1].opcode))
            {
                Plugin.Log.LogWarning("TradeCustomerExpansion: pattern not found (game updated?); trades keep vanilla sets");
                return codes;
            }
            var load = codes[call - 1];
            var inject = new List<CodeInstruction>
            {
                new CodeInstruction(load.opcode, load.operand),
                new CodeInstruction(OpCodes.Call, remap),
                StoreFor(load)
            };
            inject[0].MoveLabelsFrom(load);
            codes.InsertRange(call - 1, inject);
            return codes;
        }

        private static bool IsLdloc(OpCode op) =>
            op == OpCodes.Ldloc_0 || op == OpCodes.Ldloc_1 || op == OpCodes.Ldloc_2 || op == OpCodes.Ldloc_3 || op == OpCodes.Ldloc_S || op == OpCodes.Ldloc;

        private static CodeInstruction StoreFor(CodeInstruction ldloc)
        {
            if (ldloc.opcode == OpCodes.Ldloc_0) return new CodeInstruction(OpCodes.Stloc_0);
            if (ldloc.opcode == OpCodes.Ldloc_1) return new CodeInstruction(OpCodes.Stloc_1);
            if (ldloc.opcode == OpCodes.Ldloc_2) return new CodeInstruction(OpCodes.Stloc_2);
            if (ldloc.opcode == OpCodes.Ldloc_3) return new CodeInstruction(OpCodes.Stloc_3);
            if (ldloc.opcode == OpCodes.Ldloc_S) return new CodeInstruction(OpCodes.Stloc_S, ldloc.operand);
            return new CodeInstruction(OpCodes.Stloc, ldloc.operand);
        }
    }

    /// <summary>The binder opens on Tetramon by default; start on a custom set when vanilla cards are hidden.</summary>
    [HarmonyPatch(typeof(CollectionBinderFlipAnimCtrl), "Update")]
    internal static class BinderDefaultExpansion
    {
        private static readonly AccessTools.FieldRef<CollectionBinderFlipAnimCtrl, ECardExpansionType> Expansion =
            AccessTools.FieldRefAccess<CollectionBinderFlipAnimCtrl, ECardExpansionType>("m_ExpansionType");

        private static void Prefix(CollectionBinderFlipAnimCtrl __instance)
        {
            if (VanillaFilter.TryRedirect(Expansion(__instance), out var custom)) Expansion(__instance) = custom;
        }
    }

    /// <summary>Check Price opens its card page on Tetramon; switch to a custom set (with correct page count) when vanilla cards are hidden.</summary>
    [HarmonyPatch(typeof(CheckPriceScreen), "EvaluateCardPanelUI")]
    internal static class CheckPriceDefaultExpansion
    {
        private static readonly AccessTools.FieldRef<CheckPriceScreen, ECardExpansionType> Expansion =
            AccessTools.FieldRefAccess<CheckPriceScreen, ECardExpansionType>("m_CurrentExpansionType");
        private static readonly AccessTools.FieldRef<CheckPriceScreen, int> PageIndex =
            AccessTools.FieldRefAccess<CheckPriceScreen, int>("m_CardPageIndex");
        private static readonly AccessTools.FieldRef<CheckPriceScreen, int> PageMax =
            AccessTools.FieldRefAccess<CheckPriceScreen, int>("m_CardPageMaxIndex");

        private static void Prefix(CheckPriceScreen __instance, ref int cardPageIndex)
        {
            if (!VanillaFilter.TryRedirect(Expansion(__instance), out var exp)) return;
            Expansion(__instance) = exp;
            PageIndex(__instance) = 0;
            PageMax(__instance) = InventoryBase.GetShownMonsterList(exp).Count * CPlayerData.GetCardAmountPerMonsterType(exp) / __instance.m_MaxCardUICountPerPage - 1;
            cardPageIndex = 0;
        }
    }

    [HarmonyPatch(typeof(CGameData), nameof(CGameData.PropagateLoadData))]
    internal static class VanillaFilterOnLoad
    {
        private static void Postfix() => VanillaFilter.ApplyPlayerDefaults();
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.CreateDefaultData))]
    internal static class VanillaFilterOnNewGame
    {
        private static void Postfix() => VanillaFilter.ApplyPlayerDefaults();
    }
}

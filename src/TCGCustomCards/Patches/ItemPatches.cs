using System.Collections.Generic;
using HarmonyLib;
using TCGCustomCards.Runtime;
using TCGCustomCards.UI;

namespace TCGCustomCards.Patches
{
    // Layer 5 — custom pack & box items: injection timing, hardcoded pack/box checks, pack rolling, names, restock UI.

    // ---- Injection: as early as possible in the shop scene (these objects copy item lists in Awake/init) ----

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.CreateDefaultData))]
    internal static class InjectOnCreateDefaultData
    {
        [HarmonyPriority(Priority.First)]
        private static void Postfix() { ItemInjector.EnsureInjected(); ItemInjector.EnsurePlayerListSizes(); }
    }

    [HarmonyPatch(typeof(WorkerSetPackOpenerTypeOptionScreen), "Awake")]
    internal static class InjectBeforePackOptionScreen
    {
        private static void Prefix() => ItemInjector.EnsureInjected();
    }

    [HarmonyPatch(typeof(Worker), nameof(Worker.InitializeCharacter))]
    internal static class InjectBeforeWorkerInit
    {
        private static void Prefix() => ItemInjector.EnsureInjected();
    }

    [HarmonyPatch(typeof(CGameData), nameof(CGameData.PropagateLoadData))]
    internal static class InjectBeforeLoad
    {
        private static void Prefix() => ItemInjector.EnsureInjected();
        private static void Postfix() => ItemInjector.EnsurePlayerListSizes();
    }

    [HarmonyPatch(typeof(RestockManager), "Init")]
    internal static class RestockInitPrices
    {
        /// <summary>Custom item prices are restored from the side-car (or zeroed so Init generates them from the set definition).</summary>
        private static void Prefix()
        {
            ItemInjector.EnsureInjected();
            ItemInjector.EnsurePlayerListSizes();
        }
    }

    // ---- Hardcoded mappings ----

    [HarmonyPatch(typeof(InventoryBase), nameof(InventoryBase.ItemTypeToCollectionPackType))]
    internal static class ItemToPackTypePatch
    {
        private static bool Prefix(EItemType itemType, ref ECollectionPackType __result)
        {
            var pack = Registry.GetByItem(itemType);
            if (pack == null) return true;
            __result = pack.PackType;
            return false;
        }
    }

    [HarmonyPatch(typeof(InventoryBase), nameof(InventoryBase.GetCardExpansionType))]
    internal static class PackTypeToExpansionPatch
    {
        private static bool Prefix(ECollectionPackType collectionPackType, ref ECardExpansionType __result)
        {
            if (!Registry.IsCustom(collectionPackType)) return true;
            __result = Registry.Get(collectionPackType)?.Set.Expansion ?? ECardExpansionType.Tetramon;
            return false;
        }
    }

    [HarmonyPatch(typeof(InteractionPlayerController), nameof(InteractionPlayerController.IsCardPackType))]
    internal static class IsCardPackTypePatch
    {
        private static void Postfix(EItemType itemType, ref bool __result)
        {
            if (!__result && Registry.IsCustomPackItem(itemType)) __result = true;
        }
    }

    [HarmonyPatch(typeof(InteractionPlayerController), nameof(InteractionPlayerController.CanOpenCardBox))]
    internal static class CanOpenCardBoxPatch
    {
        private static readonly AccessTools.FieldRef<InteractionPlayerController, List<Item>> HoldItems =
            AccessTools.FieldRefAccess<InteractionPlayerController, List<Item>>("m_HoldItemList");

        private static void Postfix(InteractionPlayerController __instance, ref bool __result)
        {
            if (__result) return;
            var hold = HoldItems(__instance);
            if (hold != null && hold.Count > 0 && hold[0] != null && Registry.IsCustomBoxItem(hold[0].GetItemType())) __result = true;
        }
    }

    [HarmonyPatch(typeof(InteractionPlayerController), "CardBoxToCardPack")]
    internal static class CardBoxToCardPackPatch
    {
        private static void Postfix(EItemType cardBoxItemType, ref EItemType __result)
        {
            if (__result == EItemType.None && Registry.IsCustomBoxItem(cardBoxItemType))
                __result = Registry.GetByItem(cardBoxItemType).PackItem;
        }
    }

    // ---- Pack contents ----

    [HarmonyPatch(typeof(CardOpeningSequence), "GetPackContent")]
    internal static class GetPackContentPatch
    {
        private static readonly AccessTools.FieldRef<CardOpeningSequence, ECollectionPackType> PackTypeRef =
            AccessTools.FieldRefAccess<CardOpeningSequence, ECollectionPackType>("m_CollectionPackType");
        private static readonly AccessTools.FieldRef<CardOpeningSequence, List<CardData>> Rolled =
            AccessTools.FieldRefAccess<CardOpeningSequence, List<CardData>>("m_RolledCardDataList");
        private static readonly AccessTools.FieldRef<CardOpeningSequence, List<CardData>> Rolled2 =
            AccessTools.FieldRefAccess<CardOpeningSequence, List<CardData>>("m_SecondaryRolledCardDataList");
        private static readonly AccessTools.FieldRef<CardOpeningSequence, List<CardData>> Pool =
            AccessTools.FieldRefAccess<CardOpeningSequence, List<CardData>>("m_CardDataPool");
        private static readonly AccessTools.FieldRef<CardOpeningSequence, List<CardData>> Pool2 =
            AccessTools.FieldRefAccess<CardOpeningSequence, List<CardData>>("m_CardDataPool2");
        private static readonly AccessTools.FieldRef<CardOpeningSequence, bool> HasFoil =
            AccessTools.FieldRefAccess<CardOpeningSequence, bool>("m_HasFoilCard");

        private static bool Prefix(CardOpeningSequence __instance, bool clearList, bool isSecondaryRolledData, ECollectionPackType overrideCollectionPackType)
        {
            var packType = isSecondaryRolledData ? overrideCollectionPackType : PackTypeRef(__instance);
            if (!Registry.IsCustom(packType)) return true;
            var pack = Registry.Get(packType);
            if (pack == null) return true;

            var output = isSecondaryRolledData ? Rolled2(__instance) : Rolled(__instance);
            if (clearList) output.Clear();
            var pool = isSecondaryRolledData ? Pool2(__instance) : Pool(__instance);
            PackSizePatches.EnsurePool(pool, pack.Def.CardsPerPack); // vanilla pools hold 10; the roller stops at the pool size
            int before = output.Count;
            if (PackRoller.Roll(pack, pool, output))
                HasFoil(__instance) = true;
            // Opening only has slots for 7 when the N-card patch couldn't apply (PackSizePatches logs why).
            if (!PackSizePatches.RevealPatched && output.Count - before > 7) output.RemoveRange(before + 7, output.Count - before - 7);
            return false;
        }
    }

    // ---- Text ----

    [HarmonyPatch(typeof(ItemData), nameof(ItemData.GetName))]
    internal static class ItemNamePatch
    {
        private static bool Prefix(ItemData __instance, ref string __result)
        {
            if (!ItemInjector.CustomItemData.Contains(__instance)) return true;
            __result = __instance.name;
            return false;
        }
    }

    // ---- Restock screen: grow the fixed panel pool when rows would be cut off ----

    [HarmonyPatch]
    internal static class RestockPanelPoolPatch
    {
        // The board-game screen overrides the method without calling base.
        private static IEnumerable<System.Reflection.MethodBase> TargetMethods()
        {
            yield return AccessTools.Method(typeof(RestockItemScreen), "EvaluateRestockItemPanelUI");
            yield return AccessTools.Method(typeof(RestockItemBoardGameScreen), "EvaluateRestockItemPanelUI");
        }

        private static void Prefix(RestockItemScreen __instance)
        {
            int rows = CSingleton<InventoryBase>.Instance.m_StockItemData_SO.m_RestockDataList.Count;
            UiPool.Grow(__instance.m_RestockItemPanelUIList, rows, __instance.GetType().Name);
        }
    }

    // ---- Worker load bug: pads the pack-enabled list by only one entry (Worker.cs:3244) ----

    [HarmonyPatch(typeof(Worker), nameof(Worker.LoadWorkerSaveData))]
    internal static class WorkerPackListPadding
    {
        private static readonly AccessTools.FieldRef<Worker, List<bool>> Enabled =
            AccessTools.FieldRefAccess<Worker, List<bool>>("m_CardPackItemTypeEnabledList");

        private static void Postfix(Worker __instance)
        {
            var list = Enabled(__instance);
            int want = CSingleton<InventoryBase>.Instance.m_StockItemData_SO.m_CardPackItemTypeList.Count;
            while (list != null && list.Count < want) list.Add(true);
        }
    }
}

using HarmonyLib;
using TCGCustomCards.Runtime;
using TCGCustomCards.Save;

namespace TCGCustomCards.Patches
{
    // Layer 8 (part 1) — side-car persistence. Strip/restore of the vanilla save comes in M4.
    // Load order: CPlayerData.Awake → CreateDefaultData → CSaveLoad.Load(slot) → CGameData.PropagateLoadData.

    [HarmonyPatch(typeof(CSaveLoad), nameof(CSaveLoad.Save))]
    internal static class SaveHook
    {
        // CSaveLoad.Save retries by calling itself; strip once at the outermost call and undo when it unwinds.
        private static int _depth;
        private static System.Action _undo;

        [HarmonyPriority(Priority.First)]
        private static void Prefix()
        {
            if (_depth++ > 0) return;
            SideCar.PendingStash = SaveStripper.Strip(CGameData.instance, out _undo);
        }

        private static void Postfix(int saveSlotIndex)
        {
            if (_depth == 1) SideCar.Write(saveSlotIndex);
        }

        private static System.Exception Finalizer(System.Exception __exception)
        {
            if (--_depth == 0)
            {
                _undo?.Invoke();
                _undo = null;
            }
            return __exception;
        }
    }

    /// <summary>Puts custom entries moved out of the vanilla save back, just before ShelfManager spawns the saved objects.</summary>
    [HarmonyPatch(typeof(ShelfManager), nameof(ShelfManager.LoadInteractableObjectData))]
    internal static class RestoreStrippedData
    {
        [HarmonyPriority(Priority.First)]
        private static void Prefix()
        {
            ItemInjector.EnsureInjected();
            ItemInjector.EnsurePlayerListSizes();
            SaveStripper.Restore(SideCar.TakeLoadedStash());
            SaveStripper.CleanOrphans();
        }
    }

    [HarmonyPatch(typeof(CSaveLoad), nameof(CSaveLoad.Load))]
    internal static class LoadHook
    {
        private static void Postfix(int slotIndex, bool __result)
        {
            if (__result) SideCar.Read(slotIndex);
        }
    }

    /// <summary>The game's fallback when a slot's save can't be read: load the matching side-car too (else a stale one was kept).</summary>
    [HarmonyPatch(typeof(CSaveLoad), nameof(CSaveLoad.LoadBackup))]
    internal static class LoadBackupHook
    {
        private static void Postfix(int slotIndex, bool __result)
        {
            if (__result) SideCar.ReadBackup(slotIndex);
        }
    }

    /// <summary>
    /// Loading a manual slot while an autosave exists copies the autosave (slot 0) into the first empty slot 1–3 — vanilla files
    /// only. Copy our side-car to the same slot.
    /// </summary>
    [HarmonyPatch(typeof(CSaveLoad), nameof(CSaveLoad.AutoSaveMoveToEmptySaveSlot))]
    internal static class AutoSaveMoveHook
    {
        private static void Prefix(out int __state)
        {
            __state = -1;
            string dir = UnityEngine.Application.persistentDataPath;
            if (!System.IO.File.Exists(dir + "/savedGames_Release0.json")) return;
            for (int i = 1; i <= 3; i++)
                if (!System.IO.File.Exists(dir + $"/savedGames_Release{i}.json") && !System.IO.File.Exists(dir + $"/savedGames_Release{i}.gd"))
                {
                    __state = i; // the slot the game is about to copy into (same order as CSaveLoad)
                    return;
                }
        }

        private static void Postfix(int __state)
        {
            if (__state > 0) SideCar.CopySlot(0, __state);
        }
    }

    [HarmonyPatch(typeof(CGameData), nameof(CGameData.PropagateLoadData))]
    internal static class PropagateLoadDataHook
    {
        private static void Postfix()
        {
            SideCar.ApplyPending();
            SortListPadding.Apply();
        }
    }

    [HarmonyPatch(typeof(CPlayerData), nameof(CPlayerData.CreateDefaultData))]
    internal static class CreateDefaultDataHook
    {
        private static void Postfix()
        {
            SideCar.ResetForNewGame();
            SortListPadding.Apply();
        }
    }

    /// <summary>The binder indexes m_CollectionSortingMethodIndexList by (int)expansion (13 entries at runtime).</summary>
    internal static class SortListPadding
    {
        public static void Apply()
        {
            var list = CPlayerData.m_CollectionSortingMethodIndexList;
            if (list == null) return;
            while (list.Count <= Registry.MaxExpansionInt) list.Add((int)ECollectionSortingType.Default);
        }
    }
}

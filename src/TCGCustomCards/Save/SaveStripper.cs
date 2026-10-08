using System;
using System.Collections;
using System.Collections.Generic;
using System.Linq;
using System.Reflection;
using TCGCustomCards.Runtime;
using UnityEngine;

namespace TCGCustomCards.Save
{
    /// <summary>
    /// Keeps the vanilla save free of custom content so it still loads without the mod.
    /// Save: every CGameData field that references custom content is swapped (for the duration of CSaveLoad.Save only) for a
    /// cleaned copy; the originals are stashed in the side-car with the int allocation of that moment.
    /// Load: before ShelfManager spawns objects, stashed originals are remapped to the current ints and put back into CPlayerData.
    /// </summary>
    internal static class SaveStripper
    {
        public class StashEntry
        {
            public string field;
            public int index = -1;
            /// <summary>
            /// replace (list element / object at index), remove (appended back), removeAt (put back at its index: placed custom furniture,
            /// whose list order other save data may rely on), scalar (object or enum field), spawn (paired restock lists),
            /// warehouseBox (a box stored on a custom warehouse shelf: appended back, its shelf index is the saved list's),
            /// decoCount (owned custom decorations: "int,count"), surfaceUnlock (a bought custom wall/floor/ceiling: its index),
            /// surfaceEquip (the equipped custom look: field = the CPlayerData index field, json = its index).
            /// </summary>
            public string mode;
            public string type;
            public string json;
        }

        public class Stash
        {
            public AllocationSnapshot alloc;
            public List<StashEntry> entries = new List<StashEntry>();
        }

        /// <summary>Unordered collections: custom entries are removed from the save and appended back on load.</summary>
        private static readonly HashSet<string> RemoveLists = new HashSet<string>
        {
            "m_PackageBoxItemSaveDataList", "m_PackageBoxCardSaveDataList", "m_CustomerSaveDataList", "m_GradedCardInventoryList",
            "m_HoldCardDataList", "m_HoldItemTypeList", "m_TargetBuyItemList", "m_CustomerReviewDataList",
            "m_DecoObjectSaveDataList"
        };

        private static readonly FieldInfo[] GameFields = typeof(CGameData).GetFields(BindingFlags.Instance | BindingFlags.Public);

        // ------------------------------------------------------------------ strip (save)

        /// <summary>Swaps custom-referencing fields of <paramref name="g"/> for cleaned copies. Returns the stash and an undo action.</summary>
        public static Stash Strip(CGameData g, out Action undo)
        {
            var stash = new Stash { alloc = AllocationSnapshot.Current() };
            var originals = new List<(FieldInfo f, object value)>();
            // Newest first: a field swapped twice (pre-pass, then the generic pass) ends with its true original.
            undo = () => { for (int i = originals.Count - 1; i >= 0; i--) originals[i].f.SetValue(g, originals[i].value); };
            if (Registry.Sets.Count == 0 && Registry.Accessories.Count == 0 && Registry.Furniture.Count == 0 && Registry.Decorations.Count == 0) return stash;

            StripSpawnWaiting(g, stash, originals);
            StripWarehouseBoxes(g, stash, originals);
            foreach (var f in GameFields)
            {
                if (f.Name == "m_SpawnBoxRestockIndexWaitingList" || f.Name == "m_SpawnBoxItemCountWaitingList") continue;
                try
                {
                    object value = f.GetValue(g);
                    if (value == null) continue;
                    object cleaned = StripField(f, value, stash);
                    if (cleaned != null && !ReferenceEquals(cleaned, value))
                    {
                        originals.Add((f, value));
                        f.SetValue(g, cleaned);
                    }
                }
                catch (Exception e)
                {
                    Plugin.Log.LogError($"Save strip failed for {f.Name}: {e}");
                }
            }
            if (stash.entries.Count > 0) Plugin.Log.LogInfo($"Save: moved {stash.entries.Count} custom entries out of the vanilla save");
            return stash;
        }

        private static object StripField(FieldInfo f, object value, Stash stash)
        {
            string name = f.Name;
            var type = f.FieldType;

            if (name == "m_IsItemLicenseUnlocked" && value is List<bool> licenses)
            {
                // Custom rows live in the side-car; never leave stray unlocks at indices a game update might reuse.
                var rows = stash.alloc.restockRows.Keys.Where(r => r < licenses.Count && licenses[r]).ToList();
                if (rows.Count == 0) return value;
                var copy = new List<bool>(licenses);
                foreach (int r in rows) copy[r] = false;
                return copy;
            }
            if (name == "m_DecorationInventoryList" && value is List<int> owned)
            {
                // Owned custom decorations (by EDecoObject int): counts move to the side-car.
                var ints = stash.alloc.decorations.Keys.Where(v => v < owned.Count && owned[v] != 0).ToList();
                if (ints.Count == 0) return value;
                var copy = new List<int>(owned);
                foreach (int v in ints)
                {
                    stash.entries.Add(new StashEntry { field = name, mode = "decoCount", json = $"{v},{owned[v]}" });
                    copy[v] = 0;
                }
                return copy;
            }
            if (SurfaceKind(name) is Core.DecorationKind unlockKind && name.StartsWith("m_UnlockedDeco") && value is List<bool> unlocked)
            {
                var map = stash.alloc.SurfaceMap(unlockKind);
                var idx = map.Keys.Where(i => i < unlocked.Count && unlocked[i]).ToList();
                if (idx.Count == 0) return value;
                var copy = new List<bool>(unlocked);
                foreach (int i in idx)
                {
                    stash.entries.Add(new StashEntry { field = name, mode = "surfaceUnlock", json = i.ToString() });
                    copy[i] = false;
                }
                return copy;
            }
            if (SurfaceKind(name) is Core.DecorationKind equipKind && name.StartsWith("m_Equipped") && value is int equipped)
            {
                if (!stash.alloc.SurfaceMap(equipKind).ContainsKey(equipped)) return value;
                stash.entries.Add(new StashEntry { field = name, mode = "surfaceEquip", json = equipped.ToString() });
                return 0; // the default look
            }
            if (name == "m_ChampionCardCollectedList" && value is List<int> champions)
            {
                if (!champions.Any(CustomRefWalker.IsCustomMonster)) return value;
                foreach (int m in champions.Where(CustomRefWalker.IsCustomMonster))
                    stash.entries.Add(new StashEntry { field = name, mode = "remove", type = "monster", json = m.ToString() });
                return champions.Where(m => !CustomRefWalker.IsCustomMonster(m)).ToList();
            }
            if (type == typeof(ERarity))
            {
                // Rarity filters (workbench, quick fill, donation): a set's own rarity is kept by stable id.
                var r = (ERarity)value;
                if (!Registry.IsCustomRarity(r)) return value;
                if (Registry.TryGetRarity(r, out var set, out var def))
                    stash.entries.Add(new StashEntry { field = name, mode = "scalar", type = RarityType, json = $"{set.Def.Id}:{def.Id}" });
                return ERarity.None;
            }
            if (!CustomRefWalker.MayHold(type)) return value;

            if (type.IsEnum)
            {
                if (!CustomRefWalker.Walk(new EnumBox(value), CustomRefWalker.Mode.Detect)) return value;
                stash.entries.Add(new StashEntry { field = name, mode = "scalar", type = type.AssemblyQualifiedName, json = Convert.ToInt32(value).ToString() });
                return type == typeof(ECardExpansionType) ? ECardExpansionType.Tetramon : Enum.ToObject(type, type == typeof(EItemType) ? (int)EItemType.None : 0);
            }

            if (value is IList list && type.IsGenericType)
            {
                var elemType = type.GetGenericArguments()[0];
                bool remove = RemoveLists.Contains(name);
                IList copy = null;
                var removed = new List<int>();
                for (int i = 0; i < list.Count; i++)
                {
                    var item = list[i];
                    if (item == null) continue;
                    // A placed custom furniture piece leaves the save whole (its type can't be spawned without the mod).
                    bool piece = !elemType.IsEnum && CustomRefWalker.IsCustomFurnitureEntry(item);
                    bool custom = piece || (elemType.IsEnum ? CustomRefWalker.Walk(new EnumBox(item), CustomRefWalker.Mode.Detect) : CustomRefWalker.Walk(item, CustomRefWalker.Mode.Detect));
                    if (!custom) continue;
                    if (copy == null) copy = (IList)Activator.CreateInstance(type, list);
                    string mode = piece ? "removeAt" : remove ? "remove" : "replace";
                    stash.entries.Add(new StashEntry
                    {
                        field = name, index = i, mode = mode,
                        type = elemType.AssemblyQualifiedName,
                        json = elemType.IsEnum ? Convert.ToInt32(item).ToString() : JsonUtility.ToJson(item)
                    });
                    if (mode == "replace") copy[i] = Neutralized(item);
                    else removed.Add(i);
                }
                if (copy == null) return value;
                // Remove from the copy back to front using the recorded indices (replace entries keep theirs).
                for (int k = removed.Count - 1; k >= 0; k--) copy.RemoveAt(removed[k]);
                return copy;
            }

            if (type.IsClass && !(value is string))
            {
                if (!CustomRefWalker.Walk(value, CustomRefWalker.Mode.Detect)) return value;
                stash.entries.Add(new StashEntry { field = name, mode = "scalar", type = type.AssemblyQualifiedName, json = JsonUtility.ToJson(value) });
                return Neutralized(value);
            }
            return value;
        }

        /// <summary>Queued restock deliveries are two parallel lists (restock row, amount); custom rows are removed from both together.</summary>
        private static void StripSpawnWaiting(CGameData g, Stash stash, List<(FieldInfo f, object value)> originals)
        {
            var fRows = typeof(CGameData).GetField("m_SpawnBoxRestockIndexWaitingList");
            var fCounts = typeof(CGameData).GetField("m_SpawnBoxItemCountWaitingList");
            if (!(fRows?.GetValue(g) is List<int> rows) || !(fCounts?.GetValue(g) is List<int> counts)) return;
            if (!rows.Any(r => stash.alloc.restockRows.ContainsKey(r))) return;
            var keptRows = new List<int>();
            var keptCounts = new List<int>();
            for (int i = 0; i < rows.Count; i++)
            {
                int count = i < counts.Count ? counts[i] : 0;
                if (stash.alloc.restockRows.ContainsKey(rows[i]))
                    stash.entries.Add(new StashEntry { field = "spawnWaiting", mode = "spawn", json = $"{rows[i]},{count}" });
                else { keptRows.Add(rows[i]); keptCounts.Add(count); }
            }
            originals.Add((fRows, rows));
            originals.Add((fCounts, counts));
            fRows.SetValue(g, keptRows);
            fCounts.SetValue(g, keptCounts);
        }

        /// <summary>
        /// Surface kind of a CPlayerData/CGameData decoration field: m_UnlockedDeco{Wall,Floor,Ceiling}List and
        /// m_Equipped{Wall,Floor,Ceiling}DecoIndex(B). Null for any other field.
        /// </summary>
        private static Core.DecorationKind? SurfaceKind(string field)
        {
            if (!field.StartsWith("m_UnlockedDeco") && !(field.StartsWith("m_Equipped") && field.Contains("DecoIndex"))) return null;
            if (field.Contains("Wall")) return Core.DecorationKind.Wall;
            if (field.Contains("Floor")) return Core.DecorationKind.Floor;
            if (field.Contains("Ceiling")) return Core.DecorationKind.Ceiling;
            return null;
        }

        /// <summary>Stash type of a set's own rarity: json = "setId:rarityId".</summary>
        private const string RarityType = "rarity";
        private const string WarehouseField = "m_WarehouseShelfSaveDataList";
        private const string BoxField = "m_PackageBoxItemSaveDataList";

        /// <summary>
        /// Stored boxes point at their warehouse shelf by its index in the shelf list (ShelfManager.cs:565/854), and the generic pass
        /// takes custom shelves out of that list. Boxes on a custom shelf go to the stash with it (mode warehouseBox: full index,
        /// appended back on load). Boxes on vanilla shelves get the index their shelf has in the stripped list — also those the generic
        /// pass then stashes (custom item) — and get it back on load (<see cref="RemapStoredBoxes"/>, <see cref="RestoreOne"/>).
        /// </summary>
        private static void StripWarehouseBoxes(CGameData g, Stash stash, List<(FieldInfo f, object value)> originals)
        {
            var shelves = g.m_WarehouseShelfSaveDataList;
            var boxes = g.m_PackageBoxItemSaveDataList;
            if (shelves == null || boxes == null) return;
            var removed = new List<int>();
            for (int i = 0; i < shelves.Count; i++)
                if (shelves[i] != null && CustomRefWalker.IsCustomFurnitureEntry(shelves[i])) removed.Add(i);
            if (removed.Count == 0) return;
            var kept = new List<PackageBoxItemaveData>();
            foreach (var b in boxes)
            {
                if (b == null || !b.isStored) { kept.Add(b); continue; }
                if (removed.Contains(b.storedWarehouseShelfIndex))
                {
                    stash.entries.Add(new StashEntry { field = BoxField, mode = "warehouseBox", type = typeof(PackageBoxItemaveData).AssemblyQualifiedName, json = JsonUtility.ToJson(b) });
                    continue;
                }
                int below = removed.Count(r => r < b.storedWarehouseShelfIndex);
                if (below == 0) { kept.Add(b); continue; }
                var copy = JsonUtility.FromJson<PackageBoxItemaveData>(JsonUtility.ToJson(b));
                copy.storedWarehouseShelfIndex -= below;
                kept.Add(copy);
            }
            var f = typeof(CGameData).GetField(BoxField);
            originals.Add((f, boxes));
            f.SetValue(g, kept);
        }

        /// <summary>
        /// Load, after the custom shelves are back in the list: stripped indices of the vanilla save's boxes → current indices
        /// (<paramref name="removed"/> = saved indices of every stashed custom shelf, <paramref name="dropped"/> = those not restored).
        /// </summary>
        private static void RemapStoredBoxes(List<int> removed, HashSet<int> dropped)
        {
            if (removed.Count == 0 || CPlayerData.m_PackageBoxItemSaveDataList == null) return;
            foreach (var b in CPlayerData.m_PackageBoxItemSaveDataList)
                if (b != null && b.isStored) SetStoredShelf(b, SavedToFull(b.storedWarehouseShelfIndex, removed), dropped);
        }

        /// <summary>Index in the stripped list → index in the list as it was saved (custom shelves included).</summary>
        private static int SavedToFull(int stripped, List<int> removed)
        {
            int full = stripped;
            foreach (int r in removed) if (r <= full) full++; // removed is ascending
            return full;
        }

        /// <summary>Saved full index → current index; a box whose shelf wasn't restored is left lying where it was.</summary>
        private static void SetStoredShelf(PackageBoxItemaveData b, int full, HashSet<int> dropped)
        {
            if (dropped.Contains(full)) { b.isStored = false; return; }
            b.storedWarehouseShelfIndex = full - dropped.Count(d => d < full);
        }

        private static object Neutralized(object original)
        {
            var copy = JsonUtility.FromJson(JsonUtility.ToJson(original), original.GetType());
            CustomRefWalker.Walk(copy, CustomRefWalker.Mode.Neutralize);
            return copy;
        }

        /// <summary>Lets the walker inspect a lone enum value.</summary>
        [Serializable]
        private class EnumBox
        {
            public ECardExpansionType expansion;
            public EMonsterType monster;
            public EItemType item;
            public EDecoObject deco;

            public EnumBox(object value)
            {
                if (value is EDecoObject d) deco = d;
                if (value is ECardExpansionType e) expansion = e;
                else if (value is EMonsterType m) monster = m;
                else if (value is EItemType it) item = it;
                if (!(value is EItemType)) item = EItemType.None;
            }
        }

        // ------------------------------------------------------------------ orphan cleanup (load)

        /// <summary>
        /// Removes/blanks references to custom content that is no longer installed (a removed set, or saves written before
        /// strip/restore existed). Runs on the live CPlayerData save lists before ShelfManager spawns objects, so an unknown
        /// EItemType can never index past m_ItemDataList. Returns the number of entries changed.
        /// </summary>
        public static int CleanOrphans()
        {
            int changed = 0;
            foreach (var gf in GameFields)
            {
                var f = typeof(CPlayerData).GetField(gf.Name, BindingFlags.Static | BindingFlags.Public);
                if (f == null || f.FieldType != gf.FieldType) continue;
                try
                {
                    changed += CleanField(f);
                }
                catch (Exception e)
                {
                    Plugin.Log.LogError($"Orphan cleanup failed for {f.Name}: {e.Message}");
                }
            }
            changed += CleanSpawnWaiting();
            if (changed > 0) Plugin.Log.LogWarning($"Orphan cleanup: removed {changed} reference(s) to custom content that is no longer installed");
            return changed;
        }

        private static int CleanField(FieldInfo f)
        {
            var type = f.FieldType;
            object value = f.GetValue(null);
            if (value == null) return 0;

            if (f.Name == "m_ChampionCardCollectedList" && value is List<int> champions)
                return champions.RemoveAll(CustomRefWalker.IsOrphanMonster);

            if (type == typeof(ERarity))
            {
                var r = (ERarity)value;
                if (!Registry.IsCustomRarity(r) || Registry.TryGetRarity(r, out _, out _)) return 0;
                f.SetValue(null, ERarity.None);
                return 1;
            }

            if (!CustomRefWalker.MayHold(type)) return 0;

            if (type.IsEnum)
            {
                if (!CustomRefWalker.Walk(new EnumBox(value), CustomRefWalker.Mode.Detect, orphansOnly: true)) return 0;
                f.SetValue(null, type == typeof(ECardExpansionType) ? ECardExpansionType.Tetramon : Enum.ToObject(type, type == typeof(EItemType) ? (int)EItemType.None : 0));
                return 1;
            }

            if (value is IList list && type.IsGenericType)
            {
                var elemType = type.GetGenericArguments()[0];
                bool remove = RemoveLists.Contains(f.Name);
                int changed = 0;
                for (int i = list.Count - 1; i >= 0; i--)
                {
                    var item = list[i];
                    if (item == null) continue;
                    if (!elemType.IsEnum && CustomRefWalker.IsCustomFurnitureEntry(item, orphansOnly: true))
                    {
                        // A piece whose furniture is no longer installed would stop the whole load (unknown object type).
                        list.RemoveAt(i);
                        changed++;
                        continue;
                    }
                    bool orphan = elemType.IsEnum
                        ? CustomRefWalker.Walk(new EnumBox(item), CustomRefWalker.Mode.Detect, orphansOnly: true)
                        : CustomRefWalker.Walk(item, CustomRefWalker.Mode.Detect, orphansOnly: true);
                    if (!orphan) continue;
                    changed++;
                    if (remove) list.RemoveAt(i);
                    else if (elemType.IsEnum) list[i] = Enum.ToObject(elemType, elemType == typeof(EItemType) ? (int)EItemType.None : 0);
                    else CustomRefWalker.Walk(item, CustomRefWalker.Mode.Neutralize, orphansOnly: true);
                }
                return changed;
            }

            if (type.IsClass && !(value is string) && CustomRefWalker.Walk(value, CustomRefWalker.Mode.Detect, orphansOnly: true))
            {
                CustomRefWalker.Walk(value, CustomRefWalker.Mode.Neutralize, orphansOnly: true);
                return 1;
            }
            return 0;
        }

        /// <summary>Queued deliveries pointing past the restock table (a removed custom pack) are dropped from both parallel lists.</summary>
        private static int CleanSpawnWaiting()
        {
            var rows = CPlayerData.m_SpawnBoxRestockIndexWaitingList;
            var counts = CPlayerData.m_SpawnBoxItemCountWaitingList;
            var so = CSingleton<InventoryBase>.Instance.m_StockItemData_SO;
            if (rows == null || so == null) return 0;
            int changed = 0;
            for (int i = rows.Count - 1; i >= 0; i--)
            {
                if (rows[i] >= 0 && rows[i] < so.m_RestockDataList.Count) continue;
                rows.RemoveAt(i);
                if (counts != null && i < counts.Count) counts.RemoveAt(i);
                changed++;
            }
            return changed;
        }

        // ------------------------------------------------------------------ restore (load)

        public static void Restore(Stash stash)
        {
            if (stash == null || stash.entries.Count == 0) return;
            var alloc = stash.alloc ?? new AllocationSnapshot();
            int ok = 0, dropped = 0;
            // Warehouse shelves taken out at save (saved indices, ascending) and those that couldn't be put back: stored boxes refer to
            // shelves by index (see StripWarehouseBoxes).
            var shelves = new List<int>();
            var lostShelves = new HashSet<int>();
            bool boxesRemapped = false;
            // Pieces go back to their saved index first (ascending, so the list is as saved), then replace entries (indices refer to
            // the saved list), then removes are appended.
            foreach (var e in stash.entries.OrderBy(e => e.mode == "removeAt" ? 0 : e.mode == "replace" ? 1 : 2).ThenBy(e => e.index))
            {
                if (!boxesRemapped && e.mode != "removeAt") { RemapStoredBoxes(shelves, lostShelves); boxesRemapped = true; }
                bool restored = false;
                try
                {
                    restored = RestoreOne(e, alloc, shelves, lostShelves);
                    if (restored) ok++;
                    else { dropped++; Plugin.Log.LogWarning($"Restore: dropped {e.field}[{e.index}] (its custom set/card/pack/furniture is no longer installed)"); }
                }
                catch (Exception ex)
                {
                    dropped++;
                    Plugin.Log.LogError($"Restore failed for {e.field}[{e.index}]: {ex.Message}");
                }
                if (e.mode == "removeAt" && e.field == WarehouseField)
                {
                    shelves.Add(e.index);
                    if (!restored) lostShelves.Add(e.index);
                }
            }
            if (!boxesRemapped) RemapStoredBoxes(shelves, lostShelves);
            Plugin.Log.LogInfo($"Load: restored {ok} custom entries into the game{(dropped > 0 ? $", dropped {dropped}" : "")}");
        }

        private static bool RestoreOne(StashEntry e, AllocationSnapshot alloc, List<int> shelves, HashSet<int> lostShelves)
        {
            if (e.mode == "spawn")
            {
                var parts = e.json.Split(',');
                if (!alloc.RestockRow(int.Parse(parts[0]), out int row)) return false;
                CPlayerData.m_SpawnBoxRestockIndexWaitingList.Add(row);
                CPlayerData.m_SpawnBoxItemCountWaitingList.Add(int.Parse(parts[1]));
                return true;
            }

            var target = typeof(CPlayerData).GetField(e.field, BindingFlags.Static | BindingFlags.Public);
            if (target == null) { Plugin.Log.LogWarning($"Restore: CPlayerData.{e.field} not found"); return false; }

            if (e.mode == "decoCount")
            {
                var parts = e.json.Split(',');
                if (!alloc.Deco(int.Parse(parts[0]), out int deco) || !(target.GetValue(null) is List<int> owned)) return false;
                while (owned.Count <= deco) owned.Add(0);
                owned[deco] += int.Parse(parts[1]);
                return true;
            }
            if (e.mode == "surfaceUnlock" || e.mode == "surfaceEquip")
            {
                if (!(SurfaceKind(e.field) is Core.DecorationKind kind) || !alloc.Surface(kind, int.Parse(e.json), out int index)) return false;
                if (e.mode == "surfaceEquip") { target.SetValue(null, index); return true; }
                if (!(target.GetValue(null) is List<bool> unlocked)) return false;
                while (unlocked.Count <= index) unlocked.Add(false);
                unlocked[index] = true;
                return true;
            }

            if (e.type == RarityType)
            {
                var parts = e.json.Split(':');
                var set = parts.Length == 2 ? Registry.Get(parts[0]) : null;
                if (set == null || !set.Def.RarityById.TryGetValue(parts[1], out var rarity)) return false;
                target.SetValue(null, rarity.Value);
                return true;
            }

            if (e.type == "monster")
            {
                if (!alloc.Monster(int.Parse(e.json), out int m)) return false;
                ((List<int>)target.GetValue(null)).Add(m);
                return true;
            }

            var type = Type.GetType(e.type);
            if (type == null) return false;
            object obj;
            if (type.IsEnum)
            {
                int v = int.Parse(e.json);
                bool mapped = type == typeof(ECardExpansionType) ? alloc.Expansion(v, out v)
                            : type == typeof(EMonsterType) ? alloc.Monster(v, out v)
                            : type == typeof(EItemType) ? alloc.Item(v, out v)
                            : type == typeof(EDecoObject) ? alloc.Deco(v, out v) : true;
                if (!mapped) return false;
                obj = Enum.ToObject(type, v);
            }
            else
            {
                obj = JsonUtility.FromJson(e.json, type);
                if (!CustomRefWalker.Walk(obj, CustomRefWalker.Mode.Remap, alloc)) return false;
                if (obj is PackageBoxItemaveData box && box.isStored)
                    SetStoredShelf(box, e.mode == "warehouseBox" ? box.storedWarehouseShelfIndex : SavedToFull(box.storedWarehouseShelfIndex, shelves), lostShelves);
            }

            switch (e.mode)
            {
                case "scalar":
                    target.SetValue(null, obj);
                    return true;
                case "replace":
                    var list = (IList)target.GetValue(null);
                    if (list == null || e.index < 0 || e.index >= list.Count) return false;
                    list[e.index] = obj;
                    return true;
                case "remove":
                case "warehouseBox":
                    ((IList)target.GetValue(null))?.Add(obj);
                    return true;
                case "removeAt":
                    var l = (IList)target.GetValue(null);
                    if (l == null) return false;
                    l.Insert(Mathf.Clamp(e.index, 0, l.Count), obj);
                    return true;
            }
            return false;
        }
    }
}

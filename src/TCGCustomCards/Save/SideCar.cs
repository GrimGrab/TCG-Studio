using System;
using System.Collections.Generic;
using System.IO;
using Newtonsoft.Json;
using TCGCustomCards.Runtime;
using UnityEngine;

namespace TCGCustomCards.Save
{
    /// <summary>
    /// The mod's own per-slot save: persistentDataPath\TCGCustomCards\slot{n}.json.
    /// Keyed by stable ids (set → card → variant), never by runtime ints or list positions,
    /// so editing/reordering a set never scrambles what the player owns. Unknown entries are kept.
    /// </summary>
    internal static class SideCar
    {
        public const int FormatVersion = 1;

        private class SaveFile
        {
            public int version = FormatVersion;
            public Dictionary<string, Dictionary<string, Dictionary<string, VariantSave>>> sets =
                new Dictionary<string, Dictionary<string, Dictionary<string, VariantSave>>>();
            /// <summary>Restock license flags keyed "set/pack/Row" (rows: Pack, PackBig, Box, BoxBig).</summary>
            public Dictionary<string, bool> licenses = new Dictionary<string, bool>();
            /// <summary>Economy state of custom items keyed "set/pack/Pack|Box" (CPlayerData lists indexed by EItemType).</summary>
            public Dictionary<string, ItemSave> items = new Dictionary<string, ItemSave>();
            /// <summary>Custom entries moved out of the vanilla save (restored before shelves spawn).</summary>
            public SaveStripper.Stash stash;
            /// <summary>MTG decks of the MTG deck builder (cards in them are out of the collection counts above).</summary>
            public List<Runtime.Mtg.MtgDeckSave> mtgDecks;
            public int activeMtgDeck = -1;
        }

        /// <summary>Stash produced by the save strip currently in progress; written with the side-car.</summary>
        internal static SaveStripper.Stash PendingStash;

        /// <summary>Stash of the side-car read for the save being loaded (consumed once).</summary>
        public static SaveStripper.Stash TakeLoadedStash()
        {
            var s = _pending?.stash;
            if (_pending != null) _pending.stash = null;
            return s;
        }

        private class ItemSave
        {
            public float cost, market, setPrice, pct, avgCost;
            /// <summary>Cost/market settings the generated prices were made from (see DefinitionKey).</summary>
            public string def;
            public int count;
            public List<float> past;
        }

        private class VariantSave
        {
            public int count;
            public float price;
            public float[] graded;
            public float pct;
            public List<float> past;

            [JsonIgnore]
            public bool IsEmpty => count == 0 && price == 0f && pct == 0f && (graded == null || Array.TrueForAll(graded, g => g == 0f)) && (past == null || past.Count == 0);
        }

        /// <summary>Entries for sets/cards not currently installed; written back untouched.</summary>
        private static SaveFile _orphans = new SaveFile();
        private static SaveFile _pending;

        private static string Dir => Path.Combine(Application.persistentDataPath, "TCGCustomCards");
        private static string PathFor(int slot) => Path.Combine(Dir, $"slot{slot}.json");

        /// <summary>Reads a slot's side-car (called when the vanilla save is read). Applied later by <see cref="ApplyPending"/>.</summary>
        public static void Read(int slot) => ReadFile(slot, PathFor(slot));

        /// <summary>
        /// The game fell back to its backup save (savedGames_ReleaseBackupFile{slot}, i.e. the previous save of that slot): read the
        /// matching previous side-car (slot{n}.json.bak) when there is one, else the current one — never keep a stale pending.
        /// </summary>
        public static void ReadBackup(int slot)
        {
            string bak = PathFor(slot) + ".bak";
            ReadFile(slot, File.Exists(bak) ? bak : PathFor(slot));
        }

        /// <summary>
        /// The game copied the autosave (slot 0) into an empty slot (CSaveLoad.AutoSaveMoveToEmptySaveSlot copies only its own files):
        /// copy the side-car along, or that slot would load without its custom cards and MTG decks.
        /// </summary>
        public static void CopySlot(int from, int to)
        {
            try
            {
                string src = PathFor(from), dst = PathFor(to);
                if (!File.Exists(src)) return;
                File.Copy(src, dst, overwrite: true);
                Plugin.Log.LogInfo($"Side-car: copied slot {from} to slot {to} (autosave moved to an empty slot)");
            }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Could not copy side-car slot {from} to {to}: {e.Message}");
            }
        }

        private static void ReadFile(int slot, string path)
        {
            _pending = null;
            if (!File.Exists(path)) { _pending = new SaveFile(); return; }
            try
            {
                _pending = JsonConvert.DeserializeObject<SaveFile>(File.ReadAllText(path)) ?? new SaveFile();
                Plugin.Log.LogInfo($"Side-car: read slot {slot} ({System.IO.Path.GetFileName(path)}), {(_pending.mtgDecks?.Count ?? 0)} MTG deck(s)");
            }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Could not read {path}: {e.Message}. Custom card data for this slot starts empty; the file is left untouched.");
                File.Copy(path, path + ".corrupt", overwrite: true);
                _pending = new SaveFile();
            }
        }

        /// <summary>Pushes the pending side-car into the card stores (after vanilla data has been propagated).</summary>
        public static void ApplyPending()
        {
            if (_pending == null) return;
            Registry.ResetAllPlayerState();
            _orphans = new SaveFile();
            int applied = 0;

            foreach (var setKv in _pending.sets)
            {
                var set = Registry.Get(setKv.Key);
                foreach (var cardKv in setKv.Value)
                {
                    if (set == null || !set.PosByCardId.TryGetValue(cardKv.Key, out int pos))
                    {
                        Orphan(setKv.Key, cardKv.Key, cardKv.Value);
                        continue;
                    }
                    foreach (var varKv in cardKv.Value)
                    {
                        if (!TryParseVariant(varKv.Key, out var border, out bool foil))
                        {
                            Orphan(setKv.Key, cardKv.Key, new Dictionary<string, VariantSave> { [varKv.Key] = varKv.Value });
                            continue;
                        }
                        int slot = CardStore.Slot(pos, border, foil);
                        var v = varKv.Value;
                        var st = set.Store;
                        st.Counts[slot] = v.count;
                        st.Collected[slot] = v.count > 0;
                        st.PriceSet[slot] = v.price;
                        if (v.graded != null)
                            for (int g = 0; g < 10 && g < v.graded.Length; g++) st.GradedPriceSet[slot].floatDataList[g] = v.graded[g];
                        st.Market[slot].pricePercentChangeList = v.pct;
                        st.Market[slot].pastPricePercentChangeList = v.past ?? new List<float>();
                        applied++;
                    }
                }
            }
            ApplyLicenses(_pending.licenses);
            ApplyItems(_pending.items, keepCountsWhenMissing: true);
            Runtime.Mtg.MtgDeckStore.Load(_pending.mtgDecks, _pending.activeMtgDeck);
            Plugin.Log.LogInfo($"MTG decks loaded: {Runtime.Mtg.MtgDeckStore.Decks.Count} (active {Runtime.Mtg.MtgDeckStore.Active})");
            _pending = null;
            Plugin.Log.LogInfo($"Side-car applied: {applied} card variants restored");
        }

        /// <summary>New game / default data: empty stores and forget orphans from a previously loaded slot.</summary>
        public static void ResetForNewGame()
        {
            Registry.ResetAllPlayerState();
            _orphans = new SaveFile();
            ApplyLicenses(new Dictionary<string, bool>());
            ApplyItems(new Dictionary<string, ItemSave>(), keepCountsWhenMissing: false);
            Runtime.Mtg.MtgDeckStore.Load(null, -1);
        }

        /// <summary>Restores custom item prices; items without an entry get zeroed prices so RestockManager.Init regenerates them from the set definition.</summary>
        private static void ApplyItems(Dictionary<string, ItemSave> saved, bool keepCountsWhenMissing)
        {
            ItemInjector.EnsurePlayerListSizes();
            foreach (var (key, item, def) in CustomItems())
                {
                    int i = (int)item;
                    if (item == EItemType.None || i >= CPlayerData.m_GeneratedCostPriceList.Count) continue;
                    if (saved != null && saved.TryGetValue(key, out var s))
                    {
                        // Generated prices are kept only while the set's cost/market settings are unchanged;
                        // after an edit (e.g. TCG Studio Gamify) they are regenerated from the new definition.
                        bool defChanged = s.def != def; // also true for side-cars written before fingerprints existed
                        CPlayerData.m_GeneratedCostPriceList[i] = defChanged ? 0f : s.cost;
                        CPlayerData.m_GeneratedMarketPriceList[i] = defChanged ? 0f : s.market;
                        CPlayerData.m_SetItemPriceList[i] = s.setPrice;
                        CPlayerData.m_ItemPricePercentChangeList[i] = s.pct;
                        CPlayerData.m_AverageItemCostList[i] = s.avgCost;
                        CPlayerData.m_CurrentTotalItemCountList[i] = s.count;
                        CPlayerData.m_ItemPricePercentPastChangeList[i].floatDataList = s.past ?? new List<float>();
                    }
                    else
                    {
                        CPlayerData.m_GeneratedCostPriceList[i] = 0f;
                        CPlayerData.m_GeneratedMarketPriceList[i] = 0f;
                        CPlayerData.m_ItemPricePercentChangeList[i] = 0f;
                        if (!keepCountsWhenMissing) CPlayerData.m_CurrentTotalItemCountList[i] = 0;
                    }
                }
            if (saved != null)
                foreach (var kv in saved)
                    if (!_knownItemKeys.Contains(kv.Key)) _orphans.items[kv.Key] = kv.Value;
        }

        /// <summary>Fingerprint of the pack definition's pricing inputs; boxes follow the pack cost unless they set their own.</summary>
        private static string DefinitionKey(CustomPack pack, bool box)
        {
            var d = pack.Def;
            string cost = box && d.BoxCost.HasValue ? $"box{d.BoxCost.Value:0.###}" : $"pack{d.PackCost:0.###}";
            return System.FormattableString.Invariant($"{cost}|{d.MarketMin:0.###}-{d.MarketMax:0.###}");
        }

        /// <summary>Fingerprint of an accessory's pricing inputs (cost/market after defaults from the base item).</summary>
        /// <remarks>Taken from the injected ItemData (<see cref="CustomAccessory.DefKey"/>): the save on quit runs after the game may
        /// already have destroyed InventoryBase, so the side-car must not look anything up there.</remarks>
        private static string DefinitionKey(CustomAccessory acc) => acc.DefKey ?? "";

        internal static string DefinitionKey(ItemData d) =>
            System.FormattableString.Invariant($"acc{d.baseCost:0.###}|{d.marketPriceMinPercent:0.###}-{d.marketPriceMaxPercent:0.###}");

        /// <summary>Every custom item with its side-car key and pricing fingerprint (pack + box items, then accessories).</summary>
        private static IEnumerable<(string key, EItemType item, string def)> CustomItems()
        {
            foreach (var pack in Registry.Packs)
            {
                yield return (pack.ItemKey(false), pack.PackItem, DefinitionKey(pack, false));
                yield return (pack.ItemKey(true), pack.BoxItem, DefinitionKey(pack, true));
            }
            foreach (var acc in Registry.Accessories)
                if (acc.Item != EItemType.None) yield return (acc.ItemKey, acc.Item, DefinitionKey(acc));
        }

        /// <summary>Every custom restock row with its side-car key and default (starter packs: small-pack row unlocked).</summary>
        private static IEnumerable<(string key, int row, bool unlockedByDefault)> CustomLicenseRows()
        {
            foreach (var pack in Registry.Packs)
                for (int r = 0; r < 4; r++)
                    yield return (pack.LicenseKey((CustomPack.Row)r), pack.RestockRows[r], pack.Def.Starter && r == (int)CustomPack.Row.Pack);
            foreach (var acc in Registry.Accessories)
                for (int r = 0; r < acc.RestockRows.Count; r++)
                    yield return (acc.LicenseKey(r), acc.RestockRows[r], false);
        }

        private static HashSet<string> _knownItemKeys
        {
            get
            {
                var keys = new HashSet<string>(System.Linq.Enumerable.SelectMany(Registry.Packs, p => new[] { p.ItemKey(false), p.ItemKey(true) }));
                foreach (var acc in Registry.Accessories) keys.Add(acc.ItemKey);
                return keys;
            }
        }

        /// <summary>Sets custom restock rows' license flags from saved values; missing entries default to unlocked only for a starter pack's small-pack row.</summary>
        private static void ApplyLicenses(Dictionary<string, bool> saved)
        {
            ItemInjector.EnsurePlayerListSizes();
            var list = CPlayerData.m_IsItemLicenseUnlocked;
            var known = new HashSet<string>();
            foreach (var (key, idx, unlockedByDefault) in CustomLicenseRows())
            {
                known.Add(key);
                if (idx < 0 || idx >= list.Count) continue;
                list[idx] = saved != null && saved.TryGetValue(key, out bool v) ? v : unlockedByDefault;
            }
            if (saved != null)
                foreach (var kv in saved)
                    if (!known.Contains(kv.Key)) _orphans.licenses[kv.Key] = kv.Value;
        }

        public static void Write(int slot)
        {
            var file = new SaveFile { stash = PendingStash, mtgDecks = Runtime.Mtg.MtgDeckStore.Export(), activeMtgDeck = Runtime.Mtg.MtgDeckStore.Active };
            foreach (var kv in _orphans.sets) file.sets[kv.Key] = new Dictionary<string, Dictionary<string, VariantSave>>(kv.Value);
            foreach (var kv in _orphans.licenses) file.licenses[kv.Key] = kv.Value;
            foreach (var kv in _orphans.items) file.items[kv.Key] = kv.Value;
            foreach (var (key, item, def) in CustomItems())
                {
                    int i = (int)item;
                    if (item == EItemType.None || i >= CPlayerData.m_GeneratedCostPriceList.Count) continue;
                    file.items[key] = new ItemSave
                    {
                        def = def,
                        cost = CPlayerData.m_GeneratedCostPriceList[i],
                        market = CPlayerData.m_GeneratedMarketPriceList[i],
                        setPrice = CPlayerData.m_SetItemPriceList[i],
                        pct = CPlayerData.m_ItemPricePercentChangeList[i],
                        avgCost = CPlayerData.m_AverageItemCostList[i],
                        count = CPlayerData.m_CurrentTotalItemCountList[i],
                        past = i < CPlayerData.m_ItemPricePercentPastChangeList.Count ? CPlayerData.m_ItemPricePercentPastChangeList[i].floatDataList : null
                    };
                }
            var licenseList = CPlayerData.m_IsItemLicenseUnlocked;
            foreach (var (key, idx, _) in CustomLicenseRows())
                if (idx >= 0 && idx < licenseList.Count) file.licenses[key] = licenseList[idx];

            foreach (var set in Registry.Sets)
            {
                if (!file.sets.TryGetValue(set.Def.Id, out var cards))
                    file.sets[set.Def.Id] = cards = new Dictionary<string, Dictionary<string, VariantSave>>();
                var st = set.Store;
                for (int slotIdx = 0; slotIdx < st.SlotCount; slotIdx++)
                {
                    var v = new VariantSave
                    {
                        count = st.Counts[slotIdx],
                        price = st.PriceSet[slotIdx],
                        graded = st.GradedPriceSet[slotIdx].floatDataList.ToArray(),
                        pct = st.Market[slotIdx].pricePercentChangeList,
                        past = st.Market[slotIdx].pastPricePercentChangeList
                    };
                    if (v.IsEmpty) continue;
                    if (Array.TrueForAll(v.graded, g => g == 0f)) v.graded = null;
                    string cardId = set.Card(CardStore.CardPos(slotIdx)).Id;
                    if (!cards.TryGetValue(cardId, out var variants)) cards[cardId] = variants = new Dictionary<string, VariantSave>();
                    variants[PriceModel.VariantKey(CardStore.Border(slotIdx), CardStore.Foil(slotIdx))] = v;
                }
            }

            try
            {
                Directory.CreateDirectory(Dir);
                string path = PathFor(slot), tmp = path + ".tmp";
                File.WriteAllText(tmp, JsonConvert.SerializeObject(file, Formatting.Indented));
                if (File.Exists(path)) File.Replace(tmp, path, path + ".bak");
                else File.Move(tmp, path);
                Plugin.Log.LogInfo($"Side-car: wrote slot {slot}, {file.mtgDecks?.Count ?? 0} MTG deck(s)");
            }
            catch (Exception e)
            {
                Plugin.Log.LogError($"Could not write side-car for slot {slot}: {e}");
            }
        }

        private static void Orphan(string setId, string cardId, Dictionary<string, VariantSave> variants)
        {
            if (!_orphans.sets.TryGetValue(setId, out var cards)) _orphans.sets[setId] = cards = new Dictionary<string, Dictionary<string, VariantSave>>();
            if (!cards.TryGetValue(cardId, out var existing)) cards[cardId] = existing = new Dictionary<string, VariantSave>();
            foreach (var kv in variants) existing[kv.Key] = kv.Value;
        }

        internal static bool TryParseVariant(string key, out ECardBorderType border, out bool foil)
        {
            foil = key.EndsWith("_foil", StringComparison.Ordinal);
            string b = foil ? key.Substring(0, key.Length - 5) : key;
            return Enum.TryParse(b, out border) && (int)border >= 0 && (int)border < CardStore.BordersPerCard;
        }
    }
}

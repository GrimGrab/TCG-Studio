using System.Collections.Generic;
using System.Linq;
using TCGCustomCards.Runtime;

namespace TCGCustomCards.Save
{
    /// <summary>
    /// Which runtime int meant which stable id when a save was written. Runtime ints are reassigned every launch, so data
    /// stashed with old ints is translated old int → stable id → current int on restore.
    /// </summary>
    internal class AllocationSnapshot : CustomRefWalker.IRemapper
    {
        public Dictionary<int, string> expansions = new Dictionary<int, string>();
        public Dictionary<int, string> monsters = new Dictionary<int, string>();
        public Dictionary<int, string> items = new Dictionary<int, string>();
        public Dictionary<int, string> restockRows = new Dictionary<int, string>();
        /// <summary>setId → card ids in set order (cardSaveIndex = pos * 12 + variant).</summary>
        public Dictionary<string, List<string>> cardOrder = new Dictionary<string, List<string>>();

        public static AllocationSnapshot Current()
        {
            var s = new AllocationSnapshot();
            foreach (var set in Registry.Sets)
            {
                s.expansions[(int)set.Expansion] = set.Def.Id;
                s.cardOrder[set.Def.Id] = set.Def.Cards.Select(c => c.Id).ToList();
                for (int pos = 0; pos < set.Shown.Count; pos++) s.monsters[(int)set.Shown[pos]] = $"{set.Def.Id}|{set.Card(pos).Id}";
            }
            foreach (var pack in Registry.Packs)
            {
                if (pack.PackItem != EItemType.None) s.items[(int)pack.PackItem] = pack.ItemKey(false);
                if (pack.BoxItem != EItemType.None) s.items[(int)pack.BoxItem] = pack.ItemKey(true);
                for (int r = 0; r < 4; r++)
                    if (pack.RestockRows[r] >= 0) s.restockRows[pack.RestockRows[r]] = pack.LicenseKey((CustomPack.Row)r);
            }
            foreach (var acc in Registry.Accessories)
            {
                if (acc.Item == EItemType.None) continue;
                s.items[(int)acc.Item] = acc.ItemKey;
                for (int r = 0; r < acc.RestockRows.Count; r++) s.restockRows[acc.RestockRows[r]] = acc.LicenseKey(r);
            }
            return s;
        }

        // ---- IRemapper: this snapshot is the OLD allocation; targets are looked up in the current Registry ----

        private AllocationSnapshot _current;
        private AllocationSnapshot CurrentAlloc => _current ??= Current();
        private static int Find(Dictionary<int, string> map, string key) { foreach (var kv in map) if (kv.Value == key) return kv.Key; return int.MinValue; }

        public bool Expansion(int oldValue, out int newValue) => Translate(expansions, CurrentAlloc.expansions, oldValue, out newValue);
        public bool Monster(int oldValue, out int newValue) => Translate(monsters, CurrentAlloc.monsters, oldValue, out newValue);
        public bool Item(int oldValue, out int newValue) => Translate(items, CurrentAlloc.items, oldValue, out newValue);
        public bool WasCustomItem(int oldValue) => items.ContainsKey(oldValue);
        public bool RestockRow(int oldValue, out int newValue) => Translate(restockRows, CurrentAlloc.restockRows, oldValue, out newValue);

        public bool CardSlot(int oldExpansion, int oldSlot, out int newSlot)
        {
            newSlot = oldSlot;
            if (!expansions.TryGetValue(oldExpansion, out string setId) || !cardOrder.TryGetValue(setId, out var oldOrder)) return false;
            int oldPos = CardStore.CardPos(oldSlot);
            if (oldPos < 0 || oldPos >= oldOrder.Count) return false;
            var set = Registry.Get(setId);
            if (set == null || !set.PosByCardId.TryGetValue(oldOrder[oldPos], out int newPos)) return false;
            newSlot = newPos * CardStore.SlotsPerCard + oldSlot % CardStore.SlotsPerCard;
            return true;
        }

        private static bool Translate(Dictionary<int, string> from, Dictionary<int, string> to, int oldValue, out int newValue)
        {
            newValue = oldValue;
            if (!from.TryGetValue(oldValue, out string key)) return false;
            int v = Find(to, key);
            if (v == int.MinValue) return false;
            newValue = v;
            return true;
        }
    }
}

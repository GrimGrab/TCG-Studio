using System.Collections.Generic;
using TCGCustomCards.Core;

namespace TCGCustomCards.Runtime
{
    /// <summary>A custom booster pack (+ optional box) and its runtime ids. Item ints and restock rows are assigned by <see cref="ItemInjector"/>.</summary>
    internal class CustomPack
    {
        public enum Row { Pack, PackBig, Box, BoxBig }

        public CustomSet Set;
        public PackDef Def;
        public ECollectionPackType PackType;
        public EItemType PackItem = EItemType.None;
        public EItemType BoxItem = EItemType.None;
        /// <summary>Index into m_RestockDataList per <see cref="Row"/> (-1 = not created).</summary>
        public readonly int[] RestockRows = { -1, -1, -1, -1 };

        /// <summary>Stable key for the side-car (set/pack/row).</summary>
        public string LicenseKey(Row row) => $"{Set.Def.Id}/{Def.Id}/{row}";

        /// <summary>Stable key for the side-car item economy entry.</summary>
        public string ItemKey(bool box) => $"{Set.Def.Id}/{Def.Id}/{(box ? "Box" : "Pack")}";

        /// <summary>Card positions (in the set) this pack draws from.</summary>
        public readonly List<int> Pool = new List<int>();
    }
}

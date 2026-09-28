using System.Collections.Generic;
using TCGCustomCards.Core;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// A custom deck box or playmat from the accessory library. Its item int and restock rows are assigned by <see cref="ItemInjector"/>
    /// (ephemeral); saves use the stable keys below.
    /// </summary>
    internal class CustomAccessory
    {
        public AccessoryDef Def;
        public EItemType Item = EItemType.None;
        /// <summary>Restock rows copied from the base item, in the base item's order (deck boxes: small, big; playmats: big).</summary>
        public readonly List<int> RestockRows = new List<int>();
        /// <summary>Row names ("Small"/"Big") parallel to <see cref="RestockRows"/>, used in license keys.</summary>
        public readonly List<string> RowNames = new List<string>();

        public string ItemKey => $"acc/{Def.Id}";
        public string LicenseKey(int row) => $"acc/{Def.Id}/{RowNames[row]}";
    }
}

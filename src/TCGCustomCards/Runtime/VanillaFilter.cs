using System.Collections.Generic;
using System.Linq;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// Custom-only mode. Hiding filters what the game offers; it never deletes anything, and turning it back on restores vanilla.
    ///  • Packs hidden: vanilla card packs/boxes leave the restock lists, customer demand, worker/auto-opener pack list and play-table prizes.
    ///  • Accessories hidden (per kind: deck boxes, playmats, sleeves, dice, comics, collection books, battle decks, figurines): the vanilla items of that kind
    ///    leave the restock lists, the deck picker, play tables and customer demand.
    ///  • Cards hidden: vanilla sets leave the binder and set pickers, screens default to a custom set, trade customers bring custom cards.
    ///  • Furniture hidden: vanilla pieces leave the furniture shop (applied when the shop opens); essential ones stay without a custom one.
    ///  • Decorations hidden (posters / other decorations / wall-floor-ceiling looks): vanilla ones leave the Buy Decoration app;
    ///    owned ones stay in the Decorate screen.
    /// Each toggle is a pure hide: it does not depend on custom content. Where the game needs a replacement (default set, trade
    /// customer, play-table prize) a custom one is used when there is one; otherwise that feature simply has nothing to offer.
    /// </summary>
    internal static class VanillaFilter
    {
        public static bool HideCards => !Plugin.ShowVanillaCards.Value;
        public static bool HidePacks => !Plugin.ShowVanillaPacks.Value;
        /// <summary>Vanilla accessories are hidden per kind ([Content] ShowVanilla&lt;Kind&gt; off). The game copes with an empty category.</summary>
        public static bool HideKind(Core.AccessoryKind k) =>
            Plugin.ShowVanillaAccessory.TryGetValue(k, out var show) && !show.Value;

        public static bool HideAccessories
        {
            get
            {
                foreach (var k in Plugin.ShowVanillaAccessory.Keys) if (HideKind(k)) return true;
                return false;
            }
        }

        /// <summary>
        /// Vanilla decorations leave the "Buy Decoration" app (Poster tab / Other tab / wall-floor-ceiling tabs). The Decorate screen still
        /// lists what you own, placed decorations stay and the default wall/floor/ceiling (index 0) is never hidden (Patches/DecorationPatches).
        /// </summary>
        public static bool HidePosters => Plugin.ShowVanillaPosters != null && !Plugin.ShowVanillaPosters.Value;
        public static bool HideDecoObjects => Plugin.ShowVanillaDecoObjects != null && !Plugin.ShowVanillaDecoObjects.Value;
        public static bool HideSurfaces => Plugin.ShowVanillaSurfaces != null && !Plugin.ShowVanillaSurfaces.Value;

        /// <summary>Vanilla furniture leaves the furniture shop (placed pieces stay).</summary>
        public static bool HideFurniture => Plugin.ShowVanillaFurniture != null && !Plugin.ShowVanillaFurniture.Value;

        /// <summary>
        /// Hidden from the furniture shop right now: a vanilla piece, unless it is an essential type (cash counter, workbench, trash bin,
        /// empty box storage) with no custom piece of that type.
        /// </summary>
        public static bool IsHiddenFurniture(EObjectType t)
        {
            if (!HideFurniture || t < 0 || t >= EObjectType.MAX) return false;
            var prefab = InventoryBase.GetSpawnInteractableObjectPrefab(t);
            foreach (Core.FurnitureType type in System.Enum.GetValues(typeof(Core.FurnitureType)))
            {
                if (!Core.FurnitureKinds.Matches(type, prefab)) continue;
                if (Core.FurnitureKinds.IsEssential(type) && !Registry.Furniture.Exists(f => f.Prefab != null && f.Def.Type == type)) return false;
                break;
            }
            return true;
        }

        /// <summary>
        /// A vanilla set while vanilla cards are hidden → the first custom set. False when nothing needs changing or there is no
        /// custom set to switch to (the screen then stays where it is; its vanilla buttons are still hidden).
        /// </summary>
        public static bool TryRedirect(ECardExpansionType current, out ECardExpansionType custom)
        {
            custom = current;
            if (!HideCards || Registry.IsCustom(current) || Registry.Sets.Count == 0) return false;
            custom = Registry.Sets[0].Expansion;
            return true;
        }

        private static StockItemData_ScriptableObject _so;
        private static List<EItemType> _origShown, _origShownAll, _origPackList, _origShownAccessory, _origShownFigurine;
        private static readonly HashSet<EItemType> VanillaCardProducts = new HashSet<EItemType>();

        /// <summary>Called by ItemInjector after custom items are added: remembers the full lists so filtering is reversible.</summary>
        public static void Capture(StockItemData_ScriptableObject so)
        {
            _so = so;
            _origShown = new List<EItemType>(so.m_ShownItemType);
            _origShownAll = new List<EItemType>(so.m_ShownAllItemType);
            _origPackList = new List<EItemType>(so.m_CardPackItemTypeList);
            _origShownAccessory = new List<EItemType>(so.m_ShownAccessoryItemType);
            _origShownFigurine = new List<EItemType>(so.m_ShownFigurineItemType);
            VanillaCardProducts.Clear();
            for (int i = 0; i < (int)EItemType.Max; i++)
                if (InventoryBase.ItemTypeToCollectionPackType((EItemType)i) != ECollectionPackType.None)
                    VanillaCardProducts.Add((EItemType)i);
            Apply();
        }

        public static bool IsVanillaCardProduct(EItemType t) => VanillaCardProducts.Contains(t);

        /// <summary>Vanilla accessory whose kind is currently hidden.</summary>
        public static bool IsVanillaAccessory(EItemType t)
        {
            var k = Core.AccessoryKinds.KindOf(t);
            return k.HasValue && HideKind(k.Value);
        }

        /// <summary>Hidden right now (used by the shop lists, customer demand and the category lookups).</summary>
        public static bool IsHidden(EItemType t) => (HidePacks && IsVanillaCardProduct(t)) || IsVanillaAccessory(t);

        /// <summary>Rebuilds the shared item lists in place (other objects hold references to them).</summary>
        public static void Apply()
        {
            if (_so == null) return;
            Refill(_so.m_ShownItemType, _origShown);
            Refill(_so.m_ShownAllItemType, _origShownAll);
            Refill(_so.m_CardPackItemTypeList, _origPackList);
            Refill(_so.m_ShownAccessoryItemType, _origShownAccessory);
            Refill(_so.m_ShownFigurineItemType, _origShownFigurine);
            ApplyPlayerDefaults();
            // The deck box / playmat picker caches its lists until a license is bought; make it rebuild them.
            GameInstance.m_IsItemLicenseUnlocked = true;
        }

        private static void Refill(List<EItemType> target, List<EItemType> original)
        {
            target.Clear();
            target.AddRange(original.Where(t => !IsHidden(t)));
        }

        /// <summary>Saved "last used set" of workbench / quick-fill screens → a custom set while vanilla cards are hidden.</summary>
        public static void ApplyPlayerDefaults()
        {
            if (TryRedirect(CPlayerData.m_WorkbenchCardExpansionType, out var e)) CPlayerData.m_WorkbenchCardExpansionType = e;
            if (TryRedirect(CPlayerData.m_QuickFillCardExpansionType, out e)) CPlayerData.m_QuickFillCardExpansionType = e;
            if (TryRedirect(CPlayerData.m_DonationQuickFillCardExpansionType, out e)) CPlayerData.m_DonationQuickFillCardExpansionType = e;
        }

        /// <summary>
        /// Replacement for the play-table prize's hardcoded AscensionCardPack: a pack still on offer. Only reached while the (filtered)
        /// card-pack list has entries; an empty list skips the prize (PlayTableNoGiftWithoutPacks).
        /// </summary>
        public static EItemType GiftPack()
        {
            if (!HidePacks) return EItemType.AscensionCardPack;
            var packs = _so != null ? _so.m_CardPackItemTypeList : null;
            return packs != null && packs.Count > 0 ? packs[Random.Range(0, packs.Count)] : EItemType.AscensionCardPack;
        }

        /// <summary>No set a trade/sell customer could bring: vanilla cards hidden and no custom set.</summary>
        public static bool NoTradeCards => HideCards && Registry.Sets.Count == 0;

        /// <summary>Expansion a trade/sell customer brings: a random custom set while vanilla cards are hidden.</summary>
        public static ECardExpansionType TradeExpansion(ECardExpansionType rolled)
        {
            if (!HideCards || Registry.Sets.Count == 0) return rolled;
            return Registry.Sets[Random.Range(0, Registry.Sets.Count)].Expansion;
        }
    }
}

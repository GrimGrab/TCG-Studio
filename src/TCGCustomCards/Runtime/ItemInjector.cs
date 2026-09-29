using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Reflection;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>
    /// Adds custom pack/box items to the (shared) StockItemData ScriptableObject: ItemData + ItemMeshData at index == (int)EItemType,
    /// four RestockData rows per pack, shown-item lists and the card-pack list used by workers/auto opener/play tables.
    /// Runs once per session, as early as possible in the "Start" scene (several game objects copy these lists in Awake).
    /// </summary>
    internal static class ItemInjector
    {
        private static StockItemData_ScriptableObject _injectedInto;
        public static bool Injected => _injectedInto != null;
        /// <summary>ItemData objects we created (their names are plain text, not I2 terms).</summary>
        public static readonly HashSet<ItemData> CustomItemData = new HashSet<ItemData>();

        public static void EnsureInjected()
        {
            if (Registry.Packs.Count == 0 && Registry.Accessories.Count == 0) return;
            // Never touch CSingleton<InventoryBase>.Instance outside the shop scene: it would create an empty manager.
            var inv = Object.FindObjectOfType<InventoryBase>();
            if (inv == null || inv.m_StockItemData_SO == null) return;
            var so = inv.m_StockItemData_SO;
            if (_injectedInto == so) return;
            _injectedInto = so;

            int nextItem = Mathf.Max(so.m_ItemDataList.Count, so.m_ItemMeshDataList.Count, (int)EItemType.Max);
            PadTo(so.m_ItemDataList, nextItem, i => Clone(so.m_ItemDataList[(int)EItemType.BasicCardPack]));
            PadTo(so.m_ItemMeshDataList, nextItem, i => so.m_ItemMeshDataList[(int)EItemType.BasicCardPack]);

            var vPackData = so.m_ItemDataList[(int)EItemType.BasicCardPack];
            var vBoxData = so.m_ItemDataList[(int)EItemType.BasicCardBox];
            var vPackMesh = so.m_ItemMeshDataList[(int)EItemType.BasicCardPack];
            var vBoxMesh = so.m_ItemMeshDataList[(int)EItemType.BasicCardBox];
            var vRows = FindVanillaRows(so);

            foreach (var pack in Registry.Packs)
            {
                var def = pack.Def;
                string folder = pack.Set.Def.FolderPath;

                // --- Pack item ---
                pack.PackItem = (EItemType)nextItem++;
                var packData = Clone(vPackData);
                packData.name = def.Name;
                packData.icon = Icon(folder, def.PackIcon, vPackData.icon) ?? vPackData.icon;
                packData.baseCost = def.PackCost;
                packData.marketPriceMinPercent = def.MarketMin;
                packData.marketPriceMaxPercent = def.MarketMax;
                packData.boxFollowItemPrice = EItemType.None;
                packData.isHideItemUntilUnlocked = false;
                AddItem(so, pack.PackItem, packData, MeshData(vPackMesh, def.Name, folder, def.PackTexture));
                Registry.RegisterItem(pack.PackItem, pack);

                // --- Box item (pack int < box int so the box price can follow the pack price) ---
                if (def.HasBox)
                {
                    pack.BoxItem = (EItemType)nextItem++;
                    var boxData = Clone(vBoxData);
                    boxData.name = def.BoxName;
                    boxData.icon = Icon(folder, def.BoxIcon, vBoxData.icon) ?? vBoxData.icon;
                    boxData.isHideItemUntilUnlocked = false;
                    if (def.BoxCost.HasValue)
                    {
                        boxData.boxFollowItemPrice = EItemType.None;
                        boxData.baseCost = def.BoxCost.Value;
                        boxData.marketPriceMinPercent = def.MarketMin;
                        boxData.marketPriceMaxPercent = def.MarketMax;
                    }
                    else boxData.boxFollowItemPrice = pack.PackItem;
                    AddItem(so, pack.BoxItem, boxData, MeshData(vBoxMesh, def.BoxName, folder, def.BoxTexture));
                    Registry.RegisterItem(pack.BoxItem, pack);
                }

                // --- Restock rows (small/big box for pack and box, like vanilla) ---
                var lic = def.License;
                pack.RestockRows[(int)CustomPack.Row.Pack] = AddRow(so, vRows[0], pack.PackItem, def.Name, lic.PackLevel, lic.PackPrice);
                pack.RestockRows[(int)CustomPack.Row.PackBig] = AddRow(so, vRows[1], pack.PackItem, def.Name,
                    lic.PackBigLevel ?? lic.PackLevel + 1, lic.PackBigPrice ?? Mathf.Round(lic.PackPrice * 1.5f));
                if (def.HasBox)
                {
                    pack.RestockRows[(int)CustomPack.Row.Box] = AddRow(so, vRows[2], pack.BoxItem, def.BoxName, lic.BoxLevel, lic.BoxPrice);
                    pack.RestockRows[(int)CustomPack.Row.BoxBig] = AddRow(so, vRows[3], pack.BoxItem, def.BoxName,
                        lic.BoxBigLevel ?? lic.BoxLevel + 1, lic.BoxBigPrice ?? Mathf.Round(lic.BoxPrice * 1.5f));
                }

                so.m_CardPackItemTypeList.Add(pack.PackItem);
            }

            // Shop order = order in the shown lists (RestockItemScreen default sort). Allocation order above is by set id,
            // so list products by progression instead: pack license level, then license price (stable for ties).
            foreach (var pack in Registry.Packs.OrderBy(p => p.Def.License.PackLevel).ThenBy(p => p.Def.License.PackPrice))
            {
                foreach (var shown in new[] { so.m_ShownItemType, so.m_ShownAllItemType })
                {
                    shown.Add(pack.PackItem);
                    if (pack.Def.HasBox) shown.Add(pack.BoxItem);
                }
            }

            InjectAccessories(so, ref nextItem);

            EnsurePlayerListSizes();
            VanillaFilter.Capture(so);
            Plugin.Log.LogInfo($"Injected {Registry.Packs.Count} custom pack(s) and {Registry.Accessories.Count(a => a.Item != EItemType.None)} accessor(ies): " +
                               $"items up to {nextItem - 1}, restock rows up to {so.m_RestockDataList.Count - 1}");
        }

        /// <summary>
        /// Custom accessories: clones of the base item (model, category, box size, restock rows) with their own texture, icon, name,
        /// prices and licenses. The game finds deck boxes, playmats and comics by category (deck picker, play tables), so no further
        /// patches are needed to use them; sleeves, dice and collection books are plain shop products.
        /// </summary>
        private static void InjectAccessories(StockItemData_ScriptableObject so, ref int nextItem)
        {
            foreach (var acc in Registry.Accessories)
            {
                var def = acc.Def;
                var vData = so.m_ItemDataList[(int)def.BaseItem];
                var vMesh = so.m_ItemMeshDataList[(int)def.BaseItem];
                var vRows = so.m_RestockDataList.Where(r => r.itemType == def.BaseItem).OrderBy(r => r.isBigBox).ToList();
                if (vRows.Count == 0)
                {
                    Plugin.Log.LogError($"Accessory '{def.Id}': base {def.BaseItem} has no restock rows — skipped");
                    continue;
                }

                acc.Item = (EItemType)nextItem++;
                var data = Clone(vData);
                data.name = def.Name;
                data.icon = Icon(def.FolderPath, def.Icon, vData.icon) ?? vData.icon;
                if (def.Cost.HasValue) data.baseCost = def.Cost.Value;
                if (def.MarketMin.HasValue) data.marketPriceMinPercent = def.MarketMin.Value;
                if (def.MarketMax.HasValue) data.marketPriceMaxPercent = def.MarketMax.Value;
                data.boxFollowItemPrice = EItemType.None;
                data.isHideItemUntilUnlocked = false;
                AddItem(so, acc.Item, data, AccessoryMeshData(vMesh, def));
                Registry.RegisterItem(acc.Item, acc);

                // One row per base row: the first uses level/price, a second (big box) bigLevel/bigPrice.
                var lic = def.License;
                for (int r = 0; r < vRows.Count && r < 2; r++)
                {
                    bool first = r == 0;
                    int level = first ? lic.Level : lic.BigLevel ?? lic.Level + 3;
                    float price = first ? lic.Price : lic.BigPrice ?? Mathf.Round(lic.Price * 2f);
                    acc.RestockRows.Add(AddRow(so, vRows[r], acc.Item, def.Name, level, price));
                    acc.RowNames.Add(vRows[r].isBigBox ? "Big" : "Small");
                }
            }

            // Shop tabs: wherever the base item is listed (accessories, supplies, all), in license order.
            var added = Registry.Accessories.Where(a => a.Item != EItemType.None)
                .OrderBy(a => a.Def.License.Level).ThenBy(a => a.Def.License.Price).ToList();
            foreach (var list in new[] { so.m_ShownItemType, so.m_ShownAccessoryItemType, so.m_ShownFigurineItemType, so.m_ShownBoardGameItemType, so.m_ShownAllItemType })
                foreach (var acc in added)
                    if (list.Contains(acc.Def.BaseItem)) list.Add(acc.Item);
        }

        /// <summary>
        /// Base item's model with our texture on a per-item material copy. The texture replaces every texture property that held the base
        /// texture (shaders may sample it as _MainTex and _BaseMap/_EmissionMap), and every material slot that used the base material.
        /// </summary>
        private static ItemMeshData AccessoryMeshData(ItemMeshData vanilla, Core.AccessoryDef def)
        {
            var md = Clone(vanilla);
            md.materialList = vanilla.materialList != null ? new List<Material>(vanilla.materialList) : null;
            var tex = string.IsNullOrEmpty(def.Texture) ? null : ImageCache.Get(Path.Combine(def.FolderPath, def.Texture))?.texture;

            // Figurines with their own model: one mesh, one material (the base toy's material with our texture); no secondary parts.
            var mesh = string.IsNullOrEmpty(def.Mesh) ? null : MeshLoader.Get(Path.Combine(def.FolderPath, def.Mesh));
            if (mesh != null)
            {
                var baseMat = vanilla.material ?? vanilla.materialList?.Find(m => m != null);
                md.mesh = mesh;
                md.meshSecondary = null;
                md.materialSecondary = null;
                md.materialList = null;
                md.material = baseMat == null ? null : new Material(baseMat) { name = $"TCGCC_{def.Id}", hideFlags = HideFlags.DontUnloadUnusedAsset };
                if (md.material != null)
                {
                    var old = baseMat.mainTexture;
                    if (tex != null)
                        foreach (var prop in md.material.GetTexturePropertyNames())
                            if (md.material.GetTexture(prop) == old && old != null) md.material.SetTexture(prop, tex);
                    md.material.mainTexture = tex != null ? tex : Texture2D.whiteTexture;
                    md.material.mainTextureScale = Vector2.one;
                    md.material.mainTextureOffset = Vector2.zero;
                }
                return md;
            }
            if (tex == null || vanilla.material == null) return md;

            var copies = new Dictionary<Material, Material>();
            Material Swap(Material m)
            {
                if (m == null) return null;
                if (copies.TryGetValue(m, out var c)) return c;
                var old = m.mainTexture;
                if (old == null && m != vanilla.material) return copies[m] = m;
                c = new Material(m) { name = $"TCGCC_{def.Id}_{m.name}", hideFlags = HideFlags.DontUnloadUnusedAsset };
                foreach (var prop in c.GetTexturePropertyNames())
                    if (c.GetTexture(prop) == old && old != null) c.SetTexture(prop, tex);
                c.mainTexture = tex;
                return copies[m] = c;
            }
            // Other slots only switch when they share the base material's texture (e.g. a secondary material with a different print stays).
            var mainTex = vanilla.material.mainTexture;
            Material SwapIfSame(Material m) => m != null && (m == vanilla.material || (mainTex != null && m.mainTexture == mainTex)) ? Swap(m) : m;
            md.material = Swap(vanilla.material);
            md.materialSecondary = SwapIfSame(vanilla.materialSecondary);
            if (md.materialList != null)
                for (int i = 0; i < md.materialList.Count; i++) md.materialList[i] = SwapIfSame(md.materialList[i]);
            return md;
        }

        /// <summary>CPlayerData item lists (indexed by EItemType) and license list (by restock row) must cover our ints.</summary>
        public static void EnsurePlayerListSizes()
        {
            if (_injectedInto == null) return;
            int items = _injectedInto.m_ItemDataList.Count, rows = _injectedInto.m_RestockDataList.Count;
            PadTo(CPlayerData.m_CurrentTotalItemCountList, items, _ => 0);
            PadTo(CPlayerData.m_SetItemPriceList, items, _ => 0f);
            PadTo(CPlayerData.m_AverageItemCostList, items, _ => 0f);
            PadTo(CPlayerData.m_GeneratedCostPriceList, items, _ => 0f);
            PadTo(CPlayerData.m_GeneratedMarketPriceList, items, _ => 0f);
            PadTo(CPlayerData.m_ItemPricePercentChangeList, items, _ => 0f);
            PadTo(CPlayerData.m_ItemPricePercentPastChangeList, items, _ => new FloatList { floatDataList = new List<float>() });
            PadTo(CPlayerData.m_IsItemLicenseUnlocked, rows, _ => false);
        }

        private static void AddItem(StockItemData_ScriptableObject so, EItemType item, ItemData data, ItemMeshData mesh)
        {
            int i = (int)item;
            if (so.m_ItemDataList.Count != i || so.m_ItemMeshDataList.Count != i)
                Plugin.Log.LogError($"Item list out of step at {i} (data={so.m_ItemDataList.Count}, mesh={so.m_ItemMeshDataList.Count})");
            so.m_ItemDataList.Add(data);
            so.m_ItemMeshDataList.Add(mesh);
            CustomItemData.Add(data);
        }

        private static int AddRow(StockItemData_ScriptableObject so, RestockData template, EItemType item, string name, int level, float price)
        {
            var row = Clone(template);
            row.itemType = item;
            row.name = name;
            row.licenseShopLevelRequired = Mathf.Max(1, level);
            row.licensePrice = price;
            row.isHideItemUntilUnlocked = false;
            row.index = so.m_RestockDataList.Count;
            so.m_RestockDataList.Add(row);
            return row.index;
        }

        /// <summary>Vanilla rows 0–3: BasicCardPack small/big, BasicCardBox small/big (amounts 32/64/4/8).</summary>
        private static RestockData[] FindVanillaRows(StockItemData_ScriptableObject so)
        {
            RestockData Find(EItemType t, bool big) => so.m_RestockDataList.Find(r => r.itemType == t && r.isBigBox == big) ?? so.m_RestockDataList[0];
            return new[] { Find(EItemType.BasicCardPack, false), Find(EItemType.BasicCardPack, true), Find(EItemType.BasicCardBox, false), Find(EItemType.BasicCardBox, true) };
        }

        private static ItemMeshData MeshData(ItemMeshData vanilla, string name, string folder, string texturePath)
        {
            var md = Clone(vanilla);
            var tex = string.IsNullOrEmpty(texturePath) ? null : ImageCache.Get(Path.Combine(folder, texturePath))?.texture;
            if (tex != null && vanilla.material != null)
            {
                md.material = new Material(vanilla.material) { name = $"TCGCC_{name}", mainTexture = tex, hideFlags = HideFlags.DontUnloadUnusedAsset };
            }
            md.materialList = vanilla.materialList != null ? new List<Material>(vanilla.materialList) : null;
            return md;
        }

        /// <summary>Custom icon padded to the vanilla icon's shape (vanilla pack icon: 1024² sprite with the pack centered).</summary>
        private static Sprite Icon(string folder, string path, Sprite vanilla)
        {
            if (string.IsNullOrEmpty(path)) return null;
            float aspect = vanilla != null ? vanilla.rect.width / vanilla.rect.height : 0f;
            return ImageCache.GetPadded(Path.Combine(folder, path), aspect);
        }

        private static void PadTo<T>(List<T> list, int count, System.Func<int, T> make)
        {
            if (list == null) return;
            while (list.Count < count) list.Add(make(list.Count));
        }

        /// <summary>Field-by-field copy of a plain [Serializable] data class; lists are copied, Unity objects shared.</summary>
        private static T Clone<T>(T src) where T : new()
        {
            var dst = new T();
            foreach (var f in typeof(T).GetFields(BindingFlags.Public | BindingFlags.NonPublic | BindingFlags.Instance))
            {
                var v = f.GetValue(src);
                if (v is System.Collections.IList l && v.GetType().IsGenericType)
                    v = System.Activator.CreateInstance(v.GetType(), l);
                f.SetValue(dst, v);
            }
            return dst;
        }
    }
}

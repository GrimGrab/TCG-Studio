using System.Collections.Generic;
using System.IO;
using System.Linq;
using TCGCustomCards.Core;
using UnityEngine;

namespace TCGCustomCards.Runtime
{
    /// <summary>A loaded custom set with its runtime ids, game-facing data and storage.</summary>
    internal class CustomSet
    {
        public SetDef Def;
        public ECardExpansionType Expansion;
        public readonly List<EMonsterType> Shown = new List<EMonsterType>();
        public readonly List<MonsterData> Monsters = new List<MonsterData>();
        public readonly Dictionary<EMonsterType, int> PosByMonster = new Dictionary<EMonsterType, int>();
        public readonly Dictionary<string, int> PosByCardId = new Dictionary<string, int>();
        public CardStore Store;

        private CardUISetting _uiSetting;
        private Sprite _cardBack;
        private bool _cardBackResolved;

        public CardDef Card(int pos) => Def.Cards[pos];
        public string ImagePath(CardDef card) => string.IsNullOrEmpty(card.Image) ? null : Path.Combine(Def.FolderPath, card.Image);
        public Sprite CardImage(int pos) => ImageCache.Get(ImagePath(Card(pos)));

        /// <summary>Shallow copy of the template expansion's CardUISetting (frame sprites are shared, read-only).</summary>
        public CardUISetting UISetting
        {
            get
            {
                if (_uiSetting != null) return _uiSetting;
                var t = CSingleton<InventoryBase>.Instance.m_MonsterData_SO.m_CardUISettingList[(int)Def.FrameTemplate];
                _uiSetting = new CardUISetting
                {
                    expansionType = Expansion,
                    openPackCanUseRarity = true,
                    openPackCanHaveDuplicate = t.openPackCanHaveDuplicate,
                    artistNameIndex = 0,
                    iconIndex = 0,
                    bgIndex = 0,
                    previousEvoStageIconExpansion = ECardExpansionType.None,
                    cardUISettingDataList = t.cardUISettingDataList
                };
                return _uiSetting;
            }
        }

        /// <summary>Optional card back shared by every custom set that doesn't set its own (written by TCG Studio).</summary>
        public const string GlobalCardBackFile = "card_back.png";

        /// <summary>Image file used for this set's back (set's own → global), or null for the frame template's back.</summary>
        public string CardBackPath
        {
            get
            {
                if (!_cardBackPathResolved)
                {
                    _cardBackPathResolved = true;
                    string own = string.IsNullOrEmpty(Def.CardBack) ? null : Path.Combine(Def.FolderPath, Def.CardBack);
                    string global = Path.Combine(Plugin.PluginDir, GlobalCardBackFile);
                    _cardBackPath = own != null && File.Exists(own) ? own : File.Exists(global) ? global : null;
                }
                return _cardBackPath;
            }
        }
        private string _cardBackPath;
        private bool _cardBackPathResolved;

        public Sprite CardBack
        {
            get
            {
                if (!_cardBackResolved)
                {
                    // Priority: the set's own back → the global back (<plugin>\card_back.png) → the frame template's back.
                    _cardBackResolved = true;
                    if (!string.IsNullOrEmpty(Def.CardBack)) _cardBack = ImageCache.GetCardBack(Path.Combine(Def.FolderPath, Def.CardBack));
                    if (_cardBack == null) _cardBack = ImageCache.GetCardBack(Path.Combine(Plugin.PluginDir, GlobalCardBackFile));
                }
                return _cardBack ?? CSingleton<InventoryBase>.Instance.m_MonsterData_SO.m_CardBackImageList[(int)Def.FrameTemplate];
            }
        }

        /// <summary>Base market prices from the set definition, written into the store.</summary>
        public void ApplyGeneratedPrices()
        {
            for (int slot = 0; slot < Store.SlotCount; slot++)
                Store.Market[slot].generatedMarketPrice =
                    PriceModel.Compute(Def, Card(CardStore.CardPos(slot)), CardStore.Border(slot), CardStore.Foil(slot));
        }
    }

    /// <summary>
    /// Allocates runtime enum ints for custom content. Ints are ephemeral (recomputed every launch);
    /// everything persisted by the mod uses stable string ids.
    /// </summary>
    internal static class Registry
    {
        public const int ExpansionBase = 100;
        public const int MonsterBase = 1_000_000;
        public const int PackTypeBase = 100;

        public static readonly List<CustomSet> Sets = new List<CustomSet>();
        public static readonly List<CustomPack> Packs = new List<CustomPack>();
        private static readonly Dictionary<int, CustomPack> ByPackType = new Dictionary<int, CustomPack>();
        private static readonly Dictionary<int, CustomPack> ByItem = new Dictionary<int, CustomPack>();

        public static bool IsCustom(ECollectionPackType p) => (int)p >= PackTypeBase;
        public static CustomPack Get(ECollectionPackType p) => ByPackType.TryGetValue((int)p, out var c) ? c : null;
        /// <summary>Pack whose pack or box item is <paramref name="item"/>.</summary>
        public static CustomPack GetByItem(EItemType item) => ByItem.TryGetValue((int)item, out var c) ? c : null;
        public static bool IsCustomPackItem(EItemType item) => ByItem.TryGetValue((int)item, out var c) && c.PackItem == item;
        public static bool IsCustomBoxItem(EItemType item) => ByItem.TryGetValue((int)item, out var c) && c.BoxItem == item;

        /// <summary>Called by ItemInjector once item ints are known.</summary>
        public static void RegisterItem(EItemType item, CustomPack pack) => ByItem[(int)item] = pack;

        /// <summary>Custom deck boxes / playmats (global accessory library, not part of a set).</summary>
        public static readonly List<CustomAccessory> Accessories = new List<CustomAccessory>();
        private static readonly Dictionary<int, CustomAccessory> ByAccessoryItem = new Dictionary<int, CustomAccessory>();
        public static CustomAccessory GetAccessoryByItem(EItemType item) => ByAccessoryItem.TryGetValue((int)item, out var a) ? a : null;
        public static void RegisterItem(EItemType item, CustomAccessory acc) => ByAccessoryItem[(int)item] = acc;
        /// <summary>Any custom item (pack, box, accessory) injected this session.</summary>
        public static bool IsCustomItem(EItemType item) => ByItem.ContainsKey((int)item) || ByAccessoryItem.ContainsKey((int)item);

        public static void BuildAccessories(List<AccessoryDef> defs)
        {
            foreach (var def in defs) Accessories.Add(new CustomAccessory { Def = def });
        }
        private static readonly Dictionary<int, CustomSet> ByExpansion = new Dictionary<int, CustomSet>();
        private static readonly Dictionary<int, CustomSet> ByMonster = new Dictionary<int, CustomSet>();
        private static readonly Dictionary<string, CustomSet> ById = new Dictionary<string, CustomSet>();

        /// <summary>Returned for custom monster ids that no longer exist (removed set/card) so the game never gets null.</summary>
        public static readonly MonsterData MissingMonster = NewMonster((EMonsterType)(MonsterBase - 1), "Missing Card", ERarity.Common);

        public static bool IsCustom(ECardExpansionType e) => (int)e >= ExpansionBase;
        public static bool IsCustom(EMonsterType m) => (int)m >= MonsterBase - 1;

        public static CustomSet Get(ECardExpansionType e) => ByExpansion.TryGetValue((int)e, out var s) ? s : null;
        public static CustomSet Get(string setId) => ById.TryGetValue(setId, out var s) ? s : null;

        public static bool TryGetCard(EMonsterType m, out CustomSet set, out int pos)
        {
            pos = -1;
            if (ByMonster.TryGetValue((int)m, out set) && set.PosByMonster.TryGetValue(m, out pos)) return true;
            set = null;
            return false;
        }

        public static int MaxExpansionInt => Sets.Count == 0 ? -1 : Sets.Max(s => (int)s.Expansion);

        public static void Build(List<SetDef> defs)
        {
            int nextMonster = MonsterBase;
            foreach (var def in defs.OrderBy(d => d.Id, System.StringComparer.Ordinal))
            {
                var set = new CustomSet { Def = def, Expansion = (ECardExpansionType)(ExpansionBase + Sets.Count) };
                for (int pos = 0; pos < def.Cards.Count; pos++)
                {
                    var card = def.Cards[pos];
                    var id = (EMonsterType)nextMonster++;
                    var md = NewMonster(id, card.Name, card.Rarity);
                    md.ArtistName = card.Artist ?? "";
                    md.Description = card.Description ?? "";
                    md.ElementIndex = card.Play.Element;
                    int[] a = card.Play.LaneAttack;
                    // Play uses Fire/Earth/Water/Wind when FireElement != 0, otherwise Str/Vit/Spi/Mag ÷ 2 — fill both so lanes match either way.
                    md.BaseStats = new Stats
                    {
                        FireElement = a[0], EarthElement = a[1], WaterElement = a[2], WindElement = a[3],
                        Strength = a[0] * 2, Vitality = a[1] * 2, Spirit = a[2] * 2, Magic = a[3] * 2
                    };
                    set.Shown.Add(id);
                    set.Monsters.Add(md);
                    set.PosByMonster[id] = pos;
                    set.PosByCardId[card.Id] = pos;
                    ByMonster[(int)id] = set;
                }
                // Evolution links (same set only).
                for (int pos = 0; pos < def.Cards.Count; pos++)
                {
                    string from = def.Cards[pos].Play.EvolvesFrom;
                    if (string.IsNullOrEmpty(from)) continue;
                    int prev = set.PosByCardId[from];
                    set.Monsters[pos].PreviousEvolution = set.Monsters[prev].MonsterType;
                    set.Monsters[prev].NextEvolution = set.Monsters[pos].MonsterType;
                }
                set.Store = new CardStore(def.Cards.Count);
                set.ApplyGeneratedPrices();
                PlayEffects.Build(set);

                foreach (var packDef in def.Packs)
                {
                    var pack = new CustomPack { Set = set, Def = packDef, PackType = (ECollectionPackType)(PackTypeBase + Packs.Count) };
                    if (packDef.Cards.Count == 0) for (int pos = 0; pos < def.Cards.Count; pos++) pack.Pool.Add(pos);
                    else foreach (var id in packDef.Cards) pack.Pool.Add(set.PosByCardId[id]);
                    Packs.Add(pack);
                    ByPackType[(int)pack.PackType] = pack;
                }

                Sets.Add(set);
                ByExpansion[(int)set.Expansion] = set;
                ById[def.Id] = set;
                Plugin.Log.LogInfo($"Registered set '{def.Id}' as expansion {(int)set.Expansion}, monsters {(int)set.Shown.First()}..{(int)set.Shown.Last()}");
            }
        }

        private static MonsterData NewMonster(EMonsterType id, string name, ERarity rarity) => new MonsterData
        {
            Name = name,
            ArtistName = "",
            ArtistNameList = new List<string>(),
            Description = "",
            EffectAmount = Vector3.zero,
            ElementIndex = EElementIndex.Fire,
            Rarity = rarity,
            MonsterType = id,
            NextEvolution = EMonsterType.None,
            PreviousEvolution = EMonsterType.None,
            Roles = new List<EMonsterRole>(),
            BaseStats = new Stats(),
            SkillList = new List<ESkill>(),
            IconList = new List<Sprite>(),
            BGList = new List<Sprite>()
        };

        /// <summary>Resets every store's player state (new game / before applying a side-car).</summary>
        public static void ResetAllPlayerState()
        {
            foreach (var s in Sets) s.Store.ResetPlayerState();
        }
    }
}

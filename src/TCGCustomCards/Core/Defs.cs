using System.Collections.Generic;
using System.IO;
using Newtonsoft.Json;
using Newtonsoft.Json.Converters;

namespace TCGCustomCards.Core
{
    // On-disk content format (set.json). Contract shared with the GUI tool; see schema\set.schema.json.

    public enum RenderMode
    {
        /// <summary>The card image is the whole card (e.g. MTG scans); game frame is hidden.</summary>
        FullImage,
        /// <summary>The card image is artwork placed inside the frame of <see cref="SetDef.FrameTemplate"/>.</summary>
        Framed
    }

    public class SetDef
    {
        public const int CurrentSchemaVersion = 1;

        [JsonProperty("schemaVersion")] public int SchemaVersion = CurrentSchemaVersion;
        [JsonProperty("id")] public string Id;
        [JsonProperty("name")] public string Name;

        [JsonProperty("renderMode"), JsonConverter(typeof(StringEnumConverter))]
        public RenderMode RenderMode = RenderMode.Framed;

        /// <summary>Vanilla expansion whose CardUISetting (frames, borders, foil) and card back are borrowed.</summary>
        [JsonProperty("frameTemplate"), JsonConverter(typeof(StringEnumConverter))]
        public ECardExpansionType FrameTemplate = ECardExpansionType.Tetramon;

        /// <summary>Optional card back image (relative to the set folder).</summary>
        [JsonProperty("cardBack")] public string CardBack;

        /// <summary>Set of real MTG cards (imported from Scryfall): enables Forge deck export (MTG mode).</summary>
        [JsonProperty("mtg")] public SetMtgDef Mtg;

        /// <summary>
        /// The set's rarities, lowest first. Empty = Common, Rare, Epic, Legendary. Cards and pack slot weights name a rarity by its
        /// id; every rarity is a new ERarity value of its own (EPL-style), also those four.
        /// </summary>
        [JsonProperty("rarities")] public List<RarityDef> Rarities = new List<RarityDef>();

        [JsonProperty("priceDefaults")] public PriceDefaults PriceDefaults = new PriceDefaults();
        [JsonProperty("packs")] public List<PackDef> Packs = new List<PackDef>();
        [JsonProperty("cards")] public List<CardDef> Cards = new List<CardDef>();

        /// <summary>Absolute folder the set was loaded from (not serialized).</summary>
        [JsonIgnore] public string FolderPath;

        /// <summary>Rarities by id, case-insensitive (filled by SetLoader).</summary>
        [JsonIgnore] public readonly Dictionary<string, RarityDef> RarityById = new Dictionary<string, RarityDef>(System.StringComparer.OrdinalIgnoreCase);

        public RarityDef RarityOf(CardDef card) => RarityById.TryGetValue(card.Rarity ?? "", out var r) ? r : null;

        /// <summary>
        /// Absolute path of a file the set refers to: the set's own folder first, then the shared card-art library
        /// &lt;plugin&gt;\Library\&lt;set folder name&gt;\ that TCG Studio fills (card art shared by every setup with this set).
        /// Returns the set-folder path when neither exists (callers report it as missing).
        /// </summary>
        /// <summary>
        /// Marker TCG Studio looks for in this DLL (as text) to know the mod reads the shared card-art library; until it finds
        /// it, Studio keeps copying card art into each set folder. Logged at load so the literal stays in the build.
        /// </summary>
        public const string LibraryCapability = "tcgcc-capability:shared-card-art-library";

        public string Resolve(string relative)
        {
            if (string.IsNullOrEmpty(relative)) return null;
            string own = Path.Combine(FolderPath, relative);
            if (File.Exists(own)) return own;
            string plugin = Path.GetDirectoryName(Path.GetDirectoryName(FolderPath));
            string shared = plugin == null ? null : Path.Combine(Path.Combine(plugin, "Library"), Path.Combine(Path.GetFileName(FolderPath), relative));
            return shared != null && File.Exists(shared) ? shared : own;
        }
    }

    public class PriceDefaults
    {
        /// <summary>Multiplier per ECardBorderType (Base, FirstEdition, Silver, Gold, EX, FullArt).</summary>
        [JsonProperty("borderMultipliers")] public float[] BorderMultipliers = { 1f, 1.25f, 1.5f, 2f, 3f, 5f };
        [JsonProperty("foilMultiplier")] public float FoilMultiplier = 2.5f;
        [JsonProperty("minimum")] public float Minimum = 0.05f;
    }

    public class CardDef
    {
        [JsonProperty("id")] public string Id;
        [JsonProperty("name")] public string Name;
        [JsonProperty("description")] public string Description = "";
        [JsonProperty("artist")] public string Artist = "";

        /// <summary>Rarity id from <see cref="SetDef.Rarities"/> (a vanilla rarity name when the set has no list).</summary>
        [JsonProperty("rarity")] public string Rarity = "Common";

        /// <summary>Printed card number (display only).</summary>
        [JsonProperty("number")] public string Number;

        /// <summary>Image path relative to the set folder. FullImage: whole card. Framed: artwork only.</summary>
        [JsonProperty("image")] public string Image;

        [JsonProperty("price")] public CardPrice Price = new CardPrice();
        [JsonProperty("play")] public PlayDef Play = new PlayDef();

        /// <summary>Real MTG card data for Forge deck export; null for non-MTG cards.</summary>
        [JsonProperty("mtg")] public CardMtgDef Mtg;
    }

    /// <summary>One of a set's own rarities (EPL-style: a real rarity of its own in game, with its own pack weights).</summary>
    public class RarityDef
    {
        /// <summary>Id used by cards and pack slots.</summary>
        [JsonProperty("id")] public string Id;
        /// <summary>Shown on the card, binder, Check Price and graded slabs.</summary>
        [JsonProperty("name")] public string Name;
        /// <summary>Optional colour "#RRGGBB" (TCG Studio, rarity picker).</summary>
        [JsonProperty("color")] public string Color;

        /// <summary>Runtime value on MonsterData.Rarity (set by the loader / Registry; not serialized).</summary>
        [JsonIgnore] public ERarity Value;
        /// <summary>Position in the set's list, lowest first.</summary>
        [JsonIgnore] public int Rank;
        /// <summary>
        /// Vanilla rarity for the few vanilla things that only know those (rarity icon, fame, MTG fallback, odds of
        /// packs without slots): by its place in the list. The n-th of up to four is the n-th vanilla rarity; longer lists are
        /// spread evenly over Common…Legendary.
        /// </summary>
        [JsonIgnore] public ERarity Tier;
    }

    public class SetMtgDef
    {
        /// <summary>Forge/Scryfall edition code, upper case (DOM).</summary>
        [JsonProperty("setCode")] public string SetCode;
    }

    public class CardMtgDef
    {
        /// <summary>Name Forge accepts: front face for double-faced/adventure cards, "A // B" for split cards.</summary>
        [JsonProperty("name")] public string Name;
        [JsonProperty("typeLine")] public string TypeLine = "";
        /// <summary>Front-face mana cost, e.g. {1}{B}.</summary>
        [JsonProperty("manaCost")] public string ManaCost = "";
        /// <summary>W U B R G.</summary>
        [JsonProperty("colors")] public List<string> Colors = new List<string>();
        /// <summary>Scryfall rarity (common, uncommon, rare, mythic, special, bonus); empty on older installs.</summary>
        [JsonProperty("rarity")] public string Rarity = "";
        /// <summary>Mana value; 0 on older installs (the deck builder then derives it from ManaCost).</summary>
        [JsonProperty("cmc")] public float Cmc;
        [JsonProperty("power")] public string Power;
        [JsonProperty("toughness")] public string Toughness;
    }

    public class CardPrice
    {
        /// <summary>Market price of the Base, non-foil variant.</summary>
        [JsonProperty("base")] public float Base = 0.25f;
        [JsonProperty("foilMultiplier")] public float? FoilMultiplier;
        [JsonProperty("borderMultipliers")] public float[] BorderMultipliers;
        /// <summary>Exact prices by variant key: "Base", "Gold", "Gold_foil", ...</summary>
        [JsonProperty("overrides")] public Dictionary<string, float> Overrides;
    }

    /// <summary>Container for play-table stats. How these are derived (e.g. from MTG data) is up to the authoring tool.</summary>
    public class PlayDef
    {
        /// <summary>Attack per lane: Fire, Earth, Water, Wind.</summary>
        [JsonProperty("laneAttack")] public int[] LaneAttack = { 1, 1, 1, 1 };

        [JsonProperty("element"), JsonConverter(typeof(StringEnumConverter))]
        public EElementIndex Element = EElementIndex.Fire;

        /// <summary>Card id (same set) this card evolves from; null for a basic card.</summary>
        [JsonProperty("evolvesFrom")] public string EvolvesFrom;

        /// <summary>Optional raw PlayEffectData-shaped object (applied in M5).</summary>
        [JsonProperty("effect")] public Newtonsoft.Json.Linq.JObject Effect;
    }

    public class PackDef
    {
        [JsonProperty("id")] public string Id;
        [JsonProperty("name")] public string Name;
        [JsonProperty("boxName")] public string BoxName;
        [JsonProperty("cardsPerPack")] public int CardsPerPack = 7;
        /// <summary>License unlocked on a new game (and counts for the tutorial box task in custom-only mode).</summary>
        [JsonProperty("starter")] public bool Starter;
        /// <summary>Also sell a box of 8 packs.</summary>
        [JsonProperty("hasBox")] public bool HasBox = true;

        /// <summary>1024² texture in the vanilla card-pack UV layout (TCG Studio's pack editor shows it). Null = vanilla basic pack art.</summary>
        [JsonProperty("packTexture")] public string PackTexture;
        [JsonProperty("packIcon")] public string PackIcon;
        /// <summary>1024² texture in the vanilla card-box UV layout (TCG Studio's box editor shows it).</summary>
        [JsonProperty("boxTexture")] public string BoxTexture;
        [JsonProperty("boxIcon")] public string BoxIcon;

        /// <summary>Wholesale cost of one pack (vanilla basic pack = 1.5).</summary>
        [JsonProperty("packCost")] public float PackCost = 1.5f;
        /// <summary>Wholesale cost of one box. Null = 8 × pack cost (vanilla behaviour).</summary>
        [JsonProperty("boxCost")] public float? BoxCost;
        /// <summary>Market price range as multiples of cost (vanilla pack 1.5–2).</summary>
        [JsonProperty("marketMin")] public float MarketMin = 1.5f;
        [JsonProperty("marketMax")] public float MarketMax = 2f;

        [JsonProperty("license")] public LicenseDef License = new LicenseDef();

        /// <summary>Rarity weights per slot. Counts must add up to cardsPerPack. Empty = vanilla-like odds for every slot.</summary>
        [JsonProperty("slots")] public List<PackSlot> Slots = new List<PackSlot>();
        /// <summary>Percent chance per card to be foil (vanilla 5).</summary>
        [JsonProperty("foilChance")] public float FoilChance = 5f;
        /// <summary>Percent chance per card for each special border, checked rarest first (vanilla values by default).</summary>
        [JsonProperty("borderOdds")] public Dictionary<string, float> BorderOdds = new Dictionary<string, float>
        {
            ["FullArt"] = 0.25f, ["EX"] = 1f, ["Gold"] = 4f, ["Silver"] = 8f, ["FirstEdition"] = 20f
        };
        [JsonProperty("allowDuplicates")] public bool AllowDuplicates;
        /// <summary>Optional card ids this pack draws from; empty = every card in the set.</summary>
        [JsonProperty("cards")] public List<string> Cards = new List<string>();
    }

    /// <summary>
    /// Restock licenses. Vanilla sells each product in a small and a big delivery box, each with its own license row.
    /// The big rows are optional; when missing they default to level +1 and price ×1.5.
    /// </summary>
    public class LicenseDef
    {
        [JsonProperty("packLevel")] public int PackLevel = 1;
        [JsonProperty("packPrice")] public float PackPrice = 100f;
        [JsonProperty("packBigLevel")] public int? PackBigLevel;
        [JsonProperty("packBigPrice")] public float? PackBigPrice;
        [JsonProperty("boxLevel")] public int BoxLevel = 3;
        [JsonProperty("boxPrice")] public float BoxPrice = 200f;
        [JsonProperty("boxBigLevel")] public int? BoxBigLevel;
        [JsonProperty("boxBigPrice")] public float? BoxBigPrice;
    }

    public class PackSlot
    {
        [JsonProperty("count")] public int Count = 1;
        /// <summary>Relative weight per rarity id (the set's rarities; vanilla names when the set has no list).</summary>
        [JsonProperty("weights")] public Dictionary<string, float> Weights = new Dictionary<string, float>();
    }

    // ---- Accessory library (<plugin>\Accessories\accessories.json, written by TCG Studio) ----

    /// <summary>Custom accessories (deck boxes, playmats, sleeves, dice, comics, collection books, battle decks). Global (not part of a set); see docs\set-format.md "Accessory library".</summary>
    public class AccessoryLibraryDef
    {
        public const int CurrentSchemaVersion = 1;
        [JsonProperty("schemaVersion")] public int SchemaVersion = CurrentSchemaVersion;
        [JsonProperty("accessories")] public List<AccessoryDef> Accessories = new List<AccessoryDef>();
        [JsonProperty("furniture")] public List<FurnitureDef> Furniture = new List<FurnitureDef>();
        [JsonIgnore] public string FolderPath;
    }

    /// <summary>What a custom furniture piece does: the vanilla behaviour (component) it is built on.</summary>
    public enum FurnitureType
    {
        Shelf, CardShelf, PlayTable, BulkDonationBox, TrashBin, EmptyBoxStorage, CardStorageShelf,
        AutoPackOpener, AutoCleanser, Workbench, CashCounter, WarehouseShelf, TournamentPrizeShelf
    }

    /// <summary>
    /// A new furniture piece: a copy of a vanilla piece of the same <see cref="Type"/> (its behaviour) with its own look, name, price and,
    /// for shelves, its own spots. Sold in the furniture shop next to the vanilla pieces.
    /// </summary>
    public class FurnitureDef
    {
        /// <summary>Stable id (side-car keys "fur/&lt;id&gt;"). No spaces, ':', '/' or '|'.</summary>
        [JsonProperty("id")] public string Id;
        [JsonProperty("type"), JsonConverter(typeof(StringEnumConverter))] public FurnitureType Type;
        [JsonProperty("name")] public string Name;
        [JsonProperty("description")] public string Description;
        /// <summary>Vanilla EObjectType of the same type used as the starting point (e.g. "ShelfSmall"). Empty = the type's default.</summary>
        [JsonProperty("base")] public string Base;
        /// <summary>Furniture shop price. Null = base piece's price.</summary>
        [JsonProperty("price")] public float? Price;
        /// <summary>Shop level needed to buy it. Null = base piece's level.</summary>
        [JsonProperty("level")] public int? Level;
        /// <summary>Shop attractiveness per placed piece (customers). Null = base piece's value.</summary>
        [JsonProperty("decoBonus")] public float? DecoBonus;
        [JsonProperty("icon")] public string Icon;
        /// <summary>Replacement for the base piece's main texture (same UV layout). Null = base art.</summary>
        [JsonProperty("texture")] public string Texture;
        /// <summary>Colour multiplied onto the piece's materials, "#RRGGBB". Null = none.</summary>
        [JsonProperty("tint")] public string Tint;
        /// <summary>Own model: baked OBJ in the piece's local space (Unity space, like figurine models). Null = base model.</summary>
        [JsonProperty("mesh")] public string Mesh;
        /// <summary>
        /// Painted in TCG Studio's face editor: the base's body renderers get meshes with new UVs into one painted atlas
        /// (<see cref="Runtime.FurniturePaint"/>). Replaces texture/tint. Null = none.
        /// </summary>
        [JsonProperty("paint")] public FurniturePaintDef Paint;
        /// <summary>Shelf spots (item or card spots, depending on the type). Null/empty = the base piece's own spots.</summary>
        [JsonProperty("spots")] public List<FurnitureSpotDef> Spots;
        /// <summary>Placement area: the floor box kept free of other furniture/walls when placing. Null = the base piece's.</summary>
        [JsonProperty("area")] public FurnitureAreaDef Area;
        /// <summary>Positions the game uses (seats, where the cashier/worker stands, customer stand points…) by role. Null = the base's.</summary>
        [JsonProperty("points")] public List<FurniturePointDef> Points;

        [JsonIgnore] public EObjectType BaseObject = EObjectType.None;
        [JsonIgnore] public string FolderPath;
    }

    public class FurniturePaintDef
    {
        /// <summary>The painted atlas (every part reads it).</summary>
        [JsonProperty("texture")] public string Texture;
        [JsonProperty("parts")] public List<FurniturePaintPartDef> Parts;
    }

    public class FurniturePaintPartDef
    {
        /// <summary>Body renderer under the piece root: "&lt;sibling index&gt;:&lt;name&gt;/…" ("" = the root itself).</summary>
        [JsonProperty("renderer")] public string Renderer;
        /// <summary>Its mesh with the painting UVs: OBJ in the renderer's mesh space, one "usemtl" group per submesh.</summary>
        [JsonProperty("mesh")] public string Mesh;
    }

    public enum FurnitureSpotKind { Items, Card }

    /// <summary>
    /// A position the game uses on a piece, in its local space (metres, degrees). Points of a role replace the base piece's in order;
    /// resizable roles (lists the game picks from) may have more or fewer, the others keep the base's count. See FurnitureKinds.PointRoles.
    /// </summary>
    public class FurniturePointDef
    {
        [JsonProperty("role")] public string Role;
        [JsonProperty("pos")] public float[] Pos = { 0f, 0f, 0f };
        [JsonProperty("rot")] public float[] Rot = { 0f, 0f, 0f };
        /// <summary>Size against the vanilla piece's (screens, drawer, signs…). Null/0 = unchanged.</summary>
        [JsonProperty("scale")] public float? Scale;
    }

    /// <summary>
    /// Placement area in the piece's local space (metres): centre x/z on the floor and width (x) / depth (z). Height and rotation stay
    /// the base piece's. The game checks this box (m_MoveStateValidArea) against other furniture and walls, and snaps pieces by it.
    /// </summary>
    public class FurnitureAreaDef
    {
        [JsonProperty("pos")] public float[] Pos = { 0f, 0f };
        [JsonProperty("size")] public float[] Size;
    }

    /// <summary>
    /// One place where things go, in the piece's local space (metres, degrees). Item spot: a box of <see cref="Size"/> (width, depth, height)
    /// centred on <see cref="Pos"/>, filled with a grid of <see cref="Grid"/> item units (a pack is 1×1×1: 4×8×1 = 32 packs, like vanilla).
    /// Height 0 = items stand on that plane in one layer (vanilla shop shelves).
    /// Card spot: <see cref="Pos"/>/<see cref="Rot"/> is where the card sits.
    /// </summary>
    public class FurnitureSpotDef
    {
        [JsonProperty("kind"), JsonConverter(typeof(StringEnumConverter))] public FurnitureSpotKind Kind = FurnitureSpotKind.Items;
        [JsonProperty("pos")] public float[] Pos = { 0f, 0f, 0f };
        [JsonProperty("rot")] public float[] Rot = { 0f, 0f, 0f };
        [JsonProperty("size")] public float[] Size;
        [JsonProperty("grid")] public int[] Grid;
        /// <summary>Where customers stand to take from this spot (x, z on the floor). Null = in front of the spot.</summary>
        [JsonProperty("customer")] public float[] Customer;
        /// <summary>Price tag position. Null = kept relative to the spot as on the base piece.</summary>
        [JsonProperty("priceTag")] public float[] PriceTag;
        /// <summary>Item spots: whole boxes can be put here (like warehouse-style shelves).</summary>
        [JsonProperty("boxes")] public bool? Boxes;
    }

    /// <summary>Deck boxes, playmats, card sleeves, dice boxes, comics (Manga items), collection books (binders), battle decks (PreconDeck items), figurines (Toy items).</summary>
    public enum AccessoryKind { Deckbox, Playmat, Sleeve, Dice, Comic, Binder, BattleDeck, Figurine }

    public class AccessoryDef
    {
        /// <summary>Stable id (side-car keys "acc/&lt;id&gt;"). No spaces, ':', '/' or '|'.</summary>
        [JsonProperty("id")] public string Id;
        [JsonProperty("kind"), JsonConverter(typeof(StringEnumConverter))] public AccessoryKind Kind;
        [JsonProperty("name")] public string Name;
        /// <summary>Vanilla item of the same kind whose model, box size, shop tab and restock rows are copied (e.g. "DeckBox1", "Manga3"). Empty = the kind's default (<see cref="AccessoryKinds.DefaultBase"/>).</summary>
        [JsonProperty("base")] public string Base;
        /// <summary>Full replacement texture in the base item's texture layout (as in TCG Studio's accessory editor). Null = base item's art.</summary>
        [JsonProperty("texture")] public string Texture;
        [JsonProperty("icon")] public string Icon;
        /// <summary>
        /// Figurines only: own model as a baked OBJ written by TCG Studio (Unity space: left-handed, Y up, UV origin bottom-left; one object,
        /// triangles), placed in the base toy's mesh space so it stands in the base toy's shelf slot. Null = the base toy's model.
        /// </summary>
        [JsonProperty("mesh")] public string Mesh;
        /// <summary>Wholesale cost per item. Null = base item's cost.</summary>
        [JsonProperty("cost")] public float? Cost;
        /// <summary>Market price range as multiples of cost. Null = base item's range.</summary>
        [JsonProperty("marketMin")] public float? MarketMin;
        [JsonProperty("marketMax")] public float? MarketMax;
        [JsonProperty("license")] public AccessoryLicenseDef License = new AccessoryLicenseDef();

        [JsonIgnore] public EItemType BaseItem = EItemType.None;
        [JsonIgnore] public string FolderPath;
    }

    /// <summary>
    /// Restock licenses, following the base item's rows: deck boxes have a small and a big delivery box (two rows), every other kind
    /// only a big one (one row, which uses level/price). Missing big values default to level +3 and price ×2 (vanilla deck boxes: 5/100, 8/200).
    /// </summary>
    public class AccessoryLicenseDef
    {
        [JsonProperty("level")] public int Level = 1;
        [JsonProperty("price")] public float Price = 100f;
        [JsonProperty("bigLevel")] public int? BigLevel;
        [JsonProperty("bigPrice")] public float? BigPrice;
    }
}

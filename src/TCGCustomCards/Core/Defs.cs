using System.Collections.Generic;
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

        [JsonProperty("priceDefaults")] public PriceDefaults PriceDefaults = new PriceDefaults();
        [JsonProperty("packs")] public List<PackDef> Packs = new List<PackDef>();
        [JsonProperty("cards")] public List<CardDef> Cards = new List<CardDef>();

        /// <summary>Absolute folder the set was loaded from (not serialized).</summary>
        [JsonIgnore] public string FolderPath;
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

        [JsonProperty("rarity"), JsonConverter(typeof(StringEnumConverter))]
        public ERarity Rarity = ERarity.Common;

        /// <summary>Printed card number (display only).</summary>
        [JsonProperty("number")] public string Number;

        /// <summary>Image path relative to the set folder. FullImage: whole card. Framed: artwork only.</summary>
        [JsonProperty("image")] public string Image;

        [JsonProperty("price")] public CardPrice Price = new CardPrice();
        [JsonProperty("play")] public PlayDef Play = new PlayDef();

        /// <summary>Real MTG card data for Forge deck export; null for non-MTG cards.</summary>
        [JsonProperty("mtg")] public CardMtgDef Mtg;
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
        /// <summary>Relative weight per rarity name (Common, Rare, Epic, Legendary, SuperLegend).</summary>
        [JsonProperty("weights")] public Dictionary<string, float> Weights = new Dictionary<string, float>();
    }

    // ---- Accessory library (<plugin>\Accessories\accessories.json, written by TCG Studio) ----

    /// <summary>Custom accessories (deck boxes, playmats, sleeves, dice, comics, collection books, battle decks). Global (not part of a set); see docs\set-format.md "Accessory library".</summary>
    public class AccessoryLibraryDef
    {
        public const int CurrentSchemaVersion = 1;
        [JsonProperty("schemaVersion")] public int SchemaVersion = CurrentSchemaVersion;
        [JsonProperty("accessories")] public List<AccessoryDef> Accessories = new List<AccessoryDef>();
        [JsonIgnore] public string FolderPath;
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

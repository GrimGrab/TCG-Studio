package importer

import (
	"strconv"

	"tcgstudio/internal/setfmt"
)

// ---------------------------------------------------------------- every other TCGplayer card game (via TCGCSV)
// One engine (tcgplayer_source.go) for the card games TCGplayer sells that have no source of their own: each game is
// one tcgGame entry below and shows up as its own source on the Import page. Field names, rarities and colours come
// from surveys of the live catalogue (TestTCGSurvey, TestTCGValues), 2026-10-23.
//
// Not listed: KeyForge (every deck is unique, no packs), Dice Masters, and categories with no usable cards on TCGplayer:
// Epic (sealed only), Redakai, Alternate Souls, Architect, Neopets Battledome, Rush of Ikorr (no products yet), My Little
// Pony (21: gone; 38: 16 cards without fields), Chrono Clash (every rarity "None", most cards without a picture).

// tcgGame describes one TCGplayer category. Only ID, Category, Game and Prefix are required; every other field has a
// default that fits most games.
type tcgGame struct {
	ID       string // source id = meta.source ("tcg-<slug>"); never change once released
	Category int    // TCGplayer category id
	Game     string // name on the Import page
	Prefix   string // project id prefix ("gcg" → gcg-gd01); never change once released

	// extendedData field names ("" = the first of tcgDefaultFields that a card has).
	Color, Type, Cost, Power string
	ColorLabel               string   // filter label ("Color" when Colors is set)
	Colors                   []string // filter values in order (empty = no colour filter)
	CostLabel, PowerLabel    string   // Cards-tab sort labels (set together with Cost / Power)
	TypeParts                []string // fields added to the type line after " — "
	TextFields               []string // fields added under the rules text as "Name: value"
	Text                     string   // rules text field ("" = Description…)

	SkipRarities []string // rarities left out (lower-case prefixes; default tcgSkipRarities)
	SkipNumbers  []string // card numbers left out (prefixes: "T-" tokens, "R-" resources…)
	VariantTags  string   // regexp: name tags that mark a printing variant (default tcgVariantTags)
	DropTags     string   // regexp: name tags dropped from the name (default tcgCodeTag: card codes)
	SkipTags     string   // regexp: products with such a tag are left out (separate foil products, "Claimed" copies)

	// Ladder: the source's rarities lowest first, per game rarity Common / Rare / Epic / Legendary. Empty = the set's own
	// rarities ranked by name and count at import (tcgRankRarities).
	Ladder [4][]string

	Elements map[string]string // colour value → play element (default by colour words)
	Booster  *Booster          // nil = tcgDefaultBooster
	Normal   []string          // base price sub-types in order (default Normal, Unlimited, 1st Edition)
	Foil     []string          // foil price sub-types (default Foil, Holofoil)
}

// tcgPack is an n-card booster in the usual shape: mostly commons, a quarter uncommons, one rare-or-better slot last.
// The publishers of these games don't publish slot odds, so every one is an estimate.
func tcgPack(game string, n int) *Booster {
	u := (n + 2) / 4
	return &Booster{Game: game, Name: game + " booster, " + strconv.Itoa(n) + " cards", Estimate: true, FoilChance: 5,
		Slots: []setfmt.Slot{slot(n-u-1, cC), slot(u, cU), slot(1, w("Epic", 4, "Legendary", 1))}}
}

var (
	claimed   = `^claimed$` // PlayFusion games list every card twice: digital code claimed / unclaimed
	unclaimed = `^unclaimed$|` + tcgCodeTag
	foilCopy  = `^foil$` // separate products for the foil printing (price = foil multiplier)
	nonFoil   = `^non-foil$|` + tcgCodeTag
)

var tcgGames = []tcgGame{
	{ID: "tcg-gundam", Category: 86, Game: "Gundam Card Game", Prefix: "gcg",
		Color: "Color", Colors: []string{"Blue", "Green", "Red", "White", "Purple"},
		Elements: map[string]string{"White": "Wind", "Purple": "Wind"},
		Cost:     "Cost", CostLabel: "Cost", Power: "Attack Points", PowerLabel: "AP", TypeParts: []string{"Trait"},
		TextFields: []string{"Link Condition", "Zone"}, SkipNumbers: []string{"T-", "R-", "RP-", "EXB-", "EXR-"},
		Ladder:  [4][]string{{"Common"}, {"Uncommon"}, {"Rare", "Promo"}, {"Legend Rare", "C+", "U+", "R+", "LR+", "C++", "LR++"}},
		Booster: tcgPack("Gundam Card Game", 12)},
	{ID: "tcg-digimon", Category: 63, Game: "Digimon Card Game", Prefix: "dgm",
		Color: "Color", Colors: []string{"Red", "Blue", "Yellow", "Green", "Black", "Purple", "White"},
		Cost: "PlayCost", CostLabel: "Play cost", Power: "DigimonPowerDP", PowerLabel: "DP",
		TypeParts: []string{"DigimonForm", "DigimonAttribute", "DigimonType"}, TextFields: []string{"Inherited Effect", "Security Effect"},
		Ladder:  [4][]string{{"None", "Common"}, {"Uncommon"}, {"Rare", "Promo"}, {"Super Rare", "Special Rare", "Secret Rare", "Ultimate Rare"}},
		Booster: tcgPack("Digimon", 12)},
	{ID: "tcg-dbsfw", Category: 80, Game: "Dragon Ball Super: Fusion World", Prefix: "dbfw",
		Color: "Color", Colors: []string{"Red", "Blue", "Green", "Yellow", "Black"},
		Cost: "Cost", CostLabel: "Cost", Power: "Power", PowerLabel: "Power", TypeParts: []string{"Character Traits"},
		Ladder:  [4][]string{{"None", "Common"}, {"Uncommon"}, {"Rare", "Leader", "Promo"}, {"Super Rare", "Secret Rare"}},
		Booster: tcgPack("Fusion World", 12)},
	{ID: "tcg-dbsmasters", Category: 27, Game: "Dragon Ball Super Card Game (Masters)", Prefix: "dbs",
		Color: "Color", Colors: []string{"Red", "Blue", "Green", "Yellow", "Black", "White"},
		Cost: "EnergyColor Cost", CostLabel: "Energy", Power: "Power", PowerLabel: "Power", TypeParts: []string{"SpecialTrait"},
		Ladder: [4][]string{{"None", "Common", "Starter Rare"}, {"Uncommon"}, {"Rare", "Promo", "Merit", "Expansion Rare"},
			{"Super Rare", "Special Rare", "Special Leader Rare", "Secret Rare", "Concept Rare", "God Rare", "Dragon Ball Rare",
				"Giant Force Rare", "Son Gohan Rare"}},
		Booster: tcgPack("Dragon Ball Super", 12)},
	{ID: "tcg-weiss", Category: 20, Game: "Weiß Schwarz", Prefix: "ws",
		Color: "Color", Colors: []string{"Red", "Blue", "Green", "Yellow"},
		Cost: "Cost", CostLabel: "Cost", Power: "Power", PowerLabel: "Power", TypeParts: []string{"Trait", "Trait 2"},
		Text: "CardText", TextFields: []string{"Triggers"}, VariantTags: tcgVariantTags + `|^td$`,
		Ladder: [4][]string{{"Common", "Climax Common", "Trial Deck"}, {"Uncommon", "Holo Common", "Holo Uncommon"},
			{"Rare", "Climax Rare", "Double Rare", "Holo Double Rare", "High Rare", "Promo", "English Exclusive"},
			{"Triple Rare", "Super Rare", "Special Rare", "Over-Frame Rare", "Secret Rare", "Super Special Rare", "AoButa Rare",
				"Promo +", "Serial Rare", "Autograph Rare"}},
		Booster: tcgPack("Weiß Schwarz", 8)},
	{ID: "tcg-vanguard", Category: 16, Game: "Cardfight!! Vanguard", Prefix: "cfv",
		Color: "Nation", ColorLabel: "Nation", Colors: []string{"Dragon Empire", "Dark States", "Brandt Gate", "Keter Sanctuary",
			"Stoicheia", "Lyrical Monasterio", "Touken Ranbu", "BanG Dream!"},
		Elements: map[string]string{"Dragon Empire": "Fire", "Dark States": "Earth", "Brandt Gate": "Water", "Stoicheia": "Earth"},
		Cost:     "Grade", CostLabel: "Grade", Power: "Power", PowerLabel: "Power", TypeParts: []string{"Clan", "Race"},
		TextFields: []string{"Trigger", "Skill Icon"},
		Ladder: [4][]string{{"C", "T", "TD", "H"}, {"R", "RE", "EXC"}, {"RR", "EX", "EXS", "GBPR", "RRR", "TRR"},
			{"SP", "SR", "SEC", "SECV", "Frame Rare", "Double Frame Rare", "R+", "GBPR+", "LGR", "LGRR", "LGRRR", "LGTR", "LSR",
				"15thSP", "ORR", "TGR", "TGR+", "EXRRR", "SCR", "DSR", "PGS", "PGSP", "MSP", "VR", "SVR", "XVR"}},
		Booster: tcgPack("Vanguard", 7)},
	{ID: "tcg-shadowverse", Category: 73, Game: "Shadowverse: Evolve", Prefix: "sve",
		Color: "Class", ColorLabel: "Class", Colors: []string{"Forestcraft", "Swordcraft", "Runecraft", "Dragoncraft", "Abysscraft",
			"Havencraft", "Neutral"},
		Elements: map[string]string{"Forestcraft": "Earth", "Swordcraft": "Wind", "Runecraft": "Water", "Dragoncraft": "Fire",
			"Abysscraft": "Earth", "Havencraft": "Wind"},
		Cost: "Cost", CostLabel: "Cost", Power: "Attack", PowerLabel: "Attack", TypeParts: []string{"Trait"},
		Ladder: [4][]string{{"None", "Bronze"}, {"Silver"}, {"Gold", "Promo"},
			{"Legendary", "Super Legendary", "Ultimate", "Special", "Super Special", "Premium"}},
		Booster: tcgPack("Shadowverse: Evolve", 8)},
	{ID: "tcg-hololive", Category: 87, Game: "hololive OFFICIAL CARD GAME", Prefix: "holo",
		Color: "Color", Colors: []string{"White", "Green", "Red", "Blue", "Purple", "Yellow", "Neutral"},
		Power: "HP", PowerLabel: "HP", TypeParts: []string{"BloomLevel", "Subtype"},
		TextFields: []string{"Arts 1", "Arts 2", "Keyword", "Oshi Skill", "SP Oshi Skill", "Special Rules", "Tags"},
		// every card carries its rarity as a name tag ("(C)", "(SR)"): not a variant
		DropTags: `^(s|c|u|r|rr|sr|ur|osr|our|oc|sy|sec|hr|p)$|` + tcgCodeTag,
		Ladder: [4][]string{{"Common", "Oshi Common", "Special"}, {"Uncommon"}, {"Rare", "Double Rare", "Holomem Rare", "Promo"},
			{"Super Rare", "Ultra Rare", "Oshi Super Rare", "Oshi Ultra Rare", "Special Yell", "Secret Rare"}}},
	{ID: "tcg-riftbound", Category: 89, Game: "Riftbound: League of Legends", Prefix: "rb",
		Color: "Domain", ColorLabel: "Domain", Colors: []string{"Fury", "Calm", "Mind", "Body", "Chaos", "Order"},
		Elements: map[string]string{"Fury": "Fire", "Calm": "Water", "Mind": "Wind", "Body": "Earth", "Chaos": "Fire", "Order": "Earth"},
		Type:     "Card Type", Cost: "Energy Cost", CostLabel: "Energy", Power: "Might", PowerLabel: "Might", TypeParts: []string{"Tag"},
		Ladder:  [4][]string{{"None", "Common"}, {"Uncommon"}, {"Rare", "Promo"}, {"Epic", "Showcase"}},
		Booster: tcgPack("Riftbound", 14)},
	{ID: "tcg-grandarchive", Category: 74, Game: "Grand Archive", Prefix: "ga",
		Color: "Element", ColorLabel: "Element", Colors: []string{"Norm", "Fire", "Water", "Wind", "Arcane", "Astra", "Crux", "Exalted",
			"Exia", "Luxem", "Neos", "Tera", "Umbra"},
		Elements: map[string]string{"Norm": "Earth", "Tera": "Earth", "Umbra": "Earth", "Crux": "Earth"},
		Cost:     "Reserve Cost", CostLabel: "Cost", Power: "Power", PowerLabel: "Power", TypeParts: []string{"Class", "CardSubtype"},
		Ladder: [4][]string{{"Common"}, {"Uncommon"}, {"Rare", "Promo"},
			{"Super Rare", "Ultra Rare", "Collector Promo", "Collector Super Rare", "Collector Ultra Rare"}}},
	{ID: "tcg-sorcery", Category: 77, Game: "Sorcery: Contested Realm", Prefix: "sor",
		Color: "Element", ColorLabel: "Element", Colors: []string{"Air", "Earth", "Fire", "Water"},
		Elements: map[string]string{"Air": "Wind"},
		Cost:     "Cost", CostLabel: "Cost", Power: "Power Rating", PowerLabel: "Power", TypeParts: []string{"Card Subtype"},
		SkipTags: `^(foil|preconstructed deck)$`,
		Ladder:   [4][]string{{"None", "Ordinary"}, {"Exceptional"}, {"Elite", "Promo"}, {"Unique"}},
		Booster:  tcgPack("Sorcery", 15)},
	{ID: "tcg-godzilla", Category: 88, Game: "Godzilla Card Game", Prefix: "gzl",
		Color: "Color", Colors: []string{"Red", "Blue", "Green", "White"}, TypeParts: []string{"Traits", "Source"},
		Ladder: [4][]string{{"C", "SD", "T"}, {"UC"}, {"R", "P", "PR"},
			{"SR", "SP", "SSP", "SSSP", "PP", "RP", "PRP", "PRSP", "PRSSP", "CGP", "ACSP", "SSPT"}}},
	{ID: "tcg-cookierun", Category: 90, Game: "CookieRun: Braverse", Prefix: "crb",
		Color: "Color", Colors: []string{"Red", "Yellow", "Green", "Blue", "Purple", "Black", "Pure"},
		Cost: "Level", CostLabel: "Level", Power: "HP", PowerLabel: "HP", TypeParts: []string{"Keyword"}, TextFields: []string{"Flip"},
		Ladder: [4][]string{{"C"}, {"U"}, {"R", "SR", "P"}, {"SSR", "UR", "SEC", "SUR", "EXR", "GXR"}}},
	{ID: "tcg-palworld", Category: 91, Game: "Palworld Card Game", Prefix: "pal",
		Color: "Color", Colors: []string{"Red", "Blue", "Green", "Purple", "Colorless"},
		Cost: "Cost", CostLabel: "Cost", Power: "Power", PowerLabel: "Power", TypeParts: []string{"CardSubtype", "Element"},
		TextFields: []string{"WorkSuitability"},
		Ladder:     [4][]string{{"C", "TD"}, {"U"}, {"R", "RR", "TDR"}, {"SR", "OSR", "SP", "TSR", "SSP", "TSP", "SSS"}}},
	{ID: "tcg-cyberpunk", Category: 92, Game: "Cyberpunk TCG", Prefix: "cyb",
		Color: "Color", Colors: []string{"Red", "Blue", "Green", "Yellow"},
		Cost: "Cost", CostLabel: "Cost", Power: "Power", PowerLabel: "Power", TypeParts: []string{"Tags"},
		Ladder: [4][]string{{"C", "NR"}, {"UC"}, {"R"}, {"E", "IL", "IO", "S", "IS"}}},
	{ID: "tcg-naruto", Category: 93, Game: "Naruto Card Game", Prefix: "nar"},
	{ID: "tcg-fftcg", Category: 24, Game: "Final Fantasy TCG", Prefix: "fft",
		Color: "Element", ColorLabel: "Element", Colors: []string{"Fire", "Ice", "Wind", "Earth", "Lightning", "Water", "Light", "Dark"},
		Elements: map[string]string{"Ice": "Water", "Lightning": "Wind", "Light": "Wind", "Dark": "Earth"},
		Cost:     "Cost", CostLabel: "Cost", Power: "Power", PowerLabel: "Power", TypeParts: []string{"Job", "Category"},
		Ladder:  [4][]string{{"None", "Common", "Starter"}, {"Rare"}, {"Hero", "Promo"}, {"Legend"}},
		Booster: tcgPack("Final Fantasy TCG", 12)},
	{ID: "tcg-forceofwill", Category: 17, Game: "Force of Will", Prefix: "fow",
		Color: "Attribute", ColorLabel: "Attribute", Colors: []string{"Light", "Fire", "Water", "Wind", "Darkness", "Void"},
		Elements: map[string]string{"Light": "Wind", "Darkness": "Earth", "Void": "Earth"},
		Cost:     "TotalCost", CostLabel: "Cost", Power: "Atk", PowerLabel: "ATK", TypeParts: []string{"Race"}, Text: "RulesText",
		Ladder: [4][]string{{"Basic", "Normal", "Common"}, {"Uncommon", "Rare"}, {"Super Rare", "Promo"},
			{"Marvel Rare", "Ruler Rare", "Sub Ruler Rare", "Uber Rare", "XR", "Secret", "Secret Uber"}},
		Booster: tcgPack("Force of Will", 10)},
	{ID: "tcg-wixoss", Category: 67, Game: "WIXOSS", Prefix: "wx",
		Color: "Color", Colors: []string{"White", "Red", "Blue", "Green", "Black", "Colorless"},
		Cost: "Level", CostLabel: "Level", Power: "Power", PowerLabel: "Power", TypeParts: []string{"LRIGTypeClass"},
		TextFields: []string{"Grow Cost", "Limit", "Timing"},
		Ladder: [4][]string{{"C", "LC", "T", "ST", "Re", "None"}, {"R"}, {"LR", "SR", "PR", "1st"},
			{"UR", "SRP", "LRP", "PI", "SP", "SCR", "DiR", "ENR", "MAM", "GR", "???"}}},
	{ID: "tcg-battlespirits", Category: 72, Game: "Battle Spirits Saga", Prefix: "bss",
		Color: "Color", Colors: []string{"Red", "Purple", "Green", "White", "Yellow", "Blue"},
		Elements: map[string]string{"Purple": "Earth", "White": "Wind", "Yellow": "Wind"},
		Cost:     "Cost", CostLabel: "Cost", Power: "LV I BP", PowerLabel: "BP", TypeParts: []string{"Spirit Type"},
		Ladder: [4][]string{{"None", "Common"}, {"Uncommon"}, {"Rare", "Promo"},
			{"Special Rare", "X Rare", "Collaboration Rare", "Collaboration X Rare", "Saga Rare", "EVA SAGA"}}},
	{ID: "tcg-pokemonjp", Category: 85, Game: "Pokémon (Japanese)", Prefix: "ptcgjp",
		Color: "CardType", ColorLabel: "Type", Colors: []string{"Grass", "Fire", "Water", "Lightning", "Psychic", "Fighting", "Darkness",
			"Metal", "Dragon", "Fairy", "Colorless", "Trainer", "Energy"},
		Elements: map[string]string{"Grass": "Earth", "Lightning": "Wind", "Psychic": "Wind", "Fighting": "Earth", "Darkness": "Earth",
			"Metal": "Earth", "Dragon": "Fire", "Fairy": "Wind"},
		Power: "HP", PowerLabel: "HP", TypeParts: []string{"Stage"}, TextFields: []string{"Attack 1", "Attack 2", "Attack 3", "Weakness",
			"Resistance", "Retreat Cost"},
		Foil: []string{"Holofoil", "1st Edition Holofoil", "Reverse Holofoil"}},
	{ID: "tcg-elestrals", Category: 83, Game: "Elestrals", Prefix: "ele",
		Color: "ElementType", ColorLabel: "Element", Colors: []string{"Earth", "Fire", "Water", "Thunder", "Wind", "Frost", "Solar",
			"Lunar", "Omni"},
		Elements: map[string]string{"Thunder": "Wind", "Frost": "Water", "Solar": "Fire", "Lunar": "Wind", "Omni": "Wind"},
		Power:    "Attack", PowerLabel: "Attack", TypeParts: []string{"Subclass"}, TextFields: []string{"Element Cost"},
		VariantTags: tcgVariantTags + `|stellar`, Foil: []string{"Holo", "Reverse Holo", "Foil"},
		Ladder: [4][]string{{"C"}, {"UC"}, {"R"}, {"SR"}}},
	{ID: "tcg-alphaclash", Category: 78, Game: "Alpha Clash", Prefix: "acl",
		Color: "Color", Colors: []string{"Black", "Red", "Green", "White", "Blue", "Colorless"},
		Cost: "Resource Cost", CostLabel: "Cost", Power: "Attack", PowerLabel: "Attack", TypeParts: []string{"CardSubtype", "Affiliation"},
		VariantTags: tcgVariantTags + `|monochrome|^ink$`,
		Ladder: [4][]string{{"Common", "Starter Rare"}, {"Uncommon"}, {"Rare", "Promo"},
			{"Epic", "Legendary", "Iconic Rare", "Iconic Art", "Alpha Rare", "AxO"}}},
	{ID: "tcg-metazoo", Category: 66, Game: "MetaZoo", Prefix: "mz",
		Type: "PageType", Color: "AuraType", ColorLabel: "Aura", Colors: []string{"Colorless", "Dark", "Water", "Forest", "Light", "Spirit",
			"Cosmic", "Earth", "Flame", "Lightning", "Frost"},
		Elements: map[string]string{"Forest": "Earth", "Flame": "Fire", "Frost": "Water", "Dark": "Earth", "Lightning": "Wind"},
		Cost:     "Total Aura Cost", CostLabel: "Aura cost", Power: "LP", PowerLabel: "LP", TypeParts: []string{"Tribe", "Traits"},
		TextFields: []string{"Ability", "Attack", "4th Wall Effects"}, Foil: []string{"Holo", "Reverse Holo"},
		Ladder: [4][]string{{"None", "Bronze"}, {"Silver"}, {"Gold"}, {}}},
	{ID: "tcg-gateruler", Category: 65, Game: "Gate Ruler", Prefix: "gr",
		Color: "Faction", ColorLabel: "Faction", Colors: []string{"ATLAS", "Volnar", "Wonderverse", "Yomajin Front", "Exvader"},
		Cost: "Level", CostLabel: "Level", Power: "ATK", PowerLabel: "ATK", TypeParts: []string{"Attribute"},
		Ladder: [4][]string{{"None", "No Star", "1-Star"}, {"2-Star"}, {"3-Star"},
			{"4-Star", "Uber Rare", "Superior Rare", "Legend Rare", "???"}}},
	{ID: "tcg-akora", Category: 75, Game: "Akora TCG", Prefix: "ako",
		Type: "Card Type", Color: "Attribute", ColorLabel: "Attribute", Colors: []string{"Spirit", "Dark", "Ice", "Divine", "Fire", "Nature",
			"Electric", "Water", "Warped", "Multi"},
		Elements: map[string]string{"Ice": "Water", "Nature": "Earth", "Dark": "Earth", "Electric": "Wind"},
		Power:    "Attack Power", PowerLabel: "Attack", TypeParts: []string{"Subtype"},
		Ladder: [4][]string{{"Common"}, {"Uncommon"}, {"Rare", "Holo", "Promo"},
			{"Full Art", "Extended Art", "Extended Art - Box Topper", "Generational Rare", "Character Rare", "Alternate Art Full Art",
				"Extended Alternate Art", "Signature Serialized", "Secret Rare", "Ancient Rare", "Box Topper", "Box Topper Exclusive"}}},
	{ID: "tcg-kryptik", Category: 76, Game: "Kryptik TCG", Prefix: "kry",
		Color: "SoulBox", ColorLabel: "Soul", Colors: []string{"Black", "Blue", "White", "Green"},
		Power: "Attack", PowerLabel: "Attack", TypeParts: []string{"Faction", "SubType"},
		VariantTags: tcgVariantTags + `|kryptik lang|cosmo|glass|galaxy`,
		Ladder:      [4][]string{{"Common"}, {}, {"Rare"}, {"Noble Rare"}}},
	{ID: "tcg-universus", Category: 25, Game: "UniVersus", Prefix: "uvs",
		Color: "Resource", ColorLabel: "Resource", Colors: []string{"Air", "All", "Chaos", "Death", "Earth", "Evil", "Fire", "Good",
			"Infinity", "Life", "Order", "Void", "Water"},
		Cost: "Difficulty", CostLabel: "Difficulty", Power: "AttackDamage", PowerLabel: "Damage", TypeParts: []string{"Keywords"},
		Ladder: [4][]string{{"None", "Common", "Character Common", "Character", "Starter Exclusive", "Token"}, {"Uncommon"},
			{"Rare", "Character Rare", "Chrome Rare", "Promo"},
			{"Ultra Rare", "Character Ultra Rare", "Secret Rare", "Alternate Art Common", "Alternate Art Uncommon", "Alternate Art Rare",
				"Character Common Alternate Art", "Character Alternate Art Rare", "Character Rare Alternate Art", "Alternate Art Ultra Rare",
				"Alternate Art Secret Rare", "Promo-Alternate Art"}}},
	{ID: "tcg-swdestiny", Category: 26, Game: "Star Wars: Destiny", Prefix: "swd",
		Color: "Color", Colors: []string{"Red", "Yellow", "Blue", "Gray"},
		Cost: "Cost", CostLabel: "Cost", Power: "Health", PowerLabel: "Health", TypeParts: []string{"Affiliation", "Sub Type"},
		TextFields: []string{"Die"}, SkipTags: `^card only$`, Normal: []string{"Card and Die", "Normal"},
		Ladder: [4][]string{{"None", "Common", "Fixed"}, {"Uncommon"}, {"Rare", "Promo"}, {"Legendary"}}},
	{ID: "tcg-argentsaga", Category: 61, Game: "Argent Saga", Prefix: "ags",
		Color: "Element", ColorLabel: "Element", Colors: []string{"Air", "Light", "Dark", "Fire", "Water", "Neutral", "Argent"},
		Elements: map[string]string{"Air": "Wind", "Light": "Wind", "Dark": "Earth"},
		Cost:     "ShardCost", CostLabel: "Shards", Power: "Power", PowerLabel: "Power", TypeParts: []string{"Subtype"},
		Foil:   []string{"Foil", "1st Wave Foil"},
		Ladder: [4][]string{{"None", "Common", "Intro Deck"}, {"Rare"}, {"Super Rare", "Promo"}, {"Argent Rare", "Secret"}}},
	{ID: "tcg-bakugan", Category: 58, Game: "Bakugan TCG", Prefix: "bkg",
		Color: "Faction", ColorLabel: "Faction", Colors: []string{"Pyrus", "Aquos", "Ventus", "Darkus", "Haos", "Aurelus"},
		Elements: map[string]string{"Pyrus": "Fire", "Aquos": "Water", "Ventus": "Wind", "Darkus": "Earth", "Haos": "Wind", "Aurelus": "Earth"},
		Cost:     "EnergyCost", CostLabel: "Energy", Power: "BPower", PowerLabel: "B-Power", Foil: []string{"Holohex", "Foil"},
		Ladder: [4][]string{{"None", "Common", "Character Card"}, {"Rare"}, {"Super Rare"}, {"Awesome Rare", "Bakugan Elite"}}},
	{ID: "tcg-transformers", Category: 57, Game: "Transformers TCG", Prefix: "tf",
		Color: "Affiliation", ColorLabel: "Faction", Colors: []string{"Autobot", "Decepticon", "Neutral", "Mercenaries"},
		Elements: map[string]string{"Autobot": "Fire", "Decepticon": "Earth"},
		Cost:     "Stars", CostLabel: "Stars", TypeParts: []string{"SubType"},
		Ladder: [4][]string{{"None", "Common"}, {"Uncommon"}, {"Rare"}, {"Super Rare"}}},
	{ID: "tcg-aoschampions", Category: 54, Game: "Warhammer Age of Sigmar: Champions", Prefix: "aos",
		Color: "Alliance", ColorLabel: "Alliance", Colors: []string{"Chaos", "Death", "Destruction", "Order", "Any"},
		Elements:  map[string]string{"Chaos": "Fire", "Death": "Earth", "Destruction": "Earth", "Order": "Water"},
		TypeParts: []string{"Tag1", "Tag2"}, SkipTags: claimed, DropTags: unclaimed,
		Ladder: [4][]string{{"None", "Common", "Exclusive Starter"}, {"Uncommon"}, {"Rare"}, {"Exclusive"}}},
	{ID: "tcg-munchkin", Category: 53, Game: "Munchkin CCG", Prefix: "mun",
		Type: "Type", Color: "Faction", ColorLabel: "Faction", Colors: []string{"Neutral", "Bard", "Cleric", "Ranger", "Thief", "Warrior", "Wizard"},
		Cost: "GoldCost", CostLabel: "Gold", Power: "Power", PowerLabel: "Power", TypeParts: []string{"Subtype"},
		Ladder: [4][]string{{"None", "Common", "C - Common"}, {"Uncommon", "U - Uncommon"},
			{"Rare", "R - Rare", "Promo", "P - Promo", "Hero", "H - Hero", "Organized Play", "O - Organized Play"},
			{"Very Rare", "V - Very Rare", "X - Special", "F - Fomo", "We Destroyed Most of These", "W - We Destroyed Most of These"}}},
	{ID: "tcg-lightseekers", Category: 48, Game: "Lightseekers", Prefix: "lsk",
		Color: "PrimaryOrder", ColorLabel: "Order", Colors: []string{"Astral", "Nature", "Storm", "Tech", "Dread", "Mountain", "Unaligned"},
		Elements:  map[string]string{"Nature": "Earth", "Storm": "Wind", "Mountain": "Earth", "Dread": "Earth", "Tech": "Fire"},
		TypeParts: []string{"Family"}, SkipTags: claimed, DropTags: unclaimed,
		Ladder: [4][]string{{"None", "Common"}, {"Uncommon"}, {"Rare", "RareStarter", "RareToy"}, {"Mythic"}}},
	{ID: "tcg-exodus", Category: 47, Game: "Exodus TCG", Prefix: "exo",
		ColorLabel: "Type", Colors: []string{"Creature", "Symmetry", "Drifter", "Energy"}, // matches the start of the type line
		TypeParts: []string{"CreatureType"},
		Ladder:    [4][]string{{"None", "Common"}, {"Uncommon"}, {"Rare"}, {"Secret Rare", "Harmony Rare"}}},
	{ID: "tcg-casterchronicles", Category: 37, Game: "The Caster Chronicles", Prefix: "tcc",
		Color: "Element", ColorLabel: "Element", Colors: []string{"Aes", "Aqua", "Ignus", "Luna", "Silva", "Solis", "Terra", "Void"},
		Elements: map[string]string{"Aqua": "Water", "Ignus": "Fire", "Silva": "Earth", "Solis": "Fire", "Terra": "Earth"},
		Cost:     "Cost", CostLabel: "Cost", Power: "ATK", PowerLabel: "ATK", TypeParts: []string{"RaceandTrait"},
		SkipTags: foilCopy, DropTags: nonFoil,
		Ladder: [4][]string{{"None", "Common", "Repack"}, {"Uncommon"}, {"Rare", "Super Rare"}, {"Ultra Rare", "Secret Rare", "Hyper Rare"}}},
	{ID: "tcg-zwo", Category: 36, Game: "Zombie World Order", Prefix: "zwo",
		Color: "Color", Colors: []string{"Red", "Blue", "Green", "Grey"},
		Cost: "SummoningCost", CostLabel: "Cost", Power: "Power", PowerLabel: "Power", TypeParts: []string{"Attribute"},
		SkipTags: foilCopy, Ladder: [4][]string{{"C"}, {"UC"}, {"R", "PR"}, {}}},
	{ID: "tcg-metax", Category: 30, Game: "MetaX TCG", Prefix: "mtx",
		Color: "StatType", ColorLabel: "Stat", Colors: []string{"Strength", "Intelligence", "Special"},
		Elements: map[string]string{"Strength": "Fire", "Intelligence": "Water", "Special": "Wind"},
		Cost:     "MPPoints", CostLabel: "MP", TypeParts: []string{"CharacterType"},
		Ladder: [4][]string{{"", "None", "Common", "Starter"}, {"Uncommon"}, {"Rare", "Promo"}, {"Cross Rare", "Ultra Rare"}}},
	{ID: "tcg-dragoborne", Category: 28, Game: "Dragoborne", Prefix: "drb",
		Color: "Color", Colors: []string{"Red", "Blue", "Green", "Yellow", "Black", "Multicolored"},
		Cost: "Cost", CostLabel: "Cost", Power: "ATK", PowerLabel: "ATK", TypeParts: []string{"Nation", "CardSubType"},
		Foil:   []string{"Parallel Foil", "Foil"},
		Ladder: [4][]string{{"C", "TD"}, {"U"}, {"R", "PR"}, {"RR", "SCR"}}},
	{ID: "tcg-dbz", Category: 23, Game: "Dragon Ball Z TCG", Prefix: "dbz",
		Ladder: [4][]string{{"", "Starter", "Common"}, {"Uncommon"}, {"Rare", "Promo", "P", "Insert"}, {"Dragon Rare", "Ultra Rare"}}},
	{ID: "tcg-buddyfight", Category: 19, Game: "Future Card Buddyfight", Prefix: "bf",
		Ladder: [4][]string{{"", "C", "TD"}, {"U"}, {"R", "RR", "PR"}, {"RRR", "IR", "SR", "SP", "Secret", "BR", "ER"}}},
	{ID: "tcg-wow", Category: 13, Game: "World of Warcraft TCG", Prefix: "wow",
		Ladder: [4][]string{{"C"}, {"U"}, {"R"}, {"E"}}},
}

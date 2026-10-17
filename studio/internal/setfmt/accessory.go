package setfmt

// Accessory library: mirrors AccessoryLibraryDef/AccessoryDef in the mod's Core/Defs.cs and Core/AccessoryLoader.cs.
// Written to <setup>\accessories\accessories.json.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const AccessorySchemaVersion = 1

// AccessoryKind describes one kind of accessory: the vanilla items it can copy (texture, model, box size, shop tab, restock
// rows — every item of a kind shares one model, they differ in art and prices) and the mod's [Content] toggle that hides the
// vanilla ones. Mirrors Core/AccessoryKinds.cs (base names) in the mod.
type AccessoryKind struct {
	Kind    string   `json:"kind"`    // Deckbox | Playmat | Sleeve | Dice | Comic | Binder | BattleDeck | Figurine | BoardGame | CardSupply (JSON value in accessories.json)
	Title   string   `json:"title"`   // "Deck boxes"
	One     string   `json:"one"`     // "deck box"
	Toggle  string   `json:"toggle"`  // mod config key in [Content]
	Bases   []string `json:"bases"`   // vanilla items to start from; the first is the default
	Palette bool     `json:"palette"` // texture is a colour palette (recoloured, no artwork)
	Model   bool     `json:"model"`   // own 3D model (figurines): the base only gives the shelf slot, shop tab and prices
	// OwnModel: the art is painted on the base's model, or (optionally) the player's own model replaces it (board games, card supplies).
	OwnModel bool `json:"ownModel"`
	// PerBase: every base has its own model and layout (uvmap.ModelForBase), so changing the base changes the editor.
	PerBase bool `json:"perBase"`
}

// AccessoryShops: where an accessory can be sold instead of its base item's lists (Accessory.Shop; mirrors AccessoryShop in the mod).
var AccessoryShops = map[string]string{"items": "Restock: sets", "accessories": "Restock: accessories", "figurines": "Restock: figurines",
	"tabletop": "Tabletop Geek"}

var AccessoryKinds = []AccessoryKind{
	{Kind: "Deckbox", Title: "Deck boxes", One: "deck box", Toggle: "ShowVanillaDeckBoxes",
		Bases: []string{"DeckBox1", "DeckBox2", "DeckBox3", "DeckBox4"}},
	{Kind: "Playmat", Title: "Playmats", One: "playmat", Toggle: "ShowVanillaPlaymats",
		Bases: []string{"Playmat1", "Playmat2", "Playmat2b", "Playmat3", "Playmat4", "Playmat5", "Playmat6", "Playmat7",
			"Playmat8", "Playmat9", "Playmat10", "Playmat11", "Playmat12", "Playmat13", "Playmat14", "PlayMat15", "PlayMat16",
			"PlayMat17", "PlayMat18"}},
	{Kind: "Sleeve", Title: "Sleeves", One: "sleeve pack", Toggle: "ShowVanillaSleeves",
		Bases: []string{"CardSleeve_Tetramon", "CardSleeve_Clear", "CardSleeve_Fire", "CardSleeve_Earth", "CardSleeve_Water", "CardSleeve_Wind"}},
	{Kind: "Dice", Title: "Dice", One: "dice box", Toggle: "ShowVanillaDice", Palette: true,
		Bases: []string{"D20DiceBox", "D20DiceBox2", "D20DiceBox3", "D20DiceBox4"}},
	{Kind: "Comic", Title: "Comics", One: "comic", Toggle: "ShowVanillaComics",
		Bases: []string{"Manga1", "Manga2", "Manga3", "Manga4", "Manga5", "Manga6", "Manga7", "Manga8", "Manga9", "Manga10",
			"Manga11", "Manga12"}},
	{Kind: "Binder", Title: "Collection books", One: "collection book", Toggle: "ShowVanillaCollectionBooks",
		Bases: []string{"BinderBook", "BinderBookPremium"}},
	// Sold on the booster-pack tab (category TCG); sell-only like vanilla.
	{Kind: "BattleDeck", Title: "Battle decks", One: "battle deck", Toggle: "ShowVanillaBattleDecks",
		Bases: []string{"PreconDeck_Fire", "PreconDeck_Earth", "PreconDeck_Water", "PreconDeck_Wind", "PreconDeck_FireDestiny",
			"PreconDeck_EarthDestiny", "PreconDeck_WaterDestiny", "PreconDeck_WindDestiny"}},
	// Own model + texture; the base toy decides the shelf slot (size class), shop tab and default prices. Toy_ToonZ is left
	// out (hidden until unlocked in vanilla, in no shop list).
	{Kind: "Figurine", Title: "Figurines", One: "figurine", Toggle: "ShowVanillaFigurines", Model: true,
		Bases: []string{"Toy_PiggyA", "Toy_GolemA", "Toy_StarfishA", "Toy_BatA", "Toy_PiggyB", "Toy_GolemB", "Toy_StarfishB",
			"Toy_BatB", "Toy_FoxB", "Toy_Beetle", "Toy_PiggyC", "Toy_GolemC", "Toy_StarfishC", "Toy_BatC", "Toy_PiggyD",
			"Toy_GolemD", "Toy_StarfishD", "Toy_BatD", "Toy_PiggyEvo", "Toy_StarfishEvo"}},
	// Tabletop Geek shop: every item has its own model (one layout per base); paint it, or bring your own model. The four UP
	// playmat tubes differ only in the colour of the mat inside (its own face), so UP_Playmat1 stands for all of them.
	{Kind: "BoardGame", Title: "Board games", One: "board game", Toggle: "ShowVanillaBoardGames", OwnModel: true, PerBase: true,
		Bases: []string{"Boardgame_Speedrobo_SystemGate1", "Boardgame_Speedrobo_SystemGate2", "Boardgame_Speedrobo_Mafia",
			"Boardgame_Speedrobo_Necro", "Boardgame_Speedrobo_Claim"}},
	{Kind: "CardSupply", Title: "Card supplies", One: "card supply", Toggle: "ShowVanillaCardSupplies", OwnModel: true, PerBase: true,
		Bases: []string{"UP_PennySleeves", "UP_TowerDeckbox", "UP_MagneticHolder", "UP_Toploader", "UP_CardPreserver",
			"UP_Playmat1", "UP_PlatinumSeriesPocketPages", "UP_SemiRigidCardHolder", "UP_Album"}},
}

func KindInfo(kind string) (AccessoryKind, bool) {
	for _, k := range AccessoryKinds {
		if k.Kind == kind {
			return k, true
		}
	}
	return AccessoryKind{}, false
}

type AccessoryLibrary struct {
	SchemaVersion int          `json:"schemaVersion"`
	Accessories   []Accessory  `json:"accessories"`
	Furniture     []Furniture  `json:"furniture,omitempty"`
	Decorations   []Decoration `json:"decorations,omitempty"`
}

type Accessory struct {
	ID        string           `json:"id"`
	Kind      string           `json:"kind"` // see AccessoryKinds
	Name      string           `json:"name"`
	Base      string           `json:"base,omitempty"`
	Texture   string           `json:"texture,omitempty"`
	Icon      string           `json:"icon,omitempty"`
	Mesh      string           `json:"mesh,omitempty"` // figurines, board games, card supplies: baked model (Unity mesh space, see internal/figurine)
	Shop      string           `json:"shop,omitempty"` // shop list to sell it from (AccessoryShops); "" = the base item's lists
	Cost      *float64         `json:"cost,omitempty"`
	MarketMin *float64         `json:"marketMin,omitempty"`
	MarketMax *float64         `json:"marketMax,omitempty"`
	License   AccessoryLicense `json:"license"`
}

type AccessoryLicense struct {
	Level    int      `json:"level"`
	Price    float64  `json:"price"`
	BigLevel *int     `json:"bigLevel,omitempty"`
	BigPrice *float64 `json:"bigPrice,omitempty"`
}

// HasSmallRow reports whether the base model sells in a small and a big delivery box (deck boxes) or only a big one (all others).
func (a Accessory) HasSmallRow() bool { return a.Kind == "Deckbox" }

// DefaultBase is the model used when Base is empty (same default as the mod).
func (a Accessory) DefaultBase() string {
	if a.Base != "" {
		return a.Base
	}
	if k, ok := KindInfo(a.Kind); ok {
		return k.Bases[0]
	}
	return ""
}

func NewAccessory(kind, id, name, base string) Accessory {
	// Starting license = the cheapest vanilla item of the kind (game 1.02 restock table).
	lic := map[string]AccessoryLicense{
		"Deckbox": {Level: 5, Price: 100}, "Playmat": {Level: 7, Price: 500}, "Sleeve": {Level: 2, Price: 50},
		"Dice": {Level: 3, Price: 50}, "Comic": {Level: 13, Price: 900}, "Binder": {Level: 11, Price: 1000},
		"BattleDeck": {Level: 9, Price: 1000}, "Figurine": {Level: 6, Price: 500},
		"BoardGame": {Level: 5, Price: 200}, "CardSupply": {Level: 10, Price: 800},
	}[kind]
	return Accessory{ID: id, Kind: kind, Name: name, Base: base, License: lic}
}

func LoadAccessories(path string) (*AccessoryLibrary, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var l AccessoryLibrary
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return &l, nil
}

func (l *AccessoryLibrary) Save(path string) error {
	l.SchemaVersion = AccessorySchemaVersion
	if l.Accessories == nil {
		l.Accessories = []Accessory{}
	}
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Validate mirrors AccessoryLoader: errors reject an accessory in game; folder is used to check image files.
// Validate checks the library; resolve turns an image/model path into the file to check (the shared store for
// "assets/…", the library folder otherwise).
func (l *AccessoryLibrary) Validate(resolve func(rel string) string) (errs, warns []string) {
	ids := map[string]bool{}
	for i, a := range l.Accessories {
		where := fmt.Sprintf("accessory %q", a.ID)
		if strings.TrimSpace(a.ID) == "" {
			errs = append(errs, fmt.Sprintf("accessory #%d: missing id", i+1))
			continue
		}
		if strings.ContainsAny(a.ID, " \t:/|") {
			errs = append(errs, where+": id may not contain spaces, ':', '/' or '|'")
		}
		if ids[a.ID] {
			errs = append(errs, where+": duplicate id")
		}
		ids[a.ID] = true
		if k, ok := KindInfo(a.Kind); !ok {
			errs = append(errs, fmt.Sprintf("%s: unknown kind %q", where, a.Kind))
		} else if !contains(k.Bases, a.DefaultBase()) {
			errs = append(errs, fmt.Sprintf("%s: base %q is not a vanilla %s", where, a.Base, k.One))
		}
		if a.Cost != nil && *a.Cost <= 0 {
			errs = append(errs, where+": cost must be > 0")
		}
		for _, img := range []string{a.Texture, a.Icon} {
			if img != "" {
				if _, err := os.Stat(resolve(img)); err != nil {
					warns = append(warns, fmt.Sprintf("%s: image not found %q (vanilla art used)", where, img))
				}
			}
		}
		if a.Shop != "" {
			if _, ok := AccessoryShops[a.Shop]; !ok {
				errs = append(errs, fmt.Sprintf("%s: unknown shop %q", where, a.Shop))
			}
		}
		k, _ := KindInfo(a.Kind)
		if a.Mesh != "" {
			if !k.Model && !k.OwnModel {
				warns = append(warns, where+": 'mesh' is only used by figurines, board games and card supplies (ignored)")
			} else if _, err := os.Stat(resolve(a.Mesh)); err != nil {
				warns = append(warns, fmt.Sprintf("%s: model not found %q (base item's model used)", where, a.Mesh))
			}
		} else if k.Model {
			warns = append(warns, where+": no model imported yet (the base toy's model is used)")
		}
	}
	return
}

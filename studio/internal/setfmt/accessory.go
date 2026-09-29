package setfmt

// Accessory library: mirrors AccessoryLibraryDef/AccessoryDef in the mod's Core/Defs.cs and Core/AccessoryLoader.cs.
// Written to <plugin>\Accessories\accessories.json; see docs/set-format.md "Accessory library".

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
	Kind    string   `json:"kind"`    // Deckbox | Playmat | Sleeve | Dice | Comic | Binder | BattleDeck | Figurine (JSON value in accessories.json)
	Title   string   `json:"title"`   // "Deck boxes"
	One     string   `json:"one"`     // "deck box"
	Toggle  string   `json:"toggle"`  // mod config key in [Content]
	Bases   []string `json:"bases"`   // vanilla items to start from; the first is the default
	Palette bool     `json:"palette"` // texture is a colour palette (recoloured, no artwork)
	Model   bool     `json:"model"`   // own 3D model (figurines): the base only gives the shelf slot, shop tab and prices
}

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
	SchemaVersion int         `json:"schemaVersion"`
	Accessories   []Accessory `json:"accessories"`
}

type Accessory struct {
	ID        string           `json:"id"`
	Kind      string           `json:"kind"` // see AccessoryKinds
	Name      string           `json:"name"`
	Base      string           `json:"base,omitempty"`
	Texture   string           `json:"texture,omitempty"`
	Icon      string           `json:"icon,omitempty"`
	Mesh      string           `json:"mesh,omitempty"` // figurines: baked model (Unity mesh space, see internal/figurine)
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
func (l *AccessoryLibrary) Validate(folder string) (errs, warns []string) {
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
				if _, err := os.Stat(filepath.Join(folder, filepath.FromSlash(img))); err != nil {
					warns = append(warns, fmt.Sprintf("%s: image not found %q (vanilla art used)", where, img))
				}
			}
		}
		if a.Mesh != "" {
			if a.Kind != "Figurine" {
				warns = append(warns, where+": 'mesh' is only used by figurines (ignored)")
			} else if _, err := os.Stat(filepath.Join(folder, filepath.FromSlash(a.Mesh))); err != nil {
				warns = append(warns, fmt.Sprintf("%s: model not found %q (base toy's model used)", where, a.Mesh))
			}
		} else if a.Kind == "Figurine" {
			warns = append(warns, where+": no model imported yet (the base toy's model is used)")
		}
	}
	return
}

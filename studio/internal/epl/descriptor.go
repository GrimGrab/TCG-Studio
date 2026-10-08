// Package epl reads mods made for Enhanced Prefab Loader (EPL, https://gitlab.com/tcg-mods/prefabloader): a JSON
// descriptor X.json next to a Unity asset bundle X. Only the fields Studio converts are read (the shared models live in
// https://gitlab.com/tcg-mods/sharedresources, Models/*); unknown fields are ignored, so EPL additions don't break it.
// The conversion is one-shot: imported content never depends on EPL afterwards.
package epl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Descriptor mirrors EPL's BundleDescriptor (field names match case-insensitively, like EPL's Newtonsoft parsing).
type Descriptor struct {
	BundleID       string          `json:"BundleId"`
	Name           string          `json:"Name"`
	Items          []Item          `json:"Items"`
	Prefabs        []Prefab        `json:"Prefabs"`
	CardExpansions []CardExpansion `json:"CardExpansions"`
	CustomShops    []json.RawMessage
}

type CardExpansion struct {
	Name             string   `json:"Name"`
	MenuCategory     string   `json:"MenuCategory"`
	CardExpansion    string   `json:"CardExpansion"` // the expansion's enum name, which pack items refer to
	CardbackSprite   string   `json:"CardbackSprite"`
	HasRandomFoils   bool     `json:"HasRandomFoils"`
	FoilChance       float64  `json:"FoilChance"`
	Rarities         []string `json:"Rarities"`
	Cards            []Card   `json:"Cards"`
	TradeUnlockLevel int      `json:"TradeUnlockLevel"`
	// Card price generator (see price.go): "Default" / "RarityDriven" (or 1 / 2) and its settings.
	CardPriceStrategy                string  `json:"CardPriceStrategy"`
	DefaultBorderMultiplierBase      float64 `json:"DefaultBorderMultiplierBase"`
	DefaultFoilMultiplier            float64 `json:"DefaultFoilMultiplier"`
	RarityDrivenFloor                float64 `json:"RarityDrivenFloor"`
	RarityDrivenStepSize             float64 `json:"RarityDrivenStepSize"`
	RarityDrivenBorderMultiplierBase float64 `json:"RarityDrivenBorderMultiplierBase"`
	RarityDrivenFoilMultiplier       float64 `json:"RarityDrivenFoilMultiplier"`
}

type Card struct {
	Name        string `json:"Name"`
	ElementType string `json:"ElementType"`
	Rarity      string `json:"Rarity"`
	CardNumber  int    `json:"CardNumber"`
	Sprite      string `json:"Sprite"`
	BorderType  string `json:"BorderType"`
	IsFoil      bool   `json:"IsFoil"`
	Cardback    string `json:"Cardback"`
}

type Vec3 struct{ X, Y, Z float64 }

type Item struct {
	Name                    string   `json:"Name"`
	ItemCategory            string   `json:"ItemCategory"`
	ItemType                string   `json:"ItemType"`
	SpriteName              string   `json:"SpriteName"`
	UsesBaseGameMesh        bool     `json:"UsesBaseGameMesh"`
	MeshToUse               string   `json:"MeshToUse"`
	Mesh                    string   `json:"Mesh"`
	Material                string   `json:"Material"`
	MaterialList            []string `json:"MaterialList"`
	BaseCost                float64  `json:"BaseCost"`
	LicensePrice            float64  `json:"LicensePrice"`
	LicenseLevelRequirement int      `json:"LicenseLevelRequirement"`
	ItemDeminsion           Vec3     `json:"ItemDeminsion"` // EPL's spelling
	IsTallItem              bool     `json:"IsTallItem"`
	AddItemAsAccessory      bool     `json:"AddItemAsAccessory"`
	AddItemAsFigurine       bool     `json:"AddItemAsFigurine"`
	AddItemAsBoardGame      bool     `json:"AddItemAsBoardGame"`
	IsCardPack              bool     `json:"IsCardPack"`
	IsCardBox               bool     `json:"IsCardBox"`
	CardExpansion           string   `json:"CardExpansion"`
	SpawnsPackType          string   `json:"SpawnsPackType"`
	PackGenerationStrategy  string   `json:"PackGenerationStrategy"`
	CanHaveDuplicates       bool     `json:"CanHaveDuplicates"`
	CanHaveGodPacks         bool     `json:"CanHaveGodPacks"`
	NormalWeights           map[string]float64
	GodWeights              map[string]float64
	SlotWeights             map[string]map[string]float64
	GenerationWeights       []LegacyWeight // before 2026-03-28
}

// LegacyWeight is EPL's old per-rarity weight entry (RarityWeightInfo), still accepted by EPL.
type LegacyWeight struct {
	Rarity          string  `json:"Rarity"`
	GuaranteedSlots string  `json:"GuaranteedSlots"`
	NormalWeight    float64 `json:"NormalWeight"`
}

type Prefab struct {
	Name                   string  `json:"Name"`
	PrefabName             string  `json:"PrefabName"`
	SpriteName             string  `json:"SpriteName"`
	Price                  float64 `json:"Price"`
	LevelRequirement       int     `json:"LevelRequirement"`
	AddItemToFurnitureShop bool    `json:"AddItemToFurnitureShop"`
	Description            string  `json:"Description"`
	ObjectType             string  `json:"ObjectType"`
	DecoBonus              float64 `json:"DecoBonus"`
	IsDecoObject           bool    `json:"IsDecoObject"`
	IsDecoWallTexture      bool    `json:"IsDecoWallTexture"`
	IsDecoFloorTexture     bool    `json:"IsDecoFloorTexture"`
	IsDecoCeilingTexture   bool    `json:"IsDecoCeilingTexture"`
	DecoType               string  `json:"DecoType"`
	DecoObject             string  `json:"DecoObject"` // EDecoObject name EPL's prepatcher adds (not used: Studio allocates its own)
	// Wall / floor / ceiling looks: texture names in the bundle, colour (0–1) and smoothness.
	Texture      string  `json:"Texture"`
	NormalMap    string  `json:"NormalMap"`
	RoughnessMap string  `json:"RoughnessMap"`
	Smoothness   float64 `json:"Smoothness"`
	Color        *struct {
		R, G, B, A float64
	} `json:"Color"`
}

// ReadDescriptor parses an EPL descriptor file.
func ReadDescriptor(path string) (*Descriptor, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseDescriptor(b)
}

// ParseDescriptor parses descriptor JSON (UTF-8 BOM allowed).
func ParseDescriptor(b []byte) (*Descriptor, error) {
	// EPL reads descriptors with Newtonsoft, which turns a number in a text field into its digits (some mods write enums
	// as numbers, e.g. "PackGenerationStrategy": 2). Do the same: numbers in fields we read as text become text.
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))))
	dec.UseNumber()
	var raw any
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("not an EPL descriptor: %w", err)
	}
	fixed, err := json.Marshal(numbersAsText(raw, reflect.TypeOf(Descriptor{})))
	if err != nil {
		return nil, err
	}
	var d Descriptor
	if err := json.Unmarshal(fixed, &d); err != nil {
		return nil, fmt.Errorf("not an EPL descriptor: %w", err)
	}
	for i := range d.Items {
		d.Items[i].PackGenerationStrategy = strategyName(d.Items[i].PackGenerationStrategy)
	}
	return &d, nil
}

// numbersAsText turns JSON numbers into strings wherever the Go type t expects a string (walking structs by their json
// names, case-insensitively like encoding/json, plus slices and maps).
func numbersAsText(v any, t reflect.Type) any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		if n, ok := v.(json.Number); ok {
			return n.String()
		}
	case reflect.Slice:
		if a, ok := v.([]any); ok {
			for i := range a {
				a[i] = numbersAsText(a[i], t.Elem())
			}
		}
	case reflect.Map:
		if m, ok := v.(map[string]any); ok {
			for k := range m {
				m[k] = numbersAsText(m[k], t.Elem())
			}
		}
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			break
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := strings.Split(f.Tag.Get("json"), ",")[0]
			if name == "" {
				name = f.Name
			}
			for k := range m {
				if strings.EqualFold(k, name) {
					m[k] = numbersAsText(m[k], f.Type)
				}
			}
		}
	}
	return v
}

// strategyName maps EPL's PackGenerationStrategy enum values written as numbers to their names (Guaranteed = 1,
// AllRandomWeighted, AllRandom, GodPack; prefabloader EnhancedPrefabLoader.Core/Packs/PackGenerators/PackGenerationStrategy.cs).
func strategyName(s string) string {
	switch strings.TrimSpace(s) {
	case "1":
		return "Guaranteed"
	case "2":
		return "AllRandomWeighted"
	case "3":
		return "AllRandom"
	case "4":
		return "GodPack"
	}
	return s
}

// HasContent reports whether the descriptor defines anything (other JSON files in a mod folder parse as empty).
func (d *Descriptor) HasContent() bool {
	return len(d.CardExpansions)+len(d.Items)+len(d.Prefabs) > 0
}

// SlotNames returns a pack item's slot keys in natural order ("Slot2" before "Slot10").
func (it *Item) SlotNames() []string {
	keys := make([]string, 0, len(it.SlotWeights))
	for k := range it.SlotWeights {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ni, ei := trailingNumber(keys[i])
		nj, ej := trailingNumber(keys[j])
		if ei == nil && ej == nil && ni != nj {
			return ni < nj
		}
		return keys[i] < keys[j]
	})
	return keys
}

func trailingNumber(s string) (int, error) {
	i := len(s)
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	return strconv.Atoi(s[i:])
}

// IsFoilTier reports a tier name that EPL mods use for the foil copy of another tier ("… Foil").
func IsFoilTier(r string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(r)), " foil")
}

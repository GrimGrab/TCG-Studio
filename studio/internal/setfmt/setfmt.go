// Package setfmt mirrors the mod's set.json contract (TCG Custom Cards/src/TCGCustomCards/Core/Defs.cs)
// and its validation (Core/SetLoader.cs). Keep both sides in step; see docs/set-format.md.
package setfmt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const CurrentSchemaVersion = 1

var Rarities = []string{"Common", "Rare", "Epic", "Legendary", "SuperLegend"}
var Borders = []string{"Base", "FirstEdition", "Silver", "Gold", "EX", "FullArt"}
var Elements = []string{"Fire", "Earth", "Water", "Wind"}
var FrameTemplates = []string{"Tetramon", "Destiny", "Ghost", "Megabot", "FantasyRPG", "CatJob", "Ascension"}

type Set struct {
	SchemaVersion int           `json:"schemaVersion"`
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	RenderMode    string        `json:"renderMode"`    // FullImage | Framed
	FrameTemplate string        `json:"frameTemplate"` // vanilla expansion name
	CardBack      string        `json:"cardBack,omitempty"`
	Mtg           *SetMtg       `json:"mtg,omitempty"`      // real MTG set: lets the mod export decks to Forge
	Rarities      []Rarity      `json:"rarities,omitempty"` // the set's own rarities, lowest first; empty = vanilla ones
	PriceDefaults PriceDefaults `json:"priceDefaults"`
	// Variants: the card versions the set has ("Base", "Base_foil", "FullArt_foil", …); empty = all 12. Mirrors the mod's
	// SetDef.Variants: others are never generated and are left out of the binder and Check Price unless owned.
	Variants []string `json:"variants,omitempty"`
	Packs    []Pack   `json:"packs"`
	Cards    []Card   `json:"cards"`
}

type PriceDefaults struct {
	BorderMultipliers []float64 `json:"borderMultipliers"`
	FoilMultiplier    float64   `json:"foilMultiplier"`
	Minimum           float64   `json:"minimum"`
}

type Card struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Artist      string    `json:"artist"`
	Rarity      string    `json:"rarity"` // id from Set.Rarities (vanilla name when the set has none)
	Number      string    `json:"number,omitempty"`
	Image       string    `json:"image"`
	Price       CardPrice `json:"price"`
	Play        Play      `json:"play"`
	Mtg         *CardMtg  `json:"mtg,omitempty"`
}

// SetMtg marks a set as real Magic: The Gathering cards.
type SetMtg struct {
	SetCode string `json:"setCode"` // Forge/Scryfall edition code, upper case (DOM)
}

// CardMtg is what the mod needs to export a card to a Forge deck (MTG mode).
type CardMtg struct {
	Name     string   `json:"name"`               // name Forge knows: front face for DFC/adventure, "A // B" for split cards
	TypeLine string   `json:"typeLine,omitempty"` // full Scryfall type line
	ManaCost string   `json:"manaCost,omitempty"` // front face, e.g. {1}{B}
	Colors   []string `json:"colors,omitempty"`   // W U B R G
	// Deck builder filters (mod's MTG deck builder)
	Rarity    string  `json:"rarity,omitempty"`    // Scryfall rarity: common, uncommon, rare, mythic (special, bonus)
	CMC       float64 `json:"cmc,omitempty"`       // mana value
	Power     string  `json:"power,omitempty"`     // creatures only; may be "*"
	Toughness string  `json:"toughness,omitempty"` // creatures only
}

type CardPrice struct {
	Base              float64            `json:"base"`
	FoilMultiplier    *float64           `json:"foilMultiplier,omitempty"`
	BorderMultipliers []float64          `json:"borderMultipliers,omitempty"`
	Overrides         map[string]float64 `json:"overrides,omitempty"`
}

type Play struct {
	LaneAttack  []int           `json:"laneAttack"`
	Element     string          `json:"element"`
	EvolvesFrom string          `json:"evolvesFrom,omitempty"`
	Effect      json.RawMessage `json:"effect,omitempty"`
}

type Pack struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	BoxName         string             `json:"boxName,omitempty"`
	CardsPerPack    int                `json:"cardsPerPack"`
	Starter         bool               `json:"starter"`
	HasBox          bool               `json:"hasBox"`
	PackTexture     string             `json:"packTexture,omitempty"`
	PackIcon        string             `json:"packIcon,omitempty"`
	BoxTexture      string             `json:"boxTexture,omitempty"`
	BoxIcon         string             `json:"boxIcon,omitempty"`
	PackCost        float64            `json:"packCost"`
	BoxCost         *float64           `json:"boxCost,omitempty"`
	MarketMin       float64            `json:"marketMin"`
	MarketMax       float64            `json:"marketMax"`
	License         License            `json:"license"`
	Slots           []Slot             `json:"slots"`
	FoilChance      float64            `json:"foilChance"`
	BorderOdds      map[string]float64 `json:"borderOdds"`
	AllowDuplicates bool               `json:"allowDuplicates"`
	Cards           []string           `json:"cards"`
}

// License mirrors LicenseDef: small/big delivery rows for the pack and the box. Big rows are optional
// (the mod defaults them to level +1, price ×1.5).
type License struct {
	PackLevel    int      `json:"packLevel"`
	PackPrice    float64  `json:"packPrice"`
	PackBigLevel *int     `json:"packBigLevel,omitempty"`
	PackBigPrice *float64 `json:"packBigPrice,omitempty"`
	BoxLevel     int      `json:"boxLevel"`
	BoxPrice     float64  `json:"boxPrice"`
	BoxBigLevel  *int     `json:"boxBigLevel,omitempty"`
	BoxBigPrice  *float64 `json:"boxBigPrice,omitempty"`
}

type Slot struct {
	Count   int                `json:"count"`
	Weights map[string]float64 `json:"weights"`
}

// NewSet returns a set with the mod's defaults.
func NewSet(id, name string) *Set {
	return &Set{
		SchemaVersion: CurrentSchemaVersion, ID: id, Name: name,
		RenderMode: "Framed", FrameTemplate: "Tetramon",
		PriceDefaults: DefaultPriceDefaults(), Packs: []Pack{}, Cards: []Card{},
	}
}

func DefaultPriceDefaults() PriceDefaults {
	return PriceDefaults{BorderMultipliers: []float64{1, 1.25, 1.5, 2, 3, 5}, FoilMultiplier: 2.5, Minimum: 0.05}
}

func DefaultPlay() Play { return Play{LaneAttack: []int{1, 1, 1, 1}, Element: "Fire"} }

// NewPack returns a pack with the mod's defaults (vanilla-like odds when Slots is empty).
// MaxCardsPerPack mirrors the mod's PackSizePatches.MaxCards (the opening's final reveal fits 3 rows of 8).
const MaxCardsPerPack = 24

func NewPack(id, name string) Pack {
	return Pack{
		ID: id, Name: name, CardsPerPack: 7, HasBox: true, PackCost: 1.5, MarketMin: 1.5, MarketMax: 2,
		License: License{PackLevel: 1, PackPrice: 100, BoxLevel: 3, BoxPrice: 200},
		Slots:   []Slot{}, FoilChance: 5,
		BorderOdds: map[string]float64{"FullArt": 0.25, "EX": 1, "Gold": 4, "Silver": 8, "FirstEdition": 20},
		Cards:      []string{},
	}
}

func Load(path string) (*Set, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	b = []byte(strings.TrimPrefix(string(b), "\uFEFF"))
	var s Set
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	s.normalize()
	return &s, nil
}

func (s *Set) Save(path string) error {
	b, err := s.Marshal()
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Marshal returns the set.json bytes Save writes (after filling defaults).
func (s *Set) Marshal() ([]byte, error) {
	s.normalize()
	return json.MarshalIndent(s, "", "  ")
}

// normalize fills defaults the mod would apply, so the editor always sees complete data.
func (s *Set) normalize() {
	if s.SchemaVersion == 0 {
		s.SchemaVersion = CurrentSchemaVersion
	}
	if s.RenderMode == "" {
		s.RenderMode = "Framed"
	}
	if s.FrameTemplate == "" {
		s.FrameTemplate = "Tetramon"
	}
	if len(s.PriceDefaults.BorderMultipliers) != 6 {
		s.PriceDefaults = DefaultPriceDefaults()
	}
	if s.Cards == nil {
		s.Cards = []Card{}
	}
	if s.Packs == nil {
		s.Packs = []Pack{}
	}
	for i := range s.Rarities {
		r := &s.Rarities[i]
		if r.Name == "" {
			r.Name = r.ID
		}
	}
	for i := range s.Cards {
		c := &s.Cards[i]
		if c.Rarity == "" {
			c.Rarity = s.RarityList()[0].ID
		}
		if len(c.Play.LaneAttack) != 4 {
			c.Play.LaneAttack = []int{1, 1, 1, 1}
		}
		if c.Play.Element == "" {
			c.Play.Element = "Fire"
		}
	}
	for i := range s.Packs {
		p := &s.Packs[i]
		if p.CardsPerPack == 0 {
			p.CardsPerPack = 7
		}
		// Slots decide the pack's size: cardsPerPack is their total (it only matters on its own for packs without slots).
		if n := SlotTotal(p.Slots); n > 0 {
			p.CardsPerPack = n
		}
		if p.Slots == nil {
			p.Slots = []Slot{}
		}
		if p.Cards == nil {
			p.Cards = []string{}
		}
		if p.BorderOdds == nil {
			p.BorderOdds = NewPack("", "").BorderOdds
		}
	}
}

// Issue is a validation finding. Errors make the mod reject the set; warnings are logged by the mod.
type Issue struct {
	Level   string `json:"level"` // error | warning
	Where   string `json:"where"`
	Message string `json:"message"`
}

func SafeID(id string) bool { return id != "" && !strings.ContainsAny(id, " \t:/|") }

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Validate mirrors SetLoader.Validate. folders are where image paths are looked up, in order (the project folder, then the
// shared card-art library); none or only "" skips the file checks.
func (s *Set) Validate(folders ...string) []Issue {
	var out []Issue
	add := func(level, where, format string, args ...any) {
		out = append(out, Issue{Level: level, Where: where, Message: fmt.Sprintf(format, args...)})
	}
	for _, v := range s.Variants {
		if !IsVariant(v) {
			add("error", "set", "unknown card version %q (Base, FirstEdition, Silver, Gold, EX, FullArt, each optionally + _foil)", v)
		}
	}
	fileMissing := func(rel string) bool {
		if rel == "" {
			return false
		}
		checked := false
		for _, folder := range folders {
			if folder == "" {
				continue
			}
			checked = true
			if _, err := os.Stat(filepath.Join(folder, rel)); err == nil {
				return false
			}
		}
		return checked
	}

	if s.SchemaVersion > CurrentSchemaVersion {
		add("error", "set", "schemaVersion %d is newer than supported (%d)", s.SchemaVersion, CurrentSchemaVersion)
	}
	if !SafeID(s.ID) {
		add("error", "set", "set id %q is missing or contains spaces, ':', '/', '|'", s.ID)
	}
	if s.RenderMode != "FullImage" && s.RenderMode != "Framed" {
		add("error", "set", "renderMode must be FullImage or Framed")
	}
	if !contains(FrameTemplates, s.FrameTemplate) {
		add("error", "set", "frameTemplate %q is not a usable vanilla expansion", s.FrameTemplate)
	}
	if len(s.PriceDefaults.BorderMultipliers) != 6 {
		add("error", "set", "priceDefaults.borderMultipliers must have 6 entries")
	}
	if len(s.Cards) == 0 {
		add("error", "set", "no cards")
	}

	rarityIDs := map[string]bool{}
	for i, r := range s.Rarities {
		w := "rarity " + r.ID
		if r.ID == "" {
			add("error", "set", "rarity #%d: missing id", i+1)
			continue
		}
		if !SafeID(r.ID) {
			add("error", w, "id may not contain spaces, ':', '/', '|'")
		}
		if rarityIDs[r.ID] {
			add("error", w, "duplicate id")
		}
		rarityIDs[r.ID] = true
	}
	knownRarity := func(id string) bool {
		if s.OwnRarities() {
			return rarityIDs[id]
		}
		return contains(Rarities, id)
	}

	ids := map[string]bool{}
	for _, c := range s.Cards {
		w := "card " + c.ID
		if c.ID == "" {
			add("error", "card", "card with missing id")
			continue
		}
		if ids[c.ID] {
			add("error", w, "duplicate id")
		}
		ids[c.ID] = true
		if !SafeID(c.ID) {
			add("error", w, "id may not contain spaces, ':', '/', '|'")
		}
		if !knownRarity(c.Rarity) {
			add("error", w, "unknown rarity %q", c.Rarity)
		} else if c.Rarity == "SuperLegend" && !s.OwnRarities() {
			add("warning", w, "SuperLegend has no rarity icon in the game (shows as Common) — use Legendary")
		}
		if c.Price.BorderMultipliers != nil && len(c.Price.BorderMultipliers) != 6 {
			add("error", w, "price.borderMultipliers must have 6 entries")
		}
		if len(c.Play.LaneAttack) != 4 {
			add("error", w, "play.laneAttack must have 4 entries")
		}
		if !contains(Elements, c.Play.Element) {
			add("warning", w, "element %q is not Fire/Earth/Water/Wind", c.Play.Element)
		}
		if c.Image == "" {
			add("warning", w, "no image")
		} else if fileMissing(c.Image) {
			add("warning", w, "image not found %q", c.Image)
		}
	}
	for _, c := range s.Cards {
		if c.Play.EvolvesFrom != "" && !ids[c.Play.EvolvesFrom] {
			add("error", "card "+c.ID, "evolvesFrom %q not found in set", c.Play.EvolvesFrom)
		}
	}

	packIDs := map[string]bool{}
	for _, p := range s.Packs {
		w := "pack " + p.ID
		if !SafeID(p.ID) {
			add("error", w, "pack id missing or contains spaces, ':', '/', '|'")
		}
		if packIDs[p.ID] {
			add("error", w, "duplicate id")
		}
		packIDs[p.ID] = true
		size := p.CardsPerPack
		if n := SlotTotal(p.Slots); n > 0 {
			size = n // slots decide the pack's size
		}
		if size < 1 || size > MaxCardsPerPack {
			add("error", w, "a pack must have 1–%d cards (slots add up to %d)", MaxCardsPerPack, size)
		}
		for _, sl := range p.Slots {
			for r := range sl.Weights {
				if !knownRarity(r) {
					add("error", w, "unknown rarity %q in slot weights", r)
				}
			}
		}
		for b := range p.BorderOdds {
			if !contains(Borders, b) {
				add("error", w, "unknown border %q in borderOdds", b)
			}
		}
		for _, id := range p.Cards {
			if !ids[id] {
				add("error", w, "card %q not in set", id)
			}
		}
		for _, img := range []string{p.PackTexture, p.PackIcon, p.BoxTexture, p.BoxIcon} {
			if fileMissing(img) {
				add("warning", w, "image not found %q (vanilla art used)", img)
			}
		}
	}
	return out
}

// SlotTotal is the number of cards a pack's slots give (0 = no slots).
func SlotTotal(slots []Slot) int {
	n := 0
	for _, s := range slots {
		n += s.Count
	}
	return n
}

// IsVariant reports whether v names one of the 12 card versions: a border, optionally + "_foil" (case-insensitive, like the mod).
func IsVariant(v string) bool {
	b := strings.TrimSuffix(strings.TrimSuffix(v, "_foil"), "_Foil")
	for _, x := range Borders {
		if strings.EqualFold(x, b) {
			return true
		}
	}
	return false
}

// AllowsVariant reports whether the set has this version (no list = all 12).
func (s *Set) AllowsVariant(border string, foil bool) bool {
	if len(s.Variants) == 0 {
		return true
	}
	want := border
	if foil {
		want += "_foil"
	}
	for _, v := range s.Variants {
		if strings.EqualFold(v, want) {
			return true
		}
	}
	return false
}

package importer

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"tcgstudio/internal/project"
)

// ---------------------------------------------------------------- Flesh and Blood (TCGplayer catalog via TCGCSV)
// Category 62. Images are Legend Story Studios' own renders (no watermark, already the 63×88 shape); prices per printing
// ("Unlimited Edition Normal", "1st Edition Rainbow Foil", …). Heroes and weapons are filed under rarity "Token".

const fabCategory = 62

type fabSource struct{ t *tcgcsv }

func newFabSource() *fabSource { return &fabSource{newTCGCSV()} }

// FabRarityOrder ranks TCGplayer's Flesh and Blood rarities lowest first ("None" = unlabelled printings).
var FabRarityOrder = []string{"None", "Basic", "Token", "Common", "Rare", "Promo", "Super Rare", "Majestic", "Pirate Booty",
	"Legendary", "Gold", "Fabled", "Marvel"}

func DefaultFabRarityMap() map[string]string {
	return ladderMap(map[string][]string{
		"Common": {"None", "Basic", "Token", "Common"}, "Rare": {"Rare", "Promo"},
		"Epic": {"Super Rare", "Majestic", "Pirate Booty"}, "Legendary": {"Legendary", "Gold", "Fabled", "Marvel"},
	})
}

// fabClasses in the filter's order (Generic first, then the classes by how many cards they have).
var fabClasses = []string{"Generic", "Warrior", "Guardian", "Runeblade", "Brute", "Ninja", "Mechanologist", "Illusionist",
	"Assassin", "Ranger", "Wizard", "Pirate", "Necromancer", "Bard", "Adjudicator", "Merchant", "Thief", "Shapeshifter"}

func (s *fabSource) Info() SourceInfo {
	var colors []Facet
	for _, c := range fabClasses {
		colors = append(colors, Facet{c, c})
	}
	colors = append(colors, Facet{"M", "Multi-class"}, Facet{"C", "No class"})
	return SourceInfo{ID: "fab", Name: "TCGplayer", Game: "Flesh and Blood", ColorLabel: "Class", Colors: colors,
		RarityOrder: FabRarityOrder, Sorts: []Facet{{"cost", "Cost"}, {"power", "Power"}}, Variants: true}
}

func (s *fabSource) DefaultOptions() Options {
	return Options{IncludeVariants: true, ImageWidth: 512, RarityMap: DefaultFabRarityMap()}
}

// fabGroup sorts a TCGplayer group into the list: booster sets are the main sets.
func fabGroup(g tcgGroup) (string, bool) {
	n := strings.ToLower(g.Name)
	for _, w := range []string{"promo", "pre-release", "prerelease", "release event", "judge", "championship", "nationals",
		"calling", "pro quest", "battle hardened", "road to", "gem pack", "box topper", "cards", "token"} {
		if strings.Contains(n, w) {
			return "Promos & other", false
		}
	}
	for _, w := range []string{"deck", "blitz", "armory", "hero", "history pack", "mastery pack", "classic battles", "compendium"} {
		if strings.Contains(n, w) {
			return "Decks & packs", false
		}
	}
	if g.Abbreviation == "" {
		return "Promos & other", false
	}
	return "Booster sets", true
}

func (s *fabSource) Sets(ctx context.Context, _ string) ([]SetInfo, error) {
	gs, err := s.t.groups(ctx, fabCategory)
	if err != nil {
		return nil, err
	}
	tcgSortGroups(gs)
	var out []SetInfo
	for _, g := range gs {
		group, main := fabGroup(g)
		out = append(out, SetInfo{Code: tcgCode(g, gs), Name: g.Name, Group: group, ReleasedAt: date10(g.PublishedOn), Main: main})
	}
	return out, nil
}

func (s *fabSource) ProjectID(code, _ string) string { return "fab-" + slug(code) }

// fabPitch names a pitch value the way the game prints it on the card ("1" → "Red").
func fabPitch(v string) string {
	switch v {
	case "1":
		return "Red"
	case "2":
		return "Yellow"
	case "3":
		return "Blue"
	}
	return ""
}

func fabElement(pitch string) string {
	switch pitch {
	case "1":
		return "Fire"
	case "2":
		return "Wind"
	case "3":
		return "Water"
	}
	return "Earth"
}

// fabKeepToken: of the "Token" rarity, heroes, weapons and equipment are real cards; plain tokens, macros, resources aren't.
func fabKeepToken(cardType string) bool {
	for _, t := range []string{"Hero", "Weapon", "Equipment"} {
		if strings.Contains(cardType, t) {
			return true
		}
	}
	return false
}

func (s *fabSource) cards(ctx context.Context, g *tcgGroup, variants bool) ([]cardIn, error) {
	products, err := s.t.products(ctx, fabCategory, g.ID)
	if err != nil {
		return nil, err
	}
	prices, err := s.t.prices(ctx, fabCategory, g.ID)
	if err != nil {
		return nil, err
	}
	var out []cardIn
	for i := range products {
		p := &products[i]
		number, rarity, ctype := p.ext("Number"), p.ext("Rarity"), p.ext("CardType")
		if number == "" {
			continue
		}
		if rarity == "" {
			rarity = "None"
		}
		if rarity == "Token" && !fabKeepToken(ctype) {
			continue
		}
		name, tags := tcgName(p.Name)
		var variant []string
		for _, t := range tags {
			if t == "Red" || t == "Yellow" || t == "Blue" {
				name += " (" + t + ")" // the pitch is part of the card, not a printing
			} else {
				variant = append(variant, t)
			}
		}
		if len(variant) > 0 && !variants {
			continue
		}
		var classes []string
		for _, c := range strings.FieldsFunc(p.ext("Class"), func(r rune) bool { return r == ';' || r == '/' }) {
			if c = strings.TrimSpace(c); c != "" {
				classes = append(classes, c)
			}
		}
		pitch := p.ext("Pitch Value")
		tl := strings.ReplaceAll(ctype, ";", " ")
		if sub := p.ext("CardSubType"); sub != "" {
			tl += " — " + strings.ReplaceAll(sub, ";", ", ")
		}
		if c := fabPitch(pitch); c != "" {
			tl += " · " + c + " (" + pitch + ")"
		}
		text := tcgText(p.ext("Description"))
		if f := tcgText(p.ext("Flavor Text")); f != "" {
			text += "\n\n" + f
		}
		ci := cardIn{SourceID: strconv.Itoa(p.ID), Name: name, Number: number, Text: strings.TrimSpace(text), SrcRarity: rarity,
			TypeLine: tl, Colors: classes, Power: p.ext("Power"), Image: p.image(), Element: fabElement(pitch), Variant: variant}
		ci.Cost, _ = strconv.ParseFloat(p.ext("Cost"), 64)
		ci.USD, ci.USDFoil = tcgPrices(prices[p.ID],
			[]string{"Unlimited Edition Normal", "1st Edition Normal", "Normal"},
			[]string{"Unlimited Edition Rainbow Foil", "1st Edition Rainbow Foil", "Rainbow Foil", "Foil"})
		out = append(out, ci)
	}
	// Set order: by number, the regular printing before its variants.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Number != out[j].Number {
			return naturalLess(out[i].Number, out[j].Number)
		}
		if (len(out[i].Variant) == 0) != (len(out[j].Variant) == 0) {
			return len(out[i].Variant) == 0
		}
		a, _ := strconv.Atoi(out[i].SourceID)
		b, _ := strconv.Atoi(out[j].SourceID)
		return a < b
	})
	return out, nil
}

func (s *fabSource) Import(ctx context.Context, ws project.Workspace, code string, opt Options, report func(Progress)) (*project.Project, error) {
	g, _, err := s.t.tcgFind(ctx, fabCategory, code, "Flesh and Blood")
	if err != nil {
		return nil, err
	}
	report(Progress{Stage: "cards", Message: "Fetching card list…"})
	cards, err := s.cards(ctx, g, opt.IncludeVariants)
	if err != nil {
		return nil, err
	}
	if opt.RarityMap == nil {
		opt.RarityMap = DefaultFabRarityMap()
	}
	return buildProject(ctx, ws, setIn{ID: s.ProjectID(code, ""), Source: "fab", Code: code, Name: g.Name,
		ReleasedAt: date10(g.PublishedOn), Cards: cards, Get: s.t.c.Download, Aspect: CardAspect,
		Rarity: rarityMapper(opt.RarityMap, func(string) string { return "Rare" }),
		// Flesh and Blood boosters: commons, rares, then a majestic or better (plus a rainbow foil in every pack).
		Slots: Fab.Slots, FoilChance: Fab.FoilChance}, opt, report)
}

func (s *fabSource) RefreshMeta(ctx context.Context, p *project.Project) (int, error) {
	g, _, err := s.t.tcgFind(ctx, fabCategory, p.Meta.SetCode, "Flesh and Blood")
	if err != nil {
		return 0, err
	}
	cards, err := s.cards(ctx, g, true)
	if err != nil {
		return 0, err
	}
	return refreshFrom(p, cards), nil
}

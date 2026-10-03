package importer

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

// ---------------------------------------------------------------- One Piece Card Game (TCGplayer catalog via TCGCSV)
// Source id "optcg" is kept from the first version, which used optcgapi.com — its (and Bandai's) card images carry a
// "SAMPLE" watermark, so cards, prices and scans now come from TCGplayer (category 68). Sets imported with the old
// version have to be imported again.

const opCategory = 68

type onePieceSource struct{ t *tcgcsv }

func newOnePieceSource() *onePieceSource { return &onePieceSource{newTCGCSV()} }

// OnePieceRarityOrder ranks rarities lowest first; "Parallel" = alternate-art printings (Parallel, Box Topper, Manga…).
var OnePieceRarityOrder = []string{"C", "UC", "R", "P", "L", "SR", "SEC", "SP", "TR", "Parallel"}

func DefaultOnePieceRarityMap() map[string]string {
	return ladderMap(map[string][]string{
		"Common": {"C"}, "Rare": {"UC"}, "Epic": {"R", "P", "L"}, "Legendary": {"SR", "SEC", "SP", "TR", "Parallel"},
	})
}

var onePieceColors = []string{"Red", "Green", "Blue", "Purple", "Black", "Yellow"}

func (s *onePieceSource) Info() SourceInfo {
	var colors []Facet
	for _, c := range onePieceColors {
		colors = append(colors, Facet{c, c})
	}
	colors = append(colors, Facet{"M", "Multicolor"})
	return SourceInfo{ID: "optcg", Name: "TCGplayer", Game: "One Piece Card Game", ColorLabel: "Color", Colors: colors,
		RarityOrder: OnePieceRarityOrder, Sorts: []Facet{{"cost", "Cost"}, {"power", "Power"}}, Variants: true}
}

func (s *onePieceSource) DefaultOptions() Options {
	return Options{IncludeVariants: true, ImageWidth: 512, RarityMap: DefaultOnePieceRarityMap()}
}

// opGroup sorts TCGplayer's One Piece groups for the list: boosters (OP, EB, PRB) are the main sets.
func opGroup(g tcgGroup) (string, bool) {
	a := strings.ToUpper(g.Abbreviation)
	switch {
	case strings.Contains(a, " "): // "OP17 RE", "OP13 ANN": event/anniversary cards
		return "Promos & events", false
	case strings.HasPrefix(a, "OP"), strings.HasPrefix(a, "EB"), strings.HasPrefix(a, "PRB"):
		return "Booster sets", true
	case strings.HasPrefix(a, "ST"), strings.HasPrefix(a, "SD"):
		return "Starter decks", false
	}
	return "Promos & events", false
}

func (s *onePieceSource) Sets(ctx context.Context, _ string) ([]SetInfo, error) {
	gs, err := s.t.groups(ctx, opCategory)
	if err != nil {
		return nil, err
	}
	tcgSortGroups(gs)
	var out []SetInfo
	for _, g := range gs {
		group, main := opGroup(g)
		out = append(out, SetInfo{Code: tcgCode(g, gs), Name: g.Name, Group: group, ReleasedAt: date10(g.PublishedOn), Main: main})
	}
	return out, nil
}

func date10(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func (s *onePieceSource) ProjectID(code, _ string) string { return "op-" + slug(code) }

func opElement(colors []string) string {
	if len(colors) == 0 {
		return ""
	}
	switch colors[0] {
	case "Red":
		return "Fire"
	case "Blue":
		return "Water"
	case "Green", "Black":
		return "Earth"
	}
	return "Wind" // Purple, Yellow
}

// cards lists the group's cards (sealed products and DON!! cards left out), alternate arts as variants when wanted.
func (s *onePieceSource) cards(ctx context.Context, g *tcgGroup, variants bool) ([]cardIn, error) {
	products, err := s.t.products(ctx, opCategory, g.ID)
	if err != nil {
		return nil, err
	}
	prices, err := s.t.prices(ctx, opCategory, g.ID)
	if err != nil {
		return nil, err
	}
	var out []cardIn
	for i := range products {
		p := &products[i]
		number, rarity := p.ext("Number"), p.ext("Rarity")
		if number == "" || rarity == "" || rarity == "DON!!" {
			continue
		}
		name, tags := tcgName(p.Name)
		if len(tags) > 0 && !variants {
			continue
		}
		colors := strings.FieldsFunc(p.ext("Color"), func(r rune) bool { return r == ';' || r == '/' || r == ' ' })
		ci := cardIn{SourceID: strconv.Itoa(p.ID), Name: name, Number: number, Text: tcgText(p.ext("Description")),
			SrcRarity: rarity, Colors: colors, Power: p.ext("Power"), Image: p.image(), Element: opElement(colors), Variant: tags}
		if len(tags) > 0 && rarity != "SP" && rarity != "TR" {
			ci.SrcRarity = "Parallel"
		}
		ci.TypeLine = p.ext("CardType")
		if st := p.ext("Subtypes"); st != "" {
			ci.TypeLine += " — " + strings.ReplaceAll(st, ";", ", ")
		}
		ci.Cost, _ = strconv.ParseFloat(p.ext("Cost"), 64)
		ci.USD, ci.USDFoil = tcgPrices(prices[p.ID], []string{"Normal"}, []string{"Foil"}) // rares and up only exist foil
		out = append(out, ci)
	}
	// Set order: by number, the regular printing before its alternate arts (in TCGplayer's listing order).
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

func (s *onePieceSource) Import(ctx context.Context, ws project.Workspace, code string, opt Options, report func(Progress)) (*project.Project, error) {
	g, _, err := s.t.tcgFind(ctx, opCategory, code, "One Piece")
	if err != nil {
		return nil, err
	}
	report(Progress{Stage: "cards", Message: "Fetching card list…"})
	cards, err := s.cards(ctx, g, opt.IncludeVariants)
	if err != nil {
		return nil, err
	}
	if opt.RarityMap == nil {
		opt.RarityMap = DefaultOnePieceRarityMap()
	}
	return buildProject(ctx, ws, setIn{ID: s.ProjectID(code, ""), Source: "optcg", Code: code, Name: g.Name,
		ReleasedAt: date10(g.PublishedOn), Cards: cards, Get: s.t.c.Download, Aspect: CardAspect,
		Rarity: rarityMapper(opt.RarityMap, func(string) string { return "Legendary" }),
		// One Piece boosters: commons, uncommons, then a rare-or-better (leaders, super rares, secrets, parallels).
		Slots: []setfmt.Slot{
			{Count: 4, Weights: map[string]float64{"Common": 1}},
			{Count: 2, Weights: map[string]float64{"Rare": 1}},
			{Count: 1, Weights: map[string]float64{"Epic": 4, "Legendary": 1}},
		}, FoilChance: 5}, opt, report)
}

// RefreshMeta updates prices from TCGplayer.
func (s *onePieceSource) RefreshMeta(ctx context.Context, p *project.Project) (int, error) {
	g, _, err := s.t.tcgFind(ctx, opCategory, p.Meta.SetCode, "One Piece")
	if err != nil {
		return 0, err
	}
	cards, err := s.cards(ctx, g, true)
	if err != nil {
		return 0, err
	}
	return refreshFrom(p, cards), nil
}

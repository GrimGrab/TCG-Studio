package importer

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"tcgstudio/internal/project"
)

// ---------------------------------------------------------------- Union Arena (TCGplayer catalog via TCGCSV)
// Category 81. Cards carry Number ("UE01BT/BLC-1-003"), Rarity, CardType, SeriesName, Affinities, ActivationEnergy
// (colour), RequiredEnergy, ActionPointCost, BattlePointBP ("2000+"), Description and Trigger. Star parallels have their own
// rarity ("Super Rare 2-Star") and a name tag ("SR**"); Action Point cards are left out like One Piece's DON!!.

const uaCategory = 81

type unionArenaSource struct{ t *tcgcsv }

func newUnionArenaSource() *unionArenaSource { return &unionArenaSource{newTCGCSV()} }

// UnionArenaRarityOrder ranks TCGplayer's Union Arena rarities lowest first ("None" = unlabelled promo printings,
// "Union Rare" = promo rarity, stars = parallel arts).
var UnionArenaRarityOrder = []string{"None", "Common", "Uncommon", "Union Rare", "Rare", "Super Rare",
	"Common 1-Star", "Uncommon 1-Star", "Rare 1-Star", "Rare 2-Star", "Super Rare 1-Star", "Super Rare 2-Star",
	"Super Rare 3-Star", "Precious Common", "Precious Secret Rare"}

func DefaultUnionArenaRarityMap() map[string]string {
	return ladderMap(map[string][]string{
		"Common": {"None", "Common"}, "Rare": {"Uncommon"}, "Epic": {"Union Rare", "Rare"},
		"Legendary": {"Super Rare", "Common 1-Star", "Uncommon 1-Star", "Rare 1-Star", "Rare 2-Star", "Super Rare 1-Star",
			"Super Rare 2-Star", "Super Rare 3-Star", "Precious Common", "Precious Secret Rare"},
	})
}

var unionArenaColors = []string{"Red", "Blue", "Green", "Yellow", "Purple"}

func (s *unionArenaSource) Info() SourceInfo {
	var colors []Facet
	for _, c := range unionArenaColors {
		colors = append(colors, Facet{c, c})
	}
	return SourceInfo{ID: "unionarena", Name: "TCGplayer", Game: "Union Arena", ColorLabel: "Color", Colors: colors,
		RarityOrder: UnionArenaRarityOrder, Sorts: []Facet{{"cost", "Energy"}, {"power", "BP"}}, Variants: true}
}

func (s *unionArenaSource) DefaultOptions() Options {
	return Options{IncludeVariants: true, ImageWidth: 512, RarityMap: DefaultUnionArenaRarityMap()}
}

// uaGroup sorts a TCGplayer group into the list: boosters (UExxBT, UEXxxBT, UPxxBT) are the main sets.
func uaGroup(g tcgGroup) (string, bool) {
	a := strings.ToUpper(g.Abbreviation)
	switch {
	case strings.Contains(a, "_"), strings.HasPrefix(a, "UEPR"), strings.HasSuffix(a, "NC"): // _RE, _PRE, promos
		return "Promos & events", false
	case strings.HasSuffix(a, "BT"):
		return "Booster sets", true
	case strings.HasSuffix(a, "ST"), strings.HasSuffix(a, "DC"):
		return "Starter decks", false
	}
	return "Promos & events", false
}

func (s *unionArenaSource) Sets(ctx context.Context, _ string) ([]SetInfo, error) {
	gs, err := s.t.groups(ctx, uaCategory)
	if err != nil {
		return nil, err
	}
	tcgSortGroups(gs)
	var out []SetInfo
	for _, g := range gs {
		group, main := uaGroup(g)
		out = append(out, SetInfo{Code: tcgCode(g, gs), Name: g.Name, Group: group, ReleasedAt: date10(g.PublishedOn), Main: main})
	}
	return out, nil
}

func (s *unionArenaSource) ProjectID(code, _ string) string { return "ua-" + slug(code) }

func uaElement(color string) string {
	switch color {
	case "Red":
		return "Fire"
	case "Blue":
		return "Water"
	case "Green":
		return "Earth"
	}
	return "Wind" // Yellow, Purple
}

func (s *unionArenaSource) cards(ctx context.Context, g *tcgGroup, variants bool) ([]cardIn, error) {
	products, err := s.t.products(ctx, uaCategory, g.ID)
	if err != nil {
		return nil, err
	}
	prices, err := s.t.prices(ctx, uaCategory, g.ID)
	if err != nil {
		return nil, err
	}
	var out []cardIn
	for i := range products {
		p := &products[i]
		number, rarity := p.ext("Number"), p.ext("Rarity")
		if number == "" || rarity == "" || strings.HasPrefix(rarity, "Action Point") {
			continue
		}
		name, tags := tcgName(p.Name)
		for j, t := range tags {
			tags[j] = strings.ReplaceAll(t, "*", "★") // "SR**" → "SR★★"
		}
		if len(tags) > 0 && !variants {
			continue
		}
		var colors []string
		if c := p.ext("ActivationEnergy"); c != "" {
			colors = []string{c}
		}
		tl := p.ext("CardType")
		if a := p.ext("Affinities"); a != "" {
			tl += " — " + strings.ReplaceAll(a, ";", ", ")
		}
		if sn := p.ext("SeriesName"); sn != "" {
			tl += " · " + sn
		}
		text := tcgText(p.ext("Description"))
		if tr := tcgText(p.ext("Trigger")); tr != "" {
			text += "\n\nTrigger: " + tr
		}
		ci := cardIn{SourceID: strconv.Itoa(p.ID), Name: name, Number: number, Text: strings.TrimSpace(text), SrcRarity: rarity,
			TypeLine: tl, Colors: colors, Power: strings.TrimRight(p.ext("BattlePointBP"), "+"), Image: p.image(),
			Element: uaElement(p.ext("ActivationEnergy")), Variant: tags}
		ci.Cost, _ = strconv.ParseFloat(p.ext("RequiredEnergy"), 64)
		ci.USD, ci.USDFoil = tcgPrices(prices[p.ID], []string{"Normal"}, []string{"Foil"})
		out = append(out, ci)
	}
	// Set order: by number, the regular printing before its parallels.
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

func (s *unionArenaSource) Import(ctx context.Context, ws project.Workspace, code string, opt Options, report func(Progress)) (*project.Project, error) {
	g, _, err := s.t.tcgFind(ctx, uaCategory, code, "Union Arena")
	if err != nil {
		return nil, err
	}
	report(Progress{Stage: "cards", Message: "Fetching card list…"})
	cards, err := s.cards(ctx, g, opt.IncludeVariants)
	if err != nil {
		return nil, err
	}
	if opt.RarityMap == nil {
		opt.RarityMap = DefaultUnionArenaRarityMap()
	}
	return buildProject(ctx, ws, setIn{ID: s.ProjectID(code, ""), Source: "unionarena", Code: code, Name: g.Name,
		ReleasedAt: date10(g.PublishedOn), Cards: cards, Get: s.t.c.Download, Aspect: CardAspect,
		Rarity: rarityMapper(opt.RarityMap, func(string) string { return "Legendary" }),
		// Union Arena boosters: commons, uncommons, then a rare-or-better (super rares and star parallels).
		Slots: UnionArenaBooster(date10(g.PublishedOn)).Slots, FoilChance: UnionArenaBooster(date10(g.PublishedOn)).FoilChance}, opt, report)
}

// RefreshMeta updates prices from TCGplayer.
func (s *unionArenaSource) RefreshMeta(ctx context.Context, p *project.Project) (int, error) {
	g, _, err := s.t.tcgFind(ctx, uaCategory, p.Meta.SetCode, "Union Arena")
	if err != nil {
		return 0, err
	}
	cards, err := s.cards(ctx, g, true)
	if err != nil {
		return 0, err
	}
	return refreshFrom(p, cards), nil
}

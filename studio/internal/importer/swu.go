package importer

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"tcgstudio/internal/project"
	"tcgstudio/internal/webapi"
)

// ---------------------------------------------------------------- SWU-DB (Star Wars: Unlimited)
// https://www.swu-db.com/api — free, no key; TCGplayer market prices for the normal and foil finish.

const swuAPI = "https://api.swu-db.com/"

type swuSource struct{ c *webapi.Client }

func newSWUSource() *swuSource { return &swuSource{webapi.New("swu-db", 100*time.Millisecond)} }

// SWURarityOrder ranks rarities lowest first; "Showcase" = the showcase leader printings.
var SWURarityOrder = []string{"Special", "Common", "Uncommon", "Rare", "Legendary", "Showcase"}

func DefaultSWURarityMap() map[string]string {
	return ladderMap(map[string][]string{
		"Common": {"Common"}, "Rare": {"Uncommon", "Special"}, "Epic": {"Rare"}, "Legendary": {"Legendary", "Showcase"},
	})
}

var swuAspects = []string{"Vigilance", "Command", "Aggression", "Cunning", "Heroism", "Villainy"}

func (s *swuSource) Info() SourceInfo {
	var colors []Facet
	for _, a := range swuAspects {
		colors = append(colors, Facet{a, a})
	}
	colors = append(colors, Facet{"C", "No aspect"})
	return SourceInfo{ID: "swudb", Name: "SWU-DB", Game: "Star Wars: Unlimited", ColorLabel: "Aspect", Colors: colors,
		RarityOrder: SWURarityOrder, Sorts: []Facet{{"cost", "Cost"}, {"power", "Power"}}, Variants: true}
}

func (s *swuSource) DefaultOptions() Options {
	return Options{IncludeVariants: true, ImageWidth: 512, RarityMap: DefaultSWURarityMap()}
}

type swuSet struct {
	Name      string `json:"fullName"`
	ID        string `json:"setId"`
	Cards     int    `json:"numberCards"`
	Released  string `json:"releaseDate"` // M/D/YY
	IsBaseSet bool   `json:"isBaseSet"`
	Parent    string `json:"parentSetId"`
}

// swuDate turns "7/11/25" into "2025-07-11" ("" when missing or unreadable).
func swuDate(s string) string {
	t, err := time.Parse("1/2/06", s)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func (s *swuSource) sets(ctx context.Context) ([]swuSet, error) {
	var sets []swuSet
	return sets, s.c.GetJSON(ctx, swuAPI+"sets", &sets)
}

func (s *swuSource) Sets(ctx context.Context, _ string) ([]SetInfo, error) {
	sets, err := s.sets(ctx)
	if err != nil {
		return nil, err
	}
	var out []SetInfo
	for _, x := range sets {
		if x.Cards == 0 {
			continue
		}
		g := "Other promos"
		switch {
		case x.IsBaseSet:
			g = "Expansions"
		case x.Parent != "":
			g = "Set promos"
		}
		out = append(out, SetInfo{Code: x.ID, Name: x.Name, Group: g, ReleasedAt: swuDate(x.Released), Cards: x.Cards, Main: x.IsBaseSet})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.ReleasedAt == "") != (b.ReleasedAt == "") {
			return b.ReleasedAt == "" // undated last
		}
		if a.ReleasedAt != b.ReleasedAt {
			return a.ReleasedAt > b.ReleasedAt
		}
		return a.Name < b.Name
	})
	return out, nil
}

func (s *swuSource) ProjectID(code, _ string) string { return "swu-" + slug(code) }

type swuCard struct {
	Set         string   `json:"Set"`
	Number      string   `json:"Number"`
	Name        string   `json:"Name"`
	Subtitle    string   `json:"Subtitle"`
	Type        string   `json:"Type"`
	Aspects     []string `json:"Aspects"`
	Traits      []string `json:"Traits"`
	Cost        string   `json:"Cost"`
	Power       string   `json:"Power"`
	FrontText   string   `json:"FrontText"`
	EpicAction  string   `json:"EpicAction"`
	BackText    string   `json:"BackText"`
	Rarity      string   `json:"Rarity"`
	Artist      string   `json:"Artist"`
	VariantType string   `json:"VariantType"`
	MarketPrice string   `json:"MarketPrice"`
	FoilPrice   string   `json:"FoilPrice"`
	FrontArt    string   `json:"FrontArt"`
}

func swuElement(aspects []string) string {
	for _, a := range aspects {
		switch a {
		case "Aggression":
			return "Fire"
		case "Vigilance":
			return "Water"
		case "Command":
			return "Earth"
		case "Cunning":
			return "Wind"
		}
	}
	return ""
}

// cards lists the set's printings. Foil finishes are left out (the game makes its own foils; their price becomes
// the foil multiplier); Hyperspace/Showcase/other art variants only with variants.
func (s *swuSource) cards(ctx context.Context, code string, variants bool) ([]cardIn, error) {
	var raw []swuCard
	if err := s.c.GetJSON(ctx, swuAPI+"cards/"+url.PathEscape(strings.ToLower(code)), &raw); err != nil {
		var wrapped struct {
			Data []swuCard `json:"data"`
		}
		if err2 := s.c.GetJSON(ctx, swuAPI+"cards/"+url.PathEscape(strings.ToLower(code)), &wrapped); err2 != nil {
			return nil, err
		}
		raw = wrapped.Data
	}
	var out []cardIn
	for _, c := range raw {
		v := c.VariantType
		if strings.Contains(v, "Foil") || (v != "" && v != "Normal" && !variants) {
			continue
		}
		name := c.Name
		if c.Subtitle != "" {
			name += ", " + c.Subtitle
		}
		var text []string
		for _, t := range []string{c.FrontText, c.EpicAction, c.BackText} {
			if t != "" {
				text = append(text, t)
			}
		}
		aspects := dedupe(c.Aspects)
		ci := cardIn{SourceID: c.Set + "-" + c.Number, Name: name, Number: c.Number, Text: strings.Join(text, "\n"),
			Artist: c.Artist, SrcRarity: c.Rarity, Colors: aspects, Power: c.Power, Image: c.FrontArt,
			Element: swuElement(aspects), USD: price(c.MarketPrice), USDFoil: price(c.FoilPrice)}
		ci.Cost, _ = strconv.ParseFloat(c.Cost, 64)
		ci.TypeLine = c.Type
		if len(c.Traits) > 0 {
			ci.TypeLine += " — " + strings.Join(c.Traits, ", ")
		}
		if v != "" && v != "Normal" {
			ci.Variant = []string{v}
			if v == "Showcase" {
				ci.SrcRarity = "Showcase"
			}
		}
		out = append(out, ci)
	}
	sort.SliceStable(out, func(i, j int) bool { return naturalLess(out[i].Number, out[j].Number) })
	return out, nil
}

func dedupe(list []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range list {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func (s *swuSource) Import(ctx context.Context, ws project.Workspace, code string, opt Options, report func(Progress)) (*project.Project, error) {
	sets, err := s.sets(ctx)
	if err != nil {
		return nil, err
	}
	var set *swuSet
	for i := range sets {
		if sets[i].ID == code {
			set = &sets[i]
		}
	}
	if set == nil {
		return nil, fmt.Errorf("Star Wars: Unlimited set %s not found", code)
	}
	report(Progress{Stage: "cards", Message: "Fetching card list…"})
	cards, err := s.cards(ctx, code, opt.IncludeVariants)
	if err != nil {
		return nil, err
	}
	if opt.RarityMap == nil {
		opt.RarityMap = DefaultSWURarityMap()
	}
	return buildProject(ctx, ws, setIn{ID: s.ProjectID(code, ""), Source: "swudb", Code: code, Name: set.Name,
		ReleasedAt: swuDate(set.Released), Cards: cards, Get: s.c.Download, Rotate: true, // leaders and bases are landscape
		Rarity: rarityMapper(opt.RarityMap, func(string) string { return "Rare" }),
		// Star Wars: Unlimited boosters: commons, an uncommon, then a rare or legendary (plus a foil in every pack).
		Slots: SwuBooster.Slots, FoilChance: SwuBooster.FoilChance}, opt, report)
}

func (s *swuSource) RefreshMeta(ctx context.Context, p *project.Project) (int, error) {
	if p.Meta.SetCode == "" {
		return 0, fmt.Errorf("%s was not imported from SWU-DB", p.ID)
	}
	cards, err := s.cards(ctx, p.Meta.SetCode, true)
	if err != nil {
		return 0, err
	}
	return refreshFrom(p, cards), nil
}

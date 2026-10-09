package importer

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"tcgstudio/internal/project"
	"tcgstudio/internal/webapi"
)

// ---------------------------------------------------------------- Lorcast (Disney Lorcana)
// https://lorcast.com/docs/api — free, no key, 50–100 ms between requests; prices usd/usd_foil from TCGplayer.
// Card art is AVIF only (decoded by github.com/gen2brain/avif, saved as PNG).

const lorcastAPI = "https://api.lorcast.com/v0/"

type lorcanaSource struct{ api, cdn *webapi.Client }

func newLorcanaSource() *lorcanaSource {
	return &lorcanaSource{webapi.New("lorcast", 100*time.Millisecond), webapi.New("lorcast", 20*time.Millisecond)}
}

// LorcanaRarityOrder ranks rarities lowest first (Lorcast's "Super_rare" is shown as "Super Rare").
var LorcanaRarityOrder = []string{"Common", "Uncommon", "Rare", "Promo", "Super Rare", "Legendary", "Epic", "Enchanted", "Iconic"}

func DefaultLorcanaRarityMap() map[string]string {
	return ladderMap(map[string][]string{
		"Common": {"Common"}, "Rare": {"Uncommon"}, "Epic": {"Rare", "Promo", "Super Rare"},
		"Legendary": {"Legendary", "Epic", "Enchanted", "Iconic"},
	})
}

var lorcanaInks = []string{"Amber", "Amethyst", "Emerald", "Ruby", "Sapphire", "Steel"}

func (s *lorcanaSource) Info() SourceInfo {
	var colors []Facet
	for _, c := range lorcanaInks {
		colors = append(colors, Facet{c, c})
	}
	colors = append(colors, Facet{"M", "Dual ink"})
	return SourceInfo{ID: "lorcast", Name: "Lorcast", Game: "Disney Lorcana", ColorLabel: "Ink", Colors: colors,
		RarityOrder: LorcanaRarityOrder, Sorts: []Facet{{"cost", "Ink cost"}, {"power", "Strength"}}}
}

func (s *lorcanaSource) DefaultOptions() Options {
	return Options{IncludeVariants: true, ImageWidth: 512, RarityMap: DefaultLorcanaRarityMap()}
}

type lorcastSet struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Released string `json:"released_at"`
}

func (s *lorcanaSource) sets(ctx context.Context) ([]lorcastSet, error) {
	var res struct {
		Results []lorcastSet `json:"results"`
	}
	return res.Results, s.api.GetJSON(ctx, lorcastAPI+"sets", &res)
}

func isNumber(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func (s *lorcanaSource) Sets(ctx context.Context, _ string) ([]SetInfo, error) {
	sets, err := s.sets(ctx)
	if err != nil {
		return nil, err
	}
	var out []SetInfo
	for _, x := range sets {
		main := isNumber(x.Code) // the numbered main sets; promos and special products have letter codes
		g := "Promos & special"
		if main {
			g = "Main sets"
		}
		out = append(out, SetInfo{Code: x.Code, Name: x.Name, Group: g, ReleasedAt: x.Released, Main: main})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ReleasedAt > out[j].ReleasedAt })
	return out, nil
}

func (s *lorcanaSource) ProjectID(code, _ string) string { return "lor-" + slug(code) }

type lorcastCard struct {
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	Ink             string   `json:"ink"`
	Inks            []string `json:"inks"`
	Type            []string `json:"type"`
	Classifications []string `json:"classifications"`
	Text            string   `json:"text"`
	Cost            float64  `json:"cost"`
	Strength        *int     `json:"strength"`
	Rarity          string   `json:"rarity"`
	Illustrators    []string `json:"illustrators"`
	Number          string   `json:"collector_number"`
	ID              string   `json:"id"`
	Prices          struct {
		USD     *string `json:"usd"`
		USDFoil *string `json:"usd_foil"`
	} `json:"prices"`
	ImageURIs struct {
		Digital struct {
			Large  string `json:"large"`
			Normal string `json:"normal"`
		} `json:"digital"`
	} `json:"image_uris"`
}

// lorcanaRarity turns Lorcast's "Super_rare" into "Super Rare".
func lorcanaRarity(r string) string {
	words := strings.Fields(strings.ReplaceAll(r, "_", " "))
	for i, w := range words {
		rs := []rune(w)
		rs[0] = unicode.ToUpper(rs[0])
		words[i] = string(rs)
	}
	return strings.Join(words, " ")
}

func lorcanaElement(inks []string) string {
	if len(inks) == 0 {
		return ""
	}
	switch inks[0] {
	case "Ruby":
		return "Fire"
	case "Sapphire":
		return "Water"
	case "Emerald", "Steel":
		return "Earth"
	}
	return "Wind" // Amber, Amethyst
}

func strp(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *lorcanaSource) cards(ctx context.Context, code string) ([]cardIn, error) {
	var raw []lorcastCard
	if err := s.api.GetJSON(ctx, lorcastAPI+"sets/"+url.PathEscape(code)+"/cards", &raw); err != nil {
		return nil, err
	}
	var out []cardIn
	for _, c := range raw {
		name := c.Name
		if c.Version != "" {
			name += " - " + c.Version
		}
		inks := c.Inks
		if len(inks) == 0 && c.Ink != "" {
			inks = []string{c.Ink}
		}
		img := c.ImageURIs.Digital.Large
		if img == "" {
			img = c.ImageURIs.Digital.Normal
		}
		tl := strings.Join(c.Type, " · ")
		if len(c.Classifications) > 0 {
			tl += " — " + strings.Join(c.Classifications, ", ")
		}
		ci := cardIn{SourceID: c.ID, Name: name, Number: c.Number, Text: c.Text, Artist: strings.Join(c.Illustrators, ", "),
			SrcRarity: lorcanaRarity(c.Rarity), TypeLine: tl, Colors: inks, Cost: c.Cost, Image: img,
			Element: lorcanaElement(inks), USD: price(strp(c.Prices.USD)), USDFoil: price(strp(c.Prices.USDFoil))}
		if c.Strength != nil {
			ci.Power = strconv.Itoa(*c.Strength)
		}
		out = append(out, ci)
	}
	sort.SliceStable(out, func(i, j int) bool { return naturalLess(out[i].Number, out[j].Number) })
	return out, nil
}

func (s *lorcanaSource) Import(ctx context.Context, ws project.Workspace, code string, opt Options, report func(Progress)) (*project.Project, error) {
	sets, err := s.sets(ctx)
	if err != nil {
		return nil, err
	}
	var set *lorcastSet
	for i := range sets {
		if sets[i].Code == code {
			set = &sets[i]
		}
	}
	if set == nil {
		return nil, fmt.Errorf("Lorcana set %s not found", code)
	}
	report(Progress{Stage: "cards", Message: "Fetching card list…"})
	cards, err := s.cards(ctx, code)
	if err != nil {
		return nil, err
	}
	if opt.RarityMap == nil {
		opt.RarityMap = DefaultLorcanaRarityMap()
	}
	return buildProject(ctx, ws, setIn{ID: s.ProjectID(code, ""), Source: "lorcast", Code: code, Name: set.Name,
		ReleasedAt: set.Released, Cards: cards, Get: s.cdn.Download, Rotate: true, // locations are landscape
		Rarity: rarityMapper(opt.RarityMap, func(string) string { return "Legendary" }),
		// Lorcana boosters: commons, uncommons, then rares and up (plus a foil in every pack).
		Slots: Lorcana.Slots, FoilChance: Lorcana.FoilChance}, opt, report)
}

func (s *lorcanaSource) RefreshMeta(ctx context.Context, p *project.Project) (int, error) {
	if p.Meta.SetCode == "" {
		return 0, fmt.Errorf("%s was not imported from Lorcast", p.ID)
	}
	cards, err := s.cards(ctx, p.Meta.SetCode)
	if err != nil {
		return 0, err
	}
	return refreshFrom(p, cards), nil
}

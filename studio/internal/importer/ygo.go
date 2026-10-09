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

// ---------------------------------------------------------------- YGOPRODeck (Yu-Gi-Oh!)
// https://db.ygoprodeck.com/api-guide/ — free, no key, 20 requests/s; images are to be downloaded, not hotlinked.

const ygoAPI = "https://db.ygoprodeck.com/api/v7/"

type ygoSource struct{ c *webapi.Client }

func newYGOSource() *ygoSource { return &ygoSource{webapi.New("ygoprodeck", 60*time.Millisecond)} }

// YGORarityOrder ranks YGOPRODeck set rarities, lowest first (unknown ones sort in the middle).
var YGORarityOrder = []string{"Common", "Short Print", "Super Short Print", "New", "Reprint", "Rare", "Normal Parallel Rare",
	"Duel Terminal Normal Parallel Rare", "Starfoil", "Starfoil Rare", "Shatterfoil Rare", "Mosaic Rare", "Super Rare",
	"Super Parallel Rare", "Duel Terminal Rare Parallel Rare", "Duel Terminal Super Parallel Rare", "Ultra Rare",
	"Ultra Parallel Rare", "Duel Terminal Ultra Parallel Rare", "Ultra Rare (Pharaoh's Rare)", "Gold Rare", "Premium Gold Rare",
	"Ultimate Rare", "Secret Rare", "Gold Secret Rare", "Prismatic Secret Rare", "Platinum Rare", "Platinum Secret Rare",
	"Extra Secret", "Extra Secret Rare", "Collector's Rare", "Ghost/Gold Rare", "Ghost Rare", "Starlight Rare",
	"Quarter Century Secret Rare", "Ultra Secret Rare", "Grand Master Rare", "10000 Secret Rare"}

// DefaultYGORarityMap: commons and short prints → Common, (parallel) rares → Rare, supers → Epic, ultra and above → Legendary.
func DefaultYGORarityMap() map[string]string {
	return ladderMap(map[string][]string{
		"Common": {"Common", "Short Print", "Super Short Print", "New", "Reprint"},
		"Rare": {"Rare", "Normal Parallel Rare", "Duel Terminal Normal Parallel Rare", "Starfoil", "Starfoil Rare",
			"Shatterfoil Rare", "Mosaic Rare"},
		"Epic": {"Super Rare", "Super Parallel Rare", "Duel Terminal Rare Parallel Rare", "Duel Terminal Super Parallel Rare"},
		"Legendary": {"Ultra Rare", "Ultra Parallel Rare", "Duel Terminal Ultra Parallel Rare", "Ultra Rare (Pharaoh's Rare)",
			"Gold Rare", "Premium Gold Rare", "Ultimate Rare", "Secret Rare", "Gold Secret Rare", "Prismatic Secret Rare",
			"Platinum Rare", "Platinum Secret Rare", "Extra Secret", "Extra Secret Rare", "Collector's Rare", "Ghost/Gold Rare",
			"Ghost Rare", "Starlight Rare", "Quarter Century Secret Rare", "Ultra Secret Rare", "Grand Master Rare", "10000 Secret Rare"},
	})
}

func ygoRarityFallback(r string) string {
	l := strings.ToLower(r)
	switch {
	case strings.Contains(l, "secret"), strings.Contains(l, "ultra"), strings.Contains(l, "ghost"), strings.Contains(l, "gold"),
		strings.Contains(l, "platinum"), strings.Contains(l, "starlight"), strings.Contains(l, "collector"):
		return "Legendary"
	case strings.Contains(l, "super"):
		return "Epic"
	case strings.Contains(l, "rare"), strings.Contains(l, "parallel"):
		return "Rare"
	}
	return "Common"
}

// ygoFrames are the card-type filter values, in the order Yu-Gi-Oh! players sort by.
var ygoFrames = []string{"Normal", "Effect", "Ritual", "Fusion", "Synchro", "Xyz", "Pendulum", "Link", "Spell", "Trap", "Token", "Skill"}

func (s *ygoSource) Info() SourceInfo {
	var colors []Facet
	for _, f := range ygoFrames {
		colors = append(colors, Facet{f, f})
	}
	return SourceInfo{ID: "ygoprodeck", Name: "YGOPRODeck", Game: "Yu-Gi-Oh!", ColorLabel: "Card type", Colors: colors,
		RarityOrder: YGORarityOrder, Sorts: []Facet{{"cost", "Level / Rank / Link"}, {"power", "ATK"}}}
}

func (s *ygoSource) DefaultOptions() Options {
	return Options{IncludeVariants: true, ImageWidth: 512, RarityMap: DefaultYGORarityMap()}
}

type ygoSet struct {
	Name    string `json:"set_name"`
	Code    string `json:"set_code"`
	Cards   int    `json:"num_of_cards"`
	TCGDate string `json:"tcg_date"`
	Image   string `json:"set_image"`
}

// ygoKeys gives every set a unique import code: its set code; when sets share a code ("LOB" and its 25th Anniversary
// Edition) the oldest keeps it and the others get their name appended.
func ygoKeys(sets []ygoSet) map[string]ygoSet {
	sorted := append([]ygoSet(nil), sets...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i].TCGDate, sorted[j].TCGDate
		if (a == "") != (b == "") {
			return b == ""
		}
		return a < b
	})
	out := map[string]ygoSet{}
	for _, x := range sorted {
		k := x.Code
		if _, taken := out[k]; taken {
			k += "-" + slug(x.Name)
		}
		out[k] = x
	}
	return out
}

func ygoGroup(x ygoSet) string {
	n := strings.ToLower(x.Name)
	switch {
	case strings.Contains(n, "tin"):
		return "Tins"
	case strings.Contains(n, "deck"):
		return "Decks"
	case strings.Contains(n, "promo"), strings.Contains(n, "prize"), strings.Contains(n, "tournament"),
		strings.Contains(n, "championship"), strings.Contains(n, "participation"), strings.Contains(n, "giveaway"):
		return "Promos"
	case x.Cards < 40:
		return "Small sets"
	}
	return "Booster sets"
}

func (s *ygoSource) sets(ctx context.Context) ([]ygoSet, error) {
	var sets []ygoSet
	return sets, s.c.GetJSON(ctx, ygoAPI+"cardsets.php", &sets)
}

func (s *ygoSource) Sets(ctx context.Context, _ string) ([]SetInfo, error) {
	sets, err := s.sets(ctx)
	if err != nil {
		return nil, err
	}
	var out []SetInfo
	for k, x := range ygoKeys(sets) {
		if x.Cards == 0 {
			continue
		}
		g := ygoGroup(x)
		out = append(out, SetInfo{Code: k, Name: x.Name, Group: g, ReleasedAt: x.TCGDate, Icon: x.Image, Cards: x.Cards,
			Main: g == "Booster sets"})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ReleasedAt != out[j].ReleasedAt {
			return out[i].ReleasedAt > out[j].ReleasedAt
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (s *ygoSource) ProjectID(code, _ string) string { return "ygo-" + slug(code) }

type ygoCard struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	FrameType string `json:"frameType"`
	Desc      string `json:"desc"`
	Race      string `json:"race"`
	Attribute string `json:"attribute"`
	ATK       *int   `json:"atk"`
	Level     int    `json:"level"`
	LinkVal   int    `json:"linkval"`
	Sets      []struct {
		Name   string `json:"set_name"`
		Code   string `json:"set_code"`
		Rarity string `json:"set_rarity"`
		Price  string `json:"set_price"`
	} `json:"card_sets"`
	Images []struct {
		URL string `json:"image_url"`
	} `json:"card_images"`
	Prices []struct {
		TCGPlayer  string `json:"tcgplayer_price"`
		Cardmarket string `json:"cardmarket_price"`
	} `json:"card_prices"`
}

func ygoFrame(f string) []string {
	base, pend := strings.CutSuffix(f, "_pendulum")
	var out []string
	if base != "" {
		out = append(out, strings.ToUpper(base[:1])+base[1:])
	}
	if pend {
		out = append(out, "Pendulum")
	}
	return out
}

func ygoElement(attr string) string {
	switch attr {
	case "FIRE", "DIVINE":
		return "Fire"
	case "WATER":
		return "Water"
	case "EARTH", "DARK":
		return "Earth"
	}
	return "Wind" // WIND, LIGHT, spells/traps
}

// cards returns every printing in the set (one card can be printed in a set at several rarities), in printing-code order.
func (s *ygoSource) cards(ctx context.Context, setName string) ([]cardIn, error) {
	var res struct {
		Data []ygoCard `json:"data"`
	}
	if err := s.c.GetJSON(ctx, ygoAPI+"cardinfo.php?cardset="+url.QueryEscape(setName), &res); err != nil {
		return nil, err
	}
	var out []cardIn
	for _, c := range res.Data {
		img := ""
		if len(c.Images) > 0 {
			img = c.Images[0].URL
		}
		var usd, eur *float64
		if len(c.Prices) > 0 {
			usd, eur = price(c.Prices[0].TCGPlayer), price(c.Prices[0].Cardmarket)
		}
		tl := c.Type
		if c.Race != "" {
			tl += " — " + c.Race
		}
		if c.Attribute != "" {
			tl += " / " + c.Attribute
		}
		ci := cardIn{Name: c.Name, Text: c.Desc, TypeLine: tl, Colors: ygoFrame(c.FrameType), Image: img,
			Element: ygoElement(c.Attribute), Cost: float64(c.Level + c.LinkVal), EUR: eur}
		if c.ATK != nil && strings.Contains(c.Type, "Monster") {
			ci.Power = strconv.Itoa(*c.ATK)
		}
		// One printing per rarity: regional reprints in the same set (LOB-027, LOB-E021, LOB-EN027) collapse onto the
		// English "-EN" code (else the shortest).
		best := map[string]int{}
		for i, p := range c.Sets {
			if p.Name != setName {
				continue
			}
			j, seen := best[p.Rarity]
			if !seen || ygoPreferCode(p.Code, c.Sets[j].Code) {
				best[p.Rarity] = i
			}
		}
		for i, p := range c.Sets {
			if p.Name != setName || best[p.Rarity] != i {
				continue
			}
			x := ci
			x.SourceID, x.Number, x.SrcRarity = p.Code+"|"+p.Rarity, p.Code, p.Rarity
			x.USD = price(p.Price) // this printing's price
			if x.USD == nil {
				x.USD = usd
			}
			out = append(out, x)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Number != out[j].Number {
			return naturalLess(out[i].Number, out[j].Number)
		}
		return rankIn(YGORarityOrder, out[i].SrcRarity) < rankIn(YGORarityOrder, out[j].SrcRarity)
	})
	return out, nil
}

func (s *ygoSource) Import(ctx context.Context, ws project.Workspace, code string, opt Options, report func(Progress)) (*project.Project, error) {
	sets, err := s.sets(ctx)
	if err != nil {
		return nil, err
	}
	x, ok := ygoKeys(sets)[code]
	if !ok {
		return nil, fmt.Errorf("Yu-Gi-Oh! set %s not found", code)
	}
	report(Progress{Stage: "cards", Message: "Fetching card list…"})
	cards, err := s.cards(ctx, x.Name)
	if err != nil {
		return nil, err
	}
	if opt.RarityMap == nil {
		opt.RarityMap = DefaultYGORarityMap()
	}
	return buildProject(ctx, ws, setIn{ID: s.ProjectID(code, ""), Source: "ygoprodeck", Code: code, Name: x.Name,
		ReleasedAt: x.TCGDate, Logo: x.Image, Cards: cards, Get: s.c.Download,
		Aspect: CardAspect, // Yu-Gi-Oh! cards are 59×86 mm, narrower than the game's 63×88 slot
		Rarity: rarityMapper(opt.RarityMap, ygoRarityFallback),
		// Yu-Gi-Oh! boosters: commons, a rare, then a foil (super → ultra → secret).
		Slots: YgoBooster(x.TCGDate).Slots, FoilChance: YgoBooster(x.TCGDate).FoilChance}, opt, report)
}

func (s *ygoSource) RefreshMeta(ctx context.Context, p *project.Project) (int, error) {
	sets, err := s.sets(ctx)
	if err != nil {
		return 0, err
	}
	x, ok := ygoKeys(sets)[p.Meta.SetCode]
	if !ok {
		return 0, fmt.Errorf("Yu-Gi-Oh! set %s not found", p.Meta.SetCode)
	}
	cards, err := s.cards(ctx, x.Name)
	if err != nil {
		return 0, err
	}
	// Sets imported before the art was fitted to the card slot get fitted here (local files only).
	if _, err := FitCardImages(p, CardAspect); err != nil {
		return 0, err
	}
	return refreshFrom(p, cards), nil
}

// ygoPreferCode reports whether printing code a is a better pick than b: English ("-EN") first, then shorter.
func ygoPreferCode(a, b string) bool {
	ea, eb := strings.Contains(a, "-EN"), strings.Contains(b, "-EN")
	if ea != eb {
		return ea
	}
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

// rankIn is r's position in order (unknown = the middle).
func rankIn(order []string, r string) float64 {
	for i, x := range order {
		if x == r {
			return float64(i)
		}
	}
	return float64(len(order)-1)/2 + 0.25 // between two real ranks
}

// naturalLess compares strings with embedded numbers numerically ("OP01-9" < "OP01-10", "2" < "10").
func naturalLess(a, b string) bool {
	for a != "" && b != "" {
		da, db := digits(a), digits(b)
		if da > 0 && db > 0 {
			na, _ := strconv.Atoi(a[:da])
			nb, _ := strconv.Atoi(b[:db])
			if na != nb {
				return na < nb
			}
			a, b = a[da:], b[db:]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func digits(s string) int {
	n := 0
	for n < len(s) && n < 9 && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	return n
}

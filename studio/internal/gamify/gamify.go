// Package gamify re-prices imported sets so they fit the game's economy and progression.
//
// Progression: sets are ordered into tiers; each tier gets a pack/box license (level + price) interpolated from the vanilla
// restock table, and pack costs / card prices that rise with the tier (like Destiny/Ghost/Ascension cost more than Tetramon).
// Card prices (mode):
//   - hybrid (default): a price band per rarity from the game's curve; the card's position inside its band comes from its
//     real Scryfall price rank within that rarity (log scale), so chase cards stay chase cards without breaking the economy.
//   - game: a fixed game-like price per rarity (small deterministic spread per card).
//   - real: real USD prices (clamped), per-card foil multipliers from real foil prices.
package gamify

import (
	"hash/fnv"
	"math"
	"sort"

	"tcgstudio/internal/importer"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

type Settings struct {
	Mode          string  `json:"mode"`          // hybrid | game | real
	BorderCurve   string  `json:"borderCurve"`   // game | gentle
	TierStep      float64 `json:"tierStep"`      // card price growth per tier (0.25 = +25% per tier)
	PackCostScale float64 `json:"packCostScale"` // multiplies the vanilla pack cost of each tier (1 = same as vanilla)
	Progression   bool    `json:"progression"`   // also set license levels/prices, pack costs and market ranges
	MaxLevel      int     `json:"maxLevel"`      // shop level the last set unlocks at when there are more than 9 sets (70 = vanilla Ascension)
}

func DefaultSettings() Settings {
	return Settings{Mode: "game", BorderCurve: "game", TierStep: 0.25, PackCostScale: 1, Progression: true, MaxLevel: 70}
}

// TierData is one step of the vanilla card-product progression.
type TierData struct {
	Name                           string  `json:"name"`
	PackLevel, PackBigLevel        int     `json:"-"`
	BoxLevel, BoxBigLevel          int     `json:"-"`
	PackPrice, PackBigPrice        float64 `json:"-"`
	BoxPrice, BoxBigPrice          float64 `json:"-"`
	PackCost, MarketMin, MarketMax float64 `json:"-"`
}

// Vanilla card products in unlock order (game 1.02). Licenses: restock table (small / big delivery rows for pack and box).
// Pack cost: generated wholesale cost ÷ ~1.1 (the generator rolls ×1–1.3); market range: observed market/cost ratios.
// Ascension has no box in vanilla — its box rows follow the Destiny Legendary pattern.
var vanillaTiers = []TierData{
	{"Basic", 1, 2, 3, 4, 0, 50, 150, 200, 1.5, 1.5, 2},
	{"Rare", 5, 8, 9, 12, 300, 500, 500, 650, 2.0, 1.5, 2},
	{"Epic", 12, 15, 16, 19, 800, 900, 1200, 1500, 4.0, 1.3, 1.8},
	{"Legendary", 20, 26, 23, 30, 2000, 2500, 3000, 3500, 4.5, 1.4, 1.9},
	{"Destiny Basic", 25, 29, 27, 31, 3000, 3500, 4000, 4500, 3.5, 1.6, 2.2},
	{"Destiny Rare", 30, 35, 32, 37, 4500, 5000, 5500, 6000, 5.0, 1.8, 2.4},
	{"Destiny Epic", 40, 48, 45, 55, 5500, 6000, 6500, 7000, 7.5, 2.0, 2.8},
	{"Destiny Legendary", 50, 65, 60, 75, 7500, 8000, 9000, 10000, 10.0, 1.8, 2.4},
	{"Ascension", 70, 85, 75, 90, 10000, 15000, 12000, 16000, 13.0, 2.0, 2.6},
}

// Price bands per game rarity for the Base, non-foil variant (derived from the vanilla price generator's range).
var bands = map[string][2]float64{
	"Common": {0.08, 0.30}, "Rare": {0.18, 0.70}, "Epic": {0.35, 1.60}, "Legendary": {0.70, 4.50}, "SuperLegend": {1.20, 9.00},
}

// Border × foil curves. "game" follows the vanilla generator's steep curve (a Full Art is a big hit); "gentle" is the mod default.
func curve(name string) setfmt.PriceDefaults {
	if name == "gentle" {
		return setfmt.DefaultPriceDefaults()
	}
	return setfmt.PriceDefaults{BorderMultipliers: []float64{1, 5, 14, 30, 100, 380}, FoilMultiplier: 20, Minimum: 0.02}
}

// Preview describes what applying gamify would do to one project.
type Preview struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Tier      int                `json:"tier"`
	License   setfmt.License     `json:"license"`
	PackCost  float64            `json:"packCost"`
	Like      string             `json:"like"` // vanilla product this tier mirrors
	Installed bool               `json:"installed"`
	AvgBefore map[string]float64 `json:"avgBefore"` // by rarity
	AvgAfter  map[string]float64 `json:"avgAfter"`
	Top       []TopCard          `json:"top"` // most valuable cards after
	Locked    int                `json:"locked"`
}

type TopCard struct {
	Name   string  `json:"name"`
	Rarity string  `json:"rarity"`
	Before float64 `json:"before"`
	After  float64 `json:"after"`
}

// Apply re-prices one project: tier is its 1-based order, pos its place on the vanilla curve (see Position).
func Apply(p *project.Project, tier int, pos float64, s Settings) Preview {
	pv := Preview{ID: p.ID, Name: p.Set.Name, Tier: tier, AvgBefore: avgByRarity(p.Set.Cards), AvgAfter: map[string]float64{}}
	before := map[string]float64{}
	for _, c := range p.Set.Cards {
		before[c.ID] = c.Price.Base
	}
	tierMult := 1 + s.TierStep*pos

	if s.Mode != "real" {
		p.Set.PriceDefaults = curve(s.BorderCurve)
	} else {
		p.Set.PriceDefaults = setfmt.DefaultPriceDefaults()
	}

	// Rank by real price within each rarity (hybrid).
	ranks := map[string]float64{}
	if s.Mode == "hybrid" {
		byRarity := map[string][]setfmt.Card{}
		for _, c := range p.Set.Cards {
			byRarity[c.Rarity] = append(byRarity[c.Rarity], c)
		}
		for _, list := range byRarity {
			sort.SliceStable(list, func(i, j int) bool { return realUSD(p, list[i]) < realUSD(p, list[j]) })
			for i, c := range list {
				if len(list) == 1 {
					ranks[c.ID] = 0.5
				} else {
					ranks[c.ID] = float64(i) / float64(len(list)-1)
				}
			}
		}
	}

	for i := range p.Set.Cards {
		c := &p.Set.Cards[i]
		m := p.Meta.Cards[c.ID]
		if m.Locked {
			pv.Locked++
			continue
		}
		switch s.Mode {
		case "real":
			c.Price = importer.RealPrice(m)
			c.Price.Base = round2(math.Min(500, c.Price.Base*tierMult))
		case "game":
			b := band(c.Rarity)
			mid := math.Sqrt(b[0] * b[1])
			c.Price = setfmt.CardPrice{Base: round2(mid * jitter(c.ID) * tierMult)}
		default: // hybrid
			b := band(c.Rarity)
			r, ok := ranks[c.ID]
			if !ok {
				r = 0.5
			}
			c.Price = setfmt.CardPrice{Base: round2(b[0] * math.Pow(b[1]/b[0], r) * tierMult)}
		}
		c.Price.Base = math.Max(c.Price.Base, 0.01)
	}

	if s.Progression {
		t := At(pos, s.MaxLevel)
		lic := t.License()
		scale := s.PackCostScale
		if scale <= 0 {
			scale = 1
		}
		cost := round2(t.PackCost * scale)
		for i := range p.Set.Packs {
			pk := &p.Set.Packs[i]
			pk.License = lic
			pk.PackCost = cost
			pk.BoxCost = nil // box follows the pack price ×8, like vanilla boxes
			pk.MarketMin, pk.MarketMax = t.MarketMin, t.MarketMax
		}
		pv.License, pv.PackCost, pv.Like = lic, cost, t.Name
	}
	p.Meta.Tier = tier
	p.Meta.Pricing = &project.Pricing{Mode: s.Mode, BorderCurve: s.BorderCurve, TierStep: s.TierStep, Position: pos}

	pv.AvgAfter = avgByRarity(p.Set.Cards)
	cards := append([]setfmt.Card(nil), p.Set.Cards...)
	sort.Slice(cards, func(i, j int) bool { return cards[i].Price.Base > cards[j].Price.Base })
	for i := 0; i < len(cards) && i < 5; i++ {
		pv.Top = append(pv.Top, TopCard{Name: cards[i].Name, Rarity: cards[i].Rarity, Before: before[cards[i].ID], After: cards[i].Price.Base})
	}
	return pv
}

func band(r string) [2]float64 {
	if b, ok := bands[r]; ok {
		return b
	}
	return bands["Common"]
}

func realUSD(p *project.Project, c setfmt.Card) float64 {
	m := p.Meta.Cards[c.ID]
	switch {
	case m.USD != nil:
		return *m.USD
	case m.EUR != nil:
		return *m.EUR * 1.08
	case m.USDFoil != nil:
		return *m.USDFoil / 2.5
	}
	return c.Price.Base // manual sets: use the current price as the rank signal
}

// jitter gives each card a stable ±15% spread so game-mode prices aren't all identical.
func jitter(id string) float64 {
	h := fnv.New32a()
	h.Write([]byte(id))
	return 0.85 + 0.30*float64(h.Sum32()%1000)/999
}

func avgByRarity(cards []setfmt.Card) map[string]float64 {
	sum, n := map[string]float64{}, map[string]int{}
	for _, c := range cards {
		sum[c.Rarity] += c.Price.Base
		n[c.Rarity]++
	}
	out := map[string]float64{}
	for r, s := range sum {
		out[r] = round2(s / float64(n[r]))
	}
	return out
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// ---------------------------------------------------------------- progression

// Position places set i (0-based) of n on the vanilla curve: t ∈ [0, 8] where 0 = Basic and 8 = Ascension.
// Up to 9 sets map one-to-one onto the vanilla products; more sets are spread evenly over the same range.
func Position(i, n int) float64 {
	last := float64(len(vanillaTiers) - 1)
	if n <= len(vanillaTiers) {
		return float64(i)
	}
	return float64(i) * last / float64(n-1)
}

// At interpolates the vanilla progression at position t. Unlock levels are stretched so that t = 8 unlocks at maxLevel
// (70 = vanilla Ascension; values ≤ 0 keep vanilla levels).
func At(t float64, maxLevel int) TierData {
	last := len(vanillaTiers) - 1
	t = math.Max(0, math.Min(float64(last), t))
	i := int(math.Floor(t))
	if i >= last {
		i = last - 1
	}
	f := t - float64(i)
	a, b := vanillaTiers[i], vanillaTiers[i+1]
	lerp := func(x, y float64) float64 { return x + (y-x)*f }
	scale := 1.0
	if maxLevel > 0 {
		scale = float64(maxLevel-1) / float64(vanillaTiers[last].PackLevel-1)
	}
	lvl := func(x, y int) int { return int(math.Round(1 + (lerp(float64(x), float64(y))-1)*scale)) }
	price := func(x, y float64) float64 { return math.Round(lerp(x, y)/10) * 10 }
	name := a.Name
	if f >= 0.5 {
		name = b.Name
	}
	if f > 0.01 && f < 0.99 {
		name = "≈ " + name
	}
	return TierData{
		Name:      name,
		PackLevel: lvl(a.PackLevel, b.PackLevel), PackBigLevel: lvl(a.PackBigLevel, b.PackBigLevel),
		BoxLevel: lvl(a.BoxLevel, b.BoxLevel), BoxBigLevel: lvl(a.BoxBigLevel, b.BoxBigLevel),
		PackPrice: price(a.PackPrice, b.PackPrice), PackBigPrice: price(a.PackBigPrice, b.PackBigPrice),
		BoxPrice: price(a.BoxPrice, b.BoxPrice), BoxBigPrice: price(a.BoxBigPrice, b.BoxBigPrice),
		PackCost: round2(lerp(a.PackCost, b.PackCost)), MarketMin: round2(lerp(a.MarketMin, b.MarketMin)), MarketMax: round2(lerp(a.MarketMax, b.MarketMax)),
	}
}

// License converts a progression step into the set.json license (all four restock rows).
func (t TierData) License() setfmt.License {
	pbl, pbp, bbl, bbp := t.PackBigLevel, t.PackBigPrice, t.BoxBigLevel, t.BoxBigPrice
	return setfmt.License{PackLevel: t.PackLevel, PackPrice: t.PackPrice, PackBigLevel: &pbl, PackBigPrice: &pbp,
		BoxLevel: t.BoxLevel, BoxPrice: t.BoxPrice, BoxBigLevel: &bbl, BoxBigPrice: &bbp}
}

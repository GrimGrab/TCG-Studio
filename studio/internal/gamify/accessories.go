package gamify

import (
	"math"
	"sort"

	"tcgstudio/internal/setfmt"
)

// Accessory progression: every accessory kind (deck boxes, playmats, sleeves, dice, comics, collection books) gets its own
// ladder, independent of set tiers: in library order within the kind, license levels are spread from MinLevel to MaxLevel and
// prices follow that kind's vanilla license curve. Deck boxes also get a big-delivery row 3 levels later at twice the price,
// like vanilla (every other kind only sells in one big delivery box).

// Ladder is one kind's ladder.
type Ladder struct {
	MinLevel   int     `json:"minLevel"`   // level of the first item
	MaxLevel   int     `json:"maxLevel"`   // level of the last item
	PriceScale float64 `json:"priceScale"` // multiplies the license price curve (1 = vanilla)
}

// AccessorySettings: one ladder per kind (setfmt.AccessoryKinds[].Kind).
type AccessorySettings map[string]Ladder

// Vanilla license rows (level, price) per kind, restock table game 1.02 (template export 2026-09-26). Deck boxes and dice are
// only sold in a narrow early range in vanilla, so their curves get one synthetic point to keep rising beyond it.
var vanillaCurves = map[string][][2]float64{
	"Deckbox": {{5, 100}, {5, 100}, {6, 100}, {6, 100}, {40, 1500}},
	"Playmat": {{7, 500}, {13, 1000}, {16, 1500}, {16, 2000}, {19, 1500}, {21, 2000}, {26, 2000}, {31, 3000}, {32, 3000},
		{33, 3000}, {34, 3000}, {35, 3000}, {36, 3000}, {38, 3000}, {43, 5000}, {47, 5000}, {52, 5000}, {55, 5000}, {60, 7500},
		{63, 10000}, {68, 12000}, {74, 15000}, {77, 15000}},
	"Sleeve": {{2, 50}, {10, 500}, {14, 800}, {20, 800}, {25, 800}, {30, 800}},
	"Dice":   {{3, 50}, {3, 50}, {4, 50}, {4, 50}, {20, 500}},
	"Comic": {{13, 900}, {21, 1500}, {29, 2100}, {36, 2700}, {41, 3500}, {46, 4500}, {53, 6000}, {56, 8000}, {59, 10000},
		{64, 12000}, {69, 14000}, {72, 16000}},
	"Binder": {{11, 1000}, {50, 10000}},
	// Battle decks (PreconDeck_*): the four elements, then the Destiny variants.
	"BattleDeck": {{9, 1000}, {14, 1000}, {17, 1000}, {22, 1000}, {33, 5000}, {39, 5000}, {45, 5000}, {50, 5000}},
	// Figurines (Toy_*, restock table game 1.02; ToonZ left out, Evo trees included).
	"Figurine": {{6, 500}, {8, 700}, {10, 900}, {12, 1200}, {15, 2500}, {18, 2500}, {20, 2500}, {24, 2500}, {28, 3500},
		{30, 5000}, {34, 5000}, {42, 5000}, {50, 5000}, {55, 10000}, {65, 10000}, {70, 15000}, {75, 15000}, {77, 15000},
		{80, 20000}},
}

// DefaultAccessorySettings spans each kind's vanilla level range.
func DefaultAccessorySettings() AccessorySettings {
	return AccessorySettings{
		"Deckbox":    {MinLevel: 5, MaxLevel: 40, PriceScale: 1},
		"Playmat":    {MinLevel: 7, MaxLevel: 77, PriceScale: 1},
		"Sleeve":     {MinLevel: 2, MaxLevel: 30, PriceScale: 1},
		"Dice":       {MinLevel: 3, MaxLevel: 20, PriceScale: 1},
		"Comic":      {MinLevel: 13, MaxLevel: 72, PriceScale: 1},
		"Binder":     {MinLevel: 11, MaxLevel: 50, PriceScale: 1},
		"BattleDeck": {MinLevel: 9, MaxLevel: 50, PriceScale: 1},
		"Figurine":   {MinLevel: 6, MaxLevel: 77, PriceScale: 1},
	}
}

// priceAt interpolates a kind's smoothed vanilla license price for a level (extrapolates linearly outside the curve).
func priceAt(kind string, level float64) float64 {
	pts := smoothCurve(vanillaCurves[kind])
	if len(pts) == 0 {
		return 100
	}
	if len(pts) == 1 {
		return pts[0][1]
	}
	if level <= pts[0][0] {
		return pts[0][1] * level / pts[0][0]
	}
	for i := 1; i < len(pts); i++ {
		if level <= pts[i][0] {
			a, b := pts[i-1], pts[i]
			t := (level - a[0]) / (b[0] - a[0])
			return a[1] + t*(b[1]-a[1])
		}
	}
	a, b := pts[len(pts)-2], pts[len(pts)-1]
	return math.Max(b[1], b[1]+(level-b[0])*(b[1]-a[1])/(b[0]-a[0]))
}

// playmatPriceAt is kept for the curve test.
func playmatPriceAt(level float64) float64 { return priceAt("Playmat", level) }

// smoothCurve averages vanilla rows that share a level and makes the price non-decreasing.
func smoothCurve(raw [][2]float64) [][2]float64 {
	byLevel := map[float64][]float64{}
	for _, p := range raw {
		byLevel[p[0]] = append(byLevel[p[0]], p[1])
	}
	var out [][2]float64
	for lvl, prices := range byLevel {
		sum := 0.0
		for _, p := range prices {
			sum += p
		}
		out = append(out, [2]float64{lvl, sum / float64(len(prices))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	for i := 1; i < len(out); i++ {
		out[i][1] = math.Max(out[i][1], out[i-1][1])
	}
	return out
}

// AccessoryPreview is one row of a ladder.
type AccessoryPreview struct {
	ID     string                  `json:"id"`
	Name   string                  `json:"name"`
	Kind   string                  `json:"kind"`
	Before setfmt.AccessoryLicense `json:"before"`
	After  setfmt.AccessoryLicense `json:"after"`
	HasBig bool                    `json:"hasBig"`
}

func roundTo(v, step float64) float64 { return math.Max(step, math.Round(v/step)*step) }

// ApplyAccessories sets every accessory's license from its place on its kind's ladder (in place, library order within the
// kind) and returns the preview rows in library order. Kinds missing from s use the defaults.
func ApplyAccessories(list []setfmt.Accessory, s AccessorySettings) []AccessoryPreview {
	def := DefaultAccessorySettings()
	byKind := map[string][]int{}
	for i, a := range list {
		byKind[a.Kind] = append(byKind[a.Kind], i)
	}
	out := make([]AccessoryPreview, len(list))
	for kind, idx := range byKind {
		l, ok := s[kind]
		if !ok {
			l = def[kind]
		}
		if l.MinLevel < 1 {
			l.MinLevel = 1
		}
		if l.MaxLevel < l.MinLevel {
			l.MaxLevel = l.MinLevel
		}
		if l.PriceScale <= 0 {
			l.PriceScale = 1
		}
		for pos, i := range idx {
			a := &list[i]
			t := 0.0
			if len(idx) > 1 {
				t = float64(pos) / float64(len(idx)-1)
			}
			level := int(math.Round(float64(l.MinLevel) + t*float64(l.MaxLevel-l.MinLevel)))
			price := priceAt(kind, float64(level)) * l.PriceScale
			step := 50.0
			if price < 500 {
				step = 10
			}
			lic := setfmt.AccessoryLicense{Level: level, Price: roundTo(price, step)}
			if a.HasSmallRow() {
				bl := level + 3
				bp := roundTo(lic.Price*2, step)
				lic.BigLevel, lic.BigPrice = &bl, &bp
			}
			out[i] = AccessoryPreview{ID: a.ID, Name: a.Name, Kind: a.Kind, Before: a.License, After: lic, HasBig: a.HasSmallRow()}
			a.License = lic
		}
	}
	return out
}

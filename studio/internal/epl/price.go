package epl

import (
	"math"
	"strings"
)

// Card prices: EPL mods have no card prices; EPL prices each card per expansion with one of two generators
// (prefabloader EnhancedPrefabLoader.Core/Cards/PriceGenerators, chosen by CardPriceStrategy: Default = 1, RarityDriven = 2;
// anything else is Default). Both multiply a random factor in; Studio uses its mean, so a converted card costs what it costs
// on average with EPL. rarityIndex is the card's rarity in the expansion's Rarities list (foil tiers have their own index).
//
//	Default:      (1 + p) · (1 + 4·p^1.6) · base^border · foil · U(lerp(0.2, 0.85, border/5), 1.15), p = index/(n−1) (0.5 if n ≤ 1)
//	RarityDriven: (floor + index·step) · base^border · foil · U(lerp(0.05, 0.7, index/20), 1.3)

// PriceModel is an expansion's EPL price generator.
type PriceModel struct {
	RarityDriven bool
	Rarities     []string
	BorderBase   float64
	FoilMult     float64
	Floor, Step  float64 // RarityDriven
}

// PriceModelOf reads an expansion's price settings.
func PriceModelOf(exp *CardExpansion) PriceModel {
	s := strings.ToLower(strings.TrimSpace(exp.CardPriceStrategy))
	m := PriceModel{Rarities: exp.Rarities, RarityDriven: s == "raritydriven" || s == "2"}
	if m.RarityDriven {
		m.Floor, m.Step, m.BorderBase, m.FoilMult = exp.RarityDrivenFloor, exp.RarityDrivenStepSize, exp.RarityDrivenBorderMultiplierBase, exp.RarityDrivenFoilMultiplier
	} else {
		m.BorderBase, m.FoilMult = exp.DefaultBorderMultiplierBase, exp.DefaultFoilMultiplier
	}
	// EPL's model defaults when a mod leaves them out (sharedresources CardExpansion).
	if m.BorderBase <= 0 {
		m.BorderBase = 1
	}
	if m.FoilMult <= 0 {
		m.FoilMult = 1
	}
	return m
}

func (m PriceModel) index(rarity string) int {
	for i, r := range m.Rarities {
		if r == rarity {
			return i
		}
	}
	for i, r := range m.Rarities {
		if strings.EqualFold(r, rarity) {
			return i
		}
	}
	return -1 // as C#'s IndexOf
}

func lerp(a, b, t float64) float64 { return a + (b-a)*math.Max(0, math.Min(1, t)) } // Unity's Mathf.Lerp clamps t

// randomMean is the mean of the generator's random factor.
func (m PriceModel) randomMean(idx, border int) float64 {
	if m.RarityDriven {
		return (lerp(0.05, 0.7, float64(idx)/20) + 1.3) / 2
	}
	return (lerp(0.2, 0.85, float64(border)/5) + 1.15) / 2
}

// rarityPart is the price before border, foil and random factors.
func (m PriceModel) rarityPart(idx int) float64 {
	if m.RarityDriven {
		return m.Floor + float64(idx)*m.Step
	}
	p := 0.5
	if n := len(m.Rarities); n > 1 {
		p = float64(idx) / float64(n-1)
	}
	if p < 0 {
		p = 0 // a rarity missing from the list: EPL's Pow of a negative is NaN; take the lowest
	}
	return (1 + p) * (1 + 4*math.Pow(p, 1.6))
}

// Price is the expected EPL market price of a card of this EPL rarity with border 0–5 (Base … FullArt).
func (m PriceModel) Price(rarity string, border int, foil bool) float64 {
	idx := m.index(rarity)
	v := m.rarityPart(idx) * math.Pow(m.BorderBase, float64(border)) * m.randomMean(idx, border)
	if foil {
		v *= m.FoilMult
	}
	return math.Max(0, v)
}

// BorderMultipliers is the price of each border (Base … FullArt) relative to Base, as our set's priceDefaults.
// (Default's random factor depends on the border, so its ratios include that; RarityDriven's are base^border.)
func (m PriceModel) BorderMultipliers() []float64 {
	out := make([]float64, 6)
	for b := range out {
		v := math.Pow(m.BorderBase, float64(b))
		if !m.RarityDriven {
			v *= m.randomMean(0, b) / m.randomMean(0, 0)
		}
		out[b] = math.Round(v*1000) / 1000
	}
	return out
}

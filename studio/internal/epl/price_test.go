package epl

import (
	"math"
	"testing"
)

// Expected prices by hand from EPL's generators (prefabloader Cards/PriceGenerators), random factor at its mean.
func TestPriceRarityDriven(t *testing.T) {
	// World of Warcraft TCG's settings.
	m := PriceModelOf(&CardExpansion{Rarities: []string{"Common", "Uncommon", "Rare"}, CardPriceStrategy: "RarityDriven",
		RarityDrivenFloor: 0.5, RarityDrivenStepSize: 8, RarityDrivenBorderMultiplierBase: 1.15, RarityDrivenFoilMultiplier: 1.4})
	if !m.RarityDriven {
		t.Fatal("strategy not read")
	}
	// Common: 0.5 × mean U(0.05, 1.3) = 0.5 × 0.675
	if got := m.Price("Common", 0, false); math.Abs(got-0.3375) > 1e-9 {
		t.Fatalf("Common: %v", got)
	}
	// Rare: (0.5 + 2×8) × mean U(lerp(0.05, 0.7, 2/20) = 0.115, 1.3) = 16.5 × 0.7075; foil ×1.4; Gold ×1.15³
	want := 16.5 * 0.7075
	if got := m.Price("Rare", 0, false); math.Abs(got-want) > 1e-9 {
		t.Fatalf("Rare: %v want %v", got, want)
	}
	if got := m.Price("Rare", 3, true); math.Abs(got-want*1.4*math.Pow(1.15, 3)) > 1e-9 {
		t.Fatalf("Rare Gold foil: %v", got)
	}
	if b := m.BorderMultipliers(); b[0] != 1 || b[1] != 1.15 || b[5] != math.Round(math.Pow(1.15, 5)*1000)/1000 {
		t.Fatalf("border multipliers %v", b)
	}
	// Numbers in text fields (EPL's Newtonsoft parsing): 2 = RarityDriven.
	if !PriceModelOf(&CardExpansion{CardPriceStrategy: "2"}).RarityDriven {
		t.Fatal("strategy 2 not RarityDriven")
	}
}

func TestPriceDefault(t *testing.T) {
	r := make([]string, 13) // DBS Capsule Corp: 13 rarities, base 4.8, foil 4.2
	for i := range r {
		r[i] = string(rune('A' + i))
	}
	m := PriceModelOf(&CardExpansion{Rarities: r, CardPriceStrategy: "Default", DefaultBorderMultiplierBase: 4.8, DefaultFoilMultiplier: 4.2})
	mean0 := (0.2 + 1.15) / 2
	if got := m.Price("A", 0, false); math.Abs(got-mean0) > 1e-9 { // p = 0: 1 × 1
		t.Fatalf("lowest: %v", got)
	}
	if got := m.Price("M", 0, true); math.Abs(got-2*5*mean0*4.2) > 1e-9 { // p = 1: (1+1)(1+4), foil
		t.Fatalf("highest foil: %v", got)
	}
	// Silver (2): 4.8² × mean U(lerp(0.2, 0.85, 0.4) = 0.46, 1.15)
	mean2 := (0.46 + 1.15) / 2
	if got := m.Price("A", 2, false); math.Abs(got-4.8*4.8*mean2) > 1e-9 {
		t.Fatalf("Silver: %v", got)
	}
	if b := m.BorderMultipliers(); math.Abs(b[2]-math.Round(4.8*4.8*mean2/mean0*1000)/1000) > 1e-9 {
		t.Fatalf("border multipliers %v", b)
	}
	// Missing strategy = Default (EPL's factory default).
	if PriceModelOf(&CardExpansion{}).RarityDriven {
		t.Fatal("empty strategy should be Default")
	}
}

// Kept tiers keep exact odds: an identity tier map gives per-tier weights, and rare tiers aren't rounded away.
func TestSlotsPerTier(t *testing.T) {
	pp := PackPlan{slots: []map[string]float64{{"Low": 0.9999962, "Rare": 0.0000038}}}
	got := pp.Slots(map[string]string{"Low": "low", "Rare": "rare"})
	if len(got) != 1 || got[0].Weights["rare"] != 0.00038 || got[0].Weights["low"] != 100 {
		t.Fatalf("%+v", got)
	}
}

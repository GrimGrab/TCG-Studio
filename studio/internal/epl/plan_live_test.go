package epl

import (
	"os"
	"path/filepath"
	"testing"
)

// EPL_JSON="<descriptor.json>[;<more>]" go test ./internal/epl -run PlanLive -v
// Prints how each card expansion of real descriptors converts: tiers, default rarities, slots and foil chance.
func TestPlanLive(t *testing.T) {
	list := os.Getenv("EPL_JSON")
	if list == "" {
		t.Skip("EPL_JSON not set")
	}
	for _, path := range filepath.SplitList(list) {
		d, err := ReadDescriptor(path)
		if err != nil {
			t.Fatal(err)
		}
		for i := range d.CardExpansions {
			sp := PlanSet(d, &d.CardExpansions[i])
			t.Logf("== %s / %s: %d cards, %d tiers, %d packs, back %q, warnings %v", filepath.Base(path), sp.Exp.Name,
				len(sp.Cards), len(sp.Tiers), len(sp.Packs), sp.CardBack, sp.Warnings)
			pm := PriceModelOf(sp.Exp)
			ids := map[string]string{} // tiers kept as rarities (identity map)
			for _, tr := range sp.Tiers {
				ids[tr.Name] = tr.Name
				t.Logf("   %-45s %4d cards  %.4f/pack  %.6f/card  -> %-9s $%.2f (foil $%.2f)", tr.Name, tr.Cards, tr.PerPack, tr.PerCard, tr.Rarity,
					pm.Price(tr.Name, 0, false), pm.Price(tr.Name, 0, true))
			}
			t.Logf("   prices: rarityDriven=%v border×%v foil×%.2f", pm.RarityDriven, pm.BorderMultipliers(), pm.FoilMult)
			for _, pp := range sp.Packs {
				t.Logf("   pack %q per tier: %v", pp.Item.Name, pp.Slots(ids))
			}
			for _, pp := range sp.Packs {
				total := 0
				slots := pp.Slots(sp.DefaultRarities())
				for _, s := range slots {
					total += s.Count
				}
				box := ""
				if pp.Box != nil {
					box = pp.Box.Name
				}
				t.Logf("   pack %q (%s) box %q foil %.1f%% slots %v (sum %d)", pp.Item.Name, pp.Strategy, box, pp.FoilChance, slots, total)
				if total != cardsPerPack {
					t.Errorf("slots sum %d", total)
				}
			}
			if len(sp.Cards) > 0 {
				t.Logf("   first card %+v, last %+v", sp.Cards[0], sp.Cards[len(sp.Cards)-1])
			}
		}
	}
}

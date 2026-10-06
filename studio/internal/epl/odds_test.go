package epl

import (
	"math"
	"testing"
)

func desc(t *testing.T, s string) *Descriptor {
	t.Helper()
	d, err := ParseDescriptor([]byte("\xef\xbb\xbf" + s))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

const cardsJSON = `"Cards":[
 {"Name":"Ann","Rarity":"Low","Sprite":"s1","ElementType":"Water","CardBack":"back"},
 {"Name":"Ann","Rarity":"Low Foil","Sprite":"s1","IsFoil":true},
 {"Name":"Bob","Rarity":"Low","Sprite":"s2","ElementType":"Destiny"},
 {"Name":"Ann","Rarity":"High","Sprite":"s3"},
 {"Name":"Cy","Rarity":"High Foil","Sprite":"s4"}]`

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func sum(sp *SetPlan) int {
	n := 0
	for _, s := range sp.Packs[0].Slots(sp.DefaultRarities()) {
		n += s.Count
	}
	return n
}

// Foil entries fold into their base tier and card (same sprite); names in several tiers get a tier tag; slots map and
// merge; foil chance = expected foil cards / 7.
func TestPlanGuaranteed(t *testing.T) {
	d := desc(t, `{"CardExpansions":[{"Name":"X","CardExpansion":"X ET","Rarities":["Low","Low Foil","High","High Foil"],`+cardsJSON+`}],
	 "Items":[{"Name":"P","ItemType":"P IT","IsCardPack":true,"CardExpansion":"X ET","PackGenerationStrategy":"Guaranteed",
	   "SlotWeights":{"Slot2":{"High":1},"Slot1":{"Low":3,"High":1},"Slot3":{"Low Foil":1,"High Foil":1}}},
	  {"Name":"B","IsCardBox":true,"SpawnsPackType":"P IT"}]}`)
	sp := PlanSet(d, &d.CardExpansions[0])
	if len(sp.Cards) != 4 || sp.Cards[0].Tier != "Low" || sp.Cards[3].Tier != "High" || sp.Cards[3].Name != "Cy" {
		t.Fatalf("cards %+v", sp.Cards)
	}
	if sp.Cards[0].Variant != "Low" || sp.Cards[2].Variant != "High" || sp.Cards[1].Variant != "" {
		t.Errorf("variants %+v", sp.Cards)
	}
	if sp.Cards[0].Element != "Water" || sp.Cards[1].Element != "Fire" || sp.CardBackAlt != "back" {
		t.Errorf("element/back %+v %q", sp.Cards[:2], sp.CardBackAlt)
	}
	pp := sp.Packs[0]
	if pp.Box == nil || pp.Box.Name != "B" || len(pp.slots) != 7 {
		t.Fatalf("pack %+v", pp)
	}
	// slot 1: Low .75 High .25; slot 2: High 1; slot 3: Low .5 High .5 (foil); slots 4–7 by entry counts.
	if !approx(pp.slots[0]["Low"], 0.75) || !approx(pp.slots[1]["High"], 1) || !approx(pp.slots[2]["Low"], 0.5) {
		t.Errorf("slots %v", pp.slots[:3])
	}
	// foil: slot 3 = 1, slots 4–7: 2 foil entries of 5 each = 0.4 → 1 + 1.6 = 2.6 of 7.
	if !approx(pp.FoilChance, 100*2.6/7) {
		t.Errorf("foil %v", pp.FoilChance)
	}
	if sum(sp) != 7 {
		t.Errorf("slots sum %d", sum(sp))
	}
}

func TestPlanStrategies(t *testing.T) {
	cases := map[string]string{
		"AllRandomWeighted": `"PackGenerationStrategy":"AllRandomWeighted","NormalWeights":{"Low":9,"High":1}`,
		"legacy":            `"PackGenerationStrategy":"AllRandomWeighted","GenerationWeights":[{"Rarity":"Low","NormalWeight":9},{"Rarity":"High","NormalWeight":1}]`,
		"AllRandom":         `"PackGenerationStrategy":"AllRandom"`,
		"unknown":           `"PackGenerationStrategy":"Mystery"`,
	}
	for name, item := range cases {
		d := desc(t, `{"CardExpansions":[{"Name":"X","CardExpansion":"X ET","Rarities":["Low","High"],`+cardsJSON+`}],
		 "Items":[{"Name":"P","IsCardPack":true,"CardExpansion":"X ET",`+item+`}]}`)
		sp := PlanSet(d, &d.CardExpansions[0])
		s := sp.Packs[0].slots[0]
		switch name {
		case "AllRandomWeighted", "legacy":
			if !approx(s["Low"], 0.9) || !approx(s["High"], 0.1) {
				t.Errorf("%s: %v", name, s)
			}
		default: // by entry counts: Low 2 + Low Foil 1, High 1 + High Foil 1
			if !approx(s["Low"], 0.6) || !approx(s["High"], 0.4) {
				t.Errorf("%s: %v", name, s)
			}
		}
		if sum(sp) != 7 {
			t.Errorf("%s: slots sum %d", name, sum(sp))
		}
		if name == "unknown" && len(sp.Warnings) == 0 {
			t.Errorf("unknown strategy should warn")
		}
	}
}

func TestPlanRandomFoilsAndNoPacks(t *testing.T) {
	d := desc(t, `{"CardExpansions":[{"Name":"X","CardExpansion":"X ET","HasRandomFoils":true,"FoilChance":0.08,
	 "Rarities":["Low","High"],`+cardsJSON+`}],"Items":[{"Name":"P","IsCardPack":true,"CardExpansion":"X ET","PackGenerationStrategy":"AllRandom"}]}`)
	if sp := PlanSet(d, &d.CardExpansions[0]); !approx(sp.Packs[0].FoilChance, 8) {
		t.Errorf("random foils %v", sp.Packs[0].FoilChance)
	}
	d = desc(t, `{"CardExpansions":[{"Name":"X","CardExpansion":"X ET","Rarities":["Low","High"],`+cardsJSON+`}]}`)
	sp := PlanSet(d, &d.CardExpansions[0])
	if len(sp.Packs) != 0 || len(sp.Warnings) == 0 {
		t.Fatalf("no packs: %+v", sp)
	}
	if r := sp.DefaultRarities(); r["Low"] != "Common" || r["High"] != "Legendary" {
		t.Errorf("order-based rarities %v", r)
	}
}

func TestDefaultRarityBands(t *testing.T) {
	sp := &SetPlan{Tiers: []Tier{
		{Name: "A", Cards: 50, PerCard: 0.01}, {Name: "Award", Cards: 1, PerCard: 0.1}, {Name: "B", Cards: 25, PerCard: 0.002},
		{Name: "C", Cards: 15, PerCard: 0.0005}, {Name: "D", Cards: 9, PerCard: 0.0001}}}
	for i := range sp.Tiers {
		sp.Tiers[i].PerPack = sp.Tiers[i].PerCard * float64(sp.Tiers[i].Cards)
	}
	sp.setDefaultRarities()
	want := map[string]string{"Award": "Common", "A": "Common", "B": "Rare", "C": "Epic", "D": "Legendary"}
	for k, v := range want {
		if sp.DefaultRarities()[k] != v {
			t.Errorf("%s → %s, want %s", k, sp.DefaultRarities()[k], v)
		}
	}
}

// Some mods write enum fields as numbers (EPL reads them through Newtonsoft, which makes them text).
func TestNumbersInTextFields(t *testing.T) {
	d, err := ParseDescriptor([]byte(`{"Items":[{"Name":"P","IsCardPack":true,"PackGenerationStrategy":2,"ItemType":7,
		"GenerationWeights":[{"Rarity":3,"GuaranteedSlots":1,"NormalWeight":0.5}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	it := d.Items[0]
	if it.PackGenerationStrategy != "AllRandomWeighted" || it.ItemType != "7" || it.GenerationWeights[0].Rarity != "3" ||
		it.GenerationWeights[0].GuaranteedSlots != "1" || it.GenerationWeights[0].NormalWeight != 0.5 {
		t.Fatalf("%+v", it)
	}
}

// Many low-odds alt-art cards must not drag rare tiers into Common (cards of one game rarity come out equally often).
func TestDefaultRaritiesFollowOdds(t *testing.T) {
	tier := func(name string, cards int, perCard float64) Tier {
		return Tier{Name: name, Cards: cards, PerCard: perCard, PerPack: perCard * float64(cards)}
	}
	sp := &SetPlan{Tiers: []Tier{tier("Common", 22, 0.22), tier("Uncommon", 15, 0.13), tier("Rare", 10, 0.009),
		tier("Super Rare", 10, 0.0045), tier("Common Alt-Art", 16, 0.00027), tier("Rare Alt-Art", 26, 0.00017),
		tier("Energy Alt-Art", 21, 0.00005), tier("Unpulled", 3, 0)}}
	sp.setDefaultRarities()
	want := map[string]string{"Common": "Common", "Uncommon": "Common", "Rare": "Rare", "Super Rare": "Rare",
		"Common Alt-Art": "Epic", "Rare Alt-Art": "Epic", "Energy Alt-Art": "Legendary", "Unpulled": "Legendary"}
	for k, v := range want {
		if got := sp.DefaultRarities()[k]; got != v {
			t.Errorf("%s → %s, want %s", k, got, v)
		}
	}
}

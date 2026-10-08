package importer

import (
	"encoding/json"
	"os"
	"testing"

	"tcgstudio/internal/setfmt"
)

func TestPokemonSetID(t *testing.T) {
	for in, want := range map[[2]string]string{
		{"sv03.5", "en"}: "ptcg-sv03-5",
		{"base1", ""}:    "ptcg-base1",
		{"SV01", "fr"}:   "ptcg-sv01-fr",
	} {
		got := PokemonSetID(in[0], in[1])
		if got != want || !setfmt.SafeID(got) {
			t.Errorf("PokemonSetID(%q,%q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

// Every rarity TCGdex knows (testdata = /v2/en/rarities) maps to a game rarity with an icon (not SuperLegend).
func TestPokemonRarityOrderMatchesMap(t *testing.T) {
	m := DefaultPokemonRarityMap()
	in := map[string]bool{}
	for _, r := range PokemonRarityOrder {
		if in[r] {
			t.Errorf("%q listed twice", r)
		}
		in[r] = true
		if _, ok := m[r]; !ok {
			t.Errorf("%q is in PokemonRarityOrder but not in the default map", r)
		}
	}
	for r := range m {
		if !in[r] {
			t.Errorf("%q is in the default map but not in PokemonRarityOrder", r)
		}
	}
}

func TestPokemonRarityMapCoversTCGdex(t *testing.T) {
	b, err := os.ReadFile("testdata/tcgdex_rarities.json")
	if os.IsNotExist(err) {
		t.Skip("testdata is local only (not in the public source)")
	}
	if err != nil {
		t.Fatal(err)
	}
	var rarities []string
	if err := json.Unmarshal(b, &rarities); err != nil {
		t.Fatal(err)
	}
	m := DefaultPokemonRarityMap()
	pocket := map[string]bool{"One Diamond": true, "Two Diamond": true, "Three Diamond": true, "Four Diamond": true,
		"One Star": true, "Two Star": true, "Three Star": true, "One Shiny": true, "Two Shiny": true} // TCG Pocket, not listed
	for _, r := range rarities {
		g := PokemonRarity(m, r)
		if g == "" || g == "SuperLegend" {
			t.Errorf("%q → %q", r, g)
		}
		if _, listed := m[r]; !listed && !pocket[r] {
			t.Errorf("%q is not in the default map (falls back to %q)", r, g)
		}
	}
	for r, want := range map[string]string{"Common": "Common", "Uncommon": "Rare", "Rare Holo": "Epic", "Double rare": "Legendary",
		"Special illustration rare": "Legendary", "Some New Rare": "Legendary", "": "Common"} {
		if got := PokemonRarity(m, r); got != want {
			t.Errorf("PokemonRarity(%q) = %q, want %q", r, got, want)
		}
	}
}

func TestFitSlots(t *testing.T) {
	slots := []setfmt.Slot{
		{Count: 5, Weights: map[string]float64{"Common": 1}},
		{Count: 1, Weights: map[string]float64{"Rare": 1}},
		{Count: 1, Weights: map[string]float64{"Epic": 5, "Legendary": 2}},
	}
	cards := []setfmt.Card{{Rarity: "Common"}, {Rarity: "Epic"}, {Rarity: "Legendary"}} // no Rare (modern Yu-Gi-Oh!)
	got := fitSlots(slots, &setfmt.Set{Cards: cards})
	if len(got) != 3 || got[1].Weights["Common"] != 1 || len(got[1].Weights) != 1 || got[2].Weights["Legendary"] != 2 {
		t.Fatalf("no-Rare set: %+v", got)
	}
	got = fitSlots(slots, &setfmt.Set{Cards: []setfmt.Card{{Rarity: "Common"}, {Rarity: "Rare"}, {Rarity: "Epic"}}}) // no Legendary
	if w := got[2].Weights; len(w) != 1 || w["Epic"] != 5 {
		t.Fatalf("no-Legendary set: %+v", got)
	}
	got = fitSlots(slots, &setfmt.Set{Cards: []setfmt.Card{{Rarity: "Legendary"}}}) // promo set: everything Legendary
	for _, s := range got {
		if len(s.Weights) != 1 || s.Weights["Legendary"] == 0 {
			t.Fatalf("all-Legendary set: %+v", got)
		}
	}
}

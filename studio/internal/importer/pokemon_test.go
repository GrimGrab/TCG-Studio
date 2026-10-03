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

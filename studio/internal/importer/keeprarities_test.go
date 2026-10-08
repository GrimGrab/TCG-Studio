package importer

import (
	"math"
	"testing"

	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

// A Pokémon-like set: source rarities become the set's list in the source's order; a hand-made card keeps its game
// rarity's name; slots keep the odds (each game rarity's weight shared by the source rarities its cards had).
func TestKeepSourceRarities(t *testing.T) {
	set := setfmt.NewSet("x", "X")
	meta := &project.Meta{Cards: map[string]project.CardMeta{}}
	add := func(id, game, src string) {
		set.Cards = append(set.Cards, setfmt.Card{ID: id, Rarity: game, Play: setfmt.DefaultPlay()})
		meta.Cards[id] = project.CardMeta{SrcRarity: src}
	}
	add("1", "Epic", "Rare Holo")
	add("2", "Common", "Common")
	add("3", "Epic", "Rare")
	add("4", "Rare", "Uncommon")
	add("5", "Epic", "Rare")
	add("6", "Legendary", "")        // added by hand
	add("7", "Legendary", "Mystery") // not in the source's order
	pk := setfmt.NewPack("b", "B")
	pk.Slots = []setfmt.Slot{{Count: 6, Weights: map[string]float64{"Common": 1}}, {Count: 1, Weights: map[string]float64{"Epic": 6, "Legendary": 1}}}
	set.Packs = []setfmt.Pack{pk}
	p := &project.Project{Set: set, Meta: meta}

	if err := KeepSourceRarities(p, PokemonRarityOrder); err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range set.Rarities {
		ids = append(ids, r.ID)
	}
	want := []string{"common", "uncommon", "rare", "rare-holo", "legendary", "mystery"}
	if len(ids) != len(want) {
		t.Fatalf("rarities %v", ids)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("rarities %v, want %v", ids, want)
		}
	}
	if set.Cards[0].Rarity != "rare-holo" || set.Cards[5].Rarity != "legendary" || set.Rarities[1].Name != "Uncommon" {
		t.Fatalf("cards %+v", set.Cards)
	}
	w := set.Packs[0].Slots[1].Weights // Epic 6 over rare (2 cards) + rare-holo (1); Legendary 1 over Legendary + mystery
	if math.Abs(w["rare"]-4) > 1e-9 || math.Abs(w["rare-holo"]-2) > 1e-9 || math.Abs(w["legendary"]-0.5) > 1e-9 {
		t.Fatalf("slot %v", w)
	}
	for _, i := range set.Validate() {
		if i.Level == "error" {
			t.Fatalf("invalid after conversion: %+v", i)
		}
	}
	if err := KeepSourceRarities(p, nil); err == nil {
		t.Fatal("second conversion should fail")
	}
}

// Image folder: images at the top (no subfolder) are Common and must end up lowest, before the subfolders' rarities.
func TestKeepSourceRaritiesTopLevelCommon(t *testing.T) {
	set := setfmt.NewSet("x", "X")
	meta := &project.Meta{Cards: map[string]project.CardMeta{}, RarityOrder: []string{"Holo", "Secret"}}
	for i, c := range []struct{ game, src string }{{"Epic", "Holo"}, {"Common", ""}, {"Legendary", "Secret"}} {
		id := string(rune('a' + i))
		set.Cards = append(set.Cards, setfmt.Card{ID: id, Rarity: c.game, Play: setfmt.DefaultPlay()})
		meta.Cards[id] = project.CardMeta{SrcRarity: c.src}
	}
	if err := KeepSourceRarities(&project.Project{Set: set, Meta: meta}, nil); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range set.Rarities {
		got = append(got, r.Name)
	}
	if len(got) != 3 || got[0] != "Common" || got[1] != "Holo" || got[2] != "Secret" {
		t.Fatalf("order %v", got)
	}
}

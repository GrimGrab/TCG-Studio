package setfmt

import (
	"math"
	"testing"
)

func TestRarityID(t *testing.T) {
	for _, c := range []struct{ name, want string }{
		{"Common", "common"},
		{"Secret Rare Alt-Art", "secret-rare-alt-art"},
		{"SR★★", "sr"},
		{"★", "rarity"},
	} {
		if got := RarityID(c.name); got != c.want {
			t.Errorf("RarityID(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestOwnRaritiesValidate(t *testing.T) {
	s := NewSet("x", "X")
	s.Rarities = []Rarity{{ID: "common", Name: "Common"}, {ID: "super-rare", Name: "Super Rare"}}
	s.Cards = []Card{{ID: "a", Rarity: "common", Play: DefaultPlay()}, {ID: "b", Rarity: "super-rare", Play: DefaultPlay()}}
	p := NewPack("p", "P")
	p.Slots = []Slot{{Count: 7, Weights: map[string]float64{"common": 9, "super-rare": 1}}}
	s.Packs = []Pack{p}
	for _, i := range s.Validate() {
		if i.Level == "error" {
			t.Fatalf("unexpected error %+v", i)
		}
	}
	s.Cards[1].Rarity = "Epic" // not in the set's list
	s.Packs[0].Slots[0].Weights["Legendary"] = 1
	n := 0
	for _, i := range s.Validate() {
		if i.Level == "error" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("want 2 errors (card + slot), got %d", n)
	}
	if s.TierOf("super-rare") != "Rare" || s.TierOf("common") != "Common" || s.RankOf("super-rare") != 1 {
		t.Fatal("TierOf/RankOf")
	}
}

// Vanilla-keyed weights are shared over the rarities group puts in them, by card count.
func TestSplitWeights(t *testing.T) {
	s := NewSet("x", "X")
	s.Rarities = []Rarity{{ID: "c"}, {ID: "u"}, {ID: "sr"}, {ID: "ur"}}
	s.Cards = []Card{{Rarity: "c"}, {Rarity: "u"}, {Rarity: "sr"}, {Rarity: "sr"}, {Rarity: "sr"}, {Rarity: "ur"}}
	group := map[string]string{"c": "Common", "u": "Rare", "sr": "Epic", "ur": "Epic"}
	w := s.SplitWeights(map[string]float64{"Epic": 4, "Legendary": 1, "u": 2}, func(id string) string { return group[id] })
	if math.Abs(w["sr"]-3) > 1e-9 || math.Abs(w["ur"]-1) > 1e-9 || w["u"] != 2 || len(w) != 3 {
		t.Fatalf("%v", w)
	}
}

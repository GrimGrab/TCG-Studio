package gamify

import (
	"testing"

	"tcgstudio/internal/setfmt"
)

func TestApplyAccessoriesLadder(t *testing.T) {
	list := []setfmt.Accessory{
		setfmt.NewAccessory("Playmat", "p1", "P1", ""),
		setfmt.NewAccessory("Deckbox", "d1", "D1", ""),
		setfmt.NewAccessory("Playmat", "p2", "P2", ""),
		setfmt.NewAccessory("Deckbox", "d2", "D2", ""),
		setfmt.NewAccessory("Playmat", "p3", "P3", ""),
	}
	s := AccessorySettings{"Deckbox": {MinLevel: 5, MaxLevel: 20, PriceScale: 1}, "Playmat": {MinLevel: 7, MaxLevel: 77, PriceScale: 1}}
	rows := ApplyAccessories(list, s)
	if len(rows) != 5 || rows[1].ID != "d1" {
		t.Fatalf("rows not in library order: %+v", rows)
	}
	// Each kind has its own ladder: playmats 7 → 42 → 77, deck boxes 5 → 20, regardless of interleaving.
	if list[0].License.Level != 7 || list[2].License.Level != 42 || list[4].License.Level != 77 {
		t.Fatalf("playmat levels = %d %d %d", list[0].License.Level, list[2].License.Level, list[4].License.Level)
	}
	if list[1].License.Level != 5 || list[3].License.Level != 20 {
		t.Fatalf("deck box levels = %d %d", list[1].License.Level, list[3].License.Level)
	}
	// Deck box: two rows, big 3 levels later at twice the price; the first one costs about vanilla's 100.
	d := list[1].License
	if d.BigLevel == nil || *d.BigLevel != 8 || *d.BigPrice != d.Price*2 || d.Price < 80 || d.Price > 120 {
		t.Fatalf("deck box license = %+v", d)
	}
	// Playmats: single row, price rising with level, first one ~vanilla 500.
	if list[0].License.BigLevel != nil || list[0].License.Price != 500 || list[4].License.Price <= list[2].License.Price {
		t.Fatalf("playmats = %+v / %+v / %+v", list[0].License, list[2].License, list[4].License)
	}
}

func TestPlaymatCurveMonotonic(t *testing.T) {
	prev := 0.0
	for lvl := 1.0; lvl <= 90; lvl++ {
		p := playmatPriceAt(lvl)
		if p < prev {
			t.Fatalf("price drops at level %v: %v < %v", lvl, p, prev)
		}
		prev = p
	}
}

func TestEveryKindHasALadder(t *testing.T) {
	def := DefaultAccessorySettings()
	for _, k := range setfmt.AccessoryKinds {
		l, ok := def[k.Kind]
		if !ok || l.MinLevel < 1 || l.MaxLevel <= l.MinLevel {
			t.Fatalf("%s: default ladder %+v", k.Kind, l)
		}
		if priceAt(k.Kind, float64(l.MinLevel)) <= 0 || priceAt(k.Kind, float64(l.MaxLevel)) < priceAt(k.Kind, float64(l.MinLevel)) {
			t.Fatalf("%s: price curve not rising", k.Kind)
		}
	}
}

package importer

import (
	"fmt"
	"strings"
	"testing"

	"tcgstudio/internal/setfmt"
)

func TestMtgBooster(t *testing.T) {
	cases := []struct {
		code, date string
		want       string
		cards      int
	}{
		{"arn", "1993-12-17", MtgEight.Name, 8},
		{"chr", "1995-07-01", MtgTwelve.Name, 12},
		{"lea", "1993-08-05", MtgClassic.Name, 15},
		{"usg", "1998-10-12", MtgClassic.Name, 15},
		{"ala", "2008-10-03", MtgDraft.Name, 15},
		{"dom", "2018-04-27", MtgDraft.Name, 15},
		{"mkm", "2024-02-09", MtgPlay.Name, 14},
		{"dsk", "2024-09-27", MtgPlay.Name, 14},
	}
	for _, c := range cases {
		b := MtgBooster(c.code, c.date)
		if b.Name != c.want || setfmt.SlotTotal(b.Slots) != c.cards {
			t.Errorf("%s %s: got %q (%d cards), want %q (%d)", c.code, c.date, b.Name, setfmt.SlotTotal(b.Slots), c.want, c.cards)
		}
	}
	if MtgBooster("ulg", "1999-02-15").FoilChance == 0 || MtgBooster("usg", "1998-10-12").FoilChance != 0 {
		t.Error("foils start with Urza's Legacy")
	}
}

func TestEraBoosters(t *testing.T) {
	for _, c := range []struct {
		got   Booster
		want  string
		cards int
	}{
		{PokemonBooster("1999-01-09"), PkmClassic.Name, 11},
		{PokemonBooster("2005-02-14"), PkmEX.Name, 9},
		{PokemonBooster("2020-02-07"), PkmModern.Name, 10},
		{PokemonBooster("2023-03-31"), PkmSV.Name, 10},
		{YgoBooster("2002-03-08"), YgoClassic.Name, 9},
		{YgoBooster("2016-01-15"), YgoModern.Name, 9},
		{UnionArenaBooster("2024-03-29"), UnionArena8.Name, 8},
		{UnionArenaBooster("2025-07-25"), UnionArena12.Name, 12},
	} {
		if c.got.Name != c.want || setfmt.SlotTotal(c.got.Slots) != c.cards {
			t.Errorf("got %q (%d cards), want %q (%d)", c.got.Name, setfmt.SlotTotal(c.got.Slots), c.want, c.cards)
		}
	}
}

// Every preset fits the game (1–24 cards) and says its card count in its name.
func TestPackPresets(t *testing.T) {
	seen := map[string]bool{}
	for _, b := range PackPresets() {
		n := setfmt.SlotTotal(b.Slots)
		if n < 1 || n > setfmt.MaxCardsPerPack {
			t.Errorf("%s: %d cards", b.Name, n)
		}
		if !strings.Contains(b.Name, fmt.Sprintf("%d cards", n)) {
			t.Errorf("%s: name doesn't say %d cards", b.Name, n)
		}
		if seen[b.Name] {
			t.Errorf("duplicate preset %s", b.Name)
		}
		seen[b.Name] = true
	}
}

// Slot order is reveal order: every booster ends on its best guaranteed slot (the one whose lowest rarity is highest).
func TestPresetsEndOnBestSlot(t *testing.T) {
	rank := map[string]int{"Common": 0, "Rare": 1, "Epic": 2, "Legendary": 3}
	floor := func(s setfmt.Slot) int {
		m := 9
		for r := range s.Weights {
			if rank[r] < m {
				m = rank[r]
			}
		}
		return m
	}
	for _, b := range PackPresets() {
		last := floor(b.Slots[len(b.Slots)-1])
		for i, s := range b.Slots[:len(b.Slots)-1] {
			if floor(s) > last {
				t.Errorf("%s: slot %d is better than the last slot", b.Name, i+1)
			}
		}
	}
}

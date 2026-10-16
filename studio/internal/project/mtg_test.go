package project

import (
	"testing"

	"tcgstudio/internal/setfmt"
)

func TestForgeName(t *testing.T) {
	cases := []struct {
		display string
		meta    CardMeta
		want    string
	}{
		{"Karn, Scion of Urza", CardMeta{TypeLine: "Legendary Planeswalker — Karn"}, "Karn, Scion of Urza"},
		{"Karn, Scion of Urza (Borderless)", CardMeta{Variant: []string{"Borderless"}}, "Karn, Scion of Urza"},
		{"Azog, Moria's Ruin (Borderless, Showcase)", CardMeta{}, "Azog, Moria's Ruin"},        // tags not in meta
		{"Erase (Not the Urza's Legacy One)", CardMeta{}, "Erase (Not the Urza's Legacy One)"}, // real name
		{"Human—Time Lord Meta-Crisis", CardMeta{}, "Human-Time Lord Meta-Crisis"},
		{"x", CardMeta{Name: "Fire // Ice", Layout: "split"}, "Fire // Ice"},
		{"x", CardMeta{Name: "Delver of Secrets // Insectile Aberration", Layout: "transform"}, "Delver of Secrets"},
		// No layout recorded (older imports): fall back on the type line.
		{"Fire // Ice", CardMeta{TypeLine: "Instant // Instant"}, "Fire // Ice"},
		{"Cut // Ribbons", CardMeta{TypeLine: "Sorcery // Sorcery"}, "Cut // Ribbons"},
		{"Bonecrusher Giant // Stomp", CardMeta{TypeLine: "Creature — Giant // Instant — Adventure"}, "Bonecrusher Giant"},
		{"Delver of Secrets // Insectile Aberration (Showcase)", CardMeta{Variant: []string{"Showcase"},
			TypeLine: "Creature — Human Wizard // Creature — Human Insect"}, "Delver of Secrets"},
	}
	for _, c := range cases {
		if got := ForgeName(c.display, c.meta); got != c.want {
			t.Errorf("ForgeName(%q) = %q, want %q", c.display, got, c.want)
		}
	}
}

func TestFillMtgFilterFields(t *testing.T) {
	p := &Project{
		Set: &setfmt.Set{Cards: []setfmt.Card{{ID: "1", Name: "Grizzly Bears"}}},
		Meta: &Meta{Source: "scryfall", ScryfallCode: "dom", Cards: map[string]CardMeta{"1": {TypeLine: "Creature — Bear", ManaCost: "{1}{G}",
			Colors: []string{"G"}, SrcRarity: "common", CMC: 2, Power: "2", Toughness: "2"}}},
	}
	if !FillMtg(p) {
		t.Fatal("FillMtg reported no change")
	}
	m := p.Set.Cards[0].Mtg
	if m.Rarity != "common" || m.CMC != 2 || m.Power != "2" || m.Toughness != "2" || p.Set.Mtg.SetCode != "DOM" {
		t.Fatalf("filter fields not filled: %+v", m)
	}
	if FillMtg(p) {
		t.Fatal("second FillMtg should be a no-op")
	}
}

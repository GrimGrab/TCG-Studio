package project

import (
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/game"
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

// The game's copy gets only the MTG fields: an installed price that differs from the project stays.
func TestPatchInstalledMtg(t *testing.T) {
	gameDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(gameDir, "Card Shop Simulator_Data"), 0o755); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(game.SetsDir(gameDir), "dom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	inst := &setfmt.Set{ID: "dom", Name: "Dominaria", Cards: []setfmt.Card{{ID: "1", Name: "Grizzly Bears", Price: setfmt.CardPrice{Base: 5}}}}
	if err := inst.Save(filepath.Join(dir, SetFile)); err != nil {
		t.Fatal(err)
	}
	p := &Project{ID: "dom",
		Set: &setfmt.Set{ID: "dom", Name: "Dominaria", Cards: []setfmt.Card{{ID: "1", Name: "Grizzly Bears", Price: setfmt.CardPrice{Base: 9}}}},
		Meta: &Meta{Source: "scryfall", ScryfallCode: "dom", Cards: map[string]CardMeta{"1": {Name: "Grizzly Bears", TypeLine: "Creature — Bear"}}},
	}
	FillMtg(p)
	changed, err := PatchInstalledMtg(p, gameDir)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	got, err := setfmt.Load(filepath.Join(dir, SetFile))
	if err != nil {
		t.Fatal(err)
	}
	if got.Mtg == nil || got.Mtg.SetCode != "DOM" || got.Cards[0].Mtg == nil || got.Cards[0].Mtg.Name != "Grizzly Bears" {
		t.Fatalf("MTG data not patched in: %+v / %+v", got.Mtg, got.Cards[0].Mtg)
	}
	if got.Cards[0].Price.Base != 5 {
		t.Fatalf("installed price changed to %v", got.Cards[0].Price.Base)
	}
	if again, _ := PatchInstalledMtg(p, gameDir); again {
		t.Fatal("second patch should be a no-op")
	}
	if changed, err := PatchInstalledMtg(&Project{ID: "other", Set: p.Set}, gameDir); changed || err != nil {
		t.Fatalf("not-installed set: changed=%v err=%v", changed, err)
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

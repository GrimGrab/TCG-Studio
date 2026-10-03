package tcgdex

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

func load(t *testing.T, name string) *Card {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name + ".json")
	if os.IsNotExist(err) {
		t.Skip("testdata is local only (not in the public source)")
	}
	if err != nil {
		t.Fatal(err)
	}
	var c Card
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return &c
}

func val(p *float64) float64 {
	if p == nil {
		return -1
	}
	return math.Round(*p*100) / 100
}

func TestPricesNormalAndReverse(t *testing.T) {
	c := load(t, "sv03.5-001") // Bulbasaur: normal + reverse holo on TCGplayer
	usd, foil, eur := c.Prices()
	if val(usd) != 0.26 || val(foil) != 0.36 || val(eur) != 0.13 {
		t.Fatalf("usd %v foil %v eur %v", val(usd), val(foil), val(eur))
	}
}

func TestPricesHoloOnly(t *testing.T) {
	c := load(t, "sv03.5-006") // Charizard ex: holofoil only, no reverse
	usd, foil, eur := c.Prices()
	if val(usd) != 8.88 || foil != nil || val(eur) != 8.54 {
		t.Fatalf("usd %v foil %v eur %v", val(usd), val(foil), val(eur))
	}
}

func TestPricesVintageHolo(t *testing.T) {
	c := load(t, "base1-4") // Base Set Charizard: holofoil, no reverse variant → no foil price from Cardmarket's -holo
	usd, foil, _ := c.Prices()
	if usd == nil || *usd < 100 || foil != nil {
		t.Fatalf("usd %v foil %v", val(usd), val(foil))
	}
}

func TestTextAndTypeLine(t *testing.T) {
	c := load(t, "sv03.5-006")
	if got := c.TypeLine(); got != "Pokémon — Fire Stage2 ex" {
		t.Errorf("type line %q", got)
	}
	if txt := c.Text(); !strings.Contains(txt, "Brave Wing 60+") || !strings.Contains(txt, "Explosive Vortex 330") {
		t.Errorf("text %q", txt)
	}
	if c.ImageURL("high") != "https://assets.tcgdex.net/en/sv/sv03.5/006/high.png" {
		t.Errorf("image %q", c.ImageURL("high"))
	}
	b := load(t, "base1-4")
	if !strings.Contains(b.Text(), "Pokemon Power: Energy Burn") {
		t.Errorf("ability text %q", b.Text())
	}
}

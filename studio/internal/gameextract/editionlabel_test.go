package gameextract

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"tcgstudio/internal/unityfs"
)

// The edition label read from the installed game (skipped without it): the game's spot under the card's middle, and a font
// with letters to draw (the label's own font has no glyph table; one of the same face stands in).
func TestEditionLabel(t *testing.T) {
	game := os.Getenv("TCG_GAME")
	if game == "" {
		game = `D:\SteamLibrary\steamapps\common\TCG Card Shop Simulator`
	}
	data, err := DataDir(game)
	if err != nil {
		t.Skip(err)
	}
	env, err := unityfs.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	if env.Scripts, err = unityfs.ScriptLayoutOf(filepath.Join(data, "Managed", "Assembly-CSharp.dll")); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	x := &extractor{env: env, dir: dir, progress: func(string) {}, texNames: map[*unityfs.Object]unityfs.Texture2D{}}
	if err := x.editionLabel(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, CardFramesDir, EditionLabelFile))
	if err != nil {
		t.Fatal(err)
	}
	var l EditionLabel
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	if math.Abs(l.CardH-785) > 5 || math.Abs(l.CardW-561.5) > 5 {
		t.Errorf("card %.1f × %.1f: want the vanilla card, about 561.5 × 785", l.CardW, l.CardH)
	}
	if math.Abs(l.CardX+3.7) > 0.5 {
		t.Errorf("card centre x %.2f: want about -3.7 (the game's card sits a little left)", l.CardX)
	}
	if l.Y >= 0 || math.Abs(l.X) > 1 || l.H <= 0 {
		t.Errorf("spot (%.1f, %.1f) box %.1f: want centred below the middle", l.X, l.Y, l.H)
	}
	if len(l.Font.Glyphs) < 60 || l.Font.Gradient <= 0 {
		t.Errorf("font %q: %d glyphs, gradient %v: want the letters to draw with", l.Font.Name, len(l.Font.Glyphs), l.Font.Gradient)
	}
	if _, err := os.Stat(filepath.Join(dir, CardFramesDir, l.Atlas)); err != nil {
		t.Errorf("atlas: %v", err)
	}
	checkModCard(t, l)
}

// checkModCard: the mod draws border cards and pictures at the vanilla card's size and centre from two literals
// (CardBorderView.CardScale, CardCentreX); they must still be what the game files say (a game update that changes the
// card's mask fails here instead of border cards drifting off the vanilla card).
func checkModCard(t *testing.T, l EditionLabel) {
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "src", "TCGCustomCards", "Patches", "CardBorderView.cs"))
	if err != nil {
		t.Skip("mod source not found:", err)
	}
	read := func(name string) float64 {
		m := regexp.MustCompile(`const float ` + name + ` = (-?[0-9.]+)f;`).FindSubmatch(src)
		if m == nil {
			t.Fatalf("CardBorderView.%s is no plain float literal", name)
		}
		v, _ := strconv.ParseFloat(string(m[1]), 64)
		return v
	}
	const front = 900.0 // the card front the mod scales from
	if scale := read("CardScale"); math.Abs(scale*front-l.CardH) > 0.5 {
		t.Errorf("mod CardScale %.4f = %.1f tall, the game's card is %.1f: set it to %.4f", scale, scale*front, l.CardH, l.CardH/front)
	}
	if cx := read("CardCentreX"); math.Abs(cx*front-l.CardX) > 0.3 {
		t.Errorf("mod CardCentreX %.5f = %.2f, the game's card centre is %.2f: set it to %.5f", cx, cx*front, l.CardX, l.CardX/front)
	}
}

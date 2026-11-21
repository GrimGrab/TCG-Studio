// Command manasymbols draws the MTG mana symbols (Scryfall's card symbology SVGs) as 64×64 PNGs into the mod's
// src\TCGCustomCards\Assets\mana\ folder, which the mod embeds (TCGCustomCards.mana.<code>.png). File name = the symbol
// without braces and slashes: {W} → W.png, {2/U} → 2U.png, {W/P} → WP.png. Run once from studio\ when symbols are added:
//
//	go run ./cmd/manasymbols
//
// Mana symbols © Wizards of the Coast, used under the Fan Content Policy (see licenses\NOTICES_HEADER.txt).
package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"

	"tcgstudio/internal/scryfall"
)

const size = 64

var plain = regexp.MustCompile(`^[A-Z0-9]{1,3}$`)

func main() {
	out := filepath.Join("..", "src", "TCGCustomCards", "Assets", "mana")
	if err := os.MkdirAll(out, 0o755); err != nil {
		fail(err)
	}
	ctx := context.Background()
	c := scryfall.New()
	syms, err := c.Symbology(ctx)
	if err != nil {
		fail(err)
	}
	n := 0
	for _, s := range syms {
		if !s.RepresentsMana || s.Funny || s.SVGURI == "" {
			continue
		}
		name := strings.NewReplacer("{", "", "}", "", "/", "").Replace(s.Symbol)
		if !plain.MatchString(name) {
			continue // ½, ∞, HW … (un-sets)
		}
		svg, err := c.Download(ctx, s.SVGURI)
		if err != nil {
			fail(fmt.Errorf("%s: %w", s.Symbol, err))
		}
		img, err := render(svg)
		if err != nil {
			fail(fmt.Errorf("%s: %w", s.Symbol, err))
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			fail(err)
		}
		if err := os.WriteFile(filepath.Join(out, name+".png"), buf.Bytes(), 0o644); err != nil {
			fail(err)
		}
		n++
	}
	fmt.Printf("%d symbols written to %s\n", n, out)
}

func render(svg []byte) (image.Image, error) {
	ic, err := oksvg.ReadIconStream(bytes.NewReader(svg), oksvg.IgnoreErrorMode)
	if err != nil {
		return nil, err
	}
	ic.SetTarget(0, 0, size, size)
	rgba := image.NewRGBA(image.Rect(0, 0, size, size))
	ic.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, rgba, rgba.Bounds())), 1)
	return rgba, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "manasymbols:", err)
	os.Exit(1)
}

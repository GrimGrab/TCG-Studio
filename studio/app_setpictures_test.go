package main

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/art"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

// Saving a set cuts each picture slot into its own file (named by picture + crop), points set.json at it, keeps an unchanged
// slot's file and drops slots of deleted packs.
func TestBakeSetPictures(t *testing.T) {
	lib := t.TempDir()
	img := image.NewNRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.NRGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	if err := art.SavePNG(filepath.Join(lib, "images", "photo.png"), img); err != nil {
		t.Fatal(err)
	}
	p := &project.Project{ID: "s", LibFolder: lib,
		Set: &setfmt.Set{Packs: []setfmt.Pack{{ID: "booster"}}},
		Meta: &project.Meta{Pictures: map[string]project.PictureSource{
			"cardBack":                {File: "images/photo.png"},
			"pack/booster/packIcon":   {File: "images/photo.png", Crop: []float64{0.1, 0.1, 0.3, 0}},
			"pack/booster/boxTexture": {File: "images/photo.png"},
			"pack/gone/packTexture":   {File: "images/photo.png"},
		}}}
	baked, err := bakeSetPictures(p)
	if err != nil {
		t.Fatal(err)
	}
	pk := p.Set.Packs[0]
	if p.Set.CardBack == "" || pk.PackIcon == "" || pk.BoxTexture == "" || pk.PackTexture != "" || len(baked) != 4 { // 3 cut + the deleted pack's slot
		t.Fatalf("set files: back %q icon %q box %q pack %q, baked %v", p.Set.CardBack, pk.PackIcon, pk.BoxTexture, pk.PackTexture, baked)
	}
	if _, ok := p.Meta.Pictures["pack/gone/packTexture"]; ok || baked["pack/gone/packTexture"] != "" {
		t.Error("the deleted pack's slot stayed")
	}
	size := func(rel string) image.Point {
		f, err := os.Open(p.ImagePath(rel))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		c, _, err := image.DecodeConfig(f)
		if err != nil {
			t.Fatal(err)
		}
		return image.Pt(c.Width, c.Height)
	}
	if s := size(pk.BoxTexture); s != image.Pt(1024, 1024) {
		t.Errorf("box texture %v, want 1024²", s)
	}
	if s := size(pk.PackIcon); s.X != 120 || s.Y < 228 || s.Y > 232 { // 0.3 × 400 wide, pack icon shape
		t.Errorf("pack icon %v", s)
	}
	// Same picture and crop: same file; another crop: another file.
	icon := pk.PackIcon
	if _, err := bakeSetPictures(p); err != nil || p.Set.Packs[0].PackIcon != icon {
		t.Fatalf("unchanged slot moved: %v %q", err, p.Set.Packs[0].PackIcon)
	}
	p.Meta.Pictures["pack/booster/packIcon"] = project.PictureSource{File: "images/photo.png", Crop: []float64{0.2, 0.1, 0.3, 0}}
	if _, err := bakeSetPictures(p); err != nil || p.Set.Packs[0].PackIcon == icon {
		t.Fatalf("new crop kept the old file: %v", err)
	}
	p.Meta.Pictures["cardBack"] = project.PictureSource{File: "images/missing.png"}
	if _, err := bakeSetPictures(p); err == nil {
		t.Error("a missing picture saved")
	}
	delete(p.Meta.Pictures, "cardBack")
	// The pack art editor wrote another box texture: its picture goes, the editor's file stays.
	p.Set.Packs[0].BoxTexture = "images/editor.png"
	if baked, err := bakeSetPictures(p); err != nil || p.Set.Packs[0].BoxTexture != "images/editor.png" || baked["pack/booster/boxTexture"] != "" {
		t.Fatalf("replaced slot: %v %q %v", err, p.Set.Packs[0].BoxTexture, baked)
	}
	if _, ok := p.Meta.Pictures["pack/booster/boxTexture"]; ok {
		t.Error("the replaced slot stayed")
	}
}

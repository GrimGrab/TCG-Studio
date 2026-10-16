package art

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// A 600×1000 picture with a 50 px black frame around a pattern: the auto crop starts inside the frame, card-shaped.
func TestCardBackAutoCrop(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 600, 1000))
	for y := 0; y < 1000; y++ {
		for x := 0; x < 600; x++ {
			c := color.NRGBA{A: 255}
			if x >= 50 && x < 550 && y >= 50 && y < 950 {
				c = color.NRGBA{uint8(x), uint8(y), uint8(x ^ y), 255}
			}
			src.Set(x, y, c)
		}
	}
	c := CardBackAutoCrop(src)
	x, y, w, h := c[0]*600, c[1]*1000, c[2]*600, c[3]*1000
	if x < 49 || y < 49 || x+w > 551 || y+h > 951 {
		t.Fatalf("crop %v leaves the frame (px %.0f,%.0f %.0f×%.0f)", c, x, y, w, h)
	}
	if math.Abs(w/h-CardBackAspect) > 0.01 {
		t.Fatalf("crop shape %.3f, want %.3f", w/h, CardBackAspect)
	}
	if math.Abs(w-500) > 2 { // the frame's inside is narrower than a card: the full width is used
		t.Fatalf("crop width %.0f, want 500", w)
	}

	dir := t.TempDir()
	in, out := filepath.Join(dir, "src.png"), filepath.Join(dir, "back.png")
	f, err := os.Create(in)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, src); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := CardBackFromCrop(in, c, out); err != nil {
		t.Fatal(err)
	}
	cfg, err := readConfig(out)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != backArt.Dx() || cfg.Height != backArt.Dy() {
		t.Fatalf("card back %d×%d, want %v", cfg.Width, cfg.Height, backArt.Size())
	}
}

func readConfig(p string) (image.Config, error) {
	f, err := os.Open(p)
	if err != nil {
		return image.Config{}, err
	}
	defer f.Close()
	c, _, err := image.DecodeConfig(f)
	return c, err
}

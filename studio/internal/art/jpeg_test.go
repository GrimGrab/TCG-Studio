package art

import (
	"image"
	"image/color"
	"testing"
)

func TestOpaqueFillsCorners(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 140))
	for y := 0; y < 140; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.NRGBA{20, 20, 20, 255}) // dark border colour
		}
	}
	img.Set(0, 0, color.NRGBA{}) // transparent corner
	out := Opaque(img)
	r, g, b, a := out.At(0, 0).RGBA()
	if a != 0xffff || r>>8 > 40 || g>>8 > 40 || b>>8 > 40 {
		t.Fatalf("corner = %d %d %d %d", r>>8, g>>8, b>>8, a>>8)
	}
}

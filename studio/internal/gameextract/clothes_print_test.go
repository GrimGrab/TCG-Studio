package gameextract

import (
	"image"
	"image/color"
	"testing"
)

// The painted pixel at UV (u, v) must end up where the game samples it: texture (u, v) + PrintUVOffset, wrapping.
func TestBakeClothesPrint(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	mark := color.NRGBA{255, 0, 0, 255}
	src.SetNRGBA(1, 0, mark)
	got := BakeClothesPrint(src)
	if got.NRGBAAt(3, 2) != mark {
		t.Fatalf("painted pixel (1,0) should be at (3,2) in the game's texture, got %v there", got.NRGBAAt(3, 2))
	}
	if back := BakeClothesPrint(got); back.NRGBAAt(1, 0) != mark {
		t.Fatalf("baking twice should give the painting back (the offset is half the texture)")
	}
}

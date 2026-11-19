package packmesh

import (
	"math"
	"testing"
)

// The back's two halves overlapped in the texture (the right half's strip starts where the left half's already was); fixed,
// u follows x in one line from edge to edge, and nothing outside the back moves.
func TestFixBackOneStrip(t *testing.T) {
	m := Mesh{
		Pos: []Vec3{{-1, 0, 0}, {-0.1, 0, 0}, {0.1, 0, 0}, {1, 0, 0}, {0, 0, 0}},
		UV:  []Vec2{{0.9, 0.5}, {0.55, 0.5}, {0.6, 0.5}, {0.3, 0.5}, {0.1, 0.5}}, // halves 0.9→0.55 and 0.6→0.3 overlap; last = fin
	}
	back := func(uv Vec2) bool { return uv[0] >= 0.25 && uv[0] <= 0.95 }
	out, n, err := FixBack(m, back)
	if err != nil || n != 2 {
		t.Fatalf("changed %d, err %v", n, err)
	}
	for i, want := range []float64{0.9, 0.63, 0.57, 0.3, 0.1} {
		if math.Abs(out.UV[i][0]-want) > 1e-9 {
			t.Fatalf("vertex %d: u %v, want %v", i, out.UV[i][0], want)
		}
	}
	if m.UV[1][0] != 0.55 {
		t.Fatal("the input mesh changed")
	}
}

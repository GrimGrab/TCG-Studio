package uvmap

import "testing"

// A rect's free sides take in one more texel; a side where a neighbouring area is, or the texture's edge, stays put; the
// original model and palettes are not changed.
func TestInclusive(t *testing.T) {
	m := Model{TextureSize: 100, Faces: []Face{
		{ID: "a", Targets: []Target{{Rect: [4]float64{0, 0, 40, 50}}}},   // right edge meets b, bottom edge is free
		{ID: "b", Targets: []Target{{Rect: [4]float64{40, 0, 100, 50}}}}, // left edge meets a, right edge is the texture's edge
		{ID: "c", Targets: []Target{{Rect: [4]float64{60, 70, 80, 90}}}}, // free on every side
	}}
	got := inclusive(m)
	if r := got.Faces[0].Targets[0].Rect; r != [4]float64{0, 0, 40, 51} {
		t.Errorf("a = %v, want right edge kept (b starts there) and bottom edge +1", r)
	}
	if r := got.Faces[1].Targets[0].Rect; r != [4]float64{40, 0, 100, 51} {
		t.Errorf("b = %v, want right edge kept (texture edge) and bottom edge +1", r)
	}
	if r := got.Faces[2].Targets[0].Rect; r != [4]float64{59, 69, 81, 91} {
		t.Errorf("c = %v, want one texel more on every side", r)
	}
	if m.Faces[0].Targets[0].Rect[3] != 50 {
		t.Error("inclusive changed the original model")
	}
	m.Palette = true
	if r := inclusive(m).Faces[0].Targets[0].Rect; r[3] != 50 {
		t.Errorf("palette changed: %v", r)
	}
}

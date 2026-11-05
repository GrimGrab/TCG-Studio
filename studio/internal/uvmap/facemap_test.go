package uvmap

import (
	"math"
	"testing"
)

// A row naming an unknown face makes a new face, with all its rows, below the others in the net; none is lost.
func TestFaceMapNewFace(t *testing.T) {
	m := faceMap(Model{Faces: []Face{{ID: "front", Label: "Front", Net: [4]float64{0, 0, 1, 1}}}}, []Row{
		{Face: "front", Rect: [4]float64{0, 0, 10, 10}},
		{Face: "Rim", Part: "A", Rect: [4]float64{20, 0, 120, 10}},
		{Face: "Rim", Part: "B", Rect: [4]float64{20, 20, 120, 30}},
	})
	if len(m.Faces) != 2 || m.Faces[1].Label != "Rim" || len(m.Faces[1].Targets) != 2 {
		t.Fatalf("faces %+v", m.Faces)
	}
	if n := m.Faces[1].Net; n[1] <= 1 || n[2] <= n[3] {
		t.Errorf("new face net %v", n)
	}
}

func TestTargetPaints(t *testing.T) {
	u := Target{Rect: [4]float64{0, 0, 10, 10}, Shape: [][2]float64{{0, 0}, {10, 0}, {10, 10}, {8, 10}, {8, 2}, {2, 2}, {2, 10}, {0, 10}}} // an arch
	for _, c := range []struct {
		x, y float64
		want bool
	}{{1, 5, true}, {5, 1, true}, {9, 9, true}, {5, 5, false}, {11, 5, false}} {
		if got := u.Paints(c.x, c.y); got != c.want {
			t.Errorf("Paints(%v, %v) = %v", c.x, c.y, got)
		}
	}
	if !(Target{Rect: [4]float64{0, 0, 10, 10}}).Paints(5, 5) {
		t.Error("no shape: the whole rectangle paints")
	}
}

// A new face has the same pixels per net unit as the model's other faces, whatever the texture's size.
func TestFaceMapNewFaceScale(t *testing.T) {
	m := faceMap(Model{Faces: []Face{{ID: "top", Label: "Top", Net: [4]float64{0, 0, 2, 1}}}}, []Row{
		{Face: "top", Rect: [4]float64{0, 0, 1000, 500}},
		{Face: "Rim", Rect: [4]float64{0, 600, 400, 800}},
		{Face: "Turned", Rect: [4]float64{0, 900, 200, 1300}, Turn: true},
	})
	if n := m.Faces[1].Net; math.Abs(n[2]-0.8) > 1e-9 || math.Abs(n[3]-0.4) > 1e-9 {
		t.Errorf("Rim net %v, want 0.8 × 0.4", n)
	}
	if n := m.Faces[2].Net; math.Abs(n[2]-0.8) > 1e-9 || math.Abs(n[3]-0.4) > 1e-9 {
		t.Errorf("Turned net %v, want 0.8 × 0.4", n)
	}
}

package packmesh

import (
	"math"
	"testing"
)

// Splitting a polygon by a convex one keeps all of its area: inside + outside pieces = the polygon.
func TestSplitKeepsArea(t *testing.T) {
	r := []Vec2{{0, 0}, {4, 0}, {4, 3}, {0, 3}}
	for _, c := range [][]Vec2{
		{{1, 1}, {2, 1}, {2, 2}},             // inside
		{{3, 2}, {6, 2}, {6, 5}, {3, 5}},     // over a corner
		{{10, 10}, {11, 10}, {11, 11}},       // outside
		{{-1, -1}, {5, -1}, {5, 4}, {-1, 4}}, // covers it all
	} {
		in, out := split(r, ccw(c))
		sum := math.Abs(area2(in))
		for _, o := range out {
			sum += math.Abs(area2(o))
		}
		if math.Abs(sum-12) > 1e-9 {
			t.Fatalf("clip %v: pieces add up to %v, want 12", c, sum)
		}
	}
}

// A flat sheet printed on +z takes the UVs of a shelf quad printed on -z, turned half round (x mirrored back): the sheet's
// right edge (+x) shows the shelf's left edge (-x). A shelf UV seam down the middle cuts the sheet's triangles there.
func TestRemapFlatSheet(t *testing.T) {
	sheet := Mesh{
		Pos:  []Vec3{{-1, -1, 0}, {1, -1, 0}, {1, 1, 0}, {-1, 1, 0}},
		Norm: []Vec3{{0, 0, 1}, {0, 0, 1}, {0, 0, 1}, {0, 0, 1}},
		UV:   []Vec2{{0, 0}, {0, 0}, {0, 0}, {0, 0}},
		Tris: [][3]int{{0, 1, 2}, {0, 2, 3}},
	}
	for range sheet.Pos {
		sheet.Joints = append(sheet.Joints, [4]int{0, 0, 0, 0})
		sheet.Weights = append(sheet.Weights, [4]float64{1, 0, 0, 0})
	}
	// Shelf: two quads facing -z with a texture seam at x = 0 (left half u 0..0.25, right half u 0.5..0.75).
	shelf := Mesh{
		Pos:  []Vec3{{-1, -1, 0}, {0, -1, 0}, {0, 1, 0}, {-1, 1, 0}, {0, -1, 0}, {1, -1, 0}, {1, 1, 0}, {0, 1, 0}},
		Norm: []Vec3{{0, 0, -1}, {0, 0, -1}, {0, 0, -1}, {0, 0, -1}, {0, 0, -1}, {0, 0, -1}, {0, 0, -1}, {0, 0, -1}},
		UV:   []Vec2{{0, 0}, {0.25, 0}, {0.25, 1}, {0, 1}, {0.5, 0}, {0.75, 0}, {0.75, 1}, {0.5, 1}},
		Tris: [][3]int{{0, 2, 1}, {0, 3, 2}, {4, 6, 5}, {4, 7, 6}},
	}
	out, st := Remap(sheet, sheet.Pos, sheet.Norm, []Bone{{Name: "Root", Parent: -1}}, shelf, func(Vec2) bool { return false })
	if st.Outside != 2 || st.Uncovered != 0 || st.Pieces < 4 {
		t.Fatalf("stats %+v", st)
	}
	for i, p := range out.Pos {
		u := out.UV[i][0]
		// turned: sheet x maps to shelf -x
		sx := -p[0]
		var want float64
		switch {
		case sx < -1e-9:
			want = (sx + 1) * 0.25
		case sx > 1e-9:
			want = 0.5 + sx*0.25
		default:
			if math.Abs(u-0.25) > 1e-9 && math.Abs(u-0.5) > 1e-9 {
				t.Fatalf("vertex on the seam at %v has u %v", p, u)
			}
			continue
		}
		if math.Abs(u-want) > 1e-9 {
			t.Fatalf("vertex %v: u %v, want %v", p, u, want)
		}
	}
	for _, tr := range out.Tris {
		// no triangle spans the seam: its corners' u all on one side
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, i := range tr {
			lo, hi = math.Min(lo, out.UV[i][0]), math.Max(hi, out.UV[i][0])
		}
		if lo < 0.25+1e-9 && hi > 0.5-1e-9 {
			t.Fatalf("triangle %v spans the seam (u %v..%v)", tr, lo, hi)
		}
	}
}

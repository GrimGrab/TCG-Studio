package decoart

import (
	"image"
	"image/color"
	"math"
	"testing"

	"tcgstudio/internal/figurine"
)

// Unity draws clockwise triangles; for those Vector3.Cross(b - a, c - a) is the outward normal.
func checkWinding(t *testing.T, m *figurine.Mesh) {
	t.Helper()
	for i := 0; i+2 < len(m.Idx); i += 3 {
		a, b, c := m.Pos[m.Idx[i]], m.Pos[m.Idx[i+1]], m.Pos[m.Idx[i+2]]
		g := cross([3]float64{b[0] - a[0], b[1] - a[1], b[2] - a[2]}, [3]float64{c[0] - a[0], c[1] - a[1], c[2] - a[2]})
		n := m.Nrm[m.Idx[i]]
		if g[0]*n[0]+g[1]*n[1]+g[2]*n[2] <= 0 {
			t.Fatalf("triangle %d winds against its normal %v", i/3, n)
		}
	}
}

func TestPoster(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 300, 400))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	m, tex, err := BuildPoster(img, Poster{Width: 0.45, Frame: 0.05, Color: "#ffffff"})
	if err != nil {
		t.Fatal(err)
	}
	checkWinding(t, m)
	lo, hi := m.Bounds()
	if math.Abs(hi[0]-lo[0]-0.45) > 1e-9 || math.Abs(lo[2]) > 1e-9 || math.Abs(hi[2]-0.002) > 1e-9 {
		t.Fatalf("bounds %v %v", lo, hi)
	}
	// 300×400 image + 15 px frame each side → 330×430 board.
	if want := 0.45 * 430 / 330; math.Abs(hi[1]-lo[1]-want) > 1e-9 {
		t.Fatalf("height %v, want %v", hi[1]-lo[1], want)
	}
	// Front face: top-left of the image (u = 0) is at +x (seen from the front, right is −x).
	if m.Pos[0][0] <= 0 || m.Pos[0][1] <= 0 || m.UV[0] != [2]float64{0, 0} {
		t.Fatalf("front top-left vertex %v uv %v", m.Pos[0], m.UV[0])
	}
	if tex.Bounds().Dx() != 330+8 || tex.Bounds().Dy() != 430 {
		t.Fatalf("texture %v", tex.Bounds())
	}
	if _, _, err := BuildPoster(img, Poster{Width: 0.4, Color: "white"}); err == nil {
		t.Fatal("bad colour accepted")
	}
}

func TestOnWall(t *testing.T) {
	m, _, _ := BuildPoster(image.NewNRGBA(image.Rect(0, 0, 10, 10)), Poster{Width: 1})
	// Pretend it is a figurine placement facing −z standing on y = 0: mirror it back, then put it on the wall.
	for i := range m.Pos {
		m.Pos[i][0], m.Pos[i][2] = -m.Pos[i][0], -m.Pos[i][2]+0.3
		m.Pos[i][1] += 0.5
		m.Nrm[i][0], m.Nrm[i][2] = -m.Nrm[i][0], -m.Nrm[i][2]
	}
	OnWall(m)
	checkWinding(t, m)
	lo, hi := m.Bounds()
	if math.Abs(lo[2]) > 1e-9 || math.Abs(lo[1]+hi[1]) > 1e-9 {
		t.Fatalf("not on the wall: %v %v", lo, hi)
	}
}

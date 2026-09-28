package uvmap

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// TestRenderOrientation (set RENDER_OUT=<dir>, optional RENDER_KIND) paints every face of a model with its label through the
// model's targets, then renders the real mesh orthographically from the front, back, left, right and top. Labels must read
// upright and unmirrored in every view — a visual check of the targets' flips.
func TestRenderOrientation(t *testing.T) {
	out := os.Getenv("RENDER_OUT")
	if out == "" {
		t.Skip("set RENDER_OUT")
	}
	dir := templatesDir(t)
	kinds := []string{"Pack", "Box"}
	if k := os.Getenv("RENDER_KIND"); k != "" {
		kinds = []string{k}
	}
	for _, kind := range kinds {
		m, _ := ModelFor(kind)
		if kind == "LegendaryCardBox" { // a box base: its targets on its own mesh
			m, _ = ModelFor("Box")
			b := m.Bases[3]
			faces := append([]Face(nil), m.Faces...)
			for i := range faces {
				faces[i].Targets = b.Targets[faces[i].ID]
			}
			m.Faces, m.Mesh = faces, "CardBoxMesh_4"
		}
		mesh, err := LoadOBJ(filepath.Join(dir, m.Mesh+".obj"))
		if err != nil {
			t.Fatal(err)
		}
		tex := labelTexture(m)
		savePNGTest(filepath.Join(out, kind+"_texture.png"), tex)
		views := []struct {
			name string
			dir  Vec3 // viewing direction (camera looks along it)
			up   Vec3
		}{
			{"front", Vec3{0, 0, 1}, Vec3{0, 1, 0}}, {"back", Vec3{0, 0, -1}, Vec3{0, 1, 0}},
			{"right", Vec3{-1, 0, 0}, Vec3{0, 1, 0}}, {"left", Vec3{1, 0, 0}, Vec3{0, 1, 0}},
			{"top", Vec3{0, -1, 0}, Vec3{0, 0, 1}},
		}
		for _, v := range views {
			savePNGTest(filepath.Join(out, kind+"_"+v.name+".png"), renderOrtho(mesh, tex, v.dir, v.up, 400))
		}
	}
}

// labelTexture: each face gets a colour and its label (plus an arrow-ish "^ TOP" marker at its top), mapped by the targets.
func labelTexture(m Model) *image.NRGBA {
	s := m.TextureSize
	tex := image.NewNRGBA(image.Rect(0, 0, s, s))
	draw.Draw(tex, tex.Bounds(), &image.Uniform{color.NRGBA{40, 40, 40, 255}}, image.Point{}, draw.Src)
	cols := []color.NRGBA{{200, 60, 60, 255}, {60, 160, 60, 255}, {60, 90, 200, 255}, {190, 150, 40, 255}, {150, 60, 180, 255}, {40, 160, 170, 255}}
	for i, f := range m.Faces {
		// Face image at 100 px per unit, upright as seen from outside.
		fw, fh := max(8, int(f.Net[2]*300)), max(8, int(f.Net[3]*300))
		face := image.NewNRGBA(image.Rect(0, 0, fw, fh))
		draw.Draw(face, face.Bounds(), &image.Uniform{cols[i%len(cols)]}, image.Point{}, draw.Src)
		drawLabel(face, "^TOP", 4, 14)
		drawLabel(face, f.Label, 4, fh/2)
		drawLabel(face, "L", 4, fh-6)
		for _, tg := range f.Targets {
			if tg.Bleed {
				continue
			}
			sx0, sy0 := tg.Src[0]*float64(fw), tg.Src[1]*float64(fh)
			sw, sh := (tg.Src[2]-tg.Src[0])*float64(fw), (tg.Src[3]-tg.Src[1])*float64(fh)
			r := tg.Rect
			for y := int(r[1]); y < int(r[3]); y++ {
				for x := int(r[0]); x < int(r[2]); x++ {
					// texture-space normalized coords, undo flips, then transpose
					u := (float64(x) + 0.5 - r[0]) / (r[2] - r[0])
					w := (float64(y) + 0.5 - r[1]) / (r[3] - r[1])
					if tg.FlipX {
						u = 1 - u
					}
					if tg.FlipY {
						w = 1 - w
					}
					if tg.Transpose {
						u, w = w, u
					}
					tex.SetNRGBA(x, y, face.NRGBAAt(int(sx0+u*sw), int(sy0+w*sh)))
				}
			}
		}
	}
	return tex
}

func drawLabel(img *image.NRGBA, s string, x, y int) {
	// basicfont is 7x13; draw it 2x for legibility.
	small := image.NewNRGBA(image.Rect(0, 0, len(s)*7+2, 16))
	d := font.Drawer{Dst: small, Src: image.White, Face: basicfont.Face7x13, Dot: fixed.P(1, 12)}
	d.DrawString(s)
	for yy := 0; yy < 16*2; yy++ {
		for xx := 0; xx < small.Bounds().Dx()*2; xx++ {
			if small.NRGBAAt(xx/2, yy/2).A > 0 {
				img.SetNRGBA(x+xx, y-24+yy, color.NRGBA{255, 255, 255, 255})
			}
		}
	}
}

func renderOrtho(m *Mesh, tex *image.NRGBA, dir, up Vec3, size int) *image.NRGBA {
	right := cross(up, dir) // left-handed Unity: camera right = up × forward
	norm := func(v Vec3) Vec3 {
		l := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
		return Vec3{v[0] / l, v[1] / l, v[2] / l}
	}
	right = norm(right)
	dot := func(a, b Vec3) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
	// Bounds in view space
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range m.Pos {
		x, y := dot(p, right), dot(p, up)
		minX, maxX, minY, maxY = math.Min(minX, x), math.Max(maxX, x), math.Min(minY, y), math.Max(maxY, y)
	}
	sc := float64(size) * 0.9 / math.Max(maxX-minX, maxY-minY)
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.NRGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)
	zbuf := make([]float64, size*size)
	for i := range zbuf {
		zbuf[i] = math.Inf(1)
	}
	proj := func(p Vec3) (float64, float64, float64) {
		return float64(size)/2 + (dot(p, right)-(minX+maxX)/2)*sc, float64(size)/2 - (dot(p, up)-(minY+maxY)/2)*sc, dot(p, dir)
	}
	ts := float64(tex.Bounds().Dx())
	for _, tr := range m.Tris {
		var X, Y, Z [3]float64
		for k := 0; k < 3; k++ {
			X[k], Y[k], Z[k] = proj(m.Pos[tr[k]])
		}
		area := (X[1]-X[0])*(Y[2]-Y[0]) - (X[2]-X[0])*(Y[1]-Y[0])
		if math.Abs(area) < 1e-9 {
			continue
		}
		x0, x1 := int(math.Max(0, math.Min(X[0], math.Min(X[1], X[2])))), int(math.Min(float64(size-1), math.Max(X[0], math.Max(X[1], X[2]))))
		y0, y1 := int(math.Max(0, math.Min(Y[0], math.Min(Y[1], Y[2])))), int(math.Min(float64(size-1), math.Max(Y[0], math.Max(Y[1], Y[2]))))
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				px, py := float64(x)+0.5, float64(y)+0.5
				w0 := ((X[1]-px)*(Y[2]-py) - (X[2]-px)*(Y[1]-py)) / area
				w1 := ((X[2]-px)*(Y[0]-py) - (X[0]-px)*(Y[2]-py)) / area
				w2 := 1 - w0 - w1
				if w0 < 0 || w1 < 0 || w2 < 0 {
					continue
				}
				z := w0*Z[0] + w1*Z[1] + w2*Z[2]
				if z >= zbuf[y*size+x] {
					continue
				}
				zbuf[y*size+x] = z
				u := w0*m.UV[tr[0]][0] + w1*m.UV[tr[1]][0] + w2*m.UV[tr[2]][0]
				v := w0*m.UV[tr[0]][1] + w1*m.UV[tr[1]][1] + w2*m.UV[tr[2]][1]
				tx, ty := int(u*ts), int((1-v)*ts)
				img.SetNRGBA(x, y, tex.NRGBAAt(min(max(tx, 0), int(ts)-1), min(max(ty, 0), int(ts)-1)))
			}
		}
	}
	return img
}

func savePNGTest(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	png.Encode(f, img)
}

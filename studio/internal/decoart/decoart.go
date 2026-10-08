// Package decoart builds decoration models for the mod (Runtime/DecorationInjector): posters made from an image, and the
// placement of imported models on a wall or the floor. Output is a figurine.Mesh in the decoration's root space (Unity: left-handed,
// Y up, metres; UVs in image space, origin top-left, as figurine.GameOBJ expects) with triangles wound clockwise seen from outside.
//
// Wall decorations: the wall is the z = 0 plane and the decoration faces +z, centred on x = y = 0 (like the game's Poster 1,
// a 0.45 × 0.61 m board 2 mm thick). Floor decorations stand on y = 0, centred on x/z.
package decoart

import (
	"fmt"
	"image"
	"image/color"
	"math"

	xdraw "golang.org/x/image/draw"

	"tcgstudio/internal/figurine"
)

// Poster settings (Studio's poster editor; kept in the decoration's layout).
type Poster struct {
	Width float64 `json:"width"` // metres, the whole board (frame included); height follows the image's aspect
	Frame float64 `json:"frame"` // frame width as a fraction of the shorter side (0 = no frame)
	Color string  `json:"color"` // frame and edge colour, #RRGGBB
	Depth float64 `json:"depth"` // board thickness in metres (0 = 2 mm, the game's posters)
}

// MaxTexture is the longest side of a poster texture.
const MaxTexture = 2048

// BuildPoster makes the board and its texture: the image (scaled to fit MaxTexture) inside a frame of p.Color, plus a small
// strip of that colour the edges and back map to.
func BuildPoster(img image.Image, p Poster) (*figurine.Mesh, *image.NRGBA, error) {
	b := img.Bounds()
	if b.Dx() < 2 || b.Dy() < 2 {
		return nil, nil, fmt.Errorf("the image is too small")
	}
	if p.Width <= 0 {
		return nil, nil, fmt.Errorf("width must be > 0")
	}
	frameCol, err := ParseColor(p.Color)
	if err != nil {
		return nil, nil, err
	}
	// Texture: frame + image, then an edge strip on the right.
	scale := math.Min(1, float64(MaxTexture-16)/math.Max(float64(b.Dx()), float64(b.Dy())))
	iw, ih := int(math.Round(float64(b.Dx())*scale)), int(math.Round(float64(b.Dy())*scale))
	fr := int(math.Round(math.Max(0, p.Frame) * math.Min(float64(iw), float64(ih))))
	cw, ch := iw+2*fr, ih+2*fr
	const strip = 8
	tex := image.NewNRGBA(image.Rect(0, 0, cw+strip, ch))
	xdraw.Draw(tex, tex.Bounds(), image.NewUniform(frameCol), image.Point{}, xdraw.Src)
	xdraw.CatmullRom.Scale(tex, image.Rect(fr, fr, fr+iw, fr+ih), img, b, xdraw.Src, nil)

	w := p.Width
	h := w * float64(ch) / float64(cw)
	d := p.Depth
	if d <= 0 {
		d = 0.002
	}
	tw := float64(cw + strip)
	front := [4][2]float64{{0, 0}, {float64(cw) / tw, 0}, {float64(cw) / tw, 1}, {0, 1}} // TL, TR, BR, BL (image space)
	su := (float64(cw) + strip/2) / tw
	edge := [4][2]float64{{su, 0.5}, {su, 0.5}, {su, 0.5}, {su, 0.5}}
	m := &figurine.Mesh{}
	hw, hh := w/2, h/2
	// Front (+z), back (−z), sides: centre, outward normal, up, face width/height.
	box(m, [3]float64{0, 0, d}, [3]float64{0, 0, 1}, [3]float64{0, 1, 0}, w, h, front)
	box(m, [3]float64{0, 0, 0}, [3]float64{0, 0, -1}, [3]float64{0, 1, 0}, w, h, edge)
	box(m, [3]float64{hw, 0, d / 2}, [3]float64{1, 0, 0}, [3]float64{0, 1, 0}, d, h, edge)
	box(m, [3]float64{-hw, 0, d / 2}, [3]float64{-1, 0, 0}, [3]float64{0, 1, 0}, d, h, edge)
	box(m, [3]float64{0, hh, d / 2}, [3]float64{0, 1, 0}, [3]float64{0, 0, 1}, w, d, edge)
	box(m, [3]float64{0, -hh, d / 2}, [3]float64{0, -1, 0}, [3]float64{0, 0, 1}, w, d, edge)
	return m, tex, nil
}

// box adds one quad (a face of a box) seen from outside: n = outward normal, up = the face's up; right = n × up (left-handed space,
// numerically the same formula: seen from +z looking at −z, right is −x). Triangles TL-TR-BR, TL-BR-BL are clockwise from outside.
func box(m *figurine.Mesh, c, n, up [3]float64, w, h float64, uv [4][2]float64) {
	r := cross(n, up)
	corner := func(sx, sy float64) [3]float64 {
		return [3]float64{c[0] + r[0]*sx*w/2 + up[0]*sy*h/2, c[1] + r[1]*sx*w/2 + up[1]*sy*h/2, c[2] + r[2]*sx*w/2 + up[2]*sy*h/2}
	}
	base := uint32(len(m.Pos))
	for i, p := range [4][3]float64{corner(-1, 1), corner(1, 1), corner(1, -1), corner(-1, -1)} {
		m.Pos = append(m.Pos, p)
		m.Nrm = append(m.Nrm, n)
		m.UV = append(m.UV, uv[i])
	}
	m.Idx = append(m.Idx, base, base+1, base+2, base, base+2, base+3)
}

func cross(a, b [3]float64) [3]float64 {
	return [3]float64{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}

// ParseColor reads #RRGGBB ("" = near black, the vanilla poster edge).
func ParseColor(s string) (color.NRGBA, error) {
	if s == "" {
		return color.NRGBA{0x20, 0x20, 0x20, 0xff}, nil
	}
	var r, g, b uint8
	if len(s) != 7 || s[0] != '#' {
		return color.NRGBA{}, fmt.Errorf("colour %q is not #RRGGBB", s)
	}
	if _, err := fmt.Sscanf(s[1:], "%02x%02x%02x", &r, &g, &b); err != nil {
		return color.NRGBA{}, fmt.Errorf("colour %q is not #RRGGBB", s)
	}
	return color.NRGBA{r, g, b, 0xff}, nil
}

// OnWall moves a placed model (figurine.Place output: bottom-centre on the origin, facing −z like figurines) onto the wall: turned
// to face +z, its back on the z = 0 plane and centred on y = 0.
func OnWall(m *figurine.Mesh) {
	for i := range m.Pos {
		m.Pos[i][0], m.Pos[i][2] = -m.Pos[i][0], -m.Pos[i][2]
		m.Nrm[i][0], m.Nrm[i][2] = -m.Nrm[i][0], -m.Nrm[i][2]
	}
	lo, hi := m.Bounds()
	dy, dz := -(lo[1]+hi[1])/2, -lo[2]
	for i := range m.Pos {
		m.Pos[i][1] += dy
		m.Pos[i][2] += dz
	}
}

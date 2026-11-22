// Command stockborders writes the stock card borders (internal/cardborder/stock): stock.json and the gold and silver leaf
// pictures and icons, built into the mod's DLL and Studio. Run it in studio\ after changing this file:
//
//	go run ./cmd/stockborders
//
// A leaf border is a card-shaped picture: a rim of metal leaf laid in flakes (each flake a little lighter or darker, thin
// cracks between them, a fine grain), rounded outer corners, a bevel along the inside edge and the window for the card's
// picture left see-through. Its shader layer (stock shader "tcgcc-holo-leaf") makes the flakes catch the light.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	"tcgstudio/internal/cardshader"
	"tcgstudio/internal/setfmt"
)

// The picture's size (63×88 mm at about 300 dpi: the shape the mod gives a card with this border).
const (
	W = 750
	H = 1048
)

// The window: the card picture fills it (the same shape as a 63×88 card, so the picture is barely cropped). The rim is as
// thin as the game's own around a full-image picture: the picture is 0.9 × 0.9 of the 900-unit card box (729 × 522) on
// the 785 × 561.5 vanilla card, 3.5 % each side (it was 4.5 %, a rim the user found too thick, 2026-11-22).
var window = [4]float64{0.0351, 0.0357, 0.9298, 0.9286}

type leaf struct {
	id, name, file string
	base           [3]float64 // the leaf's colour, 0..1
	seed           int64
}

var leaves = []leaf{
	{"tcgcc-border-gold-leaf", "Gold leaf", "gold_leaf", [3]float64{0.88, 0.66, 0.24}, 7},
	{"tcgcc-border-silver-leaf", "Silver leaf", "silver_leaf", [3]float64{0.80, 0.82, 0.85}, 11},
}

// stock is one stock border: its picture and the stock shader drawn on it ("" = a picture only).
type stock struct {
	id, name, file string
	paint          func() *image.NRGBA
	shader         string
}

func borders() []stock {
	var out []stock
	for _, l := range leaves {
		out = append(out, stock{l.id, l.name, l.file, func() *image.NRGBA { return paint(l) }, "leaf"})
	}
	return append(out, styles...)
}

func main() {
	dir := filepath.Join("internal", "cardborder", "stock")
	var list []setfmt.Border
	for _, s := range borders() {
		img := s.paint()
		if err := writePNG(filepath.Join(dir, s.file+".png"), img); err != nil {
			fail(err)
		}
		if err := writePNG(filepath.Join(dir, s.file+"_icon.png"), icon(img)); err != nil {
			fail(err)
		}
		layers := []setfmt.BorderLayer{{Kind: setfmt.BorderImageLayer, File: s.file + ".png", Opacity: 1}}
		if s.shader != "" {
			layers = append(layers, setfmt.BorderLayer{Kind: setfmt.BorderShaderLayer, Shader: cardshader.StockPrefix + s.shader, Opacity: 1, Clip: true})
		}
		list = append(list, setfmt.Border{
			ID: s.id, Name: s.name, Icon: s.file + "_icon.png",
			Window: []float64{window[0], window[1], window[2], window[3]},
			Layers: layers,
		})
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(list); err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stock.json"), b.Bytes(), 0o644); err != nil {
		fail(err)
	}
	fmt.Println("Wrote", dir)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func writePNG(path string, img image.Image) error {
	var b bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&b, img); err != nil {
		return err
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// hash is a repeatable random number 0..1 for a cell.
func hash(x, y int, seed int64) float64 {
	h := uint64(x)*0x9E3779B97F4A7C15 ^ uint64(y)*0xC2B2AE3D27D4EB4F ^ uint64(seed)*0x165667B19E3779F9
	h ^= h >> 29
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 32
	return float64(h%1_000_000) / 1_000_000
}

// roundRectCover is how much of pixel (x, y) lies inside a rounded rectangle (anti-aliased edge).
func roundRectCover(x, y, x0, y0, x1, y1, r float64) float64 {
	cx := math.Max(x0+r, math.Min(x, x1-r))
	cy := math.Max(y0+r, math.Min(y, y1-r))
	d := math.Hypot(x-cx, y-cy) - r // < 0 inside the corner circle
	if x < x0 || x > x1 || y < y0 || y > y1 {
		return 0
	}
	return clamp01(0.5 - d)
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// noise is smooth value noise 0..1.
func noise(x, y float64, seed int64) float64 {
	ix, iy := math.Floor(x), math.Floor(y)
	fx, fy := x-ix, y-iy
	ux, uy := fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	a, b := hash(int(ix), int(iy), seed), hash(int(ix)+1, int(iy), seed)
	c, d := hash(int(ix), int(iy)+1, seed), hash(int(ix)+1, int(iy)+1, seed)
	return a + (b-a)*ux + (c-a)*uy + (a-b-c+d)*ux*uy
}

// crinkle is ridged noise over a few octaves: the creases of beaten leaf (0 flat .. 1 a sharp ridge).
func crinkle(x, y float64, seed int64) float64 {
	v, amp, sum := 0.0, 1.0, 0.0
	for o := 0; o < 4; o++ {
		n := 1 - math.Abs(2*noise(x, y, seed+int64(o)*31)-1)
		v += n * n * amp
		sum += amp
		x, y, amp = x*2.03, y*2.03, amp*0.5
	}
	return v / sum
}

// sheet finds the sheet of leaf a point lies on (jittered cells, the nearest centre wins) and how close it is to the seam
// with the next sheet (in pixels).
func sheet(x, y, size float64, seed int64) (id [2]int, seam float64) {
	cx, cy := int(math.Floor(x/size)), int(math.Floor(y/size))
	best, second := math.MaxFloat64, math.MaxFloat64
	for j := -1; j <= 1; j++ {
		for i := -1; i <= 1; i++ {
			gx, gy := cx+i, cy+j
			px := (float64(gx) + 0.15 + 0.7*hash(gx, gy, seed)) * size
			py := (float64(gy) + 0.15 + 0.7*hash(gx, gy, seed+5)) * size
			d := math.Hypot(x-px, y-py)
			if d < best {
				second, best, id = best, d, [2]int{gx, gy}
			} else if d < second {
				second = d
			}
		}
	}
	return id, (second - best) / 2
}

func paint(l leaf) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, W, H))
	rng := rand.New(rand.NewSource(l.seed))
	grain := make([]float64, W*H)
	for i := range grain {
		grain[i] = rng.Float64()
	}
	wx0, wy0 := window[0]*W, window[1]*H
	wx1, wy1 := wx0+window[2]*W, wy0+window[3]*H
	const outerR, innerR, bevel = 34.0, 12.0, 4.0
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			outer := roundRectCover(fx, fy, 0, 0, W, H, outerR)
			inner := roundRectCover(fx, fy, wx0, wy0, wx1, wy1, innerR)
			a := outer * (1 - inner)
			if a <= 0 {
				continue
			}
			// Sheets of leaf laid overlapping, their edges wavy (the point is moved by noise first).
			wx := fx + (noise(fx/23, fy/23, l.seed+3)-0.5)*14
			wy := fy + (noise(fx/23, fy/23, l.seed+4)-0.5)*14
			id, seam := sheet(wx, wy, 64, l.seed)
			tone := 0.9 + 0.16*hash(id[0], id[1], l.seed+9)
			// The creases of the beaten leaf catch the light: brighter ridges, darker folds.
			c := crinkle(fx/38+float64(id[0])*3.7, fy/38+float64(id[1])*5.1, l.seed)
			tone *= 0.78 + 0.42*c
			// A seam where one sheet lies over the next: a thin dark line with a light lip on one side.
			if seam < 1.6 {
				tone *= 0.72 + 0.17*seam
			} else if seam < 3.2 {
				tone *= 1.06
			}
			// Fine grain and a gentle light from the top left.
			tone *= 0.96 + 0.08*grain[y*W+x]
			tone *= 1.05 - 0.1*(fx/W+fy/H)/2
			// Bevels: a light line along the window's edge, a shadow along the card's outer edge.
			if d := insideDistance(fx, fy, wx0, wy0, wx1, wy1); d < bevel {
				tone *= 1.22 - 0.06*d
			}
			if d := insideDistance(fx, fy, 0, 0, W, H); d < bevel {
				tone *= 0.72 + 0.07*d
			}
			// Bright parts of real gold go paler and warmer, dark parts deeper: a little curve on top of the base colour.
			lift := clamp01((tone - 1) * 1.5)
			px := func(v float64) uint8 { return uint8(255 * clamp01(v*tone+(1-v)*lift*0.35)) }
			img.SetNRGBA(x, y, color.NRGBA{R: px(l.base[0]), G: px(l.base[1]), B: px(l.base[2]), A: uint8(255 * clamp01(a))})
		}
	}
	return img
}

// insideDistance is how far a point outside the rectangle is from it (0 inside), or, for points inside, a large number:
// used for bevels next to a rectangle's edge.
func insideDistance(x, y, x0, y0, x1, y1 float64) float64 {
	if x >= x0 && x <= x1 && y >= y0 && y <= y1 {
		// inside: distance to the nearest edge (the outer rim's bevel is measured from inside the card)
		return math.Min(math.Min(x-x0, x1-x), math.Min(y-y0, y1-y))
	}
	dx := math.Max(x0-x, math.Max(0, x-x1))
	dy := math.Max(y0-y, math.Max(0, y-y1))
	return math.Hypot(dx, dy)
}

// icon is the picture at 120 px wide over a plain card colour, for lists.
func icon(src *image.NRGBA) *image.NRGBA {
	const w = 120
	h := w * H / W
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, image.Rect(int(window[0]*w), int(window[1]*float64(h)), int((window[0]+window[2])*w), int((window[1]+window[3])*float64(h))),
		&image.Uniform{color.NRGBA{91, 106, 138, 255}}, image.Point{}, draw.Src)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// box filter over the source pixels this icon pixel covers
			var r, g, b, a float64
			n := 0.0
			for sy := y * H / h; sy < (y+1)*H/h; sy++ {
				for sx := x * W / w; sx < (x+1)*W/w; sx++ {
					c := src.NRGBAAt(sx, sy)
					fa := float64(c.A) / 255
					r += float64(c.R) * fa
					g += float64(c.G) * fa
					b += float64(c.B) * fa
					a += fa
					n++
				}
			}
			if a <= 0 {
				continue
			}
			alpha := a / n
			bg := out.NRGBAAt(x, y)
			mix := func(v float64, under uint8) uint8 {
				return uint8(math.Round(v/a*alpha + float64(under)*(1-alpha)))
			}
			out.SetNRGBA(x, y, color.NRGBA{mix(r, bg.R), mix(g, bg.G), mix(b, bg.B), uint8(math.Max(float64(bg.A), alpha*255))})
		}
	}
	return out
}

package figurine

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"sort"

	xdraw "golang.org/x/image/draw"
)

// The game draws a figurine with one material, so every material of the model goes into one texture:
//   - one textured material and nothing else → its image as is (UVs may tile);
//   - otherwise an atlas: each textured material gets a rectangle (UVs clamped to 0..1, with a warning when they tile),
//     untextured materials and vertex colours become 8-px colour swatches (one colour per triangle).

const (
	swatchCell = 8
	padding    = 4
	maxSingle  = 2048
)

type region struct {
	mat  int
	img  image.Image
	w, h int // packed size
	x, y int
}

// Combine merges all parts into one mesh with one texture.
func Combine(s *Scene) (*Mesh, *image.NRGBA, error) {
	out := &Mesh{}
	tris := 0
	for _, p := range s.Parts {
		tris += len(p.Idx) / 3
	}
	if tris == 0 {
		return nil, nil, fmt.Errorf("the model has no triangles")
	}
	if tris > MaxTriangles {
		return nil, nil, fmt.Errorf("the model has %d triangles; the limit is %d — reduce it first (Blender: Decimate modifier)", tris, MaxTriangles)
	}

	textured := func(p Part) bool { return p.Mat >= 0 && s.Materials[p.Mat].Image != nil && p.UV != nil }

	// Which materials need an image region, and which parts need swatches.
	imgMats := map[int]bool{}
	needSwatch := false
	for _, p := range s.Parts {
		if textured(p) {
			imgMats[p.Mat] = true
		} else {
			needSwatch = true
		}
	}

	// Simple case: a single textured material covering everything.
	if len(imgMats) == 1 && !needSwatch {
		var mi int
		for k := range imgMats {
			mi = k
		}
		m := s.Materials[mi]
		img := tint(fit(m.Image, maxSingle), m.Factor)
		for _, p := range s.Parts {
			appendPart(out, p, func(i int) [2]float64 { return p.UV[i] })
		}
		return out, img, nil
	}

	// Swatch colours: per triangle (vertex colours averaged, times the material colour), deduplicated.
	type swKey [4]uint8
	swatches := map[swKey]int{}
	var swList []color.NRGBA
	triColor := make([][]int, len(s.Parts)) // per part: swatch index per triangle
	quant := 0                              // bits dropped per channel when there are too many colours
	for {
		swatches = map[swKey]int{}
		swList = swList[:0]
		for pi, p := range s.Parts {
			if textured(p) {
				continue
			}
			f := [4]float64{1, 1, 1, 1}
			if p.Mat >= 0 {
				f = s.Materials[p.Mat].Factor
				if s.Materials[p.Mat].Image != nil && p.UV == nil {
					s.warn(fmt.Sprintf("material %q has a texture but the mesh has no UVs; drawn in its plain colour", s.Materials[p.Mat].Name))
				}
			}
			tc := make([]int, len(p.Idx)/3)
			for t := range tc {
				c := f
				if p.Color != nil {
					var avg [4]float64
					for _, v := range p.Idx[t*3 : t*3+3] {
						for q := 0; q < 4; q++ {
							avg[q] += p.Color[v][q] / 3
						}
					}
					for q := 0; q < 4; q++ {
						c[q] *= avg[q]
					}
				}
				n := toNRGBA(c)
				k := swKey{n.R >> quant << quant, n.G >> quant << quant, n.B >> quant << quant, 255}
				i, ok := swatches[k]
				if !ok {
					i = len(swList)
					swatches[k] = i
					swList = append(swList, color.NRGBA{k[0], k[1], k[2], 255})
				}
				tc[t] = i
			}
			triColor[pi] = tc
		}
		if len(swList) <= 4096 || quant >= 4 {
			break
		}
		quant++
	}

	// Regions for textured materials, largest first.
	var regions []*region
	for mi := range imgMats {
		regions = append(regions, &region{mat: mi, img: s.Materials[mi].Image})
	}
	sort.Slice(regions, func(i, j int) bool {
		a, b := regions[i].img.Bounds(), regions[j].img.Bounds()
		return a.Dx()*a.Dy() > b.Dx()*b.Dy() || (a.Dx()*a.Dy() == b.Dx()*b.Dy() && regions[i].mat < regions[j].mat)
	})
	swCols := 0
	if len(swList) > 0 {
		swCols = int(math.Ceil(math.Sqrt(float64(len(swList)))))
	}
	swSize := swCols * swatchCell

	// Atlas size from the natural area (each image capped at 1024).
	area := float64(swSize * swSize)
	for _, r := range regions {
		b := r.img.Bounds()
		w, h := capSize(b.Dx(), b.Dy(), 1024)
		area += float64((w + 2*padding) * (h + 2*padding))
	}
	S := 1024
	if area > 0.6*1024*1024 {
		S = 2048
	}
	var swX, swY int
	for scale := 1.0; ; scale *= 0.9 {
		for _, r := range regions {
			b := r.img.Bounds()
			w, h := capSize(b.Dx(), b.Dy(), 1024)
			r.w, r.h = max(4, int(float64(w)*scale)), max(4, int(float64(h)*scale))
		}
		ok := true
		swX, swY, ok = pack(regions, swSize, S)
		if ok {
			break
		}
		if scale < 0.05 {
			return nil, nil, fmt.Errorf("too many textures to fit one atlas")
		}
	}

	atlas := image.NewNRGBA(image.Rect(0, 0, S, S))
	draw.Draw(atlas, atlas.Bounds(), &image.Uniform{color.NRGBA{255, 255, 255, 255}}, image.Point{}, draw.Src)
	byMat := map[int]*region{}
	for _, r := range regions {
		byMat[r.mat] = r
		m := s.Materials[r.mat]
		tile := image.NewNRGBA(image.Rect(0, 0, r.w, r.h))
		xdraw.CatmullRom.Scale(tile, tile.Bounds(), m.Image, m.Image.Bounds(), draw.Src, nil)
		tile = tint(tile, m.Factor)
		// Paste with an extended edge (padding) against bleeding between regions.
		for y := -padding; y < r.h+padding; y++ {
			for x := -padding; x < r.w+padding; x++ {
				sx, sy := clampInt(x, 0, r.w-1), clampInt(y, 0, r.h-1)
				atlas.SetNRGBA(r.x+x, r.y+y, tile.NRGBAAt(sx, sy))
			}
		}
	}
	for i, c := range swList {
		x0, y0 := swX+(i%max(1, swCols))*swatchCell, swY+(i/max(1, swCols))*swatchCell
		draw.Draw(atlas, image.Rect(x0, y0, x0+swatchCell, y0+swatchCell), &image.Uniform{c}, image.Point{}, draw.Src)
	}

	tiled := false
	for pi, p := range s.Parts {
		if textured(p) {
			r := byMat[p.Mat]
			appendPart(out, p, func(i int) [2]float64 {
				u, v := p.UV[i][0], p.UV[i][1]
				if u < -0.01 || u > 1.01 || v < -0.01 || v > 1.01 {
					tiled = true
				}
				u, v = math.Max(0, math.Min(1, u)), math.Max(0, math.Min(1, v))
				return [2]float64{(float64(r.x) + u*float64(r.w)) / float64(S), (float64(r.y) + v*float64(r.h)) / float64(S)}
			})
			continue
		}
		// Swatch parts: one colour per triangle → unshared vertices.
		tc := triColor[pi]
		flat := Part{Mat: p.Mat}
		for t := 0; t < len(p.Idx)/3; t++ {
			i := tc[t]
			cx := float64(swX+(i%max(1, swCols))*swatchCell) + swatchCell/2.0
			cy := float64(swY+(i/max(1, swCols))*swatchCell) + swatchCell/2.0
			uv := [2]float64{cx / float64(S), cy / float64(S)}
			for _, v := range p.Idx[t*3 : t*3+3] {
				flat.Idx = append(flat.Idx, uint32(len(flat.Pos)))
				flat.Pos = append(flat.Pos, p.Pos[v])
				flat.Nrm = append(flat.Nrm, p.Nrm[v])
				flat.UV = append(flat.UV, uv)
			}
		}
		appendPart(out, flat, func(i int) [2]float64 { return flat.UV[i] })
	}
	if tiled {
		s.warn("some textures repeat (UVs outside 0–1) and were clamped, so they may look stretched — bake the model to one texture (Blender: Bake → Diffuse) for an exact result")
	}
	if quant > 0 {
		s.warn(fmt.Sprintf("the model has many vertex colours; they were reduced to %d", len(swList)))
	}
	return out, atlas, nil
}

func appendPart(out *Mesh, p Part, uv func(int) [2]float64) {
	base := uint32(len(out.Pos))
	for i := range p.Pos {
		out.Pos = append(out.Pos, p.Pos[i])
		out.Nrm = append(out.Nrm, p.Nrm[i])
		out.UV = append(out.UV, uv(i))
	}
	for _, v := range p.Idx {
		out.Idx = append(out.Idx, base+v)
	}
}

// pack places regions with a shelf packer inside an S×S square, the swatch block first. Returns the swatch position.
func pack(regions []*region, swSize, S int) (swX, swY int, ok bool) {
	x, y, rowH := padding, padding, 0
	place := func(w, h int) (int, int, bool) {
		if x+w+padding > S {
			x, y, rowH = padding, y+rowH+2*padding, 0
		}
		if y+h+padding > S || w+2*padding > S {
			return 0, 0, false
		}
		px, py := x, y
		x += w + 2*padding
		rowH = max(rowH, h)
		return px, py, true
	}
	if swSize > 0 {
		if swX, swY, ok = place(swSize, swSize); !ok {
			return
		}
	}
	for _, r := range regions {
		if r.x, r.y, ok = place(r.w, r.h); !ok {
			return
		}
	}
	return swX, swY, true
}

func capSize(w, h, limit int) (int, int) {
	if w <= limit && h <= limit {
		return w, h
	}
	f := float64(limit) / float64(max(w, h))
	return max(1, int(float64(w)*f)), max(1, int(float64(h)*f))
}

// fit returns the image as NRGBA, scaled down to at most limit pixels on its long side.
func fit(img image.Image, limit int) *image.NRGBA {
	b := img.Bounds()
	w, h := capSize(b.Dx(), b.Dy(), limit)
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	if w == b.Dx() && h == b.Dy() {
		draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	} else {
		xdraw.CatmullRom.Scale(out, out.Bounds(), img, b, draw.Src, nil)
	}
	return out
}

// tint multiplies an image by a colour factor (skipped when the factor is white).
func tint(img *image.NRGBA, f [4]float64) *image.NRGBA {
	if f[0] >= 0.999 && f[1] >= 0.999 && f[2] >= 0.999 {
		return img
	}
	for i := 0; i+3 < len(img.Pix); i += 4 {
		for q := 0; q < 3; q++ {
			img.Pix[i+q] = uint8(math.Round(float64(img.Pix[i+q]) * math.Max(0, math.Min(1, f[q]))))
		}
	}
	return img
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

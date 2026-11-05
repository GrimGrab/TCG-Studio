// migration:art-layouts-1.03
// Package relayout moves art made for one texture layout of a model onto another (the game's 2026-10-27 update gave packs,
// boxes and accessories new models with new UV layouts). Both layouts name the same faces (front, top, edges…): a texture is
// redrawn face by face, and a design's texture layers are moved piece by piece (Layers), so nothing needs the old meshes.
package relayout

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/draw"

	"tcgstudio/internal/uvmap"
)

// Image redraws src (art in layout from, any resolution) in layout to: every texel of a face of to comes from the same
// place of the same face in from; the rest is background (the base's print in the new layout; nil = transparent).
func Image(from, to uvmap.Model, src image.Image, background image.Image) *image.NRGBA {
	S := to.TextureSize
	out := image.NewNRGBA(image.Rect(0, 0, S, S))
	if background != nil {
		if b := background.Bounds(); b.Dx() == S && b.Dy() == S {
			draw.Draw(out, out.Bounds(), background, b.Min, draw.Src) // the usual case: same size, a plain copy
		} else {
			draw.ApproxBiLinear.Scale(out, out.Bounds(), background, b, draw.Src, nil)
		}
	}
	pix := nrgba(src)                                           // read pixels straight from memory, not through At()
	k := float64(src.Bounds().Dx()) / float64(from.TextureSize) // src pixels per layout pixel
	old := map[string]uvmap.Face{}
	for _, f := range from.Faces {
		old[f.ID] = f
	}
	for _, f := range to.Faces {
		of, ok := old[f.ID]
		if !ok {
			of, ok = old[f.ArtFrom] // a face split off since: the print it shared with this one
		}
		if !ok {
			continue
		}
		for _, t := range f.Targets { // bleed first (the order targets come in), the exact areas over them
			x0, y0, x1, y1 := int(math.Floor(t.Rect[0])), int(math.Floor(t.Rect[1])), int(math.Ceil(t.Rect[2])), int(math.Ceil(t.Rect[3]))
			for y := max(0, y0); y < min(S, y1); y++ {
				for x := max(0, x0); x < min(S, x1); x++ {
					if !t.Paints(float64(x)+0.5, float64(y)+0.5) {
						continue
					}
					s, r := faceAt(t, f, float64(x)+0.5, float64(y)+0.5)
					if px, py, ok := textureAt(of, s, r); ok {
						out.SetNRGBA(x, y, bilinear(pix, px*k, py*k))
					}
				}
			}
		}
	}
	return out
}

// faceAt: where texture pixel (x, y) of target t is on its face, as fractions of the face's net (0..1, top-left origin).
func faceAt(t uvmap.Target, f uvmap.Face, x, y float64) (float64, float64) {
	a := (x - t.Rect[0]) / (t.Rect[2] - t.Rect[0])
	b := (y - t.Rect[1]) / (t.Rect[3] - t.Rect[1])
	if t.FlipX {
		a = 1 - a
	}
	if t.FlipY {
		b = 1 - b
	}
	if t.Transpose {
		a, b = b, a
	}
	return t.Src[0] + a*(t.Src[2]-t.Src[0]), t.Src[1] + b*(t.Src[3]-t.Src[1])
}

// textureAt: the texture pixel showing face point (s, r) in layout face f — from its exact (non-bleed) target whose part
// of the face holds the point, else the nearest such target (clamped: a new face reaching further than the old one's art).
func textureAt(f uvmap.Face, s, r float64) (float64, float64, bool) {
	var best *uvmap.Target
	bestD := math.Inf(1)
	for i := range f.Targets {
		t := &f.Targets[i]
		if t.Bleed {
			continue
		}
		dx := math.Max(0, math.Max(t.Src[0]-s, s-t.Src[2]))
		dy := math.Max(0, math.Max(t.Src[1]-r, r-t.Src[3]))
		if d := dx*dx + dy*dy; d < bestD {
			best, bestD = t, d
		}
	}
	if best == nil {
		return 0, 0, false
	}
	t := *best
	a := clamp01((s - t.Src[0]) / (t.Src[2] - t.Src[0]))
	b := clamp01((r - t.Src[1]) / (t.Src[3] - t.Src[1]))
	if t.Transpose {
		a, b = b, a
	}
	if t.FlipX {
		a = 1 - a
	}
	if t.FlipY {
		b = 1 - b
	}
	return t.Rect[0] + a*(t.Rect[2]-t.Rect[0]), t.Rect[1] + b*(t.Rect[3]-t.Rect[1]), true
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

// nrgba is img as *image.NRGBA with its origin at 0,0 (a copy unless it already is one).
func nrgba(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	b := img.Bounds()
	n := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(n, n.Bounds(), img, b.Min, draw.Src)
	return n
}

// bilinear samples src at pixel position (x, y) (pixel centres at +0.5), clamped to its edges.
func bilinear(src *image.NRGBA, x, y float64) color.NRGBA {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	x, y = x-0.5, y-0.5
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(x0), y-float64(y0)
	at := func(px, py int) [4]float64 {
		px, py = min(max(px, 0), w-1), min(max(py, 0), h-1)
		p := src.Pix[py*src.Stride+px*4:]
		return [4]float64{float64(p[0]), float64(p[1]), float64(p[2]), float64(p[3])}
	}
	c00, c10, c01, c11 := at(x0, y0), at(x0+1, y0), at(x0, y0+1), at(x0+1, y0+1)
	var o [4]uint8
	for i := 0; i < 4; i++ {
		top := c00[i]*(1-fx) + c10[i]*fx
		bot := c01[i]*(1-fx) + c11[i]*fx
		o[i] = uint8(math.Round(top*(1-fy) + bot*fy))
	}
	return color.NRGBA{o[0], o[1], o[2], o[3]}
}

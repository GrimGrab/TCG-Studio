package main

// The stock borders besides the leaves: each a picture made pixel by pixel over the same frame (frameAlpha: rounded outer
// corners, the window cut out), and the stock shader that suits it.

import (
	"image"
	"image/color"
	"math"
)

var styles = []stock{
	{"tcgcc-border-rainbow-holo", "Rainbow holo", "rainbow_holo", paintHolo, "rainbowrim"},
	{"tcgcc-border-obsidian", "Obsidian", "obsidian", paintObsidian, "gloss"},
	{"tcgcc-border-etched-silver", "Etched silver", "etched_silver", paintEtched, "gloss"},
	{"tcgcc-border-pastel", "Pastel", "pastel", paintPastel, ""},
}

// frame is what every style paints on: for each pixel its place, how solid the frame is there (0 = see-through: the
// window or outside the corners), and how far it is from the window's edge and from the card's edge (pixels).
type frame struct {
	x, y, alpha     float64
	toWindow, toRim float64
}

func eachFrame(paint func(f frame) [3]float64) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, W, H))
	wx0, wy0 := window[0]*W, window[1]*H
	wx1, wy1 := wx0+window[2]*W, wy0+window[3]*H
	const outerR, innerR = 34.0, 12.0
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			a := roundRectCover(fx, fy, 0, 0, W, H, outerR) * (1 - roundRectCover(fx, fy, wx0, wy0, wx1, wy1, innerR))
			if a <= 0 {
				continue
			}
			f := frame{x: fx, y: fy, alpha: a, toWindow: insideDistance(fx, fy, wx0, wy0, wx1, wy1), toRim: insideDistance(fx, fy, 0, 0, W, H)}
			c := paint(f)
			img.SetNRGBA(x, y, color.NRGBA{R: unit(c[0]), G: unit(c[1]), B: unit(c[2]), A: unit(a)})
		}
	}
	return img
}

func unit(v float64) uint8 { return uint8(math.Round(255 * clamp01(v))) }

func scale(c [3]float64, k float64) [3]float64 { return [3]float64{c[0] * k, c[1] * k, c[2] * k} }

func lerp(a, b [3]float64, t float64) [3]float64 {
	return [3]float64{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t, a[2] + (b[2]-a[2])*t}
}

// bevel: a light line along the window and a shadow along the card's edge, so the rim looks raised.
func bevel(f frame, c [3]float64, light, shadow float64) [3]float64 {
	if f.toWindow < 4 {
		c = lerp(c, [3]float64{1, 1, 1}, light*(1-f.toWindow/4))
	}
	if f.toRim < 4 {
		c = scale(c, 1-shadow*(1-f.toRim/4))
	}
	return c
}

// Rainbow holo: pale brushed silver with fine diagonal diffraction lines; the shader lays the moving rainbow over it.
func paintHolo() *image.NRGBA {
	base := [3]float64{0.84, 0.86, 0.9}
	return eachFrame(func(f frame) [3]float64 {
		brush := 0.94 + 0.06*noise(f.x/90, f.y/1.6, 21) // long horizontal strokes
		lines := 0.97 + 0.03*math.Sin((f.x+f.y)*1.4)
		return bevel(f, scale(base, brush*lines), 0.5, 0.35)
	})
}

// Obsidian: black volcanic glass, rippled in shells around a few points (conchoidal fracture), lighter where it is thin
// at the edges.
func paintObsidian() *image.NRGBA {
	base := [3]float64{0.055, 0.05, 0.07}
	type shell struct{ x, y float64 }
	var shells []shell
	for i := 0; i < 14; i++ {
		shells = append(shells, shell{hash(i, 1, 33) * W, hash(i, 2, 33) * H})
	}
	return eachFrame(func(f frame) [3]float64 {
		// The nearest centre's rings.
		best := math.MaxFloat64
		for _, s := range shells {
			best = math.Min(best, math.Hypot(f.x-s.x, f.y-s.y))
		}
		ripple := 0.5 + 0.5*math.Sin(best/7+noise(f.x/40, f.y/40, 34)*4)
		c := lerp(base, [3]float64{0.2, 0.18, 0.26}, ripple*ripple*0.85)
		// A faint purple sheen, as obsidian has.
		c = lerp(c, [3]float64{0.22, 0.12, 0.3}, 0.25*noise(f.x/120, f.y/120, 35))
		return bevel(f, c, 0.35, 0.5)
	})
}

// Etched silver: polished silver engraved with a wavy guilloche pattern, with a pinstripe inside the card's edge and one
// around the window.
func paintEtched() *image.NRGBA {
	base := [3]float64{0.78, 0.8, 0.84}
	return eachFrame(func(f frame) [3]float64 {
		c := scale(base, 0.95+0.05*noise(f.x/30, f.y/30, 41))
		// Two crossing families of wavy lines, cut a little darker.
		u := f.y/6 + 2.2*math.Sin(f.x/11)
		v := f.x/6 + 2.2*math.Sin(f.y/11)
		cut := math.Max(line(u), line(v))
		c = scale(c, 1-0.32*cut)
		// Pinstripes: a darker groove with a light lip.
		for _, d := range []float64{f.toRim - 11, f.toWindow - 9} {
			if math.Abs(d) < 1.2 {
				c = scale(c, 0.55)
			} else if d >= 1.2 && d < 2.4 {
				c = lerp(c, [3]float64{1, 1, 1}, 0.35)
			}
		}
		return bevel(f, c, 0.55, 0.4)
	})
}

// line is 1 on a thin engraved line of a repeating pattern, 0 between (anti-aliased).
func line(t float64) float64 {
	d := math.Abs(t - math.Round(t)) // 0 on the line
	return clamp01(1 - d*6)
}

// Pastel: a soft gradient from pink through lavender to mint, a paper grain and a white pinstripe around the window.
func paintPastel() *image.NRGBA {
	pink, lavender, mint := [3]float64{0.98, 0.8, 0.87}, [3]float64{0.82, 0.8, 0.98}, [3]float64{0.78, 0.95, 0.89}
	return eachFrame(func(f frame) [3]float64 {
		t := clamp01((f.x/W + f.y/H) / 2)
		var c [3]float64
		if t < 0.5 {
			c = lerp(pink, lavender, t*2)
		} else {
			c = lerp(lavender, mint, (t-0.5)*2)
		}
		c = scale(c, 0.97+0.05*noise(f.x/3, f.y/3, 51))
		if d := f.toWindow - 8; math.Abs(d) < 1.5 {
			c = lerp(c, [3]float64{1, 1, 1}, 0.85)
		}
		return bevel(f, c, 0.3, 0.12)
	})
}

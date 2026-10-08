package figurine

import (
	"image"
	"math"

	xdraw "golang.org/x/image/draw"
)

// Surface detail for the game: the mod draws a model with one colour texture (no normal map), so a model's normal or bump map is
// baked into that texture as light and shade (a fixed light from the upper left, flat areas unchanged). Without it, models whose
// detail lives only in the normal map (scanned / sculpted ones, e.g. a plain dark metal colour plus a normal map) come out as a
// featureless blob.

// light direction in tangent space (x right, y up the texture, z out of the surface), normalised.
var detailLight = func() [3]float64 {
	l := [3]float64{-0.4, 0.5, 0.77}
	n := math.Sqrt(l[0]*l[0] + l[1]*l[1] + l[2]*l[2])
	return [3]float64{l[0] / n, l[1] / n, l[2] / n}
}()

// maxDetail is the longest side of a baked detail texture.
const maxDetail = 2048

// shadeFromNormal turns a tangent-space normal map (RGB = xyz, glTF/OpenGL convention: green up) into a grey shade map: 1 = as lit
// as a flat surface. strength scales the bumps (glTF normalTexture.scale). Grey images (bump / height maps) are converted to normals
// from their slopes first.
func shadeFromNormal(src image.Image, strength float64) *image.NRGBA {
	img := toNRGBAImage(fitImage(src, maxDetail))
	if strength <= 0 {
		strength = 1
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	height := isGrey(img)
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	at := func(x, y int) float64 { // height 0–1 (grey maps)
		x, y = (x%w+w)%w, (y%h+h)%h
		return float64(img.Pix[y*img.Stride+x*4]) / 255
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var n [3]float64
			if height {
				// Slopes of the height map; image y runs down, tangent y up.
				k := 4 * strength
				n = [3]float64{-(at(x+1, y) - at(x-1, y)) * k, (at(x, y+1) - at(x, y-1)) * k, 1}
			} else {
				p := img.Pix[y*img.Stride+x*4:]
				n = [3]float64{(float64(p[0])/127.5 - 1) * strength, (float64(p[1])/127.5 - 1) * strength, float64(p[2])/127.5 - 1}
				if n[2] < 0.05 {
					n[2] = 0.05
				}
			}
			l := math.Sqrt(n[0]*n[0] + n[1]*n[1] + n[2]*n[2])
			d := (n[0]*detailLight[0] + n[1]*detailLight[1] + n[2]*detailLight[2]) / l
			s := math.Max(0.35, math.Min(1.35, d/detailLight[2])) // flat = 1
			v := uint8(math.Round(math.Min(255, s*188)))          // 188 = 1.0; brighter areas up to 1.35
			i := y*out.Stride + x*4
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = v, v, v, 255
		}
	}
	return out
}

// shadeScale undoes shadeFromNormal's storage (188 = 1.0).
const shadeScale = 188.0

// withDetail multiplies a colour texture (nil = plain white, the material colour comes later) by a shade map, at the larger size.
func withDetail(base image.Image, shade *image.NRGBA) *image.NRGBA {
	sb := shade.Bounds()
	if base == nil {
		out := image.NewNRGBA(sb)
		for i := 0; i+3 < len(shade.Pix); i += 4 {
			v := uint8(math.Min(255, float64(shade.Pix[i])/shadeScale*255))
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = v, v, v, 255
		}
		return out
	}
	bb := base.Bounds()
	w, h := bb.Dx(), bb.Dy()
	if sb.Dx() > w || sb.Dy() > h {
		w, h = max(w, sb.Dx()), max(h, sb.Dy())
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	xdraw.CatmullRom.Scale(out, out.Bounds(), base, bb, xdraw.Src, nil)
	sh := shade
	if sb.Dx() != w || sb.Dy() != h {
		sh = image.NewNRGBA(out.Bounds())
		xdraw.BiLinear.Scale(sh, sh.Bounds(), shade, sb, xdraw.Src, nil)
	}
	for i := 0; i+3 < len(out.Pix); i += 4 {
		s := float64(sh.Pix[i]) / shadeScale
		for q := 0; q < 3; q++ {
			out.Pix[i+q] = uint8(math.Round(math.Min(255, float64(out.Pix[i+q])*s)))
		}
	}
	return out
}

func isGrey(img *image.NRGBA) bool {
	step := max(1, len(img.Pix)/4/4096) * 4
	for i := 0; i+3 < len(img.Pix); i += step {
		p := img.Pix[i : i+3]
		if absDiff(p[0], p[1]) > 6 || absDiff(p[1], p[2]) > 6 {
			return false
		}
	}
	return true
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

func fitImage(img image.Image, maxSide int) image.Image {
	b := img.Bounds()
	if b.Dx() <= maxSide && b.Dy() <= maxSide {
		return img
	}
	s := float64(maxSide) / math.Max(float64(b.Dx()), float64(b.Dy()))
	out := image.NewNRGBA(image.Rect(0, 0, max(1, int(float64(b.Dx())*s)), max(1, int(float64(b.Dy())*s))))
	xdraw.BiLinear.Scale(out, out.Bounds(), img, b, xdraw.Src, nil)
	return out
}

func toNRGBAImage(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n
	}
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	xdraw.Draw(out, out.Bounds(), img, b.Min, xdraw.Src)
	return out
}

// linearToSRGB converts a linear colour channel (glTF factors and vertex colours) to the sRGB value a texture pixel holds.
func linearToSRGB(v float64) float64 {
	v = math.Max(0, math.Min(1, v))
	if v <= 0.0031308 {
		return v * 12.92
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

func srgbColor(c [4]float64) [4]float64 {
	return [4]float64{linearToSRGB(c[0]), linearToSRGB(c[1]), linearToSRGB(c[2]), c[3]}
}

// flipGreen turns a DirectX normal map (green down) into the OpenGL convention shadeFromNormal expects.
func flipGreen(src image.Image) image.Image {
	img := toNRGBAImage(fitImage(src, maxDetail))
	out := image.NewNRGBA(img.Rect)
	copy(out.Pix, img.Pix)
	for i := 1; i < len(out.Pix); i += 4 {
		out.Pix[i] = 255 - out.Pix[i]
	}
	return out
}

// aoShade stores an ambient-occlusion map (grey, 1 = open) as a shade map for withDetail.
func aoShade(src image.Image) *image.NRGBA {
	img := toNRGBAImage(fitImage(src, maxDetail))
	out := image.NewNRGBA(img.Rect)
	for i := 0; i+3 < len(img.Pix); i += 4 {
		v := uint8(math.Round(float64(img.Pix[i]) / 255 * shadeScale))
		out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = v, v, v, 255
	}
	return out
}

// multiplyImages multiplies a texture by a grey shading image (as withDetail stores it, white = 1), at the larger size.
func multiplyImages(base, grey image.Image) *image.NRGBA {
	g := toNRGBAImage(grey)
	shade := image.NewNRGBA(g.Rect)
	for i := 0; i+3 < len(g.Pix); i += 4 {
		v := uint8(math.Round(float64(g.Pix[i]) / 255 * shadeScale))
		shade.Pix[i], shade.Pix[i+1], shade.Pix[i+2], shade.Pix[i+3] = v, v, v, 255
	}
	return withDetail(base, shade)
}

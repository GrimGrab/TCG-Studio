package importer

import (
	"image"
	"image/color"
	"image/draw"
)

// JPEGQuality is used when card art is saved as JPEG (about 6x smaller than PNG for card scans).
const JPEGQuality = 90

// Opaque returns img without transparency for JPEG: transparent pixels (e.g. Scryfall's rounded card corners) are blended
// with the card's outer border colour, so the corners show as part of the border instead of black.
func Opaque(img image.Image) image.Image {
	b := img.Bounds()
	if isOpaque(img) {
		return img
	}
	border := borderColour(img)
	out := image.NewRGBA(b)
	draw.Draw(out, b, &image.Uniform{border}, image.Point{}, draw.Src)
	draw.Draw(out, b, img, b.Min, draw.Over)
	return out
}

func isOpaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	return false
}

// borderColour averages the opaque pixels a few pixels in from the middle of each edge (the printed border).
func borderColour(img image.Image) color.RGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	in := max(2, min(w, h)/100)
	pts := []image.Point{
		{b.Min.X + w/2, b.Min.Y + in}, {b.Min.X + w/2, b.Max.Y - 1 - in},
		{b.Min.X + in, b.Min.Y + h/2}, {b.Max.X - 1 - in, b.Min.Y + h/2},
		{b.Min.X + w/4, b.Min.Y + in}, {b.Min.X + 3*w/4, b.Max.Y - 1 - in},
	}
	var r, g, bl, n uint32
	for _, p := range pts {
		cr, cg, cb, ca := img.At(p.X, p.Y).RGBA()
		if ca < 0xf000 {
			continue
		}
		r, g, bl, n = r+cr>>8, g+cg>>8, bl+cb>>8, n+1
	}
	if n == 0 {
		return color.RGBA{0, 0, 0, 255}
	}
	return color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), 255}
}

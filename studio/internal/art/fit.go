package art

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"
)

// printed reports whether a template pixel is part of the printed (coloured) area rather than grey foil/metal.
func printed(c color.NRGBA) bool {
	if c.A < 200 {
		return false
	}
	_, s, l := toHSL(c)
	return s >= 0.2 && l > 0.08 && l < 0.95
}

// detectPrinted returns the bounding box of printed pixels inside search, or fallback if too few are found.
func detectPrinted(img *image.NRGBA, search image.Rectangle, fallback image.Rectangle) image.Rectangle {
	search = search.Intersect(img.Bounds())
	minX, minY, maxX, maxY, n := search.Max.X, search.Max.Y, search.Min.X, search.Min.Y, 0
	for y := search.Min.Y; y < search.Max.Y; y++ {
		row := 0
		for x := search.Min.X; x < search.Max.X; x++ {
			if printed(img.NRGBAAt(x, y)) {
				row++
			}
		}
		// A row counts when it is mostly printed (ignores stray coloured pixels in the crimps).
		if row < search.Dx()/3 {
			continue
		}
		for x := search.Min.X; x < search.Max.X; x++ {
			if printed(img.NRGBAAt(x, y)) {
				minX, maxX = min(minX, x), max(maxX, x)
			}
		}
		minY, maxY = min(minY, y), max(maxY, y)
		n++
	}
	if n < 50 {
		return fallback
	}
	return image.Rect(minX, minY, maxX+1, maxY+1)
}

// faceMask describes the printed face of the rendered pack in the pack icon, measured per row and per column so the
// perspective skew is followed exactly.
type faceMask struct {
	left, right map[int]int // row → x range
	top, bottom map[int]int // column → y range
	box         image.Rectangle
}

func (m faceMask) bounds() image.Rectangle { return m.box }

func (m faceMask) inside(x, y int) bool {
	l, ok := m.left[y]
	if !ok || x < l || x > m.right[y] {
		return false
	}
	t, ok := m.top[x]
	return ok && y >= t && y <= m.bottom[x]
}

func measureFace(icon *image.NRGBA) faceMask {
	b := icon.Bounds()
	m := faceMask{left: map[int]int{}, right: map[int]int{}, top: map[int]int{}, bottom: map[int]int{}}
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		l, r, n := -1, -1, 0
		for x := b.Min.X; x < b.Max.X; x++ {
			if printed(icon.NRGBAAt(x, y)) {
				if l < 0 {
					l = x
				}
				r = x
				n++
			}
		}
		if n > b.Dx()/5 {
			m.left[y], m.right[y] = l, r
		}
	}
	for x := b.Min.X; x < b.Max.X; x++ {
		t, bt, n := -1, -1, 0
		for y := b.Min.Y; y < b.Max.Y; y++ {
			if _, ok := m.left[y]; ok && printed(icon.NRGBAAt(x, y)) {
				if t < 0 {
					t = y
				}
				bt = y
				n++
			}
		}
		if n > b.Dy()/5 {
			m.top[x], m.bottom[x] = t, bt
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, t), max(maxY, bt)
		}
	}
	if maxX > minX && maxY > minY {
		// Include the crimps above/below for the "pack only" crop used in box art.
		m.box = image.Rect(minX-8, max(b.Min.Y, minY-110), maxX+8, min(b.Max.Y, maxY+110)).Intersect(b)
	} else {
		m.box = b
	}
	return m
}

// warpFront maps the flat front panel onto the icon's printed face: u from the row's x-span, v from the column's y-span.
func warpFront(icon *image.NRGBA, panel *image.NRGBA, m faceMask) {
	pb := panel.Bounds()
	for y := m.box.Min.Y; y < m.box.Max.Y; y++ {
		l, ok := m.left[y]
		if !ok {
			continue
		}
		r := m.right[y]
		for x := l; x <= r; x++ {
			if !m.inside(x, y) {
				continue
			}
			c := icon.NRGBAAt(x, y)
			u := float64(x-l) / math.Max(1, float64(r-l))
			v := float64(y-m.top[x]) / math.Max(1, float64(m.bottom[x]-m.top[x]))
			p := panel.NRGBAAt(pb.Min.X+int(u*float64(pb.Dx()-1)), pb.Min.Y+int(v*float64(pb.Dy()-1)))
			k := 0.9 + 0.1*math.Sin(u*math.Pi) // soft edge falloff keeps a little of the 3D look
			icon.SetNRGBA(x, y, color.NRGBA{clamp(float64(p.R) * k), clamp(float64(p.G) * k), clamp(float64(p.B) * k), c.A})
		}
	}
}

// drawCover scales src to cover r (cropping the overflow, centered).
func drawCover(dst *image.NRGBA, r image.Rectangle, src image.Image) {
	sb := src.Bounds()
	scale := math.Max(float64(r.Dx())/float64(sb.Dx()), float64(r.Dy())/float64(sb.Dy()))
	cw, ch := int(float64(r.Dx())/scale), int(float64(r.Dy())/scale)
	crop := image.Rect(sb.Min.X+(sb.Dx()-cw)/2, sb.Min.Y+(sb.Dy()-ch)/2, 0, 0)
	crop.Max = crop.Min.Add(image.Pt(cw, ch))
	xdraw.CatmullRom.Scale(dst, r, src, crop, xdraw.Src, nil)
}

// drawScaled draws src scaled into r with alpha blending.
func drawScaled(dst *image.NRGBA, r image.Rectangle, src image.Image) {
	xdraw.CatmullRom.Scale(dst, r, src, src.Bounds(), xdraw.Over, nil)
}

func roundedInside(x, y int, r image.Rectangle, rad int) bool {
	if !image.Pt(x, y).In(r) {
		return false
	}
	cx, cy := x, y
	switch {
	case x < r.Min.X+rad:
		cx = r.Min.X + rad
	case x >= r.Max.X-rad:
		cx = r.Max.X - rad - 1
	}
	switch {
	case y < r.Min.Y+rad:
		cy = r.Min.Y + rad
	case y >= r.Max.Y-rad:
		cy = r.Max.Y - rad - 1
	}
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= rad*rad
}

func fillRounded(img *image.NRGBA, r image.Rectangle, rad int, c color.NRGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if roundedInside(x, y, r, rad) {
				img.SetNRGBA(x, y, c)
			}
		}
	}
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// ---------------------------------------------------------------- card backs

// Card backs are stored as plain card-shaped art in the proportions of the game's card-back art window (T_CardBackMesh:
// 615×862 at x78–692, y66–927). The mod pastes it into the vanilla back texture (rounded corners, border and card edge
// stay vanilla), so any source size/shape works: it is trimmed of plain borders and cover-cropped to this size.
var backArt = image.Rect(0, 0, 718, 1006)

// ComposeCardBack trims plain borders off src, cover-crops it to the card-back art proportions and writes a PNG.
func ComposeCardBack(src image.Image, out string) error {
	img := image.NewNRGBA(backArt)
	drawCover(img, backArt, trimBorders(src))
	return savePNG(out, img)
}

// GenerateCardBack makes a card back from a colour, the set icon and a title.
func GenerateCardBack(projectFolder, iconRel, colorHex, title, out string) error {
	base, err := parseHex(colorHex)
	if err != nil {
		return err
	}
	art := image.NewNRGBA(backArt)
	gradient(art, art.Bounds(), shade(base, 1.2), shade(base, 0.5))
	drawIcon(art, loadIcon(projectFolder, iconRel), image.Pt(art.Bounds().Dx()/2, art.Bounds().Dy()/2-40), 340, 235)
	if title != "" {
		drawText(art, func(s float64) fontFace { return newFace(s) }, title, image.Rect(40, art.Bounds().Dy()-250, art.Bounds().Dx()-40, art.Bounds().Dy()-70), 70, color.White)
	}
	return savePNG(out, art)
}

// trimBorders crops away uniform (or transparent) margins, e.g. the black/white padding around a scanned card.
func trimBorders(src image.Image) image.Image {
	b := src.Bounds()
	if b.Dx() < 16 || b.Dy() < 16 {
		return src
	}
	at := func(x, y int) color.NRGBA { return color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA) }
	// Each side is compared with the colour in the middle of that edge (corners may be rounded/transparent).
	plain := func(c, ref color.NRGBA) bool {
		if c.A < 16 {
			return true
		}
		d := func(a, b uint8) int { return absInt(int(a) - int(b)) }
		return ref.A >= 16 && d(c.R, ref.R)+d(c.G, ref.G)+d(c.B, ref.B) < 48
	}
	rowPlain := func(y int, ref color.NRGBA) bool {
		n := 0
		for x := b.Min.X; x < b.Max.X; x++ {
			if !plain(at(x, y), ref) {
				n++
			}
		}
		return n <= b.Dx()/50 // tolerate a few stray pixels
	}
	colPlain := func(x, y0, y1 int, ref color.NRGBA) bool {
		n := 0
		for y := y0; y < y1; y++ {
			if !plain(at(x, y), ref) {
				n++
			}
		}
		return n <= (y1-y0)/50
	}
	midX, midY := (b.Min.X+b.Max.X)/2, (b.Min.Y+b.Max.Y)/2
	top, bot := b.Min.Y, b.Max.Y
	for ref := at(midX, top); top < bot-1 && rowPlain(top, ref); {
		top++
	}
	for ref := at(midX, bot-1); bot > top+1 && rowPlain(bot-1, ref); {
		bot--
	}
	left, right := b.Min.X, b.Max.X
	for ref := at(left, midY); left < right-1 && colPlain(left, top, bot, ref); {
		left++
	}
	for ref := at(right-1, midY); right > left+1 && colPlain(right-1, top, bot, ref); {
		right--
	}
	r := image.Rect(left, top, right, bot)
	if r.Dx()*r.Dy() < b.Dx()*b.Dy()/2 { // looks like the image itself is mostly plain; leave it alone
		return src
	}
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	xdraw.Copy(out, image.Point{}, src, r, xdraw.Src, nil)
	return out
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// CardBackFromFile composes an image file into a card back.
func CardBackFromFile(srcPath, out string) error {
	src, err := loadImage(srcPath)
	if err != nil {
		return fmt.Errorf("card back image: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return ComposeCardBack(src, out)
}

// PackIconFromTexture renders a pack shop icon from a finished pack texture: the vanilla pack icon with the texture's front
// panel (front, pixels in the 1024² texture) warped onto its printed face, and its other saturated parts turned to the
// panel's dominant hue.
func PackIconFromTexture(templatesDir string, texture image.Image, front image.Rectangle) (*image.NRGBA, error) {
	f, err := os.Open(filepath.Join(templatesDir, "BasicCardPack_icon.png"))
	if err != nil {
		return nil, fmt.Errorf("template BasicCardPack_icon.png not found — the game templates aren't available yet (Settings → Game)")
	}
	defer f.Close()
	src, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	icon := image.NewNRGBA(src.Bounds())
	xdraw.Draw(icon, icon.Bounds(), src, src.Bounds().Min, xdraw.Src)
	tex := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	xdraw.CatmullRom.Scale(tex, tex.Bounds(), texture, texture.Bounds(), xdraw.Src, nil)
	panel := tex.SubImage(front.Intersect(tex.Bounds())).(*image.NRGBA)
	m := measureFace(icon)
	if hue, ok := dominantHue(panel); ok {
		recolor(icon, icon.Bounds(), hue)
	}
	warpFront(icon, panel, m)
	return icon, nil
}

// dominantHue is the saturation-weighted mean hue of an image (false when it is mostly grey).
func dominantHue(img *image.NRGBA) (float64, bool) {
	var sx, sy, w float64
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y += 4 {
		for x := b.Min.X; x < b.Max.X; x += 4 {
			h, s, l := toHSL(img.NRGBAAt(x, y))
			k := s * (1 - math.Abs(2*l-1))
			sx += math.Cos(h*2*math.Pi) * k
			sy += math.Sin(h*2*math.Pi) * k
			w += k
		}
	}
	n := float64(b.Dx()*b.Dy()) / 16
	if w < n*0.08 {
		return 0, false
	}
	h := math.Atan2(sy, sx) / (2 * math.Pi)
	if h < 0 {
		h++
	}
	return h, true
}

// SavePNG writes an image as PNG (creating the folder).
func SavePNG(path string, img image.Image) error { return savePNG(path, img) }

// EncodePNG writes an image as PNG.
func EncodePNG(w io.Writer, img image.Image) error { return png.Encode(w, img) }

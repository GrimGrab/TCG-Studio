// Package art generates pack/box textures and shop icons from the vanilla templates the mod exports
// (<plugin>\templates). Layouts (pixels, 1024² textures, origin top-left) are documented in docs/runtime-facts.md.
package art

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type Options struct {
	Color string `json:"color"` // #rrggbb; empty = derived from the set id
	Title string `json:"title"` // text on the pack; empty = set name
	Icon  string `json:"icon"`  // relative path to an SVG/PNG icon in the project; empty = images/set_icon.svg if present
	// FilePrefix is prepended to the generated file names (e.g. "rare_" → images/rare_pack_texture.png).
	FilePrefix string `json:"filePrefix"`
	// FrontImage (relative path) replaces the generated pack front with your own artwork (cover-fitted).
	FrontImage string `json:"frontImage"`
	// TitleOnImage also prints the title over FrontImage.
	TitleOnImage bool `json:"titleOnImage"`
}

// Result lists generated files (relative to the project folder).
type Result struct {
	PackTexture string `json:"packTexture"`
	PackIcon    string `json:"packIcon"`
	BoxTexture  string `json:"boxTexture"`
	BoxIcon     string `json:"boxIcon"`
}

var (
	packFront = image.Rect(583, 106, 998, 724)
	packBack  = image.Rect(193, 106, 580, 724)
	boxStrip  = image.Rect(0, 0, 178, 305)
	boxBanner = image.Rect(185, 10, 525, 145)
	boxFront  = image.Rect(185, 150, 560, 388)
)

// DefaultColor picks a stable, saturated color from the set id.
func DefaultColor(id string) string {
	h := fnv.New32a()
	h.Write([]byte(id))
	r, g, b := hsl(float64(h.Sum32()%360)/360, 0.62, 0.45)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// Generate writes images/pack_texture.png, pack_icon.png, box_texture.png, box_icon.png into the project folder.
func Generate(templatesDir, projectFolder, setID, setName string, o Options) (Result, error) {
	if o.Color == "" {
		o.Color = DefaultColor(setID)
	}
	if o.Title == "" {
		o.Title = setName
	}
	base, err := parseHex(o.Color)
	if err != nil {
		return Result{}, err
	}
	hue, _, _ := toHSL(base)
	icon := loadIcon(projectFolder, o.Icon)
	face := func(size float64) font.Face { return newFace(size) }

	load := func(name string) (*image.NRGBA, error) {
		f, err := os.Open(filepath.Join(templatesDir, name))
		if err != nil {
			return nil, fmt.Errorf("template %s not found — load a save once with the mod installed so it exports templates", name)
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			return nil, err
		}
		dst := image.NewNRGBA(img.Bounds())
		draw.Draw(dst, dst.Bounds(), img, img.Bounds().Min, draw.Src)
		return dst, nil
	}

	dark := shade(base, 0.45)
	light := shade(base, 1.25)
	pre := "images/" + o.FilePrefix
	res := Result{PackTexture: pre + "pack_texture.png", PackIcon: pre + "pack_icon.png", BoxTexture: pre + "box_texture.png", BoxIcon: pre + "box_icon.png"}
	var front image.Image
	if o.FrontImage != "" {
		if front, err = loadImage(filepath.Join(projectFolder, filepath.FromSlash(o.FrontImage))); err != nil {
			return res, fmt.Errorf("front image: %w", err)
		}
	}

	// ---- pack texture: printed panels are detected from the template (saturated area) so crimps are never painted over
	pack, err := load("BasicCardPack_texture.png")
	if err != nil {
		return res, err
	}
	pf := detectPrinted(pack, image.Rect(583, 60, 1024, 780), packFront)
	pb := detectPrinted(pack, image.Rect(150, 60, 583, 780), packBack)
	if front != nil {
		drawCover(pack, pf, front)
		if o.Title != "" && o.TitleOnImage {
			drawText(pack, face, o.Title, image.Rect(pf.Min.X+40, pf.Min.Y+26, pf.Max.X-40, pf.Min.Y+150), 56, color.White)
		}
	} else {
		gradient(pack, pf, light, dark)
		drawIcon(pack, icon, center(pf, 0, 20), 250, 235)
		drawText(pack, face, o.Title, image.Rect(pf.Min.X+40, pf.Min.Y+26, pf.Max.X-40, pf.Min.Y+160), 60, color.White)
		drawText(pack, face, "7 CARDS", image.Rect(pf.Min.X+18, pf.Max.Y-90, pf.Max.X-18, pf.Max.Y-30), 34, color.NRGBA{255, 255, 255, 220})
	}
	gradient(pack, pb, dark, shade(base, 0.7))
	drawIcon(pack, icon, center(pb, 0, 0), 150, 70)

	// ---- pack icon: map the flat front panel onto the rendered pack's printed face (measured from the template)
	pIcon, err := load("BasicCardPack_icon.png")
	if err != nil {
		return res, err
	}
	faceMask := measureFace(pIcon)
	recolor(pIcon, pIcon.Bounds(), hue)
	warpFront(pIcon, pack.SubImage(pf).(*image.NRGBA), faceMask)
	packOnly := pIcon.SubImage(faceMask.bounds()).(*image.NRGBA) // just the pack, for box art

	// ---- box texture (the Basic box regions are the ones our box model shows)
	box, err := load("BasicCardBox_texture.png")
	if err != nil {
		return res, err
	}
	for _, r := range []image.Rectangle{boxStrip, boxBanner, boxFront} {
		gradient(box, r, light, dark)
	}
	drawIcon(box, icon, center(boxStrip, 0, -40), 110, 235)
	drawText(box, face, o.Title, image.Rect(boxStrip.Min.X+8, boxStrip.Max.Y-110, boxStrip.Max.X-8, boxStrip.Max.Y-20), 30, color.White)
	drawIcon(box, icon, image.Pt(boxBanner.Max.X-45, (boxBanner.Min.Y+boxBanner.Max.Y)/2), 70, 235)
	drawText(box, face, o.Title, image.Rect(boxBanner.Min.X+10, boxBanner.Min.Y+10, boxBanner.Max.X-90, boxBanner.Max.Y-10), 40, color.White)
	// Pack stack on the right of the front, logo/title on the left — like the vanilla boxes.
	stackH := boxFront.Dy() - 16
	stackW := stackH * packOnly.Bounds().Dx() / packOnly.Bounds().Dy()
	for i := 0; i < 3; i++ {
		x := boxFront.Max.X - stackW - 16 - (2-i)*18
		y := boxFront.Min.Y + 8 + (2-i)*2
		drawScaled(box, image.Rect(x, y, x+stackW, y+stackH), packOnly)
	}
	drawText(box, face, o.Title, image.Rect(boxFront.Min.X+14, boxFront.Min.Y+20, boxFront.Max.X-stackW-60, boxFront.Min.Y+150), 44, color.White)
	drawText(box, face, "BOOSTER BOX", image.Rect(boxFront.Min.X+14, boxFront.Min.Y+160, boxFront.Max.X-stackW-60, boxFront.Max.Y-20), 28, color.NRGBA{255, 255, 255, 220})

	// ---- box icon: recolor, cover the vanilla logo with a title plate and the vanilla packs with ours
	bIcon, err := load("BasicCardBox_icon.png")
	if err != nil {
		return res, err
	}
	recolor(bIcon, bIcon.Bounds(), hue)
	plate := image.Rect(32, 112, 232, 280)
	fillRounded(bIcon, plate, 14, shade(base, 0.35))
	drawText(bIcon, face, o.Title, plate.Inset(12), 40, color.White)
	for i := 0; i < 3; i++ {
		x := 240 + i*14
		drawScaled(bIcon, image.Rect(x, 172+i*4, x+132, 172+i*4+254), packOnly)
	}

	for rel, img := range map[string]*image.NRGBA{res.PackTexture: pack, res.PackIcon: pIcon, res.BoxTexture: box, res.BoxIcon: bIcon} {
		if err := savePNG(filepath.Join(projectFolder, filepath.FromSlash(rel)), img); err != nil {
			return res, err
		}
	}
	return res, nil
}

// ---------------------------------------------------------------- drawing helpers

func gradient(img *image.NRGBA, r image.Rectangle, top, bottom color.NRGBA) {
	h := float64(r.Dy())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		t := float64(y-r.Min.Y) / h
		c := mix(top, bottom, t)
		for x := r.Min.X; x < r.Max.X; x++ {
			// Subtle diagonal sheen so large panels aren't flat.
			s := 1 + 0.06*math.Sin(float64(x+y)/40)
			img.SetNRGBA(x, y, color.NRGBA{clamp(float64(c.R) * s), clamp(float64(c.G) * s), clamp(float64(c.B) * s), 255})
		}
	}
}

// recolor rotates the hue of saturated pixels to the target hue (keeps metallic/grey parts and alpha).
func recolor(img *image.NRGBA, r image.Rectangle, hue float64) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := img.NRGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			_, s, l := toHSL(c)
			if s < 0.18 {
				continue
			}
			nr, ng, nb := hsl(hue, s, l)
			img.SetNRGBA(x, y, color.NRGBA{nr, ng, nb, c.A})
		}
	}
}

func center(r image.Rectangle, dx, dy int) image.Point {
	return image.Pt((r.Min.X+r.Max.X)/2+dx, (r.Min.Y+r.Max.Y)/2+dy)
}

// drawIcon draws the icon (white, given alpha) centered at c with the given size.
func drawIcon(img *image.NRGBA, icon func(size int) *image.Alpha, c image.Point, size int, alpha uint8) {
	if icon == nil {
		return
	}
	mask := icon(size)
	if mask == nil {
		return
	}
	b := mask.Bounds()
	at := image.Rect(c.X-b.Dx()/2, c.Y-b.Dy()/2, c.X+b.Dx()/2+b.Dx()%2, c.Y+b.Dy()/2+b.Dy()%2)
	// Soft shadow, then the icon.
	shadow := at.Add(image.Pt(4, 5))
	draw.DrawMask(img, shadow, image.NewUniform(color.NRGBA{0, 0, 0, alpha / 3}), image.Point{}, mask, b.Min, draw.Over)
	draw.DrawMask(img, at, image.NewUniform(color.NRGBA{255, 255, 255, alpha}), image.Point{}, mask, b.Min, draw.Over)
}

// loadIcon returns a function rendering the set icon as an alpha mask of the requested size (aspect kept).
func loadIcon(folder, rel string) func(int) *image.Alpha {
	if rel == "" {
		rel = "images/set_icon.svg"
	}
	path := filepath.Join(folder, filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		return func(size int) *image.Alpha {
			ic, err := oksvg.ReadIconStream(bytes.NewReader(data), oksvg.IgnoreErrorMode)
			if err != nil {
				return nil
			}
			w, h := ic.ViewBox.W, ic.ViewBox.H
			if w <= 0 || h <= 0 {
				return nil
			}
			sw, sh := size, size
			if w > h {
				sh = int(float64(size) * h / w)
			} else {
				sw = int(float64(size) * w / h)
			}
			ic.SetTarget(0, 0, float64(sw), float64(sh))
			rgba := image.NewRGBA(image.Rect(0, 0, sw, sh))
			ic.Draw(rasterx.NewDasher(sw, sh, rasterx.NewScannerGV(sw, sh, rgba, rgba.Bounds())), 1)
			return toMask(rgba)
		}
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return func(size int) *image.Alpha {
		b := src.Bounds()
		sw, sh := size, size
		if b.Dx() > b.Dy() {
			sh = size * b.Dy() / b.Dx()
		} else {
			sw = size * b.Dx() / b.Dy()
		}
		dst := image.NewRGBA(image.Rect(0, 0, sw, sh))
		scaleNearest(dst, src)
		return toMask(dst)
	}
}

func toMask(img *image.RGBA) *image.Alpha {
	m := image.NewAlpha(img.Bounds())
	for i := 3; i < len(img.Pix); i += 4 {
		m.Pix[i/4] = img.Pix[i]
	}
	return m
}

func scaleNearest(dst *image.RGBA, src image.Image) {
	sb, db := src.Bounds(), dst.Bounds()
	for y := 0; y < db.Dy(); y++ {
		for x := 0; x < db.Dx(); x++ {
			dst.Set(x, y, src.At(sb.Min.X+x*sb.Dx()/db.Dx(), sb.Min.Y+y*sb.Dy()/db.Dy()))
		}
	}
}

// ---------------------------------------------------------------- text

var parsedFont *opentype.Font

func newFace(size float64) font.Face {
	if parsedFont == nil {
		parsedFont, _ = opentype.Parse(gobold.TTF)
	}
	f, _ := opentype.NewFace(parsedFont, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	return f
}

// drawText fits text into r (shrinking, then wrapping onto two lines) and draws it centered with a shadow.
type fontFace = font.Face

func drawText(img *image.NRGBA, face func(float64) font.Face, text string, r image.Rectangle, maxSize float64, col color.Color) {
	text = strings.ToUpper(strings.TrimSpace(text))
	if text == "" {
		return
	}
	lines := []string{text}
	size := maxSize
	fits := func(ls []string, sz float64) bool {
		f := face(sz)
		defer f.Close()
		for _, l := range ls {
			if font.MeasureString(f, l).Ceil() > r.Dx() {
				return false
			}
		}
		return float64(len(ls))*sz*1.1 <= float64(r.Dy())
	}
	for size > 12 && !fits(lines, size) {
		if len(lines) == 1 && strings.Contains(text, " ") && size < maxSize*0.75 {
			lines = splitTwo(text)
			size = maxSize
			continue
		}
		size -= 2
	}
	f := face(size)
	defer f.Close()
	lineH := size * 1.1
	y0 := float64(r.Min.Y) + (float64(r.Dy())-lineH*float64(len(lines)))/2 + size*0.85
	for i, l := range lines {
		w := font.MeasureString(f, l).Ceil()
		x := r.Min.X + (r.Dx()-w)/2
		y := int(y0 + lineH*float64(i))
		for _, off := range []image.Point{{3, 3}, {0, 0}} {
			c := col
			if off.X != 0 {
				c = color.NRGBA{0, 0, 0, 150}
			}
			d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: f, Dot: fixed.P(x+off.X, y+off.Y)}
			d.DrawString(l)
		}
	}
}

func splitTwo(s string) []string {
	words := strings.Fields(s)
	best, bestDiff := 1, math.MaxInt32
	for i := 1; i < len(words); i++ {
		a, b := len(strings.Join(words[:i], " ")), len(strings.Join(words[i:], " "))
		if d := int(math.Abs(float64(a - b))); d < bestDiff {
			best, bestDiff = i, d
		}
	}
	return []string{strings.Join(words[:best], " "), strings.Join(words[best:], " ")}
}

// ---------------------------------------------------------------- color

func parseHex(s string) (color.NRGBA, error) {
	var r, g, b uint8
	if _, err := fmt.Sscanf(strings.TrimPrefix(s, "#"), "%02x%02x%02x", &r, &g, &b); err != nil {
		return color.NRGBA{}, fmt.Errorf("bad color %q (use #rrggbb)", s)
	}
	return color.NRGBA{r, g, b, 255}, nil
}

func shade(c color.NRGBA, f float64) color.NRGBA {
	return color.NRGBA{clamp(float64(c.R) * f), clamp(float64(c.G) * f), clamp(float64(c.B) * f), 255}
}

func mix(a, b color.NRGBA, t float64) color.NRGBA {
	return color.NRGBA{clamp(float64(a.R) + (float64(b.R)-float64(a.R))*t), clamp(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		clamp(float64(a.B) + (float64(b.B)-float64(a.B))*t), 255}
}

func clamp(v float64) uint8 { return uint8(math.Max(0, math.Min(255, v))) }

func toHSL(c color.NRGBA) (h, s, l float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	switch mx {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h / 6, s, l
}

func hsl(h, s, l float64) (uint8, uint8, uint8) {
	if s == 0 {
		v := clamp(l * 255)
		return v, v, v
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	f := func(t float64) uint8 {
		if t < 0 {
			t++
		}
		if t > 1 {
			t--
		}
		switch {
		case t < 1.0/6:
			return clamp((p + (q-p)*6*t) * 255)
		case t < 0.5:
			return clamp(q * 255)
		case t < 2.0/3:
			return clamp((p + (q-p)*(2.0/3-t)*6) * 255)
		}
		return clamp(p * 255)
	}
	return f(h + 1.0/3), f(h), f(h - 1.0/3)
}

func savePNG(path string, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

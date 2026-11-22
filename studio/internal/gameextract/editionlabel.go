package gameextract

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/unityfs"
)

// The edition label (1st Edition, Silver, Gold, EX) as the game prints it on a card: CardFront/CardGrp/AnimGrp/
// FirstEditionTextGrp/FirstEditionText, a TextMeshPro text. The mod draws a copy of it on full-image cards at that spot plus a
// set's or card's own offset (set.json "editionLabel"); Studio's card previews draw it the same way. Measured in the game
// files 2026-11-22 (game 1.04): centred, 117.95 units under the middle of the 900-unit card front, box 308 × 25.6,
// auto size 18–52, Fredoka One.

// EditionLabelFile (in CardFramesDir) holds the label's layout; EditionLabelAtlas its font's SDF atlas.
const (
	EditionLabelFile  = "editionlabel.json"
	EditionLabelAtlas = "editionlabel-font.png"
)

// The unit of the label's x / y offsets: % of 645 × 900 card-front units (the mod's FullImageGradeLabel.CardSize, a first
// guess at the card's size that the offsets have been chosen in; the real card is smaller, CardW × CardH).
const editionOffsetW, editionOffsetH = 645.0, 900.0

// EditionLabel is editionlabel.json. Lengths are card-front units.
type EditionLabel struct {
	CardW    float64         `json:"cardWidth"` // the vanilla card as drawn: the solid part of the game's card mask
	CardH    float64         `json:"cardHeight"`
	CardX    float64         `json:"cardX"` // the card's centre from the front's (+ = right): the game's card sits 3.7 left
	OffsetW  float64         `json:"offsetWidth"` // x / y offsets are % of these
	OffsetH  float64         `json:"offsetHeight"`
	X        float64         `json:"x"` // the game's spot: the label's centre from the card's centre, + = right
	Y        float64         `json:"y"` // + = up
	W        float64         `json:"w"` // its box
	H        float64         `json:"h"`
	FontSize float32         `json:"fontSize"`
	AutoSize bool            `json:"autoSize"`
	SizeMin  float32         `json:"sizeMin"`
	SizeMax  float32         `json:"sizeMax"`
	Color    [4]float32      `json:"color"`
	Font     unityfs.TMPFont `json:"font"`
	Atlas    string          `json:"atlas"` // in CardFramesDir
}

func (x *extractor) editionLabel() error {
	if err := os.MkdirAll(filepath.Join(x.dir, CardFramesDir), 0o755); err != nil {
		return err
	}
	f, err := x.shopScene()
	if err != nil {
		return err
	}
	g := newGraph(x.env, f)
	for _, front := range g.byGO {
		if front.name != "CardFront" || front.tr.Rect == nil {
			continue
		}
		var text *node
		for _, n := range subtree(front) {
			if n.name == "FirstEditionText" && n.parent != nil && n.parent.name == "FirstEditionTextGrp" {
				text = n
			}
		}
		if text == nil {
			continue
		}
		var tmp *unityfs.Object
		for _, c := range text.comps {
			if c.ClassID == unityfs.ClassMonoBehaviour && x.env.ScriptClass(c) == "TextMeshProUGUI" {
				tmp = c
			}
		}
		if tmp == nil {
			continue
		}
		t, err := unityfs.ReadTMPText(tmp)
		if err != nil {
			return err
		}
		onPath := map[*node]bool{}
		for p := text; p != nil; p = p.parent {
			onPath[p] = true
		}
		mask := cardMaskNode(front)
		for p := mask; p != nil; p = p.parent {
			onPath[p] = true
		}
		// Only the text's own chain (the walker saves the sprites of what it draws); the card front is a 900² square.
		w := newUIWalker(x, f, "")
		w.layout(front, 900, 900, func(n *node) bool { return !onPath[n] })
		r := w.rects[text]
		cw, ch, cx, err := x.cardSize(f, front, w)
		if err != nil {
			return err
		}
		lay := EditionLabel{CardW: cw, CardH: ch, CardX: cx, OffsetW: editionOffsetW, OffsetH: editionOffsetH, X: r[0], Y: r[1], W: r[2], H: r[3],
			FontSize: t.FontSize, AutoSize: t.AutoSize, SizeMin: t.SizeMin, SizeMax: t.SizeMax, Color: t.Color}
		if lay.H <= 0 || lay.W <= 0 || math.Abs(lay.X) > cw/2 || math.Abs(lay.Y) > ch/2 {
			return fmt.Errorf("edition label layout looks wrong (%.0f,%.0f %.0f×%.0f)", lay.X, lay.Y, lay.W, lay.H)
		}
		fo, err := x.env.Resolve(f, t.Font)
		if err != nil || fo == nil {
			return fmt.Errorf("edition label font not found")
		}
		font, err := x.env.ReadTMPFont(fo)
		if err != nil {
			return err
		}
		if _, m := x.material(f, t.Material); m != nil {
			font.Gradient = m.Floats["_GradientScale"]
		}
		if font.Gradient <= 0 {
			return fmt.Errorf("edition label font has no gradient scale")
		}
		if len(font.Glyphs) == 0 {
			// The label's font ("FredokaOne-Regular SDF border", game 1.04) has no glyph table of its own (TMP fills it at
			// run time): its letters come from a font of the same face that has them (the shop sign's "FredokaOne-Regular SDF").
			gradient := font.Gradient
			if fo, font = x.tmpFontLike(font); fo == nil {
				return fmt.Errorf("edition label font %q has no letters and no other %s font has", font.Name, font.Family)
			}
			font.Gradient = gradient
		}
		_, atlas, ok := x.texture(fo.File, font.Atlas)
		if !ok {
			return fmt.Errorf("edition label font atlas not found")
		}
		font.AtlasW, font.AtlasH = atlas.Width, atlas.Height
		if !x.saveTexture(atlas, filepath.Join(x.dir, CardFramesDir, EditionLabelAtlas)) {
			return fmt.Errorf("edition label font atlas unreadable")
		}
		lay.Font, lay.Atlas = font, EditionLabelAtlas
		return writeJSON(filepath.Join(x.dir, CardFramesDir, EditionLabelFile), lay)
	}
	return fmt.Errorf("no card with an edition text found")
}

// tmpFontLike finds a TMP font of the same face and size as want that has a glyph table (nil when there is none); the
// one named like want without its " border" style comes first.
func (x *extractor) tmpFontLike(want unityfs.TMPFont) (*unityfs.Object, unityfs.TMPFont) {
	var best *unityfs.Object
	var bestFont unityfs.TMPFont
	for _, name := range x.assetFiles() {
		f, err := x.env.File(name)
		if err != nil || f == nil {
			continue
		}
		for _, o := range x.env.FindScripts(f, "TMP_FontAsset") {
			font, err := x.env.ReadTMPFont(o)
			if err != nil || len(font.Glyphs) == 0 || font.Family != want.Family || font.PointSize != want.PointSize {
				continue
			}
			if best == nil || strings.HasPrefix(want.Name, font.Name) {
				best, bestFont = o, font
			}
		}
	}
	return best, bestFont
}

// cardMaskNode is the card's shape under a card front: CardGrp/AnimGrp/CardBGMask (an Image of CardFullMask); nil if none.
func cardMaskNode(front *node) *node {
	for _, n := range subtree(front) {
		if n.name == "CardBGMask" && n.parent != nil && n.parent.name == "AnimGrp" {
			return n
		}
	}
	return nil
}

// cardSize is the vanilla card as drawn, in card-front units: the solid part (alpha over half) of the card mask's picture
// over the mask's rect, and its centre's x from the front's. Measured 2026-11-22: 0.622 × 0.8696 of a 902.7 square =
// 561.5 × 785, centre 8.5 of 2048 left (-3.7).
func (x *extractor) cardSize(f *unityfs.File, front *node, w *uiWalker) (float64, float64, float64, error) {
	mask := cardMaskNode(front)
	if mask == nil {
		return 0, 0, 0, fmt.Errorf("card mask not found")
	}
	im, ok := w.image(mask)
	if !ok {
		return 0, 0, 0, fmt.Errorf("card mask has no picture")
	}
	so, err := x.env.Resolve(f, im.Sprite)
	if err != nil || so == nil {
		return 0, 0, 0, fmt.Errorf("card mask sprite not found")
	}
	sp, err := unityfs.ReadSprite(so)
	if err != nil {
		return 0, 0, 0, err
	}
	img, err := x.env.SpriteImage(sp)
	if err != nil {
		return 0, 0, 0, err
	}
	b := img.Bounds()
	x0, y0, x1, y1 := b.Max.X, b.Max.Y, b.Min.X-1, b.Min.Y-1
	for py := b.Min.Y; py < b.Max.Y; py++ {
		for px := b.Min.X; px < b.Max.X; px++ {
			if _, _, _, a := img.At(px, py).RGBA(); a > 0x8000 {
				x0, y0, x1, y1 = min(x0, px), min(y0, py), max(x1, px), max(y1, py)
			}
		}
	}
	r := w.rects[mask]
	cw, ch := float64(x1-x0+1)/float64(b.Dx())*r[2], float64(y1-y0+1)/float64(b.Dy())*r[3]
	if cw < 300 || ch < 500 || cw > r[2] || ch > r[3] {
		return 0, 0, 0, fmt.Errorf("card mask size looks wrong (%.0f × %.0f)", cw, ch)
	}
	cx := r[0] + (float64(x0+x1+1)/2/float64(b.Dx())-0.5)*r[2]
	return cw, ch, cx, nil
}

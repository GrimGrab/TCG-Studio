package gameextract

import (
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"tcgstudio/internal/fontname"
	"tcgstudio/internal/unityfs"
)

// ShopSignDir is the templates sub-folder for Settings → Shop sign: the game's fonts (fonts/*.ttf, offered as sign fonts),
// the vanilla sign art (vanilla.png), the default sign font as an SDF atlas (default-font.png) and sign.json (layout).
const ShopSignDir = "shopsign"

// Sign face of BillboardMesh in mesh units (docs: runtime-facts "Shop sign"; mod ShopSign.cs).
const signMeshW, signMeshH = 4.645, 1.2

// The vanilla art's two banner halves in the sign texture (UV, bottom-up; same values as the mod's ShopSign.cs).
var signHalves = [2][4]float64{{0.0027, 0.505, 0.823, 0.9937}, {0.0084, 0.0051, 0.812, 0.4934}}

// SignLayout is sign.json. Lengths are UI units of the name text (its canvas units); x/y offsets are from the face centre,
// + = right/up as seen by the player.
type SignLayout struct {
	FaceW   float64         `json:"faceWidth"`
	FaceH   float64         `json:"faceHeight"`
	Text    SignText        `json:"text"`
	Default unityfs.TMPFont `json:"defaultFont"`
	Atlas   string          `json:"defaultAtlas"`
	Fonts   []SignFont      `json:"fonts"`
	Vanilla string          `json:"vanilla,omitempty"`
}

type SignText struct {
	X              float64       `json:"x"`
	Y              float64       `json:"y"`
	W              float64       `json:"w"`
	H              float64       `json:"h"`
	FontSize       float32       `json:"fontSize"`
	AutoSize       bool          `json:"autoSize"`
	SizeMin        float32       `json:"sizeMin"`
	SizeMax        float32       `json:"sizeMax"`
	Color          [4]float32    `json:"color"`
	Gradient       bool          `json:"gradient"`
	GradientColors [4][4]float32 `json:"gradientColors"` // top-left, top-right, bottom-left, bottom-right
}

type SignFont struct {
	Name string `json:"name"`
	File string `json:"file"` // relative to ShopSignDir
}

func (x *extractor) shopSign() error {
	dir := filepath.Join(x.dir, ShopSignDir)
	if err := os.MkdirAll(filepath.Join(dir, "fonts"), 0o755); err != nil {
		return err
	}
	f, err := x.env.File("level1")
	if err != nil || f == nil {
		return fmt.Errorf("level1 not found")
	}
	g := newGraph(x.env, f)
	var text, bill *node
	for _, n := range g.byGO {
		switch n.path() {
		case "CanvasWorldspace/CanvasGrp/Billboard_ShopName_Text":
			text = n
		case "Level_Environment_Grp/ShopGrp/Billboard":
			bill = n
		}
	}
	if text == nil || bill == nil || text.tr.Rect == nil {
		return fmt.Errorf("sign objects not found")
	}
	var lay SignLayout
	var tmp *unityfs.Object
	for _, c := range text.comps {
		if c.ClassID == unityfs.ClassMonoBehaviour && x.env.ScriptClass(c) == "TextMeshProUGUI" {
			tmp = c
		}
	}
	if tmp == nil {
		return fmt.Errorf("sign text component not found")
	}
	t, err := unityfs.ReadTMPText(tmp)
	if err != nil {
		return err
	}

	// Placement: the canvas is a RectTransform chain, whose serialized local positions are stale — rebuild them from anchors.
	tw, bw := uiWorld(text), bill.world()
	unit := length(tw.vector([3]float64{1, 0, 0}))
	right := normalize(tw.vector([3]float64{1, 0, 0}))
	up := normalize(tw.vector([3]float64{0, 1, 0}))
	d := sub(tw.point([3]float64{}), bw.point([3]float64{}))
	dot := func(a, b [3]float64) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
	lay.FaceW = signMeshW * length(bw.vector([3]float64{1, 0, 0})) / unit
	lay.FaceH = signMeshH * length(bw.vector([3]float64{0, 1, 0})) / unit
	r := text.tr.Rect
	lay.Text = SignText{X: dot(d, right) / unit, Y: dot(d, up) / unit, W: float64(r.SizeDelta[0]), H: float64(r.SizeDelta[1]),
		FontSize: t.FontSize, AutoSize: t.AutoSize, SizeMin: t.SizeMin, SizeMax: t.SizeMax, Color: t.Color,
		Gradient: t.Gradient, GradientColors: t.GradientColors}
	if math.Abs(lay.FaceW/lay.FaceH-4.11) > 0.05 || math.Abs(lay.Text.X) > lay.FaceW/2 || math.Abs(lay.Text.Y) > lay.FaceH/2 {
		return fmt.Errorf("sign layout looks wrong (face %.0f×%.0f, text at %.0f,%.0f)", lay.FaceW, lay.FaceH, lay.Text.X, lay.Text.Y)
	}

	// The default font: glyph table + SDF atlas, so the preview draws the game's own letters.
	fo, err := x.env.Resolve(f, t.Font)
	if err != nil || fo == nil {
		return fmt.Errorf("sign font not found")
	}
	font, err := x.env.ReadTMPFont(fo)
	if err != nil {
		return err
	}
	if _, m := x.material(f, t.Material); m != nil {
		font.Gradient = m.Floats["_GradientScale"]
	}
	if font.Gradient <= 0 {
		return fmt.Errorf("sign font has no gradient scale")
	}
	_, atlas, ok := x.texture(fo.File, font.Atlas)
	if !ok {
		return fmt.Errorf("sign font atlas not found")
	}
	font.AtlasW, font.AtlasH = atlas.Width, atlas.Height
	if !x.saveTexture(atlas, filepath.Join(dir, "default-font.png")) {
		return fmt.Errorf("sign font atlas unreadable")
	}
	lay.Default, lay.Atlas = font, "default-font.png"

	// Vanilla sign art as one banner (both halves side by side), the preview's background without a custom image.
	if banner := x.vanillaBanner(f, bill); banner != nil && savePNG(filepath.Join(dir, "vanilla.png"), banner) == nil {
		lay.Vanilla = "vanilla.png"
	}

	// Every font the game ships with its data: offered as sign fonts.
	seen := map[string]bool{}
	for _, name := range []string{"resources.assets", "sharedassets0.assets", "sharedassets1.assets"} {
		sf, err := x.env.File(name)
		if err != nil || sf == nil {
			continue
		}
		for _, o := range sf.Objects {
			if o.ClassID != unityfs.ClassFont {
				continue
			}
			b, err := unityfs.FontFile(o)
			if err != nil {
				continue
			}
			on, _ := o.Name()
			display := fontname.Of(b)
			if display == "" {
				display = on
			}
			if seen[strings.ToLower(display)] {
				continue
			}
			seen[strings.ToLower(display)] = true
			ext := ".ttf"
			if string(b[:4]) == "OTTO" {
				ext = ".otf"
			}
			file := "fonts/" + safe(on) + ext
			if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(file)), b, 0o644); err != nil {
				return err
			}
			lay.Fonts = append(lay.Fonts, SignFont{Name: display, File: file})
		}
	}
	sort.Slice(lay.Fonts, func(i, j int) bool { return strings.ToLower(lay.Fonts[i].Name) < strings.ToLower(lay.Fonts[j].Name) })
	return writeJSON(filepath.Join(dir, "sign.json"), lay)
}

// uiWorld is a node's world matrix where RectTransforms take their local position from the anchors (Unity recomputes it;
// the serialized one can be stale). Only the unstretched case the sign uses (anchorMin == anchorMax).
func uiWorld(n *node) mat {
	local := n.local()
	if r := n.tr.Rect; r != nil {
		x, y := float64(r.AnchoredPosition[0]), float64(r.AnchoredPosition[1])
		if p := n.parent; p != nil && p.tr.Rect != nil {
			pr := p.tr.Rect
			for i, v := range []*float64{&x, &y} {
				size := float64(pr.SizeDelta[i])
				*v += -size*float64(pr.Pivot[i]) + size*float64(r.AnchorMin[i])
			}
		}
		local[0][3], local[1][3] = x, y
	}
	if n.parent == nil {
		return local
	}
	return mul(uiWorld(n.parent), local)
}

func (x *extractor) vanillaBanner(f *unityfs.File, bill *node) image.Image {
	for _, c := range bill.comps {
		if c.ClassID != unityfs.ClassMeshRenderer {
			continue
		}
		rend, err := unityfs.ReadRenderer(c)
		if err != nil || len(rend.Materials) == 0 {
			return nil
		}
		_, tex, ok := x.mainTexture(f, rend.Materials[0])
		if !ok {
			return nil
		}
		src, err := x.env.TextureImage(tex)
		if err != nil {
			return nil
		}
		const w = 1644
		h := int(math.Round(w / 4.11))
		out := image.NewNRGBA(image.Rect(0, 0, w, h))
		sw, sh := float64(src.Bounds().Dx()), float64(src.Bounds().Dy())
		for py := range h {
			fy := (float64(py) + 0.5) / float64(h)
			for px := range w {
				half := signHalves[0]
				fx := (float64(px) + 0.5) / float64(w) * 2
				if fx >= 1 {
					half, fx = signHalves[1], fx-1
				}
				u := half[0] + (half[2]-half[0])*fx
				v := half[3] - (half[3]-half[1])*fy // top of the banner = top of the strip (UV is bottom-up)
				sx, sy := int(u*sw), int((1-v)*sh)
				out.SetNRGBA(px, py, src.NRGBAAt(min(sx, int(sw)-1), min(sy, int(sh)-1)))
			}
		}
		return out
	}
	return nil
}

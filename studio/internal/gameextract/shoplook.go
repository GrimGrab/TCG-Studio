package gameextract

import (
	"fmt"
	"image"
	"os"
	"path/filepath"

	"tcgstudio/internal/unityfs"
)

// Settings → Shop look previews of the phone wallpaper and the open/closed sign, written next to the shop sign files
// (ShopSignDir): phone.json + phone/*.png and door.json + door.png.

// DoorBoards are the two boards of the open/closed sign texture (T_OpenCloseSign, 512²): x, y, w, h as fractions of the
// texture, top-left origin, the coloured area inside the black frame plus 1 px (the mask keeps the frame). DoorScrews are the screw heads kept on top of a custom
// picture: centre x, y as fractions of the board, radius as a fraction of the texture width. Measured from the game files
// 2026-10-10; the mod's Runtime/DoorSign.cs has the same values.
var (
	DoorBoards = map[string][4]float64{
		"open":   {42.0 / 512, 25.0 / 512, 328.0 / 512, 206.0 / 512},
		"closed": {42.0 / 512, 277.0 / 512, 328.0 / 512, 206.0 / 512},
	}
	DoorScrews = [][3]float64{{0.0412, 0.0583, 7.0 / 512}, {0.9558, 0.0583, 7.0 / 512}}
)

// PhoneLayout is phone.json: the home screen drawn over the wallpaper. Lengths are phone UI units; x/y are part centres from
// the screen centre, + = right/up.
type PhoneLayout struct {
	Width   float64     `json:"width"`
	Height  float64     `json:"height"`
	Vanilla string      `json:"vanilla"` // the vanilla wallpaper
	Parts   []PhonePart `json:"parts"`   // back to front
}

type PhonePart struct {
	Rect           [4]float64 `json:"rect"`            // x, y, w, h
	Image          string     `json:"image,omitempty"` // sprite file, relative to ShopSignDir
	PreserveAspect bool       `json:"preserveAspect,omitempty"`
	Text           string     `json:"text,omitempty"`
	FontSize       float64    `json:"fontSize,omitempty"` // phone units
	Color          [4]float32 `json:"color"`
}

// DoorLayout is door.json.
type DoorLayout struct {
	Texture string                `json:"texture"`
	Boards  map[string][4]float64 `json:"boards"`
	Screws  [][3]float64          `json:"screws"`
}

func (x *extractor) shopLook() error {
	f, err := x.env.File("level1")
	if err != nil || f == nil {
		return fmt.Errorf("level1 not found")
	}
	g := newGraph(x.env, f)
	var phone, door *node
	for _, n := range g.byGO {
		switch {
		case n.has("UI_PhoneScreen"):
			phone = n
		case n.has("InteractableOpenCloseSign"):
			door = n
		}
	}
	var errs []error
	if phone == nil {
		errs = append(errs, fmt.Errorf("phone screen not found"))
	} else if err := x.phoneLayout(f, phone); err != nil {
		errs = append(errs, fmt.Errorf("phone: %w", err))
	}
	if door == nil {
		errs = append(errs, fmt.Errorf("open/closed sign not found"))
	} else if err := x.doorLayout(f, door); err != nil {
		errs = append(errs, fmt.Errorf("open/closed sign: %w", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("%v", errs)
	}
	return nil
}

// phoneSkip are parts of the home screen the preview leaves out: the wallpaper itself, invisible click areas and the buttons'
// press overlay (BtnInteraction, hidden in game).
var phoneSkip = map[string]bool{"PhoneBG": true, "CloseScreenAreaL": true, "CloseScreenAreaR": true, "ControllerSelectorUIGrp": true,
	"BtnInteraction": true}

func (x *extractor) phoneLayout(f *unityfs.File, root *node) error {
	dir := filepath.Join(x.dir, ShopSignDir)
	if err := os.MkdirAll(filepath.Join(dir, "phone"), 0o755); err != nil {
		return err
	}
	if root.tr.Rect == nil {
		return fmt.Errorf("phone screen has no rect")
	}
	lay := PhoneLayout{Width: float64(root.tr.Rect.SizeDelta[0]), Height: float64(root.tr.Rect.SizeDelta[1])}
	saved := map[string]string{}
	sprite := func(p unityfs.PPtr) string {
		o, err := x.env.Resolve(f, p)
		if err != nil || o == nil || o.ClassID != unityfs.ClassSprite {
			return ""
		}
		s, err := unityfs.ReadSprite(o)
		if err != nil {
			return ""
		}
		file := "phone/" + safe(s.Name) + ".png"
		if _, ok := saved[file]; !ok {
			img, err := x.env.SpriteImage(s)
			if err != nil || savePNG(filepath.Join(dir, filepath.FromSlash(file)), img) != nil {
				return ""
			}
			saved[file] = s.Name
		}
		return file
	}

	// Rect layout in phone units: each node's pivot position, its size and the scale it inherits (rotation isn't used here).
	var visit func(n *node, px, py, scale, pw, ph, ppx, ppy float64)
	visit = func(n *node, px, py, scale, pw, ph, ppx, ppy float64) {
		if !n.active || phoneSkip[n.name] {
			return
		}
		var lx, ly, w, h, pivX, pivY float64
		pivX, pivY = 0.5, 0.5
		if r := n.tr.Rect; r != nil && n != root {
			w = pw*float64(r.AnchorMax[0]-r.AnchorMin[0]) + float64(r.SizeDelta[0])
			h = ph*float64(r.AnchorMax[1]-r.AnchorMin[1]) + float64(r.SizeDelta[1])
			pivX, pivY = float64(r.Pivot[0]), float64(r.Pivot[1])
			ax := float64(r.AnchorMin[0]) + float64(r.AnchorMax[0]-r.AnchorMin[0])*pivX
			ay := float64(r.AnchorMin[1]) + float64(r.AnchorMax[1]-r.AnchorMin[1])*pivY
			lx = pw*(ax-ppx) + float64(r.AnchoredPosition[0])
			ly = ph*(ay-ppy) + float64(r.AnchoredPosition[1])
		} else if n == root {
			w, h = lay.Width, lay.Height
		} else {
			lx, ly = float64(n.tr.Position[0]), float64(n.tr.Position[1])
		}
		x0, y0 := px+lx*scale, py+ly*scale // this node's pivot
		s := scale
		if n != root {
			s *= float64(n.tr.Scale[0])
		}
		cx, cy := x0+(0.5-pivX)*w*s, y0+(0.5-pivY)*h*s
		part := PhonePart{Rect: [4]float64{cx, cy, w * s, h * s}}
		for _, c := range n.comps {
			if c.ClassID != unityfs.ClassMonoBehaviour {
				continue
			}
			switch x.env.ScriptClass(c) {
			case "Image":
				im, err := unityfs.ReadUIImage(c)
				if err != nil || !im.Enabled || im.Color[3] == 0 {
					continue
				}
				if part.Image = sprite(im.Sprite); part.Image != "" {
					part.Color, part.PreserveAspect = im.Color, im.PreserveAspect
					lay.Parts = append(lay.Parts, part)
				}
			case "TextMeshProUGUI":
				t, err := unityfs.ReadTMPText(c)
				if err != nil || t.Text == "" {
					continue
				}
				lay.Parts = append(lay.Parts, PhonePart{Rect: part.Rect, Text: t.Text, FontSize: float64(t.FontSize) * s, Color: t.Color})
			}
		}
		for _, c := range n.children {
			visit(c, x0, y0, s, w, h, pivX, pivY)
		}
	}
	visit(root, 0, 0, 1, 0, 0, 0.5, 0.5)

	// The vanilla wallpaper: the home screen's PhoneBG Image (other phone screens have their own).
	for _, n := range subtree(root) {
		if n.name != "PhoneBG" || n.parent == nil || n.parent.name != "AnimGrp" || !n.activeInHierarchy() {
			continue
		}
		for _, c := range n.comps {
			if c.ClassID == unityfs.ClassMonoBehaviour && x.env.ScriptClass(c) == "Image" {
				if im, err := unityfs.ReadUIImage(c); err == nil {
					lay.Vanilla = sprite(im.Sprite)
				}
			}
		}
	}
	if lay.Vanilla == "" || len(lay.Parts) < 10 {
		return fmt.Errorf("home screen looks wrong (%d parts, wallpaper %q)", len(lay.Parts), lay.Vanilla)
	}
	return writeJSON(filepath.Join(dir, "phone.json"), lay)
}

// subtree lists a node and its descendants.
func subtree(n *node) (out []*node) {
	n.walk(func(c *node) { out = append(out, c) })
	return out
}

func (x *extractor) doorLayout(f *unityfs.File, sign *node) error {
	var tex unityfs.Texture2D
	found := false
	sign.walk(func(n *node) {
		for _, c := range n.comps {
			if found || c.ClassID != unityfs.ClassMeshRenderer {
				continue
			}
			rend, err := unityfs.ReadRenderer(c)
			if err != nil || len(rend.Materials) == 0 {
				continue
			}
			if _, t, ok := x.mainTexture(f, rend.Materials[0]); ok && t.Name == "T_OpenCloseSign" {
				tex, found = t, true
			}
		}
	})
	if !found {
		return fmt.Errorf("sign texture not found")
	}
	img, err := x.env.TextureImage(tex)
	if err != nil {
		return err
	}
	// The boards must still be where we measured them: green and red in their middles.
	if !boardColour(img, DoorBoards["open"], func(r, g, b uint8) bool { return g > 120 && r < 80 && b < 80 }) ||
		!boardColour(img, DoorBoards["closed"], func(r, g, b uint8) bool { return r > 150 && g < 80 && b < 80 }) {
		return fmt.Errorf("the sign texture changed (boards not at the measured places)")
	}
	if err := savePNG(filepath.Join(x.dir, ShopSignDir, "door.png"), img); err != nil {
		return err
	}
	return writeJSON(filepath.Join(x.dir, ShopSignDir, "door.json"), DoorLayout{Texture: "door.png", Boards: DoorBoards, Screws: DoorScrews})
}

// boardColour: the pixel just inside the board's left edge, halfway down (clear of the lettering), matches.
func boardColour(img *image.NRGBA, b [4]float64, ok func(r, g, b uint8) bool) bool {
	w, h := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	c := img.NRGBAAt(int((b[0]+0.02)*w), int((b[1]+b[3]/2)*h))
	return ok(c.R, c.G, c.B)
}

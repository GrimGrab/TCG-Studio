package gameextract

import (
	"fmt"
	"image"
	"os"
	"path/filepath"

	"tcgstudio/internal/unityfs"
)

// Game look previews, written next to the shop sign files (ShopSignDir): the phone wallpaper (phone.json + phone/*.png), the
// open/closed sign (door.json + door.png), the credit card (card.json + card.png), the main menu (menu.json + menu/*.png) and
// the loading screen (loading.json + loading/*.png).

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

// The credit card's print (texture credit_card_D, 1024×512, covering the card's whole printed side): areas the mod's
// CreditCardKeepChip / CreditCardKeepPrint paint back over a picture, x, y, w, h in texture pixels, top-left origin.
// Measured from the game files 2026-10-17 plus 4 px for the emboss shadow; the mod's Runtime/CreditCard.cs has the same values.
var (
	CardChip  = [4]int{104, 133, 164, 111}
	CardPrint = [][4]int{{92, 276, 816, 49}, {96, 333, 508, 71}, {91, 430, 324, 41}} // number, small print + expiry, name
)

// CardAspect is the width ÷ height of the card's printed side (mesh credit_card_001: 85.72 × 53.98 mm).
const CardAspect = 85.72 / 53.98

// The game's UI reference resolution (CanvasScaler): the main menu and loading screen are laid out in these units.
const screenW, screenH = 1920, 1080

// UILayout is a screen drawn over a picture (phone.json, menu.json, loading.json). Lengths are the screen's UI units; x/y are
// part centres from the screen centre, + = right/up.
type UILayout struct {
	Width   float64    `json:"width"`
	Height  float64    `json:"height"`
	Vanilla string     `json:"vanilla,omitempty"` // the vanilla background sprite ("" = a plain colour, vanillaColor)
	Colour  [4]float32 `json:"vanillaColor"`      // tint of the vanilla background
	Parts   []UIPart   `json:"parts"`             // back to front
}

type UIPart struct {
	Rect           [4]float64 `json:"rect"`            // x, y, w, h
	Image          string     `json:"image,omitempty"` // sprite file, relative to ShopSignDir
	PreserveAspect bool       `json:"preserveAspect,omitempty"`
	Sliced         bool       `json:"sliced,omitempty"` // a 9-sliced panel (drawn as a rounded box in its colour)
	Text           string     `json:"text,omitempty"`
	FontSize       float64    `json:"fontSize,omitempty"` // UI units
	Color          [4]float32 `json:"color"`
	Card           bool       `json:"card,omitempty"` // a card the game draws there (shown as a card outline)
}

// DoorLayout is door.json.
type DoorLayout struct {
	Texture string                `json:"texture"`
	Boards  map[string][4]float64 `json:"boards"`
	Screws  [][3]float64          `json:"screws"`
}

// CardLayout is card.json.
type CardLayout struct {
	Texture string   `json:"texture"`
	Aspect  float64  `json:"aspect"`
	Chip    [4]int   `json:"chip"`
	Print   [][4]int `json:"print"`
}

func (x *extractor) shopLook() error {
	var errs []error
	try := func(what string, err error) {
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", what, err))
		}
	}
	if f, err := x.env.File("level1"); err != nil || f == nil {
		errs = append(errs, fmt.Errorf("level1 not found"))
	} else {
		g := newGraph(x.env, f)
		try("phone", x.phoneLayout(f, g.scriptNode("UI_PhoneScreen")))
		try("open/closed sign", x.doorLayout(f, g.scriptNode("InteractableOpenCloseSign")))
		try("credit card", x.cardLayout(f, g.scriptNode("InteractableCustomerCash")))
	}
	if f, err := x.env.File("level0"); err != nil || f == nil {
		errs = append(errs, fmt.Errorf("level0 not found"))
	} else {
		g := newGraph(x.env, f)
		try("main menu", x.screenLayout(f, g.scriptNode("TitleScreen"), "menu", nil))
		try("loading screen", x.screenLayout(f, g.scriptNode("LoadingScreen"), "loading", map[string]bool{"CardGroup": true}))
	}
	if len(errs) > 0 {
		return fmt.Errorf("%v", errs)
	}
	return nil
}

// scriptNode is a GameObject with a MonoBehaviour of class (nil when none).
func (g *graph) scriptNode(class string) *node {
	for _, n := range g.byGO {
		if n.has(class) {
			return n
		}
	}
	return nil
}

// phoneSkip are parts of the home screen the preview leaves out: the wallpaper itself, invisible click areas and the buttons'
// press overlay (BtnInteraction, hidden in game).
var phoneSkip = map[string]bool{"PhoneBG": true, "CloseScreenAreaL": true, "CloseScreenAreaR": true, "ControllerSelectorUIGrp": true,
	"BtnInteraction": true}

func (x *extractor) phoneLayout(f *unityfs.File, root *node) error {
	if root == nil {
		return fmt.Errorf("phone screen not found")
	}
	if root.tr.Rect == nil {
		return fmt.Errorf("phone screen has no rect")
	}
	w := newUIWalker(x, f, "phone")
	lay := w.layout(root, float64(root.tr.Rect.SizeDelta[0]), float64(root.tr.Rect.SizeDelta[1]), func(n *node) bool { return phoneSkip[n.name] })
	// The vanilla wallpaper: the home screen's PhoneBG Image (other phone screens have their own).
	for _, n := range subtree(root) {
		if n.name == "PhoneBG" && n.parent != nil && n.parent.name == "AnimGrp" && n.activeInHierarchy() {
			if im, ok := w.image(n); ok {
				lay.Vanilla, lay.Colour = w.sprite(im.Sprite), im.Color
			}
		}
	}
	if lay.Vanilla == "" || len(lay.Parts) < 10 {
		return fmt.Errorf("home screen looks wrong (%d parts, wallpaper %q)", len(lay.Parts), lay.Vanilla)
	}
	return writeJSON(filepath.Join(x.dir, ShopSignDir, "phone.json"), lay)
}

// screenLayout writes <name>.json for a full-screen UI (the main menu, the loading screen): the screen's ScreenGrp without
// its BG Image (the background a picture replaces) and the parts in skip, in the game's reference resolution.
func (x *extractor) screenLayout(f *unityfs.File, screen *node, name string, skip map[string]bool) error {
	if screen == nil {
		return fmt.Errorf("screen not found")
	}
	var grp, bg *node
	for _, c := range screen.children {
		if c.name == "ScreenGrp" {
			grp = c
		}
	}
	if grp != nil {
		for _, c := range grp.children {
			if c.name == "BG" {
				bg = c
			}
		}
	}
	if bg == nil {
		return fmt.Errorf("ScreenGrp/BG not found")
	}
	w := newUIWalker(x, f, name)
	lay := w.layout(grp, screenW, screenH, func(n *node) bool { return n == bg || n.name == "BtnInteraction" || skip[n.name] })
	im, ok := w.image(bg)
	if !ok {
		return fmt.Errorf("the background has no image")
	}
	lay.Vanilla, lay.Colour = w.sprite(im.Sprite), im.Color
	// Parts the game draws itself on top (the loading screen's spinning card): their place only.
	for _, n := range subtree(grp) {
		if r, ok := w.rects[n]; ok && skip[n.name] {
			lay.Parts = append(lay.Parts, UIPart{Rect: r, Card: true})
		}
	}
	if len(lay.Parts) == 0 {
		return fmt.Errorf("no parts")
	}
	return writeJSON(filepath.Join(x.dir, ShopSignDir, name+".json"), lay)
}

// uiWalker lays out a UI hierarchy (RectTransforms, Images, TextMeshPro texts) in its root's units and saves the sprites it
// uses under ShopSignDir/<sub>/.
type uiWalker struct {
	x     *extractor
	f     *unityfs.File
	sub   string
	saved map[string]bool
	rects map[*node][4]float64 // every active node's rect (x, y, w, h), skipped ones too
}

func newUIWalker(x *extractor, f *unityfs.File, sub string) *uiWalker {
	return &uiWalker{x: x, f: f, sub: sub, saved: map[string]bool{}, rects: map[*node][4]float64{}}
}

// sprite saves a sprite as <sub>/<name>.png and returns that path ("" when it can't be read).
func (w *uiWalker) sprite(p unityfs.PPtr) string {
	o, err := w.x.env.Resolve(w.f, p)
	if err != nil || o == nil || o.ClassID != unityfs.ClassSprite {
		return ""
	}
	s, err := unityfs.ReadSprite(o)
	if err != nil {
		return ""
	}
	file := w.sub + "/" + safe(s.Name) + ".png"
	if !w.saved[file] {
		path := filepath.Join(w.x.dir, ShopSignDir, filepath.FromSlash(file))
		img, err := w.x.env.SpriteImage(s)
		if err != nil || os.MkdirAll(filepath.Dir(path), 0o755) != nil || savePNG(path, img) != nil {
			return ""
		}
		w.saved[file] = true
	}
	return file
}

// image is a node's enabled UI Image.
func (w *uiWalker) image(n *node) (unityfs.UIImage, bool) {
	for _, c := range n.comps {
		if c.ClassID == unityfs.ClassMonoBehaviour && w.x.env.ScriptClass(c) == "Image" {
			if im, err := unityfs.ReadUIImage(c); err == nil {
				return im, true
			}
		}
	}
	return unityfs.UIImage{}, false
}

// layout walks root (size width × height; drawn even when inactive, like a screen the game opens) and returns its visible
// parts back to front, leaving out inactive nodes and those skip picks (with their children; their rect is still kept).
func (w *uiWalker) layout(root *node, width, height float64, skip func(*node) bool) UILayout {
	lay := UILayout{Width: width, Height: height}
	var visit func(n *node, px, py, scale, pw, ph, ppx, ppy float64)
	visit = func(n *node, px, py, scale, pw, ph, ppx, ppy float64) {
		if n != root && !n.active {
			return
		}
		// Rect layout in root units: each node's pivot position, its size and the scale it inherits (rotation isn't used here).
		var lx, ly, rw, rh, pivX, pivY float64
		pivX, pivY = 0.5, 0.5
		if r := n.tr.Rect; r != nil && n != root {
			rw = pw*float64(r.AnchorMax[0]-r.AnchorMin[0]) + float64(r.SizeDelta[0])
			rh = ph*float64(r.AnchorMax[1]-r.AnchorMin[1]) + float64(r.SizeDelta[1])
			pivX, pivY = float64(r.Pivot[0]), float64(r.Pivot[1])
			ax := float64(r.AnchorMin[0]) + float64(r.AnchorMax[0]-r.AnchorMin[0])*pivX
			ay := float64(r.AnchorMin[1]) + float64(r.AnchorMax[1]-r.AnchorMin[1])*pivY
			lx = pw*(ax-ppx) + float64(r.AnchoredPosition[0])
			ly = ph*(ay-ppy) + float64(r.AnchoredPosition[1])
		} else if n == root {
			rw, rh = width, height
		} else {
			lx, ly = float64(n.tr.Position[0]), float64(n.tr.Position[1])
		}
		x0, y0 := px+lx*scale, py+ly*scale // this node's pivot
		s := scale
		if n != root {
			s *= float64(n.tr.Scale[0])
		}
		cx, cy := x0+(0.5-pivX)*rw*s, y0+(0.5-pivY)*rh*s
		rect := [4]float64{cx, cy, rw * s, rh * s}
		w.rects[n] = rect
		if n != root && skip(n) {
			return
		}
		for _, c := range n.comps {
			if c.ClassID != unityfs.ClassMonoBehaviour {
				continue
			}
			switch w.x.env.ScriptClass(c) {
			case "Image":
				im, err := unityfs.ReadUIImage(c)
				if err != nil || !im.Enabled || im.Color[3] == 0 {
					continue
				}
				if file := w.sprite(im.Sprite); file != "" {
					lay.Parts = append(lay.Parts, UIPart{Rect: rect, Image: file, Color: im.Color, PreserveAspect: im.PreserveAspect,
						Sliced: im.Type == 1})
				}
			case "TextMeshProUGUI":
				t, err := unityfs.ReadTMPText(c)
				if err != nil || t.Text == "" {
					continue
				}
				lay.Parts = append(lay.Parts, UIPart{Rect: rect, Text: t.Text, FontSize: float64(t.FontSize) * s, Color: t.Color})
			}
		}
		for _, c := range n.children {
			visit(c, x0, y0, s, rw, rh, pivX, pivY)
		}
	}
	visit(root, 0, 0, 1, 0, 0, 0.5, 0.5)
	return lay
}

// subtree lists a node and its descendants.
func subtree(n *node) (out []*node) {
	n.walk(func(c *node) { out = append(out, c) })
	return out
}

// rendererTexture is the first main texture called name drawn by a MeshRenderer under root (any material slot).
func (x *extractor) rendererTexture(f *unityfs.File, root *node, name string) (unityfs.Texture2D, bool) {
	var tex unityfs.Texture2D
	found := false
	root.walk(func(n *node) {
		for _, c := range n.comps {
			if found || c.ClassID != unityfs.ClassMeshRenderer {
				continue
			}
			rend, err := unityfs.ReadRenderer(c)
			if err != nil {
				continue
			}
			for _, m := range rend.Materials {
				if _, t, ok := x.mainTexture(f, m); ok && t.Name == name {
					tex, found = t, true
				}
			}
		}
	})
	return tex, found
}

func (x *extractor) doorLayout(f *unityfs.File, sign *node) error {
	if sign == nil {
		return fmt.Errorf("sign not found")
	}
	tex, found := x.rendererTexture(f, sign, "T_OpenCloseSign")
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

func (x *extractor) cardLayout(f *unityfs.File, cash *node) error {
	if cash == nil {
		return fmt.Errorf("customer cash not found")
	}
	tex, found := x.rendererTexture(f, cash, "credit_card_D")
	if !found {
		return fmt.Errorf("card texture not found")
	}
	img, err := x.env.TextureImage(tex)
	if err != nil {
		return err
	}
	// The print must still be where we measured it: the chip's middle is grey, the card beside it blue.
	chroma := func(px, py int) int {
		c := img.NRGBAAt(px, py)
		return int(max(c.R, c.G, c.B)) - int(min(c.R, c.G, c.B))
	}
	if img.Bounds().Dx() != 1024 || img.Bounds().Dy() != 512 || chroma(CardChip[0]+CardChip[2]/2, CardChip[1]+CardChip[3]/2) > 45 ||
		chroma(CardChip[0]+CardChip[2]+40, CardChip[1]+CardChip[3]/2) < 70 {
		return fmt.Errorf("the card texture changed (chip not at the measured place)")
	}
	if err := savePNG(filepath.Join(x.dir, ShopSignDir, "card.png"), img); err != nil {
		return err
	}
	return writeJSON(filepath.Join(x.dir, ShopSignDir, "card.json"), CardLayout{Texture: "card.png", Aspect: CardAspect, Chip: CardChip, Print: CardPrint})
}

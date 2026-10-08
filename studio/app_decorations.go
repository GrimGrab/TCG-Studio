package main

import (
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/decoart"
	"tcgstudio/internal/figurine"
	"tcgstudio/internal/game"
	"tcgstudio/internal/gameextract"
	"tcgstudio/internal/setfmt"
)

// ---------------------------------------------------------------- custom decorations (accessory library "decorations" list)
//
// Wall / floor / ceiling looks (a tiling texture on the shop's surfaces) and placeable decorations: posters made from an image
// and objects from an imported model, hung on a wall or stood on the floor. The vanilla ones — names, prices, icons and the two
// base models — come from the game files (game-templates\decorations\decorations.json, internal/gameextract/decorations.go).

// DecorationKinds lists the decoration kinds (tabs) and their mod toggles.
func (a *App) DecorationKinds() []setfmt.DecorationKind { return setfmt.DecorationKinds }

// DecorationTemplates returns decorations.json of the game templates as raw JSON, or "" while it isn't available.
func (a *App) DecorationTemplates() string {
	if !game.IsGameDir(a.settings.GameDir) {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(a.templatesDir(), gameextract.DecorationsDir, "decorations.json"))
	if err != nil {
		return ""
	}
	return string(b)
}

// NewDecoration makes a draft with a fresh id; nothing is stored until SaveDecoration (a look needs its texture first).
func (a *App) NewDecoration(kind, name string) (setfmt.Decoration, error) {
	l, err := a.accLib()
	if err != nil {
		return setfmt.Decoration{}, err
	}
	k, ok := setfmt.DecorationKindInfo(kind)
	if !ok {
		return setfmt.Decoration{}, errors.New("unknown decoration kind " + kind)
	}
	if strings.TrimSpace(name) == "" {
		name = "Custom " + k.One
	}
	return setfmt.NewDecoration(kind, l.NewID(name), name), nil
}

// SaveDecoration stores a decoration and its editor state (layout JSON, "" keeps it). iconPNG is a data URL (or raw base64) of an
// icon made in the editor; empty keeps the current one.
func (a *App) SaveDecoration(d setfmt.Decoration, layout, iconPNG string) (AccessoryView, error) {
	l, err := a.accLib()
	if err != nil {
		return AccessoryView{}, err
	}
	if !setfmt.SafeID(d.ID) {
		return AccessoryView{}, errors.New("bad decoration id")
	}
	if _, ok := setfmt.DecorationKindInfo(d.Kind); !ok {
		return AccessoryView{}, errors.New("unknown decoration kind " + d.Kind)
	}
	if iconPNG != "" {
		b, err := decodeDataURL(iconPNG)
		if err != nil {
			return AccessoryView{}, err
		}
		rel, err := l.WriteImage(d.ID, "icon", b)
		if err != nil {
			return AccessoryView{}, err
		}
		d.Icon = rel
	}
	if layout != "" {
		if !json.Valid([]byte(layout)) {
			return AccessoryView{}, errors.New("layout is not valid JSON")
		}
		l.Meta.Layouts[d.ID] = json.RawMessage(layout)
	}
	l.PutDecoration(d)
	return a.saveAndInstall(l)
}

func (a *App) DeleteDecoration(id string) (AccessoryView, error) { return a.DeleteDecorationMany([]string{id}) }

// DeleteDecorationMany removes several decorations at once (one save and install); the catalog keeps a copy first.
func (a *App) DeleteDecorationMany(ids []string) (AccessoryView, error) {
	l, err := a.accLib()
	if err != nil {
		return AccessoryView{}, err
	}
	if err := a.keepInCatalog("", ids, l); err != nil {
		return AccessoryView{}, err
	}
	for _, id := range ids {
		l.DeleteDecoration(id)
	}
	return a.saveAndInstall(l)
}

// DecorationBake is what a bake wrote (library-relative paths for the decoration's mesh / texture fields).
type DecorationBake struct {
	Mesh      string     `json:"mesh"`
	Texture   string     `json:"texture"`
	Triangles int        `json:"triangles"`
	Size      [3]float64 `json:"size"` // model bounds (metres)
}

func decoBake(l interface {
	PutBytes([]byte, string) (string, error)
}, m *figurine.Mesh, texRel string) (DecorationBake, error) {
	meshRel, err := l.PutBytes(figurine.GameOBJ(m), ".obj")
	if err != nil {
		return DecorationBake{}, err
	}
	lo, hi := m.Bounds()
	return DecorationBake{Mesh: meshRel, Texture: texRel, Triangles: m.Triangles(),
		Size: [3]float64{hi[0] - lo[0], hi[1] - lo[1], hi[2] - lo[2]}}, nil
}

// BakePoster builds a poster board from an image (library path, e.g. from PickAccessoryImage): the model and its texture
// (image in its frame) in the shared store.
func (a *App) BakePoster(image string, p decoart.Poster) (DecorationBake, error) {
	l, err := a.accLib()
	if err != nil {
		return DecorationBake{}, err
	}
	img, err := readImage(l.Resolve(image))
	if err != nil {
		return DecorationBake{}, err
	}
	m, tex, err := decoart.BuildPoster(img, p)
	if err != nil {
		return DecorationBake{}, err
	}
	png, err := figurine.PNG(tex)
	if err != nil {
		return DecorationBake{}, err
	}
	texRel, err := l.PutBytes(png, ".png")
	if err != nil {
		return DecorationBake{}, err
	}
	return decoBake(l, m, texRel)
}

// BakeDecorationModel places an imported source model (ImportFigurineModel) for a decoration: rotated and scaled to the height
// (metres) like furniture, then stood on the floor (centred on x/z) or hung on the wall (mount "Wall": facing out, back on the wall,
// centred on the hanging point).
func (a *App) BakeDecorationModel(model, texture, mount string, p figurine.Placement) (DecorationBake, error) {
	l, err := a.accLib()
	if err != nil {
		return DecorationBake{}, err
	}
	src, err := figurine.ReadSource(l.Resolve(model))
	if err != nil {
		return DecorationBake{}, err
	}
	p.Anchor = [3]float64{}
	g, err := figurine.Place(src, p)
	if err != nil {
		return DecorationBake{}, err
	}
	if mount == "Wall" {
		decoart.OnWall(g)
	}
	texRel, err := storedTexture(l, texture)
	if err != nil {
		return DecorationBake{}, errors.New("model texture missing: " + err.Error())
	}
	return decoBake(l, g, texRel)
}

func readImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

package main

import (
	"errors"
	_ "image/jpeg"
	_ "image/png"
	"strings"

	"tcgstudio/internal/decoart"
	"tcgstudio/internal/figurine"
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
	return a.templateJSON(gameextract.DecorationsDir, "decorations.json")
}

// NewDecoration makes a draft with a fresh id; nothing is stored until the page's Save (SaveLibraryDraft).
func (a *App) NewDecoration(kind, name string) (setfmt.Decoration, error) {
	l, err := a.items()
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
	return setfmt.NewDecoration(kind, l.NewID(), name), nil
}

// BakePoster builds a poster board from an image (library path, e.g. from PickAccessoryImage): the model and its texture
// (image in its frame) in the shared store.
func (a *App) BakePoster(image string, p decoart.Poster) (ModelBake, error) {
	l, err := a.items()
	if err != nil {
		return ModelBake{}, err
	}
	img, err := readImage(l.Resolve(image))
	if err != nil {
		return ModelBake{}, err
	}
	m, tex, err := decoart.BuildPoster(img, p)
	if err != nil {
		return ModelBake{}, err
	}
	png, err := figurine.PNG(tex)
	if err != nil {
		return ModelBake{}, err
	}
	texRel, err := l.PutBytes(png, ".png")
	if err != nil {
		return ModelBake{}, err
	}
	return storeBake(l, m, texRel)
}

// BakeDecorationModel places an imported source model (ImportFigurineModel) for a decoration: rotated and scaled to the height
// (metres) like furniture, then stood on the floor (centred on x/z) or hung on the wall (mount "Wall": facing out, back on the wall,
// centred on the hanging point).
func (a *App) BakeDecorationModel(model, texture, mount string, p figurine.Placement) (ModelBake, error) {
	l, err := a.items()
	if err != nil {
		return ModelBake{}, err
	}
	p.Anchor = [3]float64{}
	return bakeModel(l, model, texture, p, func(g *figurine.Mesh) {
		if mount == "Wall" {
			decoart.OnWall(g)
		}
	})
}

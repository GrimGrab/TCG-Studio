package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/figurine"
	"tcgstudio/internal/game"
	"tcgstudio/internal/gameextract"
	"tcgstudio/internal/setfmt"
)

// ---------------------------------------------------------------- custom furniture (accessory library "furniture" list)
//
// Pieces are new furniture built on a vanilla piece of the chosen type (its behaviour). The vanilla pieces — icons, merged
// models and spots — come from the game files (game-templates\furniture\furniture.json, internal/gameextract/furniture.go).

// FurnitureTypes lists the furniture types (behaviours), their default bases and which spots they have.
func (a *App) FurnitureTypes() []setfmt.FurnitureType { return setfmt.FurnitureTypesWithPoints() }

// FurnitureTemplates returns furniture.json of the game templates as raw JSON, or "" while it isn't available.
func (a *App) FurnitureTemplates() string {
	if !game.IsGameDir(a.settings.GameDir) {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(a.templatesDir(), gameextract.FurnitureDir, "furniture.json"))
	if err != nil {
		return ""
	}
	return string(b)
}

// furnitureBases maps each vanilla piece usable as a base to its furniture type (nil while the templates aren't read).
func (a *App) furnitureBases() map[string]string {
	raw := a.FurnitureTemplates()
	if raw == "" {
		return nil
	}
	var t struct {
		Pieces []struct {
			Base string `json:"base"`
			Type string `json:"type"`
		} `json:"pieces"`
	}
	if json.Unmarshal([]byte(raw), &t) != nil {
		return nil
	}
	m := map[string]string{}
	for _, p := range t.Pieces {
		if p.Type != "" {
			m[p.Base] = p.Type
		}
	}
	return m
}

func (a *App) NewFurniture(typ, name, base string) (setfmt.Furniture, error) {
	l, err := a.accLib()
	if err != nil {
		return setfmt.Furniture{}, err
	}
	k, ok := setfmt.FurnitureTypeInfo(typ)
	if !ok {
		return setfmt.Furniture{}, errors.New("unknown furniture type " + typ)
	}
	if strings.TrimSpace(name) == "" {
		name = "Custom " + strings.Title(k.One) //nolint:staticcheck // ASCII names
	}
	if base == k.DefaultBase {
		base = ""
	}
	f := setfmt.Furniture{ID: l.NewID(name), Type: typ, Name: name, Base: base}
	l.PutFurniture(f)
	_, err = a.saveAndInstall(l)
	return f, err
}

// SaveFurniture stores a piece and its editor state (layout JSON: imported model source, rotation, height; "" keeps it). iconPNG is
// a data URL (or raw base64) of an icon made in the editor; empty keeps the current one.
func (a *App) SaveFurniture(f setfmt.Furniture, layout, iconPNG string) (AccessoryView, error) {
	l, err := a.accLib()
	if err != nil {
		return AccessoryView{}, err
	}
	if !setfmt.SafeID(f.ID) {
		return AccessoryView{}, errors.New("bad furniture id")
	}
	if iconPNG != "" {
		b, err := decodeDataURL(iconPNG)
		if err != nil {
			return AccessoryView{}, err
		}
		rel, err := l.WriteImage(f.ID, "icon", b)
		if err != nil {
			return AccessoryView{}, err
		}
		f.Icon = rel
	}
	if layout != "" {
		if !json.Valid([]byte(layout)) {
			return AccessoryView{}, errors.New("layout is not valid JSON")
		}
		l.Meta.Layouts[f.ID] = json.RawMessage(layout)
	}
	l.PutFurniture(f)
	return a.saveAndInstall(l)
}

func (a *App) DeleteFurniture(id string) (AccessoryView, error) {
	l, err := a.accLib()
	if err != nil {
		return AccessoryView{}, err
	}
	if err := a.keepInCatalog("", []string{id}, l); err != nil {
		return AccessoryView{}, err
	}
	l.DeleteFurniture(id)
	return a.saveAndInstall(l)
}

// DeleteFurnitureMany removes several pieces at once (one save and install).
func (a *App) DeleteFurnitureMany(ids []string) (AccessoryView, error) {
	l, err := a.accLib()
	if err != nil {
		return AccessoryView{}, err
	}
	if err := a.keepInCatalog("", ids, l); err != nil {
		return AccessoryView{}, err
	}
	for _, id := range ids {
		l.DeleteFurniture(id)
	}
	return a.saveAndInstall(l)
}

// FurnitureBake is what BakeFurnitureModel wrote (library-relative paths for the piece's mesh / texture fields).
type FurnitureBake struct {
	Mesh      string     `json:"mesh"`
	Texture   string     `json:"texture"`
	Triangles int        `json:"triangles"`
	Size      [3]float64 `json:"size"` // placed model bounds (metres)
}

// BakeFurnitureModel places an imported source model (ImportFigurineModel) in the piece's root space: rotated, scaled to the
// height (metres), bottom-centre on the anchor (usually the base piece's footprint centre), and writes images/<id>_model.obj +
// images/<id>_texture.png.
func (a *App) BakeFurnitureModel(id, model, texture string, p figurine.Placement) (FurnitureBake, error) {
	if !setfmt.SafeID(id) {
		return FurnitureBake{}, errors.New("bad furniture id")
	}
	l, err := a.accLib()
	if err != nil {
		return FurnitureBake{}, err
	}
	src, err := figurine.ReadSource(l.Resolve(model))
	if err != nil {
		return FurnitureBake{}, err
	}
	g, err := figurine.Place(src, p)
	if err != nil {
		return FurnitureBake{}, err
	}
	meshRel, err := l.PutBytes(figurine.GameOBJ(g), ".obj")
	if err != nil {
		return FurnitureBake{}, err
	}
	texRel, err := storedTexture(l, texture)
	if err != nil {
		return FurnitureBake{}, errors.New("model texture missing: " + err.Error())
	}
	lo, hi := g.Bounds()
	return FurnitureBake{Mesh: meshRel, Texture: texRel, Triangles: g.Triangles(),
		Size: [3]float64{hi[0] - lo[0], hi[1] - lo[1], hi[2] - lo[2]}}, nil
}

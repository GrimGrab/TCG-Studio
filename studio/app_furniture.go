package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"tcgstudio/internal/figurine"
	"tcgstudio/internal/game"
	"tcgstudio/internal/gameextract"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/unityfs"
	"tcgstudio/internal/uvmap"
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

// ---------------------------------------------------------------- painting (face editor over the whole piece)

// FurniturePaintInfo is a piece prepared for the face editor: the model (views of the net), the baked look and the parts (meshes =
// template file names). Template names the template for PaintFurniture.
type FurniturePaintInfo struct {
	Template string             `json:"template"`
	Model    uvmap.Model        `json:"model"`
	Vanilla  string             `json:"vanilla"` // URL
	Parts    []setfmt.PaintPart `json:"parts"`
}

// paintMu: one paint template is made at a time (the Furniture tab prepares the selected piece in the background while Paint may ask too).
var paintMu sync.Mutex

// FurniturePaintTemplate prepares a vanilla piece for painting. Made from the game files the first time (about a second), then
// kept with the game templates.
func (a *App) FurniturePaintTemplate(base string) (FurniturePaintInfo, error) {
	paintMu.Lock()
	defer paintMu.Unlock()
	dir, err := a.paintDir()
	if err != nil {
		return FurniturePaintInfo{}, err
	}
	file, err := gameextract.PaintTemplate(a.settings.GameDir, dir, base)
	if err != nil {
		return FurniturePaintInfo{}, err
	}
	return a.paintInfo(file)
}

// FurniturePaintTemplateOwn prepares a piece's own model for painting, placed as it will be saved (BakeFurnitureModel): its
// texture is the base look. Kept per model + texture + placement.
func (a *App) FurniturePaintTemplateOwn(model, texture string, p figurine.Placement) (FurniturePaintInfo, error) {
	paintMu.Lock()
	defer paintMu.Unlock()
	dir, err := a.paintDir()
	if err != nil {
		return FurniturePaintInfo{}, err
	}
	l, err := a.accLib()
	if err != nil {
		return FurniturePaintInfo{}, err
	}
	src, err := figurine.ReadSource(l.Resolve(model))
	if err != nil {
		return FurniturePaintInfo{}, err
	}
	g, err := figurine.Place(src, p)
	if err != nil {
		return FurniturePaintInfo{}, err
	}
	var tex *image.NRGBA
	var texBytes []byte
	if texture != "" {
		if texBytes, err = os.ReadFile(l.Resolve(texture)); err != nil {
			return FurniturePaintInfo{}, errors.New("model texture missing: " + err.Error())
		}
		img, _, err := image.Decode(bytes.NewReader(texBytes))
		if err != nil {
			return FurniturePaintInfo{}, errors.New("model texture: " + err.Error())
		}
		tex = image.NewNRGBA(img.Bounds())
		draw.Draw(tex, tex.Bounds(), img, img.Bounds().Min, draw.Src)
	}
	obj := figurine.GameOBJ(g)
	h := sha256.Sum256(append(append([]byte{}, obj...), texBytes...))
	file, err := gameextract.PaintTemplateFromMesh(dir, "own_"+hex.EncodeToString(h[:8]), ownMesh(g), tex)
	if err != nil {
		return FurniturePaintInfo{}, err
	}
	return a.paintInfo(file)
}

// ownMesh: a placed model (figurine.Mesh: UV origin top-left) as the paint template builder's mesh (UV origin bottom-left).
func ownMesh(g *figurine.Mesh) *unityfs.Mesh {
	m := &unityfs.Mesh{Name: "own"}
	for i, p := range g.Pos {
		m.Pos = append(m.Pos, [3]float32{float32(p[0]), float32(p[1]), float32(p[2])})
		n := g.Nrm[i]
		m.Normal = append(m.Normal, [3]float32{float32(n[0]), float32(n[1]), float32(n[2])})
		uv := [2]float64{}
		if i < len(g.UV) {
			uv = g.UV[i]
		}
		m.UV = append(m.UV, [2]float32{float32(uv[0]), float32(1 - uv[1])})
	}
	m.Subs = [][]uint32{g.Idx}
	return m
}

func (a *App) paintDir() (string, error) {
	if !game.IsGameDir(a.settings.GameDir) {
		return "", errors.New("game folder not set (Settings → Game)")
	}
	dir := a.templatesDir()
	if dir == "" {
		return "", errors.New("the game templates aren't available yet (Settings → Game)")
	}
	return dir, nil
}

func (a *App) paintInfo(file string) (FurniturePaintInfo, error) {
	b, err := os.ReadFile(filepath.Join(a.templatesDir(), gameextract.FurnitureDir, file))
	if err != nil {
		return FurniturePaintInfo{}, err
	}
	var t struct {
		Vanilla string             `json:"vanilla"`
		Parts   []setfmt.PaintPart `json:"parts"`
		Model   uvmap.Model        `json:"model"`
	}
	if err := json.Unmarshal(b, &t); err != nil {
		return FurniturePaintInfo{}, err
	}
	return FurniturePaintInfo{Template: file, Model: t.Model, Vanilla: "/furntemplates/" + t.Vanilla, Parts: t.Parts}, nil
}

// PaintFurniture stores a painted atlas (PNG data URL) and the parts' meshes of a paint template (FurniturePaintInfo.Template) in the
// shared store. A vanilla piece uses the result as its paint field; an own model (one part, renderer "") as its mesh + texture.
func (a *App) PaintFurniture(template, texturePNG string) (setfmt.FurniturePaint, error) {
	if strings.ContainsAny(template, `/\`) || strings.Contains(template, "..") {
		return setfmt.FurniturePaint{}, errors.New("bad paint template")
	}
	info, err := a.paintInfo(template)
	if err != nil {
		return setfmt.FurniturePaint{}, err
	}
	l, err := a.accLib()
	if err != nil {
		return setfmt.FurniturePaint{}, err
	}
	png, err := decodeDataURL(texturePNG)
	if err != nil {
		return setfmt.FurniturePaint{}, err
	}
	p := setfmt.FurniturePaint{}
	if p.Texture, err = l.PutBytes(png, ".png"); err != nil {
		return setfmt.FurniturePaint{}, err
	}
	for _, part := range info.Parts {
		obj, err := os.ReadFile(filepath.Join(a.templatesDir(), gameextract.FurnitureDir, part.Mesh))
		if err != nil {
			return setfmt.FurniturePaint{}, err
		}
		rel, err := l.PutBytes(obj, ".obj")
		if err != nil {
			return setfmt.FurniturePaint{}, err
		}
		p.Parts = append(p.Parts, setfmt.PaintPart{Renderer: part.Renderer, Mesh: rel})
	}
	return p, nil
}

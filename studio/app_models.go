package main

import (
	"encoding/json"
	"errors"
	"image"
	"os"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/figurine"
)

// ModelBake is a baked model in the shared store: what figurines, furniture and decorations save.
type ModelBake struct {
	Mesh      string     `json:"mesh"`
	Texture   string     `json:"texture"`
	Triangles int        `json:"triangles"`
	Size      [3]float64 `json:"size"` // placed model bounds (mesh units = metres for furniture/decorations)
}

// putEditorImage stores an image the editor composed (data URL or raw base64; "" = keep *dst) as id's <suffix> file and
// points *dst at it.
func putEditorImage(l *accessories.Library, id, suffix, data string, dst *string) error {
	if data == "" {
		return nil
	}
	b, err := decodeDataURL(data)
	if err != nil {
		return err
	}
	rel, err := l.WriteImage(id, suffix, b)
	if err != nil {
		return err
	}
	*dst = rel
	return nil
}

// putLayout stores an item's editor state (the editor's own JSON; "" keeps it).
func putLayout(l *accessories.Library, id, layout string) error {
	if layout == "" {
		return nil
	}
	if !json.Valid([]byte(layout)) {
		return errors.New("layout is not valid JSON")
	}
	l.Meta.Layouts[id] = json.RawMessage(layout)
	return nil
}

// placeModel reads an imported source model (ImportFigurineModel) and places it (rotation, scale, anchor).
func placeModel(l *accessories.Library, model string, p figurine.Placement) (*figurine.Mesh, error) {
	src, err := figurine.ReadSource(l.Resolve(model))
	if err != nil {
		return nil, err
	}
	return figurine.Place(src, p)
}

// bakeModel places a source model, lets adjust move it further (nil = no change) and stores mesh + texture.
func bakeModel(l *accessories.Library, model, texture string, p figurine.Placement, adjust func(*figurine.Mesh)) (ModelBake, error) {
	g, err := placeModel(l, model, p)
	if err != nil {
		return ModelBake{}, err
	}
	if adjust != nil {
		adjust(g)
	}
	if _, err := os.Stat(l.Resolve(texture)); err != nil {
		return ModelBake{}, errors.New("model texture missing: " + err.Error())
	}
	return storeBake(l, g, texture)
}

// storeBake writes a placed mesh to the store as the game's OBJ.
func storeBake(l *accessories.Library, m *figurine.Mesh, texRel string) (ModelBake, error) {
	meshRel, err := l.PutBytes(figurine.GameOBJ(m), ".obj")
	if err != nil {
		return ModelBake{}, err
	}
	lo, hi := m.Bounds()
	return ModelBake{Mesh: meshRel, Texture: texRel, Triangles: m.Triangles(),
		Size: [3]float64{hi[0] - lo[0], hi[1] - lo[1], hi[2] - lo[2]}}, nil
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

func readImageConfig(path string) (image.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Config{}, err
	}
	defer f.Close()
	c, _, err := image.DecodeConfig(f)
	return c, err
}

package main

import (
	"errors"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/figurine"
	"tcgstudio/internal/setfmt"
)

// ---------------------------------------------------------------- figurines (own 3D model + texture)
//
// Import converts a picked model into an editable source (images/src/<name>.fig.obj + .png, right-handed Y-up) that the
// editor shows; Bake places that source in the base toy's mesh space (rotation, height, anchor chosen in the editor) and
// stores the game's model and its texture in the shared store for the mod.

var modelFilters = []runtime.FileFilter{
	{DisplayName: "3D models (*.glb;*.gltf;*.obj)", Pattern: "*.glb;*.gltf;*.obj"},
	{DisplayName: "FBX (convert to .glb first)", Pattern: "*.fbx"},
}

// FigurineSource is an imported model ready for the editor.
type FigurineSource struct {
	Model     string     `json:"model"`   // library-relative source OBJ
	Texture   string     `json:"texture"` // library-relative source PNG
	Name      string     `json:"name"`    // file name without extension (a default figurine name)
	Triangles int        `json:"triangles"`
	Vertices  int        `json:"vertices"`
	Size      [3]float64 `json:"size"` // bounds of the source model (its own units)
	Warnings  []string   `json:"warnings"`
	File      string     `json:"file"` // the picked model file (to import it again with textures the user picks)
}

// ModelTextures are textures the user picked for a model (library paths from PickAccessoryImage, "" = none): for models shared
// without their material, or with maps the game can't draw. Normal map and ambient occlusion are baked into the colour texture.
type ModelTextures struct {
	Color         string `json:"color"`
	Normal        string `json:"normal"`
	NormalDirectX bool   `json:"normalDirectX"` // the user says the normal map is DirectX style (green down)
	AO            string `json:"ao"`
}

// ImportFigurineModel lets the user pick a .glb/.gltf/.obj file and converts it. Returns an empty Model when cancelled.
func (a *App) ImportFigurineModel() (FigurineSource, error) {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose a 3D model", Filters: modelFilters})
	if err != nil || file == "" {
		return FigurineSource{}, err
	}
	return a.importFigurineWith(file, nil)
}

// ImportModelWithTextures imports a model file again with the textures the user picked (ModelTextures); no textures = as is.
func (a *App) ImportModelWithTextures(file string, t ModelTextures) (FigurineSource, error) {
	if _, err := os.Stat(file); err != nil {
		return FigurineSource{}, errors.New("the model file isn't there any more (" + filepath.Base(file) + ") — import the model again")
	}
	l, err := a.items()
	if err != nil {
		return FigurineSource{}, err
	}
	load := func(rel string) (image.Image, error) {
		if rel == "" {
			return nil, nil
		}
		img, err := readImage(l.Resolve(rel))
		if err != nil {
			return nil, errors.New("texture couldn't be read: " + err.Error())
		}
		return img, nil
	}
	set := &figurine.TextureSet{NormalDirectX: t.NormalDirectX}
	if set.Color, err = load(t.Color); err != nil {
		return FigurineSource{}, err
	}
	if set.Normal, err = load(t.Normal); err != nil {
		return FigurineSource{}, err
	}
	if set.AO, err = load(t.AO); err != nil {
		return FigurineSource{}, err
	}
	return a.importFigurineWith(file, set)
}

func (a *App) importFigurineWith(file string, set *figurine.TextureSet) (FigurineSource, error) {
	if strings.EqualFold(filepath.Ext(file), ".fbx") {
		return FigurineSource{}, errors.New("FBX isn't supported yet — open it in Blender (File → Import → FBX) and export it as glTF 2.0 (.glb), then import that")
	}
	m, img, warns, err := figurine.ImportWith(file, set)
	if err != nil {
		return FigurineSource{}, err
	}
	l, err := a.items()
	if err != nil {
		return FigurineSource{}, err
	}
	name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	model, err := l.PutBytes(figurine.SourceOBJ(m), ".obj")
	if err != nil {
		return FigurineSource{}, err
	}
	png, err := figurine.PNG(img)
	if err != nil {
		return FigurineSource{}, err
	}
	texture, err := l.PutBytes(png, ".png")
	if err != nil {
		return FigurineSource{}, err
	}
	lo, hi := m.Bounds()
	if warns == nil {
		warns = []string{}
	}
	return FigurineSource{
		Model: model, Texture: texture, Name: name, Triangles: m.Triangles(), Vertices: len(m.Pos),
		Size: [3]float64{hi[0] - lo[0], hi[1] - lo[1], hi[2] - lo[2]}, Warnings: warns, File: file,
	}, nil
}

// BakeFigurine places the source model (and copies its texture, or a replacement image) for the accessory id.
func (a *App) BakeFigurine(id, model, texture string, p figurine.Placement) (ModelBake, error) {
	if !setfmt.SafeID(id) {
		return ModelBake{}, errors.New("bad accessory id")
	}
	l, err := a.items()
	if err != nil {
		return ModelBake{}, err
	}
	return bakeModel(l, model, texture, p, nil)
}

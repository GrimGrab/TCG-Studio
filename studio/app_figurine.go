package main

import (
	"errors"
	"image"
	"os"
	"path/filepath"
	"strings"
	astore "tcgstudio/internal/assets"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/figurine"
	"tcgstudio/internal/setfmt"
)

// ---------------------------------------------------------------- figurines (own 3D model + texture)
//
// Import converts a picked model into an editable source (images/src/<name>.fig.obj + .png, right-handed Y-up) that the
// editor shows; Bake places that source in the base toy's mesh space (rotation, height, anchor chosen in the editor) and
// writes images/<id>_model.obj + images/<id>_texture.png for the mod.

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
	return a.importFigurine(file)
}

func (a *App) importFigurine(file string) (FigurineSource, error) { return a.importFigurineWith(file, nil) }

// ImportModelWithTextures imports a model file again with the textures the user picked (ModelTextures); no textures = as is.
func (a *App) ImportModelWithTextures(file string, t ModelTextures) (FigurineSource, error) {
	if _, err := os.Stat(file); err != nil {
		return FigurineSource{}, errors.New("the model file isn't there any more (" + filepath.Base(file) + ") — import the model again")
	}
	l, err := a.accLib()
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
	l, err := a.accLib()
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

// FigurineBake is what BakeFigurine wrote (library-relative paths for the accessory's mesh / texture fields).
type FigurineBake struct {
	Mesh      string     `json:"mesh"`
	Texture   string     `json:"texture"`
	Triangles int        `json:"triangles"`
	Size      [3]float64 `json:"size"` // placed model bounds (mesh units)
}

// BakeFigurine places the source model (and copies its texture, or a replacement image) for the accessory id.
func (a *App) BakeFigurine(id, model, texture string, p figurine.Placement) (FigurineBake, error) {
	if !setfmt.SafeID(id) {
		return FigurineBake{}, errors.New("bad accessory id")
	}
	l, err := a.accLib()
	if err != nil {
		return FigurineBake{}, err
	}
	src, err := figurine.ReadSource(l.Resolve(model))
	if err != nil {
		return FigurineBake{}, err
	}
	g, err := figurine.Place(src, p)
	if err != nil {
		return FigurineBake{}, err
	}
	meshRel, err := l.PutBytes(figurine.GameOBJ(g), ".obj")
	if err != nil {
		return FigurineBake{}, err
	}
	texRel, err := storedTexture(l, texture)
	if err != nil {
		return FigurineBake{}, errors.New("figurine texture missing: " + err.Error())
	}
	lo, hi := g.Bounds()
	return FigurineBake{Mesh: meshRel, Texture: texRel, Triangles: g.Triangles(),
		Size: [3]float64{hi[0] - lo[0], hi[1] - lo[1], hi[2] - lo[2]}}, nil
}

// storedTexture is a model texture's path in the shared store: the texture itself when it's already there, else a stored
// copy of the setup's own file.
func storedTexture(l *accessories.Library, texture string) (string, error) {
	if astore.IsAsset(texture) {
		if _, err := os.Stat(l.Resolve(texture)); err != nil {
			return "", err
		}
		return texture, nil
	}
	b, err := os.ReadFile(l.Resolve(texture))
	if err != nil {
		return "", err
	}
	return l.PutBytes(b, filepath.Ext(texture))
}

func safeFileName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?* `, r) {
			return '_'
		}
		return r
	}, s)
	if s == "" {
		s = "model"
	}
	return s
}

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

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
}

// ImportFigurineModel lets the user pick a .glb/.gltf/.obj file and converts it. Returns an empty Model when cancelled.
func (a *App) ImportFigurineModel() (FigurineSource, error) {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose a 3D model", Filters: modelFilters})
	if err != nil || file == "" {
		return FigurineSource{}, err
	}
	return a.importFigurine(file)
}

func (a *App) importFigurine(file string) (FigurineSource, error) {
	if strings.EqualFold(filepath.Ext(file), ".fbx") {
		return FigurineSource{}, errors.New("FBX isn't supported yet — open it in Blender (File → Import → FBX) and export it as glTF 2.0 (.glb), then import that")
	}
	m, img, warns, err := figurine.Import(file)
	if err != nil {
		return FigurineSource{}, err
	}
	l, err := a.accLib()
	if err != nil {
		return FigurineSource{}, err
	}
	name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	rel := l.FreeSourceBase(safeFileName(name) + ".fig")
	if err := figurine.WriteSource(filepath.Join(l.Folder, filepath.FromSlash(rel)), m, img); err != nil {
		return FigurineSource{}, err
	}
	lo, hi := m.Bounds()
	if warns == nil {
		warns = []string{}
	}
	return FigurineSource{
		Model: rel + ".obj", Texture: rel + ".png", Name: name, Triangles: m.Triangles(), Vertices: len(m.Pos),
		Size: [3]float64{hi[0] - lo[0], hi[1] - lo[1], hi[2] - lo[2]}, Warnings: warns,
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
	src, err := figurine.ReadSource(libPath(l, model))
	if err != nil {
		return FigurineBake{}, err
	}
	g, err := figurine.Place(src, p)
	if err != nil {
		return FigurineBake{}, err
	}
	meshRel := accessories.ImagesDir + "/" + id + "_model.obj"
	if err := figurine.WriteGame(libPath(l, meshRel), g); err != nil {
		return FigurineBake{}, err
	}
	tex, err := os.ReadFile(libPath(l, texture))
	if err != nil {
		return FigurineBake{}, errors.New("figurine texture missing: " + err.Error())
	}
	texRel, err := l.WriteImage(id, "texture", tex)
	if err != nil {
		return FigurineBake{}, err
	}
	lo, hi := g.Bounds()
	return FigurineBake{Mesh: meshRel, Texture: texRel, Triangles: g.Triangles(),
		Size: [3]float64{hi[0] - lo[0], hi[1] - lo[1], hi[2] - lo[2]}}, nil
}

// libPath resolves a library-relative path, refusing anything outside the library folder.
func libPath(l *accessories.Library, rel string) string {
	p := filepath.Join(l.Folder, filepath.FromSlash(rel))
	if r, err := filepath.Rel(l.Folder, p); err != nil || strings.HasPrefix(r, "..") {
		return filepath.Join(l.Folder, "invalid")
	}
	return p
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

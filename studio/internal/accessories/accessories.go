// Package accessories manages the global accessory library (custom deck boxes and playmats): a workspace folder
// <workspace>\accessories\ with accessories.json (the mod's format, see setfmt.AccessoryLibrary), studio.json (editor
// layouts, studio-only) and images\, installed as a whole to <plugin>\Accessories\.
package accessories

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"tcgstudio/internal/game"
	"tcgstudio/internal/setfmt"
)

const (
	LibraryFile = "accessories.json"
	MetaFile    = "studio.json"
	ImagesDir   = "images"
	// InstallDir is the folder next to the mod DLL that the mod's AccessoryLoader reads.
	InstallDir = "Accessories"
)

// Meta is studio-only data: the editor layout (layers, per-face settings) of each accessory, kept as raw JSON so the
// frontend owns its format.
type Meta struct {
	Layouts map[string]json.RawMessage `json:"layouts"`
}

type Library struct {
	Folder string                   `json:"folder"`
	Lib    *setfmt.AccessoryLibrary `json:"library"`
	Meta   *Meta                    `json:"meta"`
}

func Folder(workspaceRoot string) string { return filepath.Join(workspaceRoot, "accessories") }

// InstalledDir is <plugin>\Accessories for a game folder.
func InstalledDir(gameDir string) string { return filepath.Join(game.PluginDir(gameDir), InstallDir) }

// Open loads the library, or returns an empty one when none exists yet.
func Open(workspaceRoot string) (*Library, error) {
	l := &Library{Folder: Folder(workspaceRoot), Lib: &setfmt.AccessoryLibrary{SchemaVersion: setfmt.AccessorySchemaVersion}, Meta: &Meta{}}
	if lib, err := setfmt.LoadAccessories(filepath.Join(l.Folder, LibraryFile)); err == nil {
		l.Lib = lib
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if b, err := os.ReadFile(filepath.Join(l.Folder, MetaFile)); err == nil {
		_ = json.Unmarshal(b, l.Meta)
	}
	if l.Meta.Layouts == nil {
		l.Meta.Layouts = map[string]json.RawMessage{}
	}
	if l.Lib.Accessories == nil {
		l.Lib.Accessories = []setfmt.Accessory{}
	}
	return l, nil
}

func (l *Library) Save() error {
	if err := l.Lib.Save(filepath.Join(l.Folder, LibraryFile)); err != nil {
		return err
	}
	b, err := json.MarshalIndent(l.Meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(l.Folder, MetaFile), b, 0o644)
}

func (l *Library) Index(id string) int {
	for i, a := range l.Lib.Accessories {
		if a.ID == id {
			return i
		}
	}
	return -1
}

var nonID = regexp.MustCompile(`[^a-z0-9_-]+`)

// NewID makes a unique safe id from a name ("Dragon Mat" → "dragon-mat", "dragon-mat-2", …).
func (l *Library) NewID(name string) string {
	base := strings.Trim(nonID.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if base == "" {
		base = "accessory"
	}
	id := base
	// Accessories and furniture share images/<id>_*, so an id is unique across both lists.
	for n := 2; l.Index(id) >= 0 || l.FurnitureIndex(id) >= 0; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}

// Put inserts or replaces an accessory (matched by id).
func (l *Library) Put(a setfmt.Accessory) {
	if i := l.Index(a.ID); i >= 0 {
		l.Lib.Accessories[i] = a
		return
	}
	l.Lib.Accessories = append(l.Lib.Accessories, a)
}

// Delete removes an accessory, its layout and its images/<id>_* files (texture, icon, figurine model).
func (l *Library) Delete(id string) {
	if i := l.Index(id); i >= 0 {
		l.Lib.Accessories = append(l.Lib.Accessories[:i], l.Lib.Accessories[i+1:]...)
	}
	delete(l.Meta.Layouts, id)
	if matches, _ := filepath.Glob(filepath.Join(l.Folder, ImagesDir, id+"_*")); matches != nil {
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}
}

// FurnitureIndex is the position of a furniture piece in the library (-1 = none).
func (l *Library) FurnitureIndex(id string) int {
	for i, f := range l.Lib.Furniture {
		if f.ID == id {
			return i
		}
	}
	return -1
}

// PutFurniture inserts or replaces a furniture piece (matched by id).
func (l *Library) PutFurniture(f setfmt.Furniture) {
	if i := l.FurnitureIndex(f.ID); i >= 0 {
		l.Lib.Furniture[i] = f
		return
	}
	l.Lib.Furniture = append(l.Lib.Furniture, f)
}

// DeleteFurniture removes a piece and its images/<id>_* files (icon, texture, model).
func (l *Library) DeleteFurniture(id string) {
	if i := l.FurnitureIndex(id); i >= 0 {
		l.Lib.Furniture = append(l.Lib.Furniture[:i], l.Lib.Furniture[i+1:]...)
	}
	delete(l.Meta.Layouts, id)
	if matches, _ := filepath.Glob(filepath.Join(l.Folder, ImagesDir, id+"_*")); matches != nil {
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}
}

// Move reorders an accessory (library order = gamify ladder order).
func (l *Library) Move(id string, to int) {
	i := l.Index(id)
	if i < 0 || to < 0 || to >= len(l.Lib.Accessories) || to == i {
		return
	}
	a := l.Lib.Accessories[i]
	list := append(l.Lib.Accessories[:i:i], l.Lib.Accessories[i+1:]...)
	list = append(list[:to], append([]setfmt.Accessory{a}, list[to:]...)...)
	l.Lib.Accessories = list
}

// WriteImage stores generated image bytes as images/<id>_<suffix>.png and returns the relative path.
func (l *Library) WriteImage(id, suffix string, png []byte) (string, error) {
	rel := ImagesDir + "/" + id + "_" + suffix + ".png"
	p := filepath.Join(l.Folder, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	return rel, os.WriteFile(p, png, 0o644)
}

// CopySource copies a user-picked file into images/src/ (kept for re-editing) and returns the relative path.
func (l *Library) CopySource(src string) (string, error) {
	rel, dst := l.freeSource(filepath.Base(src))
	return rel, copyFile(src, dst)
}

// WriteSource stores image bytes made in the studio in images/src/ under a free name based on name.
func (l *Library) WriteSource(name string, b []byte) (string, error) {
	rel, dst := l.freeSource(strings.ReplaceAll(filepath.Base(name), " ", "_"))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	return rel, os.WriteFile(dst, b, 0o644)
}

// FreeSourceBase returns images/src/<base> (no extension) such that neither <base>.obj nor <base>.png exists yet (figurine sources).
func (l *Library) FreeSourceBase(base string) string {
	for n := 1; ; n++ {
		name := base
		if n > 1 {
			name = fmt.Sprintf("%s-%d", strings.TrimSuffix(base, ".fig"), n)
			if strings.HasSuffix(base, ".fig") {
				name += ".fig"
			}
		}
		rel := ImagesDir + "/src/" + name
		p := filepath.Join(l.Folder, filepath.FromSlash(rel))
		_, e1 := os.Stat(p + ".obj")
		_, e2 := os.Stat(p + ".png")
		if os.IsNotExist(e1) && os.IsNotExist(e2) {
			return rel
		}
	}
}

func (l *Library) freeSource(name string) (rel, dst string) {
	rel = ImagesDir + "/src/" + name
	dst = filepath.Join(l.Folder, filepath.FromSlash(rel))
	for n := 2; ; n++ {
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			return rel, dst
		}
		ext := filepath.Ext(name)
		rel = fmt.Sprintf("%s/src/%s-%d%s", ImagesDir, strings.TrimSuffix(name, ext), n, ext)
		dst = filepath.Join(l.Folder, filepath.FromSlash(rel))
	}
}

// Install replaces <plugin>\Accessories with accessories.json and every referenced image/model (accessories and furniture). An
// empty library uninstalls.
func (l *Library) Install(gameDir string) error {
	if !game.IsGameDir(gameDir) {
		return fmt.Errorf("game folder not set")
	}
	if len(l.Lib.Accessories) == 0 && len(l.Lib.Furniture) == 0 {
		return Uninstall(gameDir)
	}
	dest := InstalledDir(gameDir)
	tmp := dest + ".installing"
	_ = os.RemoveAll(tmp)
	for _, a := range l.Lib.Accessories {
		for _, rel := range []string{a.Texture, a.Icon, a.Mesh} {
			if rel == "" {
				continue
			}
			if err := copyFile(filepath.Join(l.Folder, filepath.FromSlash(rel)), filepath.Join(tmp, filepath.FromSlash(rel))); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	for _, f := range l.Lib.Furniture {
		for _, rel := range []string{f.Texture, f.Icon, f.Mesh} {
			if rel == "" {
				continue
			}
			if err := copyFile(filepath.Join(l.Folder, filepath.FromSlash(rel)), filepath.Join(tmp, filepath.FromSlash(rel))); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	if err := l.Lib.Save(filepath.Join(tmp, LibraryFile)); err != nil {
		return err
	}
	_ = os.RemoveAll(dest)
	return os.Rename(tmp, dest)
}

func Uninstall(gameDir string) error {
	if !game.IsGameDir(gameDir) {
		return fmt.Errorf("game folder not set")
	}
	return os.RemoveAll(InstalledDir(gameDir))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

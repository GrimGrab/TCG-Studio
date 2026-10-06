// Package accessories manages a setup's accessory library (custom deck boxes, playmats, figurines, furniture…): the
// game info in <setup>\accessories\ — accessories.json (the mod's format, see setfmt.AccessoryLibrary) and studio.json
// (editor layouts, studio-only) — and its files in the workspace's shared asset store (internal/assets), referred to as
// "assets/<hash>.<ext>". Older libraries keep per-setup files under images\ (paths without the assets/ prefix) until the
// Storage page moves them. Installed to <plugin>\Accessories\ (store files under Accessories\assets\).
package accessories

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"tcgstudio/internal/assets"
	"tcgstudio/internal/game"
	"tcgstudio/internal/origin"
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
	// Origins: where converted accessories and furniture came from (by id).
	Origins map[string]origin.Origin `json:"origins,omitempty"`
}

type Library struct {
	Folder string                   `json:"folder"`
	Lib    *setfmt.AccessoryLibrary `json:"library"`
	Meta   *Meta                    `json:"meta"`
	Assets assets.Store             `json:"-"` // the workspace's shared store, where new files go
}

func Folder(workspaceRoot string) string { return filepath.Join(workspaceRoot, "accessories") }

// InstalledDir is <plugin>\Accessories for a game folder.
func InstalledDir(gameDir string) string { return filepath.Join(game.PluginDir(gameDir), InstallDir) }

// Open loads a setup's library (root = the setup folder), or returns an empty one when none exists yet. store is the
// workspace's shared asset store.
func Open(root string, store assets.Store) (*Library, error) {
	l := &Library{Folder: Folder(root), Lib: &setfmt.AccessoryLibrary{SchemaVersion: setfmt.AccessorySchemaVersion}, Meta: &Meta{},
		Assets: store}
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
	if l.Meta.Origins == nil {
		l.Meta.Origins = map[string]origin.Origin{}
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

// ConvertedPrefix starts the ids of accessories and furniture converted from another mod (EPL): their art belongs to that
// mod's authors, so they stay on this PC (setup exports leave them out).
const ConvertedPrefix = "eplmod-"

// IsConverted reports an accessory or furniture id made by a mod conversion.
func IsConverted(id string) bool { return strings.HasPrefix(id, ConvertedPrefix) }

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
	delete(l.Meta.Origins, id)
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
	delete(l.Meta.Origins, id)
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

// Resolve is the file a library path refers to: the shared store for "assets/…", else the setup's own folder (older
// files). Paths that would leave the folder resolve to a file that doesn't exist.
func (l *Library) Resolve(rel string) string {
	if assets.IsAsset(rel) {
		return l.Assets.Path(rel)
	}
	p := filepath.Join(l.Folder, filepath.FromSlash(rel))
	if r, err := filepath.Rel(l.Folder, p); err != nil || strings.HasPrefix(r, "..") {
		return filepath.Join(l.Folder, "invalid")
	}
	return p
}

// PutBytes stores file bytes in the shared store and returns their path ("assets/<hash>.<ext>"); ext like ".png".
func (l *Library) PutBytes(b []byte, ext string) (string, error) { return l.Assets.Put(b, ext) }

// WriteImage stores a texture, icon or model made for an accessory (PNG bytes) in the shared store. id and suffix are
// kept for the callers' readability: store names come from the content, so an unchanged image costs nothing.
func (l *Library) WriteImage(id, suffix string, png []byte) (string, error) {
	return l.Assets.Put(png, ".png")
}

// CopySource stores a user-picked file (layer image, painted texture, replacement texture…) and returns its path.
func (l *Library) CopySource(src string) (string, error) { return l.Assets.PutFile(src) }

// WriteSource stores image bytes made in the studio (straightened photos…); name only gives the extension.
func (l *Library) WriteSource(name string, b []byte) (string, error) {
	ext := filepath.Ext(name)
	if ext == "" {
		ext = ".png"
	}
	return l.Assets.Put(b, ext)
}

// Files lists every file path the library refers to (accessories.json fields and the editor layouts), without duplicates.
func (l *Library) Files() []string { return l.FilesWhere(nil) }

// FilesWhere is Files for the accessories and furniture whose id keep accepts (nil = all).
func (l *Library) FilesWhere(keep func(id string) bool) []string {
	ok := func(id string) bool { return keep == nil || keep(id) }
	seen := map[string]bool{}
	var out []string
	add := func(rel string) {
		if rel != "" && !seen[rel] {
			seen[rel] = true
			out = append(out, rel)
		}
	}
	for _, a := range l.Lib.Accessories {
		if ok(a.ID) {
			add(a.Texture)
			add(a.Icon)
			add(a.Mesh)
		}
	}
	for _, f := range l.Lib.Furniture {
		if ok(f.ID) {
			add(f.Texture)
			add(f.Icon)
			add(f.Mesh)
		}
	}
	for id, raw := range l.Meta.Layouts {
		if ok(id) {
			for _, rel := range layoutFiles(raw) {
				add(rel)
			}
		}
	}
	return out
}

// layoutFiles finds the file paths in an editor layout: painted texture, layer images (and their straighten sources),
// figurine/furniture model and texture.
func layoutFiles(raw json.RawMessage) []string {
	var l struct {
		TextureFile string `json:"textureFile"`
		Model       string `json:"model"`
		Texture     string `json:"texture"`
		Layers      []struct {
			Src        string `json:"src"`
			Straighten *struct {
				Src string `json:"src"`
			} `json:"straighten"`
		} `json:"layers"`
	}
	if json.Unmarshal(raw, &l) != nil {
		return nil
	}
	out := []string{l.TextureFile, l.Model, l.Texture}
	for _, ly := range l.Layers {
		out = append(out, ly.Src)
		if ly.Straighten != nil {
			out = append(out, ly.Straighten.Src)
		}
	}
	return out
}

// Install replaces <plugin>\Accessories with accessories.json and every referenced image/model (accessories and furniture),
// each at its library path (store files under Accessories\assets\, which the mod reads like any other path). Store files
// never change, so the ones the previous install already has are moved over instead of copied. An empty library
// uninstalls.
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
	var rels []string
	for _, a := range l.Lib.Accessories {
		rels = append(rels, a.Texture, a.Icon, a.Mesh)
	}
	for _, f := range l.Lib.Furniture {
		rels = append(rels, f.Texture, f.Icon, f.Mesh)
	}
	done := map[string]bool{}
	for _, rel := range rels {
		if rel == "" || done[rel] {
			continue
		}
		done[rel] = true
		to := filepath.Join(tmp, filepath.FromSlash(rel))
		if assets.IsAsset(rel) {
			if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
				return err
			}
			if os.Rename(filepath.Join(dest, filepath.FromSlash(rel)), to) == nil {
				continue
			}
		}
		if err := copyFile(l.Resolve(rel), to); err != nil && !os.IsNotExist(err) {
			return err
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

// Package accessories manages a setup's accessory library (custom deck boxes, playmats, figurines, furniture…): the
// game info in <setup>\accessories\ — accessories.json (the mod's format, see setfmt.AccessoryLibrary) and studio.json
// (editor layouts, studio-only) — and its files in the workspace's shared asset store (internal/assets), referred to as
// "assets/<hash>.<ext>". Older libraries keep per-setup files under images\ (paths without the assets/ prefix) until the
// Storage page moves them. The game's mod reads the setup's accessories.json and the store straight from the workspace.
package accessories

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/assets"
	"tcgstudio/internal/game"
	"tcgstudio/internal/ids"
	"tcgstudio/internal/origin"
	"tcgstudio/internal/setfmt"
)

const (
	LibraryFile = "accessories.json"
	MetaFile    = "studio.json"
	// InstallDir is the folder next to the mod DLL older Studio versions copied the library into (the mod now reads the
	// setup's own accessories folder; the upgrade takes old copies over).
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
	return l.SaveMeta()
}

// SaveMeta writes only the studio-only data (the game never reads it).
func (l *Library) SaveMeta() error {
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

// FillConvertedOrigins gives converted entries without an origin (converted before origins were recorded) one with only
// the kind known. Reports whether it added any.
func (l *Library) FillConvertedOrigins() bool {
	var ids []string
	for _, a := range l.Lib.Accessories {
		ids = append(ids, a.ID)
	}
	for _, f := range l.Lib.Furniture {
		ids = append(ids, f.ID)
	}
	for _, d := range l.Lib.Decorations {
		ids = append(ids, d.ID)
	}
	changed := false
	for _, id := range ids {
		if _, ok := l.Meta.Origins[id]; !ok && IsConverted(id) {
			l.Meta.Origins[id] = origin.Origin{Kind: origin.KindEPL}
			changed = true
		}
	}
	return changed
}

// NewID makes a random id for a new accessory, furniture piece or decoration (internal/ids: unique across players and
// setups, never derived from the name), free in every list of this library.
func (l *Library) NewID() string {
	return ids.Free(l.HasItem)
}

// Put inserts or replaces an accessory (matched by id).
func (l *Library) Put(a setfmt.Accessory) {
	if i := l.Index(a.ID); i >= 0 {
		l.Lib.Accessories[i] = a
		return
	}
	l.Lib.Accessories = append(l.Lib.Accessories, a)
}

// Delete removes an accessory and its layout. Its files stay in the shared store (other setups may use them; the
// Storage page deletes unused ones).
func (l *Library) Delete(id string) {
	if i := l.Index(id); i >= 0 {
		l.Lib.Accessories = append(l.Lib.Accessories[:i], l.Lib.Accessories[i+1:]...)
	}
	delete(l.Meta.Layouts, id)
	delete(l.Meta.Origins, id)
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

// DeleteFurniture removes a piece and its layout (files: see Delete).
func (l *Library) DeleteFurniture(id string) {
	if i := l.FurnitureIndex(id); i >= 0 {
		l.Lib.Furniture = append(l.Lib.Furniture[:i], l.Lib.Furniture[i+1:]...)
	}
	delete(l.Meta.Layouts, id)
	delete(l.Meta.Origins, id)
}

// HasItem reports an accessory, furniture piece or decoration with this id.
func (l *Library) HasItem(id string) bool {
	return l.Index(id) >= 0 || l.FurnitureIndex(id) >= 0 || l.DecorationIndex(id) >= 0
}

// DecorationIndex is the position of a decoration in the library (-1 = none).
func (l *Library) DecorationIndex(id string) int {
	for i, d := range l.Lib.Decorations {
		if d.ID == id {
			return i
		}
	}
	return -1
}

// PutDecoration inserts or replaces a decoration (matched by id).
func (l *Library) PutDecoration(d setfmt.Decoration) {
	if i := l.DecorationIndex(d.ID); i >= 0 {
		l.Lib.Decorations[i] = d
		return
	}
	l.Lib.Decorations = append(l.Lib.Decorations, d)
}

// DeleteDecoration removes a decoration and its layout (files: see Delete).
func (l *Library) DeleteDecoration(id string) {
	if i := l.DecorationIndex(id); i >= 0 {
		l.Lib.Decorations = append(l.Lib.Decorations[:i], l.Lib.Decorations[i+1:]...)
	}
	delete(l.Meta.Layouts, id)
	delete(l.Meta.Origins, id)
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

// Resolve is the file a library path ("assets/…") refers to in the shared store. Any other path resolves to a file
// that doesn't exist.
func (l *Library) Resolve(rel string) string {
	if assets.IsAsset(rel) {
		return l.Assets.Path(rel)
	}
	return filepath.Join(l.Assets.Dir, "invalid")
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
			for _, rel := range f.Files() {
				add(rel)
			}
		}
	}
	for _, d := range l.Lib.Decorations {
		if ok(d.ID) {
			for _, rel := range d.Files() {
				add(rel)
			}
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
// figurine/furniture model and texture, and those of a furniture piece's paint layout.
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
		Paint json.RawMessage `json:"paint"`
	}
	if json.Unmarshal(raw, &l) != nil {
		return nil
	}
	out := []string{l.TextureFile, l.Model, l.Texture}
	if len(l.Paint) > 0 {
		out = append(out, layoutFiles(l.Paint)...)
	}
	for _, ly := range l.Layers {
		out = append(out, ly.Src)
		if ly.Straighten != nil {
			out = append(out, ly.Straighten.Src)
		}
	}
	return out
}

// StoreRefs is every shared-store file the accessory libraries of the given setup roots (setups, the catalog) refer to,
// in entries or editor layouts.
func StoreRefs(roots ...string) map[string]bool {
	refs := map[string]bool{}
	for _, root := range roots {
		dir := Folder(root)
		for _, f := range []string{LibraryFile, MetaFile} {
			if b, err := os.ReadFile(filepath.Join(dir, f)); err == nil {
				for _, r := range assets.Refs(b) {
					refs[r] = true
				}
			}
		}
	}
	return refs
}

// Kinds of library entries.
const (
	KindAccessory  = "accessory"
	KindFurniture  = "furniture"
	KindDecoration = "decoration"
)

// Entry finds id among the library's accessories, furniture and decorations: its kind and a copy of the entry.
func (l *Library) Entry(id string) (kind string, entry any, ok bool) {
	if i := l.Index(id); i >= 0 {
		return KindAccessory, l.Lib.Accessories[i], true
	}
	if i := l.FurnitureIndex(id); i >= 0 {
		return KindFurniture, l.Lib.Furniture[i], true
	}
	if i := l.DecorationIndex(id); i >= 0 {
		return KindDecoration, l.Lib.Decorations[i], true
	}
	return "", nil, false
}

// ItemRefs is the shared-store files entry id refers to (the entry and its editor layout).
func (l *Library) ItemRefs(id string) []string {
	_, entry, ok := l.Entry(id)
	if !ok {
		return nil
	}
	var refs []string
	if b, err := json.Marshal(entry); err == nil {
		refs = append(refs, assets.Refs(b)...)
	}
	if raw, ok := l.Meta.Layouts[id]; ok {
		refs = append(refs, assets.Refs(raw)...)
	}
	return refs
}

// CopyEntry copies entry id with its editor layout and origin from src into l. False when src has no such entry.
func (l *Library) CopyEntry(src *Library, id string) bool {
	_, entry, ok := src.Entry(id)
	if !ok {
		return false
	}
	switch e := entry.(type) {
	case setfmt.Accessory:
		l.Put(e)
	case setfmt.Furniture:
		l.PutFurniture(e)
	case setfmt.Decoration:
		l.PutDecoration(e)
	}
	if raw, ok := src.Meta.Layouts[id]; ok {
		l.Meta.Layouts[id] = raw
	}
	if o, ok := src.Meta.Origins[id]; ok {
		l.Meta.Origins[id] = o
	}
	return true
}

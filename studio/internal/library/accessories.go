package library

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/catalog"
	"tcgstudio/internal/setups"
)

// Accessory and furniture files: the shared asset store (<workspace>\assets, see internal/assets) and each setup's own
// older files (accessories\images\…), which the player can move into the store from the Storage page.

// AccessoryStorage is the Storage page's accessories section.
type AccessoryStorage struct {
	StoreBytes  int64          `json:"storeBytes"`
	StoreFiles  int            `json:"storeFiles"`
	Setups      []AccessorySet `json:"setups"`      // per setup: its own files the move would take
	MoveBytes   int64          `json:"moveBytes"`   // all setups
	MoveFiles   int            `json:"moveFiles"`   //
	UnusedBytes int64          `json:"unusedBytes"` // store files no setup refers to
	UnusedFiles int            `json:"unusedFiles"`
	list        []MoveFile     // the files of MoveFiles (Report.MoveList)
}

type AccessorySet struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Files int    `json:"files"`
	Bytes int64  `json:"bytes"`
	// Leftovers: files in the setup's accessories\images that nothing refers to (e.g. earlier versions of re-imported
	// models); the move deletes them.
	LeftoverFiles int   `json:"leftoverFiles"`
	LeftoverBytes int64 `json:"leftoverBytes"`
}

// leftovers lists the files under a library's images folder that no entry or layout refers to.
func leftovers(l *accessories.Library) []string {
	used := map[string]bool{}
	for _, rel := range l.Files() {
		if !assets.IsAsset(rel) {
			used[filepath.Clean(l.Resolve(rel))] = true
		}
	}
	var out []string
	_ = filepath.WalkDir(filepath.Join(l.Folder, accessories.ImagesDir), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !used[filepath.Clean(p)] {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// ownFiles lists the setup files (not store files) a library refers to that exist.
func ownFiles(l *accessories.Library) []string {
	var out []string
	for _, rel := range l.Files() {
		if !assets.IsAsset(rel) && fileExists(l.Resolve(rel)) {
			out = append(out, rel)
		}
	}
	return out
}

// referenced is every store file any setup or the catalog refers to: any "assets/…" string in its accessories.json or
// studio.json.
func referenced(h setups.Home, list []setups.Summary) map[string]bool {
	refs := map[string]bool{}
	for _, s := range append([]setups.Summary{{Folder: filepath.Join(h.Root, catalog.DirName)}}, list...) {
		dir := accessories.Folder(s.Folder)
		for _, f := range []string{accessories.LibraryFile, accessories.MetaFile} {
			if b, err := os.ReadFile(filepath.Join(dir, f)); err == nil {
				for _, r := range assets.Refs(b) {
					refs[r] = true
				}
			}
		}
	}
	return refs
}

func analyzeAccessories(h setups.Home, list []setups.Summary) AccessoryStorage {
	store := assets.For(h.Root)
	out := AccessoryStorage{Setups: []AccessorySet{}}
	entries, _ := store.List()
	refs := referenced(h, list)
	for _, e := range entries {
		out.StoreBytes += e.Size
		out.StoreFiles++
		if !refs[e.Rel] {
			out.UnusedBytes += e.Size
			out.UnusedFiles++
		}
	}
	for _, s := range list {
		l, err := accessories.Open(s.Folder, store)
		if err != nil {
			continue
		}
		as := AccessorySet{ID: s.ID, Name: s.Name}
		item := func(file string, bytes int64, action string) {
			out.list = append(out.list, MoveFile{Setup: s.Name, Owner: "Accessories & furniture", File: file, Bytes: bytes, Action: action})
		}
		for _, rel := range ownFiles(l) {
			n := size(l.Resolve(rel))
			as.Files++
			as.Bytes += n
			item(rel, n, MoveTo)
		}
		for _, p := range leftovers(l) {
			n := size(p)
			as.LeftoverFiles++
			as.LeftoverBytes += n
			rel, _ := filepath.Rel(l.Folder, p)
			item(filepath.ToSlash(rel), n, MoveLeftover)
		}
		if as.Files+as.LeftoverFiles > 0 {
			out.Setups = append(out.Setups, as)
			out.MoveFiles += as.Files + as.LeftoverFiles
			out.MoveBytes += as.Bytes + as.LeftoverBytes
		}
	}
	return out
}

// MoveAccessories moves every setup's own accessory and furniture files into the shared store: each file is stored
// (once for identical copies), the setup's accessories.json and editor layouts are pointed at it, then the old file is
// deleted. A setup is saved before its old files go, so an interrupted run leaves every reference pointing at a file
// that exists and just continues next time. Returns the files moved and the bytes freed.
func MoveAccessories(ctx context.Context, h setups.Home, progress func(Progress)) (*Result, error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	store := assets.For(h.Root)
	list, err := h.List()
	if err != nil {
		return nil, err
	}
	res := &Result{}
	for i, s := range list {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		progress(Progress{Message: "Moving accessory files of " + s.Name + "…", Done: i, Total: len(list)})
		l, err := accessories.Open(s.Folder, store)
		if err != nil {
			return res, err
		}
		own := ownFiles(l)
		junk := leftovers(l)
		if len(own)+len(junk) == 0 {
			continue
		}
		moved := map[string]string{} // old path → store path
		var freed int64
		for _, rel := range own {
			p := l.Resolve(rel)
			to, err := store.PutFile(p)
			if err != nil {
				return res, err
			}
			moved[rel] = to
			freed += size(p)
		}
		repoint(l, moved)
		if err := l.Save(); err != nil {
			return res, err
		}
		for rel := range moved {
			if os.Remove(l.Resolve(rel)) == nil {
				res.Files++
			}
		}
		for _, p := range junk { // nothing refers to these (before or after the move)
			n := size(p)
			if os.Remove(p) == nil {
				res.Files++
				freed += n
			}
		}
		removeEmptyDirs(filepath.Join(l.Folder, accessories.ImagesDir))
		res.Freed += freed
		res.Sets++ // setups changed
	}
	progress(Progress{Message: "Done", Done: len(list), Total: len(list)})
	return res, nil
}

// repoint changes every path in a library (entries and editor layouts) found in moved to its new value.
func repoint(l *accessories.Library, moved map[string]string) {
	fix := func(p *string) {
		if to, ok := moved[*p]; ok {
			*p = to
		}
	}
	for i := range l.Lib.Accessories {
		a := &l.Lib.Accessories[i]
		fix(&a.Texture)
		fix(&a.Icon)
		fix(&a.Mesh)
	}
	for i := range l.Lib.Furniture {
		f := &l.Lib.Furniture[i]
		fix(&f.Texture)
		fix(&f.Icon)
		fix(&f.Mesh)
	}
	for id, raw := range l.Meta.Layouts {
		var v any
		if json.Unmarshal(raw, &v) != nil {
			continue
		}
		if b, err := json.Marshal(replaceStrings(v, moved)); err == nil {
			l.Meta.Layouts[id] = b
		}
	}
}

func replaceStrings(v any, m map[string]string) any {
	switch t := v.(type) {
	case string:
		if to, ok := m[t]; ok {
			return to
		}
	case []any:
		for i := range t {
			t[i] = replaceStrings(t[i], m)
		}
	case map[string]any:
		for k := range t {
			t[k] = replaceStrings(t[k], m)
		}
	}
	return v
}

// DeleteUnusedAssets deletes store files that no setup refers to (edited textures leave their earlier version behind,
// deleted accessories their files). Returns bytes and files freed.
func DeleteUnusedAssets(h setups.Home) (int64, int, error) {
	list, err := h.List()
	if err != nil {
		return 0, 0, err
	}
	store := assets.For(h.Root)
	entries, err := store.List()
	if err != nil {
		return 0, 0, err
	}
	refs := referenced(h, list)
	var freed int64
	n := 0
	for _, e := range entries {
		if refs[e.Rel] {
			continue
		}
		if os.Remove(store.Path(e.Rel)) == nil {
			freed += e.Size
			n++
		}
	}
	return freed, n, nil
}

// removeEmptyDirs deletes empty folders under dir (and dir itself when it ends up empty).
func removeEmptyDirs(dir string) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, d := range des {
		if d.IsDir() {
			removeEmptyDirs(filepath.Join(dir, d.Name()))
		}
	}
	_ = os.Remove(dir) // only succeeds when empty
}

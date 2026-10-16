package library

import (
	"os"
	"path/filepath"

	"tcgstudio/internal/accessories"
)

// itemsOf lists a setup's accessories, furniture and decorations with the size of their shared files.
func itemsOf(l *accessories.Library) []ItemInfo {
	out := []ItemInfo{}
	sizes := func(rels ...string) (n int64) {
		for _, rel := range rels {
			if rel != "" {
				n += size(l.Resolve(rel))
			}
		}
		return
	}
	for _, a := range l.Lib.Accessories {
		out = append(out, ItemInfo{ID: a.ID, Name: a.Name, Kind: a.Kind, Shared: sizes(a.Texture, a.Icon, a.Mesh)})
	}
	for _, f := range l.Lib.Furniture {
		out = append(out, ItemInfo{ID: f.ID, Name: f.Name, Kind: "Furniture", Shared: sizes(f.Files()...)})
	}
	for _, d := range l.Lib.Decorations {
		out = append(out, ItemInfo{ID: d.ID, Name: d.Name, Kind: "Decoration", Shared: sizes(d.Files()...)})
	}
	return out
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

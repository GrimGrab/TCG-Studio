package library

import (
	"os"
	"path/filepath"

	"tcgstudio/internal/content"
	"tcgstudio/internal/workspace"
)

// itemsOf lists everything in a setup's item library with the size of its shared files (some kinds, like animations, have none).
func itemsOf(ws workspace.Dir, root string) []ItemInfo {
	out := []ItemInfo{}
	store := ws.Store()
	for _, k := range content.LibraryKinds() {
		items, err := k.Store.List(ws, root)
		if err != nil {
			continue
		}
		for _, it := range items {
			info := ItemInfo{ID: it.ID, Name: it.Name, Kind: k.Title}
			if it.Sub != "" {
				info.Kind += " · " + it.Sub
			}
			for _, ref := range it.Refs {
				info.Shared += size(store.Path(ref))
			}
			out = append(out, info)
		}
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

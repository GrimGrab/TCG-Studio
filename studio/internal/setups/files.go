package setups

import (
	"encoding/json"
	"os"
	"path/filepath"

	"tcgstudio/internal/fsutil"
)

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// moveTree moves every file under src to the same place under dst (replacing files there), then removes the empty folders.
func moveTree(src, dst string) error {
	var dirs []string
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			dirs = append(dirs, p)
			return nil
		}
		return fsutil.MoveFile(p, filepath.Join(dst, rel))
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		_ = os.Remove(dirs[i]) // only succeeds when empty
	}
	return nil
}

// dirEmpty reports whether dir is missing or has no entries.
func dirEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err != nil || len(entries) == 0
}

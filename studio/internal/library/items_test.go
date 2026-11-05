package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/content"
	"tcgstudio/internal/workspace"
)

// Storage lists an item of every kind in the registry: a new kind only needs its line in content.Kinds.
func TestItemsOfCoversEveryLibraryKind(t *testing.T) {
	ws := workspace.Dir(t.TempDir())
	root := t.TempDir()
	lib := map[string]any{"schemaVersion": 1}
	kinds := content.LibraryKinds()
	if len(kinds) == 0 {
		t.Fatal("no library kinds")
	}
	for _, k := range kinds {
		field := k.Store.(content.Entry).Field
		lib[field] = []map[string]any{{"id": "x-" + k.ID, "name": k.Title}}
	}
	b, err := json.Marshal(lib)
	if err != nil {
		t.Fatal(err)
	}
	dir := accessories.Folder(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, accessories.LibraryFile), b, 0o644); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, it := range itemsOf(ws, root) {
		got[it.ID] = true
	}
	for _, k := range kinds {
		if !got["x-"+k.ID] {
			t.Errorf("kind %q isn't listed", k.ID)
		}
	}
}

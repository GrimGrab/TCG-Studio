package library

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/catalog"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/workspace"
)

// Art and files only the shared catalog uses (no setup has the item any more) are not "unused".
func TestCatalogKeepsArtAndAssets(t *testing.T) {
	h, _ := newHome(t)
	c := catalog.For(h.Root)
	ws := c.Sets()
	p, err := ws.Create("dom", "Dominaria")
	if err != nil {
		t.Fatal(err)
	}
	lib := filepath.Join(p.LibFolder, "images", "c1.png")
	if err := os.MkdirAll(filepath.Dir(lib), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lib, []byte("art"), 0o644); err != nil {
		t.Fatal(err)
	}
	p.Set.Cards = []setfmt.Card{{ID: "c1", Name: "One", Image: "images/c1.png"}}
	if err := ws.Save(p); err != nil {
		t.Fatal(err)
	}
	store := workspace.Dir(h.Root).Store()
	rel, _ := store.Put([]byte("texture"), ".png")
	cat, _ := c.Items()
	cat.PutFurniture(setfmt.Furniture{ID: "bin", Type: "TrashBin", Name: "Bin", Texture: rel})
	if err := cat.Save(); err != nil {
		t.Fatal(err)
	}

	rep, err := Analyze(context.Background(), h, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Listed, but only as catalog entries (their art and files are never "unused files" on their own).
	for _, e := range rep.Unused {
		if !e.InCatalog || (e.Kind != UnusedCatalogSet && e.Kind != UnusedCatalogItem) {
			t.Fatalf("catalog art/assets reported as plain unused: %+v", rep.Unused)
		}
	}
	if len(rep.Unused) != 2 {
		t.Fatalf("unused %+v", rep.Unused)
	}
	if _, err := os.Stat(project.LibraryDir(h.Root)); err != nil {
		t.Fatal(err)
	}
}

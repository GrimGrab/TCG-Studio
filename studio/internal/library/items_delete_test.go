package library

import (
	"context"
	"os"
	"testing"

	"tcgstudio/internal/catalog"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/workspace"
)

// Deleting a catalog-only person takes the entry and the files only it uses, never a file a setup still uses.
func TestDeleteCatalogPersonKeepsFilesStillUsed(t *testing.T) {
	h, b := newHome(t)
	ws := workspace.Dir(h.Root)
	store := ws.Store()
	shared, err := store.Put([]byte("portrait a setup also uses"), ".png")
	if err != nil {
		t.Fatal(err)
	}
	own, err := store.Put([]byte("portrait only the catalog uses"), ".png")
	if err != nil {
		t.Fatal(err)
	}
	setup, err := ws.Items(h.Dir(b))
	if err != nil {
		t.Fatal(err)
	}
	setup.Lib.Faces = []setfmt.SavedFace{{ID: "in-setup", Body: "male", Portrait: shared}}
	if err := setup.Save(); err != nil {
		t.Fatal(err)
	}
	c := catalog.For(h.Root)
	cat, err := c.Items()
	if err != nil {
		t.Fatal(err)
	}
	cat.Lib.Customers = []setfmt.Customer{
		{ID: "sam", Name: "Sam", Model: -1, Traits: []string{}, Portrait: shared},
		{ID: "ann", Name: "Ann", Model: -1, Traits: []string{}, Portrait: own},
	}
	if err := cat.Save(); err != nil {
		t.Fatal(err)
	}

	res, err := DeleteUnused(context.Background(), h, []Pick{{Kind: UnusedCatalogItem, ID: "sam"}, {Kind: UnusedCatalogItem, ID: "ann"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Deleted) != 2 {
		t.Fatalf("deleted %+v", res.Deleted)
	}
	cat, err = c.Items()
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Lib.Customers) != 0 {
		t.Errorf("still in the catalog: %+v", cat.Lib.Customers)
	}
	if _, err := os.Stat(store.Path(shared)); err != nil {
		t.Errorf("a portrait a setup uses was deleted: %v", err)
	}
	if _, err := os.Stat(store.Path(own)); !os.IsNotExist(err) {
		t.Errorf("a portrait nothing uses any more was kept: %v", err)
	}
}

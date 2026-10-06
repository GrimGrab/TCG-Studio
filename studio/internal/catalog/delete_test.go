package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
)

func addSet(t *testing.T, h setups.Home, dir, id, image string) *project.Project {
	t.Helper()
	ws := project.Workspace{Root: dir, Library: project.LibraryDir(h.Root)}
	p, err := ws.Create(id, id)
	if err != nil {
		t.Fatal(err)
	}
	p.Set.Cards = []setfmt.Card{{ID: "1", Name: "One", Image: image}}
	write(t, filepath.Join(p.LibFolder, filepath.FromSlash(image)), "art")
	if err := ws.Save(p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAllListsEverythingWithUsedBy(t *testing.T) {
	h, a, b := twoSetups(t)
	c := For(h.Root)
	addSet(t, h, a, "dom", "images/1.png")
	addSet(t, h, b, "dom", "images/1.png")
	addSet(t, h, a, "only-a", "images/1.png") // only in the active setup: List skips it, All shows it
	tmpl := addSet(t, h, b, "tmpl", "images/1.png")
	if err := c.SaveSet(tmpl); err != nil {
		t.Fatal(err)
	}
	all, err := c.All(h, a, KindSet)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Entry{}
	for _, e := range all {
		got[e.Key] = e
	}
	if len(all) != 3 || len(got["dom"].UsedBy) != 2 || len(got["only-a"].UsedBy) != 1 || !got["only-a"].Here || got["dom"].Cover != "images/1.png" {
		t.Fatalf("all %+v", all)
	}
	if l, _ := c.List(h, a, KindSet); len(l) != 2 { // the picker: not what this setup has already… except as "here"
		t.Fatalf("list %+v", l)
	}
}

func TestDeleteEverywhereSet(t *testing.T) {
	h, a, b := twoSetups(t)
	c := For(h.Root)
	pa := addSet(t, h, a, "dom", "images/1.png")
	addSet(t, h, b, "dom", "images/1.png")
	addSet(t, h, a, "keep", "images/1.png")
	if err := c.SaveSet(pa); err != nil {
		t.Fatal(err)
	}
	info, _ := setups.LoadInfo(b)
	info.InstalledSets = []string{"dom", "keep"}
	_ = setups.SaveInfo(b, info)

	res, err := c.DeleteEverywhere(h, KindSet, []string{"dom"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Removed != 3 || len(res.Setups) != 2 || len(res.SetIDs) != 1 {
		t.Fatalf("result %+v", res)
	}
	for _, dir := range []string{a, b, c.Root} {
		if _, err := os.Stat(filepath.Join(dir, "projects", "dom")); !os.IsNotExist(err) {
			t.Fatalf("dom still in %s", dir)
		}
	}
	if _, err := os.Stat(filepath.Join(project.LibraryDir(h.Root), "dom")); !os.IsNotExist(err) {
		t.Fatal("shared art kept")
	}
	if _, err := os.Stat(filepath.Join(project.LibraryDir(h.Root), "keep")); err != nil {
		t.Fatal("other set's art deleted")
	}
	if info, _ := setups.LoadInfo(b); len(info.InstalledSets) != 1 || info.InstalledSets[0] != "keep" {
		t.Fatalf("installed sets %v", info.InstalledSets)
	}
}

// Deleting a set's own-art version leaves the ordinary one (and its shared files) alone.
func TestDeleteEverywhereOwnArt(t *testing.T) {
	h, a, b := twoSetups(t)
	c := For(h.Root)
	addSet(t, h, a, "dom", "images/1.png")
	addSet(t, h, b, "dom", "Dom (B)/images/1.png")
	if _, err := c.DeleteEverywhere(h, KindSet, []string{"dom|Dom (B)"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(b, "projects", "dom")); !os.IsNotExist(err) {
		t.Fatal("B's own-art copy kept")
	}
	if _, err := os.Stat(filepath.Join(a, "projects", "dom", project.SetFile)); err != nil {
		t.Fatal("A's set deleted")
	}
	lib := filepath.Join(project.LibraryDir(h.Root), "dom")
	if _, err := os.Stat(filepath.Join(lib, "Dom (B)")); !os.IsNotExist(err) {
		t.Fatal("own-art folder kept")
	}
	if _, err := os.Stat(filepath.Join(lib, "images", "1.png")); err != nil {
		t.Fatal("A's shared art deleted")
	}
}

func TestDeleteEverywhereItem(t *testing.T) {
	h, a, b := twoSetups(t)
	c := For(h.Root)
	store := assets.For(h.Root)
	gone, _ := store.Put([]byte("mat texture"), ".png")
	shared, _ := store.Put([]byte("shared texture"), ".png")
	for _, dir := range []string{a, b} {
		l, _ := accessories.Open(dir, store)
		l.Put(setfmt.Accessory{ID: "mat", Kind: "Playmat", Name: "Mat", Texture: gone})
		l.Put(setfmt.Accessory{ID: "other", Kind: "Playmat", Name: "Other", Texture: shared})
		if err := l.Save(); err != nil {
			t.Fatal(err)
		}
	}
	la, _ := accessories.Open(a, store)
	if err := c.SaveItems(la, []string{"mat"}); err != nil {
		t.Fatal(err)
	}
	res, err := c.DeleteEverywhere(h, KindAccessory, []string{"mat"})
	if err != nil || res.Removed != 3 {
		t.Fatalf("%+v %v", res, err)
	}
	for _, dir := range []string{a, b} {
		if l, _ := accessories.Open(dir, store); l.Index("mat") >= 0 || l.Index("other") < 0 {
			t.Fatalf("library of %s wrong", dir)
		}
	}
	if cat, _ := c.Items(); cat.Index("mat") >= 0 {
		t.Fatal("template kept")
	}
	if _, err := os.Stat(store.Path(gone)); !os.IsNotExist(err) {
		t.Fatal("unused store file kept")
	}
	if _, err := os.Stat(store.Path(shared)); err != nil {
		t.Fatal("store file still used was deleted")
	}
}

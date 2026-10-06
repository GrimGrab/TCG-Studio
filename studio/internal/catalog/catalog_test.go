package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/origin"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
)

func write(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// twoSetups returns a workspace with setups A (active, default) and B.
func twoSetups(t *testing.T) (setups.Home, string, string) {
	h := setups.Home{Root: t.TempDir()}
	a, err := h.Ensure("1.0")
	if err != nil {
		t.Fatal(err)
	}
	id, err := h.Create("B", "", false, "1.0")
	if err != nil {
		t.Fatal(err)
	}
	return h, a, h.Dir(id)
}

func TestSetTemplateAddAndIndependence(t *testing.T) {
	h, a, b := twoSetups(t)
	c := For(h.Root)
	wsA := project.Workspace{Root: a, Library: project.LibraryDir(h.Root)}
	p, err := wsA.Create("dom", "Dominaria")
	if err != nil {
		t.Fatal(err)
	}
	// Card art in the shared library, pack art in the project folder.
	write(t, filepath.Join(p.LibFolder, "images", "c1.png"), "card art")
	write(t, filepath.Join(p.Folder, "images", "pack.png"), "pack art")
	p.Set.Cards = []setfmt.Card{{ID: "c1", Name: "One", Image: "images/c1.png"}}
	p.Meta.Origin = &origin.Origin{Kind: "EPL mod", Mod: "Marvel", Author: "Someone"}
	if err := wsA.Save(p); err != nil {
		t.Fatal(err)
	}
	if err := c.SaveSet(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(c.Sets().Folder("dom"), "images", "c1.png")); !os.IsNotExist(err) {
		t.Fatal("card art copied into the catalog (should stay in the library)")
	}

	list, err := c.List(h, b, KindSet)
	if err != nil || len(list) != 1 || !list[0].Saved || list[0].Here || list[0].Origin != "Marvel" || list[0].Author != "Someone" {
		t.Fatalf("list from B: %+v %v", list, err)
	}
	if l, _ := c.List(h, a, KindSet); len(l) != 1 || !l[0].Here {
		t.Fatalf("list from A: %+v", l)
	}

	res, err := c.AddSets(h, b, []string{"dom"})
	if err != nil || len(res.Added) != 1 {
		t.Fatalf("add: %+v %v", res, err)
	}
	wsB := project.Workspace{Root: b, Library: project.LibraryDir(h.Root)}
	pb, err := wsB.Load("dom")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(pb.ImagePath("images/c1.png")); string(got) != "card art" {
		t.Fatalf("card art not found from B: %q", got)
	}
	if got, _ := os.ReadFile(pb.ImagePath("images/pack.png")); string(got) != "pack art" {
		t.Fatalf("pack art not copied: %q", got)
	}
	if pb.Meta.Origin == nil || pb.Meta.Origin.Mod != "Marvel" {
		t.Fatalf("origin lost: %+v", pb.Meta.Origin)
	}

	// B's copy is its own: renaming it changes neither A nor the template.
	pb.Set.Name = "B's Dominaria"
	if err := wsB.Save(pb); err != nil {
		t.Fatal(err)
	}
	if pa, _ := wsA.Load("dom"); pa.Set.Name != "Dominaria" {
		t.Fatalf("A changed: %q", pa.Set.Name)
	}
	if pt, _ := c.Sets().Load("dom"); pt.Set.Name != "Dominaria" {
		t.Fatalf("template changed: %q", pt.Set.Name)
	}

	// Adding again is skipped.
	if res, _ := c.AddSets(h, b, []string{"dom"}); len(res.Skipped) != 1 || len(res.Added) != 0 {
		t.Fatalf("second add: %+v", res)
	}
}

func TestOtherSetupsItemsAreOfferedAndTemplated(t *testing.T) {
	h, a, b := twoSetups(t)
	c := For(h.Root)
	store := assets.For(h.Root)
	// A has an older-style accessory (own file in the setup) and a furniture piece; the catalog has nothing yet.
	write(t, filepath.Join(accessories.Folder(a), "images", "mat_texture.png"), "texture")
	la, _ := accessories.Open(a, store)
	la.Put(setfmt.Accessory{ID: "mat", Kind: "Playmat", Name: "Mat", Texture: "images/mat_texture.png"})
	la.PutFurniture(setfmt.Furniture{ID: "bin", Type: "TrashBin", Name: "Bin"})
	la.Meta.Layouts["mat"] = json.RawMessage(`{"layers":[{"src":"images/mat_texture.png"}]}`)
	la.Meta.Origins["mat"] = origin.Origin{Mod: "Mats Mod"}
	if err := la.Save(); err != nil {
		t.Fatal(err)
	}

	list, err := c.List(h, b, KindAccessory)
	if err != nil || len(list) != 1 || list[0].Saved || list[0].From != "My Setup" || list[0].Origin != "Mats Mod" {
		t.Fatalf("accessories offered to B: %+v %v", list, err)
	}
	if f, _ := c.List(h, b, KindFurniture); len(f) != 1 || f[0].ID != "bin" {
		t.Fatalf("furniture offered to B: %+v", f)
	}

	lb, _ := accessories.Open(b, store)
	res, err := c.AddItems(h, b, KindAccessory, []string{"mat"}, lb)
	if err != nil || len(res.Added) != 1 {
		t.Fatalf("add: %+v %v", res, err)
	}
	acc := lb.Lib.Accessories[0]
	if !assets.IsAsset(acc.Texture) {
		t.Fatalf("B's copy should use the shared store: %+v", acc)
	}
	if got, _ := os.ReadFile(lb.Resolve(acc.Texture)); string(got) != "texture" {
		t.Fatalf("texture %q", got)
	}
	if lb.Meta.Origins["mat"].Mod != "Mats Mod" || len(lb.Meta.Layouts["mat"]) == 0 {
		t.Fatal("layout/origin not copied")
	}
	// Templated on the way; A keeps its own file.
	if l, _ := c.List(h, b, KindAccessory); !l[0].Saved {
		t.Fatalf("not templated: %+v", l)
	}
	if la2, _ := accessories.Open(a, store); la2.Lib.Accessories[0].Texture != "images/mat_texture.png" {
		t.Fatal("A was changed")
	}
}

func TestKeepSetupBeforeDelete(t *testing.T) {
	h, _, b := twoSetups(t)
	c := For(h.Root)
	wsB := project.Workspace{Root: b, Library: project.LibraryDir(h.Root)}
	if _, err := wsB.Create("neo", "Kamigawa"); err != nil {
		t.Fatal(err)
	}
	lb, _ := accessories.Open(b, assets.For(h.Root))
	lb.PutFurniture(setfmt.Furniture{ID: "bin", Type: "TrashBin", Name: "Bin"})
	if err := lb.Save(); err != nil {
		t.Fatal(err)
	}
	if err := c.KeepSetup(b); err != nil {
		t.Fatal(err)
	}
	if err := h.Delete(filepath.Base(b)); err != nil {
		t.Fatal(err)
	}
	if ids := c.SetIDs(); len(ids) != 1 || ids[0] != "neo" {
		t.Fatalf("set templates %v", ids)
	}
	if cat, _ := c.Items(); cat.FurnitureIndex("bin") < 0 {
		t.Fatal("furniture not kept")
	}
	if err := c.Remove(KindSet, "neo"); err != nil || len(c.SetIDs()) != 0 {
		t.Fatalf("remove: %v", err)
	}
}

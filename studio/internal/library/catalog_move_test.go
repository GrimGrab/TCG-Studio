package library

import (
	"bytes"
	"context"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/catalog"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
)

// The catalog templated a set at import, then the setup's Smart generate replaced its pack art: the setup's art becomes
// the shared one and the template's stale copy is dropped (never the other way round, never a "differs" for the player).
func TestMoveAllPrefersSetupOverStaleCatalogCopy(t *testing.T) {
	h, _ := newHome(t)
	red, blue := pngBytes(t, color.NRGBA{200, 0, 0, 255}), pngBytes(t, color.NRGBA{0, 0, 200, 255})
	p := addSet(t, h, setups.DefaultID, red, blue)
	pk := setfmt.NewPack("booster", "X Booster")
	pk.PackTexture = "images/pack_texture.png"
	p.Set.Packs = append(p.Set.Packs, pk)
	if err := (project.Workspace{}).Save(p); err != nil {
		t.Fatal(err)
	}
	cat := catalog.For(h.Root)
	put(t, filepath.Join(p.Folder, "images", "pack_texture.png"), red) // the import's simple art …
	if err := cat.SaveSet(p); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(p.Folder, "images", "pack_texture.png"), blue) // … then Smart generate in the setup

	rep, err := Analyze(context.Background(), h, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Differ) != 0 {
		t.Fatalf("differ %+v", rep.Differ)
	}
	actions := map[string]string{}
	for _, f := range rep.MoveList {
		if f.File == "images/pack_texture.png" {
			actions[f.Setup] = f.Action
		}
	}
	if actions["Catalog"] != MoveDrop || len(actions) != 2 {
		t.Fatalf("pack texture actions %v", actions)
	}
	if rep.MoveFiles != len(rep.MoveList) || rep.MoveBytes == 0 {
		t.Fatalf("moveFiles %d list %d bytes %d", rep.MoveFiles, len(rep.MoveList), rep.MoveBytes)
	}

	if _, err := MoveAll(context.Background(), h, nil); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(project.LibraryDir(h.Root), "x-set", "images", "pack_texture.png"))
	if err != nil || !bytes.Equal(b, blue) {
		t.Fatalf("shared pack texture is not the setup's (err %v)", err)
	}
	if fileExists(filepath.Join(cat.Sets().Folder("x-set"), "images", "pack_texture.png")) {
		t.Fatal("catalog kept its stale copy")
	}
	rep, err = Analyze(context.Background(), h, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.MoveFiles != 0 || len(rep.Differ) != 0 {
		t.Fatalf("after move: files %d differ %d %+v", rep.MoveFiles, len(rep.Differ), rep.MoveList)
	}
}

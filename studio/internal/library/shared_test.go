package library

import (
	"bytes"
	"context"
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
)

func put(t *testing.T, f string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func load(t *testing.T, h setups.Home, setup string) *project.Project {
	t.Helper()
	p, err := project.Workspace{Root: h.Dir(setup), Library: project.LibraryDir(h.Root)}.Load("x-set")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Everything a set uses moves (pack art, editor sources); leftovers go; referenced files and set.json/studio.json stay.
func TestMoveAllTakesEveryFileAndLeftovers(t *testing.T) {
	h, _ := newHome(t)
	red, blue := pngBytes(t, color.NRGBA{200, 0, 0, 255}), pngBytes(t, color.NRGBA{0, 0, 200, 255})
	p := addSet(t, h, setups.DefaultID, red, blue)
	pk := setfmt.NewPack("booster", "X Booster")
	pk.PackTexture = "images/pack_texture.png"
	p.Set.Packs = append(p.Set.Packs, pk)
	p.Meta.PackArt = map[string]map[string]json.RawMessage{"booster": {"pack": json.RawMessage(`{"layers":[{"src":"images/photo_7.png"}]}`)}}
	put(t, filepath.Join(p.Folder, "images", "pack_texture.png"), blue)
	put(t, filepath.Join(p.Folder, "images", "photo_7.png"), red)
	put(t, filepath.Join(p.Folder, "images", "set_icon.svg"), []byte("<svg/>"))
	put(t, filepath.Join(p.Folder, "images", "old_auto_1.png"), red) // nothing refers to it
	put(t, filepath.Join(p.Folder, "notes.txt"), []byte("mine"))     // not an image: never touched
	if err := (project.Workspace{}).Save(p); err != nil {
		t.Fatal(err)
	}

	rep, err := Analyze(context.Background(), h, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	si := rep.Setups[0]
	if si.Leftovers != int64(len(red)) || si.CardArt != int64(len(red)+len(blue)) || si.SetFiles == 0 || si.Info == 0 {
		t.Fatalf("breakdown %+v", si)
	}
	if sum := si.Info + si.CardArt + si.SetFiles + si.Leftovers + si.ItemFiles + si.Saves + si.Game + si.Other; sum != si.Size {
		t.Fatalf("breakdown %d != size %d", sum, si.Size)
	}
	if _, err := MoveAll(context.Background(), h, nil); err != nil {
		t.Fatal(err)
	}
	lib := filepath.Join(project.LibraryDir(h.Root), "x-set")
	for _, rel := range []string{"images/1.png", "images/2.png", "images/pack_texture.png", "images/photo_7.png", "images/set_icon.svg"} {
		if !fileExists(filepath.Join(lib, filepath.FromSlash(rel))) {
			t.Fatalf("%s not shared", rel)
		}
		if fileExists(filepath.Join(p.Folder, filepath.FromSlash(rel))) {
			t.Fatalf("%s still in the setup", rel)
		}
	}
	if fileExists(filepath.Join(p.Folder, "images", "old_auto_1.png")) {
		t.Fatal("leftover kept")
	}
	if !fileExists(filepath.Join(p.Folder, "notes.txt")) || !fileExists(filepath.Join(p.Folder, project.SetFile)) {
		t.Fatal("non-image or game info deleted")
	}
	again, _ := Analyze(context.Background(), h, "", nil)
	if again.MoveFiles != 0 || len(again.Differ) != 0 {
		t.Fatalf("second run would still do %d / differ %v", again.MoveFiles, again.Differ)
	}
}

// Differing art: "use shared" deletes the setup's copy (other format repointed); "keep as its own" moves it to a named
// shared folder and points the set at it; the set id never changes and every card still resolves.
func TestResolveDiffering(t *testing.T) {
	h, b := newHome(t)
	red, blue, green := pngBytes(t, color.NRGBA{200, 0, 0, 255}), pngBytes(t, color.NRGBA{0, 0, 200, 255}), pngBytes(t, color.NRGBA{0, 200, 0, 255})
	addSet(t, h, setups.DefaultID, red, blue)
	pb := addSet(t, h, b, green, green) // B: both cards different art
	if _, err := MoveAll(context.Background(), h, nil); err != nil {
		t.Fatal(err)
	}
	rep, _ := Analyze(context.Background(), h, "", nil)
	if len(rep.Differ) != 1 || rep.Differ[0].Files != 2 || rep.Differ[0].Cards != 2 {
		t.Fatalf("differ %+v", rep.Differ)
	}
	if pv, err := PreviewDiffer(h, b, "x-set"); err != nil || pv.Own == "" || pv.Shared == "" {
		t.Fatalf("preview %v %v", pv, err)
	}

	// Keep as its own.
	if _, err := ResolveDiffering(h, b, "x-set", KeepOwn, `X: (Test)/`); err != nil {
		t.Fatal(err)
	}
	pb = load(t, h, b)
	if pb.ID != "x-set" || pb.Set.Cards[0].Image != "X (Test)/images/1.png" {
		t.Fatalf("B points at %q", pb.Set.Cards[0].Image)
	}
	for _, c := range pb.Set.Cards {
		got, _ := os.ReadFile(pb.ImagePath(c.Image))
		if !bytes.Equal(got, green) || !pb.InLibrary(c.Image) {
			t.Fatalf("%s: not B's art from the shared folder", c.Image)
		}
	}
	if artFolder(pb) != "X (Test)" {
		t.Fatalf("art folder %q", artFolder(pb))
	}
	if a := load(t, h, setups.DefaultID); !bytes.Equal(must(os.ReadFile(a.ImagePath("images/1.png"))), red) {
		t.Fatal("A's art changed")
	}
	if rep, _ := Analyze(context.Background(), h, "", nil); len(rep.Differ) != 0 || rep.MoveFiles != 0 {
		t.Fatalf("after keep: differ %v move %d", rep.Differ, rep.MoveFiles)
	}
}

func TestResolveDifferingUseShared(t *testing.T) {
	h, b := newHome(t)
	red, blue := pngBytes(t, color.NRGBA{200, 0, 0, 255}), pngBytes(t, color.NRGBA{0, 0, 200, 255})
	addSet(t, h, setups.DefaultID, red, blue)
	pb := addSet(t, h, b, blue, blue) // B: card 1 different, card 2 the same
	// B's card 2 is a PNG while the shared one will be a JPEG of the same name (left by Shrink in one place).
	if _, err := MoveAll(context.Background(), h, nil); err != nil {
		t.Fatal(err)
	}
	lib := filepath.Join(project.LibraryDir(h.Root), "x-set", "images")
	if err := os.Rename(filepath.Join(lib, "2.png"), filepath.Join(lib, "2.jpg")); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(pb.Folder, "images", "2.png"), blue)
	if _, err := ResolveDiffering(h, b, "x-set", UseShared, ""); err != nil {
		t.Fatal(err)
	}
	pb = load(t, h, b)
	if pb.Set.Cards[1].Image != "images/2.jpg" {
		t.Fatalf("card 2 not pointed at the shared JPEG: %q", pb.Set.Cards[1].Image)
	}
	for _, c := range pb.Set.Cards {
		if !pb.InLibrary(c.Image) {
			t.Fatalf("%s still B's own", c.Image)
		}
	}
	if fileExists(filepath.Join(pb.Folder, "images", "1.png")) || fileExists(filepath.Join(pb.Folder, "images", "2.png")) {
		t.Fatal("B's copies kept")
	}
}

// Files in a set's shared folder nothing refers to are listed and deletable; a set that fails to load never loses art.
func TestUnusedSetFilesAndBrokenSets(t *testing.T) {
	h, _ := newHome(t)
	red := pngBytes(t, color.NRGBA{200, 0, 0, 255})
	addSet(t, h, setups.DefaultID, red, red)
	if _, err := MoveAll(context.Background(), h, nil); err != nil {
		t.Fatal(err)
	}
	lib := filepath.Join(project.LibraryDir(h.Root), "x-set")
	put(t, filepath.Join(lib, "images", "stale.png"), red)
	put(t, filepath.Join(lib, "Old Name", "images", "1.png"), red) // a named folder nothing uses any more
	rep, _ := Analyze(context.Background(), h, "", nil)
	var e *UnusedEntry
	for i := range rep.Unused {
		if rep.Unused[i].Kind == UnusedSetFiles {
			e = &rep.Unused[i]
		}
	}
	if e == nil || e.Files != 2 {
		t.Fatalf("unused %+v", rep.Unused)
	}
	if _, err := DeleteUnused(h, "", UnusedSetFiles, []string{"x-set"}); err != nil {
		t.Fatal(err)
	}
	if fileExists(filepath.Join(lib, "images", "stale.png")) || fileExists(filepath.Join(lib, "Old Name")) {
		t.Fatal("unused files kept")
	}
	if !fileExists(filepath.Join(lib, "images", "1.png")) {
		t.Fatal("used card deleted")
	}

	// A set.json that can't be read: its shared files are never offered for deletion.
	put(t, filepath.Join(lib, "images", "stale.png"), red)
	put(t, filepath.Join(h.Dir(setups.DefaultID), "projects", "x-set", project.SetFile), []byte("{broken"))
	rep, _ = Analyze(context.Background(), h, "", nil)
	for _, u := range rep.Unused {
		if u.ID == "x-set" {
			t.Fatalf("broken set's art offered: %+v", u)
		}
	}
}

func must(b []byte, _ error) []byte { return b }

// "Replace": the setup's art becomes the shared art, so the other setups show it too (other-format counterparts followed).
func TestResolveDifferingReplace(t *testing.T) {
	h, b := newHome(t)
	red, blue, green := pngBytes(t, color.NRGBA{200, 0, 0, 255}), pngBytes(t, color.NRGBA{0, 0, 200, 255}), pngBytes(t, color.NRGBA{0, 200, 0, 255})
	addSet(t, h, setups.DefaultID, red, blue)
	pb := addSet(t, h, b, green, green)
	if _, err := MoveAll(context.Background(), h, nil); err != nil {
		t.Fatal(err)
	}
	// The shared card 2 is a JPEG of the same name (as Shrink leaves it); A points at it.
	lib := filepath.Join(project.LibraryDir(h.Root), "x-set", "images")
	if err := os.Rename(filepath.Join(lib, "2.png"), filepath.Join(lib, "2.jpg")); err != nil {
		t.Fatal(err)
	}
	pa := load(t, h, setups.DefaultID)
	pa.Set.Cards[1].Image = "images/2.jpg"
	if err := (project.Workspace{}).Save(pa); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveDiffering(h, b, "x-set", Replace, ""); err != nil {
		t.Fatal(err)
	}
	pa, pb = load(t, h, setups.DefaultID), load(t, h, b)
	for _, p := range []*project.Project{pa, pb} {
		for _, c := range p.Set.Cards {
			got, _ := os.ReadFile(p.ImagePath(c.Image))
			if !bytes.Equal(got, green) || !p.InLibrary(c.Image) {
				t.Fatalf("%s %s: not the replaced shared art", p.Folder, c.Image)
			}
		}
	}
	if pa.Set.Cards[1].Image != "images/2.png" {
		t.Fatalf("A not moved to the replaced file: %q", pa.Set.Cards[1].Image)
	}
	if rep, _ := Analyze(context.Background(), h, "", nil); len(rep.Differ) != 0 {
		t.Fatalf("still differs: %+v", rep.Differ)
	}
}

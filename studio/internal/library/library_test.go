package library

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
)

func pngBytes(t *testing.T, c color.Color) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 64, 90))
	for y := 0; y < 90; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// addSet makes project "x-set" (source test/SET, two cards) in setup dir with the given card art.
func addSet(t *testing.T, h setups.Home, setupID string, art1, art2 []byte) *project.Project {
	t.Helper()
	ws := project.Workspace{Root: h.Dir(setupID), Library: project.LibraryDir(h.Root)}
	p, err := ws.Create("x-set", "X")
	if err != nil {
		t.Fatal(err)
	}
	p.Meta.Source, p.Meta.SetCode = "test", "SET"
	p.Set.Cards = []setfmt.Card{{ID: "1", Name: "One", Image: "images/1.png"}, {ID: "2", Name: "Two", Image: "images/2.png"}}
	for rel, b := range map[string][]byte{"images/1.png": art1, "images/2.png": art2} {
		f := filepath.Join(p.Folder, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(f), 0o755)
		if err := os.WriteFile(f, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := ws.Save(p); err != nil {
		t.Fatal(err)
	}
	return p
}

func newHome(t *testing.T) (setups.Home, string) {
	t.Helper()
	h := setups.Home{Root: t.TempDir()}
	if _, err := h.Ensure("1.0"); err != nil {
		t.Fatal(err)
	}
	b, err := h.Create("B", "", false, "1.0")
	if err != nil {
		t.Fatal(err)
	}
	return h, b
}

func TestMoveSharesIdenticalArt(t *testing.T) {
	h, b := newHome(t)
	red, blue, green := pngBytes(t, color.NRGBA{200, 0, 0, 255}), pngBytes(t, color.NRGBA{0, 0, 200, 255}), pngBytes(t, color.NRGBA{0, 200, 0, 255})
	addSet(t, h, setups.DefaultID, red, blue)
	addSet(t, h, b, red, green) // card 2 changed by hand in setup B

	rep, err := Analyze(context.Background(), h, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.MoveFiles != 3 || rep.MoveSaves != int64(len(red)) {
		t.Fatalf("estimate: files %d saves %d", rep.MoveFiles, rep.MoveSaves)
	}
	res, err := MoveAll(context.Background(), h, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Freed != int64(len(red)) {
		t.Fatalf("freed %d", res.Freed)
	}
	lib := filepath.Join(project.LibraryDir(h.Root), "x-set", "images")
	if b1, _ := os.ReadFile(filepath.Join(lib, "1.png")); !bytes.Equal(b1, red) {
		t.Fatal("card 1 not in the library")
	}
	ws := func(id string) *project.Project {
		p, err := project.Workspace{Root: h.Dir(id), Library: project.LibraryDir(h.Root)}.Load("x-set")
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	pa, pb := ws(setups.DefaultID), ws(b)
	if !pa.InLibrary("images/1.png") || !pb.InLibrary("images/1.png") {
		t.Fatal("card 1 not served from the library")
	}
	got, _ := os.ReadFile(pb.ImagePath("images/2.png"))
	if !bytes.Equal(got, green) || pb.InLibrary("images/2.png") {
		t.Fatal("setup B lost its own card 2")
	}
	got, _ = os.ReadFile(pa.ImagePath("images/2.png"))
	if !bytes.Equal(got, blue) {
		t.Fatal("setup A shows the wrong card 2")
	}
	again, _ := Analyze(context.Background(), h, "", nil)
	if again.MoveFiles != 0 {
		t.Fatalf("second estimate %d", again.MoveFiles)
	}
	// B's hand-changed card 2 differs from the shared one: listed for the player's choice, not moved.
	if len(again.Differ) != 1 || again.Differ[0].Setup != b || again.Differ[0].Files != 1 || again.Differ[0].Sample != "images/2.png" {
		t.Fatalf("differ %+v", again.Differ)
	}

	// Shrink setup A's set: library art converted once, both set.json updated; B's own card 2 untouched.
	sres, err := Shrink(context.Background(), h, setups.DefaultID, []string{"x-set"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sres.Files == 0 {
		t.Fatal("nothing converted")
	}
	pa, pb = ws(setups.DefaultID), ws(b)
	if pa.Set.Cards[0].Image != "images/1.jpg" || pa.Set.Cards[1].Image != "images/2.jpg" {
		t.Fatalf("A not updated: %v %v", pa.Set.Cards[0].Image, pa.Set.Cards[1].Image)
	}
	if pb.Set.Cards[0].Image != "images/1.jpg" || pb.Set.Cards[1].Image != "images/2.png" {
		t.Fatalf("B wrong: %v %v", pb.Set.Cards[0].Image, pb.Set.Cards[1].Image)
	}
	for _, p := range []*project.Project{pa, pb} {
		for _, c := range p.Set.Cards {
			if _, err := os.Stat(p.ImagePath(c.Image)); err != nil {
				t.Fatalf("%s missing", c.Image)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(lib, "1.png")); err == nil {
		t.Fatal("library png kept")
	}
}

func TestUnusedLibrarySets(t *testing.T) {
	h, _ := newHome(t)
	red := pngBytes(t, color.NRGBA{200, 0, 0, 255})
	addSet(t, h, setups.DefaultID, red, red)
	gone := filepath.Join(project.LibraryDir(h.Root), "old-set", "images")
	_ = os.MkdirAll(gone, 0o755)
	_ = os.WriteFile(filepath.Join(gone, "1.png"), red, 0o644)
	rep, err := Analyze(context.Background(), h, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Unused) != 1 || rep.Unused[0].ID != "old-set" {
		t.Fatalf("unused %+v", rep.Unused)
	}
	freed, err := DeleteUnused(h, "", UnusedSetArt, nil)
	if err != nil || freed != int64(len(red)) {
		t.Fatalf("freed %d %v", freed, err)
	}
	if _, err := os.Stat(gone); err == nil {
		t.Fatal("not deleted")
	}
}

func TestShrinkPreview(t *testing.T) {
	h, _ := newHome(t)
	red := pngBytes(t, color.NRGBA{200, 0, 0, 255})
	addSet(t, h, setups.DefaultID, red, red)
	p, err := Preview(h, setups.DefaultID, "x-set")
	if err != nil {
		t.Fatal(err)
	}
	if p.PNGSize != int64(len(red)) || p.JPEGSize == 0 || !strings.HasPrefix(p.JPEG, "data:image/jpeg;base64,") ||
		!strings.HasPrefix(p.PNG, "data:image/png;base64,") {
		t.Fatalf("preview %+v", p.Card)
	}
	if _, err := os.Stat(filepath.Join(h.Dir(setups.DefaultID), "projects", "x-set", "images", "1.jpg")); err == nil {
		t.Fatal("preview wrote a file")
	}
}

// A setup that downloaded its own PNG copy of a set the library has as JPEG: shrinking it ends with the library copy only.
func TestShrinkDropsCopiesOfLibraryArt(t *testing.T) {
	h, b := newHome(t)
	red, blue := pngBytes(t, color.NRGBA{200, 0, 0, 255}), pngBytes(t, color.NRGBA{0, 0, 200, 255})
	addSet(t, h, setups.DefaultID, red, blue)
	if _, err := MoveAll(context.Background(), h, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Shrink(context.Background(), h, setups.DefaultID, []string{"x-set"}, nil); err != nil {
		t.Fatal(err)
	}
	pb := addSet(t, h, b, red, blue) // re-imported as PNG into setup B (own copy: other format than the library)
	if _, err := Shrink(context.Background(), h, b, []string{"x-set"}, nil); err != nil {
		t.Fatal(err)
	}
	p, err := project.Workspace{Root: h.Dir(b), Library: project.LibraryDir(h.Root)}.Load("x-set")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range p.Set.Cards {
		if !p.InLibrary(c.Image) {
			t.Fatalf("%s still has its own copy", c.Image)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(pb.Folder, "images"))
	if len(entries) != 0 {
		t.Fatalf("leftover files in B: %d", len(entries))
	}
}

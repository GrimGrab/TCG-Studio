package importer

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"tcgstudio/internal/project"
)

// The same set imported into a second setup takes its card art from the shared library: nothing downloaded again.
func TestImportUsesSharedLibrary(t *testing.T) {
	ws := t.TempDir()
	lib := project.LibraryDir(ws)
	img := image.NewNRGBA(image.Rect(0, 0, 600, 838))
	for x := 0; x < 600; x++ { // transparent top row, like Scryfall's rounded corners
		img.Set(x, 0, color.NRGBA{})
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	var calls atomic.Int32
	in := setIn{ID: "x-set", Source: "test", Code: "SET", Name: "Set",
		Cards:  []cardIn{{SourceID: "a", Name: "A", Number: "1", Image: "u1"}, {SourceID: "b", Name: "B", Number: "2", Image: "u2"}},
		Get:    func(context.Context, string) ([]byte, error) { calls.Add(1); return buf.Bytes(), nil },
		Rarity: func(string) string { return "Common" },
	}
	opt := DefaultOptions()
	setupA := project.Workspace{Root: filepath.Join(ws, "setups", "a"), Library: lib}
	setupB := project.Workspace{Root: filepath.Join(ws, "setups", "b"), Library: lib}
	pa, err := buildProject(context.Background(), setupA, in, opt, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("first import downloaded %d", calls.Load())
	}
	if !pa.InLibrary("images/1.png") {
		t.Fatal("art not in the library")
	}
	if _, err := os.Stat(filepath.Join(pa.Folder, "images", "1.png")); err == nil {
		t.Fatal("art also copied into the project")
	}
	calls.Store(0)
	pb, err := buildProject(context.Background(), setupB, in, opt, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || !pb.InLibrary("images/2.png") {
		t.Fatalf("second import downloaded %d", calls.Load())
	}
	if issues := pb.Set.Validate(pb.Folder, pb.LibFolder); len(issues) > 0 {
		for _, i := range issues {
			if i.Level == "error" {
				t.Fatalf("validation: %+v", i)
			}
		}
	}

	// JPEG import of the same set: other format → its own files in the project folder, never mixed.
	calls.Store(0)
	opt.ImageFormat = "jpg"
	setupC := project.Workspace{Root: filepath.Join(ws, "setups", "c"), Library: lib}
	pc, err := buildProject(context.Background(), setupC, in, opt, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || pc.Set.Cards[0].Image != "images/1.jpg" || pc.InLibrary("images/1.jpg") {
		t.Fatalf("jpeg import: calls %d image %s", calls.Load(), pc.Set.Cards[0].Image)
	}
	// The player chose "use the art on this PC": JPEG picked, but the library's PNG art is used and nothing downloaded.
	calls.Store(0)
	opt.UseLibrary = true
	setupD := project.Workspace{Root: filepath.Join(ws, "setups", "d"), Library: lib}
	pd, err := buildProject(context.Background(), setupD, in, opt, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || pd.Set.Cards[0].Image != "images/1.png" || !pd.InLibrary("images/1.png") {
		t.Fatalf("use library: calls %d image %s", calls.Load(), pd.Set.Cards[0].Image)
	}
	opt.UseLibrary = false

	f, _ := os.Open(pc.ImagePath("images/1.jpg"))
	defer f.Close()
	if _, format, err := image.DecodeConfig(f); err != nil || format != "jpeg" {
		t.Fatalf("not a jpeg: %v %v", format, err)
	}
}

func TestOpaqueFillsCorners(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 140))
	for y := 0; y < 140; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.NRGBA{20, 20, 20, 255}) // dark border colour
		}
	}
	img.Set(0, 0, color.NRGBA{}) // transparent corner
	out := Opaque(img)
	r, g, b, a := out.At(0, 0).RGBA()
	if a != 0xffff || r>>8 > 40 || g>>8 > 40 || b>>8 > 40 {
		t.Fatalf("corner = %d %d %d %d", r>>8, g>>8, b>>8, a>>8)
	}
}

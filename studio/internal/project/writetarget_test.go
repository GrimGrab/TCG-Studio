package project

import (
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/setfmt"
)

// New and edited set files go to the set's shared folder; a set kept as its own art writes into its named folder; a file
// the setup still keeps itself is written there.
func TestWriteTarget(t *testing.T) {
	root := t.TempDir()
	ws := Workspace{Root: filepath.Join(root, "setup"), Library: LibraryDir(root)}
	p, err := ws.Create("x-set", "X")
	if err != nil {
		t.Fatal(err)
	}
	lib := ws.LibFolder("x-set")

	rel, err := p.WriteFile("images/pack_texture.png", []byte("new"))
	if err != nil || rel != "images/pack_texture.png" || !fileExists(filepath.Join(lib, "images", "pack_texture.png")) {
		t.Fatalf("new file: rel %q err %v", rel, err)
	}
	if !p.InLibrary(rel) {
		t.Fatal("new file not read from the shared folder")
	}

	own := filepath.Join(p.Folder, "images", "box_texture.png")
	if err := os.MkdirAll(filepath.Dir(own), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(own, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, f := p.WriteTarget("images/box_texture.png"); f != own {
		t.Fatalf("own copy: wrote %s", f)
	}

	p.Set.Cards = []setfmt.Card{{ID: "1", Image: "Mine/images/1.png"}}
	rel, f := p.WriteTarget("images/pack_texture.png")
	if rel != "Mine/images/pack_texture.png" || f != filepath.Join(lib, "Mine", "images", "pack_texture.png") {
		t.Fatalf("art folder: %s %s", rel, f)
	}

	p.LibFolder = ""
	if rel, f := p.WriteTarget("images/x.png"); f != filepath.Join(p.Folder, "Mine", "images", "x.png") || rel != "Mine/images/x.png" {
		t.Fatalf("no library: %s %s", rel, f)
	}
}

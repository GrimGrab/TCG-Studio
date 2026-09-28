package project

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"tcgstudio/internal/setfmt"
)

func TestInstallState(t *testing.T) {
	gameDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(gameDir, "Card Shop Simulator_Data"), 0o755); err != nil {
		t.Fatal(err)
	}
	ws := Workspace{Root: t.TempDir()}
	p := &Project{ID: "dom", Folder: filepath.Join(ws.Root, "dom"),
		Set: &setfmt.Set{ID: "dom", Name: "Dominaria", Packs: []setfmt.Pack{{ID: "booster", PackTexture: "images/pack_texture.png"}}}}
	img := filepath.Join(p.Folder, "images", "pack_texture.png")
	if err := os.MkdirAll(filepath.Dir(img), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(img, []byte("old art"), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	_ = os.Chtimes(img, past, past)

	if s := InstallState(p, gameDir); s != StateNone {
		t.Fatalf("before install: %s", s)
	}
	if err := ws.Install(p, gameDir); err != nil {
		t.Fatal(err)
	}
	if s := InstallState(p, gameDir); s != StateCurrent {
		t.Fatalf("after install: %s", s)
	}

	// New art saved in the project (same size, newer) → stale.
	if err := os.WriteFile(img, []byte("new art"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(img, future, future)
	if s := InstallState(p, gameDir); s != StateStale {
		t.Fatalf("edited image: %s", s)
	}
	if err := ws.Install(p, gameDir); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(img, past, past)
	if s := InstallState(p, gameDir); s != StateCurrent {
		t.Fatalf("after reinstall: %s", s)
	}

	// A set.json change (name) → stale.
	p.Set.Name = "Dominaria United"
	if s := InstallState(p, gameDir); s != StateStale {
		t.Fatalf("renamed set: %s", s)
	}
}

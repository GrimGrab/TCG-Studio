package project

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"tcgstudio/internal/game"
	"tcgstudio/internal/setfmt"
)

// A set whose art is kept as its own in a named shared folder (library\<id>\<Name>\images\…) installs that art into the
// game's Library\<id>\<Name>\…, where the mod's lookup (set folder, then Library\<id>\<path>) finds it. Pack art in the
// shared library still goes into the set's own folder (mods up to 0.15.3 read pack/box art only there).
func TestInstallNamedSharedFolder(t *testing.T) {
	gameDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(gameDir, "Card Shop Simulator_Data"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A mod DLL that reads the shared library.
	u := utf16.Encode([]rune("tcgcc-capability:shared-card-art-library"))
	var dll []byte
	for _, c := range u {
		dll = append(dll, byte(c), byte(c>>8))
	}
	if err := os.MkdirAll(game.PluginDir(gameDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(game.PluginDir(gameDir), "TCGCustomCards.dll"), dll, 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	ws := Workspace{Root: filepath.Join(root, "setup"), Library: filepath.Join(root, "library")}
	rel := "MTG Alpha (Test)/images/1.jpg"
	p := &Project{ID: "lea", Folder: ws.Folder("lea"), LibFolder: ws.LibFolder("lea"),
		Set: &setfmt.Set{ID: "lea", Name: "Alpha", Cards: []setfmt.Card{{ID: "1", Name: "One", Image: rel}},
			Packs: []setfmt.Pack{{ID: "booster", PackIcon: "images/pack_icon.png"}}}}
	for _, f := range []string{rel, "images/pack_icon.png"} {
		lib := filepath.Join(p.LibFolder, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(lib), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lib, []byte("art"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(p.Folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if !p.InLibrary(rel) {
		t.Fatal("not served from the library")
	}
	if err := ws.Install(p, gameDir); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(GameLibraryDir(gameDir), "lea", "MTG Alpha (Test)", "images", "1.jpg")); err != nil || string(b) != "art" {
		t.Fatalf("not installed into the game's library: %v", err)
	}
	if _, err := os.Stat(filepath.Join(game.SetsDir(gameDir), "lea", "images", "pack_icon.png")); err != nil {
		t.Fatalf("pack art not in the set's folder: %v", err)
	}
	if s := InstallState(p, gameDir); s != StateCurrent {
		t.Fatalf("state %s", s)
	}
}

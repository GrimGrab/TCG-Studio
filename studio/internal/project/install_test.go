package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"tcgstudio/internal/game"
)

// fakeGame makes a game folder whose mod DLL carries the library capability (none when withMod is false).
func fakeGame(t *testing.T, withMod bool) string {
	t.Helper()
	gameDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(gameDir, "Card Shop Simulator_Data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(game.PluginDir(gameDir), 0o755); err != nil {
		t.Fatal(err)
	}
	marker := "an older mod"
	if withMod {
		marker = "tcgcc-capability:studio-setup"
	}
	var dll []byte
	for _, c := range utf16.Encode([]rune(marker)) {
		dll = append(dll, byte(c), byte(c>>8))
	}
	if err := os.WriteFile(filepath.Join(game.PluginDir(gameDir), "TCGCustomCards.dll"), dll, 0o644); err != nil {
		t.Fatal(err)
	}
	return gameDir
}

func write(t *testing.T, f, s string, mod time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(f, mod, mod)
}

// Install ticks the set in the setup's installed sets and points the game at the setup and library; nothing is written
// into the game folder. InstallState follows the installed sets and where the game reads from.
func TestInstallState(t *testing.T) {
	root := t.TempDir()
	ws := Workspace{Root: filepath.Join(root, "setup"), Library: LibraryDir(root)}
	write(t, filepath.Join(ws.Root, setupInfoFile), `{"name":"A","installedSets":[]}`, time.Now())
	p, err := ws.Create("dom", "Dominaria")
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.Install(p, fakeGame(t, false)); !errors.Is(err, ModTooOld) {
		t.Fatalf("older mod: %v", err)
	}
	gameDir := fakeGame(t, true)
	if s := ws.InstallState(p, gameDir); s != StateNone {
		t.Fatalf("before install: %s", s)
	}
	if err := ws.Install(p, gameDir); err != nil {
		t.Fatal(err)
	}
	if s := ws.InstallState(p, gameDir); s != StateCurrent {
		t.Fatalf("after install: %s", s)
	}
	if b, _ := os.ReadFile(filepath.Join(ws.Root, setupInfoFile)); !strings.Contains(string(b), `"name": "A"`) || !ws.Installed()["dom"] {
		t.Fatalf("setup.json %s", b)
	}
	sp := game.ReadStudioPaths(gameDir)
	if !game.SameDir(sp.Setup, ws.Root) || !game.SameDir(sp.Library, ws.Library) {
		t.Fatalf("game reads %+v", sp)
	}
	if entries, _ := os.ReadDir(game.PluginDir(gameDir)); len(entries) != 2 { // the DLL and studio-paths.json
		t.Fatalf("files written into the game folder: %v", entries)
	}

	// The game pointed at another setup → stale until installed again.
	if err := game.UpdateStudioPaths(gameDir, func(sp *game.StudioPaths) { sp.Setup = filepath.Join(root, "elsewhere") }); err != nil {
		t.Fatal(err)
	}
	if s := ws.InstallState(p, gameDir); s != StateStale {
		t.Fatalf("other setup: %s", s)
	}
	if err := ws.Install(p, gameDir); err != nil {
		t.Fatal(err)
	}
	if err := ws.Uninstall("dom"); err != nil || ws.InstallState(p, gameDir) != StateNone {
		t.Fatalf("uninstall: %v", err)
	}
}

// Studio writes set files into the set's shared folder.
func TestWriteFile(t *testing.T) {
	root := t.TempDir()
	ws := Workspace{Root: filepath.Join(root, "setup"), Library: LibraryDir(root)}
	p, err := ws.Create("x-set", "X")
	if err != nil {
		t.Fatal(err)
	}
	rel, err := p.WriteFile("images/pack_texture.png", []byte("art"))
	if err != nil || rel != "images/pack_texture.png" {
		t.Fatalf("%q %v", rel, err)
	}
	if b, err := os.ReadFile(filepath.Join(ws.LibFolder("x-set"), "images", "pack_texture.png")); err != nil || string(b) != "art" {
		t.Fatalf("not in the shared folder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(p.Folder, "images")); !os.IsNotExist(err) {
		t.Fatal("wrote into the setup's project folder")
	}
}

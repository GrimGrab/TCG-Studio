package accessories

import (
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/assets"
	"tcgstudio/internal/setfmt"
)

func TestStoreInstallAndResolve(t *testing.T) {
	ws := t.TempDir()
	game := filepath.Join(t.TempDir(), "game")
	if err := os.MkdirAll(filepath.Join(game, "Card Shop Simulator_Data"), 0o755); err != nil {
		t.Fatal(err)
	}
	l, err := Open(filepath.Join(ws, "setups", "a"), assets.For(ws))
	if err != nil {
		t.Fatal(err)
	}
	tex, _ := l.WriteImage("m", "texture", []byte("png bytes"))
	if !assets.IsAsset(tex) || l.Resolve(tex) != assets.For(ws).Path(tex) {
		t.Fatalf("texture %q resolves to %q", tex, l.Resolve(tex))
	}
	if r := l.Resolve("../../escape.png"); filepath.Base(r) != "invalid" {
		t.Fatalf("escape resolved to %s", r)
	}
	// An older per-setup file keeps working next to store files.
	if err := os.MkdirAll(filepath.Join(l.Folder, ImagesDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(l.Folder, ImagesDir, "old_icon.png"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	l.Put(setfmt.Accessory{ID: "m", Kind: "Playmat", Name: "M", Texture: tex, Icon: "images/old_icon.png"})
	for i := 0; i < 2; i++ { // the second install moves the unchanged store file over
		if err := l.Install(game); err != nil {
			t.Fatal(err)
		}
		dir := InstalledDir(game)
		if b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(tex))); err != nil || string(b) != "png bytes" {
			t.Fatalf("install %d: store file %q %v", i, b, err)
		}
		if _, err := os.Stat(filepath.Join(dir, "images", "old_icon.png")); err != nil {
			t.Fatalf("install %d: older file missing", i)
		}
	}
	if files := l.Files(); len(files) != 2 {
		t.Fatalf("files %v", files)
	}
}

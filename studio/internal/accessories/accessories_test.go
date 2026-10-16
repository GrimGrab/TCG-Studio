package accessories

import (
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/assets"
	"tcgstudio/internal/setfmt"
)

func TestStoreSaveAndResolve(t *testing.T) {
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
	if r := l.Resolve("images/old_icon.png"); filepath.Base(r) != "invalid" {
		t.Fatalf("a path outside the store resolved to %s", r)
	}
	icon, _ := l.WriteImage("m", "icon", []byte("icon bytes"))
	l.Put(setfmt.Accessory{ID: "m", Kind: "Playmat", Name: "M", Texture: tex, Icon: icon})
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(InstalledDir(game)); !os.IsNotExist(err) {
		t.Fatal("accessory files written into the game folder")
	}
	if files := l.Files(); len(files) != 2 {
		t.Fatalf("files %v", files)
	}
}

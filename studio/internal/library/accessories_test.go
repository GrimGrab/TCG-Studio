package library

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
)

func TestMoveAccessoriesAndUnused(t *testing.T) {
	h := setups.Home{Root: t.TempDir()}
	dir, err := h.Ensure("1.0")
	if err != nil {
		t.Fatal(err)
	}
	store := assets.For(h.Root)
	put := func(rel, content string) {
		p := filepath.Join(accessories.Folder(dir), filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// An older library: files in the setup, one layer image used by a layout, one leftover nothing uses.
	put("images/mat_texture.png", "texture")
	put("images/mat_icon.png", "icon")
	put("images/src/layer.png", "layer")
	put("images/src/old-copy.fig.obj", "leftover")
	l, _ := accessories.Open(dir, store)
	l.Put(setfmt.Accessory{ID: "mat", Kind: "Playmat", Name: "Mat", Texture: "images/mat_texture.png", Icon: "images/mat_icon.png"})
	l.Meta.Layouts["mat"] = json.RawMessage(`{"version":2,"layers":[{"src":"images/src/layer.png"}]}`)
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}
	unusedRef, _ := store.Put([]byte("nobody uses me"), ".png")

	rep, err := Analyze(context.Background(), h, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	a := rep.Accessories
	if len(a.Setups) != 1 || a.Setups[0].Files != 3 || a.Setups[0].LeftoverFiles != 1 || a.UnusedFiles != 1 {
		t.Fatalf("report %+v", a)
	}

	res, err := MoveAccessories(context.Background(), h, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Files != 4 {
		t.Fatalf("moved %+v", res)
	}
	l2, _ := accessories.Open(dir, store)
	acc := l2.Lib.Accessories[0]
	if !assets.IsAsset(acc.Texture) || !assets.IsAsset(acc.Icon) || !strings.Contains(string(l2.Meta.Layouts["mat"]), assets.Prefix) {
		t.Fatalf("not repointed: %+v %s", acc, l2.Meta.Layouts["mat"])
	}
	if b, _ := os.ReadFile(l2.Resolve(acc.Texture)); string(b) != "texture" {
		t.Fatalf("texture content %q", b)
	}
	if _, err := os.Stat(filepath.Join(accessories.Folder(dir), accessories.ImagesDir)); !os.IsNotExist(err) {
		t.Fatal("old images folder still there")
	}

	freed, n, err := DeleteUnusedAssets(h)
	if err != nil || n != 1 || freed != int64(len("nobody uses me")) {
		t.Fatalf("delete unused: %d %d %v", freed, n, err)
	}
	if _, err := os.Stat(store.Path(unusedRef)); !os.IsNotExist(err) {
		t.Fatal("unused asset kept")
	}
	if _, err := os.Stat(l2.Resolve(acc.Texture)); err != nil {
		t.Fatal("used asset deleted")
	}
}

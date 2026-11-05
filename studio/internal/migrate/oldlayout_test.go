package migrate

import (
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/setups"
)

func put(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOldLayout(t *testing.T) {
	newHome := func(t *testing.T) (setups.Home, string) {
		h := setups.Home{Root: t.TempDir()}
		if _, err := h.Ensure("test"); err != nil {
			t.Fatal(err)
		}
		id, err := h.Active()
		if err != nil {
			t.Fatal(err)
		}
		return h, h.Dir(id)
	}
	t.Run("a new workspace is fine", func(t *testing.T) {
		h, _ := newHome(t)
		if OldLayout(h, "") {
			t.Fatal("flagged")
		}
	})
	t.Run("item files in a setup", func(t *testing.T) {
		h, dir := newHome(t)
		put(t, filepath.Join(dir, "accessories", "images", "a.png"))
		if !OldLayout(h, "") {
			t.Fatal("not flagged")
		}
	})
	t.Run("a named art folder", func(t *testing.T) {
		h, _ := newHome(t)
		put(t, filepath.Join(h.Root, "library", "dom", "my art", "c.png"))
		if !OldLayout(h, "") {
			t.Fatal("not flagged")
		}
	})
	t.Run("parked content, until a setup has taken it over", func(t *testing.T) {
		h, dir := newHome(t)
		put(t, filepath.Join(dir, setups.GameStateDir, "Sets", "dom", "set.json"))
		if !OldLayout(h, "") {
			t.Fatal("not flagged")
		}
		in, err := setups.LoadInfo(dir)
		if err != nil {
			t.Fatal(err)
		}
		in.AdoptedGame = true
		if err := setups.SaveInfo(dir, in); err != nil {
			t.Fatal(err)
		}
		if OldLayout(h, "") {
			t.Fatal("flagged after the upgrade ran")
		}
	})
}

// Run against a real workspace: OLD_LAYOUT_ROOT=<workspace> [OLD_LAYOUT_GAME=<game>] go test -run OldLayoutLive -v
func TestOldLayoutLive(t *testing.T) {
	root := os.Getenv("OLD_LAYOUT_ROOT")
	if root == "" {
		t.Skip("OLD_LAYOUT_ROOT not set")
	}
	t.Logf("old layout: %v", OldLayout(setups.Home{Root: root}, os.Getenv("OLD_LAYOUT_GAME")))
}

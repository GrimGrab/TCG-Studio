package main

import (
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

// Opening the 3D editor again on the same art (its layout not saved yet) reuses the snapshot instead of writing another file;
// other art gets its own.
func TestSnapshotPackBaseReusesCopy(t *testing.T) {
	lib := t.TempDir()
	write := func(rel, data string) {
		f := filepath.Join(lib, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("images/pack_a.png", "art one")
	write("images/pack_b.png", "art two")
	p := &project.Project{ID: "s", LibFolder: lib, Set: &setfmt.Set{}}
	first, err := snapshotPackBase(p, "images/booster_pack", "images/pack_a.png")
	if err != nil {
		t.Fatal(err)
	}
	again, _ := snapshotPackBase(p, "images/booster_pack", "images/pack_a.png")
	other, _ := snapshotPackBase(p, "images/booster_pack", "images/pack_b.png")
	if first != again || first == other {
		t.Fatalf("snapshots %q, %q, %q: want the same art to reuse its copy and other art to get its own", first, again, other)
	}
	if n, _ := filepath.Glob(filepath.Join(lib, "images", "booster_pack_base_*.png")); len(n) != 2 {
		t.Fatalf("got %d snapshot files, want 2", len(n))
	}
}

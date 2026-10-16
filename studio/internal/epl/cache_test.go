package epl

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestForgetAndClearCache(t *testing.T) {
	tmp := t.TempDir()
	cache := filepath.Join(tmp, "cache")
	zip := filepath.Join(tmp, "mod.zip")
	if err := os.WriteFile(zip, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(zip)
	dir := extractDir(zip, st, cache)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	Forget(zip, cache)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("Forget left %s", dir)
	}

	other := filepath.Join(cache, "epl", "leftover")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	ClearCache(cache)
	if _, err := os.Stat(filepath.Join(cache, "epl")); !os.IsNotExist(err) {
		t.Fatal("ClearCache left the epl folder in place")
	}
	for i := 0; i < 50; i++ { // the renamed folder is deleted in the background
		if m, _ := filepath.Glob(filepath.Join(cache, "epl-old-*")); len(m) == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("ClearCache didn't delete the renamed folder")
}

package modconfig

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestDefaultsCoverMod fails when the mod binds a setting the built-in defaults.cfg doesn't have (refresh it, see defaults.go).
// Only literal keys are checked; keys built in a loop (ShowVanilla<Kind>, <Grade>Color, …) are covered by the refresh.
func TestDefaultsCoverMod(t *testing.T) {
	src := filepath.Join("..", "..", "..", "src", "TCGCustomCards")
	if _, err := os.Stat(src); err != nil {
		t.Skip("mod source not found")
	}
	tmp := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(tmp, defaultsCfg, 0o644); err != nil {
		t.Fatal(err)
	}
	secs, err := Read(tmp)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, s := range secs {
		for _, e := range s.Entries {
			have[s.Name+"/"+e.Key] = true
		}
	}
	bind := regexp.MustCompile(`Config\.Bind(?:<[^>]*>)?\(\s*"([^"]+)",\s*"([^"]+)"`)
	n := 0
	err = filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".cs") {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range bind.FindAllStringSubmatch(string(b), -1) {
			n++
			if !have[m[1]+"/"+m[2]] {
				t.Errorf("defaults.cfg lacks [%s] %s — run: go run ./cmd/modcfgdefaults <game cfg>", m[1], m[2])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("no Config.Bind calls found — did the mod source move?")
	}
}

func TestWriteDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "BepInEx", "config", FileName)
	if err := WriteDefaults(path, filepath.Join(dir, "missing.cfg")); err != nil {
		t.Fatal(err)
	}
	secs, err := Read(path)
	if err != nil || len(secs) == 0 {
		t.Fatalf("Read = %v, %v", secs, err)
	}
	for _, s := range secs {
		for _, e := range s.Entries {
			if e.Value != e.Default {
				t.Errorf("[%s] %s = %q, default %q", s.Name, e.Key, e.Value, e.Default)
			}
		}
	}

	// A cached copy with fewer settings than the built-in one is ignored.
	small := filepath.Join(dir, "small.cfg")
	if err := os.WriteFile(small, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteDefaults(path, small); err != nil {
		t.Fatal(err)
	}
	if after, _ := Read(path); len(after) != len(secs) {
		t.Fatal("used the smaller cached copy")
	}
}

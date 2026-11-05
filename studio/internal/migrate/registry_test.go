package migrate

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestRegistry: every entry is marked in the code, and every mark in the code has an entry.
func TestRegistry(t *testing.T) {
	mark := regexp.MustCompile(`migration:([a-z0-9][a-z0-9.-]*)`)
	skip := map[string]bool{"node_modules": true, "wailsjs": true, "build": true, "dist": true, "bin": true, "obj": true}
	found := map[string]string{}
	for _, root := range []string{"../..", "../../../src/TCGCustomCards"} { // studio, the mod
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skip[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			switch filepath.Ext(p) {
			case ".go", ".cs", ".ts", ".svelte":
			default:
				return nil
			}
			if strings.HasPrefix(d.Name(), "registry") {
				return nil
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			for _, m := range mark.FindAllSubmatch(b, -1) {
				found[string(m[1])] = p
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	listed := map[string]bool{}
	for _, m := range Registry {
		if listed[m.ID] {
			t.Errorf("%s is listed twice", m.ID)
		}
		listed[m.ID] = true
		if m.What == "" || m.Since == "" || m.Reads == "" || m.RemoveWhen == "" {
			t.Errorf("%s: every field says something", m.ID)
		}
		if _, ok := found[m.ID]; !ok {
			t.Errorf("%s has no migration:%s mark in the code: remove the entry, or mark the code", m.ID, m.ID)
		}
	}
	for id, p := range found {
		if !listed[id] {
			t.Errorf("%s marks migration:%s, which isn't in the registry", p, id)
		}
	}
}

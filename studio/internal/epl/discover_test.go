package epl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModPath(t *testing.T) {
	root := t.TempDir()
	mk := func(rel string) string {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	plug := mk("dl/BepInEx/plugins/Corpo Capsula TCG/SB01_prefabLoader/SB01.json")
	loose := mk("other/Set_prefabLoader/Set.json")
	plain := mk("plain/X.json")
	zip := mk("mod.zip")
	rar := mk("mod.rar")
	sz := mk("mod.7z")
	for in, want := range map[string]string{
		plug:                      filepath.Join(root, "dl", "BepInEx", "plugins", "Corpo Capsula TCG"),
		loose:                     filepath.Join(root, "other"),
		plain:                     filepath.Join(root, "plain"),
		zip:                       zip,
		rar:                       rar,
		sz:                        sz,
		filepath.Join(root, "dl"): filepath.Join(root, "dl"),
	} {
		if got, err := ModPath(in); err != nil || got != want {
			t.Errorf("ModPath(%s) = %s, %v; want %s", in, got, err, want)
		}
	}
}

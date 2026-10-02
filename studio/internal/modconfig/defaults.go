package modconfig

import (
	_ "embed"
	"os"
	"path/filepath"
)

// defaultsCfg is the mod's config file with every value at its default, so the studio can create the file before the game
// has ever run. Refresh it with `go run ./cmd/modcfgdefaults <game>\BepInEx\config\tcgcustomcards.cfg` after adding mod
// settings (TestDefaultsCoverMod fails until then). BepInEx adds entries it doesn't find, so an older copy still works.
//
//go:embed defaults.cfg
var defaultsCfg []byte

// MakeDefaults writes the config file src with every value reset to its default to dst.
func MakeDefaults(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := writeFile(dst, b); err != nil {
		return err
	}
	_, err = RestoreDefaults(dst, "")
	return err
}

// WriteDefaults creates the config file at path from cached (a defaults copy made from the installed mod's own file, used
// when it has at least as many settings) or else the built-in copy.
func WriteDefaults(path, cached string) error {
	src := defaultsCfg
	if b, err := os.ReadFile(cached); err == nil && entryCount(b) >= entryCount(defaultsCfg) {
		src = b
	}
	if err := writeFile(path, src); err != nil {
		return err
	}
	_, err := RestoreDefaults(path, "")
	return err
}

func entryCount(b []byte) int {
	tmp, err := os.CreateTemp("", "tcgcfg-*.cfg")
	if err != nil {
		return 0
	}
	defer os.Remove(tmp.Name())
	_, _ = tmp.Write(b)
	tmp.Close()
	secs, err := Read(tmp.Name())
	if err != nil {
		return 0
	}
	n := 0
	for _, s := range secs {
		n += len(s.Entries)
	}
	return n
}

func writeFile(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

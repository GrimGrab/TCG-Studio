package modconfig

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
)

// defaultsCfg is the mod's config file with every value at its default, so the studio can create the file before the game
// has ever run. Refresh it with `go run ./cmd/modcfgdefaults <game>\BepInEx\config\tcgcustomcards.cfg` after adding mod
// settings (TestDefaultsCoverMod fails until then), or add the entry by hand in BepInEx's format. AddMissing copies new
// entries from here into an existing file, so a mod update's settings show in the studio without starting the game.
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

// AddMissing adds the settings of the built-in defaults that the config file at path doesn't have yet (a mod update added
// them and the game hasn't run since), at their default values: after the last entry of their section, or in a new section
// at the end. BepInEx keeps such entries. A setting that moved to another section (same key, old section no longer in the
// defaults) keeps the file's value and its old entry is removed — the mod's ConfigMoves does the same. Returns how many
// were added.
func AddMissing(path string) (int, error) {
	return addMissing(path, defaultsCfg)
}

func addMissing(path string, defaults []byte) (int, error) {
	secs, err := Read(path)
	if err != nil {
		return 0, err
	}
	have := map[string]bool{}
	lastLine := map[string]int{} // section → line of its last entry
	byKey := map[string][]Entry{}
	for _, s := range secs {
		for _, e := range s.Entries {
			have[s.Name+"/"+e.Key] = true
			lastLine[s.Name] = e.line
			byKey[e.Key] = append(byKey[e.Key], e)
		}
	}
	blocks := entryBlocks(defaults)
	inDefaults := map[string]bool{}
	for _, b := range blocks {
		inDefaults[b.section+"/"+b.key] = true
	}
	drop := map[int]bool{}       // file lines of moved entries' old blocks
	insert := map[int][]string{} // after file line → lines to add
	var newSecs []string         // sections the file doesn't have, in defaults order
	newBlocks := map[string][]string{}
	n := 0
	for _, b := range blocks {
		if have[b.section+"/"+b.key] {
			continue
		}
		n++
		var old []Entry
		for _, e := range byKey[b.key] {
			if e.Section != b.section && !inDefaults[e.Section+"/"+e.Key] {
				old = append(old, e)
			}
		}
		if len(old) == 1 { // moved: keep the value, drop the old entry (its comment lines + value line)
			b.lines = append([]string(nil), b.lines...)
			b.lines[len(b.lines)-1] = b.key + " = " + old[0].Value
			drop[old[0].line] = true
		}
		if at, ok := lastLine[b.section]; ok {
			insert[at] = append(append(insert[at], ""), b.lines...)
			continue
		}
		if _, ok := newBlocks[b.section]; !ok {
			newSecs = append(newSecs, b.section)
		}
		newBlocks[b.section] = append(append(newBlocks[b.section], ""), b.lines...)
	}
	if n == 0 {
		return 0, nil
	}
	lines, err := readLines(path)
	if err != nil {
		return 0, err
	}
	out := make([]string, 0, len(lines)+n*6)
	for i := len(lines) - 1; i >= 0; i-- { // comment lines above a dropped value line belong to it
		if drop[i] {
			for j := i - 1; j >= 0 && strings.HasPrefix(strings.TrimSpace(lines[j]), "#"); j-- {
				drop[j] = true
			}
		}
	}
	for i, l := range lines {
		if !drop[i] {
			out = append(out, l)
		}
		out = append(out, insert[i]...)
	}
	for _, s := range newSecs {
		if len(out) > 0 && out[len(out)-1] != "" {
			out = append(out, "")
		}
		out = append(out, "["+s+"]")
		out = append(out, newBlocks[s]...)
	}
	return n, writeLines(path, out)
}

type entryBlock struct {
	section, key string
	lines        []string // "## description", "# Setting type: …", …, "Key = value"
}

// entryBlocks splits a config file into one block per entry (its comment lines + value line).
func entryBlocks(b []byte) []entryBlock {
	var out []entryBlock
	section := ""
	var cur []string
	for _, raw := range strings.Split(string(b), "\n") {
		l := strings.TrimRight(raw, "\r")
		t := strings.TrimSpace(l)
		switch {
		case t == "":
			cur = nil
		case strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]"):
			section, cur = t[1:len(t)-1], nil
		case strings.HasPrefix(t, "#"):
			cur = append(cur, l)
		default:
			if k, _, ok := strings.Cut(t, "="); ok && section != "" && len(cur) > 0 {
				out = append(out, entryBlock{section: section, key: strings.TrimSpace(k), lines: append(cur, l)})
			}
			cur = nil
		}
	}
	return out
}

// WithDefaultsMeta replaces each entry's description, default, options and range with the built-in defaults' (same mod
// version as the studio), so wording changes — like an "Only used when …" condition — show before the game has rewritten
// the file. Values stay the file's; entries the defaults don't know keep the file's metadata.
func WithDefaultsMeta(secs []Section) []Section {
	meta := map[string]Entry{}
	for _, s := range parse(strings.Split(strings.ReplaceAll(string(defaultsCfg), "\r\n", "\n"), "\n")) {
		for _, e := range s.Entries {
			meta[s.Name+"/"+e.Key] = e
		}
	}
	for si := range secs {
		for ei := range secs[si].Entries {
			e := &secs[si].Entries[ei]
			if d, ok := meta[secs[si].Name+"/"+e.Key]; ok && d.Type == e.Type {
				e.Description, e.Default, e.Options, e.Min, e.Max = d.Description, d.Default, d.Options, d.Min, d.Max
			}
		}
	}
	return secs
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

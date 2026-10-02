// Package modconfig reads and edits the mod's BepInEx config file (<game>\BepInEx\config\tcgcustomcards.cfg) — the same
// settings as the in-game F1 menu. The file describes itself (description, type, default, range/options per entry), so the
// studio renders whatever settings the installed mod version has. Edits only replace the value line; everything else is kept.
// The mod watches the file and reloads it, so settings marked "Applies live" change while the game runs.
package modconfig

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const FileName = "tcgcustomcards.cfg"

func Path(gameDir string) string { return filepath.Join(gameDir, "BepInEx", "config", FileName) }

type Entry struct {
	Section     string   `json:"section"`
	Key         string   `json:"key"`
	Value       string   `json:"value"`
	Type        string   `json:"type"` // Boolean, Single, Int32, String, Color, KeyboardShortcut, or an enum type name
	Default     string   `json:"default"`
	Description string   `json:"description"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	Options     []string `json:"options,omitempty"` // enum values
	line        int
}

type Section struct {
	Name    string  `json:"name"`
	Entries []Entry `json:"entries"`
}

// Read parses the file. Entries without a "# Setting type" header (left over from older mod versions) are skipped.
func Read(path string) ([]Section, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	return parse(lines), nil
}

func parse(lines []string) []Section {
	var out []Section
	var cur *Section
	var pending Entry
	var desc []string
	reset := func() { pending = Entry{}; desc = nil }
	for i, raw := range lines {
		l := strings.TrimSpace(raw)
		switch {
		case l == "":
			continue
		case strings.HasPrefix(l, "[") && strings.HasSuffix(l, "]"):
			out = append(out, Section{Name: l[1 : len(l)-1]})
			cur = &out[len(out)-1]
			reset()
		case strings.HasPrefix(l, "## "):
			if cur != nil { // file header comments come before the first section
				desc = append(desc, strings.TrimPrefix(l, "## "))
			}
		case strings.HasPrefix(l, "# "):
			k, v, ok := strings.Cut(strings.TrimPrefix(l, "# "), ":")
			if !ok {
				continue
			}
			v = strings.TrimSpace(v)
			switch k {
			case "Setting type":
				pending.Type = v
			case "Default value":
				pending.Default = v
			case "Acceptable values":
				for _, o := range strings.Split(v, ",") {
					pending.Options = append(pending.Options, strings.TrimSpace(o))
				}
			case "Acceptable value range":
				// "From 0 to 1.5"
				f := strings.Fields(v)
				if len(f) == 4 {
					if a, err := strconv.ParseFloat(f[1], 64); err == nil {
						pending.Min = &a
					}
					if b, err := strconv.ParseFloat(f[3], 64); err == nil {
						pending.Max = &b
					}
				}
			}
		default:
			k, v, ok := strings.Cut(l, "=")
			if ok && cur != nil && pending.Type != "" {
				e := pending
				e.Section, e.Key, e.Value, e.line = cur.Name, strings.TrimSpace(k), strings.TrimSpace(v), i
				e.Description = strings.Join(desc, " ")
				cur.Entries = append(cur.Entries, e)
			}
			reset()
		}
	}
	// Drop sections that only held orphaned values.
	kept := out[:0]
	for _, s := range out {
		if len(s.Entries) > 0 {
			kept = append(kept, s)
		}
	}
	return kept
}

// Set replaces one entry's value (validated against its type/range/options) and writes the file back.
func Set(path, section, key, value string) (Entry, error) {
	secs, err := Read(path)
	if err != nil {
		return Entry{}, err
	}
	var e *Entry
	for si := range secs {
		for ei := range secs[si].Entries {
			if secs[si].Name == section && secs[si].Entries[ei].Key == key {
				e = &secs[si].Entries[ei]
			}
		}
	}
	if e == nil {
		return Entry{}, fmt.Errorf("setting [%s] %s not found", section, key)
	}
	value = strings.TrimSpace(value)
	if err := validate(*e, value); err != nil {
		return Entry{}, err
	}
	lines, err := readLines(path)
	if err != nil {
		return Entry{}, err
	}
	lines[e.line] = e.Key + " = " + value
	if err := writeLines(path, lines); err != nil {
		return Entry{}, err
	}
	e.Value = value
	return *e, nil
}

// RestoreDefaults sets every entry of a section and its sub-sections ("Foil" also resets "Foil - Base"; all sections when
// section is "") back to its default value in one write.
// Returns the number of values that changed.
func RestoreDefaults(path, section string) (int, error) {
	secs, err := Read(path)
	if err != nil {
		return 0, err
	}
	lines, err := readLines(path)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, s := range secs {
		if section != "" && s.Name != section && !strings.HasPrefix(s.Name, section+" - ") {
			continue
		}
		for _, e := range s.Entries {
			if e.Value != e.Default {
				lines[e.line] = e.Key + " = " + e.Default
				n++
			}
		}
	}
	if n == 0 {
		return 0, nil
	}
	return n, writeLines(path, lines)
}

func writeLines(path string, lines []string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\r\n")+"\r\n"), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func validate(e Entry, v string) error {
	switch e.Type {
	case "Boolean":
		if v != "true" && v != "false" {
			return fmt.Errorf("%s must be true or false", e.Key)
		}
	case "Single", "Double", "Int32":
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("%s must be a number", e.Key)
		}
		if e.Type == "Int32" && f != float64(int64(f)) {
			return fmt.Errorf("%s must be a whole number", e.Key)
		}
		if (e.Min != nil && f < *e.Min) || (e.Max != nil && f > *e.Max) {
			return fmt.Errorf("%s must be between %v and %v", e.Key, *e.Min, *e.Max)
		}
	case "Color":
		if len(v) != 8 && len(v) != 6 {
			return fmt.Errorf("%s must be a colour like RRGGBBAA", e.Key)
		}
		if _, err := strconv.ParseUint(v, 16, 32); err != nil {
			return fmt.Errorf("%s must be a hex colour", e.Key)
		}
	default:
		if len(e.Options) > 0 {
			for _, o := range e.Options {
				if o == v {
					return nil
				}
			}
			return fmt.Errorf("%s must be one of %s", e.Key, strings.Join(e.Options, ", "))
		}
	}
	if strings.ContainsAny(v, "\r\n") {
		return fmt.Errorf("%s: value may not contain line breaks", e.Key)
	}
	return nil
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		lines = append(lines, strings.TrimRight(sc.Text(), "\r"))
	}
	return lines, sc.Err()
}

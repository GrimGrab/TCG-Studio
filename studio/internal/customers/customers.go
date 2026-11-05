// Package customers backs the Customers page: the built-in traits and lines (defaults.json, the same file the mod embeds),
// the setup's own lines (<setup>\game\customer-lines.json, read by the mod's Cast) and the regulars the game made in the
// player's saves (the side-car's "personas"), so they can be adopted and edited. Customers made on the page live in the
// setup's item library (setfmt.Customer, content kind "customer").
package customers

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/setfmt"
)

//go:embed defaults.json
var defaultsJSON []byte

//go:embed default-animations.json
var defaultAnimationsJSON []byte

// DefaultAnimations are the built-in animations and which play at each moment (tools\make-default-animations.py; the mod embeds the same file).
type DefaultAnimations struct {
	Animations []setfmt.Animation  `json:"animations"`
	Moves      map[string][]string `json:"moves"`
}

// LoadDefaultAnimations parses the embedded default-animations.json.
func LoadDefaultAnimations() (DefaultAnimations, error) {
	var d DefaultAnimations
	err := json.Unmarshal(defaultAnimationsJSON, &d)
	return d, err
}

type Trait struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Opposite    string              `json:"opposite,omitempty"`
	Requires    string              `json:"requires,omitempty"`
	Weight      float64             `json:"weight,omitempty"`
	Effects     map[string]float64  `json:"effects"`
	Lines       map[string][]string `json:"lines"`
}

type Situation struct {
	ID string `json:"id"`
	// Mood is how the line sounds (the talking sounds: happy, angry, sad, excited; empty = neutral).
	Mood  string `json:"mood,omitempty"`
	Label string `json:"label"`
}

// Defaults is defaults.json: what every customer starts from.
type Defaults struct {
	Nicknames  []NickDeed          `json:"nicknames"`
	Names      map[string][]string `json:"names"`
	Base       map[string]float64  `json:"base"`
	Traits     []Trait             `json:"traits"`
	Situations []Situation         `json:"situations"`
	Lines      map[string][]string `json:"lines"`
	// Employees: perks are like traits, with their own situations and lines.
	EmployeePerks      []Perk              `json:"employeePerks"`
	EmployeeSituations []Situation         `json:"employeeSituations"`
	EmployeeLines      map[string][]string `json:"employeeLines"`
}

// Perk is an employee perk (the mod: Runtime/Employees/EmployeeLife).
type Perk struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Opposite    string              `json:"opposite,omitempty"`
	Effects     map[string]float64  `json:"effects"`
	Lines       map[string][]string `json:"lines,omitempty"`
}

// NickDeed is what earns a nickname; the game keeps the rules, Studio lists the words.
type NickDeed struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Words []string `json:"words"`
}

// LoadDefaults parses the embedded defaults.json.
func LoadDefaults() (Defaults, error) {
	var d Defaults
	err := json.Unmarshal(defaultsJSON, &d)
	return d, err
}

// TraitIDs is the set of built-in trait ids.
func (d Defaults) TraitIDs() map[string]bool {
	out := map[string]bool{}
	for _, t := range d.Traits {
		out[t.ID] = true
	}
	return out
}

// LinesFile is <setup>\game\customer-lines.json.
const LinesFile = "customer-lines.json"

// SetupLines is the setup's lines file: per situation, lines every customer says on top of the built-in ones, and built-in
// lines switched off (mod: Traits.LinesFor).
type SetupLines struct {
	Lines   map[string][]string `json:"lines"`
	Removed map[string][]string `json:"removed,omitempty"`
	// Generated names per list (NameLists): added, and built-in ones switched off.
	Names    map[string][]string `json:"names,omitempty"`
	NamesOff map[string][]string `json:"namesOff,omitempty"`
	// Employees' lines per situation: added, and built-in ones switched off.
	EmployeeLines   map[string][]string `json:"employeeLines,omitempty"`
	EmployeeRemoved map[string][]string `json:"employeeRemoved,omitempty"`
}

// NameLists are the lists generated names come from.
var NameLists = []string{"female", "male", "last"}

// LoadLines reads the setup's lines (empty when it has none). gameDir is the setup's game folder.
func LoadLines(gameDir string) (SetupLines, error) {
	var d SetupLines
	b, err := os.ReadFile(filepath.Join(gameDir, LinesFile))
	if os.IsNotExist(err) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	err = json.Unmarshal(b, &d)
	return d, err
}

// SaveLines writes the setup's lines; situations without lines are left out, and nothing at all removes the file.
func SaveLines(gameDir string, d SetupLines) error {
	clean := SetupLines{Lines: Clean(d.Lines), Removed: Clean(d.Removed), Names: Clean(d.Names), NamesOff: Clean(d.NamesOff), EmployeeLines: Clean(d.EmployeeLines), EmployeeRemoved: Clean(d.EmployeeRemoved)}
	p := filepath.Join(gameDir, LinesFile)
	if len(clean.Lines) == 0 && len(clean.Removed) == 0 && len(clean.Names) == 0 && len(clean.NamesOff) == 0 && len(clean.EmployeeLines) == 0 && len(clean.EmployeeRemoved) == 0 {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	b, err := json.MarshalIndent(clean, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		return err
	}
	return fsutil.WriteAtomic(p, b)
}

// Clean trims lines and drops empty ones and situations left without any (nil when nothing is left).
func Clean(lines map[string][]string) map[string][]string {
	var out map[string][]string
	for k, list := range lines {
		var keep []string
		for _, l := range list {
			if l = strings.TrimSpace(l); l != "" {
				keep = append(keep, l)
			}
		}
		if len(keep) > 0 {
			if out == nil {
				out = map[string][]string{}
			}
			out[k] = keep
		}
	}
	return out
}

// Saved is a regular the game made in one of the player's saves (not made on the Customers page).
type Saved struct {
	setfmt.Customer
	Visits  int     `json:"visits"`
	Spent   float64 `json:"spent"`
	LastDay int     `json:"lastDay"`
	Origin  string  `json:"origin"` // "promoted" (a walk-in who came back), …
	Slot    string  `json:"slot"`   // save file it was found in ("slot1")
}

// savedRegular is a Persona as the mod saves it: model, skill and lastDay are left out while -1.
type savedRegular struct {
	ID      string              `json:"id"`
	Name    string              `json:"name"`
	Female  bool                `json:"female"`
	Model   *int                `json:"model"`
	Traits  []string            `json:"traits"`
	Skill   *int                `json:"skill"`
	Lines   map[string][]string `json:"lines"`
	Visits  int                 `json:"visits"`
	Spent   float64             `json:"spent"`
	LastDay *int                `json:"lastDay"`
	Origin  string              `json:"origin"`
}

type sideCar struct {
	Personas *struct {
		Regulars []savedRegular `json:"regulars"`
	} `json:"personas"`
}

func orMinus1(v *int) int {
	if v == nil {
		return -1
	}
	return *v
}

// FromSaves lists the regulars in the side-car saves of dir (<save folder>\TCGCustomCards), newest save first; a regular
// in several saves is listed once (from the newest). Customers already on the page (have) are left out. Unreadable files
// are skipped.
func FromSaves(dir string, have map[string]bool) []Saved {
	entries, _ := os.ReadDir(dir)
	type file struct {
		name string
		mod  int64
	}
	var files []file
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasPrefix(n, "slot") || !strings.HasSuffix(n, ".json") {
			continue
		}
		if info, err := e.Info(); err == nil {
			files = append(files, file{n, info.ModTime().UnixNano()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod > files[j].mod })
	seen := map[string]bool{}
	var out []Saved
	for _, f := range files {
		var sc sideCar
		if b, err := readSideCar(filepath.Join(dir, f.name)); err != nil || json.Unmarshal(b, &sc) != nil || sc.Personas == nil {
			continue
		}
		for _, r := range sc.Personas.Regulars {
			if r.ID == "" || seen[r.ID] || have[r.ID] || r.Origin == "authored" {
				continue
			}
			seen[r.ID] = true
			c := setfmt.Customer{ID: r.ID, Name: r.Name, Female: r.Female, Model: orMinus1(r.Model), Traits: r.Traits,
				Skill: orMinus1(r.Skill), Lines: r.Lines}
			if c.Traits == nil {
				c.Traits = []string{}
			}
			out = append(out, Saved{Customer: c, Visits: r.Visits, Spent: r.Spent, LastDay: orMinus1(r.LastDay), Origin: r.Origin,
				Slot: strings.TrimSuffix(f.name, ".json")})
		}
	}
	return out
}

// readSideCar returns a side-car's JSON: gzip-compressed (current mod) or plain (older versions).
func readSideCar(p string) ([]byte, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	if len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b {
		r, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(r)
	}
	return b, nil
}

// Package setups manages named setups inside the studio workspace. A setup is a complete studio root (projects\ + the
// accessory library, its installed sets in setup.json) plus game\ (the parked mod config, the global card back and Shop
// look pictures the mod reads from there) and, while it isn't the active one, its parked game saves (saves\). The game's
// mod reads the active setup straight from its folder (game.StudioPathsFile), so switching only moves the mod config and
// the saves and points the game at the other setup. Setup files (.tcgsetup, without saves) are written and read by
// internal/share; archive.go here reads the older format versions.
//
//	<workspace>\setups.json         registry: active setup + order
//	<workspace>\switch.json         journal of an unfinished switch
//	<workspace>\cache\              shared (Scryfall)
//	<workspace>\setups\<id>\        setup.json, projects\, accessories\, game\, saves\
package setups

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/setfmt"
)

const (
	FormatVersion = 1
	InfoFile      = "setup.json"
	RegistryFile  = "setups.json"
	JournalFile   = "switch.json"
	SetupsDir     = "setups"
	GameStateDir  = "game"  // inside a setup
	SavesDir      = "saves" // inside a setup; never exported
	DefaultID     = "default"
	cfgFile       = "tcgcustomcards.cfg"
)

// Info is a setup's setup.json.
type Info struct {
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	Created       time.Time `json:"created"`
	StudioVersion string    `json:"studioVersion,omitempty"`
	FormatVersion int       `json:"formatVersion"`
	// InstalledSets = the project ids the game's mod loads while this setup is active (Studio's Install / Uninstall).
	InstalledSets []string `json:"installedSets"`
	// AdoptedGame is set once the setup has taken over what older Studio versions kept in the game folder (upgrade.AdoptGame).
	AdoptedGame bool `json:"adoptedGame,omitempty"`
}

type Registry struct {
	Active string   `json:"active"`
	Order  []string `json:"order"`
}

// RemoveSaveBackups deletes the saves-backup-<time> folders older versions copied the game's saves into before the first
// switch (the switch journal covers an interrupted switch; nothing reads these copies).
func (h Home) RemoveSaveBackups() {
	old, _ := filepath.Glob(filepath.Join(h.Root, "saves-backup-*"))
	for _, d := range old {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			os.RemoveAll(d)
		}
	}
}

type Summary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Created     time.Time `json:"created"`
	Folder      string    `json:"folder"`
	Active      bool      `json:"active"`
	Sets        int       `json:"sets"`
	Accessories int       `json:"accessories"`
	Furniture   int       `json:"furniture"`
	Decorations int       `json:"decorations"`
	HasSaves    bool      `json:"hasSaves"` // parked saves (inactive setups only; the active one's saves are in the game)
}

// Home is the workspace root that holds the setups.
type Home struct{ Root string }

func (h Home) Dir(id string) string { return filepath.Join(h.Root, SetupsDir, id) }
func (h Home) CacheDir() string     { return filepath.Join(h.Root, "cache") }

func (h Home) Registry() (*Registry, error) {
	r := &Registry{}
	b, err := os.ReadFile(filepath.Join(h.Root, RegistryFile))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, r); err != nil {
		return nil, fmt.Errorf("%s: %w", RegistryFile, err)
	}
	return r, nil
}

func (h Home) saveRegistry(r *Registry) error {
	return writeJSON(filepath.Join(h.Root, RegistryFile), r)
}

// Active returns the active setup's id.
func (h Home) Active() (string, error) {
	r, err := h.Registry()
	if err != nil {
		return "", err
	}
	return r.Active, nil
}

func LoadInfo(dir string) (*Info, error) {
	b, err := os.ReadFile(filepath.Join(dir, InfoFile))
	if err != nil {
		return nil, err
	}
	in := &Info{}
	if err := json.Unmarshal(b, in); err != nil {
		return nil, fmt.Errorf("%s: %w", InfoFile, err)
	}
	return in, nil
}

// SaveInfo writes a whole setup.json (a new setup, an export). Changes to an existing one go through EditInfo.
func SaveInfo(dir string, in *Info) error {
	path := filepath.Join(dir, InfoFile)
	return fsutil.WithLock(path, func() error { return writeJSON(path, in) })
}

// EditInfo changes a setup's setup.json under its file lock (installs edit the same file, see project.setInstalled).
func EditInfo(dir string, edit func(in *Info)) error {
	path := filepath.Join(dir, InfoFile)
	return fsutil.WithLock(path, func() error {
		in, err := LoadInfo(dir)
		if err != nil {
			return err
		}
		edit(in)
		return writeJSON(path, in)
	})
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Ensure prepares the workspace: on the first run it creates the setup "default" — from the layout before setups
// (<workspace>\projects + accessories) by renaming the two folders, rolled back if a rename fails — and it repairs a registry
// whose active setup is gone. Returns the active setup's folder.
func (h Home) Ensure(studioVersion string) (string, error) {
	if r, err := h.Registry(); err == nil {
		r.Order = h.knownIDs(r.Order)
		if r.Active == "" || !fsutil.Exists(filepath.Join(h.Dir(r.Active), InfoFile)) {
			if len(r.Order) == 0 {
				return "", errors.New("no setups found in the workspace")
			}
			r.Active = r.Order[0]
		}
		return h.Dir(r.Active), h.saveRegistry(r)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	dir := h.Dir(DefaultID)
	if fsutil.Exists(filepath.Join(dir, InfoFile)) { // registry lost, setups still there
		r := &Registry{Active: DefaultID, Order: h.knownIDs(nil)}
		return dir, h.saveRegistry(r)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	var moved []string
	for _, name := range []string{"projects", "accessories"} {
		src := filepath.Join(h.Root, name)
		if !fsutil.Exists(src) {
			continue
		}
		if err := os.Rename(src, filepath.Join(dir, name)); err != nil {
			for _, m := range moved {
				_ = os.Rename(filepath.Join(dir, m), filepath.Join(h.Root, m))
			}
			_ = os.Remove(dir)
			return "", fmt.Errorf("couldn't move %s into the setups folder (is a file open?): %w", name, err)
		}
		moved = append(moved, name)
	}
	in := &Info{Name: "My Setup", Created: time.Now(), StudioVersion: studioVersion, FormatVersion: FormatVersion, InstalledSets: []string{}}
	if err := SaveInfo(dir, in); err != nil {
		return "", err
	}
	return dir, h.saveRegistry(&Registry{Active: DefaultID, Order: []string{DefaultID}})
}

// knownIDs returns order (minus setups that no longer exist) followed by setup folders that aren't listed yet.
func (h Home) knownIDs(order []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, id := range order {
		if !seen[id] && fsutil.Exists(filepath.Join(h.Dir(id), InfoFile)) {
			seen[id] = true
			out = append(out, id)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(h.Root, SetupsDir))
	for _, e := range entries {
		if e.IsDir() && !seen[e.Name()] && !strings.Contains(e.Name(), ".") && fsutil.Exists(filepath.Join(h.Dir(e.Name()), InfoFile)) {
			seen[e.Name()] = true
			out = append(out, e.Name())
		}
	}
	return out
}

// List returns every setup in registry order.
func (h Home) List() ([]Summary, error) {
	r, err := h.Registry()
	if err != nil {
		return nil, err
	}
	out := []Summary{}
	for _, id := range h.knownIDs(r.Order) {
		dir := h.Dir(id)
		in, err := LoadInfo(dir)
		if err != nil {
			continue
		}
		s := Summary{ID: id, Name: in.Name, Description: in.Description, Created: in.Created, Folder: dir, Active: id == r.Active}
		entries, _ := os.ReadDir(filepath.Join(dir, "projects"))
		for _, e := range entries {
			if e.IsDir() && fsutil.Exists(filepath.Join(dir, "projects", e.Name(), "set.json")) {
				s.Sets++
			}
		}
		if lib, err := setfmt.LoadAccessories(filepath.Join(dir, "accessories", "accessories.json")); err == nil {
			s.Accessories, s.Furniture, s.Decorations = len(lib.Accessories), len(lib.Furniture), len(lib.Decorations)
		}
		s.HasSaves = !s.Active && HasSaves(filepath.Join(dir, SavesDir))
		out = append(out, s)
	}
	return out, nil
}

var nonID = regexp.MustCompile(`[^a-z0-9_-]+`)

// newID makes a free folder name from a setup name.
func (h Home) newID(name string) string {
	base := strings.Trim(nonID.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if base == "" {
		base = "setup"
	}
	if len(base) > 40 {
		base = strings.Trim(base[:40], "-")
	}
	id := base
	for n := 2; fsutil.Exists(h.Dir(id)) || fsutil.Exists(h.Dir(id)+".importing") || fsutil.Exists(h.Dir(id)+".creating"); n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}

// uniqueName returns name, or "name (2)", "name (3)", … when another setup already has it.
func (h Home) uniqueName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Setup"
	}
	list, _ := h.List()
	taken := map[string]bool{}
	for _, s := range list {
		taken[strings.ToLower(s.Name)] = true
	}
	out := name
	for n := 2; taken[strings.ToLower(out)]; n++ {
		out = fmt.Sprintf("%s (%d)", name, n)
	}
	return out
}

// Create makes a new setup: empty when from is "", else a copy of setup from (with its parked saves when withSaves).
// Copying the active setup's game state and saves is the caller's job (Snapshot / CopyGameSaves), since they live in the game.
func (h Home) Create(name, from string, withSaves bool, studioVersion string) (string, error) {
	r, err := h.Registry()
	if err != nil {
		return "", err
	}
	name = h.uniqueName(name)
	id := h.newID(name)
	dst := h.Dir(id)
	tmp := dst + ".creating"
	_ = os.RemoveAll(tmp)
	in := &Info{InstalledSets: []string{}}
	if from != "" {
		src := h.Dir(from)
		if in, err = LoadInfo(src); err != nil {
			return "", err
		}
		err := fsutil.CopyTree(src, tmp, func(rel string, dir bool) bool {
			return rel == InfoFile || (!withSaves && rel == SavesDir) || strings.HasPrefix(rel, SavesDir+"-")
		})
		if err != nil {
			_ = os.RemoveAll(tmp)
			return "", err
		}
	}
	in.Name, in.Created, in.StudioVersion, in.FormatVersion = name, time.Now(), studioVersion, FormatVersion
	if err := SaveInfo(tmp, in); err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	r.Order = append(h.knownIDs(r.Order), id)
	r.Order = h.knownIDs(r.Order)
	return id, h.saveRegistry(r)
}

// Update changes a setup's name and description.
func (h Home) Update(id, name, description string) error {
	in, err := LoadInfo(h.Dir(id))
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("the name can't be empty")
	}
	if !strings.EqualFold(name, in.Name) {
		name = h.uniqueName(name)
	}
	description = strings.TrimSpace(description)
	return EditInfo(h.Dir(id), func(in *Info) { in.Name, in.Description = name, description })
}

// Delete removes an inactive setup with everything in it, including its parked saves.
func (h Home) Delete(id string) error {
	r, err := h.Registry()
	if err != nil {
		return err
	}
	if id == r.Active {
		return errors.New("switch to another setup before deleting this one")
	}
	if id == "" || strings.ContainsAny(id, `/\.:`) || !fsutil.Exists(filepath.Join(h.Dir(id), InfoFile)) {
		return errors.New("unknown setup")
	}
	if err := os.RemoveAll(h.Dir(id)); err != nil {
		return err
	}
	r.Order = h.knownIDs(r.Order)
	return h.saveRegistry(r)
}

// Move puts a setup at position to in the list.
func (h Home) Move(id string, to int) error {
	r, err := h.Registry()
	if err != nil {
		return err
	}
	order := h.knownIDs(r.Order)
	i := -1
	for k, v := range order {
		if v == id {
			i = k
		}
	}
	if i < 0 || to < 0 || to >= len(order) || to == i {
		return nil
	}
	order = append(order[:i:i], order[i+1:]...)
	order = append(order[:to], append([]string{id}, order[to:]...)...)
	r.Order = order
	return h.saveRegistry(r)
}

// Folders is the setup folders of a list.
func Folders(list []Summary) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = s.Folder
	}
	return out
}

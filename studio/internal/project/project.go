// Package project manages the studio workspace: one folder per set with set.json, images and studio.json metadata.
package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"tcgstudio/internal/game"
	"tcgstudio/internal/setfmt"
)

const (
	SetFile  = "set.json"
	MetaFile = "studio.json"
)

// Meta is studio-only data that the mod never reads.
type Meta struct {
	Source        string              `json:"source"` // "scryfall" | "manual"
	ScryfallCode  string              `json:"scryfallCode,omitempty"`
	ReleasedAt    string              `json:"releasedAt,omitempty"`
	ImportedAt    time.Time           `json:"importedAt"`
	PricesUpdated time.Time           `json:"pricesUpdated"`
	Tier          int                 `json:"tier"` // progression order for gamify (0 = unset)
	Pricing       *Pricing            `json:"pricing,omitempty"`
	Cards         map[string]CardMeta `json:"cards"`
	// PackArt holds the pack editor's layouts per pack id: {"pack": layout, "box": layout} (layout = the accessory editor's
	// JSON, see frontend lib/accessoryArt.ts). Packs without an entry use generated or chosen images only.
	PackArt map[string]map[string]json.RawMessage `json:"packArt,omitempty"`
}

// Pricing remembers the last gamify settings so a price refresh can re-apply them.
type Pricing struct {
	Mode        string  `json:"mode"`
	BorderCurve string  `json:"borderCurve"`
	TierStep    float64 `json:"tierStep"`
	Position    float64 `json:"position"` // place on the vanilla curve (0 = Basic … 8 = Ascension)
}

// CardMeta keeps the real-world data behind an imported card.
type CardMeta struct {
	ScryfallID string   `json:"scryfallId,omitempty"`
	Name       string   `json:"name,omitempty"`   // Scryfall name (no variant tags)
	Layout     string   `json:"layout,omitempty"` // Scryfall layout (normal, split, transform, adventure, …)
	Colors     []string `json:"colors,omitempty"`
	TypeLine   string   `json:"typeLine,omitempty"`
	ManaCost   string   `json:"manaCost,omitempty"`
	CMC        float64  `json:"cmc,omitempty"`
	Power      string   `json:"power,omitempty"`
	Toughness  string   `json:"toughness,omitempty"`
	SrcRarity  string   `json:"srcRarity,omitempty"`
	Variant    []string `json:"variant,omitempty"` // Borderless / Showcase / Extended Art / Etched / Full Art
	USD        *float64 `json:"usd,omitempty"`
	USDFoil    *float64 `json:"usdFoil,omitempty"`
	EUR        *float64 `json:"eur,omitempty"`
	Locked     bool     `json:"locked,omitempty"` // gamify/refresh leave this card's price alone
}

type Project struct {
	ID     string      `json:"id"`
	Folder string      `json:"folder"`
	Set    *setfmt.Set `json:"set"`
	Meta   *Meta       `json:"meta"`
}

type Summary struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Cards      int       `json:"cards"`
	Packs      int       `json:"packs"`
	Source     string    `json:"source"`
	Code       string    `json:"code"`
	ReleasedAt string    `json:"releasedAt"`
	Tier       int       `json:"tier"`
	Modified   time.Time `json:"modified"`
	Installed  bool      `json:"installed"`
	// InstallState: "none" (not in the game), "current" (the game has this version) or "stale" (edited since the last install).
	InstallState string `json:"installState"`
	Cover        string `json:"cover"` // relative image for the list thumbnail
}

type Workspace struct{ Root string }

func DefaultRoot() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "TCG Studio")
}

func (w Workspace) ProjectsDir() string { return filepath.Join(w.Root, "projects") }
func (w Workspace) CacheDir() string    { return filepath.Join(w.Root, "cache") }
func (w Workspace) Folder(id string) string {
	return filepath.Join(w.ProjectsDir(), id)
}

func (w Workspace) List(gameDir string) ([]Summary, error) {
	entries, err := os.ReadDir(w.ProjectsDir())
	if os.IsNotExist(err) {
		return []Summary{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Summary
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p, err := w.Load(e.Name())
		if err != nil {
			continue
		}
		info, _ := os.Stat(filepath.Join(p.Folder, SetFile))
		s := Summary{ID: p.ID, Name: p.Set.Name, Cards: len(p.Set.Cards), Packs: len(p.Set.Packs),
			Source: p.Meta.Source, Code: p.Meta.ScryfallCode, ReleasedAt: p.Meta.ReleasedAt, Tier: p.Meta.Tier}
		if info != nil {
			s.Modified = info.ModTime()
		}
		if gameDir != "" {
			s.InstallState = InstallState(p, gameDir)
			s.Installed = s.InstallState != StateNone
		}
		if len(p.Set.Cards) > 0 {
			s.Cover = p.Set.Cards[0].Image
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	if out == nil {
		out = []Summary{}
	}
	return out, nil
}

func (w Workspace) Load(id string) (*Project, error) {
	folder := w.Folder(id)
	set, err := setfmt.Load(filepath.Join(folder, SetFile))
	if err != nil {
		return nil, err
	}
	meta := &Meta{Source: "manual", Cards: map[string]CardMeta{}}
	if b, err := os.ReadFile(filepath.Join(folder, MetaFile)); err == nil {
		_ = json.Unmarshal(b, meta)
		if meta.Cards == nil {
			meta.Cards = map[string]CardMeta{}
		}
	}
	p := &Project{ID: id, Folder: folder, Set: set, Meta: meta}
	FillMtg(p) // older imports: saved (and installed) with the project's next save/install
	return p, nil
}

// Save writes set.json + studio.json. The folder name is the set id and never changes after creation.
func (w Workspace) Save(p *Project) error {
	if err := os.MkdirAll(p.Folder, 0o755); err != nil {
		return err
	}
	if err := p.Set.Save(filepath.Join(p.Folder, SetFile)); err != nil {
		return err
	}
	b, err := json.MarshalIndent(p.Meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(p.Folder, MetaFile), b, 0o644)
}

// Create makes a new empty project; fails if the id is taken.
func (w Workspace) Create(id, name string) (*Project, error) {
	if !setfmt.SafeID(id) {
		return nil, fmt.Errorf("set id %q may not contain spaces, ':', '/', '|'", id)
	}
	folder := w.Folder(id)
	if _, err := os.Stat(folder); err == nil {
		return nil, fmt.Errorf("a project with id %q already exists", id)
	}
	p := &Project{ID: id, Folder: folder, Set: setfmt.NewSet(id, name),
		Meta: &Meta{Source: "manual", ImportedAt: time.Now(), Cards: map[string]CardMeta{}}}
	return p, w.Save(p)
}

func (w Workspace) Delete(id string) error {
	if !setfmt.SafeID(id) {
		return fmt.Errorf("bad id")
	}
	return os.RemoveAll(w.Folder(id))
}

// Install copies set.json and every referenced image into the game's Sets folder (replacing an older copy).
func (w Workspace) Install(p *Project, gameDir string) error {
	if !game.IsGameDir(gameDir) {
		return fmt.Errorf("game folder not set")
	}
	dest := filepath.Join(game.SetsDir(gameDir), p.ID)
	tmp := dest + ".installing"
	_ = os.RemoveAll(tmp)
	for rel := range installFiles(p) {
		if err := copyFile(filepath.Join(p.Folder, rel), filepath.Join(tmp, rel)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := p.Set.Save(filepath.Join(tmp, SetFile)); err != nil {
		return err
	}
	_ = os.RemoveAll(dest)
	return os.Rename(tmp, dest)
}

// installFiles lists the project files Install copies into the game (relative paths).
func installFiles(p *Project) map[string]bool {
	files := map[string]bool{}
	add := func(rel string) {
		if rel != "" {
			files[rel] = true
		}
	}
	add(p.Set.CardBack)
	for _, c := range p.Set.Cards {
		add(c.Image)
	}
	for _, pk := range p.Set.Packs {
		add(pk.PackTexture)
		add(pk.PackIcon)
		add(pk.BoxTexture)
		add(pk.BoxIcon)
	}
	return files
}

const (
	StateNone    = "none"
	StateCurrent = "current"
	StateStale   = "stale"
)

// InstallState compares the project with its copy in the game: StateNone when not installed, StateStale when set.json differs
// from what Install would write or an image was edited since (missing, other size, or newer in the project), else StateCurrent.
// Uses sizes and modification times, not hashes, so it stays cheap for sets with hundreds of card images.
func InstallState(p *Project, gameDir string) string {
	if gameDir == "" || !setfmt.SafeID(p.ID) {
		return StateNone
	}
	dest := filepath.Join(game.SetsDir(gameDir), p.ID)
	installed, err := os.ReadFile(filepath.Join(dest, SetFile))
	if err != nil {
		return StateNone
	}
	want, err := p.Set.Marshal()
	if err != nil || !bytes.Equal(bytes.TrimSpace(installed), bytes.TrimSpace(want)) {
		return StateStale
	}
	for rel := range installFiles(p) {
		src, err := os.Stat(filepath.Join(p.Folder, filepath.FromSlash(rel)))
		if err != nil {
			continue // Install skips missing files too
		}
		dst, err := os.Stat(filepath.Join(dest, filepath.FromSlash(rel)))
		if err != nil || dst.Size() != src.Size() || src.ModTime().After(dst.ModTime()) {
			return StateStale
		}
	}
	return StateCurrent
}

func Uninstall(id, gameDir string) error {
	if !setfmt.SafeID(id) || !game.IsGameDir(gameDir) {
		return fmt.Errorf("bad id or game folder")
	}
	return os.RemoveAll(filepath.Join(game.SetsDir(gameDir), id))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

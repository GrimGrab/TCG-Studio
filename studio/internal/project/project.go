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
	"tcgstudio/internal/origin"
	"tcgstudio/internal/setfmt"
)

const (
	SetFile  = "set.json"
	MetaFile = "studio.json"
)

// Meta is studio-only data that the mod never reads.
type Meta struct {
	Source        string         `json:"source"` // import source id ("scryfall", "tcgdex", …) or "manual"
	ScryfallCode  string         `json:"scryfallCode,omitempty"`
	SetCode       string         `json:"setCode,omitempty"`     // the source's set id for sources other than Scryfall
	SourceDir     string         `json:"sourceDir,omitempty"`   // image-folder imports: the folder read (Refresh prices re-reads its cards.csv)
	RarityOrder   []string       `json:"rarityOrder,omitempty"` // the set's own rarities, lowest first, when the source has no fixed list
	Origin        *origin.Origin `json:"origin,omitempty"`      // converted content: where it came from (EPL mod)
	Lang          string         `json:"lang,omitempty"`        // card language of the import, when the source has several
	ReleasedAt    string         `json:"releasedAt,omitempty"`
	ImportedAt    time.Time      `json:"importedAt"`
	PricesUpdated time.Time      `json:"pricesUpdated"`
	Tier          int            `json:"tier"` // progression order for gamify (0 = unset)
	Pricing       *Pricing       `json:"pricing,omitempty"`
	// SrcPriceDefaults: the source's own border/foil multipliers (EPL mods' price generators); Real pricing keeps them.
	SrcPriceDefaults *setfmt.PriceDefaults `json:"srcPriceDefaults,omitempty"`
	Cards            map[string]CardMeta   `json:"cards"`
	// PackArt holds the pack editor's layouts per pack id: {"pack": layout, "box": layout} (layout = the accessory editor's
	// JSON, see frontend lib/accessoryArt.ts). Packs without an entry use generated or chosen images only.
	PackArt map[string]map[string]json.RawMessage `json:"packArt,omitempty"`
}

// SourceCode is the imported set's code at its source (empty for hand-made sets).
func (m *Meta) SourceCode() string {
	if m.ScryfallCode != "" {
		return m.ScryfallCode
	}
	return m.SetCode
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
	SourceID   string   `json:"sourceId,omitempty"` // the card's id at a source other than Scryfall (e.g. TCGdex "sv03.5-006")
	Name       string   `json:"name,omitempty"`     // source name (no variant tags)
	Layout     string   `json:"layout,omitempty"`   // Scryfall layout (normal, split, transform, adventure, …)
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
	ID     string `json:"id"`
	Folder string `json:"folder"`
	// LibFolder is the set's folder in the shared card-art library (<workspace>\library\<id>), "" when there is none.
	// Images are looked up in Folder first, then here (ImagePath).
	LibFolder string      `json:"libFolder"`
	Set       *setfmt.Set `json:"set"`
	Meta      *Meta       `json:"meta"`
}

// ImagePath is the file for an image path of the set: the project's own folder first (art changed in this setup), then the
// shared card-art library. Returns the project-folder path when neither has it.
func (p *Project) ImagePath(rel string) string {
	own := filepath.Join(p.Folder, filepath.FromSlash(rel))
	if p.LibFolder == "" {
		return own
	}
	if _, err := os.Stat(own); err == nil {
		return own
	}
	if lib := filepath.Join(p.LibFolder, filepath.FromSlash(rel)); fileExists(lib) {
		return lib
	}
	return own
}

// InLibrary reports whether rel is served from the shared library (not overridden in the project folder).
func (p *Project) InLibrary(rel string) bool {
	return p.LibFolder != "" && p.ImagePath(rel) != filepath.Join(p.Folder, filepath.FromSlash(rel))
}

// WriteTarget is where Studio writes a set file it makes or changes (pack/box art, Smart-art parts, card back, photos,
// rotated card art) given as rel ("images/…"). New and edited files go to the set's shared folder, so a setup keeps only
// its game info and an edit shows in every setup using the shared art; a set kept as its own art (ArtFolder) writes
// into that named folder, never over the ordinary shared files. A file this setup still keeps itself (an own copy the
// Storage page hasn't moved yet) is written there, so the setup never starts reading a different file. Returns the path
// to store in set.json / studio.json and the file to write; without a library both are in the project folder.
func (p *Project) WriteTarget(rel string) (string, string) {
	if af := p.ArtFolder(); af != "" && !strings.HasPrefix(rel, af+"/") {
		rel = af + "/" + rel
	}
	own := filepath.Join(p.Folder, filepath.FromSlash(rel))
	if p.LibFolder == "" || fileExists(own) {
		return rel, own
	}
	return rel, filepath.Join(p.LibFolder, filepath.FromSlash(rel))
}

// WriteFile writes a set file through WriteTarget (folders created) and returns the path to store.
func (p *Project) WriteFile(rel string, b []byte) (string, error) {
	rel, f := p.WriteTarget(rel)
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return "", err
	}
	return rel, os.WriteFile(f, b, 0o644)
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

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
	Cover        string `json:"cover"`            // relative image for the list thumbnail
	Origin       string `json:"origin,omitempty"` // converted content: the mod it came from (for display and sorting)
}

// OriginName is the mod a converted set came from ("" = not converted). Sets converted before origins were recorded show
// their mod folder's name.
func (m *Meta) OriginName() string {
	switch {
	case m.Origin != nil && m.Origin.Mod != "":
		return m.Origin.Mod
	case m.Source == "epl" && m.SourceDir != "":
		return strings.TrimSuffix(filepath.Base(m.SourceDir), filepath.Ext(m.SourceDir))
	}
	return ""
}

// Workspace is one setup's projects. Library is the workspace-wide card-art library root (<workspace>\library), shared by
// every setup; "" = no library (images only in project folders).
type Workspace struct {
	Root    string
	Library string
}

// LibraryDirName is the shared card-art library folder: in the studio workspace and in the mod folder (<plugin>\Library).
const (
	LibraryDirName     = "library"
	GameLibraryDirName = "Library"
)

// LibraryDir is the card-art library of a studio workspace (the folder that holds the setups).
func LibraryDir(workspace string) string { return filepath.Join(workspace, LibraryDirName) }

// GameLibraryDir is the mod's shared card-art folder.
func GameLibraryDir(gameDir string) string {
	return filepath.Join(game.PluginDir(gameDir), GameLibraryDirName)
}

// ImagePath resolves an image path of project id like Project.ImagePath (project folder, then the shared library).
func (w Workspace) ImagePath(id, rel string) string {
	return (&Project{ID: id, Folder: w.Folder(id), LibFolder: w.LibFolder(id)}).ImagePath(rel)
}

// LibFolder is set id's folder in the shared card-art library ("" without a library).
func (w Workspace) LibFolder(id string) string {
	if w.Library == "" {
		return ""
	}
	return filepath.Join(w.Library, id)
}

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
			Source: p.Meta.Source, Code: p.Meta.SourceCode(), ReleasedAt: p.Meta.ReleasedAt, Tier: p.Meta.Tier,
			Origin: p.Meta.OriginName()}
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
	p := &Project{ID: id, Folder: folder, LibFolder: w.LibFolder(id), Set: set, Meta: meta}
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
	p := &Project{ID: id, Folder: folder, LibFolder: w.LibFolder(id), Set: setfmt.NewSet(id, name),
		Meta: &Meta{Source: "manual", ImportedAt: time.Now(), Cards: map[string]CardMeta{}}}
	return p, w.Save(p)
}

func (w Workspace) Delete(id string) error {
	if !setfmt.SafeID(id) {
		return fmt.Errorf("bad id")
	}
	return os.RemoveAll(w.Folder(id))
}

// Install copies set.json and every referenced image into the game's Sets folder (replacing an older copy). Card art that
// lives in the shared library (not changed in this setup) goes to the mod's Library folder instead when the installed mod
// reads it: copied once, kept across setup switches, so installing and switching only copy the set's small files.
func (w Workspace) Install(p *Project, gameDir string) error {
	if !game.IsGameDir(gameDir) {
		return fmt.Errorf("game folder not set")
	}
	shared := game.ModHasLibrary(gameDir)
	dest := filepath.Join(game.SetsDir(gameDir), p.ID)
	tmp := dest + ".installing"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil { // a set without images copies no file that would create it
		return err
	}
	for rel := range installFiles(p) {
		src, dst := installPaths(p, gameDir, rel, shared)
		if toGameLibrary(p, rel, shared) {
			if !upToDate(src, dst) { // the game's library copy: only what's missing or changed
				if err := copyFile(src, dst); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
			continue
		}
		if err := copyFile(src, filepath.Join(tmp, filepath.FromSlash(rel))); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := p.Set.Save(filepath.Join(tmp, SetFile)); err != nil {
		return err
	}
	_ = os.RemoveAll(dest)
	return os.Rename(tmp, dest)
}

// installPaths returns where rel comes from (project folder or library) and where Install puts it in the game: the set's
// folder, or <plugin>\Library\<id> for library art when the mod reads it (shared).
func installPaths(p *Project, gameDir, rel string, shared bool) (src, dst string) {
	src = p.ImagePath(rel)
	if toGameLibrary(p, rel, shared) {
		return src, filepath.Join(GameLibraryDir(gameDir), p.ID, filepath.FromSlash(rel))
	}
	return src, filepath.Join(game.SetsDir(gameDir), p.ID, filepath.FromSlash(rel))
}

// toGameLibrary reports whether Install puts rel in the game's Library folder: shared library files the mod looks up
// there (card images and the card back, through SetDef.Resolve). Pack/box art always goes into the set's folder — mods up
// to 0.15.3 read it from there only — and it is only a few files.
func toGameLibrary(p *Project, rel string, shared bool) bool {
	if !shared || !p.InLibrary(rel) {
		return false
	}
	for _, pk := range p.Set.Packs {
		if rel == pk.PackTexture || rel == pk.PackIcon || rel == pk.BoxTexture || rel == pk.BoxIcon {
			return false
		}
	}
	return true
}

// upToDate reports whether dst is a current copy of src (same size, not older).
func upToDate(src, dst string) bool {
	s, err := os.Stat(src)
	if err != nil {
		return true // nothing to copy
	}
	d, err := os.Stat(dst)
	return err == nil && d.Size() == s.Size() && !s.ModTime().After(d.ModTime())
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
	shared := game.ModHasLibrary(gameDir)
	for rel := range installFiles(p) {
		src, dst := installPaths(p, gameDir, rel, shared)
		if _, err := os.Stat(src); err != nil {
			continue // Install skips missing files too
		}
		if !upToDate(src, dst) {
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

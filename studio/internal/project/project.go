// Package project manages the studio workspace: one folder per set with set.json, images and studio.json metadata.
package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"tcgstudio/internal/fsutil"
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
	// Pictures: set files cut from a picture with Studio's crop box (the set's card back, pack/box textures and icons) by
	// slot ("cardBack", "pack/<pack id>/<field>"); saving the set makes each file again when its picture or crop changed.
	Pictures map[string]PictureSource `json:"pictures,omitempty"`
}

// PictureSource is the picture a set file was cut from and the part used.
type PictureSource struct {
	File string    `json:"file"`           // the picture (a set file path, "images/…")
	Crop []float64 `json:"crop,omitempty"` // x, y, w, h fractions of the picture from its top-left; empty = the middle
	// Out is the file saving last cut it into. When set.json names another file, something else replaced it (the pack art
	// editor, Smart generate, Generate) and the picture is dropped.
	Out string `json:"out,omitempty"`
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
	Variant    []string `json:"variant,omitempty"`  // Borderless / Showcase / Extended Art / Etched / Full Art
	BackName   string   `json:"backName,omitempty"` // double-faced cards (transform, modal DFC): the back face's name
	USD        *float64 `json:"usd,omitempty"`
	USDFoil    *float64 `json:"usdFoil,omitempty"`
	EUR        *float64 `json:"eur,omitempty"`
	Locked     bool     `json:"locked,omitempty"` // gamify/refresh leave this card's price alone
}

type Project struct {
	ID     string `json:"id"`
	Folder string `json:"folder"` // the setup's project folder: set.json and studio.json (game info)
	// LibFolder is the set's shared folder (<workspace>\library\<id>): every file the set refers to lives there.
	LibFolder string      `json:"libFolder"`
	Set       *setfmt.Set `json:"set"`
	Meta      *Meta       `json:"meta"`
}

// ImagePath is the file for a path the set refers to ("images/…"), in the set's shared folder.
func (p *Project) ImagePath(rel string) string {
	return filepath.Join(p.LibFolder, filepath.FromSlash(rel))
}

// WriteTarget is where Studio writes a set file it makes or changes (pack/box art, Smart-art parts, card back, photos,
// rotated card art): the set's shared folder, so an edit shows in every setup using the set. Returns the path to store
// in set.json / studio.json (rel itself) and the file to write.
func (p *Project) WriteTarget(rel string) (string, string) {
	return rel, p.ImagePath(rel)
}

// WriteFile writes a set file into the set's shared folder (folders created) and returns the path to store.
func (p *Project) WriteFile(rel string, b []byte) (string, error) {
	rel, f := p.WriteTarget(rel)
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return "", err
	}
	return rel, fsutil.WriteAtomic(f, b)
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
	Cover        string `json:"cover"`                // relative image for the list thumbnail
	Origin       string `json:"origin,omitempty"`     // where it came from: the mod or shared file's source (display, sorting)
	OriginKind   string `json:"originKind,omitempty"` // origin.KindEPL / origin.KindShared
	Collection   string `json:"collection,omitempty"` // set.json collection (game/series), for the Set tab's suggestions
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

// Workspace is one setup's projects (Root = the setup folder, or the catalog). Library is the workspace-wide shared
// folder of set files (<workspace>\library) every setup reads from.
type Workspace struct {
	Root    string
	Library string
}

// LibraryDirName is the workspace's shared library folder (every set's files); GameLibraryDirName the game-folder copy
// older Studio versions made (unused now: the mod reads the workspace's library, see game.StudioPathsFile).
const (
	LibraryDirName     = "library"
	GameLibraryDirName = "Library"
)

// LibraryDir is the card-art library of a studio workspace (the folder that holds the setups).
func LibraryDir(workspace string) string { return filepath.Join(workspace, LibraryDirName) }

// GameLibraryDir is the game-folder copy of set files older Studio versions made (library.RemoveGameCopies deletes it).
func GameLibraryDir(gameDir string) string {
	return filepath.Join(game.PluginDir(gameDir), GameLibraryDirName)
}

// ImagePath is the file for a path set id refers to (see Project.ImagePath).
func (w Workspace) ImagePath(id, rel string) string {
	return (&Project{ID: id, LibFolder: w.LibFolder(id)}).ImagePath(rel)
}

// LibFolder is set id's shared folder.
func (w Workspace) LibFolder(id string) string { return filepath.Join(w.Library, id) }

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
			Origin: p.Meta.OriginName(), Collection: p.Set.Collection}
		if s.Origin != "" {
			s.OriginKind = origin.KindEPL
			if p.Meta.Origin != nil {
				s.OriginKind = p.Meta.Origin.Kind
			}
		}
		if info != nil {
			s.Modified = info.ModTime()
		}
		if game.IsGameDir(gameDir) {
			s.InstallState = w.InstallState(p, gameDir)
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
	FillMtg(p)        // older imports: saved (and installed) with the project's next save/install
	MigratePackArt(p) // pack editor layouts made for an older net
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

// ModTooOld is Install's error when the game's mod doesn't read set files from Studio's library.
var ModTooOld = errors.New("the mod in the game is older than this TCG Studio — open Game & mod and click Install / Repair")

// Install puts the set into the game: it is saved and ticked in the setup's installed sets (setup.json), and the mod is
// pointed at this setup and the shared library (game.StudioPathsFile). The game reads set.json from the project folder
// and the set's files from the library when it next starts; nothing is written into the game folder.
func (w Workspace) Install(p *Project, gameDir string) error {
	if !game.IsGameDir(gameDir) {
		return fmt.Errorf("game folder not set")
	}
	if !game.ModReadsLibrary(gameDir) {
		return ModTooOld
	}
	if err := w.Save(p); err != nil {
		return err
	}
	if err := w.setInstalled(p.ID, true); err != nil {
		return err
	}
	return game.UpdateStudioPaths(gameDir, func(sp *game.StudioPaths) {
		// Another setup of this workspace is the active one's business (a switch may have finished while this install ran);
		// only a missing setup or another workspace is repointed here.
		if sp.Setup == "" || !fsutil.Exists(sp.Setup) || !game.SameDir(sp.Library, w.Library) {
			sp.Library, sp.Setup = w.Library, w.Root
		}
	})
}

const (
	StateNone    = "none"
	StateCurrent = "current"
	StateStale   = "stale"
)

// InstallState is StateNone when the set isn't in the setup's installed sets, StateStale when the game doesn't read this
// setup and library (another setup or workspace, or an older mod), else StateCurrent.
func (w Workspace) InstallState(p *Project, gameDir string) string {
	if gameDir == "" || !setfmt.SafeID(p.ID) || !w.Installed()[p.ID] {
		return StateNone
	}
	sp := game.ReadStudioPaths(gameDir)
	if !game.ModReadsLibrary(gameDir) || !game.SameDir(sp.Library, w.Library) || !game.SameDir(sp.Setup, w.Root) {
		return StateStale
	}
	return StateCurrent
}

// Uninstall takes set id out of the setup's installed sets (the game no longer loads it).
func (w Workspace) Uninstall(id string) error {
	if !setfmt.SafeID(id) {
		return fmt.Errorf("bad id")
	}
	return w.setInstalled(id, false)
}

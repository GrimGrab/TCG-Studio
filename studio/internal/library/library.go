// Package library runs the Storage page: how much disk the workspace, the shared set files (<workspace>\library\<setId>),
// the shared item store and each setup take, and the player's actions on them:
//   - Shrink: a set's PNG card art becomes JPEG (about 6x smaller), set.json of every setup with the set updated;
//   - things no setup uses (shared set folders, unused files in them, the game's copies, store files, catalog-only
//     entries) can be listed and deleted.
//
// Bringing a workspace from older storage layouts to shared storage is internal/upgrade.
package library

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/art"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/catalog"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/game"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

// Progress of a report or an action.
type Progress struct {
	Message string `json:"message"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
}

// SetInfo is one shared set: its folder and who uses it.
type SetInfo struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	UsedBy   []string `json:"usedBy"`   // setup names ("Catalog" for a catalog template)
	Shared   int64    `json:"shared"`   // every file it uses from its shared folder
	PNG      int64    `json:"png"`      // PNG card art (importer file names), what Shrink would convert
	PNGFiles int      `json:"pngFiles"` //
	ShrinkTo int64    `json:"shrinkTo"` // estimated size of that art as JPEG
}

// ItemInfo is an accessory, furniture piece or decoration of a setup.
type ItemInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`   // accessory kind (Playmat, …), "Furniture" or "Decoration"
	Shared int64  `json:"shared"` // its files in the shared store
}

// SetupInfo is one setup: its own folder broken down, and the sets and items it uses.
type SetupInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
	Size   int64  `json:"size"` // the setup's own folder (= the parts below)
	// Breakdown of Size.
	Info  int64      `json:"info"`  // game info: set.json, studio.json, accessories.json, setup.json
	Saves int64      `json:"saves"` // parked game saves (inactive setups)
	Game  int64      `json:"game"`  // the setup's game state (mod settings, global card back, shop look, hand-installed content)
	Other int64      `json:"other"`
	Sets  []string   `json:"sets"` // set ids (Report.Sets)
	Items []ItemInfo `json:"items"`
}

// Report is the Storage page.
type Report struct {
	Workspace       int64         `json:"workspace"`     // whole workspace folder
	Library         int64         `json:"library"`       // <workspace>\library
	Assets          int64         `json:"assets"`        // <workspace>\assets
	Catalog         int64         `json:"catalog"`       // <workspace>\catalog
	SetupsSize      int64         `json:"setupsSize"`    // <workspace>\setups (game info and parked saves)
	Cache           int64         `json:"cache"`         // <workspace>\cache (import lists, unpacked EPL downloads)
	GameLibrary     int64         `json:"gameLibrary"`   // copies of set and item files older Studio versions put in the game folder
	LibraryLinked   bool          `json:"libraryLinked"` // the game's mod reads set files from this workspace's library
	GameFound       bool          `json:"gameFound"`
	ModReadsLibrary bool          `json:"modReadsLibrary"`
	Sets            []SetInfo     `json:"sets"` // every set any setup or the catalog has, biggest first
	Setups          []SetupInfo   `json:"setups"`
	Unused          []UnusedEntry `json:"unused"` // everything no setup uses, whatever the kind
	StoreBytes      int64         `json:"storeBytes"`
	StoreFiles      int           `json:"storeFiles"`
	PlainSaves      PlainSaves    `json:"plainSaves"`
	Kept            int64         `json:"kept"`    // files the storage upgrade kept instead of deleting (workspace.Dir.Kept)
	KeptDir         string        `json:"keptDir"` // mod save files not compressed yet (CompressSaves)
	Backups         []BackupInfo  `json:"backups"` // TCGStudio_backup_* folders in the game folder
}

// loc is one setup's copy of a set.
type loc struct {
	setup string
	p     *project.Project
}

// catalogSetup is the loc.setup of the catalog's set templates (not a setup id: those never contain ':'). They count as
// users of the shared files, but aren't listed as a setup.
const catalogSetup = ":catalog"

// sets loads every project of every setup and of the catalog (last), grouped by set id.
func sets(h setups.Home) (map[string][]loc, []setups.Summary, error) {
	list, err := h.List()
	if err != nil {
		return nil, nil, err
	}
	out := map[string][]loc{}
	dirs := append(append([]setups.Summary{}, list...), setups.Summary{ID: catalogSetup, Folder: catalog.For(h.Root).Root})
	for _, s := range dirs {
		ws := workspace.Dir(h.Root).Projects(s.Folder)
		entries, _ := os.ReadDir(ws.ProjectsDir())
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if p, err := ws.Load(e.Name()); err == nil {
				out[e.Name()] = append(out[e.Name()], loc{setup: s.ID, p: p})
			}
		}
	}
	return out, list, nil
}

func sortedIDs(m map[string][]loc) []string {
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// cardImages returns the set's card image paths (each once).
func cardImages(p *project.Project) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range p.Set.Cards {
		for _, img := range []string{c.Image, c.BackImage} {
			if img != "" && !seen[img] {
				seen[img] = true
				out = append(out, img)
			}
		}
	}
	return out
}

func size(p string) int64 {
	if info, err := os.Stat(p); err == nil {
		return info.Size()
	}
	return 0
}

func dirSize(dir string) (n int64, files int) {
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
				files++
			}
		}
		return nil
	})
	return
}

// Analyze measures everything for the Storage page. Read-only.
func Analyze(ctx context.Context, h setups.Home, gameDir string, progress func(Progress)) (*Report, error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	progress(Progress{Message: "Reading setups…"})
	bySet, list, err := sets(h)
	if err != nil {
		return nil, err
	}
	r := &Report{Sets: []SetInfo{}, Setups: []SetupInfo{}, Unused: []UnusedEntry{}, Backups: []BackupInfo{}}
	r.Workspace, _ = dirSize(h.Root)
	r.Library, _ = dirSize(project.LibraryDir(h.Root))
	r.Assets, _ = dirSize(filepath.Join(h.Root, assets.DirName))
	r.Catalog, _ = dirSize(catalog.For(h.Root).Root)
	r.SetupsSize, _ = dirSize(filepath.Join(h.Root, setups.SetupsDir))
	r.Cache, _ = dirSize(h.CacheDir())
	if game.IsGameDir(gameDir) {
		r.GameFound = true
		r.ModReadsLibrary = game.ModReadsLibrary(gameDir)
		for _, dir := range gameCopies(gameDir) {
			n, _ := dirSize(dir)
			if info, err := os.Stat(dir); err == nil && !info.IsDir() {
				n = info.Size()
			}
			r.GameLibrary += n
		}
		r.LibraryLinked = game.SameDir(game.LibraryPath(gameDir), project.LibraryDir(h.Root))
		r.Backups = Backups(gameDir)
	}
	names := map[string]string{catalogSetup: "Catalog"}
	for _, sm := range list {
		names[sm.ID] = sm.Name
	}

	// Every set once: its shared files, users, and PNG art with its JPEG estimate for Shrink.
	ids := sortedIDs(bySet)
	for i, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		progress(Progress{Message: "Measuring sets…", Done: i + 1, Total: len(ids)})
		locs := bySet[id]
		set := SetInfo{ID: id, Name: locs[0].p.Set.Name, UsedBy: []string{}}
		files := map[string]bool{}
		for _, l := range locs {
			set.UsedBy = append(set.UsedBy, names[l.setup])
			for _, rel := range l.p.Files() {
				files[rel] = true
			}
		}
		for rel := range files {
			set.Shared += size(locs[0].p.ImagePath(rel))
		}
		for _, rel := range pngCards(locs) {
			set.PNG += size(locs[0].p.ImagePath(rel))
			set.PNGFiles++
		}
		if set.PNGFiles > 0 {
			set.ShrinkTo = int64(float64(set.PNG) * jpegRatio(locs[0].p))
		}
		r.Sets = append(r.Sets, set)
	}
	sort.SliceStable(r.Sets, func(i, j int) bool { return r.Sets[i].Shared > r.Sets[j].Shared })

	// Per setup: its folder broken down, the sets and items it uses.
	store := workspace.Dir(h.Root).Store()
	for _, sm := range list {
		si := SetupInfo{ID: sm.ID, Name: sm.Name, Active: sm.Active, Sets: []string{}, Items: []ItemInfo{}}
		for _, id := range ids {
			for _, l := range bySet[id] {
				if l.setup == sm.ID {
					si.Sets = append(si.Sets, id)
				}
			}
		}
		breakdown(&si, sm.Folder)
		if l, err := workspace.Dir(h.Root).Items(sm.Folder); err == nil {
			si.Items = itemsOf(l)
		}
		r.Setups = append(r.Setups, si)
	}
	if entries, err := store.List(); err == nil {
		for _, e := range entries {
			r.StoreBytes += e.Size
			r.StoreFiles++
		}
	}
	r.Unused = unusedEntries(h, bySet, list)
	r.PlainSaves = checkPlainSaves(h, r.GameFound)
	r.KeptDir = workspace.Dir(h.Root).Kept()
	r.Kept, _ = dirSize(r.KeptDir)
	return r, nil
}

// breakdown splits a setup folder's size into its parts (the parts add up to Size).
func breakdown(si *SetupInfo, dir string) {
	_ = filepath.WalkDir(dir, func(f string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		n := info.Size()
		si.Size += n
		rel, _ := filepath.Rel(dir, f)
		top, _, _ := strings.Cut(filepath.ToSlash(rel), "/")
		switch {
		case top == setups.SavesDir || strings.HasPrefix(top, setups.SavesDir+"-"):
			si.Saves += n
		case top == setups.GameStateDir:
			si.Game += n
		case strings.EqualFold(filepath.Ext(f), ".json"):
			si.Info += n
		default:
			si.Other += n
		}
		return nil
	})
}

// isPNG reports a PNG card image under an importer file name (images/<cid>.png), not art added or changed in Studio
// under another name. Shrink only converts those.
func isPNG(rel string) bool {
	return strings.EqualFold(path.Ext(rel), ".png") && strings.HasPrefix(rel, "images/") &&
		!strings.Contains(strings.TrimPrefix(rel, "images/"), "/")
}

// pngCards lists the PNG card images (isPNG) any copy of a set uses that exist in its shared folder.
func pngCards(locs []loc) []string {
	seen := map[string]bool{}
	var out []string
	for _, l := range locs {
		for _, rel := range cardImages(l.p) {
			if !seen[rel] && isPNG(rel) && size(l.p.ImagePath(rel)) > 0 {
				seen[rel] = true
				out = append(out, rel)
			}
		}
	}
	sort.Strings(out)
	return out
}

// jpegRatio encodes a few of p's PNG card images as JPEG and returns the average size ratio.
func jpegRatio(p *project.Project) float64 {
	var png, jpg int64
	n := 0
	for _, rel := range cardImages(p) {
		if n == 3 {
			break
		}
		if !strings.EqualFold(path.Ext(rel), ".png") {
			continue
		}
		b, err := os.ReadFile(p.ImagePath(rel))
		if err != nil {
			continue
		}
		out, err := toJPEG(b)
		if err != nil {
			continue
		}
		png += int64(len(b))
		jpg += int64(len(out))
		n++
	}
	if png == 0 {
		return 0.17 // measured average for card scans
	}
	return float64(jpg) / float64(png)
}

func toJPEG(b []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	return art.EncodeCardJPEG(img)
}

// HumanSize formats bytes for messages.
func HumanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// gameCopies is the content older Studio versions put into the game folder: installed sets (<plugin>\Sets), their files
// (<plugin>\Library), the accessory library and its files (<plugin>\Accessories), the global card back and the Shop look
// pictures (<plugin>\ShopSign). The mod reads all of it from the workspace now (game.StudioPathsFile); the game folder
// holds only the mod.
func gameCopies(gameDir string) []string {
	plugin := game.PluginDir(gameDir)
	return []string{game.SetsDir(gameDir), project.GameLibraryDir(gameDir), accessories.InstalledDir(gameDir),
		filepath.Join(plugin, "card_back.png"), filepath.Join(plugin, "ShopSign")}
}

// RemoveGameCopies moves the game-folder copies (gameCopies) into <game>\TCGStudio_backup_<time>\ (like Setup's
// backups), so anything Studio couldn't take over — a hand-made file, an older Studio's only copy — can still be found.
// Returns the bytes moved out of the plugin folder and the backup folder ("" when there was nothing).
func RemoveGameCopies(gameDir string) (int64, string, error) {
	var moved int64
	backup := ""
	for _, src := range gameCopies(gameDir) {
		info, err := os.Stat(src)
		if err != nil {
			continue
		}
		n := info.Size()
		if info.IsDir() {
			n, _ = dirSize(src)
		}
		if backup == "" {
			base := filepath.Join(gameDir, "TCGStudio_backup_"+time.Now().Format("20060102-150405"))
			backup = base
			for i := 2; fsutil.Exists(backup); i++ {
				backup = fmt.Sprintf("%s-%d", base, i)
			}
		}
		rel, _ := filepath.Rel(gameDir, src)
		dst := filepath.Join(backup, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return moved, backup, err
		}
		if err := os.Rename(src, dst); err != nil {
			return moved, backup, err
		}
		moved += n
	}
	return moved, backup, nil
}

// KeptSize is the size of the storage upgrade's kept files (workspace.Dir.Kept).
func KeptSize(dir string) int64 {
	n, _ := dirSize(dir)
	return n
}

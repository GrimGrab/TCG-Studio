// Package library runs the Storage page: how much disk the workspace, each setup and each set take, and the player's
// actions on the shared card-art library (<workspace>\library\<setId>, see project.Workspace.Library):
//   - Move: card art kept in each setup's project folder moves into the library; byte-identical copies in other setups
//     are deleted (the lookup finds the library copy), different ones stay as that setup's own art;
//   - Shrink: PNG card art of a set becomes JPEG (about 6x smaller), set.json of every setup with the set updated;
//   - unused library sets (workspace and the mod's <plugin>\Library) can be listed and deleted.
//
// Plain files only: no links. Every step leaves each image either in the project folder or in the library, so the lookup
// (project folder first, then library) always finds it, and an interrupted run just continues next time.
package library

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/catalog"
	"tcgstudio/internal/game"
	"tcgstudio/internal/importer"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setups"
)

// Progress of a report or an action.
type Progress struct {
	Message string `json:"message"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
}

// SetInfo is one set of one setup.
type SetInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Own       int64  `json:"own"`       // files the set keeps in the setup's project folder (game info included)
	Library   int64  `json:"library"`   // card art it uses from the shared library
	Shared    int64  `json:"shared"`    // every file it uses from the shared library
	Differs   int    `json:"differs"`   // files that differ from the shared ones (waiting for the player's choice)
	PNG       int64  `json:"png"`       // PNG card art it uses (own + library), what Shrink would convert
	PNGFiles  int    `json:"pngFiles"`  //
	ShrinkTo  int64  `json:"shrinkTo"`  // estimated size of that art as JPEG
	Movable   int    `json:"movable"`   // files Move everything would take or drop (incl. leftovers)
	Shareable bool   `json:"shareable"` // kept for older pages
	ArtFolder string `json:"artFolder"` // its art is kept as its own in this named shared folder ("" = the set's shared art)
}

// ItemInfo is an accessory or furniture piece of a setup.
type ItemInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"`   // accessory kind (Playmat, …) or "Furniture"
	Own    int64  `json:"own"`    // its files still in the setup
	Shared int64  `json:"shared"` // its files in the shared store
}

// SetupInfo is one setup: its folder size broken down, and every set and item it uses.
type SetupInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
	Size   int64  `json:"size"` // the setup's own folder (= the parts below)
	// Breakdown of Size.
	Info      int64      `json:"info"`      // game info: set.json, studio.json, accessories.json, setup.json
	CardArt   int64      `json:"cardArt"`   // card images kept in the setup
	SetFiles  int64      `json:"setFiles"`  // other set files kept in the setup (pack & box art, card back, photos, …)
	Leftovers int64      `json:"leftovers"` // images in set folders nothing refers to
	ItemFiles int64      `json:"itemFiles"` // older accessory & furniture files kept in the setup
	Saves     int64      `json:"saves"`     // parked game saves (inactive setups)
	Game      int64      `json:"game"`      // the setup's game state (mod settings, global card back, hand-installed content)
	Other     int64      `json:"other"`
	Sets      []SetInfo  `json:"sets"`
	Items     []ItemInfo `json:"items"`
}

// Report is the Storage page.
type Report struct {
	Workspace     int64         `json:"workspace"`   // whole workspace folder
	Library       int64         `json:"library"`     // <workspace>\library
	Assets        int64         `json:"assets"`      // <workspace>\assets
	Catalog       int64         `json:"catalog"`     // <workspace>\catalog
	GameSets      int64         `json:"gameSets"`    // <plugin>\Sets
	GameLibrary   int64         `json:"gameLibrary"` // <plugin>\Library
	GameFound     bool          `json:"gameFound"`
	ModHasLibrary bool          `json:"modHasLibrary"`
	Setups        []SetupInfo   `json:"setups"`
	MoveSaves     int64         `json:"moveSaves"` // bytes Move everything frees (copies already shared, leftovers)
	MoveFiles     int           `json:"moveFiles"` // files it moves, drops or deletes (set and accessory files)
	MoveBytes     int64         `json:"moveBytes"` // bytes it moves into shared folders (stay on disk, just elsewhere)
	MoveList      []MoveFile    `json:"moveList"`  // every file of MoveFiles, for the page to list
	Differ        []Differ      `json:"differ"`    // sets whose files differ from the shared ones: the player's choice
	Unused        []UnusedEntry `json:"unused"`    // everything no setup uses, whatever the kind
	// Accessories: the shared asset store and each setup's own older accessory files.
	Accessories AccessoryStorage `json:"accessories"`
}

// MoveFile is one file Move everything touches.
type MoveFile struct {
	Setup  string `json:"setup"`  // setup name ("Catalog" for the catalog's templates)
	Owner  string `json:"owner"`  // set name, or "Accessories & furniture"
	File   string `json:"file"`   // path in the setup's set (or accessories) folder
	Bytes  int64  `json:"bytes"`  //
	Action string `json:"action"` // MoveTo (moved to shared), MoveDrop (same file already shared: deleted), MoveLeftover (unused: deleted)
}

// MoveFile actions.
const (
	MoveTo       = "move"
	MoveDrop     = "duplicate"
	MoveLeftover = "leftover"
)

// loc is one setup's copy of a set.
type loc struct {
	setup string
	p     *project.Project
}

// catalogSetup is the loc.setup of the shared catalog's set templates (not a setup id: those never contain ':'). They
// count as users of library art and take part in Move/Shrink like a setup's copy, but aren't listed as a setup. A
// template's file that differs from a setup's is a stale copy (templated before the setup changed it, e.g. by Smart
// generate after an import): Move drops it and the template uses the shared file.
const catalogSetup = ":catalog"

// sets loads every project of every setup and of the catalog, grouped by set id. The catalog comes last, so a setup's
// file becomes the shared one rather than the template's older copy.
func sets(h setups.Home) (map[string][]loc, []setups.Summary, error) {
	list, err := h.List()
	if err != nil {
		return nil, nil, err
	}
	out := map[string][]loc{}
	dirs := append(append([]setups.Summary{}, list...), setups.Summary{ID: catalogSetup, Folder: filepath.Join(h.Root, catalog.DirName)})
	for _, s := range dirs {
		ws := project.Workspace{Root: s.Folder, Library: project.LibraryDir(h.Root)}
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

// libMetaFor describes how p's art was imported; ok=false when it can't be told (no images, unknown source).
func libMetaFor(p *project.Project) (project.LibMeta, bool) {
	m := project.LibMeta{Source: p.Meta.Source, Code: p.Meta.SourceCode(), Lang: p.Meta.Lang, Format: "png"}
	if m.Source == "" || m.Source == "manual" {
		return m, false
	}
	imgs := cardImages(p)
	if len(imgs) == 0 {
		return m, false
	}
	if strings.EqualFold(path.Ext(imgs[0]), ".jpg") {
		m.Format = "jpg"
	}
	if f, err := os.Open(p.ImagePath(imgs[0])); err == nil {
		cfg, _, err := image.DecodeConfig(f)
		f.Close()
		if err == nil {
			// Back to the Import page's size setting (384 / 512 / Original): art is shrunk to the setting, never enlarged,
			// so anything up to 512 px other than exactly 384 came from 512 (the default), and wider art is Original.
			switch {
			case cfg.Width == 384:
				m.Width = 384
			case cfg.Width <= 512:
				m.Width = 512
			default:
				m.Width = 0
			}
		}
	}
	return m, true
}

// shareable reports whether p's art may live in the library: no library folder for the set yet, or one made the same way.
func shareable(p *project.Project) (project.LibMeta, bool) {
	m, ok := libMetaFor(p)
	if !ok || p.LibFolder == "" {
		return m, false
	}
	have := project.LoadLibMeta(p.LibFolder)
	if have == nil {
		return m, true
	}
	return m, have.Source == m.Source && have.Code == m.Code && have.Lang == m.Lang && have.Format == m.Format &&
		have.Width == m.Width
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
	r := &Report{Setups: []SetupInfo{}, Unused: []UnusedEntry{}, Differ: []Differ{}, MoveList: []MoveFile{}}
	r.Workspace, _ = dirSize(h.Root)
	r.Library, _ = dirSize(project.LibraryDir(h.Root))
	r.Assets, _ = dirSize(filepath.Join(h.Root, assets.DirName))
	r.Catalog, _ = dirSize(filepath.Join(h.Root, catalog.DirName))
	if game.IsGameDir(gameDir) {
		r.GameFound = true
		r.ModHasLibrary = game.ModHasLibrary(gameDir)
		r.GameSets, _ = dirSize(game.SetsDir(gameDir))
		r.GameLibrary, _ = dirSize(project.GameLibraryDir(gameDir))
	}

	names := map[string]string{catalogSetup: "Catalog"}
	for _, sm := range list {
		names[sm.ID] = sm.Name
	}
	ids := sortedIDs(bySet)
	store := assets.For(h.Root)

	// What Move everything does, per setup and set: files it takes or drops, leftovers it deletes.
	movable := map[string]map[string]int{} // setup → set → files
	for i, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		progress(Progress{Message: "Comparing workspace files…", Done: i + 1, Total: len(ids)})
		claimed := map[string]string{} // rel → the setup file that will become the shared one (MoveAll's order)
		for _, l := range bySet[id] {
			if l.p.LibFolder == "" {
				continue
			}
			n := 0
			add := func(file string, bytes int64, action string) {
				n++
				r.MoveList = append(r.MoveList, MoveFile{Setup: names[l.setup], Owner: l.p.Set.Name, File: file, Bytes: bytes, Action: action})
				if action == MoveTo {
					r.MoveBytes += bytes
				} else {
					r.MoveSaves += bytes
				}
			}
			for _, rel := range ownFilesOf(l.p) {
				own := filepath.Join(l.p.Folder, filepath.FromSlash(rel))
				switch cp := counterpart(l.p, rel); {
				case cp == "" && claimed[rel] == "":
					claimed[rel] = own
					add(rel, size(own), MoveTo)
				case cp == "" && sameFile(own, claimed[rel]):
					add(rel, size(own), MoveDrop)
				case cp == rel && sameFile(own, filepath.Join(l.p.LibFolder, filepath.FromSlash(rel))):
					add(rel, size(own), MoveDrop)
				case l.setup == catalogSetup && (cp == rel || cp == "" && claimed[rel] != ""):
					// A template's stale copy: the shared file (or the setup's, moved first) wins.
					add(rel, size(own), MoveDrop)
				}
			}
			for _, f := range setLeftovers(l.p) {
				rel, _ := filepath.Rel(l.p.Folder, f)
				add(filepath.ToSlash(rel), size(f), MoveLeftover)
			}
			r.MoveFiles += n
			if movable[l.setup] == nil {
				movable[l.setup] = map[string]int{}
			}
			movable[l.setup][id] = n
		}
	}
	r.Differ = differs(bySet, names)
	differing := map[string]int{} // setup/set → files
	for _, d := range r.Differ {
		differing[d.Setup+"/"+d.Set] = d.Files
	}

	// Per setup: folder breakdown, sets (PNG art and its JPEG estimate for Shrink) and items.
	ratio := map[string]float64{} // set id → jpeg/png size ratio
	for _, sm := range list {
		si := SetupInfo{ID: sm.ID, Name: sm.Name, Active: sm.Active, Sets: []SetInfo{}, Items: []ItemInfo{}}
		cards := map[string]bool{} // project file (abs) → card image
		used := map[string]bool{}  // project file (abs) → referenced
		for _, id := range ids {
			for _, l := range bySet[id] {
				if l.setup != sm.ID {
					continue
				}
				set := SetInfo{ID: id, Name: l.p.Set.Name, Movable: movable[sm.ID][id], Differs: differing[sm.ID+"/"+id]}
				_, set.Shareable = shareable(l.p)
				set.Own, _ = dirSize(l.p.Folder)
				set.ArtFolder = artFolder(l.p)
				for _, rel := range l.p.Files() {
					f := filepath.Clean(filepath.Join(l.p.Folder, filepath.FromSlash(rel)))
					used[f] = true
					if l.p.InLibrary(rel) {
						set.Shared += size(l.p.ImagePath(rel))
					}
				}
				for _, rel := range cardImages(l.p) {
					cards[filepath.Clean(filepath.Join(l.p.Folder, filepath.FromSlash(rel)))] = true
					f := l.p.ImagePath(rel)
					n := size(f)
					if l.p.InLibrary(rel) {
						set.Library += n
					}
					if strings.EqualFold(path.Ext(rel), ".png") && defaultName(rel) {
						set.PNG += n
						set.PNGFiles++
					}
				}
				if set.PNGFiles > 0 {
					rt, ok := ratio[id]
					if !ok {
						rt = jpegRatio(l.p)
						ratio[id] = rt
					}
					set.ShrinkTo = int64(float64(set.PNG) * rt)
				}
				si.Sets = append(si.Sets, set)
			}
		}
		sort.Slice(si.Sets, func(i, j int) bool { return si.Sets[i].Own+si.Sets[i].Shared > si.Sets[j].Own+si.Sets[j].Shared })
		breakdown(&si, sm.Folder, cards, used)
		if l, err := accessories.Open(sm.Folder, store); err == nil {
			si.Items = itemsOf(l)
		}
		r.Setups = append(r.Setups, si)
	}

	r.Accessories = analyzeAccessories(h, list)
	for _, f := range r.Accessories.list { // Move everything takes these too
		r.MoveList = append(r.MoveList, f)
		r.MoveFiles++
		if f.Action == MoveTo {
			r.MoveBytes += f.Bytes
		} else {
			r.MoveSaves += f.Bytes
		}
	}
	r.Unused = unusedEntries(h, gameDirIf(r.GameFound, gameDir), bySet, list)
	return r, nil
}

func gameDirIf(ok bool, dir string) string {
	if ok {
		return dir
	}
	return ""
}

// artFolder is the named shared folder a set's art is kept in as its own, or "" (see project.Project.ArtFolder).
func artFolder(p *project.Project) string { return p.ArtFolder() }

// breakdown splits a setup folder's size into its parts (the parts add up to Size).
func breakdown(si *SetupInfo, dir string, cards, used map[string]bool) {
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
		clean := filepath.Clean(f)
		switch {
		case top == setups.SavesDir || strings.HasPrefix(top, setups.SavesDir+"-"):
			si.Saves += n
		case top == setups.GameStateDir:
			si.Game += n
		case strings.EqualFold(filepath.Ext(f), ".json"):
			si.Info += n
		case top == "projects" && cards[clean]:
			si.CardArt += n
		case top == "projects" && used[clean]:
			si.SetFiles += n
		case top == "projects" && isImageName(f):
			si.Leftovers += n
		case top == "accessories":
			si.ItemFiles += n
		default:
			si.Other += n
		}
		return nil
	})
}

// itemsOf lists a setup's accessories and furniture with their own and shared file sizes.
func itemsOf(l *accessories.Library) []ItemInfo {
	out := []ItemInfo{}
	sizes := func(rels ...string) (own, shared int64) {
		for _, rel := range rels {
			if rel == "" {
				continue
			}
			n := size(l.Resolve(rel))
			if assets.IsAsset(rel) {
				shared += n
			} else {
				own += n
			}
		}
		return
	}
	for _, a := range l.Lib.Accessories {
		it := ItemInfo{ID: a.ID, Name: a.Name, Kind: a.Kind}
		it.Own, it.Shared = sizes(a.Texture, a.Icon, a.Mesh)
		out = append(out, it)
	}
	for _, f := range l.Lib.Furniture {
		it := ItemInfo{ID: f.ID, Name: f.Name, Kind: "Furniture"}
		it.Own, it.Shared = sizes(f.Files()...)
		out = append(out, it)
	}
	for _, d := range l.Lib.Decorations {
		it := ItemInfo{ID: d.ID, Name: d.Name, Kind: "Decoration"}
		it.Own, it.Shared = sizes(d.Files()...)
		out = append(out, it)
	}
	return out
}

func unionImages(locs []loc) []string {
	seen := map[string]bool{}
	var out []string
	for _, l := range locs {
		for _, rel := range cardImages(l.p) {
			if !seen[rel] {
				seen[rel] = true
				out = append(out, rel)
			}
		}
	}
	sort.Strings(out)
	return out
}

// defaultName reports whether rel is an importer file name (images/<cid>.png|jpg), not art added or changed in Studio
// under another name. Shrink only converts those.
func defaultName(rel string) bool {
	return strings.HasPrefix(rel, "images/") && !strings.Contains(strings.TrimPrefix(rel, "images/"), "/")
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// sameFile reports whether two files have the same bytes.
func sameFile(a, b string) bool {
	ia, err := os.Stat(a)
	if err != nil {
		return false
	}
	ib, err := os.Stat(b)
	if err != nil || ia.Size() != ib.Size() {
		return false
	}
	fa, err := os.Open(a)
	if err != nil {
		return false
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return false
	}
	defer fb.Close()
	ba, bb := make([]byte, 64<<10), make([]byte, 64<<10)
	for {
		na, ea := io.ReadFull(fa, ba)
		nb, eb := io.ReadFull(fb, bb)
		if na != nb || !bytes.Equal(ba[:na], bb[:nb]) {
			return false
		}
		if ea != nil || eb != nil {
			return ea != nil && eb != nil
		}
	}
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
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, importer.Opaque(img), &jpeg.Options{Quality: importer.JPEGQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

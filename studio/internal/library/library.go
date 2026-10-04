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
	Own       int64  `json:"own"`       // files in the setup's project folder
	Library   int64  `json:"library"`   // card art it uses from the shared library
	PNG       int64  `json:"png"`       // PNG card art it uses (own + library), what Shrink would convert
	PNGFiles  int    `json:"pngFiles"`  //
	ShrinkTo  int64  `json:"shrinkTo"`  // estimated size of that art as JPEG
	Movable   int    `json:"movable"`   // card images still in the project folder that Move would take
	Shareable bool   `json:"shareable"` // its art can go into the library (no library version of it made differently)
}

// SetupInfo is one setup.
type SetupInfo struct {
	ID     string    `json:"id"`
	Name   string    `json:"name"`
	Active bool      `json:"active"`
	Size   int64     `json:"size"` // the setup's own folder
	Sets   []SetInfo `json:"sets"`
}

// Unused is a library set no setup uses any more.
type Unused struct {
	ID    string `json:"id"`
	Size  int64  `json:"size"`
	Files int    `json:"files"`
}

// Report is the Storage page.
type Report struct {
	Workspace     int64       `json:"workspace"`   // whole workspace folder
	Library       int64       `json:"library"`     // <workspace>\library
	GameSets      int64       `json:"gameSets"`    // <plugin>\Sets
	GameLibrary   int64       `json:"gameLibrary"` // <plugin>\Library
	GameFound     bool        `json:"gameFound"`
	ModHasLibrary bool        `json:"modHasLibrary"`
	Setups        []SetupInfo `json:"setups"`
	MoveSaves     int64       `json:"moveSaves"` // bytes Move frees (copies of the same art in several setups)
	MoveFiles     int         `json:"moveFiles"` // card images Move takes into the library
	Unused        []Unused    `json:"unused"`
	GameUnused    []Unused    `json:"gameUnused"`
}

// loc is one setup's copy of a set.
type loc struct {
	setup string
	p     *project.Project
}

// sets loads every project of every setup, grouped by set id.
func sets(h setups.Home) (map[string][]loc, []setups.Summary, error) {
	list, err := h.List()
	if err != nil {
		return nil, nil, err
	}
	out := map[string][]loc{}
	for _, s := range list {
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
		if c.Image != "" && !seen[c.Image] {
			seen[c.Image] = true
			out = append(out, c.Image)
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
	r := &Report{Setups: []SetupInfo{}, Unused: []Unused{}, GameUnused: []Unused{}}
	r.Workspace, _ = dirSize(h.Root)
	r.Library, _ = dirSize(project.LibraryDir(h.Root))
	if game.IsGameDir(gameDir) {
		r.GameFound = true
		r.ModHasLibrary = game.ModHasLibrary(gameDir)
		r.GameSets, _ = dirSize(game.SetsDir(gameDir))
		r.GameLibrary, _ = dirSize(project.GameLibraryDir(gameDir))
	}

	// Move estimate: per set id and image, the copies in project folders that are byte-identical to the one kept.
	ids := make([]string, 0, len(bySet))
	for id := range bySet {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	movable := map[string]map[string]int{} // setup → set → movable files
	for i, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		progress(Progress{Message: "Comparing card art…", Done: i + 1, Total: len(ids)})
		locs := bySet[id]
		var share []loc
		for _, l := range locs {
			if _, ok := shareable(l.p); ok {
				share = append(share, l)
			}
		}
		for _, rel := range unionImages(share) {
			var kept string // the file the library will have
			if lib := filepath.Join(share[0].p.LibFolder, filepath.FromSlash(rel)); fileExists(lib) {
				kept = lib
			}
			for _, l := range share {
				own := filepath.Join(l.p.Folder, filepath.FromSlash(rel))
				if !fileExists(own) {
					continue
				}
				if movable[l.setup] == nil {
					movable[l.setup] = map[string]int{}
				}
				switch {
				case kept == "":
					kept = own // moves into the library
					movable[l.setup][id]++
					r.MoveFiles++
				case sameFile(own, kept):
					movable[l.setup][id]++
					r.MoveFiles++
					r.MoveSaves += size(own)
				}
			}
		}
	}

	// Per setup and set sizes, PNG art and its JPEG estimate (a few cards encoded per set).
	ratio := map[string]float64{} // set id → jpeg/png size ratio
	for _, s := range list {
		si := SetupInfo{ID: s.ID, Name: s.Name, Active: s.Active, Sets: []SetInfo{}}
		si.Size, _ = dirSize(s.Folder)
		for _, id := range ids {
			for _, l := range bySet[id] {
				if l.setup != s.ID {
					continue
				}
				set := SetInfo{ID: id, Name: l.p.Set.Name, Movable: movable[s.ID][id]}
				_, set.Shareable = shareable(l.p)
				set.Own, _ = dirSize(l.p.Folder)
				for _, rel := range cardImages(l.p) {
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
		sort.Slice(si.Sets, func(i, j int) bool { return si.Sets[i].Own+si.Sets[i].Library > si.Sets[j].Own+si.Sets[j].Library })
		r.Setups = append(r.Setups, si)
	}

	// Unused library sets: library folders whose set id no setup has.
	r.Unused = unused(project.LibraryDir(h.Root), bySet)
	if r.GameFound {
		r.GameUnused = unused(project.GameLibraryDir(gameDir), bySet)
	}
	return r, nil
}

func unused(root string, bySet map[string][]loc) []Unused {
	out := []Unused{}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if !e.IsDir() || len(bySet[e.Name()]) > 0 {
			continue
		}
		n, files := dirSize(filepath.Join(root, e.Name()))
		out = append(out, Unused{ID: e.Name(), Size: n, Files: files})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
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

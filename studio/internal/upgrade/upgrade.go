// Package upgrade brings a workspace from the older storage layouts to shared-only storage, once, when the player clicks
// "Upgrade storage" on the Storage page:
//   - set files kept in a setup's project folder move to the set's shared folder <workspace>\library\<setId>; identical
//     copies are dropped, files nothing refers to are deleted, and files that differ from the shared ones wait for the
//     player's choice per set (use the shared art, or make this setup's art the shared art);
//   - named "own art" folders library\<setId>\<Name>\ are folded into the set's shared art the same way;
//   - accessory, furniture and decoration files kept in a setup's accessories\images move to the shared store.
//
// Nothing else in Studio reads the old layouts. This package is the only code that knows them and can be deleted once
// workspaces have been upgraded.
package upgrade

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"tcgstudio/internal/catalog"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

// Choices for a differing copy or a named folder.
const (
	UseShared = "shared"  // the same art: this copy goes, the set uses the shared files
	Replace   = "replace" // this copy becomes the shared art (every setup using the shared files shows it)
)

// Progress of a check or a run.
type Progress struct {
	Message string `json:"message"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
}

// File is one file the upgrade moves or deletes.
type File struct {
	Setup  string `json:"setup"`  // setup name ("Catalog" for the catalog's templates)
	Owner  string `json:"owner"`  // set name, or "Accessories & furniture"
	File   string `json:"file"`   // path in the set's (or the setup's accessories) folder
	Bytes  int64  `json:"bytes"`  //
	Action string `json:"action"` // ActionMove, ActionDrop, ActionLeftover
}

// File actions.
const (
	ActionMove     = "move"      // moved to the shared folder or store
	ActionDrop     = "duplicate" // the same file is shared already: deleted
	ActionLeftover = "leftover"  // nothing refers to it: deleted
)

// Differ is a setup's set whose files differ from the shared files at the same paths: the player chooses.
type Differ struct {
	Key       string `json:"key"`       // Choices.Differ key
	Setup     string `json:"setup"`     // setup id
	SetupName string `json:"setupName"` //
	Set       string `json:"set"`       // set id
	SetName   string `json:"setName"`   //
	Files     int    `json:"files"`
	Bytes     int64  `json:"bytes"`
	Sample    string `json:"sample"` // a differing file (relative), for the side-by-side preview
}

// Named is a named art folder library\<setId>\<Name>\ (from the older "keep as its own" choice): the player chooses.
type Named struct {
	Key     string   `json:"key"` // Choices.Named key
	Set     string   `json:"set"` //
	SetName string   `json:"setName"`
	Name    string   `json:"name"`
	UsedBy  []string `json:"usedBy"` // setup names
	Files   int      `json:"files"`
	Bytes   int64    `json:"bytes"`
	Sample  string   `json:"sample"` // relative to the named folder
}

// Report is what an upgrade would do. Empty = the workspace is on shared-only storage.
type Report struct {
	Files  int      `json:"files"` // setup files moved or deleted (sets and accessories)
	Bytes  int64    `json:"bytes"`
	List   []File   `json:"list"`
	Differ []Differ `json:"differ"`
	Named  []Named  `json:"named"`
	// Game: the active setup's content that is still only in the game folder (installed sets, card back, Shop look).
	Game GameState `json:"game"`
}

// Empty reports whether nothing is left to upgrade.
func (r *Report) Empty() bool {
	return r.Files == 0 && len(r.Differ) == 0 && len(r.Named) == 0 && r.Game.empty()
}

// Choices are the player's answers, by Differ.Key and Named.Key.
type Choices struct {
	Differ map[string]string `json:"differ"`
	Named  map[string]string `json:"named"`
	// Partial (Studio's automatic run at start): copies and folders without an answer are left as they are, for the
	// player to decide later; everything else is done.
	Partial bool `json:"-"`
}

// Result of Run.
type Result struct {
	Files int      `json:"files"` // files moved or deleted
	Freed int64    `json:"freed"` // bytes freed in the workspace
	Notes []string `json:"notes"` // what was taken over from the game folder, for the player
	Left  int      `json:"left"`  // differing copies and folders still waiting for the player's choice (Partial runs)
	// Kept: files that weren't exact copies, moved into KeptDir instead of being deleted or overwritten.
	Kept      int    `json:"kept"`
	KeptBytes int64  `json:"keptBytes"`
	KeptDir   string `json:"keptDir"`

	root, keptRun string // workspace; this run's kept folder (made on first use)
}

// loc is one setup's (or the catalog's) copy of a set.
type loc struct {
	setup string
	p     *project.Project
}

// catalogSetup is the loc.setup of the catalog's set templates. A template's file that differs from a setup's is a stale
// copy (templated before the setup changed it): the setup's file wins, the template's is dropped.
const catalogSetup = ":catalog"

// sets loads every project of every setup and of the catalog (last), grouped by set id.
func sets(h setups.Home) (map[string][]loc, map[string]string, error) {
	list, err := h.List()
	if err != nil {
		return nil, nil, err
	}
	names := map[string]string{catalogSetup: "Catalog"}
	out := map[string][]loc{}
	dirs := append(append([]setups.Summary{}, list...), setups.Summary{ID: catalogSetup, Folder: catalog.For(h.Root).Root})
	for _, s := range dirs {
		if s.ID != catalogSetup {
			names[s.ID] = s.Name
		}
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
	return out, names, nil
}

func sortedIDs(m map[string][]loc) []string {
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func size(p string) int64 {
	if info, err := os.Stat(p); err == nil {
		return info.Size()
	}
	return 0
}

func own(p *project.Project, rel string) string {
	return filepath.Join(p.Folder, filepath.FromSlash(rel))
}
func lib(p *project.Project, rel string) string {
	return filepath.Join(p.LibFolder, filepath.FromSlash(rel))
}

// files is every file a set uses (project.Files), plus the implicit ones (set icon) its project folder still holds.
func files(p *project.Project) []string {
	out := p.Files()
	seen := map[string]bool{}
	for _, rel := range out {
		seen[rel] = true
	}
	for _, rel := range project.ImplicitFiles {
		if !seen[rel] && fsutil.Exists(own(p, rel)) {
			out = append(out, rel)
		}
	}
	return out
}

// ownFiles lists the files a set uses that its project folder still holds.
func ownFiles(p *project.Project) []string {
	var out []string
	for _, rel := range files(p) {
		if fsutil.Exists(own(p, rel)) {
			out = append(out, rel)
		}
	}
	return out
}

// leftovers lists image files in a set's project folder that nothing refers to (e.g. PNGs left by Shrink, older Smart
// art). set.json, studio.json and other files stay.
func leftovers(p *project.Project) []string {
	used := map[string]bool{}
	for _, rel := range files(p) {
		used[filepath.Clean(own(p, rel))] = true
	}
	var out []string
	_ = filepath.WalkDir(p.Folder, func(f string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || used[filepath.Clean(f)] {
			return nil
		}
		if rel, _ := filepath.Rel(p.Folder, f); project.IsRelFile(filepath.ToSlash(rel)) || fsutil.IsImage(f) {
			out = append(out, f)
		}
		return nil
	})
	return out
}

// counterpart is the shared file a setup's own file rel stands for: the library file at the same path, or a card image
// with the same name in the other format (x.png ↔ x.jpg, left by Shrink in one place). "" when the library has none.
func counterpart(p *project.Project, rel string) string {
	if fsutil.Exists(lib(p, rel)) {
		return rel
	}
	ext := strings.ToLower(path.Ext(rel))
	if ext != ".png" && ext != ".jpg" {
		return ""
	}
	other := strings.TrimSuffix(rel, path.Ext(rel)) + map[string]string{".png": ".jpg", ".jpg": ".png"}[ext]
	if fsutil.Exists(lib(p, other)) {
		return other
	}
	return ""
}

// differing lists the own files of l that differ from the shared ones (as things are on disk now).
func differing(l loc) []string {
	var rels []string
	for _, rel := range ownFiles(l.p) {
		cp := counterpart(l.p, rel)
		if cp == "" || cp == rel && fsutil.SameContent(own(l.p, rel), lib(l.p, rel)) {
			continue // Move takes it, or drops an identical copy
		}
		rels = append(rels, rel)
	}
	return rels
}

func differKey(setup, set string) string { return setup + "/" + set }
func namedKey(set, name string) string   { return set + "/" + name }

// Check works out what Run would do. Read-only. gameDir "" skips the game folder.
func Check(ctx context.Context, h setups.Home, gameDir string, progress func(Progress)) (*Report, error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	bySet, names, err := sets(h)
	if err != nil {
		return nil, err
	}
	r := &Report{List: []File{}, Differ: []Differ{}, Named: []Named{}}
	add := func(setup, owner, file string, n int64, action string) {
		r.List = append(r.List, File{Setup: names[setup], Owner: owner, File: file, Bytes: n, Action: action})
		r.Files++
		r.Bytes += n
	}
	ids := sortedIDs(bySet)
	for i, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		progress(Progress{Message: "Checking set files…", Done: i + 1, Total: len(ids)})
		claimed := map[string]string{} // rel → the setup file Run moves first (sets order)
		for _, l := range bySet[id] {
			d := Differ{Key: differKey(l.setup, id), Setup: l.setup, SetupName: names[l.setup], Set: id, SetName: l.p.Set.Name}
			for _, rel := range ownFiles(l.p) {
				f := own(l.p, rel)
				n := size(f)
				cp := counterpart(l.p, rel)
				switch {
				case cp == "" && claimed[rel] == "":
					claimed[rel] = f
					add(l.setup, l.p.Set.Name, rel, n, ActionMove)
				case l.setup == catalogSetup,
					cp == "" && fsutil.SameContent(f, claimed[rel]),
					cp == rel && fsutil.SameContent(f, lib(l.p, rel)):
					add(l.setup, l.p.Set.Name, rel, n, ActionDrop)
				default:
					d.Files++
					d.Bytes += n
					if d.Sample == "" {
						d.Sample = rel
					}
				}
			}
			for _, f := range leftovers(l.p) {
				rel, _ := filepath.Rel(l.p.Folder, f)
				add(l.setup, l.p.Set.Name, filepath.ToSlash(rel), size(f), ActionLeftover)
			}
			if d.Files > 0 {
				r.Differ = append(r.Differ, d)
			}
		}
		r.Named = append(r.Named, namedFolders(h, id, bySet[id], names)...)
	}
	sort.SliceStable(r.Differ, func(i, j int) bool { return r.Differ[i].Bytes > r.Differ[j].Bytes })
	progress(Progress{Message: "Checking accessory files…"})
	acc, err := checkAccessories(h)
	if err != nil {
		return nil, err
	}
	for _, f := range acc {
		r.List = append(r.List, f)
		r.Files++
		r.Bytes += f.Bytes
	}
	r.Game = gameState(h, gameDir)
	return r, nil
}

// namedFolders lists the named art folders of set id with the setups (and catalog) using them.
func namedFolders(h setups.Home, id string, locs []loc, names map[string]string) []Named {
	dir := filepath.Join(workspace.Dir(h.Root).Library(), id)
	entries, _ := os.ReadDir(dir)
	var out []Named
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "images" {
			continue
		}
		n := Named{Key: namedKey(id, e.Name()), Set: id, Name: e.Name(), UsedBy: []string{}}
		prefix := e.Name() + "/"
		for _, l := range locs {
			n.SetName = l.p.Set.Name
			for _, rel := range l.p.Files() {
				if strings.HasPrefix(rel, prefix) {
					n.UsedBy = append(n.UsedBy, names[l.setup])
					break
				}
			}
		}
		_ = filepath.WalkDir(filepath.Join(dir, e.Name()), func(f string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				n.Files++
				n.Bytes += size(f)
				if n.Sample == "" {
					rel, _ := filepath.Rel(filepath.Join(dir, e.Name()), f)
					n.Sample = filepath.ToSlash(rel)
				}
			}
			return nil
		})
		// Only a folder some copies of the set use and others don't needs a choice; one that every copy uses is just
		// where the set keeps those files, and one nobody uses shows up under the Storage page's unused files.
		if len(n.UsedBy) == 0 || len(n.UsedBy) == len(locs) {
			continue
		}
		out = append(out, n)
	}
	return out
}

// Run upgrades the workspace. Every differing copy and named folder Check lists needs a choice; Run stops before
// changing anything when one is missing. Each file is moved or dropped on its own and every reference points at an
// existing file at all times, so an interrupted run just continues next time.
func Run(ctx context.Context, h setups.Home, gameDir string, choices Choices, progress func(Progress)) (*Result, error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	before, err := Check(ctx, h, "", nil)
	if err != nil {
		return nil, err
	}
	var missing []string
	for _, d := range before.Differ {
		if c := choices.Differ[d.Key]; c != UseShared && c != Replace {
			missing = append(missing, d.SetName+" ("+d.SetupName+")")
		}
	}
	for _, n := range before.Named {
		if c := choices.Named[n.Key]; c != UseShared && c != Replace {
			missing = append(missing, n.SetName+" — "+n.Name)
		}
	}
	if len(missing) > 0 && !choices.Partial {
		return nil, fmt.Errorf("choose shared or replace for: %s", strings.Join(missing, ", "))
	}
	res := &Result{Notes: []string{}, Left: len(missing), root: h.Root}
	progress(Progress{Message: "Taking content over from the game folder…"})
	if err := AdoptGame(h, gameDir, res); err != nil {
		return res, err
	}
	if err := moveSets(ctx, h, res, progress); err != nil {
		return res, err
	}
	bySet, _, err := sets(h)
	if err != nil {
		return res, err
	}
	for _, id := range sortedIDs(bySet) {
		for _, l := range bySet[id] {
			if l.setup == catalogSetup {
				continue
			}
			c, ok := choices.Differ[differKey(l.setup, id)]
			if !ok {
				continue
			}
			if err := resolveDiffering(l, bySet[id], c, res); err != nil {
				return res, err
			}
		}
	}
	for _, n := range before.Named {
		if c := choices.Named[n.Key]; c != UseShared && c != Replace {
			continue // Partial: the player decides later
		}
		progress(Progress{Message: "Folding " + n.SetName + " — " + n.Name + " into the shared art…"})
		if err := foldNamed(h, n, choices.Named[n.Key], res); err != nil {
			return res, err
		}
	}
	progress(Progress{Message: "Moving accessory files…"})
	if err := moveAccessories(ctx, h, res); err != nil {
		return res, err
	}
	progress(Progress{Message: "Done"})
	return res, nil
}

// moveSets moves every own set file without a differing shared counterpart into the library and drops identical copies and
// leftovers (the catalog's stale copies too).
func moveSets(ctx context.Context, h setups.Home, res *Result, progress func(Progress)) error {
	bySet, _, err := sets(h)
	if err != nil {
		return err
	}
	ids := sortedIDs(bySet)
	for i, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		progress(Progress{Message: fmt.Sprintf("Moving files of %s…", id), Done: i, Total: len(ids)})
		for _, l := range bySet[id] {
			// The set's import settings, for Import's "already in the library" check, when the library has none yet.
			if m, ok := libMetaFor(l.p); ok && project.LoadLibMeta(l.p.LibFolder) == nil {
				if err := project.SaveLibMeta(l.p.LibFolder, &m); err != nil {
					return err
				}
			}
			var drop []string
			repoint := map[string]string{} // catalog template → the shared file in the other format
			for _, rel := range ownFiles(l.p) {
				f := own(l.p, rel)
				switch cp := counterpart(l.p, rel); {
				case cp == "":
					if err := fsutil.MoveFile(f, lib(l.p, rel)); err != nil {
						return err
					}
					res.Files++
				case cp == rel && fsutil.SameContent(f, lib(l.p, rel)), l.setup == catalogSetup:
					// The same file is shared already, or a catalog template's stale copy (see catalogSetup).
					drop = append(drop, rel)
					if cp != rel {
						repoint[rel] = cp
					}
				}
				// Anything else differs from the shared file: the player's choice (resolveDiffering).
			}
			if len(repoint) > 0 { // save before deleting: every reference points at an existing file at all times
				l.p.RepointFiles(func(rel string) string {
					if to, ok := repoint[rel]; ok {
						return to
					}
					return rel
				})
				if err := (project.Workspace{}).Save(l.p); err != nil {
					return err
				}
			}
			for _, rel := range drop { // identical → deleted; a catalog template's older version → kept
				if err := res.discard(own(l.p, rel), lib(l.p, rel)); err != nil {
					return err
				}
			}
			for _, f := range leftovers(l.p) { // nothing refers to them, but they may be someone's work: kept
				if err := res.keep(f, false); err != nil {
					return err
				}
			}
			removeEmptyDirs(l.p.Folder, false)
		}
	}
	return nil
}

// resolveDiffering applies a choice to one setup's set whose files differ from the shared ones. UseShared deletes the
// setup's copies (a card in the other format is pointed at the shared one first); Replace copies them over the shared
// files and points every copy of the set that used a shared counterpart in the other format at the new file.
func resolveDiffering(l loc, all []loc, choice string, res *Result) error {
	rels := differing(l)
	if len(rels) == 0 {
		return nil
	}
	ws := project.Workspace{}
	switch choice {
	case UseShared:
		repoint := map[string]string{}
		for _, rel := range rels {
			if cp := counterpart(l.p, rel); cp != rel {
				repoint[rel] = cp
			}
		}
		if len(repoint) > 0 { // save before deleting: every reference points at an existing file at all times
			l.p.RepointFiles(func(rel string) string {
				if to, ok := repoint[rel]; ok {
					return to
				}
				return rel
			})
			if err := ws.Save(l.p); err != nil {
				return err
			}
		}
	case Replace:
		repoint := map[string]string{} // shared counterpart → this setup's path (worked out before the library changes)
		for _, rel := range rels {
			if cp := counterpart(l.p, rel); cp != "" && cp != rel {
				repoint[cp] = rel
			}
		}
		for _, rel := range rels {
			if err := res.keepCopy(lib(l.p, rel), own(l.p, rel)); err != nil { // the shared version other setups had
				return err
			}
			if err := fsutil.CopyFile(own(l.p, rel), lib(l.p, rel)); err != nil {
				return err
			}
		}
		for _, o := range all {
			if len(repoint) == 0 {
				break
			}
			if o.setup == l.setup {
				continue
			}
			changed := false
			o.p.RepointFiles(func(rel string) string {
				if to, ok := repoint[rel]; ok && !fsutil.Exists(own(o.p, rel)) {
					changed = true
					return to
				}
				return rel
			})
			if changed {
				if err := ws.Save(o.p); err != nil {
					return err
				}
			}
		}
	default:
		return errors.New("choose " + UseShared + " or " + Replace)
	}
	for _, rel := range rels { // Replace: now identical to the shared file → deleted; UseShared: the setup's version → kept
		if err := res.discard(own(l.p, rel), lib(l.p, rel)); err != nil {
			return err
		}
	}
	removeEmptyDirs(l.p.Folder, false)
	return nil
}

// foldNamed folds a named art folder into the set's shared art: every copy of the set using "<Name>/x" uses "x" after.
// UseShared keeps the shared files (a file only the named folder has moves up); Replace copies the named files over them.
func foldNamed(h setups.Home, n Named, choice string, res *Result) error {
	bySet, _, err := sets(h)
	if err != nil {
		return err
	}
	base := filepath.Join(workspace.Dir(h.Root).Library(), n.Set)
	dir := filepath.Join(base, n.Name)
	var rels []string
	_ = filepath.WalkDir(dir, func(f string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, f)
			rels = append(rels, filepath.ToSlash(rel))
		}
		return nil
	})
	for _, rel := range rels {
		from, to := filepath.Join(dir, filepath.FromSlash(rel)), filepath.Join(base, filepath.FromSlash(rel))
		if choice == Replace || !fsutil.Exists(to) {
			if err := res.keepCopy(to, from); err != nil {
				return err
			}
			if err := fsutil.CopyFile(from, to); err != nil {
				return err
			}
		}
	}
	prefix := n.Name + "/"
	for _, l := range bySet[n.Set] {
		changed := false
		l.p.RepointFiles(func(rel string) string {
			if strings.HasPrefix(rel, prefix) {
				changed = true
				return strings.TrimPrefix(rel, prefix)
			}
			return rel
		})
		if changed {
			if err := (project.Workspace{}).Save(l.p); err != nil {
				return err
			}
		}
	}
	for _, rel := range rels {
		if err := res.discard(filepath.Join(dir, filepath.FromSlash(rel)), filepath.Join(base, filepath.FromSlash(rel))); err != nil {
			return err
		}
	}
	removeEmptyDirs(dir, true)
	return nil
}

// libMetaFor describes how p's art was imported; ok=false when it can't be told (no images, unknown source).
func libMetaFor(p *project.Project) (project.LibMeta, bool) {
	m := project.LibMeta{Source: p.Meta.Source, Code: p.Meta.SourceCode(), Lang: p.Meta.Lang, Format: "png"}
	if m.Source == "" || m.Source == "manual" || len(p.Set.Cards) == 0 || p.Set.Cards[0].Image == "" {
		return m, false
	}
	first := p.Set.Cards[0].Image
	if strings.EqualFold(path.Ext(first), ".jpg") {
		m.Format = "jpg"
	}
	f, err := os.Open(own(p, first))
	if err != nil {
		f, err = os.Open(lib(p, first))
	}
	if err == nil {
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

// removeEmptyDirs deletes empty folders under dir, and dir itself when self is set and it ends up empty.
func removeEmptyDirs(dir string, self bool) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, d := range des {
		if d.IsDir() {
			removeEmptyDirs(filepath.Join(dir, d.Name()), true)
		}
	}
	if self {
		_ = os.Remove(dir) // only succeeds when empty
	}
}

// Needed reports quickly (stops at the first find) whether anything is still in an older layout: a file other than
// set.json / studio.json in a project folder, an accessories\images folder, a named art folder, or content of the
// active setup that is only in the game folder.
func Needed(h setups.Home, gameDir string) bool {
	if GamePending(h, gameDir) {
		return true
	}
	list, err := h.List()
	if err != nil {
		return false
	}
	roots := append(setups.Folders(list), catalog.For(h.Root).Root)
	errFound := errors.New("found")
	for _, root := range roots {
		if hasFiles(filepath.Join(root, "accessories", oldImagesDir)) {
			return true
		}
		// Image files in a project folder (only of projects Studio can read: Check skips the others too).
		ws := workspace.Dir(h.Root).Projects(root)
		projects, _ := os.ReadDir(ws.ProjectsDir())
		for _, e := range projects {
			if !e.IsDir() {
				continue
			}
			folder := filepath.Join(ws.ProjectsDir(), e.Name())
			if _, err := setfmt.Load(filepath.Join(folder, project.SetFile)); err != nil {
				continue
			}
			err := filepath.WalkDir(folder, func(f string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				if n := d.Name(); n != project.SetFile && n != project.MetaFile && fsutil.IsImage(n) {
					return errFound
				}
				return nil
			})
			if err == errFound {
				return true
			}
		}
	}
	sets, _ := os.ReadDir(workspace.Dir(h.Root).Library())
	for _, s := range sets {
		dirs, _ := os.ReadDir(filepath.Join(workspace.Dir(h.Root).Library(), s.Name()))
		for _, d := range dirs {
			if d.IsDir() && d.Name() != "images" {
				return true
			}
		}
	}
	return false
}

// hasFiles reports whether dir holds at least one file (an empty leftover folder needs no upgrade).
func hasFiles(dir string) bool {
	found := errors.New("found")
	return filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			return found
		}
		return nil
	}) == found
}

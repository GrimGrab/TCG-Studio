package library

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"tcgstudio/internal/project"
	"tcgstudio/internal/setups"
)

// "Move everything to shared": every file a set refers to (project.Files: card art, card back, pack & box art, editor
// sources, photos) goes from the setups' project folders into the set's shared folder <workspace>\library\<setId>, so a
// setup keeps only its game info. A file already shared with the same bytes is dropped from the setup; a file that differs
// from the shared one is left where it is and listed (Differ) for the player to decide per set: the same art (use the
// shared file) or different art (kept as its own, in a named shared folder library\<setId>\<Name>\). Nothing is guessed.

// Differ is a setup's set whose files differ from the shared files at the same paths.
type Differ struct {
	Setup     string `json:"setup"`     // setup id
	SetupName string `json:"setupName"` //
	Set       string `json:"set"`       // set id
	SetName   string `json:"setName"`   //
	Files     int    `json:"files"`
	Bytes     int64  `json:"bytes"`
	Cards     int    `json:"cards"`     // of which card images
	CardBytes int64  `json:"cardBytes"` // their size (the rest is pack & box art, photos, …)
	Sample    string `json:"sample"`    // a differing file (relative), for the side-by-side preview
}

// counterpart is the shared file a setup's own file rel stands for: the library file at the same path, or a card image
// with the same name in the other format (x.png ↔ x.jpg, left by Shrink in one place). "" when the library has none.
func counterpart(p *project.Project, rel string) string {
	if p.LibFolder == "" {
		return ""
	}
	if fileExists(filepath.Join(p.LibFolder, filepath.FromSlash(rel))) {
		return rel
	}
	ext := strings.ToLower(path.Ext(rel))
	if ext != ".png" && ext != ".jpg" {
		return ""
	}
	other := strings.TrimSuffix(rel, path.Ext(rel)) + map[string]string{".png": ".jpg", ".jpg": ".png"}[ext]
	if fileExists(filepath.Join(p.LibFolder, filepath.FromSlash(other))) {
		return other
	}
	return ""
}

// ownFilesOf lists the files of a set kept in its project folder that it refers to.
func ownFilesOf(p *project.Project) []string {
	var out []string
	for _, rel := range p.Files() {
		if fileExists(filepath.Join(p.Folder, filepath.FromSlash(rel))) {
			out = append(out, rel)
		}
	}
	return out
}

// setLeftovers lists image files in a set's project folder that nothing refers to (e.g. PNGs left by Shrink, older Smart
// art). Only image files: set.json, studio.json and anything else stay.
func setLeftovers(p *project.Project) []string {
	used := map[string]bool{}
	for _, rel := range p.Files() {
		used[filepath.Clean(filepath.Join(p.Folder, filepath.FromSlash(rel)))] = true
	}
	var out []string
	_ = filepath.WalkDir(p.Folder, func(f string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || used[filepath.Clean(f)] {
			return nil
		}
		rel, _ := filepath.Rel(p.Folder, f)
		if project.IsRelFile(filepath.ToSlash(rel)) || isImageName(f) {
			out = append(out, f)
		}
		return nil
	})
	return out
}

func isImageName(f string) bool {
	switch strings.ToLower(filepath.Ext(f)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".svg", ".gif", ".bmp":
		return true
	}
	return false
}

// diffOf works out what a setup's set still keeps for itself after a move: files that differ from the shared ones.
func diffOf(l loc, names map[string]string) (Differ, []string) {
	d := Differ{Setup: l.setup, SetupName: names[l.setup], Set: l.p.ID, SetName: l.p.Set.Name}
	cards := map[string]bool{}
	for _, rel := range cardImages(l.p) {
		cards[rel] = true
	}
	var rels []string
	for _, rel := range ownFilesOf(l.p) {
		cp := counterpart(l.p, rel)
		if cp == "" {
			continue // the library has no such file: Move takes it
		}
		own := filepath.Join(l.p.Folder, filepath.FromSlash(rel))
		if cp == rel && sameFile(own, filepath.Join(l.p.LibFolder, filepath.FromSlash(rel))) {
			continue // identical: Move drops it
		}
		rels = append(rels, rel)
		d.Files++
		d.Bytes += size(own)
		if cards[rel] {
			d.Cards++
			d.CardBytes += size(own)
			if d.Sample == "" {
				d.Sample = rel
			}
		}
	}
	if d.Sample == "" && len(rels) > 0 {
		d.Sample = rels[0]
	}
	return d, rels
}

// MoveAll moves every setup's (and catalog template's) set files into the shared library and deletes leftovers; then the
// accessory and furniture files into the shared store (MoveAccessories). Differing files stay (see Differ). Each file is
// moved or dropped on its own and paths don't change, so an interrupted run just continues next time.
func MoveAll(ctx context.Context, h setups.Home, progress func(Progress)) (*Result, error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	bySet, _, err := sets(h)
	if err != nil {
		return nil, err
	}
	res := &Result{}
	ids := sortedIDs(bySet)
	for i, id := range ids {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		progress(Progress{Message: fmt.Sprintf("Moving files of %s…", id), Done: i, Total: len(ids)})
		changed := false
		for _, l := range bySet[id] {
			if l.p.LibFolder == "" {
				continue
			}
			// The set's import settings, for Import's "already in the library" check, when the library has none yet.
			if m, ok := libMetaFor(l.p); ok && project.LoadLibMeta(l.p.LibFolder) == nil {
				if err := project.SaveLibMeta(l.p.LibFolder, &m); err != nil {
					return res, err
				}
			}
			for _, rel := range ownFilesOf(l.p) {
				own := filepath.Join(l.p.Folder, filepath.FromSlash(rel))
				lib := filepath.Join(l.p.LibFolder, filepath.FromSlash(rel))
				switch cp := counterpart(l.p, rel); {
				case cp == "":
					if err := moveFile(own, lib); err != nil {
						return res, err
					}
				case cp == rel && (sameFile(own, lib) || l.setup == catalogSetup):
					// The same file is shared already, or a catalog template's stale copy (see catalogSetup).
					n := size(own)
					if err := os.Remove(own); err != nil {
						return res, err
					}
					res.Freed += n
				default:
					continue // differs from the shared file: the player decides (ResolveDiffering)
				}
				res.Files++
				changed = true
			}
			for _, f := range setLeftovers(l.p) {
				n := size(f)
				if os.Remove(f) == nil {
					res.Freed += n
					res.Files++
					changed = true
				}
			}
			removeEmptyDirs(filepath.Join(l.p.Folder, "images"))
		}
		if changed {
			res.Sets++
		}
	}
	progress(Progress{Message: "Moving accessory files…", Done: len(ids), Total: len(ids)})
	ar, err := MoveAccessories(ctx, h, progress)
	if ar != nil {
		res.Files += ar.Files
		res.Freed += ar.Freed
	}
	return res, err
}

// Choices for ResolveDiffering.
const (
	UseShared = "shared"  // the same art: the setup's differing files go, it uses the shared ones
	KeepOwn   = "own"     // different art: moved to the named shared folder library\<setId>\<Name>\
	Replace   = "replace" // this setup's art becomes the shared art (every setup using the shared files shows it)
)

// CleanFolderName makes a player-given name usable as a shared folder name ("" when nothing usable is left).
func CleanFolderName(name string) string {
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return -1
		}
		return r
	}, strings.TrimSpace(name))
	name = strings.Trim(name, ". ")
	if strings.EqualFold(name, "images") || len(name) > 80 {
		return ""
	}
	return name
}

// ResolveDiffering applies the player's choice for one setup's set whose files differ from the shared ones.
// UseShared deletes the setup's differing files (a card in the other format is pointed at the shared one first);
// KeepOwn moves them to library\<setId>\<name>\ and points the set at them. The set id never changes.
func ResolveDiffering(h setups.Home, setupID, setID, choice, name string) (*Result, error) {
	bySet, list, err := sets(h)
	if err != nil {
		return nil, err
	}
	names := map[string]string{}
	for _, s := range list {
		names[s.ID] = s.Name
	}
	var l *loc
	for i := range bySet[setID] {
		if bySet[setID][i].setup == setupID {
			l = &bySet[setID][i]
		}
	}
	if l == nil || l.p.LibFolder == "" {
		return nil, errors.New("set not found in that setup")
	}
	_, rels := diffOf(*l, names)
	res := &Result{}
	if len(rels) == 0 {
		return res, nil
	}
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
			if err := (project.Workspace{}).Save(l.p); err != nil {
				return res, err
			}
		}
		for _, rel := range rels {
			f := filepath.Join(l.p.Folder, filepath.FromSlash(rel))
			n := size(f)
			if os.Remove(f) == nil {
				res.Freed += n
				res.Files++
			}
		}
	case KeepOwn:
		name = CleanFolderName(name)
		if name == "" {
			return nil, errors.New("give the folder a name (letters, numbers, spaces)")
		}
		dir := filepath.Join(l.p.LibFolder, name)
		for _, rel := range rels { // never mix with another folder's different files
			to := filepath.Join(dir, filepath.FromSlash(rel))
			if fileExists(to) && !sameFile(to, filepath.Join(l.p.Folder, filepath.FromSlash(rel))) {
				return nil, fmt.Errorf("the shared folder %q already holds other art — pick another name", name)
			}
		}
		for _, rel := range rels {
			own := filepath.Join(l.p.Folder, filepath.FromSlash(rel))
			to := filepath.Join(dir, filepath.FromSlash(rel))
			if fileExists(to) { // the same file is there already
				continue
			}
			if err := copyFile(own, to); err != nil {
				return res, err
			}
		}
		moved := map[string]bool{}
		for _, rel := range rels {
			moved[rel] = true
		}
		l.p.RepointFiles(func(rel string) string {
			if moved[rel] {
				return name + "/" + rel
			}
			return rel
		})
		if err := (project.Workspace{}).Save(l.p); err != nil {
			return res, err
		}
		for _, rel := range rels {
			f := filepath.Join(l.p.Folder, filepath.FromSlash(rel))
			if os.Remove(f) == nil {
				res.Files++
			}
		}
	case Replace:
		// The setup's file goes to the shared path it stands for; a counterpart in the other format (x.jpg for this x.png)
		// is replaced by this file under its own name, and every copy of the set pointing at the counterpart follows.
		repoint := map[string]string{} // shared counterpart → this setup's path (worked out before the library changes)
		for _, rel := range rels {
			if cp := counterpart(l.p, rel); cp != "" && cp != rel {
				repoint[cp] = rel
			}
		}
		for _, rel := range rels {
			own := filepath.Join(l.p.Folder, filepath.FromSlash(rel))
			if err := copyFile(own, filepath.Join(l.p.LibFolder, filepath.FromSlash(rel))); err != nil {
				return res, err
			}
		}
		if len(repoint) > 0 {
			for _, o := range bySet[setID] {
				changed := false
				o.p.RepointFiles(func(rel string) string {
					if to, ok := repoint[rel]; ok && o.p.InLibrary(rel) {
						changed = true
						return to
					}
					return rel
				})
				if changed {
					if err := (project.Workspace{}).Save(o.p); err != nil {
						return res, err
					}
				}
			}
		}
		for _, rel := range rels { // the setup now uses the shared copy of its own art
			if os.Remove(filepath.Join(l.p.Folder, filepath.FromSlash(rel))) == nil {
				res.Files++
			}
		}
	default:
		return nil, errors.New("choose " + UseShared + ", " + KeepOwn + " or " + Replace)
	}
	removeEmptyDirs(filepath.Join(l.p.Folder, "images"))
	res.Sets = 1
	return res, nil
}

// DifferPreview is one differing file of a setup's set next to the shared one, for the player's choice.
type DifferPreview struct {
	File   string `json:"file"`
	Own    string `json:"own"`    // data: URL
	Shared string `json:"shared"` // data: URL
}

// PreviewDiffer returns the sample file of a setup's set and its shared counterpart.
func PreviewDiffer(h setups.Home, setupID, setID string) (*DifferPreview, error) {
	bySet, _, err := sets(h)
	if err != nil {
		return nil, err
	}
	for _, l := range bySet[setID] {
		if l.setup != setupID {
			continue
		}
		d, _ := diffOf(l, nil)
		if d.Sample == "" {
			return nil, errors.New("nothing differs")
		}
		own, err := dataURL(filepath.Join(l.p.Folder, filepath.FromSlash(d.Sample)))
		if err != nil {
			return nil, err
		}
		shared, err := dataURL(filepath.Join(l.p.LibFolder, filepath.FromSlash(counterpart(l.p, d.Sample))))
		if err != nil {
			return nil, err
		}
		return &DifferPreview{File: d.Sample, Own: own, Shared: shared}, nil
	}
	return nil, errors.New("set not found in that setup")
}

func dataURL(f string) (string, error) {
	b, err := os.ReadFile(f)
	if err != nil {
		return "", err
	}
	mime := map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp",
		".svg": "image/svg+xml", ".gif": "image/gif"}[strings.ToLower(filepath.Ext(f))]
	if mime == "" {
		mime = "application/octet-stream"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b), nil
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return writeAtomic(dst, b)
}

// differs lists every setup's sets whose files differ from the shared ones (catalog templates left out: they follow the
// setup they were made from).
func differs(bySet map[string][]loc, names map[string]string) []Differ {
	out := []Differ{}
	for _, id := range sortedIDs(bySet) {
		for _, l := range bySet[id] {
			if l.setup == catalogSetup || l.p.LibFolder == "" {
				continue
			}
			if d, rels := diffOf(l, names); len(rels) > 0 {
				out = append(out, d)
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Bytes > out[j].Bytes })
	return out
}

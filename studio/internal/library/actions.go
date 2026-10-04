package library

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
)

// Result of Move or Shrink.
type Result struct {
	Files int   `json:"files"` // card images moved or converted
	Freed int64 `json:"freed"` // bytes freed in the workspace
	Sets  int   `json:"sets"`  // sets changed
}

// Move takes card art from the setups' project folders into the shared library. For each set and image the first copy
// moves; copies with the same bytes in other setups are deleted; copies that differ stay (that setup's own art).
func Move(ctx context.Context, h setups.Home, progress func(Progress)) (*Result, error) {
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
		progress(Progress{Message: fmt.Sprintf("Moving card art of %s…", id), Done: i, Total: len(ids)})
		locs := bySet[id]
		// The first shareable copy defines the library folder's import settings when it has none yet.
		for _, l := range locs {
			if m, ok := shareable(l.p); ok && project.LoadLibMeta(l.p.LibFolder) == nil {
				if err := project.SaveLibMeta(l.p.LibFolder, &m); err != nil {
					return res, err
				}
				break
			}
		}
		var share []loc
		for _, l := range locs {
			if _, ok := shareable(l.p); ok {
				share = append(share, l)
			}
		}
		changed := false
		for _, rel := range unionImages(share) {
			lib := filepath.Join(share[0].p.LibFolder, filepath.FromSlash(rel))
			for _, l := range share {
				own := filepath.Join(l.p.Folder, filepath.FromSlash(rel))
				if !fileExists(own) {
					continue
				}
				switch {
				case !fileExists(lib):
					if err := moveFile(own, lib); err != nil {
						return res, err
					}
				case sameFile(own, lib):
					n := size(own)
					if err := os.Remove(own); err != nil {
						return res, err
					}
					res.Freed += n
				default:
					continue // this setup's own version of the card: stays
				}
				res.Files++
				changed = true
			}
		}
		if changed {
			res.Sets++
		}
	}
	progress(Progress{Message: "Done", Done: len(ids), Total: len(ids)})
	return res, nil
}

// Shrink converts the PNG card art of the given sets of setup setupID to JPEG. Art in the shared library is converted once
// and every setup using it is updated; the setup's own PNG card art is converted for that setup only. Card art added or
// changed in Studio under another file name is left alone.
func Shrink(ctx context.Context, h setups.Home, setupID string, ids []string, progress func(Progress)) (*Result, error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	bySet, _, err := sets(h)
	if err != nil {
		return nil, err
	}
	res := &Result{}
	for i, id := range ids {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		progress(Progress{Message: fmt.Sprintf("Converting %s…", id), Done: i, Total: len(ids)})
		n, freed, err := shrinkSet(ctx, bySet[id], setupID)
		res.Files += n
		res.Freed += freed
		if n > 0 {
			res.Sets++
		}
		if err != nil {
			return res, err
		}
	}
	progress(Progress{Message: "Done", Done: len(ids), Total: len(ids)})
	return res, nil
}

func jpgName(rel string) string { return strings.TrimSuffix(rel, path.Ext(rel)) + ".jpg" }

func isPNG(rel string) bool { return strings.EqualFold(path.Ext(rel), ".png") && defaultName(rel) }

func shrinkSet(ctx context.Context, locs []loc, setupID string) (files int, freed int64, err error) {
	var sel *loc
	for i := range locs {
		if locs[i].setup == setupID {
			sel = &locs[i]
		}
	}
	if sel == nil {
		return 0, 0, nil
	}
	// 1. Write the JPEGs next to the PNGs: library art (shared) and the selected setup's own art.
	var written int64
	convert := func(png string) error {
		jpg := strings.TrimSuffix(png, filepath.Ext(png)) + ".jpg"
		if fileExists(jpg) {
			return nil
		}
		b, err := os.ReadFile(png)
		if err != nil {
			return err
		}
		out, err := toJPEG(b)
		if err != nil {
			return err
		}
		written += int64(len(out))
		return writeAtomic(jpg, out)
	}
	libConverted := map[string]bool{}
	for _, rel := range cardImages(sel.p) {
		if ctx.Err() != nil {
			return files, freed, ctx.Err()
		}
		if !isPNG(rel) {
			continue
		}
		f := sel.p.ImagePath(rel)
		if !fileExists(f) {
			continue
		}
		if err := convert(f); err != nil {
			return files, freed, err
		}
		if sel.p.InLibrary(rel) {
			libConverted[rel] = true
		}
	}
	// 2. Point set.json at the JPEGs: the selected setup for all converted cards, the others for shared library art.
	var oldFiles []string
	for _, l := range locs {
		changed := false
		for ci, c := range l.p.Set.Cards {
			if !isPNG(c.Image) {
				continue
			}
			inLib := l.p.InLibrary(c.Image)
			if (l.setup == setupID && fileExists(jpgPathFor(l.p, c.Image))) || (inLib && libConverted[c.Image]) {
				if !inLib {
					oldFiles = append(oldFiles, l.p.ImagePath(c.Image))
				}
				l.p.Set.Cards[ci].Image = jpgName(c.Image)
				changed = true
			}
		}
		if changed {
			if err := (project.Workspace{}).Save(l.p); err != nil {
				return files, freed, err
			}
		}
	}
	// 3. Delete the PNGs no set.json uses any more.
	for rel := range libConverted {
		lib := filepath.Join(sel.p.LibFolder, filepath.FromSlash(rel))
		if usedBy(locs, rel, true) {
			continue
		}
		n := size(lib)
		if os.Remove(lib) == nil {
			freed += n
			files++
		}
	}
	for _, f := range oldFiles {
		n := size(f)
		if os.Remove(f) == nil {
			freed += n
			files++
		}
	}
	freed -= written // net: the PNGs deleted minus the JPEGs written
	// A converted file identical to the library's copy (the same art another setup converted before) isn't needed: the
	// lookup finds the library one.
	if sel.p.LibFolder != "" {
		for _, rel := range cardImages(sel.p) {
			own := filepath.Join(sel.p.Folder, filepath.FromSlash(rel))
			lib := filepath.Join(sel.p.LibFolder, filepath.FromSlash(rel))
			if fileExists(own) && fileExists(lib) && sameFile(own, lib) {
				n := size(own)
				if os.Remove(own) == nil {
					freed += n
				}
			}
		}
	}
	if len(libConverted) > 0 {
		if m := project.LoadLibMeta(sel.p.LibFolder); m != nil && !usesPNGFromLib(locs) {
			m.Format = "jpg"
			_ = project.SaveLibMeta(sel.p.LibFolder, m)
		}
	}
	return files, freed, nil
}

// jpgPathFor is where the JPEG of rel was written (next to the PNG it was made from).
func jpgPathFor(p *project.Project, rel string) string {
	f := p.ImagePath(rel)
	return strings.TrimSuffix(f, filepath.Ext(f)) + ".jpg"
}

// usedBy reports whether any setup's set.json still uses rel from the library.
func usedBy(locs []loc, rel string, fromLib bool) bool {
	for _, l := range locs {
		for _, c := range l.p.Set.Cards {
			if c.Image == rel && (!fromLib || l.p.InLibrary(rel)) {
				return true
			}
		}
	}
	return false
}

func usesPNGFromLib(locs []loc) bool {
	for _, l := range locs {
		for _, c := range l.p.Set.Cards {
			if isPNG(c.Image) && l.p.InLibrary(c.Image) {
				return true
			}
		}
	}
	return false
}

// DeleteUnused removes library sets (folders under root) that no setup uses. ids empty = every unused one.
func DeleteUnused(h setups.Home, root string, ids []string) (int64, error) {
	bySet, _, err := sets(h)
	if err != nil {
		return 0, err
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var freed int64
	for _, u := range unused(root, bySet) {
		if len(ids) > 0 && !want[u.ID] {
			continue
		}
		if !setfmt.SafeID(u.ID) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, u.ID)); err != nil {
			return freed, err
		}
		freed += u.Size
	}
	return freed, nil
}

func sortedIDs(m map[string][]loc) []string {
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		in.Close()
		return err
	}
	_, err = io.Copy(out, in)
	in.Close()
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, dst)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Remove(src)
}

func writeAtomic(p string, b []byte) error {
	tmp := p + ".part"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

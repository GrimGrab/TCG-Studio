package library

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setups"
)

// Result of Shrink.
type Result struct {
	Files int   `json:"files"` // card images converted
	Freed int64 `json:"freed"` // bytes freed in the workspace
	Sets  int   `json:"sets"`  // sets changed
}

// Shrink converts the PNG card art of the given sets to JPEG: once in the set's shared folder, then every setup's (and
// the catalog's) set.json is pointed at the JPEGs and the PNGs are deleted. Card art added or changed in Studio under
// another file name is left alone.
func Shrink(ctx context.Context, h setups.Home, ids []string, progress func(Progress)) (*Result, error) {
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
		n, freed, err := shrinkSet(ctx, bySet[id])
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

func shrinkSet(ctx context.Context, locs []loc) (files int, freed int64, err error) {
	if len(locs) == 0 {
		return 0, 0, nil
	}
	p := locs[0].p // every copy reads the same shared folder
	// 1. Write the JPEGs next to the PNGs.
	converted := map[string]bool{}
	for _, rel := range pngCards(locs) {
		if ctx.Err() != nil {
			return files, freed, ctx.Err()
		}
		jpg := p.ImagePath(jpgName(rel))
		if !fsutil.Exists(jpg) {
			b, err := os.ReadFile(p.ImagePath(rel))
			if err != nil {
				return files, freed, err
			}
			out, err := toJPEG(b)
			if err != nil {
				return files, freed, err
			}
			if err := fsutil.WriteAtomic(jpg, out); err != nil {
				return files, freed, err
			}
			freed -= int64(len(out))
		}
		converted[rel] = true
	}
	if len(converted) == 0 {
		return 0, 0, nil
	}
	// 2. Point every copy's set.json at the JPEGs: every picture a card names (main, back, version pictures, mask), the same
	// list step 1 converted (Card.Images / RepointImages).
	for _, l := range locs {
		changed := false
		for ci := range l.p.Set.Cards {
			l.p.Set.Cards[ci].RepointImages(func(rel *string) {
				if converted[*rel] {
					*rel = jpgName(*rel)
					changed = true
				}
			})
		}
		if changed {
			if err := (project.Workspace{}).Save(l.p); err != nil {
				return files, freed, err
			}
		}
	}
	// 3. Delete the PNGs.
	for rel := range converted {
		f := p.ImagePath(rel)
		n := size(f)
		if os.Remove(f) == nil {
			freed += n
			files++
		}
	}
	if m := project.LoadLibMeta(p.LibFolder); m != nil {
		m.Format = "jpg"
		_ = project.SaveLibMeta(p.LibFolder, m)
	}
	return files, freed, nil
}

// RepairShrunk points pictures that an older Shrink left on deleted PNGs at the JPEGs it wrote. That Shrink converted every
// picture of a card but only renamed the main one in set.json, so back pictures, version pictures and shader masks named a
// .png that was gone while its .jpg was right beside it. Only such pairs are touched (a missing PNG with no JPEG stays as it
// is). Every setup's and the catalog's copy is fixed. Returns the names of the sets changed.
// migration:shrink-repoint
func RepairShrunk(h setups.Home) ([]string, error) {
	bySet, _, err := sets(h)
	if err != nil {
		return nil, err
	}
	var fixed []string
	for _, id := range sortedIDs(bySet) {
		changedSet := false
		for _, l := range bySet[id] {
			changed := false
			for ci := range l.p.Set.Cards {
				l.p.Set.Cards[ci].RepointImages(func(rel *string) {
					if !isPNG(*rel) || fsutil.Exists(l.p.ImagePath(*rel)) {
						return
					}
					if jpg := jpgName(*rel); fsutil.Exists(l.p.ImagePath(jpg)) {
						*rel = jpg
						changed = true
					}
				})
			}
			if !changed {
				continue
			}
			if err := (project.Workspace{}).Save(l.p); err != nil {
				return fixed, err
			}
			changedSet = true
		}
		if changedSet {
			fixed = append(fixed, bySet[id][0].p.Set.Name)
		}
	}
	return fixed, nil
}

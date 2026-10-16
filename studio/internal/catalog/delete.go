package catalog

import (
	"fmt"
	"os"
	"path/filepath"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
)

// Deleting from the catalog deletes everywhere (user decision 2026-10-06): the template, every setup's copy and the shared
// files nothing uses any more, and each setup's installed sets lose them (the game reads those from the setup).

// Deleted reports a DeleteEverywhere.
type Deleted struct {
	Setups  []string `json:"setups"`  // setup ids that had something removed
	SetIDs  []string `json:"setIds"`  // set ids no setup has any more
	Removed int      `json:"removed"` // copies removed (templates + setups)
}

// DeleteEverywhere removes sets or accessories/furniture (by id) from the catalog and from every setup.
func (c Catalog) DeleteEverywhere(h setups.Home, kind string, keys []string) (*Deleted, error) {
	list, err := h.List()
	if err != nil {
		return nil, err
	}
	res := &Deleted{Setups: []string{}, SetIDs: []string{}}
	touched := map[string]bool{}
	if kind == KindSet {
		for _, key := range keys {
			id := key
			if !setfmt.SafeID(id) {
				return res, fmt.Errorf("bad set id %q", id)
			}
			for _, s := range list {
				ws := c.ws().Projects(s.Folder)
				if _, err := ws.Load(id); err != nil {
					continue
				}
				if err := ws.Delete(id); err != nil {
					return res, err
				}
				res.Removed++
				touched[s.ID] = true
				_ = setups.EditInfo(s.Folder, func(info *setups.Info) {
					kept := info.InstalledSets[:0]
					for _, x := range info.InstalledSets {
						if x != id {
							kept = append(kept, x)
						}
					}
					info.InstalledSets = kept
				})
			}
			if _, err := c.Sets().Load(id); err == nil {
				if err := c.Sets().Delete(id); err != nil {
					return res, err
				}
				res.Removed++
			}
			// The set's shared folder goes once nothing has the set any more.
			if !c.setInUse(list, id) {
				_ = os.RemoveAll(filepath.Join(c.ws().Library(), id))
				res.SetIDs = append(res.SetIDs, id)
			}
		}
	} else {
		store := assets.For(c.Workspace)
		var refs []string
		drop := func(l *accessories.Library, id string) bool {
			if _, _, ok := l.Entry(id); !ok {
				return false
			}
			refs = append(refs, l.ItemRefs(id)...)
			l.Delete(id)
			l.DeleteFurniture(id)
			l.DeleteDecoration(id)
			return true
		}
		for _, s := range list {
			l, err := accessories.Open(s.Folder, store)
			if err != nil {
				continue
			}
			n := 0
			for _, id := range keys {
				if drop(l, id) {
					n++
				}
			}
			if n > 0 {
				if err := l.Save(); err != nil {
					return res, err
				}
				res.Removed += n
				touched[s.ID] = true
			}
		}
		if cat, err := c.Items(); err == nil {
			n := 0
			for _, id := range keys {
				if drop(cat, id) {
					n++
				}
			}
			if n > 0 {
				if err := cat.Save(); err != nil {
					return res, err
				}
				res.Removed += n
			}
		}
		still := accessories.StoreRefs(append(setups.Folders(list), c.Root)...)
		for _, r := range refs {
			if !still[r] {
				_ = os.Remove(store.Path(r))
			}
		}
	}
	for _, s := range list {
		if touched[s.ID] {
			res.Setups = append(res.Setups, s.ID)
		}
	}
	return res, nil
}

// setInUse reports whether any setup or template still has set id (folder != "": using that own-art folder).
func (c Catalog) setInUse(list []setups.Summary, id string) bool {
	roots := []string{c.Root}
	for _, s := range list {
		roots = append(roots, s.Folder)
	}
	for _, root := range roots {
		ws := c.ws().Projects(root)
		if _, err := os.Stat(filepath.Join(ws.Folder(id), project.SetFile)); err == nil {
			return true // there (readable or not: keep its files)
		}
	}
	return false
}

// storeRefs is every store file any setup or the catalog refers to.

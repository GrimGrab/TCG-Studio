package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
)

// Deleting from the catalog deletes everywhere (user decision 2026-10-06): the template, every setup's copy and the shared
// files nothing uses any more. The game folder is the App's part (it knows the active setup and whether the game runs).

// Deleted reports a DeleteEverywhere.
type Deleted struct {
	Setups  []string `json:"setups"`  // setup ids that had something removed
	SetIDs  []string `json:"setIds"`  // set ids no setup has any more (the App removes the game's copies)
	Removed int      `json:"removed"` // copies removed (templates + setups)
}

// DeleteEverywhere removes sets (by Entry.Key) or accessories/furniture (by id) from the catalog and from every setup.
func (c Catalog) DeleteEverywhere(h setups.Home, kind string, keys []string) (*Deleted, error) {
	list, err := h.List()
	if err != nil {
		return nil, err
	}
	res := &Deleted{Setups: []string{}, SetIDs: []string{}}
	touched := map[string]bool{}
	if kind == KindSet {
		for _, key := range keys {
			id, folder, _ := strings.Cut(key, "|")
			if !setfmt.SafeID(id) {
				return res, fmt.Errorf("bad set id %q", id)
			}
			match := func(p *project.Project) bool { return keyOf(p) == key }
			for _, s := range list {
				ws := project.Workspace{Root: s.Folder, Library: project.LibraryDir(c.Workspace)}
				p, err := ws.Load(id)
				if err != nil || !match(p) {
					continue
				}
				if err := ws.Delete(id); err != nil {
					return res, err
				}
				res.Removed++
				touched[s.ID] = true
				if info, err := setups.LoadInfo(s.Folder); err == nil {
					kept := info.InstalledSets[:0]
					for _, x := range info.InstalledSets {
						if x != id {
							kept = append(kept, x)
						}
					}
					info.InstalledSets = kept
					_ = setups.SaveInfo(s.Folder, info)
				}
			}
			if p, err := c.Sets().Load(id); err == nil && match(p) {
				if err := c.Sets().Delete(id); err != nil {
					return res, err
				}
				res.Removed++
			}
			// Shared files: the named folder of own art, or the whole set folder once nothing has the set any more.
			lib := project.LibraryDir(c.Workspace)
			if !c.setInUse(list, id, "") {
				_ = os.RemoveAll(filepath.Join(lib, id))
				res.SetIDs = append(res.SetIDs, id)
			} else if folder != "" && !c.setInUse(list, id, folder) {
				_ = os.RemoveAll(filepath.Join(lib, id, folder))
			}
		}
	} else {
		store := assets.For(c.Workspace)
		var refs []string
		drop := func(l *accessories.Library, id string) bool {
			var entry any
			if i := l.FurnitureIndex(id); i >= 0 {
				entry = l.Lib.Furniture[i]
			} else if i := l.Index(id); i >= 0 {
				entry = l.Lib.Accessories[i]
			} else {
				return false
			}
			if b, err := json.Marshal(entry); err == nil {
				refs = append(refs, assets.Refs(b)...)
			}
			if raw, ok := l.Meta.Layouts[id]; ok {
				refs = append(refs, assets.Refs(raw)...)
			}
			l.Delete(id)
			l.DeleteFurniture(id)
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
		still := c.storeRefs(list)
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
func (c Catalog) setInUse(list []setups.Summary, id, folder string) bool {
	roots := []string{c.Root}
	for _, s := range list {
		roots = append(roots, s.Folder)
	}
	for _, root := range roots {
		ws := project.Workspace{Root: root, Library: project.LibraryDir(c.Workspace)}
		p, err := ws.Load(id)
		if err != nil {
			if _, serr := os.Stat(filepath.Join(ws.Folder(id), project.SetFile)); serr == nil {
				return true // there but unreadable: keep its files
			}
			continue
		}
		if folder == "" || p.ArtFolder() == folder {
			return true
		}
	}
	return false
}

// storeRefs is every store file any setup or the catalog refers to.
func (c Catalog) storeRefs(list []setups.Summary) map[string]bool {
	refs := map[string]bool{}
	dirs := []string{accessories.Folder(c.Root)}
	for _, s := range list {
		dirs = append(dirs, accessories.Folder(s.Folder))
	}
	for _, dir := range dirs {
		for _, f := range []string{accessories.LibraryFile, accessories.MetaFile} {
			if b, err := os.ReadFile(filepath.Join(dir, f)); err == nil {
				for _, r := range assets.Refs(b) {
					refs[r] = true
				}
			}
		}
	}
	return refs
}

package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/catalog"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
)

// "Not used by any setup", whatever the kind: everything stored that no setup refers to.

// Kinds of UnusedEntry.
const (
	UnusedSetArt      = "set-art"      // library\<setId> of a set nothing has (no setup, no catalog template)
	UnusedSetFiles    = "set-files"    // files in a used set's library folder that nothing refers to
	UnusedGameArt     = "game-art"     // the game's <plugin>\Library\<setId> of a set no setup has
	UnusedSharedFiles = "shared-files" // accessory/furniture store files nothing refers to
	UnusedCatalogSet  = "catalog-set"  // a set only the catalog keeps (no setup has it)
	UnusedCatalogItem = "catalog-item" // an accessory or furniture piece only the catalog keeps
)

// UnusedEntry is one thing no setup uses.
type UnusedEntry struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Files     int    `json:"files"`
	InCatalog bool   `json:"inCatalog"` // kept by the catalog: deleting removes it from the catalog too
	Note      string `json:"note,omitempty"`
}

// brokenSets are set ids with a project folder whose set.json can't be read (in any setup or the catalog). Their shared
// files are never reported as unused: a set that failed to load must not lose its art.
func brokenSets(h setups.Home) map[string]bool {
	out := map[string]bool{}
	list, _ := h.List()
	dirs := append([]string{filepath.Join(h.Root, catalog.DirName)}, func() []string {
		var d []string
		for _, s := range list {
			d = append(d, s.Folder)
		}
		return d
	}()...)
	for _, dir := range dirs {
		ws := project.Workspace{Root: dir, Library: project.LibraryDir(h.Root)}
		entries, _ := os.ReadDir(ws.ProjectsDir())
		for _, e := range entries {
			if !e.IsDir() || strings.Contains(e.Name(), ".") {
				continue
			}
			if _, err := os.Stat(filepath.Join(ws.Folder(e.Name()), project.SetFile)); err != nil {
				continue
			}
			if _, err := ws.Load(e.Name()); err != nil {
				out[e.Name()] = true
			}
		}
	}
	return out
}

// unusedLibraryFiles lists the files in a set's library folder that no copy of the set refers to (library.json kept).
func unusedLibraryFiles(libFolder string, locs []loc) []string {
	used := map[string]bool{}
	for _, l := range locs {
		for _, rel := range l.p.Files() {
			used[filepath.Clean(filepath.Join(libFolder, filepath.FromSlash(rel)))] = true
		}
	}
	var out []string
	_ = filepath.WalkDir(libFolder, func(f string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || used[filepath.Clean(f)] || strings.EqualFold(d.Name(), "library.json") {
			return nil
		}
		out = append(out, f)
		return nil
	})
	return out
}

// unusedEntries lists everything no setup uses. bySet includes the catalog's templates (setup catalogSetup).
func unusedEntries(h setups.Home, gameDir string, bySet map[string][]loc, list []setups.Summary) []UnusedEntry {
	out := []UnusedEntry{}
	broken := brokenSets(h)
	libRoot := project.LibraryDir(h.Root)
	inSetup := func(id string) bool {
		for _, l := range bySet[id] {
			if l.setup != catalogSetup {
				return true
			}
		}
		return false
	}

	// Library folders: of sets nothing has, and unused files inside used ones.
	entries, _ := os.ReadDir(libRoot)
	for _, e := range entries {
		id := e.Name()
		if !e.IsDir() || broken[id] || !setfmt.SafeID(id) {
			continue
		}
		dir := filepath.Join(libRoot, id)
		if len(bySet[id]) == 0 {
			n, files := dirSize(dir)
			out = append(out, UnusedEntry{Kind: UnusedSetArt, ID: id, Name: id, Size: n, Files: files})
			continue
		}
		if files := unusedLibraryFiles(dir, bySet[id]); len(files) > 0 {
			var n int64
			for _, f := range files {
				n += size(f)
			}
			out = append(out, UnusedEntry{Kind: UnusedSetFiles, ID: id, Name: bySet[id][0].p.Set.Name, Size: n, Files: len(files),
				Note: "older or replaced files in its shared folder"})
		}
	}

	// Sets only the catalog keeps.
	for _, id := range sortedIDs(bySet) {
		if inSetup(id) || broken[id] {
			continue
		}
		var n int64
		var files int
		name := id
		for _, l := range bySet[id] {
			name = l.p.Set.Name
			s, f := dirSize(l.p.Folder)
			n, files = n+s, files+f
		}
		if s, f := dirSize(filepath.Join(libRoot, id)); f > 0 {
			n, files = n+s, files+f
		}
		out = append(out, UnusedEntry{Kind: UnusedCatalogSet, ID: id, Name: name, Size: n, Files: files, InCatalog: true})
	}

	// The game's copies of sets no setup has.
	if gameDir != "" {
		gameRoot := project.GameLibraryDir(gameDir)
		ge, _ := os.ReadDir(gameRoot)
		for _, e := range ge {
			if !e.IsDir() || inSetup(e.Name()) || broken[e.Name()] {
				continue
			}
			n, files := dirSize(filepath.Join(gameRoot, e.Name()))
			out = append(out, UnusedEntry{Kind: UnusedGameArt, ID: e.Name(), Name: e.Name(), Size: n, Files: files,
				Note: "copy in the game folder"})
		}
	}

	// Accessories and furniture only the catalog keeps, then store files nothing refers to.
	store := assets.For(h.Root)
	setupItems := map[string]bool{}
	for _, s := range list {
		if l, err := accessories.Open(s.Folder, store); err == nil {
			for _, a := range l.Lib.Accessories {
				setupItems[a.ID] = true
			}
			for _, f := range l.Lib.Furniture {
				setupItems[f.ID] = true
			}
		}
	}
	setupRefs := referencedBySetups(list)
	if cat, err := catalog.For(h.Root).Items(); err == nil {
		add := func(id, name string, entry any) {
			if setupItems[id] {
				return
			}
			var n int64
			files := 0
			for _, ref := range itemRefs(cat, id, entry) {
				if !setupRefs[ref] {
					n += size(store.Path(ref))
					files++
				}
			}
			out = append(out, UnusedEntry{Kind: UnusedCatalogItem, ID: id, Name: name, Size: n, Files: files, InCatalog: true})
		}
		for _, a := range cat.Lib.Accessories {
			add(a.ID, a.Name, a)
		}
		for _, f := range cat.Lib.Furniture {
			add(f.ID, f.Name, f)
		}
	}
	if store, err := store.List(); err == nil {
		refs := referenced(h, list)
		var n int64
		files := 0
		for _, e := range store {
			if !refs[e.Rel] {
				n += e.Size
				files++
			}
		}
		if files > 0 {
			out = append(out, UnusedEntry{Kind: UnusedSharedFiles, ID: "assets", Name: "Accessory & furniture files", Size: n,
				Files: files, Note: "earlier versions of edited textures, deleted items"})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	return out
}

// referencedBySetups is like referenced, without the catalog.
func referencedBySetups(list []setups.Summary) map[string]bool {
	refs := map[string]bool{}
	for _, s := range list {
		dir := accessories.Folder(s.Folder)
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

// itemRefs is the store files a catalog item refers to (its entry and editor layout).
func itemRefs(cat *accessories.Library, id string, entry any) []string {
	var refs []string
	if b, err := json.Marshal(entry); err == nil {
		refs = append(refs, assets.Refs(b)...)
	}
	if raw, ok := cat.Meta.Layouts[id]; ok {
		refs = append(refs, assets.Refs(raw)...)
	}
	return refs
}

// DeleteUnused deletes unused things of one kind (ids empty = all of that kind; for UnusedSharedFiles ids are ignored).
// Catalog-only sets and items are removed from the catalog too. gameDir "" leaves the game folder alone. Returns bytes freed.
func DeleteUnused(h setups.Home, gameDir, kind string, ids []string) (int64, error) {
	bySet, list, err := sets(h)
	if err != nil {
		return 0, err
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	pick := func(e UnusedEntry) bool { return e.Kind == kind && (len(ids) == 0 || want[e.ID]) }
	var freed int64
	libRoot := project.LibraryDir(h.Root)
	for _, e := range unusedEntries(h, gameDir, bySet, list) {
		if !pick(e) || !setfmt.SafeID(e.ID) && kind != UnusedSharedFiles && kind != UnusedCatalogItem {
			continue
		}
		switch kind {
		case UnusedSetArt:
			if err := os.RemoveAll(filepath.Join(libRoot, e.ID)); err != nil {
				return freed, err
			}
			freed += e.Size
		case UnusedSetFiles:
			dir := filepath.Join(libRoot, e.ID)
			for _, f := range unusedLibraryFiles(dir, bySet[e.ID]) {
				n := size(f)
				if os.Remove(f) == nil {
					freed += n
				}
			}
			removeEmptyDirs(dir)
		case UnusedGameArt:
			if err := os.RemoveAll(filepath.Join(project.GameLibraryDir(gameDir), e.ID)); err != nil {
				return freed, err
			}
			freed += e.Size
		case UnusedCatalogSet:
			if err := catalog.For(h.Root).Remove(catalog.KindSet, e.ID); err != nil {
				return freed, err
			}
			_ = os.RemoveAll(filepath.Join(libRoot, e.ID)) // no setup has the set (else it wouldn't be listed)
			freed += e.Size
		case UnusedCatalogItem:
			c := catalog.For(h.Root)
			cat, err := c.Items()
			if err != nil {
				return freed, err
			}
			k := catalog.KindAccessory
			var entry any
			if i := cat.FurnitureIndex(e.ID); i >= 0 {
				k, entry = catalog.KindFurniture, cat.Lib.Furniture[i]
			} else if i := cat.Index(e.ID); i >= 0 {
				entry = cat.Lib.Accessories[i]
			}
			refs := itemRefs(cat, e.ID, entry)
			if err := c.Remove(k, e.ID); err != nil {
				return freed, err
			}
			still := referenced(h, list) // its files go unless something else still refers to them
			store := assets.For(h.Root)
			for _, r := range refs {
				if !still[r] {
					n := size(store.Path(r))
					if os.Remove(store.Path(r)) == nil {
						freed += n
					}
				}
			}
		case UnusedSharedFiles:
			n, _, err := DeleteUnusedAssets(h)
			if err != nil {
				return freed, err
			}
			freed += n
		}
	}
	return freed, nil
}

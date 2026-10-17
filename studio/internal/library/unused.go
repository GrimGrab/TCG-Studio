package library

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/catalog"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

// "Not used by any setup", whatever the kind: everything stored that no setup refers to.

// Kinds of UnusedEntry.
const (
	UnusedSetArt      = "set-art"      // library\<setId> of a set nothing has (no setup, no catalog template)
	UnusedSetFiles    = "set-files"    // files in a used set's library folder that nothing refers to
	UnusedSharedFiles = "shared-files" // one accessory/furniture/decoration store file nothing refers to (ID = "assets/…")
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
	// Paths: for UnusedSetFiles, the files (relative to the set's shared folder, slash-separated) the entry deletes.
	Paths []string `json:"paths,omitempty"`
	Meta
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
		ws := workspace.Dir(h.Root).Projects(dir)
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
func unusedEntries(h setups.Home, bySet map[string][]loc, list []setups.Summary) []UnusedEntry {
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
			paths := make([]string, 0, len(files))
			for _, f := range files {
				n += size(f)
				rel, _ := filepath.Rel(dir, f)
				paths = append(paths, filepath.ToSlash(rel))
			}
			sort.Strings(paths)
			out = append(out, UnusedEntry{Kind: UnusedSetFiles, ID: id, Name: bySet[id][0].p.Set.Name, Size: n, Files: len(files),
				Note: "files no setup uses (earlier pack & box art versions, replaced card art, unused photos)", Paths: paths})
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

	// Accessories and furniture only the catalog keeps, then store files nothing refers to.
	store := workspace.Dir(h.Root).Store()
	setupItems := map[string]bool{}
	for _, s := range list {
		if l, err := accessories.Open(s.Folder, store); err == nil {
			for _, a := range l.Lib.Accessories {
				setupItems[a.ID] = true
			}
			for _, f := range l.Lib.Furniture {
				setupItems[f.ID] = true
			}
			for _, d := range l.Lib.Decorations {
				setupItems[d.ID] = true
			}
		}
	}
	setupRefs := accessories.StoreRefs(setups.Folders(list)...)
	if cat, err := catalog.For(h.Root).Items(); err == nil {
		add := func(id, name string) {
			if setupItems[id] {
				return
			}
			var n int64
			files := 0
			for _, ref := range cat.ItemRefs(id) {
				if !setupRefs[ref] {
					n += size(store.Path(ref))
					files++
				}
			}
			out = append(out, UnusedEntry{Kind: UnusedCatalogItem, ID: id, Name: name, Size: n, Files: files, InCatalog: true})
		}
		for _, a := range cat.Lib.Accessories {
			add(a.ID, a.Name)
		}
		for _, f := range cat.Lib.Furniture {
			add(f.ID, f.Name)
		}
		for _, d := range cat.Lib.Decorations {
			add(d.ID, d.Name)
		}
	}
	// Each store file nothing refers to (earlier versions of edited textures, deleted items' files), by file.
	if store, err := store.List(); err == nil {
		refs := accessories.StoreRefs(append(setups.Folders(list), catalog.For(h.Root).Root)...)
		for _, e := range store {
			if !refs[e.Rel] {
				out = append(out, UnusedEntry{Kind: UnusedSharedFiles, ID: e.Rel, Name: path.Base(e.Rel), Size: e.Size, Files: 1,
					Note: storeFileType(e.Rel)})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	return out
}

// Pick names one entry of the unused list (UnusedEntry.Kind and ID).
type Pick struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	// Paths: for UnusedSetFiles, the files the player was shown (UnusedEntry.Paths); only those are deleted, so a file
	// written since (an import or edit not saved yet) is never taken along. Empty = all of them.
	Paths []string `json:"paths,omitempty"`
}

type pickKey struct{ kind, id string }

// Deleted is what DeleteUnused did.
type Deleted struct {
	Freed   int64  `json:"freed"`
	Deleted []Pick `json:"deleted"` // the entries gone (the page drops them from its list)
}

// DeleteUnused deletes the picked unused entries (worked out once, then deleted one by one with progress; cancelling keeps
// what's done). Catalog-only sets and items are removed from the catalog too.
func DeleteUnused(ctx context.Context, h setups.Home, picks []Pick, progress func(Progress)) (*Deleted, error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	res := &Deleted{Deleted: []Pick{}}
	progress(Progress{Message: "Checking what's unused…"})
	bySet, list, err := sets(h)
	if err != nil {
		return res, err
	}
	want := map[pickKey]Pick{}
	for _, p := range picks {
		want[pickKey{p.Kind, p.ID}] = p
	}
	var todo []UnusedEntry
	for _, e := range unusedEntries(h, bySet, list) {
		p, ok := want[pickKey{e.Kind, e.ID}]
		if !ok || !(setfmt.SafeID(e.ID) || e.Kind == UnusedSharedFiles || e.Kind == UnusedCatalogItem) {
			continue
		}
		if e.Kind == UnusedSetFiles && len(p.Paths) > 0 {
			shown := map[string]bool{}
			for _, f := range p.Paths {
				shown[f] = true
			}
			kept := e.Paths[:0]
			for _, f := range e.Paths {
				if shown[f] {
					kept = append(kept, f)
				}
			}
			e.Paths = kept
		}
		todo = append(todo, e)
	}
	libRoot := project.LibraryDir(h.Root)
	for i, e := range todo {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		progress(Progress{Message: "Deleting " + e.Name + "…", Done: i, Total: len(todo)})
		freed, err := deleteEntry(h, libRoot, e, bySet, list)
		res.Freed += freed
		if err != nil {
			return res, err
		}
		res.Deleted = append(res.Deleted, Pick{Kind: e.Kind, ID: e.ID})
	}
	progress(Progress{Message: "Done", Done: len(todo), Total: len(todo)})
	return res, nil
}

// deleteEntry deletes one unused entry; returns bytes freed.
func deleteEntry(h setups.Home, libRoot string, e UnusedEntry, bySet map[string][]loc, list []setups.Summary) (int64, error) {
	var freed int64
	switch e.Kind {
	case UnusedSetArt:
		if err := os.RemoveAll(filepath.Join(libRoot, e.ID)); err != nil {
			return freed, err
		}
		freed += e.Size
	case UnusedSetFiles:
		dir := filepath.Join(libRoot, e.ID)
		for _, rel := range e.Paths { // still unused (listed again just now) and shown to the player
			f := filepath.Join(dir, filepath.FromSlash(rel))
			n := size(f)
			if os.Remove(f) == nil {
				freed += n
			}
		}
		removeEmptyDirs(dir)
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
		k, _, ok := cat.Entry(e.ID)
		if !ok {
			k = catalog.KindAccessory
		}
		refs := cat.ItemRefs(e.ID)
		if err := c.Remove(k, e.ID); err != nil {
			return freed, err
		}
		still := accessories.StoreRefs(append(setups.Folders(list), catalog.For(h.Root).Root)...) // its files go unless something else still refers to them
		store := workspace.Dir(h.Root).Store()
		for _, r := range refs {
			if !still[r] {
				n := size(store.Path(r))
				if os.Remove(store.Path(r)) == nil {
					freed += n
				}
			}
		}
	case UnusedSharedFiles:
		if assets.IsAsset(e.ID) { // listed just now as referred to by nothing
			if err := os.Remove(workspace.Dir(h.Root).Store().Path(e.ID)); err != nil && !os.IsNotExist(err) {
				return freed, err
			}
			freed += e.Size
		}
	}
	return freed, nil
}

// storeFileType names a store file's kind for the list ("Image", "3D model", …).
func storeFileType(rel string) string {
	switch strings.ToLower(path.Ext(rel)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp":
		return "Image"
	case ".obj", ".glb", ".gltf", ".fbx":
		return "3D model"
	case ".mtl":
		return "Model material"
	}
	return "File"
}

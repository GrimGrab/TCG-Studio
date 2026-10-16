// Package share writes and reads the files players send each other: item files (.tcgitems: any sets, accessories,
// furniture pieces and decorations) and setup files (.tcgsetup, version 3: a setup's info and game settings plus every
// item). Both carry items the same way, through the content registry (internal/content): items/<kind>/<id>/ holds what
// that kind's Storage.Export wrote. So any kind the registry knows is shared without code here.
//
// Rules (user decisions 2026-10-16): an import never overwrites or merges — an id this workspace already uses (any
// setup, the catalog, a set's shared folder) gets a new random id (internal/ids); the person importing an item file
// decides its source (origin), the file only suggests one; card art travels with its set; content converted from another
// mod stays on the PC it was converted on (its art belongs to that mod's authors).
package share

import (
	"archive/zip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tcgstudio/internal/catalog"
	"tcgstudio/internal/content"
	"tcgstudio/internal/ids"
	"tcgstudio/internal/origin"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
	"tcgstudio/internal/ziputil"
)

const (
	// ItemsExt is the extension of an item file.
	ItemsExt     = ".tcgitems"
	itemsFormat  = "tcgitems"
	ItemsVersion = 1
	// SetupVersion is the setup-file format this package writes (setups.Import reads versions 1 and 2).
	SetupVersion = 3
	manifestFile = "manifest.json"
	itemsDir     = "items/"
)

// Manifest is manifest.json of an item or setup file. Setup files keep the fields older Studio versions show.
type Manifest struct {
	Format        string    `json:"format"`
	FormatVersion int       `json:"formatVersion"`
	StudioVersion string    `json:"studioVersion,omitempty"`
	Exported      time.Time `json:"exported"`
	Items         []Item    `json:"items"`
	// Setup files only.
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Sets        int    `json:"sets"`
	Accessories int    `json:"accessories,omitempty"`
	Furniture   int    `json:"furniture,omitempty"`
	Decorations int    `json:"decorations,omitempty"`
}

// Item is one item in a file.
type Item struct {
	Kind   string         `json:"kind"`
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Sub    string         `json:"sub,omitempty"`
	Origin *origin.Origin `json:"origin,omitempty"` // where it came from on the sender's PC (a suggestion for item files)
}

// Pick names an item by kind and id.
type Pick struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

func prefix(kind, id string) string { return itemsDir + kind + "/" + id + "/" }

// located is an item found in a root, ready to export.
type located struct {
	kind content.Kind
	root string
	item content.Item
}

// exportItems writes the items into z and returns their manifest entries. progress gets (done, total).
func exportItems(ws workspace.Dir, list []located, z *ziputil.Writer, progress func(done, total int)) ([]Item, error) {
	var out []Item
	for i, l := range list {
		if err := l.kind.Store.Export(ws, l.root, l.item.ID, prefix(l.kind.ID, l.item.ID), z); err != nil {
			return nil, fmt.Errorf("%s: %w", l.item.Name, err)
		}
		out = append(out, Item{Kind: l.kind.ID, ID: l.item.ID, Name: l.item.Name, Sub: l.item.Sub, Origin: l.item.Origin})
		if progress != nil {
			progress(i+1, len(list))
		}
	}
	return out, nil
}

// ExportItems writes the picked items (each read from where the catalog finds it: a template or the setup that has it)
// to dest. Converted content is left out and named in skipped; nothing exportable is an error.
func ExportItems(h setups.Home, activeRoot string, picks []Pick, dest, studioVersion string, progress func(done, total int)) (skipped []string, err error) {
	ws := workspace.Dir(h.Root)
	c := catalog.For(h.Root)
	var list []located
	byKind := map[string][]catalog.Entry{}
	for _, p := range picks {
		k, ok := content.Get(p.Kind)
		if !ok {
			return nil, fmt.Errorf("unknown kind %q", p.Kind)
		}
		if _, done := byKind[p.Kind]; !done {
			if byKind[p.Kind], err = c.All(h, activeRoot, p.Kind); err != nil {
				return nil, err
			}
		}
		e, ok := findEntry(byKind[p.Kind], p.ID)
		if !ok {
			return nil, fmt.Errorf("%s %s not found", k.One, p.ID)
		}
		if e.Converted {
			skipped = append(skipped, e.Name)
			continue
		}
		it, err := itemIn(ws, k, e.FromRoot(), p.ID)
		if err != nil {
			return nil, err
		}
		list = append(list, located{k, e.FromRoot(), it})
	}
	if len(list) == 0 {
		return skipped, errors.New("nothing to export: content converted from another mod stays on this PC (its art belongs to the mod's authors)")
	}
	z, err := ziputil.Create(dest)
	if err != nil {
		return nil, err
	}
	items, err := exportItems(ws, list, z, progress)
	if err == nil {
		err = z.JSON(manifestFile, Manifest{Format: itemsFormat, FormatVersion: ItemsVersion, StudioVersion: studioVersion,
			Exported: time.Now().UTC(), Items: items})
	}
	if err != nil {
		z.Abort()
		return nil, err
	}
	return skipped, z.Close()
}

func findEntry(list []catalog.Entry, id string) (catalog.Entry, bool) {
	for _, e := range list {
		if e.ID == id {
			return e, true
		}
	}
	return catalog.Entry{}, false
}

// itemIn reads one item of a root (with its full origin).
func itemIn(ws workspace.Dir, k content.Kind, root, id string) (content.Item, error) {
	items, err := k.Store.List(ws, root)
	if err != nil {
		return content.Item{}, err
	}
	for _, it := range items {
		if it.ID == id {
			return it, nil
		}
	}
	return content.Item{}, fmt.Errorf("%s %s not found", k.One, id)
}

// readManifest opens a share file and reads its manifest (format checked by the caller).
func readManifest(src string) (*zip.ReadCloser, *Manifest, error) {
	zr, err := ziputil.Open(src)
	if err != nil {
		return nil, nil, fmt.Errorf("not a TCG Studio file: %w", err)
	}
	m := &Manifest{}
	found, err := ziputil.ReadJSON(&zr.Reader, manifestFile, m)
	if err != nil || !found {
		zr.Close()
		if err == nil {
			err = errors.New("no manifest")
		}
		return nil, nil, fmt.Errorf("not a TCG Studio file (%v)", err)
	}
	return zr, m, nil
}

// checkItems refuses items this Studio can't read (kinds added by a newer version).
func checkItems(m *Manifest) error {
	var unknown []string
	for _, it := range m.Items {
		if _, ok := content.Get(it.Kind); !ok {
			unknown = append(unknown, it.Kind)
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("this file has content this TCG Studio doesn't know (%s) — it was made with TCG Studio %s; update first",
			strings.Join(unknown, ", "), m.StudioVersion)
	}
	return nil
}

// usedIDs is every id the workspace uses, of any kind, in the catalog and every setup (ids of library entries share one
// namespace, so all kinds are checked together).
func usedIDs(h setups.Home) (func(id string) bool, error) {
	ws := workspace.Dir(h.Root)
	list, err := h.List()
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	for _, root := range append(setups.Folders(list), catalog.For(h.Root).Root) {
		for _, k := range content.Kinds {
			items, _ := k.Store.List(ws, root)
			for _, it := range items {
				used[it.ID] = true
			}
		}
	}
	return func(id string) bool {
		if used[id] {
			return true
		}
		for _, k := range content.Kinds {
			if k.Store.Taken(ws, id) {
				return true
			}
		}
		return false
	}, nil
}

// newIDs gives each item its id on this PC: its own when free, else a new random one (also free of the others).
func newIDs(h setups.Home, items []Item) (map[Pick]string, error) {
	taken, err := usedIDs(h)
	if err != nil {
		return nil, err
	}
	claimed := map[string]bool{}
	isTaken := func(id string) bool { return claimed[id] || taken(id) }
	out := map[Pick]string{}
	for _, it := range items {
		id := it.ID
		if isTaken(id) {
			id = ids.Free(isTaken)
		}
		claimed[id] = true
		out[Pick{it.Kind, it.ID}] = id
	}
	return out, nil
}

// unpack extracts the given items' folders of zr into dir; returns each item's folder.
func unpack(zr *zip.Reader, dir string, items []Item) (map[Pick]string, error) {
	want := map[string]Pick{}
	out := map[Pick]string{}
	for _, it := range items {
		p := Pick{it.Kind, it.ID}
		want[prefix(it.Kind, it.ID)] = p
		out[p] = filepath.Join(dir, filepath.FromSlash(prefix(it.Kind, it.ID)))
	}
	err := ziputil.ExtractAll(zr, dir, func(name string) bool {
		if !strings.HasPrefix(name, itemsDir) {
			return false
		}
		rest := strings.TrimPrefix(name, itemsDir)
		kind, rest, ok1 := strings.Cut(rest, "/")
		id, _, ok2 := strings.Cut(rest, "/")
		_, ok := want[itemsDir+kind+"/"+id+"/"]
		return ok1 && ok2 && ok
	})
	return out, err
}

// importInto imports unpacked items into root, kind by kind in registry order (each kind keeps the file's order).
// originOf gives each item's origin.
func importInto(ws workspace.Dir, root string, items []Item, dirs, newID map[Pick]string, originOf func(Item, string) *origin.Origin) error {
	for _, k := range content.Kinds {
		var in []content.In
		for _, it := range items {
			if it.Kind != k.ID {
				continue
			}
			p := Pick{it.Kind, it.ID}
			in = append(in, content.In{Dir: dirs[p], NewID: newID[p], Origin: originOf(it, newID[p])})
		}
		if len(in) > 0 {
			if err := k.Store.Import(ws, root, in); err != nil {
				return err
			}
		}
	}
	return nil
}

func tempDir(base string) (string, func(), error) {
	dir := filepath.Join(base, fmt.Sprintf(".importing-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", nil, err
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

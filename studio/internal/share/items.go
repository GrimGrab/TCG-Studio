package share

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"tcgstudio/internal/catalog"
	"tcgstudio/internal/content"
	"tcgstudio/internal/origin"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

// Preview is an item file as the Import page shows it before importing.
type Preview struct {
	File   string        `json:"file"`
	Items  []PreviewItem `json:"items"`
	Source Source        `json:"source"` // suggested source (the person importing decides)
	Studio string        `json:"studio"` // the TCG Studio version that wrote the file
}

// PreviewItem is one item of the file and the id it gets here.
type PreviewItem struct {
	Item
	NewID string `json:"newId"` // == ID unless this workspace already uses ID
}

// Source is where imported items say they came from: written as their origin (empty Name = your own, no origin).
type Source struct {
	Name   string `json:"name"`
	Author string `json:"author"`
	Link   string `json:"link"`
}

// PreviewItems reads an item file: its items, the ids they would get and a suggested source (the origin the items
// carried, else the file's name).
func PreviewItems(h setups.Home, src string) (*Preview, error) {
	zr, m, err := readManifest(src)
	if err != nil {
		return nil, err
	}
	zr.Close()
	if err := checkItemsFile(m); err != nil {
		return nil, err
	}
	ids, err := newIDs(h, m.Items)
	if err != nil {
		return nil, err
	}
	pv := &Preview{File: src, Studio: m.StudioVersion, Items: []PreviewItem{}}
	for _, it := range m.Items {
		pv.Items = append(pv.Items, PreviewItem{Item: it, NewID: ids[Pick{it.Kind, it.ID}]})
		if o := it.Origin; o != nil && o.Mod != "" && pv.Source.Name == "" {
			pv.Source = Source{Name: o.Mod, Author: o.Author, Link: o.Link}
		}
	}
	if pv.Source.Name == "" {
		pv.Source.Name = strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	}
	return pv, nil
}

func checkItemsFile(m *Manifest) error {
	switch {
	case m.Format != itemsFormat:
		return fmt.Errorf("not a TCG Studio item file")
	case m.FormatVersion > ItemsVersion:
		return fmt.Errorf("this file was made with a newer TCG Studio (%s) — update TCG Studio first", m.StudioVersion)
	}
	return checkItems(m)
}

// Selection is what the person importing chose: which items, their source, and whether to add them to the active setup.
type Selection struct {
	Picks   []Pick `json:"picks"` // by the ids in the file; empty = all
	Source  Source `json:"source"`
	AddHere bool   `json:"addHere"`
}

// Imported is one imported item.
type Imported struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"` // its id on this PC
	Name   string `json:"name"`
	OrigID string `json:"origId,omitempty"` // its id in the file, when it had to change
}

// Result reports an import.
type Result struct {
	Items []Imported `json:"items"`
	Added int        `json:"added"` // added to the active setup
}

// ImportItems imports the chosen items of an item file into the catalog (their files into shared storage) and, when
// sel.AddHere, adds them to the active setup (sets are added, not installed). Ids this workspace uses get new ones.
func ImportItems(h setups.Home, activeRoot, src string, sel Selection, progress func(string)) (*Result, error) {
	zr, m, err := readManifest(src)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	if err := checkItemsFile(m); err != nil {
		return nil, err
	}
	want := map[Pick]bool{}
	for _, p := range sel.Picks {
		want[p] = true
	}
	var items []Item
	for _, it := range m.Items {
		if len(want) == 0 || want[Pick{it.Kind, it.ID}] {
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("nothing picked to import")
	}
	ids, err := newIDs(h, items)
	if err != nil {
		return nil, err
	}
	c := catalog.For(h.Root)
	tmp, done, err := tempDir(c.Root)
	if err != nil {
		return nil, err
	}
	defer done()
	report(progress, "Unpacking…")
	dirs, err := unpack(&zr.Reader, tmp, items)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	originOf := func(it Item, newID string) *origin.Origin {
		s := sel.Source
		if strings.TrimSpace(s.Name) == "" {
			return nil // your own
		}
		o := &origin.Origin{Kind: origin.KindShared, Mod: strings.TrimSpace(s.Name), Author: strings.TrimSpace(s.Author),
			Link: strings.TrimSpace(s.Link), Package: filepath.Base(src), Item: it.Name, Imported: now}
		if newID != it.ID {
			o.OrigID = it.ID
		}
		return o
	}
	report(progress, "Adding to the catalog…")
	if err := importInto(workspace.Dir(h.Root), c.Root, items, dirs, ids, originOf); err != nil {
		return nil, err
	}
	res := &Result{Items: []Imported{}}
	byKind := map[string][]string{}
	for _, it := range items {
		id := ids[Pick{it.Kind, it.ID}]
		im := Imported{Kind: it.Kind, ID: id, Name: it.Name}
		if id != it.ID {
			im.OrigID = it.ID
		}
		res.Items = append(res.Items, im)
		byKind[it.Kind] = append(byKind[it.Kind], id)
	}
	if sel.AddHere {
		report(progress, "Adding to this setup…")
		for _, k := range content.Kinds {
			if len(byKind[k.ID]) == 0 {
				continue
			}
			r, err := c.Add(h, activeRoot, k.ID, byKind[k.ID])
			if err != nil {
				return res, err
			}
			res.Added += len(r.Added)
		}
	}
	return res, nil
}

func report(progress func(string), msg string) {
	if progress != nil {
		progress(msg)
	}
}

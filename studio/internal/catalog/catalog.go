// Package catalog is the workspace's shared catalog: a starting template of every set, accessory, furniture piece and
// decoration that entered Studio, so any setup can add it without importing again. The files themselves are already
// shared (card art in <workspace>\library, item files in <workspace>\assets); a template is only the game info (set.json +
// studio.json, library entries + editor layouts + origins). Adding copies that game info into a setup, which owns it
// from then on: names, rarities, tiers, prices, costs, license levels and ladder order are per setup and editing never
// touches the catalog or other setups.
//
// On disk the catalog is laid out like a setup — <workspace>\catalog\projects\<setId>\ and <workspace>\catalog\accessories\ —
// and every function works on any kind of the content registry (internal/content), never through a switch on the kind.
package catalog

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/content"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

// DirName is the catalog folder in the workspace.
const DirName = "catalog"

// Kinds of catalog entries, as named in the content registry.
const (
	KindSet        = "set"
	KindAccessory  = accessories.KindAccessory
	KindFurniture  = accessories.KindFurniture
	KindDecoration = accessories.KindDecoration
)

type Catalog struct {
	Root      string // <workspace>\catalog
	Workspace string
}

func For(workspace string) Catalog {
	return Catalog{Root: filepath.Join(workspace, DirName), Workspace: workspace}
}

// Sets is the catalog's set templates as a project workspace (card art in the shared library).
func (c Catalog) Sets() project.Workspace { return c.ws().Projects(c.Root) }

// Items opens the catalog's accessory-library templates.
func (c Catalog) Items() (*accessories.Library, error) { return c.ws().Items(c.Root) }

func (c Catalog) ws() workspace.Dir { return workspace.Dir(c.Workspace) }

func kind(id string) (content.Kind, error) {
	k, ok := content.Get(id)
	if !ok {
		return k, fmt.Errorf("unknown kind %q", id)
	}
	return k, nil
}

// Save stores (or replaces) the templates of items of root (a setup folder): their game info (their files are shared
// already).
func (c Catalog) Save(kindID, root string, ids []string) error {
	k, err := kind(kindID)
	if err != nil {
		return err
	}
	_, err = k.Store.Copy(c.ws(), root, c.Root, ids)
	return err
}

// SaveSet stores (or replaces) a set's template.
func (c Catalog) SaveSet(p *project.Project) error {
	return c.Save(KindSet, filepath.Dir(filepath.Dir(p.Folder)), []string{p.ID})
}

// SaveItems stores (or replaces) the templates of accessory-library entries of any kind (by id) of src's setup.
func (c Catalog) SaveItems(src *accessories.Library, ids []string) error {
	root := filepath.Dir(src.Folder)
	for _, k := range content.Kinds {
		if k.ID == KindSet {
			continue
		}
		if _, err := k.Store.Copy(c.ws(), root, c.Root, ids); err != nil {
			return err
		}
	}
	return nil
}

// Entry is one item as the catalog lists it.
type Entry struct {
	Kind       string     `json:"kind"` // a content kind (set | accessory | furniture | decoration)
	Key        string     `json:"key"`  // what "Add" and "Delete" take (the id)
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Sub        string     `json:"sub"`        // accessory kind (Playmat…), furniture type, or "" for sets
	Detail     string     `json:"detail"`     // "983 cards · 1 pack", …
	Origin     string     `json:"origin"`     // where it came from: the mod it was converted from, or a shared file's source
	OriginKind string     `json:"originKind"` // origin.KindEPL / origin.KindShared ("" = your own)
	Author     string     `json:"author"`
	Source     string     `json:"source"`    // sets: the import source (scryfall, tcgdex…; "manual" = made in Studio)
	Converted  bool       `json:"converted"` // converted from another mod: never exported
	Saved      bool       `json:"saved"`     // the catalog holds a template (else it's only in another setup)
	From       string     `json:"from"`      // where adding copies it from: "catalog" or the setup's name
	fromRoot   string     // that folder
	Here       bool       `json:"here"`   // the active setup already has this id
	UsedBy     []SetupRef `json:"usedBy"` // the setups that have it
	Cover      string     `json:"cover"`  // sets: a card image path (relative to the set) for the thumbnail
	Icon       string     `json:"icon"`   // library entries: shop icon path (accessories.Library relative)
	Base       string     `json:"base"`   // library entries: the vanilla item it is built on ("" = the kind's default)
}

// FromRoot is the folder the entry is read from (the catalog, or the setup that has it).
func (e Entry) FromRoot() string { return e.fromRoot }

// SetupRef names a setup.
type SetupRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// List returns the catalog's templates of a kind plus the items only other setups have (they can be added too; adding
// templates them). activeRoot is the active setup's folder.
func (c Catalog) List(h setups.Home, activeRoot, kind string) ([]Entry, error) {
	return c.collect(h, activeRoot, kind, false)
}

// All returns everything of a kind that exists anywhere — templates, every setup's (the active one included) — once each,
// with the setups using it. The Catalog page.
func (c Catalog) All(h setups.Home, activeRoot, kind string) ([]Entry, error) {
	return c.collect(h, activeRoot, kind, true)
}

func (c Catalog) collect(h setups.Home, activeRoot, kindID string, withActive bool) ([]Entry, error) {
	k, err := kind(kindID)
	if err != nil {
		return nil, err
	}
	list, err := h.List()
	if err != nil {
		return nil, err
	}
	have := map[string]bool{} // ids in the active setup
	if items, err := k.Store.List(c.ws(), activeRoot); err == nil {
		for _, it := range items {
			have[it.ID] = true
		}
	}
	usedBy := map[string][]SetupRef{} // id → setups having it
	for _, s := range list {
		items, _ := k.Store.List(c.ws(), s.Folder)
		for _, it := range items {
			usedBy[it.ID] = append(usedBy[it.ID], SetupRef{ID: s.ID, Name: s.Name})
		}
	}
	// Templates first, then the setups (the active one only with withActive: its items are "here" already).
	type source struct {
		root, name string
		saved      bool
	}
	sources := []source{{c.Root, "catalog", true}}
	for _, s := range list {
		if withActive || filepath.Clean(s.Folder) != filepath.Clean(activeRoot) {
			sources = append(sources, source{s.Folder, s.Name, false})
		}
	}
	seen := map[string]int{} // id → index in out
	var out []Entry
	for _, src := range sources {
		items, err := k.Store.List(c.ws(), src.root)
		if err != nil {
			continue
		}
		for _, it := range items {
			if i, ok := seen[it.ID]; ok {
				// A template saved before its picture was made (a furniture icon is rendered on save): show a setup's.
				if out[i].Icon == "" {
					out[i].Icon = it.Icon
				}
				if out[i].Cover == "" {
					out[i].Cover = it.Cover
				}
				continue
			}
			seen[it.ID] = len(out)
			e := Entry{Kind: kindID, Key: it.ID, ID: it.ID, Name: it.Name, Sub: it.Sub, Detail: it.Detail, Source: it.Source,
				Converted: it.Converted, Saved: src.saved, From: src.name, fromRoot: src.root, Cover: it.Cover, Icon: it.Icon,
				Base: it.Base, Here: have[it.ID], UsedBy: usedBy[it.ID]}
			if o := it.Origin; o != nil {
				e.Origin, e.OriginKind, e.Author = o.Name(), o.Kind, o.Author
			}
			if e.UsedBy == nil {
				e.UsedBy = []SetupRef{}
			}
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Origin != out[j].Origin {
			return out[i].Origin < out[j].Origin
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// AddResult reports an add: what was added and what the setup already had.
type AddResult struct {
	Added   []string `json:"added"`
	Skipped []string `json:"skipped"` // already in this setup
}

// Add copies templates (or another setup's copy, which is templated on the way) of a kind into the active setup by id,
// on disk. Sets are added, not installed; library entries are appended (last on the Gamify ladder).
func (c Catalog) Add(h setups.Home, activeRoot, kindID string, ids []string) (*AddResult, error) {
	k, err := kind(kindID)
	if err != nil {
		return nil, err
	}
	entries, err := c.List(h, activeRoot, kindID)
	if err != nil {
		return nil, err
	}
	res := &AddResult{Added: []string{}, Skipped: []string{}}
	for _, key := range ids {
		e, ok := find(entries, key)
		switch {
		case !ok:
			return res, fmt.Errorf("%s is not in the catalog", key)
		case e.Here:
			res.Skipped = append(res.Skipped, e.ID)
			continue
		}
		if !e.Saved { // only in another setup: template it so it stays available
			if _, err := k.Store.Copy(c.ws(), e.fromRoot, c.Root, []string{e.ID}); err != nil {
				return res, err
			}
		}
		done, err := k.Store.Copy(c.ws(), c.Root, activeRoot, []string{e.ID})
		if err != nil {
			return res, err
		}
		res.Added = append(res.Added, done...)
	}
	return res, nil
}

// Remove deletes a template (setups that have the item keep their copy).
func (c Catalog) Remove(kindID, id string) error {
	if !setfmt.SafeID(id) {
		return fmt.Errorf("bad id %q", id)
	}
	k, err := kind(kindID)
	if err != nil {
		return err
	}
	_, _, err = k.Store.Remove(c.ws(), c.Root, []string{id})
	return err
}

// SetIDs lists the catalog's set templates (the Storage page keeps their card art).
func (c Catalog) SetIDs() []string { return c.IDs(KindSet, c.Root) }

// IDs lists the ids of a kind in a root.
func (c Catalog) IDs(kindID, root string) []string {
	k, err := kind(kindID)
	if err != nil {
		return nil
	}
	items, _ := k.Store.List(c.ws(), root)
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func find(list []Entry, key string) (Entry, bool) {
	for _, e := range list {
		if e.Key == key {
			return e, true
		}
	}
	return Entry{}, false
}

// KeepSetup templates everything a setup has that the catalog doesn't hold yet (before the setup is deleted, so its
// content stays available to the other setups). Existing templates are left as they are.
func (c Catalog) KeepSetup(dir string) error {
	for _, k := range content.Kinds {
		if err := c.Keep(k.ID, dir, c.IDs(k.ID, dir)); err != nil {
			return err
		}
	}
	return nil
}

// Keep templates items of root the catalog doesn't hold yet; existing templates stay as they are.
func (c Catalog) Keep(kindID, root string, ids []string) error {
	saved := map[string]bool{}
	for _, id := range c.IDs(kindID, c.Root) {
		saved[id] = true
	}
	var missing []string
	for _, id := range ids {
		if !saved[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return c.Save(kindID, root, missing)
}

// KeepSets templates the sets (of a setup's workspace) the catalog doesn't hold yet.
func (c Catalog) KeepSets(ws project.Workspace, ids []string) error {
	return c.Keep(KindSet, ws.Root, ids)
}

// KeepItems templates the accessory-library entries (any kind) of src's setup the catalog doesn't hold yet.
func (c Catalog) KeepItems(src *accessories.Library, ids []string) error {
	root := filepath.Dir(src.Folder)
	for _, k := range content.Kinds {
		if k.ID != KindSet {
			if err := c.Keep(k.ID, root, ids); err != nil {
				return err
			}
		}
	}
	return nil
}

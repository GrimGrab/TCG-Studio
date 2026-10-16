// Package catalog is the workspace's shared catalog: a starting template of every set, accessory and furniture piece that
// entered Studio, so any setup can add it without importing again. The files themselves are already shared (card art in
// <workspace>\library, accessory/furniture files in <workspace>\assets); a template is only the game info (set.json +
// studio.json, accessory/furniture entries + editor layouts + origins) and a set's own small files (pack/box art, card
// back). Adding copies that game info into a setup, which owns it from then on: names, rarities, tiers, prices, costs,
// license levels and ladder order are per setup and editing never touches the catalog or other setups.
//
// On disk the catalog is laid out like a setup — <workspace>\catalog\projects\<setId>\ and <workspace>\catalog\accessories\ —
// so project.Workspace and accessories.Library work on it unchanged.
package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/origin"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

// DirName is the catalog folder in the workspace.
const DirName = "catalog"

// Kinds of catalog entries.
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
func (c Catalog) Sets() project.Workspace {
	return c.ws().Projects(c.Root)
}

// Items opens the catalog's accessory and furniture templates.
func (c Catalog) Items() (*accessories.Library, error) {
	return c.ws().Items(c.Root)
}

// SaveSet stores (or replaces) a set's template: a copy of its project folder (set.json, studio.json; the set's files
// stay in its shared folder).
func (c Catalog) SaveSet(p *project.Project) error {
	dst := c.Sets().Folder(p.ID)
	tmp := dst + ".saving"
	_ = os.RemoveAll(tmp)
	if err := fsutil.CopyTree(p.Folder, tmp, nil); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	_ = os.RemoveAll(dst)
	return os.Rename(tmp, dst)
}

// SaveItems stores (or replaces) the templates of accessories and furniture of a library (by id): entries, editor layouts
// and origins (their files are shared store files already).
func (c Catalog) SaveItems(src *accessories.Library, ids []string) error {
	cat, err := c.Items()
	if err != nil {
		return err
	}
	for _, id := range ids {
		cat.CopyEntry(src, id)
	}
	return cat.Save()
}

// Entry is one catalog item as the "Add from catalog" picker lists it.
type Entry struct {
	Kind     string     `json:"kind"` // set | accessory | furniture
	Key      string     `json:"key"`  // what "Add" and "Delete" take (the id)
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Sub      string     `json:"sub"`    // accessory kind (Playmat…), furniture type, or "" for sets
	Detail   string     `json:"detail"` // "983 cards · 1 pack", …
	Origin   string     `json:"origin"` // converted content: the mod it came from
	Author   string     `json:"author"`
	Source   string     `json:"source"` // sets: the import source (scryfall, tcgdex…; "" = made in Studio)
	Saved    bool       `json:"saved"`  // the catalog holds a template (else it's only in another setup)
	From     string     `json:"from"`   // where adding copies it from: "catalog" or the setup's name
	fromRoot string     // that setup's folder (unsaved entries)
	Here     bool       `json:"here"`   // the active setup already has this id
	UsedBy   []SetupRef `json:"usedBy"` // the setups that have it
	Cover    string     `json:"cover"`  // sets: a card image path (relative to the set) for the thumbnail
	Icon     string     `json:"icon"`   // accessories/furniture: shop icon path (accessories.Library relative)
}

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

func (c Catalog) collect(h setups.Home, activeRoot, kind string, withActive bool) ([]Entry, error) {
	list, err := h.List()
	if err != nil {
		return nil, err
	}
	have := map[string]bool{} // ids in the active setup
	switch kind {
	case KindSet:
		for _, id := range projectIDs(project.Workspace{Root: activeRoot}) {
			have[id] = true
		}
	default:
		if l, err := c.ws().Items(activeRoot); err == nil {
			for _, id := range itemIDs(l, kind) {
				have[id] = true
			}
		}
	}
	usedBy := map[string][]SetupRef{} // key → setups having it
	for _, s := range list {
		ref := SetupRef{ID: s.ID, Name: s.Name}
		if kind == KindSet {
			ws := c.ws().Projects(s.Folder)
			for _, id := range projectIDs(ws) {
				if p, err := ws.Load(id); err == nil {
					usedBy[p.ID] = append(usedBy[p.ID], ref)
				}
			}
		} else if l, err := c.ws().Items(s.Folder); err == nil {
			for _, id := range itemIDs(l, kind) {
				usedBy[id] = append(usedBy[id], ref)
			}
		}
	}
	seen := map[string]bool{}
	var out []Entry
	add := func(e Entry) {
		if e.Key == "" {
			e.Key = e.ID
		}
		if seen[e.Key] {
			return
		}
		seen[e.Key] = true
		e.Here = have[e.ID]
		e.UsedBy = usedBy[e.Key]
		if e.UsedBy == nil {
			e.UsedBy = []SetupRef{}
		}
		out = append(out, e)
	}
	// Templates first, then other setups (active setup excluded: its items are "here" already).
	sources := []struct {
		root, name string
		saved      bool
	}{{c.Root, "catalog", true}}
	for _, s := range list {
		if withActive || filepath.Clean(s.Folder) != filepath.Clean(activeRoot) {
			sources = append(sources, struct {
				root, name string
				saved      bool
			}{s.Folder, s.Name, false})
		}
	}
	for _, src := range sources {
		switch kind {
		case KindSet:
			ws := c.ws().Projects(src.root)
			for _, id := range projectIDs(ws) {
				p, err := ws.Load(id)
				if err != nil {
					continue
				}
				e := Entry{Kind: kind, ID: id, Name: p.Set.Name, Source: p.Meta.Source, Saved: src.saved, From: src.name, fromRoot: src.root,
					Detail: fmt.Sprintf("%d cards · %d pack%s", len(p.Set.Cards), len(p.Set.Packs), plural(len(p.Set.Packs)))}
				if o := p.Meta.Origin; o != nil {
					e.Origin, e.Author = o.Mod, o.Author
				} else if n := p.Meta.OriginName(); n != "" {
					e.Origin = n
				}
				for _, c := range p.Set.Cards {
					if c.Image != "" {
						e.Cover = c.Image
						break
					}
				}
				add(e)
			}
		default:
			l, err := c.ws().Items(src.root)
			if err != nil {
				continue
			}
			for _, a := range l.Lib.Accessories {
				if kind == KindAccessory {
					e := itemEntry(kind, a.ID, a.Name, a.Kind, l.Meta.Origins, src.saved, src.name, src.root)
					e.Icon = a.Icon
					add(e)
				}
			}
			for _, f := range l.Lib.Furniture {
				if kind == KindFurniture {
					e := itemEntry(kind, f.ID, f.Name, f.Type, l.Meta.Origins, src.saved, src.name, src.root)
					e.Icon = f.Icon
					add(e)
				}
			}
			for _, d := range l.Lib.Decorations {
				if kind == KindDecoration {
					e := itemEntry(kind, d.ID, d.Name, d.Kind, l.Meta.Origins, src.saved, src.name, src.root)
					e.Icon = d.Icon
					add(e)
				}
			}
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

func itemEntry(kind, id, name, sub string, origins map[string]origin.Origin, saved bool, from, root string) Entry {
	e := Entry{Kind: kind, ID: id, Name: name, Sub: sub, Detail: sub, Saved: saved, From: from, fromRoot: root}
	if o, ok := origins[id]; ok {
		e.Origin, e.Author = o.Name(), o.Author
	}
	return e
}

// AddResult reports an add: what was added and what the setup already had.
type AddResult struct {
	Added   []string `json:"added"`
	Skipped []string `json:"skipped"` // already in this setup
}

// AddSets copies set templates (or another setup's copy, which is templated on the way) into the active setup (by id).
func (c Catalog) AddSets(h setups.Home, activeRoot string, ids []string) (*AddResult, error) {
	entries, err := c.List(h, activeRoot, KindSet)
	if err != nil {
		return nil, err
	}
	dst := c.ws().Projects(activeRoot)
	res := &AddResult{Added: []string{}, Skipped: []string{}}
	for _, key := range ids {
		e, ok := find(entries, key)
		id := e.ID
		switch {
		case !ok:
			return res, fmt.Errorf("%s is not in the catalog", key)
		case e.Here:
			res.Skipped = append(res.Skipped, id)
			continue
		}
		srcWS := c.ws().Projects(e.fromRoot)
		p, err := srcWS.Load(id)
		if err != nil {
			return res, err
		}
		if !e.Saved { // only in another setup: template it so it stays available
			if err := c.SaveSet(p); err != nil {
				return res, err
			}
		}
		to := dst.Folder(id)
		if err := fsutil.CopyTree(p.Folder, to+".adding", nil); err != nil {
			_ = os.RemoveAll(to + ".adding")
			return res, err
		}
		if err := os.Rename(to+".adding", to); err != nil {
			_ = os.RemoveAll(to + ".adding")
			return res, err
		}
		res.Added = append(res.Added, id)
	}
	return res, nil
}

// AddItems copies accessory or furniture templates (or another setup's, templated on the way) into a setup's library
// (appended, so they come last on the Gamify ladder). The caller saves and installs the library.
func (c Catalog) AddItems(h setups.Home, activeRoot, kind string, ids []string, dst *accessories.Library) (*AddResult, error) {
	entries, err := c.List(h, activeRoot, kind)
	if err != nil {
		return nil, err
	}
	res := &AddResult{Added: []string{}, Skipped: []string{}}
	libs := map[string]*accessories.Library{}
	for _, id := range ids {
		e, ok := find(entries, id)
		switch {
		case !ok:
			return res, fmt.Errorf("%s is not in the catalog", id)
		case e.Here:
			res.Skipped = append(res.Skipped, id)
			continue
		}
		src := libs[e.fromRoot]
		if src == nil {
			if src, err = c.ws().Items(e.fromRoot); err != nil {
				return res, err
			}
			libs[e.fromRoot] = src
		}
		if !e.Saved {
			if err := c.SaveItems(src, []string{id}); err != nil {
				return res, err
			}
			if src, err = c.Items(); err != nil { // take the templated copy (its files are in the store)
				return res, err
			}
			libs[e.fromRoot] = nil
		}
		if !dst.CopyEntry(src, id) {
			continue
		}
		res.Added = append(res.Added, id)
	}
	return res, nil
}

// Remove deletes a template (setups that have the item keep their copy).
func (c Catalog) Remove(kind, id string) error {
	if !setfmt.SafeID(id) {
		return fmt.Errorf("bad id %q", id)
	}
	if kind == KindSet {
		return c.Sets().Delete(id)
	}
	cat, err := c.Items()
	if err != nil {
		return err
	}
	if kind == KindFurniture {
		cat.DeleteFurniture(id)
	} else if kind == KindDecoration {
		cat.DeleteDecoration(id)
	} else {
		cat.Delete(id)
	}
	return cat.Save()
}

// SetIDs lists the catalog's set templates (the Storage page keeps their card art).
func (c Catalog) SetIDs() []string { return projectIDs(c.Sets()) }

func find(list []Entry, key string) (Entry, bool) {
	for _, e := range list {
		if e.Key == key {
			return e, true
		}
	}
	return Entry{}, false
}

func projectIDs(ws project.Workspace) []string {
	des, err := os.ReadDir(ws.ProjectsDir())
	if err != nil {
		return nil
	}
	var out []string
	for _, d := range des {
		if d.IsDir() && !strings.Contains(d.Name(), ".") {
			if _, err := os.Stat(filepath.Join(ws.ProjectsDir(), d.Name(), project.SetFile)); err == nil {
				out = append(out, d.Name())
			}
		}
	}
	return out
}

func itemIDs(l *accessories.Library, kind string) []string {
	var out []string
	if kind == KindFurniture {
		for _, f := range l.Lib.Furniture {
			out = append(out, f.ID)
		}
		return out
	}
	if kind == KindDecoration {
		for _, d := range l.Lib.Decorations {
			out = append(out, d.ID)
		}
		return out
	}
	for _, a := range l.Lib.Accessories {
		out = append(out, a.ID)
	}
	return out
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// KeepSetup templates everything a setup has that the catalog doesn't hold yet (before the setup is deleted, so its
// sets and items stay available to the other setups). Existing templates are left as they are.
func (c Catalog) KeepSetup(dir string) error {
	ws := c.ws().Projects(dir)
	if err := c.KeepSets(ws, projectIDs(ws)); err != nil {
		return err
	}
	src, err := c.ws().Items(dir)
	if err != nil {
		return nil // no library
	}
	return c.KeepItems(src, append(append(itemIDs(src, KindAccessory), itemIDs(src, KindFurniture)...), itemIDs(src, KindDecoration)...))
}

// KeepSets templates the sets (of a setup's workspace) the catalog doesn't hold yet; existing templates stay as they are.
func (c Catalog) KeepSets(ws project.Workspace, ids []string) error {
	saved := map[string]bool{}
	for _, id := range c.SetIDs() {
		saved[id] = true
	}
	for _, id := range ids {
		if saved[id] {
			continue
		}
		p, err := ws.Load(id)
		if err != nil {
			continue
		}
		if err := c.SaveSet(p); err != nil {
			return err
		}
	}
	return nil
}

// KeepItems templates the accessories/furniture (of a setup's library) the catalog doesn't hold yet.
func (c Catalog) KeepItems(src *accessories.Library, ids []string) error {
	cat, err := c.Items()
	if err != nil {
		return err
	}
	var missing []string
	for _, id := range ids {
		if !cat.HasItem(id) && src.HasItem(id) {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return c.SaveItems(src, missing)
}

func (c Catalog) ws() workspace.Dir { return workspace.Dir(c.Workspace) }

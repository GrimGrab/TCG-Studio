package main

import (
	"errors"
	"time"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/catalog"
	"tcgstudio/internal/content"
	"tcgstudio/internal/game"
	"tcgstudio/internal/setfmt"
)

// The shared catalog: a template of every set, accessory and furniture piece that entered Studio, so any setup can add
// it without importing again (files stay shared; the setup gets its own copy of the game info). See internal/catalog.

func (a *App) catalog() catalog.Catalog { return catalog.For(a.workspaceDir()) }

// CatalogList lists what the "Add from catalog" picker offers for a kind (set | accessory | furniture): the catalog's
// templates and what other setups have, each flagged when this setup already has it.
func (a *App) CatalogList(kind string) ([]catalog.Entry, error) {
	a.catalogMu.Lock()
	defer a.catalogMu.Unlock()
	list, err := a.catalog().List(a.home(), a.root(), kind)
	if list == nil {
		list = []catalog.Entry{}
	}
	return list, err
}

// inCatalog reports whether set id can be added from the catalog (a template, or another setup has it) while the active
// setup doesn't have it.
func (a *App) inCatalog(id string) bool {
	list, _ := a.CatalogList(catalog.KindSet)
	for _, e := range list {
		if e.ID == id && !e.Here {
			return true
		}
	}
	return false
}

// AddFromCatalog copies catalog items of a kind into the active setup. Sets are added (Install them on the Sets page as
// usual); library entries are added at the end of the library, which the game reads like any edit.
func (a *App) AddFromCatalog(kind string, ids []string) (*catalog.AddResult, error) {
	a.catalogMu.Lock()
	defer a.catalogMu.Unlock()
	res, err := a.catalog().Add(a.home(), a.root(), kind, ids)
	if err != nil || len(res.Added) == 0 || kind == catalog.KindSet {
		return res, err
	}
	l, err := a.items()
	if err != nil {
		return res, err
	}
	_, err = a.saveLibrary(l)
	return res, err
}

// ContentKinds is the content registry (sets, accessories, furniture, decorations…) for pages that list every kind.
func (a *App) ContentKinds() []content.Kind { return content.Kinds }

// SaveToCatalog replaces the catalog's templates of these items with this setup's current version (what other setups
// get when they add them from now on; setups that already have them keep their own).
func (a *App) SaveToCatalog(kind string, ids []string) error {
	a.catalogMu.Lock()
	defer a.catalogMu.Unlock()
	for _, id := range ids {
		if !setfmt.SafeID(id) {
			return errors.New("bad id " + id)
		}
	}
	return a.catalog().Save(kind, a.root(), ids)
}

// RemoveFromCatalog deletes templates. Setups that have the items keep them (and the picker still offers them from there).
func (a *App) RemoveFromCatalog(kind string, ids []string) error {
	a.catalogMu.Lock()
	defer a.catalogMu.Unlock()
	var errs []error
	for _, id := range ids {
		errs = append(errs, a.catalog().Remove(kind, id))
	}
	return errors.Join(errs...)
}

// keepInCatalog templates items about to be deleted from this setup when the catalog doesn't hold them yet, so they stay
// available to every setup (Remove from catalog forgets them). l = the setup's library (accessories/furniture), nil for sets.
func (a *App) keepInCatalog(kind string, ids []string, l *accessories.Library) error {
	a.catalogMu.Lock()
	defer a.catalogMu.Unlock()
	var err error
	if kind == catalog.KindSet {
		err = a.catalog().KeepSets(a.ws(), ids)
	} else {
		err = a.catalog().KeepItems(l, ids)
	}
	if err != nil {
		return errors.New("couldn't keep a copy in the shared catalog, so nothing was deleted: " + err.Error())
	}
	return nil
}

// catalogSet templates a set that just entered Studio (import). Failures only cost the template.
func (a *App) catalogSet(id string) {
	a.catalogMu.Lock()
	defer a.catalogMu.Unlock()
	if p, err := a.loadProject(id); err == nil {
		_ = a.catalog().SaveSet(p)
	}
}

// catalogItemsSince templates the accessories/furniture an import created or replaced (origin stamped at or after since).
func (a *App) catalogItemsSince(since time.Time) {
	a.catalogMu.Lock()
	defer a.catalogMu.Unlock()
	l, err := a.items()
	if err != nil {
		return
	}
	var ids []string
	for id, o := range l.Meta.Origins {
		if !o.Imported.Before(since) {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 {
		_ = a.catalog().SaveItems(l, ids)
	}
}

// catalogItems refreshes the templates of items the catalog already holds (e.g. after their icons were rendered).
func (a *App) catalogItems(ids []string) {
	a.catalogMu.Lock()
	defer a.catalogMu.Unlock()
	l, err := a.items()
	if err != nil {
		return
	}
	cat, err := a.catalog().Items()
	if err != nil {
		return
	}
	var have []string
	for _, id := range ids {
		if cat.HasItem(id) {
			have = append(have, id)
		}
	}
	if len(have) > 0 {
		_ = a.catalog().SaveItems(l, have)
	}
}

// CatalogAll lists everything of a kind that exists anywhere (the Catalog page), with the setups using each.
func (a *App) CatalogAll(kind string) ([]catalog.Entry, error) {
	a.catalogMu.Lock()
	defer a.catalogMu.Unlock()
	list, err := a.catalog().All(a.home(), a.root(), kind)
	if list == nil {
		list = []catalog.Entry{}
	}
	return list, err
}

// DeleteFromCatalog deletes sets (by key) or accessories/furniture (by id) everywhere: the catalog, every setup that has
// them (incl. their installed sets) and the shared files nothing uses any more. Nothing is written into the game folder,
// so the game may run: it drops them at its next start.
func (a *App) DeleteFromCatalog(kind string, keys []string) (*catalog.Deleted, error) {
	a.setups.mu.Lock()
	defer a.setups.mu.Unlock()
	gameOK := game.IsGameDir(a.gameDir())
	a.catalogMu.Lock()
	defer a.catalogMu.Unlock()
	// The active setup's sets that go: uninstalled from the game afterwards.
	activeSets := map[string]bool{}
	if kind == catalog.KindSet {
		for _, key := range keys {
			if _, err := a.loadProject(key); err == nil {
				activeSets[key] = true
			}
		}
	}
	res, err := a.catalog().DeleteEverywhere(a.home(), kind, keys)
	if err != nil || !gameOK {
		return res, err
	}
	if kind == catalog.KindSet {
		for id := range activeSets {
			_ = a.ws().Uninstall(id)
		}
		return res, nil
	}
	active := a.activeID()
	for _, s := range res.Setups {
		if s == active {
			if l, lerr := a.items(); lerr == nil {
				_, err = a.saveLibrary(l)
			}
		}
	}
	return res, err
}

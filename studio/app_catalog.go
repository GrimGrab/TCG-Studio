package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/catalog"
	"tcgstudio/internal/game"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

// The shared catalog: a template of every set, accessory and furniture piece that entered Studio, so any setup can add
// it without importing again (files stay shared; the setup gets its own copy of the game info). See internal/catalog.

var catalogMu sync.Mutex

func (a *App) catalog() catalog.Catalog { return catalog.For(a.settings.Workspace) }

// CatalogList lists what the "Add from catalog" picker offers for a kind (set | accessory | furniture): the catalog's
// templates and what other setups have, each flagged when this setup already has it.
func (a *App) CatalogList(kind string) ([]catalog.Entry, error) {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	list, err := a.catalog().List(a.home(), a.root(), kind)
	if list == nil {
		list = []catalog.Entry{}
	}
	return list, err
}

// AddFromCatalog copies catalog items into the active setup. Sets are added (Install them on the Sets page as usual);
// accessories and furniture are added at the end of the library, which is installed like any edit.
func (a *App) AddFromCatalog(kind string, ids []string) (*catalog.AddResult, error) {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	c := a.catalog()
	if kind == catalog.KindSet {
		return c.AddSets(a.home(), a.root(), ids)
	}
	l, err := a.accLib()
	if err != nil {
		return nil, err
	}
	res, err := c.AddItems(a.home(), a.root(), kind, ids, l)
	if err != nil {
		return res, err
	}
	if len(res.Added) > 0 {
		if _, err := a.saveAndInstall(l); err != nil {
			return res, err
		}
	}
	return res, nil
}

// SaveToCatalog replaces the catalog's templates of these items with this setup's current version (what other setups
// get when they add them from now on; setups that already have them keep their own).
func (a *App) SaveToCatalog(kind string, ids []string) error {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	c := a.catalog()
	if kind == catalog.KindSet {
		for _, id := range ids {
			p, err := a.ws().Load(id)
			if err != nil {
				return err
			}
			if err := c.SaveSet(p); err != nil {
				return err
			}
		}
		return nil
	}
	l, err := a.accLib()
	if err != nil {
		return err
	}
	return c.SaveItems(l, ids)
}

// RemoveFromCatalog deletes templates. Setups that have the items keep them (and the picker still offers them from there).
func (a *App) RemoveFromCatalog(kind string, ids []string) error {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	var errs []error
	for _, id := range ids {
		errs = append(errs, a.catalog().Remove(kind, id))
	}
	return errors.Join(errs...)
}

// keepInCatalog templates items about to be deleted from this setup when the catalog doesn't hold them yet, so they stay
// available to every setup (Remove from catalog forgets them). l = the setup's library (accessories/furniture), nil for sets.
func (a *App) keepInCatalog(kind string, ids []string, l *accessories.Library) error {
	catalogMu.Lock()
	defer catalogMu.Unlock()
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
	catalogMu.Lock()
	defer catalogMu.Unlock()
	if p, err := a.ws().Load(id); err == nil {
		_ = a.catalog().SaveSet(p)
	}
}

// catalogItemsSince templates the accessories/furniture an import created or replaced (origin stamped at or after since).
func (a *App) catalogItemsSince(since time.Time) {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	l, err := a.accLib()
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
	catalogMu.Lock()
	defer catalogMu.Unlock()
	l, err := a.accLib()
	if err != nil {
		return
	}
	cat, err := a.catalog().Items()
	if err != nil {
		return
	}
	var have []string
	for _, id := range ids {
		if cat.Index(id) >= 0 || cat.FurnitureIndex(id) >= 0 {
			have = append(have, id)
		}
	}
	if len(have) > 0 {
		_ = a.catalog().SaveItems(l, have)
	}
}

// CatalogAll lists everything of a kind that exists anywhere (the Catalog page), with the setups using each.
func (a *App) CatalogAll(kind string) ([]catalog.Entry, error) {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	list, err := a.catalog().All(a.home(), a.root(), kind)
	if list == nil {
		list = []catalog.Entry{}
	}
	return list, err
}

// DeleteFromCatalog deletes sets (by key) or accessories/furniture (by id) everywhere: the catalog, every setup that has
// them, the shared files nothing uses any more, and the game's copies of the active setup (the game must be closed).
func (a *App) DeleteFromCatalog(kind string, keys []string) (*catalog.Deleted, error) {
	a.setups.mu.Lock()
	defer a.setups.mu.Unlock()
	gameOK := game.IsGameDir(a.settings.GameDir)
	if gameOK && game.IsRunning() {
		return nil, errors.New("close the game first — its files are in use")
	}
	catalogMu.Lock()
	defer catalogMu.Unlock()
	// The active setup's sets that go: uninstalled from the game afterwards.
	activeSets := map[string]bool{}
	if kind == catalog.KindSet {
		for _, key := range keys {
			id, folder, _ := strings.Cut(key, "|")
			if p, err := a.ws().Load(id); err == nil && p.ArtFolder() == folder {
				activeSets[id] = true
			}
		}
	}
	res, err := a.catalog().DeleteEverywhere(a.home(), kind, keys)
	if err != nil || !gameOK {
		return res, err
	}
	if kind == catalog.KindSet {
		for id := range activeSets {
			_ = project.Uninstall(id, a.settings.GameDir)
		}
		for _, id := range res.SetIDs { // no setup has it any more: the game's shared copy goes too
			if setfmt.SafeID(id) {
				_ = os.RemoveAll(filepath.Join(project.GameLibraryDir(a.settings.GameDir), id))
			}
		}
		return res, nil
	}
	for _, s := range res.Setups {
		if s == a.activeID() {
			if l, lerr := a.accLib(); lerr == nil {
				_, err = a.saveAndInstall(l)
			}
		}
	}
	return res, err
}

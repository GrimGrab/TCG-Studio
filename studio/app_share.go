package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/importer"
	"tcgstudio/internal/share"
)

// Item files (.tcgitems, internal/share): Catalog → Export… writes the picked items; Import → "TCG Studio file" reads
// one like an EPL mod (pick or drop the file, preview, source, Import selected). Progress: "import:progress" events.

var itemFilters = []runtime.FileFilter{{DisplayName: "TCG Studio items (*" + share.ItemsExt + ")", Pattern: "*" + share.ItemsExt}}

// ExportResult is a finished export: the file written ("" = cancelled) and the converted items left out.
type ExportResult struct {
	File    string   `json:"file"`
	Skipped []string `json:"skipped"`
}

// ExportItems writes the picked items of the Catalog page (one kind, by id) to a file the player picks.
func (a *App) ExportItems(kind string, ids []string, name string) (ExportResult, error) {
	if len(ids) == 0 {
		return ExportResult{}, errors.New("nothing selected")
	}
	base := strings.TrimSpace(name)
	if base == "" {
		base = fmt.Sprintf("%d items", len(ids))
	}
	dest, err := a.saveFileAs("Export", safeFileName(base)+share.ItemsExt, share.ItemsExt, itemFilters)
	if err != nil || dest == "" {
		return ExportResult{}, err
	}
	picks := make([]share.Pick, len(ids))
	for i, id := range ids {
		picks[i] = share.Pick{Kind: kind, ID: id}
	}
	a.catalogMu.Lock()
	defer a.catalogMu.Unlock()
	// No progress events: "import:progress" belongs to the Import page's queue (it would show a stuck import), and the
	// Catalog page shows "Exporting…" itself.
	skipped, err := share.ExportItems(a.home(), a.root(), picks, dest, Version, nil)
	if skipped == nil {
		skipped = []string{}
	}
	return ExportResult{File: dest, Skipped: skipped}, err
}

// PickShareFile asks for an item file and returns its path ("" when cancelled); dropping a file works too (SharePath).
func (a *App) PickShareFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Select a TCG Studio item file", Filters: itemFilters})
}

// SharePath checks a dropped file is an item file.
func (a *App) SharePath(dropped string) (string, error) {
	if !strings.EqualFold(filepath.Ext(dropped), share.ItemsExt) {
		return "", errors.New("drop a " + share.ItemsExt + " file (exported from a Catalog page)")
	}
	if _, err := os.Stat(dropped); err != nil {
		return "", err
	}
	return dropped, nil
}

// PreviewShare reads an item file for the Import page: its items, the ids they'd get here and a suggested source.
func (a *App) PreviewShare(path string) (*share.Preview, error) {
	if err := a.setupsErr(); err != nil {
		return nil, err
	}
	a.catalogMu.Lock()
	defer a.catalogMu.Unlock()
	return share.PreviewItems(a.home(), path)
}

// ImportShare imports the chosen items of an item file into the catalog and (sel.AddHere) the active setup.
func (a *App) ImportShare(path string, sel share.Selection) (*share.Result, error) {
	a.setups.mu.Lock() // no setup switch meanwhile
	defer a.setups.mu.Unlock()
	if err := a.setupsErr(); err != nil {
		return nil, err
	}
	a.catalogMu.Lock()
	res, err := share.ImportItems(a.home(), a.root(), path, sel, func(msg string) {
		runtime.EventsEmit(a.ctx, evImportProgress, importer.Progress{Stage: "share", Message: msg})
	})
	a.catalogMu.Unlock()
	if err != nil || res == nil || res.Added == 0 {
		return res, err
	}
	if l, lerr := a.items(); lerr == nil {
		_, err = a.saveLibrary(l) // points the game at this setup's library, like any edit
	}
	return res, err
}

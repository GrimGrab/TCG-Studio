package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/game"
	"tcgstudio/internal/importer"
	"tcgstudio/internal/project"
)

// SyncInstalledSets brings the sets already installed in the game up to date with what this Studio version adds to
// set.json (today: the MTG export data, plus the Scryfall names/layouts older imports lack), so players don't have to
// reopen and reinstall every set after an update. Only the new fields are added to each installed set's set.json (which the
// game reads) — prices and licenses stay as they were. Runs once per Studio version (the frontend calls it
// on start); returns a short message for the player, or "" when nothing changed.
func (a *App) SyncInstalledSets() (string, error) {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	if a.cfg().SyncedVersion == Version && Version != "dev" {
		return "", nil
	}
	if !game.IsGameDir(a.gameDir()) {
		return "", nil // try again once the game folder is known
	}
	ws := a.ws()
	list, err := ws.List(a.gameDir())
	if err != nil {
		return "", err
	}
	var updated, failed []string
	offline := false
	for _, s := range list {
		if !s.Installed {
			continue
		}
		setFile := filepath.Join(ws.Folder(s.ID), project.SetFile)
		before, _ := os.ReadFile(setFile)
		p, err := a.loadProject(s.ID) // Load fills the MTG data from studio.json
		if err != nil {
			failed = append(failed, s.ID)
			continue
		}
		if importer.NeedsNames(p) {
			if _, err := importer.FillNames(a.ctx, a.sf, p); err != nil {
				offline = true // keep what we have; retried next start since the version isn't marked synced
			}
		}
		a.fillCollection(p) // binder grouping for sets imported before set.json had it
		if err := ws.Save(p); err != nil {
			failed = append(failed, p.Set.Name)
			continue
		}
		// The game reads set.json from the project: saving is all an update takes.
		if after, _ := os.ReadFile(setFile); !bytes.Equal(before, after) {
			updated = append(updated, p.Set.Name)
		}
	}
	if !offline && len(failed) == 0 {
		_, _ = a.updateSettings(func(s *Settings) { s.SyncedVersion = Version })
	}
	msg := ""
	if len(updated) > 0 {
		msg = fmt.Sprintf("Updated %d installed set(s) for this version: %s. Restart the game to load them.", len(updated), strings.Join(updated, ", "))
	}
	if len(failed) > 0 {
		return msg, fmt.Errorf("couldn't update installed set(s): %s", strings.Join(failed, ", "))
	}
	return msg, nil
}

package main

import (
	"fmt"
	"strings"

	"tcgstudio/internal/game"
	"tcgstudio/internal/importer"
	"tcgstudio/internal/project"
)

// SyncInstalledSets brings the sets already installed in the game up to date with what this Studio version adds to
// set.json (today: the MTG export data, plus the Scryfall names/layouts older imports lack), so players don't have to
// reopen and reinstall every set after an update. Only the new fields are written into the game's copy — prices,
// licenses and anything edited but not installed stay as they were. Runs once per Studio version (the frontend calls it
// on start); returns a short message for the player, or "" when nothing changed.
func (a *App) SyncInstalledSets() (string, error) {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()
	if a.settings.SyncedVersion == Version && Version != "dev" {
		return "", nil
	}
	if !game.IsGameDir(a.settings.GameDir) {
		return "", nil // try again once the game folder is known
	}
	list, err := a.ws().List(a.settings.GameDir)
	if err != nil {
		return "", err
	}
	var updated, failed []string
	offline := false
	for _, s := range list {
		if !a.isInstalled(s.ID) {
			continue
		}
		p, err := a.ws().Load(s.ID) // Load fills the MTG data from studio.json
		if err != nil {
			failed = append(failed, s.ID)
			continue
		}
		if importer.NeedsNames(p) {
			if _, err := importer.FillNames(a.ctx, a.sf, p); err != nil {
				offline = true // keep what we have; retried next start since the version isn't marked synced
			}
		}
		if err := a.ws().Save(p); err != nil {
			failed = append(failed, p.Set.Name)
			continue
		}
		changed, err := project.PatchInstalledMtg(p, a.settings.GameDir)
		if err != nil {
			failed = append(failed, p.Set.Name)
			continue
		}
		if changed {
			updated = append(updated, p.Set.Name)
		}
	}
	if !offline && len(failed) == 0 {
		a.settings.SyncedVersion = Version
		_ = saveSettings(a.settings)
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

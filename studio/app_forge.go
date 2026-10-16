package main

import (
	"errors"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/forge"
)

// ---------------------------------------------------------------- MTG mode (Forge)

// ForgeStatus reports whether Forge + Java for MTG mode are installed in the game folder.
func (a *App) ForgeStatus() forge.Status { return forge.Inspect(a.gameDir()) }

// InstallForge downloads Java and Forge into <game>\TCGForge (progress via "forge:progress" events), then saves the
// installed Scryfall sets so their set.json carries the MTG export data. Returns a summary line.
func (a *App) InstallForge() (string, error) {
	gameDir, err := a.requireGame()
	if err != nil {
		return "", err
	}
	ctx, done, err := a.task.start(a.ctx, "another download is already running")
	if err != nil {
		return "", err
	}
	defer done()
	if err := forge.Install(ctx, gameDir, func(p forge.Progress) {
		runtime.EventsEmit(a.ctx, evForgeProgress, p)
	}); err != nil {
		return "", err
	}
	n, err := a.saveMtgSets()
	msg := fmt.Sprintf("Forge %s installed.", forge.Version)
	if n > 0 {
		msg += fmt.Sprintf(" Updated %d installed MTG set(s) for deck export.", n)
	}
	return msg, err
}

// CancelForgeInstall stops a running InstallForge (same cancel slot as imports).
func (a *App) CancelForgeInstall() { a.task.stop() }

// RemoveForge deletes <game>\TCGForge.
func (a *App) RemoveForge() error {
	gameDir, err := a.requireGameClosed()
	if err != nil {
		return err
	}
	return forge.Remove(gameDir)
}

// saveMtgSets saves the installed Scryfall projects so their set.json carries the "mtg" data (filled on load) that deck
// export needs; the game reads it from there.
func (a *App) saveMtgSets() (int, error) {
	list, err := a.ws().List(a.gameDir())
	if err != nil {
		return 0, err
	}
	n := 0
	var errs []error
	for _, s := range list {
		if !s.Installed || s.Source != "scryfall" {
			continue
		}
		p, err := a.loadProject(s.ID)
		if err != nil || p.Set.Mtg == nil {
			continue
		}
		if _, err := a.saveSet(p); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Name, err)) // the other sets still get theirs
			continue
		}
		n++
	}
	return n, errors.Join(errs...)
}

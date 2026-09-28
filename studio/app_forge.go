package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/forge"
	"tcgstudio/internal/game"
)

// ---------------------------------------------------------------- MTG mode (Forge)

// ForgeStatus reports whether Forge + Java for MTG mode are installed in the game folder.
func (a *App) ForgeStatus() forge.Status { return forge.Inspect(a.settings.GameDir) }

// InstallForge downloads Java and Forge into <game>\TCGForge (progress via "forge:progress" events), then reinstalls the
// installed Scryfall sets so their set.json carries the MTG export data. Returns a summary line.
func (a *App) InstallForge() (string, error) {
	if !game.IsGameDir(a.settings.GameDir) {
		return "", errors.New("set the game folder first")
	}
	a.mu.Lock()
	if a.cancel != nil {
		a.mu.Unlock()
		return "", errors.New("another download is already running")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.cancel = nil
		a.mu.Unlock()
		cancel()
	}()
	if err := forge.Install(ctx, a.settings.GameDir, func(p forge.Progress) {
		runtime.EventsEmit(a.ctx, "forge:progress", p)
	}); err != nil {
		return "", err
	}
	n, err := a.reinstallMtgSets()
	msg := fmt.Sprintf("Forge %s installed.", forge.Version)
	if n > 0 {
		msg += fmt.Sprintf(" Updated %d installed MTG set(s) for deck export.", n)
	}
	return msg, err
}

// CancelForgeInstall stops a running InstallForge (same cancel slot as imports).
func (a *App) CancelForgeInstall() { a.CancelImport() }

// RemoveForge deletes <game>\TCGForge.
func (a *App) RemoveForge() error {
	if game.IsRunning() {
		return errors.New("close the game (and Forge) first")
	}
	return forge.Remove(a.settings.GameDir)
}

// reinstallMtgSets re-copies installed Scryfall projects to the game so they include set.json "mtg" data.
func (a *App) reinstallMtgSets() (int, error) {
	list, err := a.ws().List(a.settings.GameDir)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, s := range list {
		if !s.Installed || s.Source != "scryfall" {
			continue
		}
		p, err := a.ws().Load(s.ID)
		if err != nil || p.Set.Mtg == nil {
			continue
		}
		if err := a.ws().Save(p); err != nil {
			return n, err
		}
		if err := a.ws().Install(p, a.settings.GameDir); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

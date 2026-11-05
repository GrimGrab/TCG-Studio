package main

import (
	"tcgstudio/internal/game"
	"tcgstudio/internal/installer"
	"tcgstudio/internal/migrate"
)

// On every start the game is pointed at the active setup and an older mod in the game is updated. A workspace still in the
// layout from before shared storage is left alone: TCG Studio v1.14.0 upgrades it (migrate.OldLayout).

// StartupResult tells the app what the start-up step did and what is still waiting.
type StartupResult struct {
	OldLayoutURL string `json:"oldLayoutUrl"` // the workspace needs v1.14.0 first: where to get it
	Version      string `json:"version"`      // that version
	Waiting      string `json:"waiting"`      // why it couldn't run now (the game is running), shown until it can
	ModUpdated   bool   `json:"modUpdated"`   // the mod in the game was updated
}

// StartupUpdate runs the start-up step (see above). The frontend calls it on start and after Setup actions.
func (a *App) StartupUpdate() (*StartupResult, error) {
	out := &StartupResult{}
	if a.setupsErr() != nil {
		return out, nil
	}
	gameDir := a.gameDir()
	if migrate.OldLayout(a.home(), gameDir) {
		out.OldLayoutURL, out.Version = legacyReleaseURL(), legacyRelease
		return out, nil
	}
	if game.IsGameDir(gameDir) && game.IsRunning() {
		if modOutdatedOnly(gameDir) {
			out.Waiting = waitText
		}
		return out, nil
	}
	a.pointGameAtWorkspace()
	if modOutdatedOnly(gameDir) {
		if a.waitingForGame(out) {
			return out, nil
		}
		if _, err := a.SetupRepair(); err != nil {
			return out, err
		}
		out.ModUpdated = true
	}
	return out, nil
}

// autoTaskLabel is the storage task of the automatic art conversion (the frontend's AUTO_TASK shows its progress).
const autoTaskLabel = "Updating your content for the game"

const waitText = "Please close the game: TCG Studio then updates the mod in it."

// waitingForGame re-checks right before a step (the game may have been started meanwhile).
func (a *App) waitingForGame(out *StartupResult) bool {
	if game.IsGameDir(a.gameDir()) && game.IsRunning() {
		out.Waiting = waitText
		return true
	}
	return false
}

// modOutdatedOnly: the game has BepInEx and our mod, and only the mod is older than the one this Studio installs. Anything
// else (no BepInEx, conflicts, a friend's first run) stays on the Game & mod page for the player.
func modOutdatedOnly(gameDir string) bool {
	if !game.IsGameDir(gameDir) {
		return false
	}
	p, err := installer.Embedded()
	if err != nil {
		return false
	}
	st := installer.Inspect(gameDir, p, true, false)
	outdated := false
	for _, it := range st.Items {
		switch {
		case it.ID == "mod" && it.State == installer.Outdated:
			outdated = true
		case it.ID == "templates", it.State == installer.OK, it.State == installer.Info:
		default:
			return false
		}
	}
	return outdated
}

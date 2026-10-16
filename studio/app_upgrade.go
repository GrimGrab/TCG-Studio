package main

import (
	"fmt"

	"tcgstudio/internal/game"
	"tcgstudio/internal/installer"
	"tcgstudio/internal/upgrade"
)

// What the game's mod needs from an older TCG Studio's layout happens on its own, on every start (each step does nothing
// once done): every setup takes over what older versions kept in the game folder, set and item files move into shared
// storage, the game is pointed at the active setup, and an older mod in the game is updated. Only real choices (a setup's
// art that differs from the shared art) wait for the player on the Storage page; cleanup stays a button there.

// AutoUpgradeResult tells the app what the start-up upgrade did and what is still waiting.
type AutoUpgradeResult struct {
	Notes      []string `json:"notes"`      // what was taken over (shown once)
	Files      int      `json:"files"`      // files moved
	Choices    int      `json:"choices"`    // differing copies / folders waiting for the player (Storage page)
	Waiting    string   `json:"waiting"`    // why it couldn't run now (the game is running), shown until it can
	ModUpdated bool     `json:"modUpdated"` // the mod in the game was updated
}

// AutoUpgrade runs the required upgrade steps (see above). The frontend calls it on start and after Setup actions.
func (a *App) AutoUpgrade() (*AutoUpgradeResult, error) {
	out := &AutoUpgradeResult{Notes: []string{}}
	if a.setupsErr() != nil {
		return out, nil
	}
	a.setups.mu.Lock()
	err := upgrade.FillOrigins(a.home())
	a.setups.mu.Unlock()
	if err != nil {
		return out, err
	}
	gameDir := a.gameDir()
	if game.IsGameDir(gameDir) && game.IsRunning() {
		if upgrade.Needed(a.home(), gameDir) || modOutdatedOnly(gameDir) {
			out.Waiting = waitText
		}
		return out, nil
	}
	if upgrade.Needed(a.home(), gameDir) {
		if a.waitingForGame(out) {
			return out, nil
		}
		res, err := a.runAutoUpgrade(gameDir)
		if res != nil {
			out.Notes, out.Files, out.Choices = res.Notes, res.Files, res.Left
			if res.Kept > 0 {
				out.Notes = append(out.Notes, fmt.Sprintf("%d file(s) that weren't exact copies were kept, not deleted (Storage → Kept by the upgrade)", res.Kept))
			}
		}
		if err != nil {
			return out, err
		}
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

const waitText = "Please close the game: TCG Studio then finishes updating your content and the mod for it."

// waitingForGame re-checks right before a step (the game may have been started meanwhile).
func (a *App) waitingForGame(out *AutoUpgradeResult) bool {
	if game.IsGameDir(a.gameDir()) && game.IsRunning() {
		out.Waiting = waitText
		return true
	}
	return false
}

// runAutoUpgrade is upgrade.Run without the player's choices, as a storage task (the Storage page shows its progress).
func (a *App) runAutoUpgrade(gameDir string) (*upgrade.Result, error) {
	ctx, done, err := a.startStorage("Updating your content for the game") // the frontend's AUTO_TASK
	if err != nil {
		return nil, nil // another storage task is running: next start
	}
	defer done()
	a.setups.mu.Lock()
	defer a.setups.mu.Unlock()
	return upgrade.Run(ctx, a.home(), gameDir, upgrade.Choices{Partial: true}, a.upgradeProgress)
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

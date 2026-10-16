package setups

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/workspace"

	"tcgstudio/internal/game"
	"tcgstudio/internal/modconfig"
	"tcgstudio/internal/project"
)

// Journal records an unfinished switch so the next start can finish or undo it.
type Journal struct {
	From    string    `json:"from"`
	To      string    `json:"to"`
	Phase   string    `json:"phase"` // PhaseCapture (undo) or PhaseApply (finish)
	Started time.Time `json:"started"`
}

const (
	PhaseCapture = "capture"
	PhaseApply   = "apply"
)

// failAt lets tests (and TCGSTUDIO_SWITCH_FAIL=<phase> for a manual test) stop a switch right after a phase starts.
var failAt = os.Getenv("TCGSTUDIO_SWITCH_FAIL")

func (h Home) journalPath() string { return filepath.Join(h.Root, JournalFile) }

func (h Home) PendingSwitch() (*Journal, bool) {
	j := &Journal{}
	if err := readJSON(h.journalPath(), j); err != nil {
		return nil, false
	}
	return j, true
}

// Snapshot copies the game's mod config into the active setup's game\ folder. Everything else the game uses already lives
// in the setup (installed sets, accessory library, card back, Shop look pictures; game.StudioPathsFile). Nothing in the
// game changes.
func Snapshot(dir, gameDir string) error {
	if !game.IsGameDir(gameDir) {
		return errors.New("game folder not set")
	}
	if _, err := LoadInfo(dir); err != nil {
		return err
	}
	dst := filepath.Join(dir, GameStateDir, cfgFile)
	if cfg := modconfig.Path(gameDir); fsutil.Exists(cfg) {
		return fsutil.CopyFile(cfg, dst)
	}
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// clearGame removes the active setup's mod config from the game (saves are handled separately).
func clearGame(gameDir string) error {
	if err := os.Remove(modconfig.Path(gameDir)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// applyGame makes setup dir the game's: its mod config goes into the game (a setup without one leaves none, so the mod
// writes its defaults) and the mod is pointed at the setup and the workspace's shared folders (game.StudioPathsFile).
// Nothing else is copied. Returns warnings for the player.
func applyGame(dir, gameDir string, ws workspace.Dir) ([]string, error) {
	in, err := LoadInfo(dir)
	if err != nil {
		return nil, err
	}
	var warnings []string
	if len(in.InstalledSets) > 0 && !game.ModReadsLibrary(gameDir) {
		warnings = append(warnings, fmt.Sprintf("%d set(s) won't load: %s", len(in.InstalledSets), project.ModTooOld.Error()))
	}
	if src := filepath.Join(dir, GameStateDir, cfgFile); fsutil.Exists(src) {
		if err := fsutil.CopyFile(src, modconfig.Path(gameDir)); err != nil {
			return warnings, err
		}
	}
	err = game.UpdateStudioPaths(gameDir, func(sp *game.StudioPaths) {
		sp.Setup, sp.Library, sp.Assets = dir, ws.Library(), ws.Store().Dir
	})
	return warnings, err
}

// Switch makes setup to the active one: the active setup's game state is snapshotted and its saves parked in its saves\
// folder, the game is cleared and the new setup installed with its saves. The caller makes sure the game isn't running.
func (h Home) Switch(to, gameDir string, progress func(string)) ([]string, error) {
	if progress == nil {
		progress = func(string) {}
	}
	if !game.IsGameDir(gameDir) {
		return nil, errors.New("game folder not set (Settings → Game)")
	}
	if j, ok := h.PendingSwitch(); ok {
		return nil, fmt.Errorf("an earlier switch (%s → %s) didn't finish; restart TCG Studio to repair it", j.From, j.To)
	}
	r, err := h.Registry()
	if err != nil {
		return nil, err
	}
	from := r.Active
	if to == from {
		return nil, nil
	}
	if !fsutil.Exists(filepath.Join(h.Dir(to), InfoFile)) {
		return nil, errors.New("unknown setup")
	}
	if !r.SavesBackedUp && HasSaves(SaveDir()) {
		progress("Backing up your game saves (first switch only)…")
		backup := filepath.Join(h.Root, "saves-backup-"+time.Now().Format("20060102-150405"))
		if err := copySaves(SaveDir(), backup); err != nil {
			return nil, fmt.Errorf("couldn't back up the saves: %w", err)
		}
		r.SavesBackedUp = true
		if err := h.saveRegistry(r); err != nil {
			return nil, err
		}
	}

	j := &Journal{From: from, To: to, Phase: PhaseCapture, Started: time.Now()}
	if err := writeJSON(h.journalPath(), j); err != nil {
		return nil, err
	}
	progress("Saving the current setup…")
	if err := h.capture(from, gameDir); err != nil {
		if rbErr := h.undoCapture(j); rbErr != nil {
			return nil, fmt.Errorf("%v (undo failed too: %v — restart TCG Studio to retry)", err, rbErr)
		}
		return nil, err
	}
	if failAt == PhaseCapture {
		return nil, errors.New("test: stopped after capture")
	}
	j.Phase = PhaseApply
	if err := writeJSON(h.journalPath(), j); err != nil {
		return nil, err
	}
	return h.finish(j, gameDir, progress)
}

// capture snapshots the game state into setup from and moves the saves into its saves\ folder (an existing one is kept
// aside as saves-conflict-<time>, so nothing is overwritten).
func (h Home) capture(from, gameDir string) error {
	dir := h.Dir(from)
	if err := Snapshot(dir, gameDir); err != nil {
		return err
	}
	saves := filepath.Join(dir, SavesDir)
	if !dirEmpty(saves) {
		if err := os.Rename(saves, saves+"-conflict-"+time.Now().Format("20060102-150405")); err != nil {
			return err
		}
	}
	return moveSaves(SaveDir(), saves)
}

// undoCapture puts the saves moved by an interrupted capture back into the game.
func (h Home) undoCapture(j *Journal) error {
	saves := filepath.Join(h.Dir(j.From), SavesDir)
	if err := moveSaves(saves, SaveDir()); err != nil {
		return err
	}
	_ = os.Remove(saves)
	return os.Remove(h.journalPath())
}

// finish clears the game, installs setup j.To with its saves and marks it active. Safe to run again after an interruption.
func (h Home) finish(j *Journal, gameDir string, progress func(string)) ([]string, error) {
	if progress == nil {
		progress = func(string) {}
	}
	progress("Taking the old setup's settings out of the game…")
	if err := clearGame(gameDir); err != nil {
		return nil, err
	}
	if failAt == PhaseApply {
		return nil, errors.New("test: stopped during apply")
	}
	progress("Pointing the game at the new setup…")
	warnings, err := applyGame(h.Dir(j.To), gameDir, workspace.Dir(h.Root))
	if err != nil {
		return warnings, err
	}
	progress("Restoring its saves…")
	saves := filepath.Join(h.Dir(j.To), SavesDir)
	if err := moveSaves(saves, SaveDir()); err != nil {
		return warnings, err
	}
	_ = os.Remove(saves)
	r, err := h.Registry()
	if err != nil {
		return warnings, err
	}
	r.Active = j.To
	if err := h.saveRegistry(r); err != nil {
		return warnings, err
	}
	return warnings, os.Remove(h.journalPath())
}

// Recover finishes or undoes a switch that was interrupted (Studio closed or crashed). Returns a message for the player,
// or "" when there was nothing to do.
func (h Home) Recover(gameDir string) (string, error) {
	j, ok := h.PendingSwitch()
	if !ok {
		return "", nil
	}
	if j.Phase == PhaseCapture {
		if err := h.undoCapture(j); err != nil {
			return "", fmt.Errorf("couldn't undo the unfinished setup switch: %w", err)
		}
		return "An unfinished setup switch was undone; your previous setup is still active.", nil
	}
	if !game.IsGameDir(gameDir) {
		return "", errors.New("an unfinished setup switch needs the game folder (Settings → Game) to finish")
	}
	warnings, err := h.finish(j, gameDir, nil)
	if err != nil {
		return "", fmt.Errorf("couldn't finish the unfinished setup switch: %w", err)
	}
	msg := "An unfinished setup switch was completed."
	if len(warnings) > 0 {
		msg += " " + strings.Join(warnings, "; ")
	}
	return msg, nil
}

// CopyGameSaves copies the game's current saves into setup id's saves\ folder (duplicating the active setup with saves).
func (h Home) CopyGameSaves(id string) error {
	return copySaves(SaveDir(), filepath.Join(h.Dir(id), SavesDir))
}

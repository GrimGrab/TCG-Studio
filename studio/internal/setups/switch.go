package setups

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/game"
	"tcgstudio/internal/modconfig"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
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

// Snapshot copies the game-side state of the active setup into its game\ folder and records which of its projects are
// installed. Nothing in the game changes. Accessories installed in the game while the setup's library is empty (put there
// by hand) are kept as a copy too, so a switch never loses them.
func Snapshot(dir, gameDir string) error {
	if !game.IsGameDir(gameDir) {
		return errors.New("game folder not set")
	}
	in, err := LoadInfo(dir)
	if err != nil {
		return err
	}
	g := filepath.Join(dir, GameStateDir)
	tmp := g + ".new"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	if cfg := modconfig.Path(gameDir); exists(cfg) {
		if err := copyFile(cfg, filepath.Join(tmp, cfgFile)); err != nil {
			return err
		}
	}
	if back := filepath.Join(game.PluginDir(gameDir), backFile); exists(back) {
		if err := copyFile(back, filepath.Join(tmp, backFile)); err != nil {
			return err
		}
	}
	installed := []string{}
	entries, _ := os.ReadDir(game.SetsDir(gameDir))
	for _, e := range entries {
		if !e.IsDir() || strings.HasSuffix(e.Name(), ".installing") {
			continue
		}
		if exists(filepath.Join(dir, "projects", e.Name(), project.SetFile)) {
			installed = append(installed, e.Name())
			continue
		}
		if err := copyTree(filepath.Join(game.SetsDir(gameDir), e.Name()), filepath.Join(tmp, "Sets", e.Name()), nil); err != nil {
			return err
		}
	}
	if acc := accessories.InstalledDir(gameDir); exists(acc) && libraryEmpty(dir) {
		if err := copyTree(acc, filepath.Join(tmp, accessories.InstallDir), nil); err != nil {
			return err
		}
	}
	_ = os.RemoveAll(g)
	if err := os.Rename(tmp, g); err != nil {
		return err
	}
	in.InstalledSets = installed
	return SaveInfo(dir, in)
}

func libraryEmpty(dir string) bool {
	lib, err := setfmt.LoadAccessories(filepath.Join(accessories.Folder(dir), accessories.LibraryFile))
	return err != nil || (len(lib.Accessories) == 0 && len(lib.Furniture) == 0)
}

// clearGame removes everything a setup puts into the game: installed sets, accessories, global card back, mod config.
// Saves are handled separately.
func clearGame(gameDir string) error {
	entries, _ := os.ReadDir(game.SetsDir(gameDir))
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(game.SetsDir(gameDir), e.Name())); err != nil {
			return err
		}
	}
	for _, p := range []string{accessories.InstalledDir(gameDir), filepath.Join(game.PluginDir(gameDir), backFile), modconfig.Path(gameDir)} {
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}

// applyGame installs a setup into a cleared game. A setup without a mod config leaves none, so the mod writes its defaults.
// Returns warnings for projects that couldn't be installed.
func applyGame(dir, gameDir string) ([]string, error) {
	in, err := LoadInfo(dir)
	if err != nil {
		return nil, err
	}
	var warnings []string
	ws := project.Workspace{Root: dir, Library: project.LibraryDir(filepath.Dir(filepath.Dir(dir)))} // dir = <workspace>\setups\<id>
	for _, id := range in.InstalledSets {
		p, err := ws.Load(id)
		if err == nil {
			err = ws.Install(p, gameDir)
		}
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("set %s: %v", id, err))
		}
	}
	g := filepath.Join(dir, GameStateDir)
	if lib, err := accessories.Open(dir); err != nil {
		warnings = append(warnings, "accessories: "+err.Error())
	} else if len(lib.Lib.Accessories) > 0 || len(lib.Lib.Furniture) > 0 {
		if err := lib.Install(gameDir); err != nil {
			warnings = append(warnings, "accessories: "+err.Error())
		}
	} else if src := filepath.Join(g, accessories.InstallDir); exists(src) {
		if err := copyTree(src, accessories.InstalledDir(gameDir), nil); err != nil {
			return warnings, err
		}
	}
	if src := filepath.Join(g, "Sets"); exists(src) {
		if err := copyTree(src, game.SetsDir(gameDir), nil); err != nil {
			return warnings, err
		}
	}
	if src := filepath.Join(g, backFile); exists(src) {
		if err := copyFile(src, filepath.Join(game.PluginDir(gameDir), backFile)); err != nil {
			return warnings, err
		}
	}
	if src := filepath.Join(g, cfgFile); exists(src) {
		if err := copyFile(src, modconfig.Path(gameDir)); err != nil {
			return warnings, err
		}
	}
	return warnings, nil
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
	if !exists(filepath.Join(h.Dir(to), InfoFile)) {
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
	progress("Removing the old setup from the game…")
	if err := clearGame(gameDir); err != nil {
		return nil, err
	}
	if failAt == PhaseApply {
		return nil, errors.New("test: stopped during apply")
	}
	progress("Installing the new setup…")
	warnings, err := applyGame(h.Dir(j.To), gameDir)
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

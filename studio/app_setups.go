package main

import (
	"errors"
	"os/exec"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/game"
	"tcgstudio/internal/setups"
)

// setupsState is the active setup's folder (what ws() and the accessory library use) plus what startup found.
type setupsState struct {
	mu      sync.Mutex // serializes switch / create / import / export
	rootMu  sync.RWMutex
	root    string
	err     string // migration failed: Studio keeps using the old single-workspace layout
	message string // result of repairing an interrupted switch, shown once
}

func (a *App) home() setups.Home { return setups.Home{Root: a.settings.Workspace} }

// root is the active setup's folder, or the workspace itself when setups couldn't be set up.
func (a *App) root() string {
	a.setups.rootMu.RLock()
	defer a.setups.rootMu.RUnlock()
	if a.setups.root != "" {
		return a.setups.root
	}
	return a.settings.Workspace
}

func (a *App) setRoot(dir string) {
	a.setups.rootMu.Lock()
	a.setups.root = dir
	a.setups.rootMu.Unlock()
}

// initSetups migrates the workspace to the setups layout if needed and repairs an interrupted switch.
func (a *App) initSetups() {
	a.setups.mu.Lock()
	defer a.setups.mu.Unlock()
	a.setups.err, a.setups.message = "", ""
	a.setRoot("")
	h := a.home()
	if game.IsGameDir(a.settings.GameDir) && !game.IsRunning() {
		msg, err := h.Recover(a.settings.GameDir)
		if err != nil {
			a.setups.err = err.Error()
		}
		a.setups.message = msg
	}
	dir, err := h.Ensure(Version)
	if err != nil {
		a.setups.err = err.Error()
		return
	}
	a.setRoot(dir)
}

// SetupsView is the Setups page.
type SetupsView struct {
	Setups      []setups.Summary `json:"setups"`
	Active      string           `json:"active"`
	GameRunning bool             `json:"gameRunning"`
	GameFound   bool             `json:"gameFound"`
	SteamCloud  bool             `json:"steamCloud"` // Steam Auto-Cloud syncs the save folder
	Error       string           `json:"error"`      // setups unavailable (migration failed)
	Message     string           `json:"message"`    // one-time notice (interrupted switch repaired)
	Pending     bool             `json:"pending"`    // an interrupted switch still waits for repair
	Workspace   string           `json:"workspace"`
}

func (a *App) ListSetups() (SetupsView, error) {
	h := a.home()
	v := SetupsView{GameRunning: game.IsRunning(), GameFound: game.IsGameDir(a.settings.GameDir), SteamCloud: setups.SteamCloud(),
		Error: a.setups.err, Message: a.setups.message, Workspace: a.settings.Workspace, Setups: []setups.Summary{}}
	a.setups.message = ""
	_, v.Pending = h.PendingSwitch()
	if v.Error != "" && !v.Pending {
		return v, nil
	}
	list, err := h.List()
	if err != nil {
		if v.Error == "" {
			v.Error = err.Error()
		}
		return v, nil
	}
	v.Setups = list
	for _, s := range list {
		if s.Active {
			v.Active = s.ID
		}
	}
	return v, nil
}

// RetrySetups runs the startup migration / repair again (after closing the game or a file that blocked it).
func (a *App) RetrySetups() (SetupsView, error) {
	a.initSetups()
	return a.ListSetups()
}

func (a *App) activeID() string {
	id, _ := a.home().Active()
	return id
}

// snapshotActive copies the game-side state of the active setup into it (before duplicating or exporting it).
func (a *App) snapshotActive(id string) error {
	if id != a.activeID() || !game.IsGameDir(a.settings.GameDir) {
		return nil
	}
	return setups.Snapshot(a.home().Dir(id), a.settings.GameDir)
}

// CreateSetup makes a new setup: empty (copyFrom "") or a copy of another one, with that setup's saves when withSaves.
func (a *App) CreateSetup(name, copyFrom string, withSaves bool) (SetupsView, error) {
	if err := a.createSetup(name, copyFrom, withSaves); err != nil {
		return SetupsView{}, err
	}
	return a.ListSetups()
}

func (a *App) createSetup(name, copyFrom string, withSaves bool) error {
	a.setups.mu.Lock()
	defer a.setups.mu.Unlock()
	if a.setups.err != "" {
		return errors.New(a.setups.err)
	}
	h := a.home()
	active := copyFrom != "" && copyFrom == a.activeID()
	if active && withSaves && game.IsRunning() {
		return errors.New("close the game first so its saves can be copied")
	}
	if err := a.snapshotActive(copyFrom); err != nil {
		return err
	}
	id, err := h.Create(name, copyFrom, withSaves && !active, Version)
	if err != nil {
		return err
	}
	if active && withSaves {
		if err := h.CopyGameSaves(id); err != nil {
			_ = h.Delete(id)
			return err
		}
	}
	return nil
}

func (a *App) UpdateSetup(id, name, description string) (SetupsView, error) {
	if err := a.home().Update(id, name, description); err != nil {
		return SetupsView{}, err
	}
	return a.ListSetups()
}

func (a *App) MoveSetup(id string, to int) (SetupsView, error) {
	if err := a.home().Move(id, to); err != nil {
		return SetupsView{}, err
	}
	return a.ListSetups()
}

func (a *App) DeleteSetup(id string) (SetupsView, error) {
	a.setups.mu.Lock()
	// Keep its sets and items available to the other setups (files are shared; only the game info is templated).
	catalogMu.Lock()
	err := a.catalog().KeepSetup(a.home().Dir(id))
	if err != nil {
		err = errors.New("couldn't keep its sets and items in the shared catalog, so it wasn't deleted: " + err.Error())
	}
	catalogMu.Unlock()
	if err == nil {
		err = a.home().Delete(id)
	}
	a.setups.mu.Unlock()
	if err != nil {
		return SetupsView{}, err
	}
	return a.ListSetups()
}

// SwitchResult tells the frontend what happened; it reloads every view afterwards.
type SwitchResult struct {
	Warnings []string `json:"warnings"`
}

// SwitchSetup makes another setup the active one (game closed): parks the current game content and saves in the current
// setup and installs the chosen one with its saves. Progress: "setups:progress" events with a text.
func (a *App) SwitchSetup(id string) (SwitchResult, error) {
	a.setups.mu.Lock()
	defer a.setups.mu.Unlock()
	if a.setups.err != "" {
		return SwitchResult{}, errors.New(a.setups.err)
	}
	if !game.IsGameDir(a.settings.GameDir) {
		return SwitchResult{}, errors.New("game folder not set (Settings → Game)")
	}
	if game.IsRunning() {
		return SwitchResult{}, errors.New("close the game first — its files are in use")
	}
	h := a.home()
	warnings, err := h.Switch(id, a.settings.GameDir, func(s string) { runtime.EventsEmit(a.ctx, "setups:progress", s) })
	if act, aerr := h.Active(); aerr == nil {
		a.setRoot(h.Dir(act))
	}
	if err != nil {
		return SwitchResult{Warnings: warnings}, err
	}
	// The new setup's sets may predate this Studio version: let SyncInstalledSets look at them again.
	a.settings.SyncedVersion = ""
	_ = saveSettings(a.settings)
	if warnings == nil {
		warnings = []string{}
	}
	return SwitchResult{Warnings: warnings}, nil
}

var setupFilters = []runtime.FileFilter{{DisplayName: "TCG Studio setup (*" + setups.Ext + ")", Pattern: "*" + setups.Ext}}

// ExportSetup writes a setup (without saves) to a file the player picks. Returns the file, or "" when cancelled.
// Progress: "setups:export" events with {done, total}.
func (a *App) ExportSetup(id string) (string, error) {
	h := a.home()
	in, err := setups.LoadInfo(h.Dir(id))
	if err != nil {
		return "", err
	}
	dest, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "Export setup", Filters: setupFilters,
		DefaultFilename: safeFileName(in.Name) + setups.Ext})
	if err != nil || dest == "" {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(dest), setups.Ext) {
		dest += setups.Ext
	}
	a.setups.mu.Lock()
	defer a.setups.mu.Unlock()
	if err := a.snapshotActive(id); err != nil {
		return "", err
	}
	err = h.Export(id, dest, Version, func(done, total int) {
		runtime.EventsEmit(a.ctx, "setups:export", map[string]int{"done": done, "total": total})
	})
	return dest, err
}

// SetupFileInfo previews a setup file before importing it.
type SetupFileInfo struct {
	File     string           `json:"file"`
	Manifest *setups.Manifest `json:"manifest"`
}

// PickSetupFile asks for a .tcgsetup file and reads its manifest (nil result when cancelled).
func (a *App) PickSetupFile() (*SetupFileInfo, error) {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Import setup", Filters: setupFilters})
	if err != nil || file == "" {
		return nil, err
	}
	m, err := setups.ReadManifest(file)
	if err != nil {
		return nil, err
	}
	return &SetupFileInfo{File: file, Manifest: m}, nil
}

// ImportSetup adds the setup in file as a new, inactive setup with no saves and returns its id.
func (a *App) ImportSetup(file string) (string, error) {
	a.setups.mu.Lock()
	defer a.setups.mu.Unlock()
	if a.setups.err != "" {
		return "", errors.New(a.setups.err)
	}
	return a.home().Import(file, Version)
}

func (a *App) OpenSetupFolder(id string) {
	_ = exec.Command("explorer", a.home().Dir(id)).Start()
}

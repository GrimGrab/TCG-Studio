package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/game"
	"tcgstudio/internal/setups"
)

// setupsState is the active setup's folder (what ws() and the accessory library use) plus what startup found.
type setupsState struct {
	mu      sync.Mutex   // serializes switch / create / import / export
	rootMu  sync.RWMutex // root, err and message (read by bound methods while mu is held for a long switch)
	root    string
	err     string // migration failed: Studio keeps using the old single-workspace layout
	message string // result of repairing an interrupted switch, shown once
}

func (a *App) home() setups.Home { return setups.Home{Root: a.workspaceDir()} }

// root is the active setup's folder, or the workspace itself when setups couldn't be set up.
func (a *App) root() string {
	a.setups.rootMu.RLock()
	defer a.setups.rootMu.RUnlock()
	if a.setups.root != "" {
		return a.setups.root
	}
	return a.workspaceDir()
}

// setupsErr is the migration error that keeps Studio on the old layout, or nil.
func (a *App) setupsErr() error {
	a.setups.rootMu.RLock()
	defer a.setups.rootMu.RUnlock()
	if a.setups.err != "" {
		return errors.New(a.setups.err)
	}
	return nil
}

func (a *App) setSetupsStatus(errText, message string) {
	a.setups.rootMu.Lock()
	a.setups.err, a.setups.message = errText, message
	a.setups.rootMu.Unlock()
}

// takeSetupsStatus returns the error and the one-time message (cleared).
func (a *App) takeSetupsStatus() (errText, message string) {
	a.setups.rootMu.Lock()
	defer a.setups.rootMu.Unlock()
	errText, message = a.setups.err, a.setups.message
	a.setups.message = ""
	return
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
	a.setRoot("")
	var errText, msg string
	defer func() { a.setSetupsStatus(errText, msg) }()
	h := a.home()
	if gameDir, err := a.requireGameClosed(); err == nil {
		if msg, err = h.Recover(gameDir); err != nil {
			errText = err.Error()
		}
	}
	dir, err := h.Ensure(Version)
	if err != nil {
		errText = err.Error()
		return
	}
	a.setRoot(dir)
	a.pointGameAtWorkspace()
}

// pointGameAtWorkspace keeps the game's mod reading the active setup and this workspace's shared folders
// (game.StudioPathsFile).
func (a *App) pointGameAtWorkspace() {
	if gameDir := a.gameDir(); game.IsGameDir(gameDir) {
		_ = game.UpdateStudioPaths(gameDir, func(sp *game.StudioPaths) {
			sp.Setup, sp.Library, sp.Assets = a.root(), a.wsDir().Library(), a.wsDir().Store().Dir
		})
	}
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
	v := SetupsView{GameRunning: game.IsRunning(), GameFound: game.IsGameDir(a.gameDir()), SteamCloud: setups.SteamCloud(),
		Workspace: a.workspaceDir(), Setups: []setups.Summary{}}
	v.Error, v.Message = a.takeSetupsStatus()
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
	gameDir, err := a.requireGame()
	if id != a.activeID() || err != nil {
		return nil
	}
	return setups.Snapshot(a.home().Dir(id), gameDir)
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
	if err := a.setupsErr(); err != nil {
		return err
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
	if err := a.deleteSetup(id); err != nil {
		return SetupsView{}, err
	}
	return a.ListSetups()
}

func (a *App) deleteSetup(id string) error {
	a.setups.mu.Lock()
	defer a.setups.mu.Unlock()
	h := a.home()
	if act, err := h.Active(); err == nil && act == id {
		return errors.New("switch to another setup before deleting this one")
	}
	// Keep its sets and items available to the other setups (files are shared; only the game info is templated).
	a.catalogMu.Lock()
	err := a.catalog().KeepSetup(h.Dir(id))
	a.catalogMu.Unlock()
	if err != nil {
		return errors.New("couldn't keep its sets and items in the shared catalog, so it wasn't deleted: " + err.Error())
	}
	return h.Delete(id)
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
	if err := a.setupsErr(); err != nil {
		return SwitchResult{}, err
	}
	gameDir, err := a.requireGameClosed()
	if err != nil {
		return SwitchResult{}, err
	}
	if !game.ModReadsLibrary(gameDir) {
		return SwitchResult{}, errOldMod // an older mod reads the game folder, which a switch no longer changes
	}
	h := a.home()
	warnings, err := h.Switch(id, gameDir, func(s string) { runtime.EventsEmit(a.ctx, evSetupsProgress, s) })
	if act, aerr := h.Active(); aerr == nil {
		a.setRoot(h.Dir(act))
	}
	if err != nil {
		return SwitchResult{Warnings: warnings}, err
	}
	// The new setup's sets may predate this Studio version: let SyncInstalledSets look at them again.
	_, _ = a.updateSettings(func(s *Settings) { s.SyncedVersion = "" })
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
	dest, err := a.saveFileAs("Export setup", safeFileName(in.Name)+setups.Ext, setups.Ext, setupFilters)
	if err != nil || dest == "" {
		return "", err
	}
	a.setups.mu.Lock()
	defer a.setups.mu.Unlock()
	if err := a.snapshotActive(id); err != nil {
		return "", err
	}
	err = h.Export(id, dest, Version, func(done, total int) {
		runtime.EventsEmit(a.ctx, evSetupsExport, map[string]int{"done": done, "total": total})
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
func (a *App) ImportSetup(file string) (SetupImport, error) {
	a.setups.mu.Lock()
	defer a.setups.mu.Unlock()
	if err := a.setupsErr(); err != nil {
		return SetupImport{}, err
	}
	id, kept, err := a.home().Import(file, Version)
	r := SetupImport{ID: id}
	if len(kept) > 0 {
		r.Note = fmt.Sprintf("%d file(s) of its sets differ from the ones on this PC: this PC's were kept (%s).", len(kept), strings.Join(firstN(kept, 3), ", "))
	}
	return r, err
}

// SetupImport is an imported setup: its id and a note for the player ("" = nothing to say).
type SetupImport struct {
	ID   string `json:"id"`
	Note string `json:"note"`
}

func firstN(list []string, n int) []string {
	if len(list) > n {
		return append(list[:n:n], "…")
	}
	return list
}

func (a *App) OpenSetupFolder(id string) {
	reveal(a.home().Dir(id))
}

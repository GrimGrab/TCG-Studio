package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/game"
	"tcgstudio/internal/library"
	"tcgstudio/internal/project"
)

// Storage page (views/Storage.svelte): disk use per setup and set, and the player's actions on the shared card-art library
// (internal/library). Nothing here runs unless the player presses its button. Progress: "storage:progress" events.

// storageRun is the one storage task that may run at a time. Its label and last progress live here (not in the page), so
// the Storage page shows a running task again after the player navigates away and back.
var storageRun struct {
	sync.Mutex
	cancel context.CancelFunc
	label  string
	last   library.Progress
}

// StorageTaskInfo is the running storage task, if any.
type StorageTaskInfo struct {
	Running  bool             `json:"running"`
	Label    string           `json:"label"`
	Progress library.Progress `json:"progress"`
}

// StorageTask returns the running storage task (the Storage page reattaches to it when opened).
func (a *App) StorageTask() StorageTaskInfo {
	storageRun.Lock()
	defer storageRun.Unlock()
	return StorageTaskInfo{Running: storageRun.cancel != nil, Label: storageRun.label, Progress: storageRun.last}
}

// startStorage registers a task; the returned func ends it and emits "storage:done" (with the label).
func (a *App) startStorage(label string) (context.Context, func(), error) {
	storageRun.Lock()
	defer storageRun.Unlock()
	if storageRun.cancel != nil {
		return nil, nil, errors.New(storageRun.label + " is still running — wait for it or cancel it on the Storage page")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	storageRun.cancel, storageRun.label, storageRun.last = cancel, label, library.Progress{Message: label}
	return ctx, func() {
		storageRun.Lock()
		storageRun.cancel = nil
		storageRun.Unlock()
		cancel()
		runtime.EventsEmit(a.ctx, "storage:done", label)
	}, nil
}

func (a *App) storageProgress(p library.Progress) {
	storageRun.Lock()
	storageRun.last = p
	storageRun.Unlock()
	runtime.EventsEmit(a.ctx, "storage:progress", p)
}

// CancelStorage stops the running storage task at the next set (what's done so far is kept).
func (a *App) CancelStorage() {
	storageRun.Lock()
	defer storageRun.Unlock()
	if storageRun.cancel != nil {
		storageRun.cancel()
	}
}

func (a *App) storageReady() error {
	if a.setups.err != "" {
		return errors.New(a.setups.err)
	}
	return nil
}

// StorageReport measures the workspace, every setup and set, the game's copies and what the actions would do. Read-only.
func (a *App) StorageReport() (*library.Report, error) {
	if err := a.storageReady(); err != nil {
		return nil, err
	}
	ctx, done, err := a.startStorage("Measuring")
	if err != nil {
		return nil, err
	}
	defer done()
	return library.Analyze(ctx, a.home(), a.settings.GameDir, a.storageProgress)
}

// StorageResult is what an action did, plus a note about the game's copies.
type StorageResult struct {
	library.Result
	Reinstalled int    `json:"reinstalled"` // installed sets of the active setup brought up to date in the game
	Note        string `json:"note"`
}

// MoveEverything moves every setup's set files into the shared library and accessory/furniture files into the shared
// store, and deletes leftovers nothing refers to. Files that differ from the shared ones stay until the player chooses
// (ResolveDiffering). Then the active setup's accessories are reinstalled so the game reads the new paths.
func (a *App) MoveEverything() (*StorageResult, error) {
	return a.storageAction("Moving everything to shared", func(ctx context.Context) (*library.Result, error) {
		res, err := library.MoveAll(ctx, a.home(), a.storageProgress)
		if err == nil && game.IsGameDir(a.settings.GameDir) {
			if l, lerr := a.accLib(); lerr == nil && (len(l.Lib.Accessories) > 0 || len(l.Lib.Furniture) > 0 || len(l.Lib.Decorations) > 0) {
				a.storageProgress(library.Progress{Message: "Updating accessories in the game…"})
				err = l.Install(a.settings.GameDir)
			}
		}
		return res, err
	})
}

// ResolveDiffering applies the player's choice for a setup's set whose files differ from the shared ones: choice
// "shared" (the same art — use the shared files), "own" (different art — kept in the named shared folder name) or
// "replace" (different art — this setup's becomes the shared art for every setup using it).
func (a *App) ResolveDiffering(setupID, setID, choice, name string) (*StorageResult, error) {
	return a.storageAction("Applying your choice", func(ctx context.Context) (*library.Result, error) {
		return library.ResolveDiffering(a.home(), setupID, setID, choice, name)
	})
}

// DifferPreview shows a setup's differing file next to the shared one.
func (a *App) DifferPreview(setupID, setID string) (*library.DifferPreview, error) {
	return library.PreviewDiffer(a.home(), setupID, setID)
}

// ShrinkSets converts the PNG card art of the picked sets to JPEG: picks maps a setup id to its set ids. One task, so it
// keeps running (and stays visible) while the player is on another page.
func (a *App) ShrinkSets(picks map[string][]string) (*StorageResult, error) {
	return a.storageAction("Converting card art", func(ctx context.Context) (*library.Result, error) {
		total := &library.Result{}
		for setupID, ids := range picks {
			r, err := library.Shrink(ctx, a.home(), setupID, ids, a.storageProgress)
			if r != nil {
				total.Files += r.Files
				total.Freed += r.Freed
				total.Sets += r.Sets
			}
			if err != nil {
				return total, err
			}
		}
		return total, nil
	})
}

func (a *App) storageAction(label string, run func(context.Context) (*library.Result, error)) (*StorageResult, error) {
	a.setups.mu.Lock() // no setup switch meanwhile
	defer a.setups.mu.Unlock()
	if err := a.storageReady(); err != nil {
		return nil, err
	}
	ctx, done, err := a.startStorage(label)
	if err != nil {
		return nil, err
	}
	defer done()
	res, err := run(ctx)
	out := &StorageResult{}
	if res != nil {
		out.Result = *res
	}
	out.Reinstalled, out.Note = a.refreshInstalled()
	return out, err
}

// refreshInstalled re-installs the active setup's sets that are in the game but no longer match it (paths or files changed
// by a storage action), so the game keeps showing every card.
func (a *App) refreshInstalled() (int, string) {
	if !game.IsGameDir(a.settings.GameDir) {
		return 0, ""
	}
	list, err := a.ws().List(a.settings.GameDir)
	if err != nil {
		return 0, ""
	}
	stale := 0
	for _, s := range list {
		if s.InstallState == project.StateStale {
			stale++
		}
	}
	if stale == 0 {
		return 0, ""
	}
	if game.IsRunning() {
		return 0, "The game is running: open Sets and use \"Update all outdated\" after closing it."
	}
	n := 0
	for _, s := range list {
		if s.InstallState != project.StateStale {
			continue
		}
		a.storageProgress(library.Progress{Message: "Updating sets in the game…", Done: n, Total: stale})
		if p, err := a.ws().Load(s.ID); err == nil && a.ws().Install(p, a.settings.GameDir) == nil {
			n++
		}
	}
	return n, ""
}

// DeleteUnused deletes things no setup uses, of one kind (library.Unused* kinds; ids empty = all of that kind). The game
// folder is only touched for the game's copies (game closed). Returns bytes freed.
func (a *App) DeleteUnused(kind string, ids []string) (int64, error) {
	a.setups.mu.Lock()
	defer a.setups.mu.Unlock()
	if err := a.storageReady(); err != nil {
		return 0, err
	}
	gameDir := ""
	if kind == library.UnusedGameArt {
		if !game.IsGameDir(a.settings.GameDir) {
			return 0, errors.New("game folder not set")
		}
		if game.IsRunning() {
			return 0, errors.New("close the game first — its files are in use")
		}
		gameDir = a.settings.GameDir
	}
	catalogMu.Lock()
	defer catalogMu.Unlock()
	return library.DeleteUnused(a.home(), gameDir, kind, ids)
}

// ShrinkPreview shows one card of a set before (PNG) and after (JPEG) conversion; nothing is written.
func (a *App) ShrinkPreview(setupID, setID string) (*library.ShrinkPreview, error) {
	return library.Preview(a.home(), setupID, setID)
}

// LibraryArtInfo describes card art already in the shared library for a set about to be imported.
type LibraryArtInfo struct {
	Format string `json:"format"` // "png" or "jpg"
	Width  int    `json:"width"`  // 0 = original size
	Cards  int    `json:"cards"`  // image files there
}

// LibraryArt reports the shared library's art for set code of source (nil when there is none). The Import page asks the
// player what to do when it was made with another format or width than the one picked.
func (a *App) LibraryArt(source, code, lang string) *LibraryArtInfo {
	src, err := a.sources.Get(source)
	if err != nil {
		return nil
	}
	dir := a.ws().LibFolder(src.ProjectID(code, lang))
	m := project.LoadLibMeta(dir)
	if m == nil || m.Source != source {
		return nil
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "images"))
	if len(entries) == 0 {
		return nil
	}
	return &LibraryArtInfo{Format: m.Format, Width: m.Width, Cards: len(entries)}
}

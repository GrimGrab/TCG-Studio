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
	"tcgstudio/internal/upgrade"
)

// Storage page (views/Storage.svelte): disk use per set and setup, the player's actions on shared storage (internal/library)
// and the one-time upgrade from older layouts (internal/upgrade). Nothing here runs unless the player presses its button.
// Progress: "storage:progress" events.

// storageState is the one storage task that may run at a time. Its label and last progress live here (not in the page),
// so the Storage page shows a running task again after the player navigates away and back.
type storageState struct {
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
	a.storage.Lock()
	defer a.storage.Unlock()
	return StorageTaskInfo{Running: a.storage.cancel != nil, Label: a.storage.label, Progress: a.storage.last}
}

// startStorage registers a task; the returned func ends it and emits "storage:done" (with the label).
func (a *App) startStorage(label string) (context.Context, func(), error) {
	a.storage.Lock()
	defer a.storage.Unlock()
	if a.storage.cancel != nil {
		return nil, nil, errors.New(a.storage.label + " is still running — wait for it or cancel it on the Storage page")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.storage.cancel, a.storage.label, a.storage.last = cancel, label, library.Progress{Message: label}
	return ctx, func() {
		a.storage.Lock()
		a.storage.cancel = nil
		a.storage.Unlock()
		cancel()
		runtime.EventsEmit(a.ctx, evStorageDone, label)
	}, nil
}

func (a *App) storageProgress(p library.Progress) {
	a.storage.Lock()
	a.storage.last = p
	a.storage.Unlock()
	runtime.EventsEmit(a.ctx, evStorageProgress, p)
}

// CancelStorage stops the running storage task at the next set (what's done so far is kept).
func (a *App) CancelStorage() {
	a.storage.Lock()
	defer a.storage.Unlock()
	if a.storage.cancel != nil {
		a.storage.cancel()
	}
}

// errOldMod: the game's mod predates reading content from the workspace (and compressed save files).
var errOldMod = errors.New("the mod in the game is older than this TCG Studio — update it on the Game & mod page (Install / Repair) first")

// StorageView is the Storage page: the measurements and what "Upgrade storage" would still do (empty once a workspace
// uses shared storage only).
type StorageView struct {
	Report  *library.Report `json:"report"`
	Upgrade *upgrade.Report `json:"upgrade"`
}

// StorageReport measures the workspace, every set and setup, the game's copies and what an upgrade would do. Read-only.
func (a *App) StorageReport() (*StorageView, error) {
	if err := a.setupsErr(); err != nil {
		return nil, err
	}
	ctx, done, err := a.startStorage("Measuring")
	if err != nil {
		return nil, err
	}
	defer done()
	r, err := library.Analyze(ctx, a.home(), a.gameDir(), a.storageProgress)
	if err != nil {
		return nil, err
	}
	u, err := upgrade.Check(ctx, a.home(), a.gameDir(), a.upgradeProgress)
	return &StorageView{Report: r, Upgrade: u}, err
}

func (a *App) upgradeProgress(p upgrade.Progress) { a.storageProgress(library.Progress(p)) }

// StorageResult is what an action did, plus a note about the game's copies.
type StorageResult struct {
	Files int    `json:"files"`
	Freed int64  `json:"freed"`
	Note  string `json:"note"`
}

// UpgradeStorage brings the workspace to shared storage (internal/upgrade) with the player's choices for differing copies
// and named folders (the game reads the shared files once it is pointed at the workspace).
func (a *App) UpgradeStorage(choices upgrade.Choices) (*StorageResult, error) {
	if game.IsRunning() {
		return nil, errGameRunning
	}
	return a.storageAction("Upgrading storage", func(ctx context.Context) (int, int64, error) {
		res, err := upgrade.Run(ctx, a.home(), a.gameDir(), choices, a.upgradeProgress)
		a.pointGameAtWorkspace()
		if res == nil {
			return 0, 0, err
		}
		return res.Files, res.Freed, err
	})
}

// UpgradePreviewDiffer shows a differing copy's file next to the shared one (key = upgrade.Differ.Key).
func (a *App) UpgradePreviewDiffer(key string) (*upgrade.Preview, error) {
	return upgrade.PreviewDiffer(a.home(), key)
}

// UpgradePreviewNamed shows a named folder's sample file next to the set's shared one.
func (a *App) UpgradePreviewNamed(set, name, sample string) (*upgrade.Preview, error) {
	return upgrade.PreviewNamed(a.home(), set, name, sample)
}

// CompressSaves compresses the mod's save files that are still plain JSON (older mod versions): every setup's parked saves,
// and the game's save folder while the game is closed. The mod reads both forms; only the file size changes.
func (a *App) CompressSaves() (*StorageResult, error) {
	if !game.ModReadsLibrary(a.gameDir()) {
		return nil, errOldMod // an older mod can't read compressed side-car files
	}
	return a.storageAction("Compressing mod save files", func(ctx context.Context) (int, int64, error) {
		closed := game.IsGameDir(a.gameDir()) && !game.IsRunning()
		r, err := library.CompressSaves(ctx, a.home(), closed, a.storageProgress)
		if r == nil {
			return 0, 0, err
		}
		return r.Files, r.Freed, err
	})
}

// RemoveGameCopies deletes the copies of set and item files older Studio versions put in the game folder: the game reads
// them from the workspace now. Game closed.
func (a *App) RemoveGameCopies() (*StorageResult, error) {
	gameDir, err := a.requireGameClosed()
	if err != nil {
		return nil, err
	}
	if !game.ModReadsLibrary(gameDir) {
		return nil, errOldMod
	}
	if upgrade.GamePending(a.home(), gameDir) {
		return nil, errors.New("your setups' content isn't taken over from the game folder yet — restart TCG Studio with the game closed")
	}
	var backup string
	r, err := a.storageAction("Removing the game's copies of set files", func(ctx context.Context) (int, int64, error) {
		a.storageProgress(library.Progress{Message: "Moving the game's copies to a backup folder…"})
		moved, b, err := library.RemoveGameCopies(gameDir)
		backup = b
		return 0, moved, err
	})
	if r != nil && backup != "" {
		r.Note = "Moved to " + backup + " — delete that folder once the game shows everything."
	}
	return r, err
}

// DeleteKept deletes the files the storage upgrade kept instead of deleting (workspace.Dir.Kept), after the player has
// looked through them.
func (a *App) DeleteKept() (*StorageResult, error) {
	return a.storageAction("Deleting the kept files", func(ctx context.Context) (int, int64, error) {
		dir := a.wsDir().Kept()
		n := library.KeptSize(dir)
		return 0, n, os.RemoveAll(dir)
	})
}

// ShrinkSets converts the PNG card art of the given sets to JPEG. One task, so it keeps running (and stays visible) while
// the player is on another page.
func (a *App) ShrinkSets(ids []string) (*StorageResult, error) {
	return a.storageAction("Converting card art", func(ctx context.Context) (int, int64, error) {
		r, err := library.Shrink(ctx, a.home(), ids, a.storageProgress)
		if r == nil {
			return 0, 0, err
		}
		return r.Files, r.Freed, err
	})
}

func (a *App) storageAction(label string, run func(context.Context) (files int, freed int64, err error)) (*StorageResult, error) {
	a.setups.mu.Lock() // no setup switch meanwhile
	defer a.setups.mu.Unlock()
	if err := a.setupsErr(); err != nil {
		return nil, err
	}
	ctx, done, err := a.startStorage(label)
	if err != nil {
		return nil, err
	}
	defer done()
	out := &StorageResult{}
	out.Files, out.Freed, err = run(ctx)
	return out, err
}

// DeleteUnused deletes the picked entries of the "Not used by any setup" list, as one storage task with progress. Returns
// the bytes freed and the entries deleted.
func (a *App) DeleteUnused(picks []library.Pick) (*library.Deleted, error) {
	a.setups.mu.Lock()
	defer a.setups.mu.Unlock()
	if err := a.setupsErr(); err != nil {
		return nil, err
	}
	ctx, done, err := a.startStorage("Deleting")
	if err != nil {
		return nil, err
	}
	defer done()
	a.catalogMu.Lock()
	defer a.catalogMu.Unlock()
	return library.DeleteUnused(ctx, a.home(), picks, a.storageProgress)
}

// ShrinkPreview shows one card of a set before (PNG) and after (JPEG) conversion; nothing is written.
func (a *App) ShrinkPreview(setID string) (*library.ShrinkPreview, error) {
	return library.Preview(a.home(), setID)
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

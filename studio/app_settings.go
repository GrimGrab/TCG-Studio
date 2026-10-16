package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/game"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setups"
)

type Settings struct {
	GameDir   string `json:"gameDir"`
	Workspace string `json:"workspace"`
	// SyncedVersion = the Studio version that last brought the installed sets up to date (SyncInstalledSets).
	SyncedVersion string `json:"syncedVersion,omitempty"`
	// ImageFormat is the default card image format for imports: "png" (default) or "jpg" (about 6x smaller). The Import
	// page starts from it and can still pick the other one for a single import.
	ImageFormat string `json:"imageFormat,omitempty"`
}

func settingsPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "TCG Studio", "settings.json")
}

// cacheDir is Studio's cache folder (game templates, mod defaults): rebuilt when missing.
func cacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "TCG Studio")
}

func loadSettings() Settings {
	var s Settings
	if b, err := os.ReadFile(settingsPath()); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

func saveSettings(s Settings) error {
	if err := os.MkdirAll(filepath.Dir(settingsPath()), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath(), b, 0o644)
}

// cfg is a copy of the settings. Bound methods run concurrently (and some start goroutines), so the settings are only
// read through cfg / gameDir / workspaceDir and changed through updateSettings.
func (a *App) cfg() Settings {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	return a.settings
}

func (a *App) gameDir() string      { return a.cfg().GameDir }
func (a *App) workspaceDir() string { return a.cfg().Workspace }

// updateSettings changes the settings under the lock and saves them; returns the new settings.
func (a *App) updateSettings(change func(*Settings)) (Settings, error) {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	change(&a.settings)
	return a.settings, saveSettings(a.settings)
}

func (a *App) GetSettings() Settings { return a.cfg() }

func (a *App) SaveSettings(s Settings) (Settings, error) {
	old := a.cfg()
	if s.GameDir != "" && !game.IsGameDir(s.GameDir) {
		return old, errors.New("that folder is not a TCG Card Shop Simulator install")
	}
	if s.Workspace == "" {
		s.Workspace = project.DefaultRoot()
	}
	saved, err := a.updateSettings(func(cur *Settings) {
		s.SyncedVersion = cur.SyncedVersion // backend-only
		*cur = s
	})
	if saved.GameDir != old.GameDir {
		a.gameDirChanged()
	}
	if saved.Workspace != old.Workspace {
		a.initSetups()
	}
	return saved, err
}

// gameDirChanged reads the new game's templates if needed and points the game at the workspace.
func (a *App) gameDirChanged() {
	a.RefreshTemplates(false)
	if game.IsGameDir(a.gameDir()) {
		a.pointGameAtWorkspace()
	}
}

func (a *App) GameStatus() game.Status {
	s := game.GetStatus(a.gameDir())
	s.TemplatesFound = a.templatesReady()
	return s
}

func (a *App) LocateGame() game.Status {
	if dir := game.Locate(); dir != "" {
		changed := dir != a.gameDir()
		_, _ = a.updateSettings(func(s *Settings) { s.GameDir = dir })
		if changed {
			a.gameDirChanged()
		}
	}
	return a.GameStatus()
}

func (a *App) BrowseGameFolder() (game.Status, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Select the TCG Card Shop Simulator folder"})
	if err != nil || dir == "" {
		return a.GameStatus(), err
	}
	s := a.cfg()
	s.GameDir = dir
	_, err = a.SaveSettings(s)
	return a.GameStatus(), err
}

// WorkspaceHasSetups reports whether a folder already is a TCG Studio workspace (the Settings page asks before switching to
// an empty one: the game then reads that workspace, so it shows none of the current sets until they're moved there).
func (a *App) WorkspaceHasSetups(dir string) bool {
	return dir != "" && (fsutil.Exists(filepath.Join(dir, setups.RegistryFile)) || fsutil.Exists(filepath.Join(dir, "projects")))
}

// LibraryFolder is the workspace's shared library (every set's files): the game's mod reads set files from there.
func (a *App) LibraryFolder() string { return a.wsDir().Library() }

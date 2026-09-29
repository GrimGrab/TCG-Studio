package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/game"
	"tcgstudio/internal/gameextract"
)

// Game templates: the vanilla art, meshes and item data the editors build on. Studio reads them from the installed game's
// files into its own cache (internal/gameextract), so nothing has to be run in game first. Mod versions up to 0.7.x
// exported them in game (<plugin>\templates); such a leftover export is only used until Studio's own read is done
// (Setup → Install / Repair deletes it).

// TemplatesStatus is shown in Settings and by the editors.
type TemplatesStatus struct {
	Ready   bool   `json:"ready"`
	Source  string `json:"source"` // "game" (read from the game files) | "mod" (an old in-game export, until then) | ""
	Busy    bool   `json:"busy"`
	Message string `json:"message"` // progress while busy
	Error   string `json:"error"`
}

type templatesState struct {
	mu      sync.Mutex
	busy    bool
	message string
	err     string
	dir     string // resolved folder ("" = not resolved yet)
	gameDir string // game folder dir was resolved for
}

func templatesCacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "TCG Studio", "game-templates")
}

// cacheCurrent: Studio's extraction exists, was made by this extractor version and from these game files.
func cacheCurrent(gameDir string) bool {
	b, err := os.ReadFile(filepath.Join(templatesCacheDir(), "studio.json"))
	if err != nil {
		return false
	}
	var info gameextract.Info
	if json.Unmarshal(b, &info) != nil || info.Extractor != gameextract.Version {
		return false
	}
	stamp, err := gameextract.Stamp(gameDir)
	return err == nil && info.Stamp == stamp
}

func resolveTemplates(gameDir string) (dir, source string) {
	switch {
	case gameDir == "" || !game.IsGameDir(gameDir):
		return "", ""
	case cacheCurrent(gameDir):
		return templatesCacheDir(), "game"
	case exists(filepath.Join(game.TemplatesDir(gameDir), "BasicCardPack_texture.png")):
		return game.TemplatesDir(gameDir), "mod" // leftover in-game export while Studio reads the game files
	}
	return "", ""
}

// templatesDir is the folder the editors read templates from (see resolveTemplates; cached per game folder).
func (a *App) templatesDir() string {
	t := &a.tpl
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.dir == "" || t.gameDir != a.settings.GameDir {
		t.dir, _ = resolveTemplates(a.settings.GameDir)
		t.gameDir = a.settings.GameDir
	}
	return t.dir
}

func (a *App) templatesReady() bool {
	dir := a.templatesDir()
	return dir != "" && exists(filepath.Join(dir, "BasicCardPack_texture.png"))
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// TemplatesStatus reports where the templates come from and whether Studio is reading them.
func (a *App) TemplatesStatus() TemplatesStatus {
	dir := a.templatesDir()
	_, source := resolveTemplates(a.settings.GameDir)
	t := &a.tpl
	t.mu.Lock()
	defer t.mu.Unlock()
	s := TemplatesStatus{Busy: t.busy, Message: t.message, Error: t.err, Source: source}
	s.Ready = dir != "" && exists(filepath.Join(dir, "BasicCardPack_texture.png")) && exists(filepath.Join(dir, "accessories", "accessories.json"))
	if !s.Ready {
		s.Source = ""
	}
	return s
}

// RefreshTemplates reads the templates from the game files unless Studio's copy is current (force: always). Runs in the
// background; the frontend gets "templates:progress" and "templates:ready".
func (a *App) RefreshTemplates(force bool) TemplatesStatus {
	gameDir := a.settings.GameDir
	t := &a.tpl
	t.mu.Lock()
	if t.busy || gameDir == "" || !game.IsGameDir(gameDir) || (!force && cacheCurrent(gameDir)) {
		t.mu.Unlock()
		return a.TemplatesStatus()
	}
	t.busy, t.message, t.err = true, "Reading the game files…", ""
	t.mu.Unlock()
	go func() {
		_, err := gameextract.Extract(gameDir, templatesCacheDir(), func(msg string) {
			t.mu.Lock()
			t.message = msg
			t.mu.Unlock()
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "templates:progress", msg)
			}
		})
		t.mu.Lock()
		t.busy, t.message, t.dir = false, "", ""
		if err != nil {
			t.err = "Couldn't read the game files: " + err.Error()
		}
		t.mu.Unlock()
		// The game folder changed while this one was being read: read the new one.
		if a.settings.GameDir != gameDir {
			a.RefreshTemplates(false)
			return
		}
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "templates:ready", a.TemplatesStatus())
		}
	}()
	return a.TemplatesStatus()
}

// invalidateTemplates forgets the resolved folder (game folder changed) and reads the new game's files if needed.
func (a *App) invalidateTemplates() {
	a.tpl.mu.Lock()
	a.tpl.dir = ""
	a.tpl.mu.Unlock()
	a.RefreshTemplates(false)
}

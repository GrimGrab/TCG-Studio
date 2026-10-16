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
// (Game & mod → Install / Repair deletes it).

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
	// resolveTemplates' answer for resolvedFor, kept until the game folder changes or a read of the game files ends.
	resolved    bool
	resolvedFor string
	dir, source string
}

func templatesCacheDir() string { return filepath.Join(cacheDir(), "game-templates") }

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

// templates is resolveTemplates for the current game folder, cached (it reads the game files' stamp).
func (a *App) templates() (dir, source string) {
	gameDir := a.gameDir()
	t := &a.tpl
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.resolved || t.resolvedFor != gameDir {
		t.dir, t.source = resolveTemplates(gameDir)
		t.resolved, t.resolvedFor = true, gameDir
	}
	return t.dir, t.source
}

// templatesDir is the folder the editors read templates from ("" = none yet).
func (a *App) templatesDir() string {
	dir, _ := a.templates()
	return dir
}

// templateFile is a file of the game templates (parts joined, kept inside the folder), or "" while there are none.
func (a *App) templateFile(parts ...string) string { return safeJoin(a.templatesDir(), parts...) }

// templateJSON returns a JSON file of the game templates as text, or "" while it isn't available.
func (a *App) templateJSON(parts ...string) string {
	b, err := os.ReadFile(a.templateFile(parts...))
	if err != nil {
		return ""
	}
	return string(b)
}

// templatesReadyIn: dir holds a complete set of templates (the editors need both).
func templatesReadyIn(dir string) bool {
	return dir != "" && exists(filepath.Join(dir, "BasicCardPack_texture.png")) &&
		exists(filepath.Join(dir, "accessories", "accessories.json"))
}

func (a *App) templatesReady() bool { return templatesReadyIn(a.templatesDir()) }

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// TemplatesStatus reports where the templates come from and whether Studio is reading them.
func (a *App) TemplatesStatus() TemplatesStatus {
	dir, source := a.templates()
	s := TemplatesStatus{Ready: templatesReadyIn(dir)}
	if s.Ready {
		s.Source = source
	}
	t := &a.tpl
	t.mu.Lock()
	s.Busy, s.Message, s.Error = t.busy, t.message, t.err
	t.mu.Unlock()
	return s
}

// RefreshTemplates reads the templates from the game files unless Studio's copy is current (force: always). Runs in the
// background; the frontend gets "templates:progress" and "templates:ready".
func (a *App) RefreshTemplates(force bool) TemplatesStatus {
	gameDir := a.gameDir()
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
				runtime.EventsEmit(a.ctx, evTemplatesProgress, msg)
			}
		})
		t.mu.Lock()
		t.busy, t.message, t.resolved = false, "", false
		if err != nil {
			t.err = "Couldn't read the game files: " + err.Error()
		}
		t.mu.Unlock()
		// The game folder changed while this one was being read: read the new one.
		if a.gameDir() != gameDir {
			a.RefreshTemplates(false)
			return
		}
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, evTemplatesReady, a.TemplatesStatus())
		}
	}()
	return a.TemplatesStatus()
}

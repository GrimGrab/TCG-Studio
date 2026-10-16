package main

import (
	"encoding/base64"
	"errors"
	"strings"

	"tcgstudio/internal/game"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

var (
	errNoGame      = errors.New("game folder not set (Settings → Game)")
	errGameRunning = errors.New("close the game first — its files are in use")
	errNoTemplates = errors.New("the game templates aren't available yet — check Settings → Game (TCG Studio reads them from the game folder)")
	errBadID       = errors.New("bad project id")
)

// requireGame returns the game folder, or errNoGame.
func (a *App) requireGame() (string, error) {
	dir := a.gameDir()
	if !game.IsGameDir(dir) {
		return "", errNoGame
	}
	return dir, nil
}

// requireGameClosed returns the game folder when it is set and the game isn't running.
func (a *App) requireGameClosed() (string, error) {
	dir, err := a.requireGame()
	if err == nil && game.IsRunning() {
		err = errGameRunning
	}
	return dir, err
}

// requireTemplates returns the game templates folder, or errNoTemplates.
func (a *App) requireTemplates() (string, error) {
	dir := a.templatesDir()
	if !templatesReadyIn(dir) {
		return "", errNoTemplates
	}
	return dir, nil
}

// loadProject loads a set project of the active setup; id comes from the frontend, so it is checked first.
func (a *App) loadProject(id string) (*project.Project, error) {
	if !setfmt.SafeID(id) {
		return nil, errBadID
	}
	return a.ws().Load(id)
}

// saveSet saves a project. The game reads an installed set's saved project itself, so that's all it takes; returns
// whether the set is installed (the caller then says the game shows the change after a restart).
func (a *App) saveSet(p *project.Project) (bool, error) {
	if err := a.ws().Save(p); err != nil {
		return false, err
	}
	return a.isInstalled(p.ID), nil
}

// safeFileName makes s usable as a Windows file name (for save dialogs' suggested names).
func safeFileName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?* `, r) {
			return '_'
		}
		return r
	}, s)
	if s == "" {
		s = "model"
	}
	return s
}

// decodeDataURL decodes an image the frontend sends as a data URL (or bare base64).
func decodeDataURL(s string) ([]byte, error) {
	if i := strings.Index(s, ","); strings.HasPrefix(s, "data:") && i >= 0 {
		s = s[i+1:]
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, errors.New("bad image data: " + err.Error())
	}
	return b, nil
}

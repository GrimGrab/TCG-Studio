package migrate

import (
	"errors"
	"os"
	"path/filepath"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/catalog"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/game"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

// migration:old-layout-check
// OldLayout reports whether a workspace still has content where TCG Studio before shared storage (v1.9.0) kept it: card art
// or item files in project and setup folders, a named art folder, or content only in the game folder. This version doesn't
// move it: TCG Studio v1.14.0 does, once opened, and the files stay untouched until then. The first find ends the search.
func OldLayout(h setups.Home, gameDir string) bool {
	list, err := h.List()
	if err != nil {
		return false
	}
	if gameContentNotTaken(h, gameDir, list) {
		return true
	}
	for _, root := range append(setups.Folders(list), catalog.For(h.Root).Root) {
		if hasFiles(filepath.Join(root, "accessories", "images")) || projectHasImages(workspace.Dir(h.Root).Projects(root)) {
			return true
		}
	}
	lib := workspace.Dir(h.Root).Library()
	sets, _ := os.ReadDir(lib)
	for _, s := range sets {
		dirs, _ := os.ReadDir(filepath.Join(lib, s.Name()))
		for _, d := range dirs {
			if d.IsDir() && d.Name() != "images" {
				return true
			}
		}
	}
	return false
}

// gameContentNotTaken: no setup was ever marked as having taken over the game folder's content, and there is some. Any one
// mark means the upgrade ran; later setups never get one, and the game's copies are then only leftovers.
func gameContentNotTaken(h setups.Home, gameDir string, list []setups.Summary) bool {
	dirs := []string{}
	if game.IsGameDir(gameDir) {
		dirs = append(dirs, game.SetsDir(gameDir), accessories.InstalledDir(gameDir))
	}
	for _, s := range list {
		if in, err := setups.LoadInfo(h.Dir(s.ID)); err == nil && in.AdoptedGame {
			return false
		}
		parked := filepath.Join(h.Dir(s.ID), setups.GameStateDir)
		dirs = append(dirs, filepath.Join(parked, "Sets"), filepath.Join(parked, accessories.InstallDir))
	}
	for _, d := range dirs {
		if hasFiles(d) {
			return true
		}
	}
	return false
}

// projectHasImages: an image in a project folder (only of projects Studio can read).
func projectHasImages(ws project.Workspace) bool {
	projects, _ := os.ReadDir(ws.ProjectsDir())
	for _, e := range projects {
		folder := filepath.Join(ws.ProjectsDir(), e.Name())
		if !e.IsDir() {
			continue
		}
		if _, err := setfmt.Load(filepath.Join(folder, project.SetFile)); err != nil {
			continue
		}
		found := errors.New("found")
		if filepath.WalkDir(folder, func(_ string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && fsutil.IsImage(d.Name()) {
				return found
			}
			return nil
		}) == found {
			return true
		}
	}
	return false
}

// hasFiles reports whether dir holds at least one file (an empty leftover folder doesn't count).
func hasFiles(dir string) bool {
	found := errors.New("found")
	return filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			return found
		}
		return nil
	}) == found
}

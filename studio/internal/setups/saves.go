package setups

import (
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/game"
)

// SaveDir is the game's save folder; a variable so tests can point it at a temp folder.
var SaveDir = game.SaveDir

// sideCarDir is the mod's per-slot side-car folder inside SaveDir (slot{n}.json, .bak, …).
const sideCarDir = "TCGCustomCards"

// isSaveFile matches the vanilla per-slot saves: savedGames_Release{n}.json/.gd and savedGames_ReleaseBackupFile{n}.*.
// Key bindings (savedGames_KeybindSetting.gd), the cloud file, logs and PlayerPrefs stay shared between setups.
func isSaveFile(name string) bool { return strings.HasPrefix(name, "savedGames_Release") }

// HasSaves reports whether dir holds any game save (vanilla file or side-car).
func HasSaves(dir string) bool {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if (!e.IsDir() && isSaveFile(e.Name())) || (e.IsDir() && e.Name() == sideCarDir && !dirEmpty(filepath.Join(dir, sideCarDir))) {
			return true
		}
	}
	return false
}

// moveSaves moves the saves from src to dst (files in dst with the same names are replaced).
func moveSaves(src, dst string) error {
	entries, err := os.ReadDir(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		switch {
		case !e.IsDir() && isSaveFile(e.Name()):
			if err := moveFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		case e.IsDir() && e.Name() == sideCarDir:
			if err := moveTree(filepath.Join(src, sideCarDir), filepath.Join(dst, sideCarDir)); err != nil {
				return err
			}
		}
	}
	return nil
}

// copySaves copies the saves from src to dst.
func copySaves(src, dst string) error {
	entries, err := os.ReadDir(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		switch {
		case !e.IsDir() && isSaveFile(e.Name()):
			if err := copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		case e.IsDir() && e.Name() == sideCarDir:
			if err := copyTree(filepath.Join(src, sideCarDir), filepath.Join(dst, sideCarDir), nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// SteamCloud reports whether Steam Auto-Cloud syncs the save folder (Steam writes steam_autocloud.vdf there). Steam may then
// download saves that were moved away back into the folder on the next game start.
func SteamCloud() bool { return exists(filepath.Join(SaveDir(), "steam_autocloud.vdf")) }

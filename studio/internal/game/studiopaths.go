package game

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// StudioPathsFile, next to the mod DLL, names the Studio workspace folders the mod reads all content from — the game folder
// holds only the mod: the active setup (its installed sets' set.json, accessory library, card back, Game look pictures),
// every set's files (<library>\<set id>) and accessory / furniture / decoration files ("assets/<hash>.<ext>", <assets>).
// Not a mod setting: Studio writes it (Core/StudioPaths.cs reads it at game start).
const StudioPathsFile = "studio-paths.json"

// StudioPaths are the folders in StudioPathsFile.
type StudioPaths struct {
	Library string `json:"library"` // <workspace>\library
	Assets  string `json:"assets"`  // <workspace>\assets
	Setup   string `json:"setup"`   // the active setup's folder: its installed sets, accessory library, card back, Game look
}

// ReadStudioPaths is the game's StudioPathsFile (empty fields when it is missing).
func ReadStudioPaths(gameDir string) StudioPaths {
	var p StudioPaths
	if b, err := os.ReadFile(filepath.Join(PluginDir(gameDir), StudioPathsFile)); err == nil {
		_ = json.Unmarshal(b, &p)
	}
	return p
}

// LibraryPath is the library folder the game's mod reads set files from ("" = none set).
func LibraryPath(gameDir string) string { return ReadStudioPaths(gameDir).Library }

// SameDir reports whether two folder paths name the same folder (Windows: case-insensitive).
func SameDir(a, b string) bool {
	return a != "" && b != "" && strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// UpdateStudioPaths changes the game's StudioPathsFile (written only when something changed).
func UpdateStudioPaths(gameDir string, change func(*StudioPaths)) error {
	old := ReadStudioPaths(gameDir)
	p := old
	change(&p)
	same := func(a, b string) bool { return (a == "" && b == "") || SameDir(a, b) }
	if same(p.Library, old.Library) && same(p.Assets, old.Assets) && same(p.Setup, old.Setup) &&
		exists(filepath.Join(PluginDir(gameDir), StudioPathsFile)) {
		return nil
	}
	if err := os.MkdirAll(PluginDir(gameDir), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(PluginDir(gameDir), StudioPathsFile), b, 0o644)
}

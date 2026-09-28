// Package game finds the TCG Card Shop Simulator install (via Steam) and installs sets into the mod folder.
package game

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const gameFolder = "TCG Card Shop Simulator"

// Locate returns the game install folder, or "" if not found.
func Locate() string {
	for _, lib := range steamLibraries() {
		p := filepath.Join(lib, "steamapps", "common", gameFolder)
		if IsGameDir(p) {
			return p
		}
	}
	return ""
}

// IsGameDir reports whether dir looks like the game install.
func IsGameDir(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "Card Shop Simulator_Data"))
	return err == nil
}

// PluginDir is where the mod lives; SetsDir holds installed sets.
func PluginDir(gameDir string) string {
	return filepath.Join(gameDir, "BepInEx", "plugins", "TCGCustomCards")
}
func SetsDir(gameDir string) string { return filepath.Join(PluginDir(gameDir), "Sets") }
func TemplatesDir(gameDir string) string {
	return filepath.Join(PluginDir(gameDir), "templates")
}

// Status describes what is installed in the game folder.
type Status struct {
	GameDir        string `json:"gameDir"`
	Found          bool   `json:"found"`
	BepInEx        bool   `json:"bepInEx"`
	ModInstalled   bool   `json:"modInstalled"`
	TemplatesFound bool   `json:"templatesFound"`
}

func GetStatus(gameDir string) Status {
	s := Status{GameDir: gameDir, Found: gameDir != "" && IsGameDir(gameDir)}
	if !s.Found {
		return s
	}
	s.BepInEx = exists(filepath.Join(gameDir, "BepInEx", "core"))
	s.ModInstalled = exists(filepath.Join(PluginDir(gameDir), "TCGCustomCards.dll"))
	s.TemplatesFound = exists(filepath.Join(TemplatesDir(gameDir), "BasicCardPack_texture.png"))
	return s
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

// ExeName is the game's process image name.
const ExeName = "Card Shop Simulator.exe"

// DefaultAppID is the game's Steam app id (used when no appmanifest can be read).
const DefaultAppID = "3070070"

// IsRunning reports whether the game process is running (files can't be replaced while it is).
func IsRunning() bool {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+ExeName, "/NH", "/FO", "CSV")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	return err == nil && strings.Contains(strings.ToLower(string(out)), strings.ToLower(ExeName))
}

var acfInstallDir = regexp.MustCompile(`"installdir"\s+"([^"]+)"`)
var acfAppID = regexp.MustCompile(`"appid"\s+"(\d+)"`)

// SteamAppID reads the app id from the Steam library's appmanifest that owns gameDir (…\steamapps\common\<installdir>).
func SteamAppID(gameDir string) string {
	steamapps := filepath.Dir(filepath.Dir(gameDir))
	want := strings.ToLower(filepath.Base(gameDir))
	files, _ := filepath.Glob(filepath.Join(steamapps, "appmanifest_*.acf"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if m := acfInstallDir.FindSubmatch(b); m != nil && strings.ToLower(string(m[1])) == want {
			if id := acfAppID.FindSubmatch(b); id != nil {
				return string(id[1])
			}
		}
	}
	return DefaultAppID
}

var vdfPath = regexp.MustCompile(`"path"\s+"([^"]+)"`)

func steamLibraries() []string {
	var roots []string
	if k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE); err == nil {
		if p, _, err := k.GetStringValue("SteamPath"); err == nil {
			roots = append(roots, filepath.FromSlash(p))
		}
		k.Close()
	}
	roots = append(roots, `C:\Program Files (x86)\Steam`, `C:\Program Files\Steam`)

	seen := map[string]bool{}
	var libs []string
	addLib := func(p string) {
		p = filepath.Clean(p)
		if !seen[strings.ToLower(p)] {
			seen[strings.ToLower(p)] = true
			libs = append(libs, p)
		}
	}
	for _, root := range roots {
		addLib(root)
		f, err := os.Open(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if m := vdfPath.FindStringSubmatch(sc.Text()); m != nil {
				addLib(strings.ReplaceAll(m[1], `\\`, `\`))
			}
		}
		f.Close()
	}
	// Common manual library locations as a last resort.
	for _, d := range "CDEFGH" {
		addLib(fmt.Sprintf(`%c:\SteamLibrary`, d))
	}
	return libs
}

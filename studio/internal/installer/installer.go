package installer

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Paths inside the game folder.
const (
	pluginRel   = "BepInEx/plugins/TCGCustomCards"
	modDLLRel   = pluginRel + "/TCGCustomCards.dll"
	bundleRel   = pluginRel + "/tcgcc_foil"
	bridgeRel   = pluginRel + "/tcgcc-forge-bridge.jar"
	cfgMgrRel   = "BepInEx/plugins/ConfigurationManager/ConfigurationManager.dll"
	doorstopIni = "doorstop_config.ini"
	proxyDLL    = "winhttp.dll"

	legacyTemplatesRel = pluginRel + "/templates" // in-game template export of mod versions up to 0.7.x
)

// Item states shown on the Setup screen.
const (
	OK       = "ok"
	Missing  = "missing"
	Outdated = "outdated"
	Conflict = "conflict"
	Blocked  = "blocked"
	Info     = "info"
)

type Item struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	State   string `json:"state"`
	Message string `json:"message"`
}

type State struct {
	GameDir string `json:"gameDir"`
	Found   bool   `json:"found"`
	Running bool   `json:"running"`
	Ready   bool   `json:"ready"` // everything needed is installed and current
	Items   []Item `json:"items"`
	Version string `json:"version"`
	Payload bool   `json:"payload"` // this build can install
}

// loader describes what mod loader is in the game folder.
type loader struct {
	bep5, bep6, melon, broken bool
	problems                  []string
}

func abs(game, rel string) string { return filepath.Join(game, filepath.FromSlash(rel)) }

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func sameFile(path string, want []byte) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return sha256.Sum256(b) == sha256.Sum256(want)
}

var doorstopEnabled = regexp.MustCompile(`(?mi)^\s*enabled\s*=\s*(\w+)`)
var doorstopTarget = regexp.MustCompile(`(?mi)^\s*target_assembly\s*=\s*(.+)$`)

func detectLoader(game string) loader {
	var l loader
	core := abs(game, "BepInEx/core")
	l.bep6 = exists(filepath.Join(core, "BepInEx.Core.dll")) || exists(filepath.Join(core, "BepInEx.Unity.Mono.dll")) ||
		exists(filepath.Join(core, "BepInEx.Unity.IL2CPP.dll"))
	l.melon = exists(abs(game, "MelonLoader")) || exists(abs(game, "version.dll"))
	hasCore := exists(filepath.Join(core, "BepInEx.dll"))
	if !hasCore || l.bep6 {
		return l
	}
	if !exists(filepath.Join(core, "BepInEx.Preloader.dll")) {
		l.problems = append(l.problems, "BepInEx.Preloader.dll is missing")
	}
	if !exists(abs(game, proxyDLL)) {
		l.problems = append(l.problems, "winhttp.dll (the loader hook) is missing")
	}
	if b, err := os.ReadFile(abs(game, doorstopIni)); err != nil {
		l.problems = append(l.problems, "doorstop_config.ini is missing")
	} else {
		if m := doorstopEnabled.FindSubmatch(b); m == nil || strings.ToLower(string(m[1])) != "true" {
			l.problems = append(l.problems, "the loader is disabled in doorstop_config.ini")
		}
		if m := doorstopTarget.FindSubmatch(b); m == nil || !strings.Contains(strings.ToLower(string(m[1])), "bepinex.preloader.dll") {
			l.problems = append(l.problems, "doorstop_config.ini doesn't point at BepInEx")
		}
	}
	l.broken = len(l.problems) > 0
	l.bep5 = !l.broken
	return l
}

// strayModCopies finds TCGCustomCards.dll anywhere under plugins except our folder (would load the mod twice).
func strayModCopies(game string) []string {
	var out []string
	root := abs(game, "BepInEx/plugins")
	own := strings.ToLower(abs(game, modDLLRel))
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(d.Name(), "TCGCustomCards.dll") && strings.ToLower(p) != own {
			rel, _ := filepath.Rel(game, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

// Inspect reports what is installed and what Repair would do.
func Inspect(game string, p Payload, havePayload, running bool) State {
	s := State{GameDir: game, Running: running, Version: p.Version, Payload: havePayload}
	s.Found = game != "" && exists(abs(game, "Card Shop Simulator_Data"))
	if !s.Found {
		s.Items = append(s.Items, Item{"game", "Game folder", Missing, "TCG Card Shop Simulator wasn't found — pick its folder."})
		return s
	}
	add := func(id, title, state, msg string) { s.Items = append(s.Items, Item{id, title, state, msg}) }
	if running {
		add("running", "Game closed", Blocked, "The game is running — close it so files can be installed.")
	}

	l := detectLoader(game)
	switch {
	case l.melon:
		add("loader", "Mod loader (BepInEx 5)", Conflict, "MelonLoader is installed; it will be backed up and replaced by BepInEx 5.")
	case l.bep6:
		add("loader", "Mod loader (BepInEx 5)", Conflict, "BepInEx 6 is installed; this mod needs BepInEx 5 — it will be backed up and replaced.")
	case l.broken:
		add("loader", "Mod loader (BepInEx 5)", Outdated, "BepInEx is incomplete: "+strings.Join(l.problems, "; ")+". It will be repaired.")
	case l.bep5:
		add("loader", "Mod loader (BepInEx 5)", OK, "BepInEx 5 is installed.")
	default:
		add("loader", "Mod loader (BepInEx 5)", Missing, "BepInEx 5 will be installed.")
	}

	if exists(abs(game, cfgMgrRel)) || findFile(abs(game, "BepInEx/plugins"), "ConfigurationManager.dll") {
		add("configmgr", "Settings menu (F1)", OK, "ConfigurationManager is installed.")
	} else {
		add("configmgr", "Settings menu (F1)", Missing, "ConfigurationManager (the in-game F1 settings menu) will be installed.")
	}

	switch {
	case !exists(abs(game, modDLLRel)):
		add("mod", "TCG Custom Cards mod", Missing, "The mod will be installed.")
	case !havePayload:
		add("mod", "TCG Custom Cards mod", OK, "The mod is installed (this build can't check its version).")
	case !sameFile(abs(game, modDLLRel), p.ModDLL) || (len(p.Bundle) > 0 && !sameFile(abs(game, bundleRel), p.Bundle)) ||
		(len(p.Bridge) > 0 && !sameFile(abs(game, bridgeRel), p.Bridge)):
		add("mod", "TCG Custom Cards mod", Outdated, "A different version of the mod is installed; it will be updated.")
	default:
		add("mod", "TCG Custom Cards mod", OK, "The mod is installed and up to date.")
	}
	if stray := strayModCopies(game); len(stray) > 0 {
		add("stray", "Duplicate mod copies", Conflict, "Extra copies would load the mod twice and will be backed up: "+strings.Join(stray, ", "))
	}

	if exists(abs(game, legacyTemplatesRel)) {
		add("templates", "Old art templates", Outdated, "Exported in game by an older mod version and no longer used (TCG Studio reads the game files) — Install / Repair removes them.")
	}
	sets, _ := os.ReadDir(abs(game, pluginRel+"/Sets"))
	n := 0
	for _, e := range sets {
		if e.IsDir() {
			n++
		}
	}
	add("sets", "Card sets", Info, fmt.Sprintf("%d set(s) installed. Import sets from Scryfall in My Sets / Import, then Install.", n))

	s.Ready = true
	for _, it := range s.Items {
		if it.State != OK && it.State != Info {
			s.Ready = false
		}
	}
	return s
}

func findFile(root, name string) bool {
	found := false
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(d.Name(), name) {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// session tracks one Repair/Uninstall run: lazily created backup folder + a log.
type session struct {
	game   string
	backup string
	log    func(string)
	lines  []string
}

func newSession(game string, log func(string)) *session {
	return &session{game: game, log: log}
}

func (s *session) say(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	s.lines = append(s.lines, msg)
	if s.log != nil {
		s.log(msg)
	}
}

// moveToBackup moves a game-relative file/folder into TCGStudio_backup_<time>/ (same relative path).
func (s *session) moveToBackup(rel string) error {
	src := abs(s.game, rel)
	if !exists(src) {
		return nil
	}
	if s.backup == "" {
		base := filepath.Join(s.game, "TCGStudio_backup_"+time.Now().Format("20060102-150405"))
		s.backup = base
		for i := 2; exists(s.backup); i++ { // another run in the same second
			s.backup = fmt.Sprintf("%s-%d", base, i)
		}
	}
	dst := filepath.Join(s.backup, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("backing up %s: %w", rel, err)
	}
	s.say("Backed up %s", rel)
	return nil
}

// writeFile writes a game-relative file, backing up a different existing file first.
func (s *session) writeFile(rel string, data []byte) error {
	dst := abs(s.game, rel)
	if sameFile(dst, data) {
		return nil
	}
	if exists(dst) {
		if err := s.moveToBackup(rel); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", rel, err)
	}
	s.say("Installed %s", rel)
	return nil
}

// extract writes the BepInEx package. keep(rel) = true leaves an existing file alone (user configs, other plugins' data).
func (s *session) extract(zipData []byte, include func(rel string) bool, keep func(rel string) bool) error {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("reading BepInEx package: %w", err)
	}
	for _, f := range zr.File {
		rel := strings.TrimPrefix(filepath.ToSlash(f.Name), "/")
		if f.FileInfo().IsDir() || rel == "" || strings.Contains(rel, "..") || !include(rel) {
			continue
		}
		if keep != nil && keep(rel) && exists(abs(s.game, rel)) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		if err := s.writeFile(rel, data); err != nil {
			return err
		}
	}
	return nil
}

// Repair installs/repairs everything that isn't OK. Other plugins and existing configs are kept.
func Repair(game string, p Payload, running bool, log func(string)) (backupDir string, err error) {
	if !exists(abs(game, "Card Shop Simulator_Data")) {
		return "", fmt.Errorf("that folder isn't a TCG Card Shop Simulator install")
	}
	if running {
		return "", fmt.Errorf("close the game first — its files are in use")
	}
	s := newSession(game, log)
	defer func() { s.writeLog(err) }()

	l := detectLoader(game)
	if l.melon {
		s.say("MelonLoader found — moving it to the backup folder")
		for _, rel := range []string{"version.dll", "MelonLoader"} {
			if err = s.moveToBackup(rel); err != nil {
				return s.backup, err
			}
		}
	}
	if l.bep6 {
		s.say("BepInEx 6 found — moving its core to the backup folder")
		if err = s.moveToBackup("BepInEx/core"); err != nil {
			return s.backup, err
		}
	}
	if !l.bep5 {
		s.say("Installing BepInEx 5")
		// Core, loader hook and doorstop config from the package; keep the user's BepInEx configs / cache if present.
		// Skipped: plugins (handled below), BepInEx's cache (rebuilt on launch) and its changelog.txt (not needed in the game root).
		err = s.extract(p.BepInExZip,
			func(rel string) bool {
				return !strings.HasPrefix(rel, "BepInEx/plugins/") && !strings.HasPrefix(rel, "BepInEx/cache/") && rel != "changelog.txt"
			},
			func(rel string) bool { return strings.HasPrefix(rel, "BepInEx/config/") })
		if err != nil {
			return s.backup, err
		}
		if err = s.fixDoorstop(p.BepInExZip); err != nil {
			return s.backup, err
		}
	}
	if !(exists(abs(game, cfgMgrRel)) || findFile(abs(game, "BepInEx/plugins"), "ConfigurationManager.dll")) {
		s.say("Installing ConfigurationManager (F1 menu)")
		if err = s.extract(p.BepInExZip,
			func(rel string) bool { return strings.HasPrefix(rel, "BepInEx/plugins/ConfigurationManager/") },
			nil); err != nil {
			return s.backup, err
		}
	}
	for _, rel := range strayModCopies(game) {
		if err = s.moveToBackup(rel); err != nil {
			return s.backup, err
		}
	}
	if err = s.writeFile(modDLLRel, p.ModDLL); err != nil {
		return s.backup, err
	}
	if len(p.Bundle) > 0 {
		if err = s.writeFile(bundleRel, p.Bundle); err != nil {
			return s.backup, err
		}
	}
	if len(p.Bridge) > 0 {
		if err = s.writeFile(bridgeRel, p.Bridge); err != nil {
			return s.backup, err
		}
	}
	if exists(abs(game, pluginRel+"/TCGCustomCards.pdb")) {
		if err = s.moveToBackup(pluginRel + "/TCGCustomCards.pdb"); err != nil {
			return s.backup, err
		}
	}
	// Older mod versions exported art templates in game; TCG Studio now reads them from the game files. Generated data, so it
	// is deleted rather than backed up.
	if exists(abs(game, legacyTemplatesRel)) {
		s.say("Removing the old in-game template export (no longer used)")
		if err = os.RemoveAll(abs(game, legacyTemplatesRel)); err != nil {
			return s.backup, err
		}
	}
	if len(s.lines) == 0 {
		s.say("Everything was already installed — nothing to do")
	} else {
		s.say("Done")
	}
	return s.backup, nil
}

// fixDoorstop makes sure an existing doorstop_config.ini is enabled and targets BepInEx (writes the package's copy otherwise).
func (s *session) fixDoorstop(zipData []byte) error {
	path := abs(s.game, doorstopIni)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil // extract already wrote it
	}
	fixed := doorstopEnabled.ReplaceAll(b, []byte("enabled = true"))
	if m := doorstopTarget.FindSubmatch(fixed); m == nil || !strings.Contains(strings.ToLower(string(m[1])), "bepinex.preloader.dll") {
		// Unknown target: fall back to the package's config.
		zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
		if err != nil {
			return err
		}
		for _, f := range zr.File {
			if f.Name == doorstopIni {
				rc, _ := f.Open()
				fixed, _ = io.ReadAll(rc)
				rc.Close()
			}
		}
	}
	if bytes.Equal(fixed, b) {
		return nil
	}
	return s.writeFile(doorstopIni, fixed)
}

// Uninstall removes the mod (all=false) or the mod plus BepInEx (all=true); everything goes to the backup folder.
func Uninstall(game string, all, running bool, log func(string)) (backupDir string, err error) {
	if running {
		return "", fmt.Errorf("close the game first — its files are in use")
	}
	s := newSession(game, log)
	defer func() { s.writeLog(err) }()
	rels := []string{pluginRel}
	if all {
		rels = []string{"BepInEx", proxyDLL, doorstopIni, ".doorstop_version"}
	}
	for _, rel := range rels {
		if err = s.moveToBackup(rel); err != nil {
			return s.backup, err
		}
	}
	if len(s.lines) == 0 {
		s.say("Nothing to remove")
	} else {
		s.say("Removed — saves are untouched; the files are in the backup folder if you want them back")
	}
	return s.backup, nil
}

func (s *session) writeLog(err error) {
	if s.backup == "" {
		return
	}
	lines := append([]string{"TCG Studio setup " + time.Now().Format(time.RFC3339)}, s.lines...)
	if err != nil {
		lines = append(lines, "ERROR: "+err.Error())
	}
	_ = os.WriteFile(filepath.Join(s.backup, "install.log"), []byte(strings.Join(lines, "\r\n")+"\r\n"), 0o644)
}

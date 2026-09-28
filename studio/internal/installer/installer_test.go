package installer

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakePayload(t *testing.T) Payload {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"BepInEx/core/BepInEx.dll":                                      "bep5",
		"BepInEx/core/BepInEx.Preloader.dll":                            "preloader",
		"BepInEx/config/BepInEx.cfg":                                    "[Chainloader]\nHideManagerGameObject = true\n",
		"BepInEx/plugins/ConfigurationManager/ConfigurationManager.dll": "cfgmgr",
		"winhttp.dll":         "proxy",
		"doorstop_config.ini": "[General]\nenabled = true\ntarget_assembly=BepInEx\\core\\BepInEx.Preloader.dll\n",
		".doorstop_version":   "4.5.0",
	}
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	zw.Close()
	return Payload{BepInExZip: buf.Bytes(), ModDLL: []byte("mod v2"), Bundle: []byte("bundle v2"), Bridge: []byte("bridge v2"), Version: "test"}
}

func newGame(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "Card Shop Simulator_Data"), 0o755)
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if strings.HasSuffix(rel, "/") {
			continue
		}
		os.WriteFile(p, []byte(body), 0o644)
	}
	return dir
}

func read(t *testing.T, game, rel string) string {
	b, err := os.ReadFile(filepath.Join(game, filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	return string(b)
}

func stateOf(s State, id string) string {
	for _, it := range s.Items {
		if it.ID == id {
			return it.State
		}
	}
	return ""
}

func mustReady(t *testing.T, game string, p Payload) {
	t.Helper()
	s := Inspect(game, p, true, false)
	if !s.Ready {
		t.Fatalf("not ready after repair: %+v", s.Items)
	}
}

func TestCleanInstall(t *testing.T) {
	p := fakePayload(t)
	game := newGame(t, nil)
	if s := Inspect(game, p, true, false); s.Ready || stateOf(s, "loader") != Missing || stateOf(s, "mod") != Missing {
		t.Fatalf("clean game should need everything: %+v", s.Items)
	}
	backup, err := Repair(game, p, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if backup != "" {
		t.Fatalf("clean install shouldn't back anything up, got %s", backup)
	}
	mustReady(t, game, p)
	if read(t, game, "BepInEx/plugins/TCGCustomCards/TCGCustomCards.dll") != "mod v2" {
		t.Fatal("mod not written")
	}
	if read(t, game, "BepInEx/plugins/TCGCustomCards/tcgcc-forge-bridge.jar") != "bridge v2" {
		t.Fatal("Forge bridge not written")
	}
}

func TestAlreadyInstalledIsNoop(t *testing.T) {
	p := fakePayload(t)
	game := newGame(t, nil)
	Repair(game, p, false, nil)
	var log []string
	backup, err := Repair(game, p, false, func(s string) { log = append(log, s) })
	if err != nil || backup != "" || len(log) != 1 || !strings.Contains(log[0], "nothing to do") {
		t.Fatalf("second repair should do nothing: backup=%q err=%v log=%v", backup, err, log)
	}
}

func TestKeepsUserConfigAndOtherPlugins(t *testing.T) {
	p := fakePayload(t)
	game := newGame(t, map[string]string{
		"BepInEx/config/BepInEx.cfg":         "user config",
		"BepInEx/plugins/Other/Other.dll":    "other plugin",
		"BepInEx/core/BepInEx.dll":           "bep5",
		"BepInEx/core/BepInEx.Preloader.dll": "preloader",
		// winhttp.dll missing → broken
		"doorstop_config.ini": "enabled = false\ntarget_assembly=BepInEx\\core\\BepInEx.Preloader.dll\n",
	})
	if s := Inspect(game, p, true, false); stateOf(s, "loader") != Outdated {
		t.Fatalf("partial BepInEx should be outdated: %+v", s.Items)
	}
	if _, err := Repair(game, p, false, nil); err != nil {
		t.Fatal(err)
	}
	mustReady(t, game, p)
	if read(t, game, "BepInEx/config/BepInEx.cfg") != "user config" {
		t.Fatal("user config overwritten")
	}
	if read(t, game, "BepInEx/plugins/Other/Other.dll") != "other plugin" {
		t.Fatal("other plugin touched")
	}
	if !strings.Contains(read(t, game, "doorstop_config.ini"), "enabled = true") {
		t.Fatal("doorstop not re-enabled")
	}
}

func TestReplacesBepInEx6AndMelonWithBackup(t *testing.T) {
	p := fakePayload(t)
	game := newGame(t, map[string]string{
		"BepInEx/core/BepInEx.Core.dll":       "bep6",
		"BepInEx/core/BepInEx.Unity.Mono.dll": "bep6",
		"version.dll":                         "melon proxy",
		"MelonLoader/MelonLoader.dll":         "melon",
	})
	s := Inspect(game, p, true, false)
	if stateOf(s, "loader") != Conflict {
		t.Fatalf("expected conflict: %+v", s.Items)
	}
	backup, err := Repair(game, p, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	mustReady(t, game, p)
	for _, rel := range []string{"version.dll", "MelonLoader/MelonLoader.dll", "BepInEx/core/BepInEx.Core.dll"} {
		if _, err := os.Stat(filepath.Join(backup, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s not in backup: %v", rel, err)
		}
		if _, err := os.Stat(filepath.Join(game, filepath.FromSlash(rel))); err == nil {
			t.Fatalf("%s still in game folder", rel)
		}
	}
	if _, err := os.Stat(filepath.Join(backup, "install.log")); err != nil {
		t.Fatal("no install.log in backup")
	}
}

func TestUpdatesOldModAndRemovesStrayCopies(t *testing.T) {
	p := fakePayload(t)
	game := newGame(t, nil)
	Repair(game, p, false, nil)
	os.WriteFile(filepath.Join(game, "BepInEx/plugins/TCGCustomCards/TCGCustomCards.dll"), []byte("mod v1"), 0o644)
	os.WriteFile(filepath.Join(game, "BepInEx/plugins/TCGCustomCards.dll"), []byte("loose copy"), 0o644)
	s := Inspect(game, p, true, false)
	if stateOf(s, "mod") != Outdated || stateOf(s, "stray") != Conflict {
		t.Fatalf("expected outdated + stray: %+v", s.Items)
	}
	backup, err := Repair(game, p, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	mustReady(t, game, p)
	if read(t, game, "BepInEx/plugins/TCGCustomCards/TCGCustomCards.dll") != "mod v2" {
		t.Fatal("mod not updated")
	}
	if b, _ := os.ReadFile(filepath.Join(backup, "BepInEx/plugins/TCGCustomCards/TCGCustomCards.dll")); string(b) != "mod v1" {
		t.Fatal("old mod not backed up")
	}
	if _, err := os.Stat(filepath.Join(game, "BepInEx/plugins/TCGCustomCards.dll")); err == nil {
		t.Fatal("stray copy still present")
	}
}

func TestRunningGameBlocks(t *testing.T) {
	p := fakePayload(t)
	game := newGame(t, nil)
	if s := Inspect(game, p, true, true); s.Ready || stateOf(s, "running") != Blocked {
		t.Fatal("running game should block")
	}
	if _, err := Repair(game, p, true, nil); err == nil {
		t.Fatal("repair must refuse while the game runs")
	}
}

func TestUninstall(t *testing.T) {
	p := fakePayload(t)
	game := newGame(t, nil)
	Repair(game, p, false, nil)
	if _, err := Uninstall(game, false, false, nil); err != nil {
		t.Fatal(err)
	}
	if s := Inspect(game, p, true, false); stateOf(s, "mod") != Missing || stateOf(s, "loader") != OK {
		t.Fatalf("mod-only uninstall wrong: %+v", s.Items)
	}
	if _, err := Uninstall(game, true, false, nil); err != nil {
		t.Fatal(err)
	}
	if s := Inspect(game, p, true, false); stateOf(s, "loader") != Missing {
		t.Fatalf("full uninstall should remove BepInEx: %+v", s.Items)
	}
}

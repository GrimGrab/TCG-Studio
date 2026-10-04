package debuglog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sampleLog = "[Message:   BepInEx] BepInEx 5.4.23.5 - Card Shop Simulator (9/24/2026 4:33:58 PM)\r\n" +
	"[Info   :   BepInEx] Loading [TCG Custom Cards 0.13.3]\r\n" +
	"[Info   :TCG Custom Cards] Save path D:/Profiles/Alice/AppData/LocalLow/OPNeonGames\r\n" +
	"[Error  :TCG Custom Cards] boom in D:\\PROFILES\\alice\\Desktop\r\n"

func writeGame(t *testing.T, log string, dllAfterLog bool) string {
	t.Helper()
	game := t.TempDir()
	logPath := filepath.Join(game, "BepInEx", "LogOutput.log")
	dll := filepath.Join(game, filepath.FromSlash(modDLLRel))
	for _, p := range []string{logPath, dll} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(logPath, []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dll, []byte("dll"), 0o644); err != nil {
		t.Fatal(err)
	}
	logTime, dllTime := time.Now(), time.Now().Add(-time.Hour)
	if dllAfterLog {
		dllTime = logTime.Add(time.Hour)
	}
	_ = os.Chtimes(logPath, logTime, logTime)
	_ = os.Chtimes(dll, dllTime, dllTime)
	return game
}

func hasNote(r Report, level, part string) bool {
	for _, n := range r.Notes {
		if n.Level == level && strings.Contains(n.Text, part) {
			return true
		}
	}
	return false
}

func TestVersionMismatch(t *testing.T) {
	game := writeGame(t, sampleLog, false)
	r := Build(Input{GameDir: game, StudioVersion: "0.15.0", ModVersion: "0.15.0", SetupReady: true, Home: `D:\Profiles\Alice`})
	if !r.Found || r.LoadedVersion != "0.13.3" || r.BepInEx != "5.4.23.5" || r.Errors != 1 {
		t.Fatalf("parsed %+v", r)
	}
	if !hasNote(r, "err", "loaded mod v0.13.3, but this TCG Studio installs v0.15.0") {
		t.Errorf("no mismatch note: %+v", r.Notes)
	}
	if strings.Contains(strings.ToLower(r.Log), "alice") || !strings.Contains(r.Log, "%USERPROFILE%/AppData") {
		t.Errorf("user name not masked:\n%s", r.Log)
	}
	if !strings.Contains(r.Header, "mod loaded by the game: 0.13.3") || !strings.Contains(r.Header, "Setup: all green") {
		t.Errorf("header:\n%s", r.Header)
	}
}

func TestMatchAndStale(t *testing.T) {
	r := Build(Input{GameDir: writeGame(t, strings.Replace(sampleLog, "0.13.3", "0.15.0", 1), false), ModVersion: "0.15.0"})
	if !hasNote(r, "ok", "v0.15.0") {
		t.Errorf("no ok note: %+v", r.Notes)
	}
	r = Build(Input{GameDir: writeGame(t, sampleLog, true), ModVersion: "0.15.0"})
	if !hasNote(r, "warn", "hasn't been started since") || hasNote(r, "err", "but this TCG Studio installs") {
		t.Errorf("stale log notes: %+v", r.Notes)
	}
}

func TestNotLoadedTwiceMissing(t *testing.T) {
	r := Build(Input{GameDir: writeGame(t, "[Message:   BepInEx] BepInEx 5.4.23.5 - x\n", false), ModVersion: "1"})
	if !hasNote(r, "err", "wasn't loaded") {
		t.Errorf("not loaded: %+v", r.Notes)
	}
	r = Build(Input{GameDir: writeGame(t, sampleLog+"[Info   :   BepInEx] Loading [TCG Custom Cards 0.13.3]\n", false)})
	if !hasNote(r, "err", "loaded 2 times") {
		t.Errorf("twice: %+v", r.Notes)
	}
	r = Build(Input{GameDir: t.TempDir()})
	if r.Found || !hasNote(r, "err", "No BepInEx log") {
		t.Errorf("missing: %+v", r)
	}
}

func TestTruncate(t *testing.T) {
	big := sampleLog + strings.Repeat("x", MaxLog) + "\nLAST LINE\n"
	r := Build(Input{GameDir: writeGame(t, big, false)})
	if !r.Truncated || len(r.Log) > MaxLog+100 || !strings.Contains(r.Log, "Loading [TCG Custom Cards") || !strings.HasSuffix(r.Log, "LAST LINE\n") {
		t.Errorf("truncated=%v len=%d", r.Truncated, len(r.Log))
	}
}

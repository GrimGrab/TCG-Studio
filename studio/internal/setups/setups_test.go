package setups

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tcgstudio/internal/game"
	"tcgstudio/internal/modconfig"
	"tcgstudio/internal/project"
)

type env struct {
	h       Home
	gameDir string
	saveDir string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{h: Home{Root: t.TempDir()}, gameDir: t.TempDir(), saveDir: t.TempDir()}
	mkdir(t, filepath.Join(e.gameDir, "Card Shop Simulator_Data"))
	mkdir(t, game.SetsDir(e.gameDir))
	old := SaveDir
	SaveDir = func() string { return e.saveDir }
	t.Cleanup(func() { SaveDir = old; failAt = "" })
	return e
}

func mkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, p, s string) {
	t.Helper()
	mkdir(t, filepath.Dir(p))
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

// addInstalledSet creates a project in setup dir and installs it into the game.
func (e *env) addInstalledSet(t *testing.T, dir, id string) {
	t.Helper()
	ws := project.Workspace{Root: dir}
	p, err := ws.Create(id, strings.ToUpper(id))
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.Install(p, e.gameDir); err != nil {
		t.Fatal(err)
	}
}

func TestMigrationMovesOldLayout(t *testing.T) {
	e := newEnv(t)
	write(t, filepath.Join(e.h.Root, "projects", "dom", "set.json"), "{}")
	write(t, filepath.Join(e.h.Root, "accessories", "accessories.json"), "{}")
	dir, err := e.h.Ensure("1.0")
	if err != nil {
		t.Fatal(err)
	}
	if dir != e.h.Dir(DefaultID) {
		t.Fatalf("active dir = %s", dir)
	}
	if read(t, filepath.Join(dir, "projects", "dom", "set.json")) != "{}" || exists(filepath.Join(e.h.Root, "projects")) {
		t.Fatal("projects not moved")
	}
	if !exists(filepath.Join(dir, "accessories", "accessories.json")) {
		t.Fatal("accessories not moved")
	}
	if again, err := e.h.Ensure("1.0"); err != nil || again != dir {
		t.Fatalf("second Ensure = %s, %v", again, err)
	}
}

func TestMigrationRollsBack(t *testing.T) {
	e := newEnv(t)
	write(t, filepath.Join(e.h.Root, "projects", "dom", "set.json"), "{}")
	write(t, filepath.Join(e.h.Root, "accessories", "accessories.json"), "{}")
	// A non-empty target makes the second rename fail.
	write(t, filepath.Join(e.h.Dir(DefaultID), "accessories", "blocker"), "x")
	if _, err := e.h.Ensure("1.0"); err == nil {
		t.Fatal("expected an error")
	}
	if !exists(filepath.Join(e.h.Root, "projects", "dom", "set.json")) || !exists(filepath.Join(e.h.Root, "accessories", "accessories.json")) {
		t.Fatal("old layout not restored")
	}
	if exists(filepath.Join(e.h.Root, RegistryFile)) {
		t.Fatal("registry written after a failed migration")
	}
}

func TestSwitchRoundTrip(t *testing.T) {
	e := newEnv(t)
	a, err := e.h.Ensure("1.0")
	if err != nil {
		t.Fatal(err)
	}
	e.addInstalledSet(t, a, "dom")
	write(t, filepath.Join(game.SetsDir(e.gameDir), "loose", "set.json"), `{"id":"loose"}`)
	write(t, modconfig.Path(e.gameDir), "[Content]\r\nShowVanillaCards = false\r\n")
	write(t, filepath.Join(game.PluginDir(e.gameDir), backFile), "back")
	write(t, filepath.Join(e.saveDir, "savedGames_Release1.json"), "save A")
	write(t, filepath.Join(e.saveDir, sideCarDir, "slot1.json"), "side A")
	write(t, filepath.Join(e.saveDir, "savedGames_KeybindSetting.gd"), "keys")

	b, err := e.h.Create("Vanilla", "", false, "1.0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.h.Switch(b, e.gameDir, nil); err != nil {
		t.Fatal(err)
	}
	if HasSaves(e.saveDir) {
		t.Fatal("saves of A still in the game")
	}
	if !exists(filepath.Join(e.saveDir, "savedGames_KeybindSetting.gd")) {
		t.Fatal("key bindings must stay")
	}
	if !dirEmpty(game.SetsDir(e.gameDir)) || exists(modconfig.Path(e.gameDir)) || exists(filepath.Join(game.PluginDir(e.gameDir), backFile)) {
		t.Fatal("game not cleared for the empty setup")
	}
	if len(e.h.backups(t)) != 1 {
		t.Fatal("expected one saves backup")
	}
	write(t, filepath.Join(e.saveDir, "savedGames_Release2.json"), "save B")

	if _, err := e.h.Switch(DefaultID, e.gameDir, nil); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(e.saveDir, "savedGames_Release1.json")) != "save A" || read(t, filepath.Join(e.saveDir, sideCarDir, "slot1.json")) != "side A" {
		t.Fatal("saves of A not restored")
	}
	if exists(filepath.Join(e.saveDir, "savedGames_Release2.json")) {
		t.Fatal("save of B leaked into A")
	}
	if read(t, filepath.Join(e.h.Dir(b), SavesDir, "savedGames_Release2.json")) != "save B" {
		t.Fatal("save of B not parked")
	}
	if !exists(filepath.Join(game.SetsDir(e.gameDir), "dom", "set.json")) || read(t, filepath.Join(game.SetsDir(e.gameDir), "loose", "set.json")) != `{"id":"loose"}` {
		t.Fatal("sets of A not reinstalled")
	}
	if !strings.Contains(read(t, modconfig.Path(e.gameDir)), "ShowVanillaCards = false") || read(t, filepath.Join(game.PluginDir(e.gameDir), backFile)) != "back" {
		t.Fatal("config / card back of A not restored")
	}
	if act, _ := e.h.Active(); act != DefaultID {
		t.Fatalf("active = %s", act)
	}
	if len(e.h.backups(t)) != 1 {
		t.Fatal("backup must only be made once")
	}
}

func (h Home) backups(t *testing.T) []string {
	m, _ := filepath.Glob(filepath.Join(h.Root, "saves-backup-*"))
	return m
}

func TestRecoverUndoesCapture(t *testing.T) {
	e := newEnv(t)
	if _, err := e.h.Ensure("1.0"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(e.saveDir, "savedGames_Release0.json"), "A")
	b, _ := e.h.Create("B", "", false, "1.0")
	failAt = PhaseCapture
	if _, err := e.h.Switch(b, e.gameDir, nil); err == nil {
		t.Fatal("expected the test failure")
	}
	failAt = ""
	if HasSaves(e.saveDir) {
		t.Fatal("capture should have moved the save")
	}
	msg, err := e.h.Recover(e.gameDir)
	if err != nil || msg == "" {
		t.Fatalf("Recover = %q, %v", msg, err)
	}
	if read(t, filepath.Join(e.saveDir, "savedGames_Release0.json")) != "A" {
		t.Fatal("save not put back")
	}
	if act, _ := e.h.Active(); act != DefaultID {
		t.Fatal("active changed")
	}
	if _, ok := e.h.PendingSwitch(); ok {
		t.Fatal("journal left behind")
	}
}

func TestRecoverFinishesApply(t *testing.T) {
	e := newEnv(t)
	if _, err := e.h.Ensure("1.0"); err != nil {
		t.Fatal(err)
	}
	b, _ := e.h.Create("B", "", false, "1.0")
	e.addInstalledSet(t, e.h.Dir(b), "neo")
	in, _ := LoadInfo(e.h.Dir(b))
	in.InstalledSets = []string{"neo"}
	_ = SaveInfo(e.h.Dir(b), in)
	_ = project.Uninstall("neo", e.gameDir)
	write(t, filepath.Join(e.h.Dir(b), SavesDir, "savedGames_Release3.json"), "B")

	failAt = PhaseApply
	if _, err := e.h.Switch(b, e.gameDir, nil); err == nil {
		t.Fatal("expected the test failure")
	}
	failAt = ""
	if _, err := e.h.Switch(b, e.gameDir, nil); err == nil {
		t.Fatal("a new switch must refuse while one is pending")
	}
	if _, err := e.h.Recover(e.gameDir); err != nil {
		t.Fatal(err)
	}
	if act, _ := e.h.Active(); act != b {
		t.Fatalf("active = %s", act)
	}
	if !exists(filepath.Join(game.SetsDir(e.gameDir), "neo", "set.json")) || read(t, filepath.Join(e.saveDir, "savedGames_Release3.json")) != "B" {
		t.Fatal("setup B not applied")
	}
}

func TestExportImport(t *testing.T) {
	e := newEnv(t)
	a, _ := e.h.Ensure("1.0")
	e.addInstalledSet(t, a, "dom")
	write(t, modconfig.Path(e.gameDir), "[MTG]\r\nForgeFolder = D:\\Games\\Forge\r\nOther = x\r\n")
	write(t, filepath.Join(a, "projects", "dom", "studio.json"), `{"source":"manual","pick":"C:\\Users\\Someone\\art.png","cards":{}}`)
	write(t, filepath.Join(e.saveDir, "savedGames_Release1.json"), "save")
	b, _ := e.h.Create("Other", "", false, "1.0")
	if _, err := e.h.Switch(b, e.gameDir, nil); err != nil { // parks A's saves in A\saves
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "My Setup"+Ext)
	if err := e.h.Export(DefaultID, out, "1.0", nil); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
		if strings.HasPrefix(f.Name, SavesDir) {
			t.Fatalf("saves exported: %s", f.Name)
		}
	}
	zr.Close()
	for _, want := range []string{manifestFile, InfoFile, "projects/dom/set.json", "game/" + cfgFile} {
		if !names[want] {
			t.Fatalf("missing %s in export", want)
		}
	}

	id, err := e.h.Import(out, "1.0")
	if err != nil {
		t.Fatal(err)
	}
	in, _ := LoadInfo(e.h.Dir(id))
	if in.Name != "My Setup (2)" || len(in.InstalledSets) != 1 || in.InstalledSets[0] != "dom" {
		t.Fatalf("imported info = %+v", in)
	}
	cfg := read(t, filepath.Join(e.h.Dir(id), GameStateDir, cfgFile))
	if strings.Contains(cfg, `D:\Games`) || !strings.Contains(cfg, "ForgeFolder = \r\n") || !strings.Contains(cfg, "Other = x") {
		t.Fatalf("cfg not scrubbed: %q", cfg)
	}
	if meta := read(t, filepath.Join(e.h.Dir(id), "projects", "dom", "studio.json")); strings.Contains(meta, "Someone") {
		t.Fatalf("studio.json not scrubbed: %s", meta)
	}
	if exists(filepath.Join(e.h.Dir(id), SavesDir)) {
		t.Fatal("imported setup has saves")
	}
}

func TestImportRejectsUnsafePaths(t *testing.T) {
	e := newEnv(t)
	if _, err := e.h.Ensure("1.0"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"projects/../../evil.txt", "C:/evil.txt", "/evil.txt"} {
		p := filepath.Join(t.TempDir(), "bad"+Ext)
		f, _ := os.Create(p)
		zw := zip.NewWriter(f)
		w, _ := zw.Create(manifestFile)
		_, _ = w.Write([]byte(`{"format":"tcgsetup","formatVersion":1,"name":"Bad"}`))
		w, _ = zw.Create(bad)
		_, _ = w.Write([]byte("x"))
		_ = zw.Close()
		_ = f.Close()
		if _, err := e.h.Import(p, "1.0"); err == nil {
			t.Fatalf("%s accepted", bad)
		}
	}
	if list, _ := e.h.List(); len(list) != 1 {
		t.Fatalf("setups after failed imports = %d", len(list))
	}
	newer := filepath.Join(t.TempDir(), "new"+Ext)
	f, _ := os.Create(newer)
	zw := zip.NewWriter(f)
	w, _ := zw.Create(manifestFile)
	_, _ = w.Write([]byte(`{"format":"tcgsetup","formatVersion":99,"name":"Future","studioVersion":"9.0.0"}`))
	_ = zw.Close()
	_ = f.Close()
	if _, err := e.h.Import(newer, "1.0"); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("newer format: %v", err)
	}
}

func TestDuplicateAndDelete(t *testing.T) {
	e := newEnv(t)
	a, _ := e.h.Ensure("1.0")
	write(t, filepath.Join(a, "projects", "dom", "set.json"), "{}")
	id, err := e.h.Create("My Setup", DefaultID, false, "1.0")
	if err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(e.h.Dir(id), "projects", "dom", "set.json")) {
		t.Fatal("projects not copied")
	}
	list, _ := e.h.List()
	if len(list) != 2 || list[1].Name != "My Setup (2)" || list[1].Sets != 1 {
		t.Fatalf("list = %+v", list)
	}
	if err := e.h.Delete(DefaultID); err == nil {
		t.Fatal("deleted the active setup")
	}
	if err := e.h.Delete(id); err != nil || exists(e.h.Dir(id)) {
		t.Fatalf("delete: %v", err)
	}
}

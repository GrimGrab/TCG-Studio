package setups

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/game"
	"tcgstudio/internal/modconfig"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

type env struct {
	h       Home
	gameDir string
	saveDir string
}

const backFile = "card_back.png"

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{h: Home{Root: t.TempDir()}, gameDir: t.TempDir(), saveDir: t.TempDir()}
	mkdir(t, filepath.Join(e.gameDir, "Card Shop Simulator_Data"))
	mkdir(t, game.PluginDir(e.gameDir))
	var dll []byte // a mod that reads set files from <plugin>\Library
	for _, c := range utf16.Encode([]rune("tcgcc-capability:studio-setup")) {
		dll = append(dll, byte(c), byte(c>>8))
	}
	write(t, filepath.Join(game.PluginDir(e.gameDir), "TCGCustomCards.dll"), string(dll))
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

// addInstalledSet creates a project in setup dir and installs it (ticked in the setup; the game reads it from there).
func (e *env) addInstalledSet(t *testing.T, dir, id string) {
	t.Helper()
	ws := project.Workspace{Root: dir, Library: project.LibraryDir(e.h.Root)}
	p, err := ws.Create(id, strings.ToUpper(id))
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.Install(p, e.gameDir); err != nil {
		t.Fatal(err)
	}
}

func TestSwitchRoundTrip(t *testing.T) {
	e := newEnv(t)
	a, err := e.h.Ensure("1.0")
	if err != nil {
		t.Fatal(err)
	}
	e.addInstalledSet(t, a, "dom")
	write(t, modconfig.Path(e.gameDir), "[Content]\r\nShowVanillaCards = false\r\n")
	write(t, filepath.Join(a, GameStateDir, backFile), "back") // lives in the setup
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
	if !fsutil.Exists(filepath.Join(e.saveDir, "savedGames_KeybindSetting.gd")) {
		t.Fatal("key bindings must stay")
	}
	if fsutil.Exists(modconfig.Path(e.gameDir)) || !game.SameDir(game.ReadStudioPaths(e.gameDir).Setup, e.h.Dir(b)) {
		t.Fatal("the game doesn't point at the empty setup")
	}
	if len(e.h.backups(t)) != 0 {
		t.Fatal("a switch must not copy the saves into the workspace")
	}
	write(t, filepath.Join(e.saveDir, "savedGames_Release2.json"), "save B")

	if _, err := e.h.Switch(DefaultID, e.gameDir, nil); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(e.saveDir, "savedGames_Release1.json")) != "save A" || read(t, filepath.Join(e.saveDir, sideCarDir, "slot1.json")) != "side A" {
		t.Fatal("saves of A not restored")
	}
	if fsutil.Exists(filepath.Join(e.saveDir, "savedGames_Release2.json")) {
		t.Fatal("save of B leaked into A")
	}
	if read(t, filepath.Join(e.h.Dir(b), SavesDir, "savedGames_Release2.json")) != "save B" {
		t.Fatal("save of B not parked")
	}
	sp := game.ReadStudioPaths(e.gameDir)
	if !game.SameDir(sp.Setup, a) || !game.SameDir(sp.Library, project.LibraryDir(e.h.Root)) {
		t.Fatalf("the game doesn't point at A: %+v", sp)
	}
	if in, _ := LoadInfo(a); len(in.InstalledSets) != 1 || in.InstalledSets[0] != "dom" {
		t.Fatalf("installed sets of A lost: %+v", in.InstalledSets)
	}
	if !strings.Contains(read(t, modconfig.Path(e.gameDir)), "ShowVanillaCards = false") || read(t, filepath.Join(a, GameStateDir, backFile)) != "back" {
		t.Fatal("config of A not restored / card back of A not kept in the setup")
	}
	if entries, _ := os.ReadDir(game.PluginDir(e.gameDir)); len(entries) != 2 { // the DLL and studio-paths.json
		t.Fatalf("content copied into the game folder: %v", entries)
	}
	if act, _ := e.h.Active(); act != DefaultID {
		t.Fatalf("active = %s", act)
	}
	write(t, filepath.Join(e.h.Root, "saves-backup-20261002-101208", "savedGames_Release1.json"), "old")
	e.h.RemoveSaveBackups()
	if len(e.h.backups(t)) != 0 {
		t.Fatal("RemoveSaveBackups left an old saves backup")
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
	write(t, filepath.Join(e.h.Dir(b), SavesDir, "savedGames_Release3.json"), "B")
	// Installing into B (inactive) may point the game at it: start from the active setup, so the check below means something.
	_ = game.UpdateStudioPaths(e.gameDir, func(sp *game.StudioPaths) { sp.Setup = e.h.Dir(DefaultID) })

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
	if !game.SameDir(game.ReadStudioPaths(e.gameDir).Setup, e.h.Dir(b)) || read(t, filepath.Join(e.saveDir, "savedGames_Release3.json")) != "B" {
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

	id, _, err := e.h.Import(out, "1.0")
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
	if fsutil.Exists(filepath.Join(e.h.Dir(id), SavesDir)) {
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
		if _, _, err := e.h.Import(p, "1.0"); err == nil {
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
	if _, _, err := e.h.Import(newer, "1.0"); err == nil || !strings.Contains(err.Error(), "newer") {
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
	if !fsutil.Exists(filepath.Join(e.h.Dir(id), "projects", "dom", "set.json")) {
		t.Fatal("projects not copied")
	}
	list, _ := e.h.List()
	if len(list) != 2 || list[1].Name != "My Setup (2)" || list[1].Sets != 1 {
		t.Fatalf("list = %+v", list)
	}
	if err := e.h.Delete(DefaultID); err == nil {
		t.Fatal("deleted the active setup")
	}
	if err := e.h.Delete(id); err != nil || fsutil.Exists(e.h.Dir(id)) {
		t.Fatalf("delete: %v", err)
	}
}

// Shared accessory files go into an export under accessories/assets/ and come back into the importing workspace's store;
// converted mod content (eplmod-) stays out.
func TestExportImportSharedAssets(t *testing.T) {
	e := newEnv(t)
	a, _ := e.h.Ensure("1.0")
	store := assets.For(e.h.Root)
	l, err := accessories.Open(a, store)
	if err != nil {
		t.Fatal(err)
	}
	own, _ := l.WriteImage("mat", "texture", []byte("own playmat"))
	conv, _ := l.WriteImage(accessories.ConvertedPrefix+"x", "texture", []byte("mod art"))
	l.Put(setfmt.Accessory{ID: "mat", Kind: "Playmat", Name: "Mat", Texture: own})
	l.Put(setfmt.Accessory{ID: accessories.ConvertedPrefix + "x", Kind: "Playmat", Name: "Mod", Texture: conv})
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "s"+Ext)
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
	}
	zr.Close()
	if !names["accessories/"+own] || names["accessories/"+conv] {
		t.Fatalf("export: own %v, converted %v", names["accessories/"+own], names["accessories/"+conv])
	}
	_ = os.Remove(store.Path(own)) // as on another PC: the store doesn't have it yet
	id, _, err := e.h.Import(out, "1.0")
	if err != nil {
		t.Fatal(err)
	}
	if !fsutil.Exists(store.Path(own)) {
		t.Fatal("imported asset not in the store")
	}
	if fsutil.Exists(filepath.Join(e.h.Dir(id), "accessories", assets.DirName)) {
		t.Fatal("imported setup kept its own assets folder")
	}
	l2, _ := accessories.Open(e.h.Dir(id), store)
	if len(l2.Lib.Accessories) != 1 || l2.Lib.Accessories[0].Texture != own {
		t.Fatalf("imported library %+v", l2.Lib.Accessories)
	}
}

// A set's files travel under library/<setId>/ and land in the importing workspace's shared folder; a file this PC
// already has with other bytes is kept and reported. Files an older export kept in projects/<setId>/ are taken too.
func TestExportImportSetFiles(t *testing.T) {
	e := newEnv(t)
	a, _ := e.h.Ensure("1.0")
	ws := project.Workspace{Root: a, Library: project.LibraryDir(e.h.Root)}
	p, err := ws.Create("dom", "Dominaria")
	if err != nil {
		t.Fatal(err)
	}
	p.Set.Cards = []setfmt.Card{{ID: "1", Name: "One", Image: "images/1.png"}, {ID: "2", Name: "Two", Image: "images/2.png"}}
	if err := ws.Save(p); err != nil {
		t.Fatal(err)
	}
	write(t, p.ImagePath("images/1.png"), "one")
	write(t, p.ImagePath("images/2.png"), "two")
	out := filepath.Join(t.TempDir(), "s"+Ext)
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
	}
	zr.Close()
	if !names["library/dom/images/1.png"] || names["projects/dom/images/1.png"] {
		t.Fatalf("export entries %v", names)
	}

	// As on another PC: card 1 missing, card 2 different.
	_ = os.Remove(p.ImagePath("images/1.png"))
	write(t, p.ImagePath("images/2.png"), "two, this PC")
	_, kept, err := e.h.Import(out, "1.0")
	if err != nil {
		t.Fatal(err)
	}
	if read(t, p.ImagePath("images/1.png")) != "one" || read(t, p.ImagePath("images/2.png")) != "two, this PC" {
		t.Fatal("shared files not absorbed as expected")
	}
	if len(kept) != 1 || kept[0] != "dom/images/2.png" {
		t.Fatalf("kept %v", kept)
	}

	// An older export: the files inside projects/<id>/.
	old := filepath.Join(t.TempDir(), "old"+Ext)
	f, _ := os.Create(old)
	zw := zip.NewWriter(f)
	for name, body := range map[string]string{
		manifestFile:                `{"format":"tcgsetup","formatVersion":1,"name":"Old"}`,
		InfoFile:                    `{"name":"Old","installedSets":[]}`,
		"projects/lea/set.json":     `{"id":"lea","name":"Alpha"}`,
		"projects/lea/images/1.png": "alpha",
	} {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	_ = f.Close()
	id, _, err := e.h.Import(old, "1.0")
	if err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(project.LibraryDir(e.h.Root), "lea", "images", "1.png")) != "alpha" {
		t.Fatal("older export's files not in the shared folder")
	}
	if fsutil.Exists(filepath.Join(e.h.Dir(id), "projects", "lea", "images")) {
		t.Fatal("older export's files kept in the project folder")
	}
}

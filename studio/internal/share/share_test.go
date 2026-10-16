package share

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/catalog"
	"tcgstudio/internal/content"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/origin"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// workspaceWith returns a workspace whose active setup has set "dom" (two card images and pack art in the shared folder)
// and playmat "mat" (store texture, editor layout pointing at a second store file).
func workspaceWith(t *testing.T) (setups.Home, string) {
	t.Helper()
	h := setups.Home{Root: t.TempDir()}
	a, err := h.Ensure("1.0")
	if err != nil {
		t.Fatal(err)
	}
	ws := workspace.Dir(h.Root)
	pw := ws.Projects(a)
	p, err := pw.Create("dom", "Dominaria")
	if err != nil {
		t.Fatal(err)
	}
	p.Set.Cards = []setfmt.Card{{ID: "1", Name: "One", Image: "images/1.png"}, {ID: "2", Name: "Two", Image: "images/2.png"}}
	p.Set.Packs = []setfmt.Pack{{ID: "booster", Name: "Booster", PackTexture: "images/pack.png"}}
	p.Meta.Source = "scryfall"
	p.Meta.PackArt = map[string]map[string]json.RawMessage{"booster": {"pack": json.RawMessage(`{"pick":"C:\\Users\\Me\\photo.png"}`)}}
	if err := pw.Save(p); err != nil {
		t.Fatal(err)
	}
	write(t, p.ImagePath("images/1.png"), "card one")
	write(t, p.ImagePath("images/2.png"), "card two")
	write(t, p.ImagePath("images/pack.png"), "pack art")
	in, _ := setups.LoadInfo(a) // installed (Install needs a game folder)
	in.InstalledSets = []string{"dom"}
	if err := setups.SaveInfo(a, in); err != nil {
		t.Fatal(err)
	}
	l, err := ws.Items(a)
	if err != nil {
		t.Fatal(err)
	}
	tex, _ := l.WriteImage("mat", "texture", []byte("mat texture"))
	src, _ := l.PutBytes([]byte("layer photo"), ".png")
	l.Put(setfmt.Accessory{ID: "mat", Kind: "Playmat", Name: "Mat", Texture: tex})
	l.Meta.Layouts["mat"] = json.RawMessage(`{"layers":[{"src":"` + src + `"}]}`)
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}
	return h, a
}

func active(t *testing.T, h setups.Home) string {
	t.Helper()
	id, err := h.Active()
	if err != nil {
		t.Fatal(err)
	}
	return h.Dir(id)
}

func exportBoth(t *testing.T, h setups.Home, a string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "Bob's stuff"+ItemsExt)
	skipped, err := ExportItems(h, a, []Pick{{"set", "dom"}, {"accessory", "mat"}}, out, "1.0", nil)
	if err != nil || len(skipped) != 0 {
		t.Fatalf("export: %v %v", skipped, err)
	}
	return out
}

func TestItemsRoundTrip(t *testing.T) {
	h1, a1 := workspaceWith(t)
	out := exportBoth(t, h1, a1)

	// Another PC.
	h2 := setups.Home{Root: t.TempDir()}
	if _, err := h2.Ensure("1.0"); err != nil {
		t.Fatal(err)
	}
	a2 := active(t, h2)
	pv, err := PreviewItems(h2, out)
	if err != nil || len(pv.Items) != 2 || pv.Source.Name != "Bob's stuff" {
		t.Fatalf("preview: %+v %v", pv, err)
	}
	for _, it := range pv.Items {
		if it.NewID != it.ID {
			t.Fatalf("free id renamed: %+v", it)
		}
	}
	res, err := ImportItems(h2, a2, out, Selection{Source: Source{Name: "Bob's stuff", Author: "Bob"}, AddHere: true}, nil)
	if err != nil || len(res.Items) != 2 || res.Added != 2 {
		t.Fatalf("import: %+v %v", res, err)
	}
	ws := workspace.Dir(h2.Root)
	p, err := ws.Projects(a2).Load("dom")
	if err != nil {
		t.Fatal(err)
	}
	if read(t, p.ImagePath("images/1.png")) != "card one" || read(t, p.ImagePath("images/pack.png")) != "pack art" {
		t.Fatal("set files not in the shared folder")
	}
	if p.Meta.Source != "scryfall" || p.Meta.Origin == nil || p.Meta.Origin.Kind != origin.KindShared || p.Meta.Origin.Author != "Bob" {
		t.Fatalf("set meta: source %q origin %+v", p.Meta.Source, p.Meta.Origin)
	}
	if b, _ := json.Marshal(p.Meta.PackArt); strings.Contains(string(b), "Users") {
		t.Fatalf("absolute path not scrubbed: %s", b)
	}
	if in, _ := setups.LoadInfo(a2); len(in.InstalledSets) != 0 {
		t.Fatal("an added set must not be installed")
	}
	l, _ := ws.Items(a2)
	if len(l.Lib.Accessories) != 1 || read(t, l.Resolve(l.Lib.Accessories[0].Texture)) != "mat texture" {
		t.Fatalf("accessory: %+v", l.Lib.Accessories)
	}
	if refs := assets.Refs(l.Meta.Layouts["mat"]); len(refs) != 1 || read(t, l.Resolve(refs[0])) != "layer photo" {
		t.Fatalf("layout files: %s", l.Meta.Layouts["mat"])
	}
	if o := l.Meta.Origins["mat"]; o.Kind != origin.KindShared || o.Mod != "Bob's stuff" {
		t.Fatalf("accessory origin %+v", o)
	}
	if cat, _ := catalog.For(h2.Root).Items(); cat.Index("mat") < 0 {
		t.Fatal("not in the catalog")
	}

	// The same file again: nothing is overwritten, both get new ids.
	res, err = ImportItems(h2, a2, out, Selection{AddHere: true}, nil)
	if err != nil || len(res.Items) != 2 {
		t.Fatalf("second import: %+v %v", res, err)
	}
	for _, it := range res.Items {
		if it.ID == it.OrigID || it.OrigID == "" || !setfmt.SafeID(it.ID) {
			t.Fatalf("not renamed: %+v", it)
		}
		if it.Kind == "set" {
			q, err := ws.Projects(a2).Load(it.ID)
			if err != nil || q.Set.ID != it.ID || q.Meta.Origin != nil || read(t, q.ImagePath("images/2.png")) != "card two" {
				t.Fatalf("renamed set: %+v %v", q, err)
			}
		} else {
			l, _ := ws.Items(a2)
			if l.Index(it.ID) < 0 || len(l.Meta.Layouts[it.ID]) == 0 || len(l.Lib.Accessories) != 2 {
				t.Fatalf("renamed accessory %s: %+v", it.ID, l.Lib.Accessories)
			}
			if _, ok := l.Meta.Origins[it.ID]; ok {
				t.Fatal("empty source must leave no origin")
			}
		}
	}
	if p, _ := ws.Projects(a2).Load("dom"); p.Set.Name != "Dominaria" {
		t.Fatal("the first copy changed")
	}
}

func TestOrphanLibraryFolderIsTaken(t *testing.T) {
	h1, a1 := workspaceWith(t)
	out := exportBoth(t, h1, a1)
	h2 := setups.Home{Root: t.TempDir()}
	if _, err := h2.Ensure("1.0"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(workspace.Dir(h2.Root).Library(), "dom", "images", "1.png"), "someone else's art")
	pv, err := PreviewItems(h2, out)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range pv.Items {
		if it.Kind == "set" && it.NewID == "dom" {
			t.Fatal("a set id with a shared folder must count as taken")
		}
	}
}

func TestConvertedContentStays(t *testing.T) {
	h, a := workspaceWith(t)
	l, _ := workspace.Dir(h.Root).Items(a)
	l.Put(setfmt.Accessory{ID: accessories.ConvertedPrefix + "x", Kind: "Playmat", Name: "Mod mat"})
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "x"+ItemsExt)
	skipped, err := ExportItems(h, a, []Pick{{"accessory", accessories.ConvertedPrefix + "x"}}, out, "1.0", nil)
	if err == nil || len(skipped) != 1 || fsutil.Exists(out) {
		t.Fatalf("converted exported: %v %v", skipped, err)
	}
}

func zipWith(t *testing.T, entries map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f"+ItemsExt)
	f, _ := os.Create(p)
	zw := zip.NewWriter(f)
	for name, body := range entries {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	_ = f.Close()
	return p
}

func TestRefusedFiles(t *testing.T) {
	h := setups.Home{Root: t.TempDir()}
	if _, err := h.Ensure("1.0"); err != nil {
		t.Fatal(err)
	}
	a := active(t, h)
	ok := `{"format":"tcgitems","formatVersion":1,"items":[{"kind":"accessory","id":"x","name":"X"}]}`
	cases := map[string]map[string]string{
		"newer": {manifestFile: `{"format":"tcgitems","formatVersion":99,"studioVersion":"9.0","items":[]}`},
		"kind":  {manifestFile: `{"format":"tcgitems","formatVersion":1,"items":[{"kind":"boardgamelist","id":"x"}]}`},
		"setup": {manifestFile: `{"format":"tcgsetup","formatVersion":3,"items":[]}`},
		"slip":  {manifestFile: ok, "items/accessory/x/../../../evil.txt": "x", "items/accessory/x/entry.json": `{"id":"x"}`},
	}
	for name, entries := range cases {
		if _, err := ImportItems(h, a, zipWith(t, entries), Selection{}, nil); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if fsutil.Exists(filepath.Join(filepath.Dir(h.Root), "evil.txt")) {
		t.Fatal("zip slip")
	}
}

// Content Studio doesn't know yet as a kind travels anyway: an accessory with an unknown kind value and a model file.
func TestNewAccessoryKindNeedsNoCode(t *testing.T) {
	h, a := workspaceWith(t)
	ws := workspace.Dir(h.Root)
	l, _ := ws.Items(a)
	mesh, _ := l.PutBytes([]byte("o board\nv 0 0 0\n"), ".obj")
	l.Put(setfmt.Accessory{ID: "chess", Kind: "BoardGame", Name: "Chess", Mesh: mesh})
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "chess"+ItemsExt)
	if _, err := ExportItems(h, a, []Pick{{"accessory", "chess"}}, out, "1.0", nil); err != nil {
		t.Fatal(err)
	}
	h2 := setups.Home{Root: t.TempDir()}
	if _, err := h2.Ensure("1.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportItems(h2, active(t, h2), out, Selection{AddHere: true}, nil); err != nil {
		t.Fatal(err)
	}
	l2, _ := workspace.Dir(h2.Root).Items(active(t, h2))
	i := l2.Index("chess")
	if i < 0 || l2.Lib.Accessories[i].Kind != "BoardGame" || !strings.Contains(read(t, l2.Resolve(l2.Lib.Accessories[i].Mesh)), "board") {
		t.Fatalf("board game: %+v", l2.Lib.Accessories)
	}
}

// Every list of the accessory library has a kind in the registry (add one line to content.Kinds for a new list).
func TestKindsCoverLibrary(t *testing.T) {
	have := map[string]bool{}
	for _, k := range content.Kinds {
		if e, ok := k.Store.(content.Entry); ok {
			have[e.Field] = true
		}
	}
	for _, name := range accessories.ListNames() {
		if !have[name] {
			t.Errorf("accessory library list %q has no kind in content.Kinds", name)
		}
	}
}

func TestSetupRoundTrip(t *testing.T) {
	h, a := workspaceWith(t)
	write(t, filepath.Join(a, setups.GameStateDir, setups.CfgFile), "[MTG]\r\nForgeFolder = D:\\Games\\Forge\r\nOther = x\r\n")
	write(t, filepath.Join(a, setups.GameStateDir, "card_back.png"), "back")
	write(t, filepath.Join(a, setups.GameStateDir, "Sets", "old", "set.json"), "parked")
	write(t, filepath.Join(a, setups.SavesDir, "savedGames_Release1.json"), "save")
	l, _ := workspace.Dir(h.Root).Items(a)
	l.PutFurniture(setfmt.Furniture{ID: "bin", Type: "TrashBin", Name: "Bin"})
	l.Put(setfmt.Accessory{ID: "dice", Kind: "Dice", Name: "Dice"})
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "My Setup"+setups.Ext)
	if err := ExportSetup(h, filepath.Base(a), out, "1.0", nil); err != nil {
		t.Fatal(err)
	}
	zr, _ := zip.OpenReader(out)
	for _, f := range zr.File {
		if strings.Contains(f.Name, "saves") || strings.Contains(f.Name, "Sets/old") {
			t.Fatalf("exported %s", f.Name)
		}
	}
	zr.Close()
	info, err := ReadSetup(out)
	if err != nil || info.Manifest.Sets != 1 || info.Manifest.Accessories != 2 || info.Manifest.Furniture != 1 {
		t.Fatalf("manifest %+v %v", info, err)
	}

	check := func(h setups.Home, wantRenamed bool) {
		t.Helper()
		id, _, err := ImportSetup(h, out, "1.0")
		if err != nil {
			t.Fatal(err)
		}
		dir := h.Dir(id)
		in, _ := setups.LoadInfo(dir)
		if len(in.InstalledSets) != 1 || (in.InstalledSets[0] == "dom") == wantRenamed {
			t.Fatalf("installed sets %v", in.InstalledSets)
		}
		ws := workspace.Dir(h.Root)
		p, err := ws.Projects(dir).Load(in.InstalledSets[0])
		if err != nil || read(t, p.ImagePath("images/1.png")) != "card one" {
			t.Fatalf("installed set %v", err)
		}
		cfg := read(t, filepath.Join(dir, setups.GameStateDir, setups.CfgFile))
		if strings.Contains(cfg, `D:\Games`) || !strings.Contains(cfg, "Other = x") {
			t.Fatalf("cfg %q", cfg)
		}
		if read(t, filepath.Join(dir, setups.GameStateDir, "card_back.png")) != "back" || fsutil.Exists(filepath.Join(dir, setups.SavesDir)) {
			t.Fatal("game folder")
		}
		l, _ := ws.Items(dir)
		if len(l.Lib.Accessories) != 2 || l.Lib.Accessories[1].Name != "Dice" || len(l.Lib.Furniture) != 1 {
			t.Fatalf("library order %+v", l.Lib)
		}
		if (l.Lib.Accessories[0].ID == "mat") == wantRenamed {
			t.Fatalf("accessory id %s", l.Lib.Accessories[0].ID)
		}
		if fsutil.Exists(filepath.Join(dir, ".importing")) {
			t.Fatal("temp folder left")
		}
	}
	other := setups.Home{Root: t.TempDir()}
	if _, err := other.Ensure("1.0"); err != nil {
		t.Fatal(err)
	}
	check(other, false) // another PC: ids kept
	check(h, true)      // this PC: every id is taken, so all are new
}

package upgrade

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/catalog"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/game"
	"tcgstudio/internal/origin"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

func pngBytes(t *testing.T, c color.Color) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 64, 90))
	for y := 0; y < 90; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func put(t *testing.T, f string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func newHome(t *testing.T) (setups.Home, string) {
	t.Helper()
	h := setups.Home{Root: t.TempDir()}
	if _, err := h.Ensure("1.0"); err != nil {
		t.Fatal(err)
	}
	b, err := h.Create("B", "", false, "1.0")
	if err != nil {
		t.Fatal(err)
	}
	return h, b
}

// oldSet makes project "x-set" (two cards) in a setup root the older way: card art in the project folder.
func oldSet(t *testing.T, h setups.Home, root string, art1, art2 []byte) *project.Project {
	t.Helper()
	ws := workspace.Dir(h.Root).Projects(root)
	p, err := ws.Create("x-set", "X")
	if err != nil {
		t.Fatal(err)
	}
	p.Meta.Source, p.Meta.SetCode = "test", "SET"
	p.Set.Cards = []setfmt.Card{{ID: "1", Name: "One", Image: "images/1.png"}, {ID: "2", Name: "Two", Image: "images/2.png"}}
	put(t, filepath.Join(p.Folder, "images", "1.png"), art1)
	put(t, filepath.Join(p.Folder, "images", "2.png"), art2)
	if err := ws.Save(p); err != nil {
		t.Fatal(err)
	}
	return p
}

func load(t *testing.T, h setups.Home, root string) *project.Project {
	t.Helper()
	p, err := workspace.Dir(h.Root).Projects(root).Load("x-set")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func check(t *testing.T, h setups.Home) *Report {
	t.Helper()
	r, err := Check(context.Background(), h, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func run(t *testing.T, h setups.Home, c Choices) {
	t.Helper()
	if _, err := Run(context.Background(), h, "", c, nil); err != nil {
		t.Fatal(err)
	}
	if r := check(t, h); !r.Empty() {
		t.Fatalf("still to upgrade after Run: %+v", r)
	}
}

func art(t *testing.T, p *project.Project, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(p.ImagePath(rel))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	return b
}

// Every file a set uses moves (pack art, editor sources, set icon); identical copies and leftovers go; set.json,
// studio.json and non-image files stay; a catalog template's stale copy gives way to the setup's file.
func TestRunMovesSetFiles(t *testing.T) {
	h, b := newHome(t)
	red, blue, green := pngBytes(t, color.NRGBA{200, 0, 0, 255}), pngBytes(t, color.NRGBA{0, 0, 200, 255}), pngBytes(t, color.NRGBA{0, 200, 0, 255})
	p := oldSet(t, h, h.Dir(setups.DefaultID), red, blue)
	pk := setfmt.NewPack("booster", "X Booster")
	pk.PackTexture = "images/pack_texture.png"
	p.Set.Packs = append(p.Set.Packs, pk)
	p.Meta.PackArt = map[string]map[string]json.RawMessage{"booster": {"pack": json.RawMessage(`{"layers":[{"src":"images/photo_7.png"}]}`)}}
	put(t, filepath.Join(p.Folder, "images", "pack_texture.png"), blue)
	put(t, filepath.Join(p.Folder, "images", "photo_7.png"), red)
	put(t, filepath.Join(p.Folder, "images", "set_icon.svg"), []byte("<svg/>"))
	put(t, filepath.Join(p.Folder, "images", "old_auto_1.png"), red) // nothing refers to it
	put(t, filepath.Join(p.Folder, "notes.txt"), []byte("mine"))     // not an image: never touched
	if err := (project.Workspace{}).Save(p); err != nil {
		t.Fatal(err)
	}
	oldSet(t, h, h.Dir(b), red, blue)                    // the same art in setup B: dropped
	oldSet(t, h, catalog.For(h.Root).Root, green, green) // the catalog's stale template: dropped

	r := check(t, h)
	if r.Empty() || len(r.Differ) != 0 {
		t.Fatalf("check %+v", r)
	}
	run(t, h, Choices{})
	lib := filepath.Join(project.LibraryDir(h.Root), "x-set")
	for _, rel := range []string{"images/1.png", "images/2.png", "images/pack_texture.png", "images/photo_7.png", "images/set_icon.svg"} {
		if !fsutil.Exists(filepath.Join(lib, filepath.FromSlash(rel))) {
			t.Fatalf("%s not shared", rel)
		}
		if fsutil.Exists(filepath.Join(p.Folder, filepath.FromSlash(rel))) {
			t.Fatalf("%s still in the setup", rel)
		}
	}
	if fsutil.Exists(filepath.Join(p.Folder, "images", "old_auto_1.png")) {
		t.Fatal("leftover kept")
	}
	if !fsutil.Exists(filepath.Join(p.Folder, "notes.txt")) || !fsutil.Exists(filepath.Join(p.Folder, project.SetFile)) {
		t.Fatal("non-image or game info deleted")
	}
	if !bytes.Equal(art(t, load(t, h, catalog.For(h.Root).Root), "images/1.png"), red) {
		t.Fatal("the catalog template should use the setup's art")
	}
	if project.LoadLibMeta(lib) == nil {
		t.Fatal("library.json not written")
	}
}

// Differing copies need a choice; "shared" drops the setup's copy, "replace" makes it the shared art for everyone.
func TestRunDiffering(t *testing.T) {
	for _, choice := range []string{UseShared, Replace} {
		t.Run(choice, func(t *testing.T) {
			h, b := newHome(t)
			red, blue, green := pngBytes(t, color.NRGBA{200, 0, 0, 255}), pngBytes(t, color.NRGBA{0, 0, 200, 255}), pngBytes(t, color.NRGBA{0, 200, 0, 255})
			oldSet(t, h, h.Dir(setups.DefaultID), red, blue)
			oldSet(t, h, h.Dir(b), red, green) // card 2 changed by hand in setup B
			r := check(t, h)
			if len(r.Differ) != 1 || r.Differ[0].Setup != b || r.Differ[0].Files != 1 || r.Differ[0].Sample != "images/2.png" {
				t.Fatalf("differ %+v", r.Differ)
			}
			if _, err := Run(context.Background(), h, "", Choices{}, nil); err == nil || !strings.Contains(err.Error(), "choose") {
				t.Fatalf("run without a choice: %v", err)
			}
			if pv, err := PreviewDiffer(h, r.Differ[0].Key); err != nil || pv.Own == "" {
				t.Fatalf("preview %v %v", pv, err)
			}
			run(t, h, Choices{Differ: map[string]string{r.Differ[0].Key: choice}})
			want := blue
			if choice == Replace {
				want = green
			}
			for _, root := range []string{h.Dir(setups.DefaultID), h.Dir(b)} {
				if got := art(t, load(t, h, root), "images/2.png"); !bytes.Equal(got, want) {
					t.Fatalf("%s: card 2 is not the %s art", root, choice)
				}
			}
		})
	}
}

// A named folder (older "keep as its own") folds into the set's shared art; every copy using it is repointed.
func TestRunNamedFolder(t *testing.T) {
	for _, choice := range []string{UseShared, Replace} {
		t.Run(choice, func(t *testing.T) {
			h, b := newHome(t)
			red, blue, green := pngBytes(t, color.NRGBA{200, 0, 0, 255}), pngBytes(t, color.NRGBA{0, 0, 200, 255}), pngBytes(t, color.NRGBA{0, 200, 0, 255})
			oldSet(t, h, h.Dir(setups.DefaultID), red, blue)
			run(t, h, Choices{}) // shared art red/blue
			lib := filepath.Join(project.LibraryDir(h.Root), "x-set")
			put(t, filepath.Join(lib, "Mine", "images", "1.png"), green)
			put(t, filepath.Join(lib, "Mine", "images", "9.png"), green) // only the named folder has it
			pb := oldSet(t, h, h.Dir(b), red, blue)
			for _, f := range []string{"1.png", "2.png"} {
				_ = os.Remove(filepath.Join(pb.Folder, "images", f))
			}
			pb.Set.Cards[0].Image = "Mine/images/1.png"
			pb.Set.Cards = append(pb.Set.Cards, setfmt.Card{ID: "9", Name: "Nine", Image: "Mine/images/9.png"})
			if err := (project.Workspace{}).Save(pb); err != nil {
				t.Fatal(err)
			}
			r := check(t, h)
			if len(r.Named) != 1 || r.Named[0].Name != "Mine" || len(r.Named[0].UsedBy) != 1 || r.Named[0].Files != 2 {
				t.Fatalf("named %+v", r.Named)
			}
			run(t, h, Choices{Named: map[string]string{r.Named[0].Key: choice}})
			pb = load(t, h, h.Dir(b))
			if pb.Set.Cards[0].Image != "images/1.png" || pb.Set.Cards[2].Image != "images/9.png" {
				t.Fatalf("not repointed: %+v", pb.Set.Cards)
			}
			want := red
			if choice == Replace {
				want = green
			}
			if !bytes.Equal(art(t, pb, "images/1.png"), want) || !bytes.Equal(art(t, pb, "images/9.png"), green) {
				t.Fatal("wrong art after folding")
			}
			if fsutil.Exists(filepath.Join(lib, "Mine")) {
				t.Fatal("named folder kept")
			}
		})
	}
}

// Accessory files kept in a setup move to the shared store; entries and layouts are repointed; leftovers go.
func TestRunAccessories(t *testing.T) {
	h, _ := newHome(t)
	root := h.Dir(setups.DefaultID)
	l, err := workspace.Dir(h.Root).Items(root)
	if err != nil {
		t.Fatal(err)
	}
	red := pngBytes(t, color.NRGBA{200, 0, 0, 255})
	put(t, filepath.Join(l.Folder, "images", "mat_texture.png"), red)
	put(t, filepath.Join(l.Folder, "images", "mat_old.png"), red)
	l.Put(setfmt.Accessory{ID: "mat", Name: "Mat", Kind: "Playmat", Texture: "images/mat_texture.png"})
	l.Meta.Layouts = map[string]json.RawMessage{"mat": json.RawMessage(`{"src":"images/mat_texture.png"}`)}
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}
	if r := check(t, h); r.Files != 2 {
		t.Fatalf("check %+v", r)
	}
	run(t, h, Choices{})
	l, _ = workspace.Dir(h.Root).Items(root)
	tex := l.Lib.Accessories[0].Texture
	if !strings.HasPrefix(tex, "assets/") || !strings.Contains(string(l.Meta.Layouts["mat"]), tex) {
		t.Fatalf("not repointed: %s / %s", tex, l.Meta.Layouts["mat"])
	}
	if b, _ := os.ReadFile(l.Resolve(tex)); !bytes.Equal(b, red) {
		t.Fatal("store file differs")
	}
	if fsutil.Exists(filepath.Join(l.Folder, "images")) {
		t.Fatal("old images folder kept")
	}
}

func TestFillOrigins(t *testing.T) {
	h, _ := newHome(t)
	root := h.Dir(setups.DefaultID)
	l, err := workspace.Dir(h.Root).Items(root)
	if err != nil {
		t.Fatal(err)
	}
	old, named := accessories.ConvertedPrefix+"old", accessories.ConvertedPrefix+"named"
	l.Put(setfmt.Accessory{ID: old, Name: "Old", Kind: "Playmat"})
	l.Put(setfmt.Accessory{ID: named, Name: "Named", Kind: "Playmat"})
	l.Put(setfmt.Accessory{ID: "own", Name: "Own", Kind: "Playmat"})
	l.Meta.Origins[named] = origin.Origin{Kind: origin.KindEPL, Mod: "Mats Mod"}
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // the second run changes nothing
		if err := FillOrigins(h); err != nil {
			t.Fatal(err)
		}
	}
	l, _ = workspace.Dir(h.Root).Items(root)
	if o := l.Meta.Origins[old]; o.Kind != origin.KindEPL || o.Mod != "" || o.Name() != "an unnamed EPL mod" {
		t.Fatalf("old: %+v", o)
	}
	if o := l.Meta.Origins[named]; o.Mod != "Mats Mod" {
		t.Fatalf("named overwritten: %+v", o)
	}
	if _, ok := l.Meta.Origins["own"]; ok {
		t.Fatal("own accessory got an origin")
	}
}

func TestNeeded(t *testing.T) {
	h, _ := newHome(t)
	if Needed(h, "") {
		t.Fatal("empty workspace needs an upgrade")
	}
	red := pngBytes(t, color.NRGBA{200, 0, 0, 255})
	oldSet(t, h, h.Dir(setups.DefaultID), red, red)
	if !Needed(h, "") {
		t.Fatal("old layout not found")
	}
	run(t, h, Choices{})
	if Needed(h, "") {
		t.Fatal("still needed after Run")
	}
}

// The active setup's content kept only in the game folder (installed sets, card back, Game look) moves into the setup.
func TestRunAdoptsGameContent(t *testing.T) {
	h, _ := newHome(t)
	gameDir := t.TempDir()
	put(t, filepath.Join(gameDir, "Card Shop Simulator_Data", "x"), []byte("x"))
	plugin := game.PluginDir(gameDir)
	put(t, filepath.Join(plugin, "Sets", "x-set", project.SetFile), []byte("{}"))
	put(t, filepath.Join(plugin, "card_back.png"), []byte("back"))
	put(t, filepath.Join(plugin, "ShopSign", "sign_1.png"), []byte("sign"))
	red := pngBytes(t, color.NRGBA{200, 0, 0, 255})
	oldSet(t, h, h.Dir(setups.DefaultID), red, red)

	if r, _ := Check(context.Background(), h, gameDir, nil); len(r.Game.Sets) != 1 || !r.Game.CardBack || len(r.Game.ShopLook) != 1 {
		t.Fatalf("game state %+v", r.Game)
	}
	if _, err := Run(context.Background(), h, gameDir, Choices{}, nil); err != nil {
		t.Fatal(err)
	}
	dir := h.Dir(setups.DefaultID)
	if in, _ := setups.LoadInfo(dir); len(in.InstalledSets) != 1 || in.InstalledSets[0] != "x-set" {
		t.Fatalf("installed %+v", in.InstalledSets)
	}
	for _, f := range []string{"card_back.png", filepath.Join("ShopSign", "sign_1.png")} {
		if !fsutil.Exists(filepath.Join(dir, setups.GameStateDir, f)) {
			t.Fatalf("%s not taken over", f)
		}
	}
	if Needed(h, gameDir) || GamePending(h, gameDir) {
		t.Fatal("still pending after Run")
	}
}

// A set only in the game folder (hand-installed) becomes a project; inactive setups take over their parked sets; each
// setup adopts once, so a set uninstalled afterwards isn't installed again from the old copy; a newer game card back wins.
func TestAdoptGameOnce(t *testing.T) {
	h, other := newHome(t)
	gameDir := t.TempDir()
	put(t, filepath.Join(gameDir, "Card Shop Simulator_Data", "x"), []byte("x"))
	plugin := game.PluginDir(gameDir)
	hand := `{"schemaVersion":1,"id":"hand","name":"Hand","cards":[{"id":"1","name":"One","image":"images/1.png"}]}`
	put(t, filepath.Join(plugin, "Sets", "hand", project.SetFile), []byte(hand))
	put(t, filepath.Join(plugin, "Library", "hand", "images", "1.png"), []byte("art"))
	dir := h.Dir(setups.DefaultID)
	put(t, filepath.Join(dir, setups.GameStateDir, "card_back.png"), []byte("old"))
	put(t, filepath.Join(plugin, "card_back.png"), []byte("new"))
	future := time.Now().Add(time.Hour)
	_ = os.Chtimes(filepath.Join(plugin, "card_back.png"), future, future)
	otherDir := h.Dir(other)
	put(t, filepath.Join(otherDir, setups.GameStateDir, "Sets", "parked", project.SetFile), []byte(`{"schemaVersion":1,"id":"parked","name":"P"}`))

	if !GamePending(h, gameDir) {
		t.Fatal("nothing pending")
	}
	if _, err := Run(context.Background(), h, gameDir, Choices{Partial: true}, nil); err != nil {
		t.Fatal(err)
	}
	if !fsutil.Exists(filepath.Join(dir, "projects", "hand", project.SetFile)) || !fsutil.Exists(filepath.Join(workspace.Dir(h.Root).Library(), "hand", "images", "1.png")) {
		t.Fatal("hand-installed set not imported")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, setups.GameStateDir, "card_back.png")); string(b) != "new" {
		t.Fatalf("card back %q", b)
	}
	if in, _ := setups.LoadInfo(otherDir); len(in.InstalledSets) != 1 || in.InstalledSets[0] != "parked" || !in.AdoptedGame {
		t.Fatalf("inactive setup %+v", in)
	}
	if GamePending(h, gameDir) {
		t.Fatal("still pending")
	}
	// Uninstalled later: the game folder's copy doesn't bring it back.
	if err := setups.EditInfo(dir, func(in *setups.Info) { in.InstalledSets = nil }); err != nil {
		t.Fatal(err)
	}
	if GamePending(h, gameDir) {
		t.Fatal("uninstalled set pending again")
	}
	if _, err := Run(context.Background(), h, gameDir, Choices{Partial: true}, nil); err != nil {
		t.Fatal(err)
	}
	if in, _ := setups.LoadInfo(dir); len(in.InstalledSets) != 0 {
		t.Fatalf("set came back: %+v", in.InstalledSets)
	}
}

// The upgrade deletes only exact copies: an unreferenced image in a project folder, a setup's own art the player chose
// to give up and an older card back are kept in the workspace's upgrade-kept folder.
func TestNothingLost(t *testing.T) {
	h, b := newHome(t)
	red := pngBytes(t, color.NRGBA{200, 0, 0, 255})
	blue := pngBytes(t, color.NRGBA{0, 0, 200, 255})
	a := h.Dir(setups.DefaultID)
	pa := oldSet(t, h, a, red, red)
	put(t, filepath.Join(pa.Folder, "images", "sketch.png"), blue) // nothing refers to it
	run(t, h, Choices{})
	oldSet(t, h, h.Dir(b), blue, red) // card 1 differs from the shared art
	r, err := Run(context.Background(), h, "", Choices{Differ: map[string]string{differKey(b, "x-set"): UseShared}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kept == 0 {
		t.Fatalf("nothing kept: %+v", r)
	}
	var kept []string
	_ = filepath.WalkDir(workspace.Dir(h.Root).Kept(), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			kept = append(kept, filepath.Base(p))
		}
		return nil
	})
	has := func(name string, want []byte) bool {
		found := false
		_ = filepath.WalkDir(workspace.Dir(h.Root).Kept(), func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && filepath.Base(p) == name {
				if got, _ := os.ReadFile(p); bytes.Equal(got, want) {
					found = true
				}
			}
			return nil
		})
		return found
	}
	if !has("sketch.png", blue) {
		t.Fatalf("unreferenced image lost (kept %v)", kept)
	}
	if !has("1.png", blue) {
		t.Fatalf("setup B's own card art lost (kept %v)", kept)
	}
}

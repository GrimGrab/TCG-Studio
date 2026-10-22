package importer

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

// testArtSource serves a fixed card list, like a card database's fresh download.
type testArtSource struct {
	cards []cardIn
	get   func(context.Context, string) ([]byte, error)
}

func (testArtSource) Info() SourceInfo                                           { return SourceInfo{ID: "test"} }
func (testArtSource) DefaultOptions() Options                                    { return DefaultOptions() }
func (testArtSource) Sets(context.Context, string) ([]SetInfo, error)            { return nil, nil }
func (testArtSource) ProjectID(code, _ string) string                            { return code }
func (testArtSource) RefreshMeta(context.Context, *project.Project) (int, error) { return 0, nil }
func (testArtSource) Import(context.Context, project.Workspace, string, Options, func(Progress)) (*project.Project, error) {
	return nil, nil
}
func (s testArtSource) cardArt(_ context.Context, p *project.Project, _ []string) (*cardArt, error) {
	return artFromCards(p, s.cards, s.get, false, 0), nil
}

func pngOf(c color.NRGBA) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 63, 88))
	for y := 0; y < 88; y++ {
		for x := 0; x < 63; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// Re-download image writes the source's art for just that card under a new name and leaves the old file alone.
func TestRedownloadImages(t *testing.T) {
	ws := t.TempDir()
	w := project.Workspace{Root: filepath.Join(ws, "setups", "a"), Library: project.LibraryDir(ws)}
	red, blue := pngOf(color.NRGBA{255, 0, 0, 255}), pngOf(color.NRGBA{0, 0, 255, 255})
	cards := []cardIn{{SourceID: "a", Name: "A", Number: "1", Image: "u1"}, {SourceID: "b", Name: "B", Number: "2", Image: "u2"}}
	get := func(context.Context, string) ([]byte, error) { return red, nil }
	p, err := buildProject(context.Background(), w, setIn{ID: "x-set", Source: "test", Code: "SET", Name: "Set",
		Cards: cards, Get: get, Rarity: func(string) string { return "Common" }}, DefaultOptions(), func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	// The player's card 1 art was overwritten with something else (a booster picture).
	if err := os.WriteFile(p.ImagePath("images/1.png"), blue, 0o644); err != nil {
		t.Fatal(err)
	}
	r := &Registry{list: []Source{testArtSource{cards: cards, get: get}}}
	got, missing, err := r.RedownloadImages(context.Background(), p, []string{"1", "gone"}, func(rel string) string {
		return strings.TrimSuffix(rel, ".png") + ".v2.png"
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0] != "gone" {
		t.Fatalf("missing = %v", missing)
	}
	if got["1"].Image != "images/1.v2.png" {
		t.Fatalf("new path = %+v", got["1"])
	}
	b, err := os.ReadFile(p.ImagePath("images/1.v2.png"))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if r, _, _, _ := img.At(10, 10).RGBA(); r>>8 != 255 {
		t.Fatal("re-downloaded art isn't the source's")
	}
	if old, _ := os.ReadFile(p.ImagePath("images/1.png")); !bytes.Equal(old, blue) {
		t.Fatal("old file changed before Save")
	}
}

// Sets from a mod have no card database to download from.
func TestRedownloadImagesNoSource(t *testing.T) {
	p := &project.Project{ID: "epl-x", Set: setfmt.NewSet("epl-x", "Mod set"), Meta: &project.Meta{Source: "epl"}}
	r := &Registry{list: []Source{eplSource{}}}
	if _, _, err := r.RedownloadImages(context.Background(), p, []string{"1"}, func(s string) string { return s }); err == nil ||
		!strings.Contains(err.Error(), "came from a mod") {
		t.Fatalf("err = %v", err)
	}
}

// Every card database source can download a card's art again.
func TestSourcesCanRedownload(t *testing.T) {
	for _, s := range NewRegistry(nil, nil).All() {
		id := s.Info().ID
		if _, ok := s.(artSource); !ok && id != "epl" && id != SharedSourceID {
			t.Errorf("%s can't re-download card art", id)
		}
	}
}

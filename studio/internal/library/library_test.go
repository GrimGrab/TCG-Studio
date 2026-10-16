package library

import (
	"bytes"
	"compress/gzip"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

// pngBytes is a noisy card-sized image around colour c (noise like a scan, so JPEG is smaller than PNG).
func pngBytes(t *testing.T, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 128, 180))
	rnd := rand.New(rand.NewSource(int64(c.R)<<16 | int64(c.G)<<8 | int64(c.B)))
	for y := 0; y < 180; y++ {
		for x := 0; x < 128; x++ {
			n := uint8(rnd.Intn(48))
			img.Set(x, y, color.NRGBA{c.R/2 + n, c.G/2 + n, c.B/2 + n, 255})
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

// addSet makes project "x-set" (source test/SET, two cards) in a setup, its card art in the set's shared folder.
func addSet(t *testing.T, h setups.Home, setupID string, art1, art2 []byte) *project.Project {
	t.Helper()
	ws := workspace.Dir(h.Root).Projects(h.Dir(setupID))
	p, err := ws.Create("x-set", "X")
	if err != nil {
		t.Fatal(err)
	}
	p.Meta.Source, p.Meta.SetCode = "test", "SET"
	p.Set.Cards = []setfmt.Card{{ID: "1", Name: "One", Image: "images/1.png"}, {ID: "2", Name: "Two", Image: "images/2.png"}}
	put(t, p.ImagePath("images/1.png"), art1)
	put(t, p.ImagePath("images/2.png"), art2)
	if err := ws.Save(p); err != nil {
		t.Fatal(err)
	}
	return p
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

func analyze(t *testing.T, h setups.Home) *Report {
	t.Helper()
	r, err := Analyze(context.Background(), h, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// A set used by two setups is listed once with both users; each setup's breakdown adds up to its folder size.
func TestAnalyze(t *testing.T) {
	h, b := newHome(t)
	red, blue := pngBytes(t, color.NRGBA{200, 0, 0, 255}), pngBytes(t, color.NRGBA{0, 0, 200, 255})
	addSet(t, h, setups.DefaultID, red, blue)
	addSet(t, h, b, red, blue)
	r := analyze(t, h)
	if len(r.Sets) != 1 || len(r.Sets[0].UsedBy) != 2 || r.Sets[0].PNGFiles != 2 || r.Sets[0].Shared != int64(len(red)+len(blue)) {
		t.Fatalf("sets %+v", r.Sets)
	}
	if r.Sets[0].ShrinkTo <= 0 || r.Sets[0].ShrinkTo >= r.Sets[0].PNG {
		t.Fatalf("shrink estimate %d of %d", r.Sets[0].ShrinkTo, r.Sets[0].PNG)
	}
	for _, s := range r.Setups {
		if len(s.Sets) != 1 || s.Info == 0 || s.Info+s.Saves+s.Game+s.Other != s.Size {
			t.Fatalf("setup %+v", s)
		}
	}
	if len(r.Unused) != 0 {
		t.Fatalf("unused %+v", r.Unused)
	}
}

// Shrink converts the shared PNGs once and points every setup's copy at the JPEGs.
func TestShrink(t *testing.T) {
	h, b := newHome(t)
	red, blue := pngBytes(t, color.NRGBA{200, 0, 0, 255}), pngBytes(t, color.NRGBA{0, 0, 200, 255})
	p := addSet(t, h, setups.DefaultID, red, blue)
	addSet(t, h, b, red, blue)
	if err := project.SaveLibMeta(p.LibFolder, &project.LibMeta{Source: "test", Code: "SET", Format: "png"}); err != nil {
		t.Fatal(err)
	}
	pv, err := Preview(h, "x-set")
	if err != nil || pv.PNGSize != int64(len(red)) || !strings.HasPrefix(pv.JPEG, "data:image/jpeg;base64,") {
		t.Fatalf("preview %v %v", pv, err)
	}
	if fsutil.Exists(p.ImagePath("images/1.jpg")) {
		t.Fatal("preview wrote a file")
	}
	res, err := Shrink(context.Background(), h, []string{"x-set"}, nil)
	if err != nil || res.Files != 2 || res.Freed <= 0 {
		t.Fatalf("shrink %+v %v", res, err)
	}
	for _, setup := range []string{setups.DefaultID, b} {
		q, err := workspace.Dir(h.Root).Projects(h.Dir(setup)).Load("x-set")
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range q.Set.Cards {
			if !strings.HasSuffix(c.Image, ".jpg") || !fsutil.Exists(q.ImagePath(c.Image)) {
				t.Fatalf("%s: card %s", setup, c.Image)
			}
		}
	}
	if fsutil.Exists(p.ImagePath("images/1.png")) {
		t.Fatal("PNG kept")
	}
	if m := project.LoadLibMeta(p.LibFolder); m == nil || m.Format != "jpg" {
		t.Fatalf("library.json %+v", m)
	}
	if r := analyze(t, h); r.Sets[0].PNGFiles != 0 {
		t.Fatalf("PNG left %+v", r.Sets[0])
	}
}

// Shared folders of sets nothing has, unused files in a used set's folder and unused store files are listed and deleted;
// a set whose set.json can't be read keeps its files.
func TestUnused(t *testing.T) {
	h, _ := newHome(t)
	red := pngBytes(t, color.NRGBA{200, 0, 0, 255})
	p := addSet(t, h, setups.DefaultID, red, red)
	lib := project.LibraryDir(h.Root)
	put(t, filepath.Join(lib, "old-set", "images", "1.png"), red)
	put(t, p.ImagePath("images/old_auto_1.png"), red)
	put(t, filepath.Join(lib, "broken", "images", "1.png"), red)
	put(t, filepath.Join(h.Dir(setups.DefaultID), "projects", "broken", project.SetFile), []byte("{not json"))
	store := workspace.Dir(h.Root).Store()
	if _, err := store.Put([]byte("old texture"), ".png"); err != nil {
		t.Fatal(err)
	}

	kinds := map[string]string{}
	for _, u := range analyze(t, h).Unused {
		kinds[u.Kind] = u.ID
	}
	if kinds[UnusedSetArt] != "old-set" || kinds[UnusedSetFiles] != "x-set" || kinds[UnusedSharedFiles] == "" || len(kinds) != 3 {
		t.Fatalf("unused %v", kinds)
	}
	var picks []Pick
	for _, u := range analyze(t, h).Unused {
		picks = append(picks, Pick{Kind: u.Kind, ID: u.ID})
	}
	res, err := DeleteUnused(context.Background(), h, picks, nil)
	if err != nil || res.Freed == 0 || len(res.Deleted) != 3 {
		t.Fatalf("delete %+v %v", res, err)
	}
	if fsutil.Exists(filepath.Join(lib, "old-set")) || fsutil.Exists(p.ImagePath("images/old_auto_1.png")) {
		t.Fatal("not deleted")
	}
	if !fsutil.Exists(p.ImagePath("images/1.png")) || !fsutil.Exists(filepath.Join(lib, "broken", "images", "1.png")) {
		t.Fatal("deleted files in use")
	}
	if r := analyze(t, h); len(r.Unused) != 0 {
		t.Fatalf("still unused %+v", r.Unused)
	}
}

// Plain side-car saves of every setup are compressed in place: the same JSON back, mtime kept, nothing left to do after.
func TestCompressSaves(t *testing.T) {
	h, b := newHome(t)
	dir := filepath.Join(h.Dir(b), setups.SavesDir, "TCGCustomCards")
	body := []byte(`{"version": 1, "sets": {"x": {}}}` + strings.Repeat(" ", 4000))
	put(t, filepath.Join(dir, "slot1.json"), body)
	put(t, filepath.Join(dir, "slot1.json.bak"), body)
	put(t, filepath.Join(dir, "notes.txt"), body) // not a side-car: untouched
	old := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(filepath.Join(dir, "slot1.json"), old, old)
	if p := checkPlainSaves(h, false); p.Files != 2 || p.Bytes != int64(2*len(body)) {
		t.Fatalf("plain %+v", p)
	}
	res, err := CompressSaves(context.Background(), h, false, nil)
	if err != nil || res.Files != 2 || res.Freed <= 0 {
		t.Fatalf("compress %+v %v", res, err)
	}
	f, _ := os.Open(filepath.Join(dir, "slot1.json"))
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	back, _ := io.ReadAll(zr)
	f.Close()
	if !bytes.Equal(back, body) {
		t.Fatal("content changed")
	}
	if info, _ := os.Stat(filepath.Join(dir, "slot1.json")); info.ModTime().Sub(old).Abs() > time.Second {
		t.Fatal("modification time not kept")
	}
	if g, _ := os.ReadFile(filepath.Join(dir, "notes.txt")); !bytes.Equal(g, body) {
		t.Fatal("other file touched")
	}
	if p := checkPlainSaves(h, false); p.Files != 0 {
		t.Fatalf("still plain %+v", p)
	}
}

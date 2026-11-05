package relayout

import (
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/project"
	"tcgstudio/internal/uvmap"
)

func load(t *testing.T, p string) image.Image {
	t.Helper()
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// TestFaceRoundTrip: a face point taken through one target and back lands where it started (orientation maths agree).
func TestFaceRoundTrip(t *testing.T) {
	for _, tg := range []uvmap.Target{
		{Rect: [4]float64{10, 20, 110, 70}, Src: [4]float64{0, 0, 1, 1}},
		{Rect: [4]float64{10, 20, 110, 70}, Src: [4]float64{0, 0, 1, 1}, FlipX: true, FlipY: true},
		{Rect: [4]float64{10, 20, 60, 220}, Src: [4]float64{0, 0, 1, 1}, Transpose: true, FlipX: true},
		{Rect: [4]float64{10, 20, 60, 220}, Src: [4]float64{0.2, 0, 0.8, 1}, Transpose: true, FlipY: true},
	} {
		f := uvmap.Face{ID: "f", Targets: []uvmap.Target{tg}}
		for _, p := range [][2]float64{{0.3, 0.6}, {0.5, 0.1}} {
			px, py, _ := textureAt(f, p[0], p[1])
			s, r := faceAt(tg, f, px, py)
			if d := (s-p[0])*(s-p[0]) + (r-p[1])*(r-p[1]); d > 1e-12 {
				t.Errorf("%+v: %v → (%v,%v) → (%v,%v)", tg, p, px, py, s, r)
			}
		}
	}
}

// TestImageMovesFaces: a face painted in one layout shows on that face's areas in the other, background elsewhere.
func TestImageMovesFaces(t *testing.T) {
	from := uvmap.Model{TextureSize: 100, Faces: []uvmap.Face{{ID: "front", Targets: []uvmap.Target{{Rect: [4]float64{0, 0, 50, 50}, Src: [4]float64{0, 0, 1, 1}}}}}}
	to := uvmap.Model{TextureSize: 200, Faces: []uvmap.Face{{ID: "front", Targets: []uvmap.Target{
		{Rect: [4]float64{100, 100, 200, 200}, Src: [4]float64{0, 0, 1, 1}, FlipX: true}}}}}
	src := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			c := color.NRGBA{255, 0, 0, 255} // left half of the face red, right half green
			if x >= 25 {
				c = color.NRGBA{0, 255, 0, 255}
			}
			src.SetNRGBA(x, y, c)
		}
	}
	out := Image(from, to, src, nil)
	if c := out.NRGBAAt(110, 150); c.G < 200 { // mirrored: the face's right half is at the target's left
		t.Errorf("left of the new area %v, want green", c)
	}
	if c := out.NRGBAAt(190, 150); c.R < 200 {
		t.Errorf("right of the new area %v, want red", c)
	}
	if c := out.NRGBAAt(20, 20); c.A != 0 {
		t.Errorf("outside the faces %v, want background (transparent)", c)
	}
}

// TestConvertRealBox writes a real old box texture converted to the current box layout (RELAYOUT_BOX = the old texture,
// RELAYOUT_OUT = where to write; templates from TCG Studio's cache). For looking at, not a check.
func TestConvertRealBox(t *testing.T) {
	in, out := os.Getenv("RELAYOUT_BOX"), os.Getenv("RELAYOUT_OUT")
	if in == "" || out == "" {
		t.Skip("set RELAYOUT_BOX and RELAYOUT_OUT")
	}
	cache, _ := os.UserCacheDir()
	tpl := filepath.Join(cache, "TCG Studio", "game-templates")
	from, _ := uvmap.LegacyModelFor("Box")
	to, ok := uvmap.ModelFor("Box")
	if !ok || to.Mesh != "CardBoxMesh_low" {
		t.Fatalf("no derived box (%v %s)", ok, to.Mesh)
	}
	img := Image(from, to, load(t, in), load(t, filepath.Join(tpl, "BasicCardBox_texture.png")))
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// TestConvertRealProject runs Project on a copy of a real set (RELAYOUT_WS = a TCG Studio workspace, RELAYOUT_SET = a set id
// in its default setup, RELAYOUT_OUT = a folder for the copy). For looking at the result, not a check.
func TestConvertRealProject(t *testing.T) {
	ws, id, out := os.Getenv("RELAYOUT_WS"), os.Getenv("RELAYOUT_SET"), os.Getenv("RELAYOUT_OUT")
	if ws == "" || id == "" || out == "" {
		t.Skip("set RELAYOUT_WS, RELAYOUT_SET and RELAYOUT_OUT")
	}
	_ = os.RemoveAll(out)
	copyDir(t, filepath.Join(ws, "setups", "default", "projects", id), filepath.Join(out, "root", "projects", id))
	copyDir(t, filepath.Join(ws, "library", id), filepath.Join(out, "library", id))
	cache, _ := os.UserCacheDir()
	tpl := filepath.Join(cache, "TCG Studio", "game-templates")
	w := project.Workspace{Root: filepath.Join(out, "root"), Library: filepath.Join(out, "library")}
	p, err := w.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	before := p.Set.Packs[0].BoxTexture
	changed, err := Project(p, tpl, []string{"Box", "Pack"})
	if err != nil || !changed {
		t.Fatalf("changed %v err %v", changed, err)
	}
	if err := w.Save(p); err != nil {
		t.Fatal(err)
	}
	t.Logf("box %s → %s, pack → %s", before, p.Set.Packs[0].BoxTexture, p.Set.Packs[0].PackTexture)
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		dst := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestConvertRealPack: like TestConvertRealBox for a pack texture (RELAYOUT_PACK).
func TestConvertRealPack(t *testing.T) {
	in, out := os.Getenv("RELAYOUT_PACK"), os.Getenv("RELAYOUT_OUT")
	if in == "" || out == "" {
		t.Skip("set RELAYOUT_PACK and RELAYOUT_OUT")
	}
	cache, _ := os.UserCacheDir()
	tpl := filepath.Join(cache, "TCG Studio", "game-templates")
	from, _ := uvmap.LegacyModelFor("Pack")
	to, ok := uvmap.ModelFor("Pack")
	if !ok || to.Mesh != "CardPackStaticMesh_low" {
		t.Fatalf("no derived pack (%v %s)", ok, to.Mesh)
	}
	img := Image(from, to, load(t, in), load(t, filepath.Join(tpl, "BasicCardPack_texture.png")))
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

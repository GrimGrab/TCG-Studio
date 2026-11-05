package art

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/uvmap"
)

// TestGenerateFillsTheFaces: the generated art lands in every area of the box's and the pack's faces in the current layouts,
// not where the game 1.02 layouts had them.
// A set's card back carries the set icon (found through resolve); without a resolver there is no icon, and no error.
func TestGenerateCardBackIcon(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	icon := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="#fff"/></svg>`
	if err := os.WriteFile(filepath.Join(dir, "images", "set_icon.svg"), []byte(icon), 0o644); err != nil {
		t.Fatal(err)
	}
	centre := func(resolve func(string) string, name string) color.NRGBA {
		out := filepath.Join(dir, name)
		if err := GenerateCardBack(resolve, "#883344", "", out); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(out)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			t.Fatal(err)
		}
		b := img.Bounds()
		return color.NRGBAModel.Convert(img.At(b.Dx()/2, b.Dy()/2-40)).(color.NRGBA)
	}
	plain, withIcon := centre(nil, "plain.png"), centre(InFolder(dir), "icon.png")
	if plain == withIcon {
		t.Fatalf("the icon changed nothing: %v", plain)
	}
}

func TestGenerateFillsTheFaces(t *testing.T) {
	dir := t.TempDir()
	vanilla := color.NRGBA{10, 200, 10, 255}
	template := func(name string, size int) {
		img := image.NewNRGBA(image.Rect(0, 0, size, size))
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				img.SetNRGBA(x, y, vanilla)
			}
		}
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}
	template("BasicCardPack_texture.png", 1024)
	template("BasicCardBox_texture.png", 2048)
	template("BasicCardPack_icon.png", 512)
	template("BasicCardBox_icon.png", 512)

	r, err := Generate(dir, dir, "set", "Test Set", Options{Color: "#883344", Resolve: InFolder(dir)})
	if err != nil {
		t.Fatal(err)
	}
	load := func(rel string) image.Image {
		f, err := os.Open(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	painted := func(img image.Image, x, y int) bool {
		return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA) != vanilla
	}
	check := func(kind, rel string, faces ...string) {
		img := load(rel)
		m, _ := uvmap.ModelFor(kind)
		for _, f := range m.Faces {
			for _, want := range faces {
				if f.ID != want {
					continue
				}
				for _, tg := range f.Targets {
					x, y := int(tg.Rect[0]+tg.Rect[2])/2, int(tg.Rect[1]+tg.Rect[3])/2
					if !painted(img, x, y) {
						t.Errorf("%s %s: area %v keeps the template's print", kind, f.ID, tg.Rect)
					}
				}
			}
		}
	}
	check("Box", r.BoxTexture, "top", "bottom", "front", "back", "right", "left")
	check("Pack", r.PackTexture, "front", "back")

	// Where game 1.02's box front was: no face there now, so nothing is painted.
	if painted(load(r.BoxTexture), 300, 250) {
		t.Error("art was painted at the game 1.02 box front's place")
	}
}

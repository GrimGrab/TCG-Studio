package importer

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/uvmap"
)

// TestEPLOldArtStaysUnmarked: art from a bundle older than the game's update isn't marked as being in the current layouts, so
// Studio offers to convert it; a re-import from a current bundle marks it.
func TestEPLOldArtStaysUnmarked(t *testing.T) {
	root := t.TempDir()
	lib, err := accessories.Open(filepath.Join(root, "setup"), assets.For(root))
	if err != nil {
		t.Fatal(err)
	}
	lib.Put(setfmt.Accessory{ID: "x", Kind: "Sleeve"})
	markEPLArt(lib, "x", false)
	if lib.Meta.ArtCurrent["x"] {
		t.Error("an old bundle's accessory is marked as current")
	}
	markEPLArt(lib, "x", true)
	if !lib.Meta.ArtCurrent["x"] {
		t.Error("a current bundle's accessory isn't marked")
	}

	// A set: its first save marks it, and clearing that before the import's last save has to stick.
	ws := project.Workspace{Root: filepath.Join(root, "setup"), Library: filepath.Join(root, "library")}
	p, err := ws.Create("old", "Old")
	if err != nil {
		t.Fatal(err)
	}
	p.Meta.ArtKinds = nil
	if err := ws.Save(p); err != nil {
		t.Fatal(err)
	}
	if got := ws.ReadArtKinds("old"); len(got) != 0 {
		t.Errorf("an old bundle's set is marked %v", got)
	}
}

// TestBasicBoxLayoutOldAtlas: an old bundle's box on another base has its faces moved to the Basic box's areas of the 1.02
// layout (the conversion reads them there); a current bundle's texture is left alone.
func TestBasicBoxLayoutOldAtlas(t *testing.T) {
	model, _ := uvmap.LegacyModelFor("Box")
	var base *uvmap.Base
	for i := range model.Bases {
		if model.Bases[i].Targets != nil {
			base = &model.Bases[i]
			break
		}
	}
	if base == nil {
		t.Fatal("no 1.02 box base with its own areas")
	}
	var face uvmap.Face
	for _, f := range model.Faces {
		if len(base.Targets[f.ID]) > 0 {
			face = f
			break
		}
	}
	if face.ID == "" {
		t.Fatal("no face shared with the base")
	}
	S := model.TextureSize
	src := image.NewNRGBA(image.Rect(0, 0, S, S))
	red := color.NRGBA{255, 0, 0, 255}
	from := base.Targets[face.ID][0].Rect
	for y := int(from[1]); y < int(from[3]); y++ {
		for x := int(from[0]); x < int(from[2]); x++ {
			src.SetNRGBA(x, y, red)
		}
	}
	to := face.Targets[len(face.Targets)-1].Rect
	cx, cy := int(to[0]+to[2])/2, int(to[1]+to[3])/2
	if inFrom := float64(cx) >= from[0] && float64(cx) < from[2] && float64(cy) >= from[1] && float64(cy) < from[3]; inFrom {
		t.Skip("the base's area covers the Basic area")
	}
	if got := basicBoxLayout(src, base.ID, true); got != src {
		t.Error("a current bundle's texture was changed")
	}
	if got := basicBoxLayout(src, base.ID, false).NRGBAAt(cx, cy); got != red {
		t.Errorf("face %s wasn't moved to the Basic area: %v", face.ID, got)
	}
}

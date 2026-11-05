package relayout

import (
	"bytes"
	"image"
	"image/png"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

// TestOwnModelIsNotConverted: a card supply with its own mesh keeps its texture, whatever its base.
func TestOwnModelIsNotConverted(t *testing.T) {
	album := setfmt.NewAccessory("CardSupply", "a", "A", "")
	album.Base = "UP_Album"
	if KeyOf(album) != "UP_Album" {
		t.Fatalf("album key %q", KeyOf(album))
	}
	album.Mesh = "assets/own.obj"
	if KeyOf(album) != "" {
		t.Errorf("own model has key %q, want none", KeyOf(album))
	}
}

// TestBadAccessoryDoesNotBlockOthers: the good one converts and is marked, the bad one stays as it was and is named.
func TestBadAccessoryDoesNotBlockOthers(t *testing.T) {
	root := t.TempDir()
	l, err := accessories.Open(filepath.Join(root, "setup"), assets.For(root))
	if err != nil {
		t.Fatal(err)
	}
	var good bytes.Buffer
	if err := png.Encode(&good, image.NewNRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	goodTex, _ := l.PutBytes(good.Bytes(), ".png")
	badTex, _ := l.PutBytes([]byte("not a picture"), ".png")
	l.Put(setfmt.Accessory{ID: "bad", Kind: "Sleeve", Name: "Broken", Texture: badTex})
	l.Put(setfmt.Accessory{ID: "good", Kind: "Sleeve", Name: "Fine", Texture: goodTex})
	l.Meta.ArtCurrent = map[string]bool{} // as accessories from before marks

	changed, err := Accessories(l, "", []string{"Sleeve"})
	if !changed || err == nil || !strings.Contains(err.Error(), "Broken") {
		t.Fatalf("changed %v, err %v", changed, err)
	}
	if !l.Meta.ArtCurrent["good"] || l.Meta.ArtCurrent["bad"] {
		t.Errorf("marks: %v", l.Meta.ArtCurrent)
	}
	if l.Lib.Accessories[0].Texture != badTex {
		t.Error("the bad accessory's texture changed")
	}
	if l.Lib.Accessories[1].Texture == goodTex {
		t.Error("the good accessory wasn't converted")
	}
}

// TestNewArtIsMarked: what this version makes is never listed for conversion; older content (no mark) is, until marked.
func TestNewArtIsMarked(t *testing.T) {
	root := t.TempDir()
	ws := project.Workspace{Root: filepath.Join(root, "setup"), Library: filepath.Join(root, "library")}
	if _, err := ws.Create("new", "New"); err != nil {
		t.Fatal(err)
	}
	if got := ws.ReadArtKinds("new"); !slices.Equal(got, project.CurrentArtKinds) {
		t.Errorf("new set marked %v", got)
	}

	l, err := accessories.Open(ws.Root, assets.For(root))
	if err != nil {
		t.Fatal(err)
	}
	l.Put(setfmt.Accessory{ID: "new", Kind: "Sleeve"})
	if got := PendingAccessoryKeys(l, AccessoryKeys()); len(got) != 0 {
		t.Errorf("a new accessory is pending: %v", got)
	}
	delete(l.Meta.ArtCurrent, "new") // as an accessory from before marks
	if got := PendingAccessoryKeys(l, AccessoryKeys()); !slices.Equal(got, []string{"Sleeve"}) {
		t.Errorf("an old accessory isn't pending: %v", got)
	}
	if changed, err := Accessories(l, "", []string{"Sleeve"}); err != nil || !changed || !l.Meta.ArtCurrent["new"] {
		t.Errorf("converted: changed %v, err %v, marks %v", changed, err, l.Meta.ArtCurrent)
	}
}

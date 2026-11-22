package library

import (
	"context"
	"image/color"
	"os"
	"testing"

	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

// Shrink renames every picture a card names, not only the main one: back, version pictures and the mask point at the
// JPEGs too (they were left on deleted PNGs before).
func TestShrinkRepointsEveryPicture(t *testing.T) {
	h, _ := newHome(t)
	red, blue := pngBytes(t, color.NRGBA{200, 0, 0, 255}), pngBytes(t, color.NRGBA{0, 0, 200, 255})
	p := addSet(t, h, setups.DefaultID, red, blue)
	ws := workspace.Dir(h.Root).Projects(h.Dir(setups.DefaultID))
	put(t, p.ImagePath("images/gold.png"), blue)
	put(t, p.ImagePath("images/back.png"), red)
	c := &p.Set.Cards[0]
	c.VersionImages = map[string]string{"Gold": "images/gold.png"}
	c.BackImage = "images/back.png"
	if err := ws.Save(p); err != nil {
		t.Fatal(err)
	}
	if _, err := Shrink(context.Background(), h, []string{"x-set"}, nil); err != nil {
		t.Fatal(err)
	}
	q, err := ws.Load("x-set")
	if err != nil {
		t.Fatal(err)
	}
	c = &q.Set.Cards[0]
	for _, rel := range []string{c.Image, c.BackImage, c.VersionImages["Gold"]} {
		if !fsutil.Exists(q.ImagePath(rel)) {
			t.Errorf("%s points at a file that is gone", rel)
		}
	}
	if c.VersionImages["Gold"] != "images/gold.jpg" || c.BackImage != "images/back.jpg" {
		t.Errorf("not repointed: %+v / %s", c.VersionImages, c.BackImage)
	}
}

// RepairShrunk points pictures an older Shrink left on a deleted PNG at the JPEG beside it, and leaves alone a missing PNG
// with no JPEG.
func TestRepairShrunk(t *testing.T) {
	h, _ := newHome(t)
	red, blue := pngBytes(t, color.NRGBA{200, 0, 0, 255}), pngBytes(t, color.NRGBA{0, 0, 200, 255})
	p := addSet(t, h, setups.DefaultID, red, blue)
	ws := workspace.Dir(h.Root).Projects(h.Dir(setups.DefaultID))
	// What the older Shrink left: the JPEG written, the PNG deleted, only the main picture renamed.
	put(t, p.ImagePath("images/gold.jpg"), []byte("jpeg"))
	c := &p.Set.Cards[0]
	c.VersionImages = map[string]string{"Gold": "images/gold.png", "Silver": "images/lost.png"}
	if err := ws.Save(p); err != nil {
		t.Fatal(err)
	}
	fixed, err := RepairShrunk(h)
	if err != nil || len(fixed) != 1 {
		t.Fatalf("fixed %v %v", fixed, err)
	}
	q, _ := ws.Load("x-set")
	if got := q.Set.Cards[0].VersionImages; got["Gold"] != "images/gold.jpg" || got["Silver"] != "images/lost.png" {
		t.Fatalf("version images %+v", got)
	}
	// Nothing left to repair: a second run changes nothing.
	before, _ := os.ReadFile(ws.Folder("x-set") + "/set.json")
	if again, _ := RepairShrunk(h); len(again) != 0 {
		t.Fatalf("second run fixed %v", again)
	}
	if after, _ := os.ReadFile(ws.Folder("x-set") + "/set.json"); string(after) != string(before) {
		t.Fatal("second run rewrote set.json")
	}
}

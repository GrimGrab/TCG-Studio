package importer

import (
	"image"
	"testing"

	"tcgstudio/internal/epl"
)

func TestEPLDecoItems(t *testing.T) {
	d, err := epl.ParseDescriptor([]byte(`{"Prefabs": [
		{"Name": "Neon Sign", "PrefabName": "neon", "IsDecoObject": true, "DecoType": "Sign", "DecoObject": "NeonSign", "Price": 800},
		{"Name": "Brick", "IsDecoWallTexture": true, "IsDecoFloorTexture": true, "Texture": "brick", "Color": {"R": 1, "G": 0.5, "B": 0.5, "A": 1}, "Smoothness": 0.3},
		{"Name": "Lamp", "PrefabName": "lamp"}
	]}`))
	if err != nil {
		t.Fatal(err)
	}
	if c := d.Prefabs[1].Color; c == nil || c.G != 0.5 {
		t.Fatalf("colour not read: %+v", c)
	}
	b := &epl.Bundle{Path: `C:\mods\deco_bundle`, Rel: "deco.json", Desc: d}
	items, skipped := eplItems(b, nil, false)
	if len(items) != 3 {
		t.Fatalf("items %+v", items)
	}
	want := []struct{ base, id string }{
		{"Object", "eplmod-deco-bundle-neon-sign"},
		{"Wall", "eplmod-deco-bundle-brick"},
		{"Floor", "eplmod-deco-bundle-brick-floor"},
	}
	for i, w := range want {
		if items[i].Kind != "Decoration" || items[i].Base != w.base || items[i].ID != w.id {
			t.Errorf("item %d = %+v, want %s %s", i, items[i], w.base, w.id)
		}
	}
	if len(skipped) != 1 || skipped[0].Name != "Lamp" {
		t.Fatalf("skipped %+v", skipped)
	}
}

func TestUnswizzleNormal(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 255, 128, 0, 200 // DXT5nm: x in alpha, y in green
	}
	out := unswizzleNormal(img)
	if out.Pix[0] != 200 || out.Pix[1] != 128 || out.Pix[2] < 200 || out.Pix[3] != 255 {
		t.Fatalf("got %v", out.Pix[:4])
	}
	plain := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for i := 0; i < len(plain.Pix); i += 4 {
		plain.Pix[i], plain.Pix[i+1], plain.Pix[i+2], plain.Pix[i+3] = 128, 128, 255, 255
	}
	if unswizzleNormal(plain) != plain {
		t.Fatal("plain normal map changed")
	}
}

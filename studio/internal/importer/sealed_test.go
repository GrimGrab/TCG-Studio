package importer

import (
	"context"
	"image"
	"image/color"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestSealedKind(t *testing.T) {
	for name, want := range map[string]string{
		"Bloomburrow - Play Booster Pack":                     "pack",
		"Bloomburrow - Collector Booster Pack":                "pack",
		"Tempest - Booster Pack":                              "pack",
		"151 Booster Pack":                                    "pack",
		"Bloomburrow - Play Booster Display":                  "box",
		"Tempest - Booster Box":                               "box",
		"Bloomburrow - Play Booster Display Case":             "other",
		"Bloomburrow - Prerelease Pack":                       "other",
		"Bloomburrow - Sleeved Play Booster Pack":             "other",
		"151 Booster Bundle":                                  "other",
		"151 Elite Trainer Box":                               "other",
		"Bloomburrow - Bundle":                                "other",
		"Bloomburrow - Tin (Boat)":                            "other",
		"Mirrodin Theme Deck - Little Bashers":                "other",
		"Monarch Booster Box [1st Edition]":                   "box",
		"Monarch Blitz Deck Display":                          "box",
		"Bloomburrow - Collector Booster Display Master Case": "other",
	} {
		if got := sealedKind(name); got != want {
			t.Errorf("sealedKind(%q) = %s, want %s", name, got, want)
		}
	}
}

func TestMatchGroup(t *testing.T) {
	gs := []tcgGroup{
		{ID: 1, Name: "Art Series: Bloomburrow"}, {ID: 2, Name: "Bloomburrow", Abbreviation: "BLB"},
		{ID: 3, Name: "Commander: Bloomburrow", Abbreviation: "BLC"}, {ID: 4, Name: "SV: Scarlet & Violet 151", Abbreviation: "MEW"},
		{ID: 5, Name: "SV: Scarlet & Violet 151 Mini Tins"}, {ID: 6, Name: "Base Set", Abbreviation: "BS"},
	}
	for _, c := range []struct {
		code, name string
		want       int
	}{
		{"blb", "Bloomburrow", 2}, {"", "Bloomburrow", 2}, {"blc", "", 3}, {"sv03.5", "151", 4}, {"base1", "Base Set", 6},
		{"xyz", "Nothing like it", 0}, {"", "", 0},
	} {
		if got := matchGroup(gs, c.code, c.name); got != c.want {
			t.Errorf("matchGroup(%q, %q) = %d, want %d", c.code, c.name, got, c.want)
		}
	}
}

func TestTrimBackground(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 200))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}
	for y := 20; y < 180; y++ {
		for x := 10; x < 90; x++ {
			img.Set(x, y, color.RGBA{40, 90, 160, 255})
		}
	}
	if got := TrimBackground(img).Bounds(); got != image.Rect(10, 20, 90, 180) {
		t.Errorf("trimmed to %v", got)
	}
}

// TestSealedLive lists real products and fetches the first pack photo:
// SEALED_LIVE="1:BLB:Bloomburrow,3:sv03.5:151,1:TMP:Tempest" go test ./internal/importer -run SealedLive -v
func TestSealedLive(t *testing.T) {
	spec := os.Getenv("SEALED_LIVE")
	if spec == "" {
		t.Skip("set SEALED_LIVE=<category>:<code>:<name>[,…] to run")
	}
	s := NewSealed()
	ctx := context.Background()
	for _, item := range strings.Split(spec, ",") {
		parts := strings.SplitN(item, ":", 3)
		cat, _ := strconv.Atoi(parts[0])
		t.Run(item, func(t *testing.T) {
			_, g, err := s.Groups(ctx, cat, parts[1], parts[2])
			if err != nil || g == 0 {
				t.Fatalf("no group (%v)", err)
			}
			ps, err := s.Products(ctx, cat, g)
			if err != nil || len(ps) == 0 || ps[0].Kind != "pack" {
				t.Fatalf("products: %v %+v", err, ps)
			}
			for _, p := range ps {
				t.Logf("%-5s %d %s", p.Kind, p.ID, p.Name)
			}
			img, err := s.Photo(ctx, ps[0])
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("group %d: pack photo %v", g, img.Bounds())
		})
	}
}

func TestPickPackAndBox(t *testing.T) {
	ps := []SealedProduct{
		{ID: 1, Name: "Bloomburrow - Play Booster Pack", Kind: "pack"}, {ID: 2, Name: "Bloomburrow - Collector Booster Pack", Kind: "pack"},
		{ID: 3, Name: "Bloomburrow - Collector Booster Sample Pack", Kind: "pack"}, {ID: 4, Name: "Bloomburrow - Play Booster Display", Kind: "box"},
		{ID: 5, Name: "Bloomburrow - Collector Booster Display", Kind: "box"}, {ID: 6, Name: "Bloomburrow - Bundle", Kind: "other"},
	}
	for _, c := range []struct{ index, pack, box int }{{0, 1, 4}, {1, 2, 5}, {2, 3, 5}, {9, 3, 5}} {
		p, b := PickPackAndBox(ps, c.index)
		if p == nil || b == nil || p.ID != c.pack || b.ID != c.box {
			t.Errorf("index %d: got %v %v, want pack %d box %d", c.index, p, b, c.pack, c.box)
		}
	}
	tmp := []SealedProduct{{ID: 7, Name: "Tempest - Booster Pack", Kind: "pack"}, {ID: 8, Name: "Tempest Tournament Pack", Kind: "pack"},
		{ID: 9, Name: "Tempest - Booster Box", Kind: "box"}, {ID: 10, Name: "Tempest Tournament Pack Display", Kind: "box"}}
	if p, b := PickPackAndBox(tmp, 0); p.ID != 7 || b.ID != 9 {
		t.Errorf("tempest: %v %v", p, b)
	}
	if p, b := PickPackAndBox(nil, 0); p != nil || b != nil {
		t.Error("empty list should give nothing")
	}
}

func TestPickPackAndBoxSkipsDeckDisplays(t *testing.T) {
	ps := []SealedProduct{{ID: 1, Name: "ST-33: Starter Deck 33 BLUE Kuzan Display", Kind: "box"}, {ID: 2, Name: "Commander: Bloomburrow - Commander Deck Display", Kind: "box"}}
	if _, b := PickPackAndBox(ps, 0); b != nil {
		t.Errorf("picked %s as a booster box", b.Name)
	}
}

package importer

import (
	"image"
	"math"
	"slices"
	"testing"

	"tcgstudio/internal/scryfall"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/tcgdex"
)

// Every source's rarity order and default rarity map list the same rarities, map only to game rarities with an icon,
// and its facets/sorts are well formed.
func TestSourcesConsistent(t *testing.T) {
	reg := NewRegistry(scryfall.New(), tcgdex.New())
	ids := map[string]bool{}
	for _, s := range reg.All() {
		info := s.Info()
		if ids[info.ID] {
			t.Errorf("duplicate source id %q", info.ID)
		}
		ids[info.ID] = true
		m := s.DefaultOptions().RarityMap
		for r, g := range m {
			if !slices.Contains(info.RarityOrder, r) && info.ID != "tcgdex" { // tcgdex: own test (Pocket rarities)
				t.Errorf("%s: %q is mapped but not in RarityOrder", info.ID, r)
			}
			if !slices.Contains(setfmt.Rarities, g) || g == "SuperLegend" {
				t.Errorf("%s: %q maps to %q", info.ID, r, g)
			}
		}
		for _, r := range info.RarityOrder {
			if _, ok := m[r]; !ok {
				t.Errorf("%s: %q is in RarityOrder but not mapped", info.ID, r)
			}
		}
		if (len(info.Colors) == 0 || info.ColorLabel == "") && !info.Local { // local files have no fixed colours
			t.Errorf("%s: no colour/type filter", info.ID)
		}
		for _, f := range info.Sorts {
			if f.Value != "cost" && f.Value != "power" {
				t.Errorf("%s: unknown sort %q", info.ID, f.Value)
			}
		}
		if id := s.ProjectID("Ab.1", ""); !setfmt.SafeID(id) {
			t.Errorf("%s: unsafe project id %q", info.ID, id)
		}
	}
}

func TestNaturalLess(t *testing.T) {
	in := []string{"OP01-010", "OP01-002_p1", "OP01-002", "10", "9", "LOB-EN100", "LOB-EN020"}
	slices.SortStableFunc(in, func(a, b string) int {
		if naturalLess(a, b) {
			return -1
		}
		if naturalLess(b, a) {
			return 1
		}
		return 0
	})
	want := []string{"9", "10", "LOB-EN020", "LOB-EN100", "OP01-002", "OP01-002_p1", "OP01-010"}
	if !slices.Equal(in, want) {
		t.Errorf("got %v", in)
	}
}

func TestFitImage(t *testing.T) {
	ygo := image.NewNRGBA(image.Rect(0, 0, 421, 614)) // YGOPRODeck card image
	got, ok := fitImage(ygo, CardAspect, 0)
	if r := float64(got.Bounds().Dx()) / float64(got.Bounds().Dy()); !ok || got.Bounds().Dy() != 614 || math.Abs(r-CardAspect) > 0.003 {
		t.Errorf("stretch: %v %v", got.Bounds(), ok)
	}
	got, _ = fitImage(ygo, CardAspect, 384)
	if got.Bounds().Dx() != 384 || math.Abs(float64(384)/float64(got.Bounds().Dy())-CardAspect) > 0.005 {
		t.Errorf("stretch + shrink: %v", got.Bounds())
	}
	mtg := image.NewNRGBA(image.Rect(0, 0, 745, 1040))
	if _, ok := fitImage(mtg, CardAspect, 0); ok {
		t.Error("a 63×88 image should be left alone")
	}
}

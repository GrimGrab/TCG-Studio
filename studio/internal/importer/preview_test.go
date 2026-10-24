package importer

import (
	"context"
	"testing"

	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

type fakePreviewSource struct{ in setIn }

func (f fakePreviewSource) Info() SourceInfo {
	return SourceInfo{ID: "fake", Name: "Fake", RarityOrder: []string{"Common", "Rare", "Mythic"}}
}
func (f fakePreviewSource) DefaultOptions() Options {
	return Options{RarityMap: map[string]string{"Common": "Common", "Rare": "Epic", "Mythic": "Legendary"}}
}
func (f fakePreviewSource) Sets(context.Context, string) ([]SetInfo, error) { return nil, nil }
func (f fakePreviewSource) ProjectID(code, _ string) string                 { return "fake-" + code }
func (f fakePreviewSource) Import(context.Context, project.Workspace, string, Options, func(Progress)) (*project.Project, error) {
	return nil, nil
}
func (f fakePreviewSource) RefreshMeta(context.Context, *project.Project) (int, error) { return 0, nil }
func (f fakePreviewSource) prepare(_ context.Context, _ string, opt *Options, _ func(Progress)) (setIn, error) {
	in := f.in
	in.Rarity = rarityMapper(opt.RarityMap, func(string) string { return "Common" })
	return in, nil
}

func TestPreview(t *testing.T) {
	p := func(v float64) *float64 { return &v }
	src := fakePreviewSource{setIn{Name: "Test", Cards: []cardIn{
		{Name: "A", Number: "1", SrcRarity: "Mythic", USD: p(10), Image: "https://tcgplayer-cdn.tcgplayer.com/product/1_in_1000x1000.jpg"},
		{Name: "B", Number: "2", SrcRarity: "Common", USD: p(0.1)},
		{Name: "B", Number: "2", SrcRarity: "Common", Variant: []string{"Foil"}, EUR: p(2)},
		{Name: "C", Number: "3", SrcRarity: "Rare"},
	}, Slots: []setfmt.Slot{slot(2, cC), slot(1, cR), slot(1, w("Legendary", 1))}, FoilChance: 5}}
	got, err := Preview(context.Background(), src, "t", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Cards != 4 || got.Variants != 1 || got.Priced != 3 || got.Value < 12.09 || got.Value > 12.11 {
		t.Errorf("counts: %+v", got)
	}
	if len(got.Rarities) != 3 || got.Rarities[0].Name != "Common" || got.Rarities[0].Count != 2 || got.Rarities[2].Game != "Legendary" {
		t.Errorf("rarities in the source's order: %+v", got.Rarities)
	}
	if len(got.Top) != 3 || got.Top[0].Name != "A" || got.Top[1].Price == nil || *got.Top[1].Price != 2 {
		t.Errorf("top cards (priced only, EUR fallback): %+v", got.Top)
	}
	if got.All[0].Thumb != "https://tcgplayer-cdn.tcgplayer.com/product/1_200w.jpg" {
		t.Errorf("thumb %s", got.All[0].Thumb)
	}
	if got.Pack.Cards != 4 {
		t.Errorf("pack %+v", got.Pack)
	}
}

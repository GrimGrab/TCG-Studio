package importer

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestTCGGames(t *testing.T) {
	ids, prefixes, cats := map[string]bool{}, map[string]bool{}, map[int]bool{}
	// project id prefixes of the other sources
	for _, p := range []string{"mtg", "ptcg", "ygo", "op", "swu", "lor", "fab", "ua", "img", "epl"} {
		prefixes[p] = true
	}
	for _, c := range []int{1, 2, 3, fabCategory, opCategory, 71, 79, uaCategory} {
		cats[c] = true
	}
	srcs := newTCGGameSources() // panics on a bad regexp
	if len(srcs) != len(tcgGames) {
		t.Fatal("one source per game")
	}
	for _, g := range tcgGames {
		if !strings.HasPrefix(g.ID, "tcg-") || g.Game == "" || g.Prefix == "" || g.Category == 0 {
			t.Errorf("%+v: ID tcg-…, Game, Prefix and Category are required", g.ID)
		}
		if ids[g.ID] || prefixes[g.Prefix] || cats[g.Category] {
			t.Errorf("%s: id, prefix %q or category %d used twice", g.ID, g.Prefix, g.Category)
		}
		ids[g.ID], prefixes[g.Prefix], cats[g.Category] = true, true, true
		if len(g.Colors) > 0 && g.Color == "" && g.ColorLabel != "Type" {
			t.Errorf("%s: Colors without the Color field", g.ID)
		}
		if (g.Cost == "") != (g.CostLabel == "") || (g.Power == "") != (g.PowerLabel == "") {
			t.Errorf("%s: Cost/Power need their sort label and the other way round", g.ID)
		}
		seen := map[string]bool{}
		for _, tier := range g.Ladder {
			for _, r := range tier {
				if seen[r] {
					t.Errorf("%s: rarity %q twice in the ladder", g.ID, r)
				}
				seen[r] = true
			}
		}
		if g.Booster != nil {
			n := 0
			for _, s := range g.Booster.Slots {
				if s.Count <= 0 {
					t.Errorf("%s: empty booster slot", g.ID)
				}
				n += s.Count
			}
			if !strings.Contains(g.Booster.Name, " "+strconv.Itoa(n)+" cards") {
				t.Errorf("%s: booster %q has %d cards", g.ID, g.Booster.Name, n)
			}
		}
	}
	if SealedCategory("tcg-gundam") != 86 {
		t.Error("product photos must find the game's TCGplayer category")
	}
}

func tcgTestProduct(id int, name, number, rarity string, ext ...string) tcgProduct {
	p := tcgProduct{ID: id, Name: name, ImageURL: "https://img/x_200w.jpg", Images: 1}
	if number != "" {
		ext = append(ext, "Number", number)
	}
	if rarity != "" {
		ext = append(ext, "Rarity", rarity)
	}
	for i := 0; i+1 < len(ext); i += 2 {
		p.Extended = append(p.Extended, struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}{ext[i], ext[i+1]})
	}
	return p
}

func gundamSource(t *testing.T) *tcgGameSource {
	for _, s := range newTCGGameSources() {
		if s.Info().ID == "tcg-gundam" {
			return s.(*tcgGameSource)
		}
	}
	t.Fatal("no Gundam source")
	return nil
}

func TestTCGParseGundam(t *testing.T) {
	s := gundamSource(t)
	unit := []string{"CardType", "Unit", "Color", "Blue", "Cost", "3", "Attack Points", "3", "Trait", "(Earth Federation)",
		"Description", "<b>Repair 1</b><br>Draw 1.", "Link Condition", "[Amuro Ray]"}
	noImage := tcgTestProduct(9, "Zaku", "GD01-099", "Common")
	noImage.Images = 0
	products := []tcgProduct{
		tcgTestProduct(1, "Gundam (GD01-001)", "GD01-001", "Legend Rare", unit...),
		tcgTestProduct(2, "Gundam (GD01-001) (LR+)", "GD01-001", "LR+", unit...),
		tcgTestProduct(3, "Unicorn Gundam (Destroy Mode)", "GD01-002", "Legend Rare", "Color", "Red"),
		tcgTestProduct(4, "Resource (R-002) (C+)", "R-002", "C+"),
		tcgTestProduct(5, "Fatum-00 (T-011) Token", "T-011", "Common"),
		tcgTestProduct(6, "Newtype Rising Booster Box", "", ""),
		tcgTestProduct(7, "Starter Box", "", "None", "Description", "Contents…"),
		noImage,
	}
	prices := map[int]map[string]float64{1: {"Normal": 2.5, "Holofoil": 9}, 2: {"Holofoil": 40}}
	cards, missing := s.parse(products, prices, true)
	if missing != 1 {
		t.Errorf("cards without a picture: %d, want 1", missing)
	}
	if len(cards) != 3 {
		t.Fatalf("got %d cards, want 3 (sealed, token, resource skipped): %+v", len(cards), cards)
	}
	base, plus, unicorn := cards[0], cards[1], cards[2]
	if base.Name != "Gundam" || len(base.Variant) != 0 || base.Number != "GD01-001" {
		t.Errorf("base printing: %+v", base)
	}
	if plus.Name != "Gundam" || !slices.Equal(plus.Variant, []string{"LR+"}) {
		t.Errorf("parallel: name %q variant %v", plus.Name, plus.Variant)
	}
	if unicorn.Name != "Unicorn Gundam (Destroy Mode)" || len(unicorn.Variant) != 0 || unicorn.Element != "Fire" {
		t.Errorf("mode tag is part of the name: %+v", unicorn)
	}
	if base.Cost != 3 || base.Power != "3" || base.Element != "Water" || !slices.Equal(base.Colors, []string{"Blue"}) {
		t.Errorf("stats: cost %v power %q element %q colors %v", base.Cost, base.Power, base.Element, base.Colors)
	}
	if base.TypeLine != "Unit — (Earth Federation)" || base.Text != "Repair 1\nDraw 1.\nLink Condition: [Amuro Ray]" {
		t.Errorf("type %q text %q", base.TypeLine, base.Text)
	}
	if base.USD == nil || *base.USD != 2.5 || base.USDFoil == nil || *base.USDFoil != 9 || plus.USD == nil || *plus.USD != 40 {
		t.Errorf("prices: %v %v / %v", base.USD, base.USDFoil, plus.USD)
	}
	if !strings.Contains(base.Image, "_in_1000x1000.") {
		t.Errorf("large scan: %s", base.Image)
	}
	if cards, _ := s.parse(products, prices, false); len(cards) != 2 {
		t.Errorf("without variants: %d cards, want 2", len(cards))
	}
	m := s.DefaultOptions().RarityMap
	if m["Common"] != "Common" || m["Uncommon"] != "Rare" || m["Rare"] != "Epic" || m["LR+"] != "Legendary" {
		t.Errorf("rarity map: %v", m)
	}
}

func TestTCGSkipTags(t *testing.T) {
	var aos *tcgGameSource
	for _, s := range newTCGGameSources() {
		if s.Info().ID == "tcg-aoschampions" {
			aos = s.(*tcgGameSource)
		}
	}
	cards, _ := aos.parse([]tcgProduct{
		tcgTestProduct(1, "Lord-Celestant (Claimed)", "1", "Rare"),
		tcgTestProduct(2, "Lord-Celestant (Unclaimed)", "1", "Rare"),
	}, nil, true)
	if len(cards) != 1 || cards[0].Name != "Lord-Celestant" || len(cards[0].Variant) != 0 {
		t.Errorf("claimed copy skipped, unclaimed tag dropped: %+v", cards)
	}
}

func TestTCGRarities(t *testing.T) {
	var cards []cardIn
	add := func(r string, n int) {
		for range n {
			cards = append(cards, cardIn{SrcRarity: r})
		}
	}
	add("Ordinary", 50)
	add("Exceptional", 20)
	add("Unique", 5)
	add("Full Art", 3)
	add("Rare", 8)
	order, tiers := tcgRankRarities(cards)
	if !slices.Equal(order, []string{"Ordinary", "Exceptional", "Rare", "Unique", "Full Art"}) {
		t.Errorf("order %v", order)
	}
	if tiers["Ordinary"] != "Common" || tiers["Exceptional"] != "Epic" || tiers["Full Art"] != "Legendary" || tiers["Rare"] != "Epic" {
		t.Errorf("tiers %v", tiers)
	}
	for r, want := range map[string]string{"Starter Exclusive": "Common", "LR+": "Legendary", "Promo": "Epic", "Uncommon": "Rare",
		"Secret Rare": "Legendary", "Holo Rare": "Epic", "Ordinary": ""} {
		if got := tcgTier(r); got != want {
			t.Errorf("tcgTier(%q) = %q, want %q", r, got, want)
		}
	}
	// A rarity the game's ladder doesn't know goes after the listed ones of its tier.
	s := gundamSource(t)
	got := s.mergeOrder([]string{"Common", "Full Art", "Uncommon Special"}, map[string]string{"Full Art": "Legendary", "Uncommon Special": "Rare"})
	if i, j := slices.Index(got, "Uncommon Special"), slices.Index(got, "Uncommon"); i != j+1 {
		t.Errorf("merged order %v", got)
	}
	if got[len(got)-1] != "Full Art" {
		t.Errorf("merged order %v", got)
	}
}

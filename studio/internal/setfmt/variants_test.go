package setfmt

import "testing"

func TestVariants(t *testing.T) {
	for _, v := range []string{"Base", "Base_foil", "fullart_foil", "EX", "FirstEdition_foil"} {
		if !IsVariant(v) {
			t.Errorf("%q should be a card version", v)
		}
	}
	for _, v := range []string{"", "Foil", "Base_holo", "Platinum"} {
		if IsVariant(v) {
			t.Errorf("%q isn't a card version", v)
		}
	}
	s := &Set{Variants: []string{"Base", "FullArt_foil"}}
	if !s.AllowsVariant("Base", false) || s.AllowsVariant("Base", true) || !s.AllowsVariant("FullArt", true) {
		t.Error("AllowsVariant follows the list")
	}
	if !(&Set{}).AllowsVariant("Gold", true) {
		t.Error("no list = all 12")
	}
	bad := &Set{ID: "x", Name: "x", Variants: []string{"Shiny"}}
	found := false
	for _, is := range bad.Validate() {
		if is.Level == "error" && is.Where == "set" {
			found = true
		}
	}
	if !found {
		t.Error("unknown version should be an error")
	}
}

func TestCardVersions(t *testing.T) {
	s := &Set{Variants: []string{"Base", "Gold"}}
	own := &Card{Variants: []string{"FullArt_foil"}}
	follow := &Card{}
	if !s.CardAllowsVariant(own, "FullArt", true) || s.CardAllowsVariant(own, "Base", false) {
		t.Error("a card's own list replaces the set's")
	}
	if !s.CardAllowsVariant(follow, "Gold", false) || s.CardAllowsVariant(follow, "Gold", true) {
		t.Error("no own list = the set's")
	}

	c := Card{ID: "a", Image: "images/a.png", BackImage: "images/a_back.png",
		VersionImages: map[string]string{"Gold_foil": "images/a_gf.png", "Base": "images/a.png", "EX": ""}}
	got := c.Images()
	want := []string{"images/a.png", "images/a_back.png", "images/a_gf.png"}
	if len(got) != len(want) {
		t.Fatalf("Images() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Images() = %v, want %v", got, want)
		}
	}
	c.RepointImages(func(s *string) {
		if *s != "" {
			*s = "new/" + *s
		}
	})
	if c.VersionImages["Gold_foil"] != "new/images/a_gf.png" || c.Image != "new/images/a.png" {
		t.Errorf("RepointImages missed a path: %+v", c)
	}

	bad := &Set{ID: "x", Name: "x", Cards: []Card{{ID: "a", Name: "a", Rarity: "Common", Play: Play{LaneAttack: []int{1, 1, 1, 1}, Element: "Fire"},
		Variants: []string{"Shiny"}, VersionImages: map[string]string{"Holo": "x.png"}}}}
	n := 0
	for _, is := range bad.Validate() {
		if is.Level == "error" && is.Where == "card a" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("unknown card version names should be 2 errors, got %d: %+v", n, bad.Validate())
	}
}

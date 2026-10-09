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

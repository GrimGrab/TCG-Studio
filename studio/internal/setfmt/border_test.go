package setfmt

import (
	"testing"
)

func TestBorderOf(t *testing.T) {
	none := ""
	s := &Set{Borders: map[string]BorderSlot{
		"Gold":   {Look: "leaf"},
		"silver": {Look: "plain", FoilLook: &none},
		"EX":     {FoilLook: strPtr("shiny")},
	}}
	c := &Card{VersionBorders: map[string]string{"gold_foil": "own", "Base": ""}}
	cases := []struct {
		card   *Card
		border string
		foil   bool
		want   string
	}{
		{nil, "Gold", false, "leaf"},
		{nil, "Gold", true, "leaf"}, // no foilLook = the normal one
		{nil, "Silver", true, ""},   // foilLook "" = the game's rim
		{nil, "Silver", false, "plain"},
		{nil, "EX", false, ""},
		{nil, "EX", true, "shiny"},
		{c, "Gold", true, "own"},
		{c, "Gold", false, "leaf"},
		{nil, "Base", false, ""},
	}
	for _, tc := range cases {
		if got := s.BorderOf(tc.card, tc.border, tc.foil); got != tc.want {
			t.Errorf("BorderOf(%s, foil=%v) = %q, want %q", tc.border, tc.foil, got, tc.want)
		}
	}
	if ids := s.BorderIDs(); len(ids) != 3 {
		t.Errorf("BorderIDs = %v, want leaf, plain, shiny", ids)
	}
	s.Cards = []Card{*c}
	s.RenameBorders(map[string]string{"leaf": "leaf2", "own": "own2", "shiny": "shiny2"})
	if s.Borders["Gold"].Look != "leaf2" || *s.Borders["EX"].FoilLook != "shiny2" || s.Cards[0].VersionBorders["gold_foil"] != "own2" {
		t.Errorf("RenameBorders missed one: %+v %+v", s.Borders, s.Cards[0].VersionBorders)
	}
}

func TestValidateBorders(t *testing.T) {
	ok := []Border{{ID: "a", Name: "A", Layers: []BorderLayer{{Kind: BorderImageLayer, File: "assets/x.png", Opacity: 1}, {Kind: BorderShaderLayer, Shader: "s", Clip: true}}}}
	if issues := ValidateBorders(ok); len(issues) != 0 {
		t.Errorf("valid border refused: %v", issues)
	}
	bad := []Border{
		{ID: "a", Name: "A", Window: []float64{0, 0, 1}, Layers: []BorderLayer{{Kind: "paint"}, {Kind: BorderImageLayer}, {Kind: BorderShaderLayer, Params: map[string][]float64{"x": {1, 2, 3}}}}},
		{ID: "a"},
	}
	// window, unknown kind, picture missing, shader missing, 3 numbers, duplicate id, no name
	if issues := ValidateBorders(bad); len(issues) != 7 {
		t.Errorf("got %d issues, want 7: %v", len(issues), issues)
	}
	set := &Set{Borders: map[string]BorderSlot{"Golden": {Look: "a"}}}
	found := false
	for _, is := range set.Validate() {
		if is.Level == "error" && is.Where == "set" {
			found = true
		}
	}
	if !found {
		t.Error("an unknown border slot name should be an error")
	}
}

func strPtr(s string) *string { return &s }


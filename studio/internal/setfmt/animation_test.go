package setfmt

import (
	"strings"
	"testing"
)

func TestValidateAnimations(t *testing.T) {
	ok := Animation{ID: "anim_a", Name: "Wave", Length: 1, Mask: "upper", Bones: map[string][]AnimKey{
		"upperarm_r": {{T: 0, R: [4]float64{0, 0, 0, 1}}, {T: 1, R: [4]float64{0, 0, 0.7071068, 0.7071068}}},
	}}
	l := AccessoryLibrary{Animations: []Animation{ok}}
	if errs, _ := l.ValidateAnimations(); len(errs) > 0 {
		t.Fatalf("valid animation: %v", errs)
	}
	bad := []struct {
		name string
		edit func(a *Animation, l *AccessoryLibrary)
		want string
	}{
		{"length", func(a *Animation, _ *AccessoryLibrary) { a.Length = 0 }, "length"},
		{"mask", func(a *Animation, _ *AccessoryLibrary) { a.Mask = "legs" }, "mask"},
		{"key past the end", func(a *Animation, _ *AccessoryLibrary) { a.Bones["upperarm_r"][1].T = 2 }, "bad key"},
		{"keys out of order", func(a *Animation, _ *AccessoryLibrary) { a.Bones["upperarm_r"][1].T = 0 }, "bad key"},
		{"not a rotation", func(a *Animation, _ *AccessoryLibrary) { a.Bones["upperarm_r"][0].R = [4]float64{1, 1, 0, 0} }, "bad key"},
	}
	for _, c := range bad {
		a := ok
		a.Bones = map[string][]AnimKey{"upperarm_r": append([]AnimKey{}, ok.Bones["upperarm_r"]...)}
		lib := AccessoryLibrary{Animations: []Animation{a}}
		c.edit(&lib.Animations[0], &lib)
		errs, _ := lib.ValidateAnimations()
		if len(errs) == 0 || !strings.Contains(strings.Join(errs, "\n"), c.want) {
			t.Errorf("%s: want an error with %q, got %v", c.name, c.want, errs)
		}
	}
}

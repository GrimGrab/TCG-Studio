package customers

import (
	"testing"

	"tcgstudio/internal/setfmt"
)

func TestDefaultAnimations(t *testing.T) {
	a, err := LoadDefaultAnimations()
	if err != nil {
		t.Fatal(err)
	}
	d, err := LoadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	lib := setfmt.AccessoryLibrary{Animations: a.Animations}
	if errs, warns := lib.ValidateAnimations(); len(errs)+len(warns) > 0 {
		t.Errorf("%v %v", errs, warns)
	}
	ids := map[string]bool{}
	for _, x := range a.Animations {
		ids[x.ID] = true
	}
	moments := map[string]bool{}
	for _, s := range d.EmployeeSituations {
		moments[s.ID] = true
	}
	for _, s := range d.Situations {
		moments[s.ID] = true
	}
	for _, m := range setfmt.AnimationMoments {
		moments[m.ID] = true
	}
	for m, list := range a.Moves {
		if !moments[m] {
			t.Errorf("moment %q doesn't exist", m)
		}
		for _, id := range list {
			if !ids[id] {
				t.Errorf("moment %q names unknown animation %q", m, id)
			}
		}
	}
}

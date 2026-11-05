package setfmt

import "testing"

func TestValidateEmployees(t *testing.T) {
	ok := Employee{ID: "a", Name: "Sam", Model: -1, RestockSpeed: 2, CheckoutSpeed: 2, Wage: 100, HireCost: 500, Level: 1, Perks: []string{"chatty"}}
	known := map[string]bool{"chatty": true}
	cases := []struct {
		name string
		bad  func(*Employee)
	}{
		{"no id", func(e *Employee) { e.ID = " " }},
		{"no name", func(e *Employee) { e.Name = " " }},
		{"look too high", func(e *Employee) { e.Model = 9999 }},
		{"face without a character", func(e *Employee) { e.Face = &CustomerFace{} }},
		{"zero speed", func(e *Employee) { e.CheckoutSpeed = 0 }},
		{"negative wage", func(e *Employee) { e.Wage = -1 }},
		{"voice out of range", func(e *Employee) { e.Voice = &CustomerVoice{Pitch: 9, Tone: 0.5} }},
	}
	l := AccessoryLibrary{Employees: []Employee{ok}}
	if errs, warns := l.ValidateEmployees(known); len(errs)+len(warns) > 0 {
		t.Errorf("a good employee was refused: %v %v", errs, warns)
	}
	for _, c := range cases {
		e := ok
		c.bad(&e)
		l := AccessoryLibrary{Employees: []Employee{e}}
		if errs, _ := l.ValidateEmployees(known); len(errs) == 0 {
			t.Errorf("%s: not refused", c.name)
		}
	}
	two := AccessoryLibrary{Employees: []Employee{ok, ok}}
	if errs, _ := two.ValidateEmployees(known); len(errs) == 0 {
		t.Error("a repeated id wasn't refused")
	}
	unknown := ok
	unknown.Perks = []string{"nope"}
	l = AccessoryLibrary{Employees: []Employee{unknown}}
	if _, warns := l.ValidateEmployees(known); len(warns) == 0 {
		t.Error("an unknown perk didn't warn")
	}
}

package setfmt

import (
	"strings"
	"testing"
)

// Out-of-range label values are errors on the set and on a card; left-out fields are fine.
func TestEditionLabelValidate(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	s := &Set{ID: "s", Name: "S", EditionLabel: &EditionLabel{Size: f(5)},
		Cards: []Card{{ID: "a", Name: "A", EditionLabel: &EditionLabel{Y: f(-41)}}, {ID: "b", Name: "B", EditionLabel: &EditionLabel{X: f(10), Y: f(65)}}}}
	var got []string
	for _, i := range s.Validate() {
		if strings.Contains(i.Message, "editionLabel") {
			got = append(got, i.Where+": "+i.Message)
		}
	}
	if len(got) != 2 || !strings.Contains(got[0], "size") || !strings.Contains(got[1], "y must be") {
		t.Errorf("want a size error on the set and a y error on card a (65 is allowed), got %v", got)
	}
}

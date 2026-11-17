package cardshader

import "testing"

// A setup's own stock values keep only what the mod can use and what differs from the stock shader.
func TestCleanStockValues(t *testing.T) {
	base := StockPrefix + "base"
	got := CleanStockValues(map[string]map[string][]float64{
		base:                 {"strength": {1.2}, "tiling": {2.882}, "tint": {1, 0, 0}, "nope": {1}},
		"unknown-id":         {"strength": {1}},
		StockPrefix + "gold": {"strength": {0.31}}, // the stock value: nothing of its own
	})
	if len(got) != 1 || len(got[base]) != 1 || got[base]["strength"][0] != 1.2 {
		t.Fatalf("cleaned to %v", got)
	}
	if CleanStockValues(nil) != nil {
		t.Fatal("nothing given should clean to nil")
	}
	for _, s := range Stock() {
		if s.ID == base {
			if w := s.WithValues(got[base]); w.Params[3].Name != "strength" || w.Params[3].Value[0] != 1.2 || s.Params[3].Value[0] == 1.2 {
				t.Fatal("WithValues must replace the value on a copy")
			}
		}
	}
}

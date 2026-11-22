package cardborder

import (
	"strings"
	"testing"

	"tcgstudio/internal/cardshader"
	"tcgstudio/internal/setfmt"
)

// Every stock border is valid, has its pictures built in and uses shaders that come with the mod.
func TestStockBorders(t *testing.T) {
	list := Stock()
	if len(list) == 0 {
		t.Fatal("no stock borders: run go run ./cmd/stockborders")
	}
	if issues := setfmt.ValidateBorders(list); len(issues) > 0 {
		t.Fatalf("stock borders invalid: %v", issues)
	}
	stock := map[string]bool{}
	for _, s := range cardshader.Stock() {
		stock[s.ID] = true
	}
	for _, b := range list {
		if !strings.HasPrefix(b.ID, setfmt.BorderStockPrefix) {
			t.Errorf("%s: id must start with %s", b.ID, setfmt.BorderStockPrefix)
		}
		files := []string{b.Icon}
		for _, l := range b.Layers {
			if l.Kind == setfmt.BorderImageLayer {
				files = append(files, l.File)
			}
		}
		for _, f := range files {
			if _, ok := StockFile(f); !ok {
				t.Errorf("%s: picture %s is not built in", b.ID, f)
			}
		}
		for _, id := range b.ShaderIDs() {
			if !stock[id] {
				t.Errorf("%s: shader %s is not a stock shader", b.ID, id)
			}
		}
	}
	if _, ok := StockFile("../cardborder.go"); ok {
		t.Error("StockFile must only serve the stock folder's own files")
	}
}

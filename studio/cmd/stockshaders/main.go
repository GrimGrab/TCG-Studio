// Command stockshaders writes the stock card shaders' files (internal/cardshader/stock): stock.json and the compiled
// bundle tcgcc_stock, both built into the mod's DLL. Run it in studio\ after changing internal/cardshader/stock.go or
// rebuilding the card shader template:  go run ./cmd/stockshaders
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"tcgstudio/internal/cardshader"
)

func main() {
	dir := filepath.Join("internal", "cardshader", "stock")
	template, err := cardshader.Template()
	if err == nil {
		var bundle []byte
		if bundle, err = cardshader.StockBundle(template); err == nil {
			if err = os.WriteFile(filepath.Join(dir, "stock.json"), cardshader.StockJSON(), 0o644); err == nil {
				err = os.WriteFile(filepath.Join(dir, "tcgcc_stock"), bundle, 0o644)
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Wrote", dir)
}

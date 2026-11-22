// Package cardborder holds the card borders that come with the mod (stock borders): their list (stock/stock.json) and
// pictures, made by `go run ./cmd/stockborders`. The mod embeds the same files (TCGCustomCards.csproj) and draws them
// like the setup's own borders; Studio lists them in its pickers and on the Borders page (read-only, copy to edit).
package cardborder

import (
	"embed"
	"encoding/json"
	"path"

	"tcgstudio/internal/setfmt"
)

//go:embed stock
var stockFS embed.FS

// URLPrefix is the URL path (after "/") the stock pictures are served under (app_files.go): "stockborder/<file>".
const URLPrefix = "stockborder/"

// Stock returns the stock borders (an empty list when none are built in). Their image layers name files of this
// package's stock folder (File without "assets/"), which StockFile reads.
func Stock() []setfmt.Border {
	b, err := stockFS.ReadFile("stock/stock.json")
	if err != nil {
		return []setfmt.Border{}
	}
	var list []setfmt.Border
	if json.Unmarshal(b, &list) != nil || list == nil {
		return []setfmt.Border{}
	}
	return list
}

// StockFile returns a stock picture's bytes (false when there is no such file).
func StockFile(name string) ([]byte, bool) {
	if name == "" || path.Base(name) != name || name == "stock.json" {
		return nil, false
	}
	b, err := stockFS.ReadFile("stock/" + name)
	return b, err == nil
}

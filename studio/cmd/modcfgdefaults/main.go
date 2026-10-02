// Command modcfgdefaults refreshes the built-in default mod config (internal/modconfig/defaults.cfg) from a config file the
// mod wrote in game: every value is reset to its default, so nothing of the local setup ends up in the studio.
//
//	go run ./cmd/modcfgdefaults "<game>\BepInEx\config\tcgcustomcards.cfg"
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"tcgstudio/internal/modconfig"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: modcfgdefaults <path to tcgcustomcards.cfg>")
		os.Exit(2)
	}
	dst := filepath.Join("internal", "modconfig", "defaults.cfg")
	if err := modconfig.MakeDefaults(os.Args[1], dst); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote", dst)
}

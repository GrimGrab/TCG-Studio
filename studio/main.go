package main

import (
	"embed"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"tcgstudio/internal/updater"
)

//go:embed all:frontend/dist
var assets embed.FS

// Set at build time by tools\package.ps1 (-ldflags "-X main.Version=… -X main.UpdateRepo=owner/name").
// A dev build ("dev", no repo) never checks for updates.
var (
	Version    = "dev"
	UpdateRepo = ""
)

func main() {
	app := NewApp()
	for _, a := range os.Args[1:] {
		if v, ok := strings.CutPrefix(a, "--updated="); ok {
			app.updatedTo = v
		}
	}
	if exe, err := os.Executable(); err == nil {
		updater.Cleanup(exe) // remove the previous version left behind by an update
	}
	err := wails.Run(&options.App{
		Title:     "TCG Studio",
		Width:     1400,
		Height:    900,
		MinWidth:  1000,
		MinHeight: 650,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app.fileHandler(), // project images + templates
		},
		BackgroundColour: &options.RGBA{R: 22, G: 24, B: 30, A: 1},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
		// Dropped files/folders reach the page as paths (Import → EPL mod); the webview never opens a dropped file itself.
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true, DisableWebViewDrop: true},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
)

// clearWebViewExperiments deletes the Edge experiment seed ("Variations*" files) from Studio's own WebView2 profile before
// the WebView starts. A fresh profile has none; the WebView downloads one during a session and applies it from the next start
// on. A seed served in October 2026 stopped <input type="color"> from opening its picker (every colour field in Studio:
// clicks arrived, nothing cancelled them, no popup), while the same build worked on a profile without a seed. Without the
// seed the WebView runs on its built-in defaults. The profile is go-webview2's default: %AppData%\<exe file name>.
func clearWebViewExperiments() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Join(os.Getenv("AppData"), filepath.Base(exe), "EBWebView")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // no profile yet
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "Variations") {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

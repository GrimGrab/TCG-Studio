package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Copied fonts and door backgrounds stay while the shop name or a door sign design uses them; unused ones go, other files stay.
func TestPruneFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("font_sign.ttf", "x")
	write("font_door.ttf", "x")
	write("font_old.ttf", "x")
	write("sign_banner.png", "x")
	write("open_text.png", "x")
	write("open_bg_cat.png", "x")
	write("closed_bg_old.png", "x")
	write("open_text.png.json", `{"text":"HI","font":{"file":"font_door.ttf","name":"Door"},"background":{"file":"open_bg_cat.png"}}`)
	pruneFiles(dir, "font_sign.ttf")
	for name, want := range map[string]bool{"font_sign.ttf": true, "font_door.ttf": true, "font_old.ttf": false, "open_bg_cat.png": true, "closed_bg_old.png": false,
		"sign_banner.png": true, "open_text.png": true} {
		if _, err := os.Stat(filepath.Join(dir, name)); (err == nil) != want {
			t.Errorf("%s: exists = %v, want %v", name, err == nil, want)
		}
	}
}

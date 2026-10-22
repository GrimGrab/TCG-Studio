package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Copied pictures, fonts and door backgrounds stay while a setting or a used door sign design names them; unused ones (e.g.
// a picture chosen but never saved) go, files we didn't copy stay.
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
	write("sign_unsaved.png", "x")
	write("by_hand.jpg", "x")
	write("loading_a.png", "x")
	write("loading_b.png", "x")
	write("card_old.png", "x")
	write("open_text_1.png", "x")
	write("open_bg_cat.png", "x")
	write("closed_bg_old.png", "x")
	write("closed_text_2.png", "x")
	write("closed_text_2.png.json", `{"text":"OLD","font":{"file":"font_old.ttf","name":"Old"},"background":{"file":"closed_bg_old.png"}}`)
	write("open_text_1.png.json", `{"text":"HI","font":{"file":"font_door.ttf","name":"Door"},"background":{"file":"open_bg_cat.png"}}`)
	write("box_restockBig_1.png", "x")
	write("box_restockBig_1.png.json", `{"layers":[{"kind":"image","src":"boximg_logo.png"}],"textureFile":"boximg_painted.png"}`)
	write("box_restockBig_0.png", "x")
	write("boximg_logo.png", "x")
	write("boximg_painted.png", "x")
	write("boximg_unused.png", "x")
	pruneFiles(dir, []string{"font_sign.ttf", "sign_banner.png", "open_text_1.png", "loading_a.png", "box_restockBig_1.png"})
	for name, want := range map[string]bool{"font_sign.ttf": true, "font_door.ttf": true, "font_old.ttf": false, "open_bg_cat.png": true,
		"closed_bg_old.png": false, "closed_text_2.png": false, "closed_text_2.png.json": false, "sign_banner.png": true,
		"sign_unsaved.png": false, "by_hand.jpg": true, "loading_a.png": true, "loading_b.png": false, "card_old.png": false, "open_text_1.png": true, "open_text_1.png.json": true,
		"box_restockBig_1.png": true, "box_restockBig_1.png.json": true, "box_restockBig_0.png": false, "boximg_logo.png": true,
		"boximg_painted.png": true, "boximg_unused.png": false} {
		if _, err := os.Stat(filepath.Join(dir, name)); (err == nil) != want {
			t.Errorf("%s: exists = %v, want %v", name, err == nil, want)
		}
	}
}

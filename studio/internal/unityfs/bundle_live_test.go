package unityfs

import (
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// BUNDLE_LIVE=<bundle file> [BUNDLE_OUT=<dir>] go test ./internal/unityfs -run BundleLive -v
// Opens a real asset bundle (e.g. an Enhanced Prefab Loader mod's), lists its contents and decodes a few sprites.
func TestBundleLive(t *testing.T) {
	path := os.Getenv("BUNDLE_LIVE")
	if path == "" {
		t.Skip("BUNDLE_LIVE not set")
	}
	start := time.Now()
	b, err := OpenBundle(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	t.Logf("Unity %s, %d blocks, nodes:", b.Unity, len(b.blocks))
	for _, n := range b.Nodes {
		t.Logf("  %s  %d bytes serialized=%v", n.Name, n.Size, n.Serialized)
	}
	env := NewBundleEnv(b)
	files, err := env.BundleFiles()
	if err != nil {
		t.Fatal(err)
	}
	classes := map[int32]int{}
	formats := map[int]int{}
	var sprites []*Object
	for _, f := range files {
		for _, o := range f.Objects {
			classes[o.ClassID]++
			switch o.ClassID {
			case ClassSprite:
				sprites = append(sprites, o)
			case ClassTexture2D:
				if tex, err := ReadTexture2D(o); err == nil {
					formats[tex.Format]++
				} else {
					t.Errorf("texture %d: %v", o.PathID, err)
				}
			}
		}
	}
	t.Logf("classes %v, texture formats %v, opened in %v", classes, formats, time.Since(start))
	sort.Slice(sprites, func(i, j int) bool { return sprites[i].PathID < sprites[j].PathID })
	out := os.Getenv("BUNDLE_OUT")
	for i, o := range sprites {
		if i >= 3 {
			break
		}
		s, err := ReadSprite(o)
		if err != nil {
			t.Fatal(err)
		}
		img, err := env.SpriteImage(s)
		if err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
		t.Logf("sprite %s: %v", s.Name, img.Bounds())
		if out != "" {
			f, _ := os.Create(filepath.Join(out, s.Name+".png"))
			_ = png.Encode(f, img)
			f.Close()
		}
	}
	t.Logf("done in %v", time.Since(start))
}

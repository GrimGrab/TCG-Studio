package art

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A synthetic head-on display: lid art on top, a row of "packs", the front panel at the bottom, on white.
func TestDetectDisplaySynthetic(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1000, 900))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	fill := func(r image.Rectangle, c color.NRGBA) {
		draw.Draw(img, r, image.NewUniform(c), image.Point{}, draw.Src)
	}
	fill(image.Rect(40, 60, 960, 470), color.NRGBA{60, 90, 160, 255})    // lid
	fill(image.Rect(40, 470, 960, 600), color.NRGBA{200, 190, 120, 255}) // packs
	fill(image.Rect(40, 600, 960, 880), color.NRGBA{30, 110, 60, 255})   // front panel
	f := DetectDisplay(img)
	want := DisplayFaces{Front: Quad{40, 600, 960, 600, 960, 880, 40, 880}, Lid: Quad{40, 60, 960, 60, 960, 449, 40, 449}} // lid bottom = 72 % of the way to the panel
	for i := range 8 {
		if d := f.Front[i] - want.Front[i]; d > 15 || d < -15 {
			t.Errorf("front %v, want %v", f.Front, want.Front)
			break
		}
	}
	for i := range 8 {
		if d := f.Lid[i] - want.Lid[i]; d > 15 || d < -15 {
			t.Errorf("lid %v, want %v", f.Lid, want.Lid)
			break
		}
	}
	if !f.Confident {
		t.Error("not confident on a clean synthetic display")
	}
}

// TestDetectDisplayLive runs on real TCGplayer photos and draws the found quads (front green, lid blue) into BOX_OUT:
// BOX_LIVE="541235,485256,27308" BOX_OUT=<dir> go test ./internal/art -run DetectDisplayLive -v
// (photos are downloaded, never committed).
func TestDetectDisplayLive(t *testing.T) {
	spec := os.Getenv("BOX_LIVE")
	if spec == "" {
		t.Skip("set BOX_LIVE=<TCGplayer product ids> to run")
	}
	for _, id := range strings.Split(spec, ",") {
		t.Run(id, func(t *testing.T) {
			resp, err := http.Get("https://tcgplayer-cdn.tcgplayer.com/product/" + id + "_in_1000x1000.jpg")
			if err != nil {
				t.Fatal(err)
			}
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			src, _, err := image.Decode(bytes.NewReader(b))
			if err != nil {
				t.Fatal(err)
			}
			f := DetectDisplay(src)
			t.Logf("front %v\nlid   %v\nconfident %v", f.Front, f.Lid, f.Confident)
			if out := os.Getenv("BOX_OUT"); out != "" {
				img := image.NewNRGBA(src.Bounds())
				draw.Draw(img, img.Bounds(), src, src.Bounds().Min, draw.Src)
				drawQuad(img, f.Front, color.NRGBA{0, 220, 0, 255})
				drawQuad(img, f.Lid, color.NRGBA{0, 80, 255, 255})
				if err := savePNG(filepath.Join(out, fmt.Sprintf("box_%s.png", id)), img); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func drawQuad(img *image.NRGBA, q Quad, c color.NRGBA) {
	for i := 0; i < 4; i++ {
		x0, y0, x1, y1 := q[i*2], q[i*2+1], q[(i*2+2)%8], q[(i*2+3)%8]
		n := int(max(abs(x1-x0), abs(y1-y0))) + 1
		for s := 0; s <= n; s++ {
			x, y := int(x0+(x1-x0)*float64(s)/float64(n)), int(y0+(y1-y0)*float64(s)/float64(n))
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					img.Set(x+dx, y+dy, c)
				}
			}
		}
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// TestEdgeProfileLive prints the panel-top edge profile (BOX_LIVE ids) to tune the search.
func TestEdgeProfileLive(t *testing.T) {
	spec := os.Getenv("BOX_PROFILE")
	if spec == "" {
		t.Skip("set BOX_PROFILE=<TCGplayer product ids> to run")
	}
	for _, id := range strings.Split(spec, ",") {
		resp, err := http.Get("https://tcgplayer-cdn.tcgplayer.com/product/" + id + "_in_1000x1000.jpg")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		src, _, _ := image.Decode(bytes.NewReader(b))
		g := newGray(src, 400)
		bottom, _ := g.contours()
		base, _ := longestLine(bottom, float64(g.h)*0.015, 0.6)
		var sb strings.Builder
		for _, e := range g.edgeProfile(base, float64(g.h)*0.1, float64(g.h)*0.75) {
			if int(e.d)%3 == 0 {
				fmt.Fprintf(&sb, "d=%3.0f y@mid=%5.0f %s\n", e.d, e.l.y((base.x0+base.x1)/2)*g.scale, strings.Repeat("#", int(e.score*40)))
			}
		}
		t.Logf("%s (h=%d)\n%s", id, g.h, sb.String())
	}
}

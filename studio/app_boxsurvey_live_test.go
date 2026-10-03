package main

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"tcgstudio/internal/art"
	"tcgstudio/internal/importer"
)

// TestBoxSurveyLive runs box-photo detection over a spread of sets per game and writes, into BOX_SURVEY:
// sheet_<game>.png (photos with the found front panel green and lid blue) and ws\projects\<cat>-<group>\ (photos +
// sources_booster.json) for rendering the finished art. BOX_SURVEY_N = sets per game (default 6).
// BOX_SURVEY=<dir> go test . -run BoxSurveyLive -v -timeout 30m
func TestBoxSurveyLive(t *testing.T) {
	out := os.Getenv("BOX_SURVEY")
	if out == "" {
		t.Skip("set BOX_SURVEY=<dir> to run")
	}
	per := 6
	if n, err := strconv.Atoi(os.Getenv("BOX_SURVEY_N")); err == nil && n > 0 {
		per = n
	}
	s := importer.NewSealed()
	ctx := context.Background()
	var list []map[string]string
	// BOX_SURVEY_EXTRA="<category>:<group id>,…" adds named sets (only those, per game, when given).
	extra := map[int][]int{}
	for _, e := range strings.Split(os.Getenv("BOX_SURVEY_EXTRA"), ",") {
		var c, g int
		if _, err := fmt.Sscanf(e, "%d:%d", &c, &g); err == nil {
			extra[c] = append(extra[c], g)
		}
	}
	for _, game := range importer.SealedGames {
		gs, _, err := s.Groups(ctx, game.Category, "", "")
		if err != nil {
			t.Errorf("%s: %v", game.Name, err)
			continue
		}
		if len(extra) > 0 {
			var keep []importer.SealedGroup
			for _, g := range gs {
				if slices.Contains(extra[game.Category], g.ID) {
					keep = append(keep, g)
				}
			}
			gs = keep
		}
		var tiles []*image.NRGBA
		// A spread from newest to old: every k-th group, skipping ones without a box photo.
		step := max(1, len(gs)/(per*3))
		if len(extra) > 0 {
			step = 1
		}
		for i := 0; i < len(gs) && len(tiles) < per; i += step {
			g := gs[i]
			ps, err := s.Products(ctx, game.Category, g.ID)
			if err != nil {
				continue
			}
			pack, box := importer.PickPackAndBox(ps, 0)
			if box == nil {
				continue
			}
			img, err := s.Photo(ctx, *box)
			if err != nil {
				continue
			}
			f := art.DetectDisplay(img)
			label := fmt.Sprintf("%s (%d)%s", g.Name, box.ID, map[bool]string{true: "", false: " ?"}[f.Confident])
			tiles = append(tiles, overlayTile(img, f, label))
			t.Logf("%-22s %-45s box %d confident %v", game.Name, g.Name, box.ID, f.Confident)

			// For the harness: photos + sources (no cards: the box from the photo is what's checked).
			id := fmt.Sprintf("%d-%d", game.Category, g.ID)
			dir := filepath.Join(out, "ws", "projects", id, "images")
			_ = os.MkdirAll(dir, 0o755)
			src := map[string]any{"packPhoto": "", "packName": "", "boxPhoto": fmt.Sprintf("images/photo_%d.png", box.ID), "boxName": box.Name,
				"box": f, "cards": []any{}, "icon": "", "notes": []string{}}
			_ = art.SavePNG(filepath.Join(dir, fmt.Sprintf("photo_%d.png", box.ID)), img)
			if pack != nil {
				if pi, err := s.Photo(ctx, *pack); err == nil {
					_ = art.SavePNG(filepath.Join(dir, fmt.Sprintf("photo_%d.png", pack.ID)), pi)
					src["packPhoto"], src["packName"] = fmt.Sprintf("images/photo_%d.png", pack.ID), pack.Name
				}
			}
			b, _ := json.Marshal(src)
			_ = os.WriteFile(filepath.Join(out, "ws", "projects", id, "sources_booster.json"), b, 0o644)
			list = append(list, map[string]string{"id": id, "title": g.Name, "game": game.Name})
		}
		if len(tiles) > 0 {
			if err := art.SavePNG(filepath.Join(out, fmt.Sprintf("sheet_%d.png", game.Category)), sheet(tiles)); err != nil {
				t.Error(err)
			}
		}
	}
	b, _ := json.MarshalIndent(list, "", " ")
	name := "list.json"
	if len(extra) > 0 {
		name = "list_extra.json"
	}
	_ = os.WriteFile(filepath.Join(out, name), b, 0o644)
}

// overlayTile draws the found quads on the photo, scaled to 360 px high, with a label strip.
func overlayTile(src image.Image, f art.DisplayFaces, label string) *image.NRGBA {
	b := src.Bounds()
	h := 360
	k := float64(h) / float64(b.Dy())
	w := int(float64(b.Dx()) * k)
	t := image.NewNRGBA(image.Rect(0, 0, w, h+18))
	draw.Draw(t, t.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			t.Set(x, y, src.At(b.Min.X+int(float64(x)/k), b.Min.Y+int(float64(y)/k)))
		}
	}
	quad := func(q art.Quad, c color.NRGBA) {
		for i := 0; i < 4; i++ {
			x0, y0 := (q[i*2]-float64(b.Min.X))*k, (q[i*2+1]-float64(b.Min.Y))*k
			x1, y1 := (q[(i*2+2)%8]-float64(b.Min.X))*k, (q[(i*2+3)%8]-float64(b.Min.Y))*k
			n := int(max(abs(x1-x0), abs(y1-y0))) + 1
			for s := 0; s <= n; s++ {
				x, y := int(x0+(x1-x0)*float64(s)/float64(n)), int(y0+(y1-y0)*float64(s)/float64(n))
				for d := -1; d <= 1; d++ {
					t.Set(x+d, y, c)
					t.Set(x, y+d, c)
				}
			}
		}
	}
	quad(f.Front, color.NRGBA{0, 210, 0, 255})
	quad(f.Lid, color.NRGBA{0, 80, 255, 255})
	drawLabel(t, label, h+3)
	return t
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// sheet lays tiles out in rows of three.
func sheet(tiles []*image.NRGBA) *image.NRGBA {
	cols := 3
	cw, rh := 0, 0
	for _, t := range tiles {
		cw, rh = max(cw, t.Bounds().Dx()), max(rh, t.Bounds().Dy())
	}
	rows := (len(tiles) + cols - 1) / cols
	s := image.NewNRGBA(image.Rect(0, 0, cols*(cw+8), rows*(rh+8)))
	draw.Draw(s, s.Bounds(), image.NewUniform(color.NRGBA{40, 40, 40, 255}), image.Point{}, draw.Src)
	for i, t := range tiles {
		p := image.Pt((i%cols)*(cw+8), (i/cols)*(rh+8))
		draw.Draw(s, t.Bounds().Add(p), t, image.Point{}, draw.Src)
	}
	return s
}

func drawLabel(img *image.NRGBA, s string, y int) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(color.Black), Face: basicfont.Face7x13, Dot: fixed.P(4, y+11)}
	d.DrawString(s)
}

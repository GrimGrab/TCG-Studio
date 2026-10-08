package gameextract

import (
	"encoding/binary"
	"encoding/json"
	"image"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tcgstudio/internal/unityfs"
	"tcgstudio/internal/uvmap"
)

// Extracts from the installed game (TCG_GAME, default Steam path) into a temp folder; EXTRACT_OUT keeps it somewhere.
func TestExtract(t *testing.T) {
	game := os.Getenv("TCG_GAME")
	if game == "" {
		game = `D:\SteamLibrary\steamapps\common\TCG Card Shop Simulator`
	}
	if _, err := DataDir(game); err != nil {
		t.Skip(err)
	}
	out := os.Getenv("EXTRACT_OUT")
	if out == "" {
		out = filepath.Join(t.TempDir(), "templates")
	}
	start := time.Now()
	info, err := Extract(game, out, func(s string) { t.Log(s) })
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("done in %v, %d warnings", time.Since(start).Round(time.Millisecond), len(info.Warnings))
	for _, w := range info.Warnings {
		t.Log("warning:", w)
	}
}

// Paint templates of a few kinds of pieces (real unwrap, two-colour strip, plain colours, many parts) from the installed game;
// PAINT_OUT keeps them, PAINT_BASE picks one piece.
func TestPaintTemplate(t *testing.T) {
	game := os.Getenv("TCG_GAME")
	if game == "" {
		game = `D:\SteamLibrary\steamapps\common\TCG Card Shop Simulator`
	}
	if _, err := DataDir(game); err != nil {
		t.Skip(err)
	}
	out := os.Getenv("PAINT_OUT")
	if out == "" {
		out = t.TempDir()
	}
	bases := []string{"Shelf", "WarehouseShelf", "TournamentPrizeShelf", "CardShelf", "PlayTable", "CashCounter"}
	if b := os.Getenv("PAINT_BASE"); b != "" {
		bases = []string{b}
	}
	for _, base := range bases {
		start := time.Now()
		file, err := PaintTemplate(game, out, base)
		if err != nil {
			t.Fatalf("%s: %v", base, err)
		}
		var tpl paintTemplate
		b, _ := os.ReadFile(filepath.Join(out, FurnitureDir, file))
		if err := json.Unmarshal(b, &tpl); err != nil {
			t.Fatal(err)
		}
		if len(tpl.Parts) == 0 || len(tpl.Model.Views) == 0 || !tpl.Model.Projected {
			t.Fatalf("%s: empty template", base)
		}
		for _, q := range tpl.Parts {
			m, err := uvmap.LoadOBJ(filepath.Join(out, FurnitureDir, q.Mesh))
			if err != nil {
				t.Fatalf("%s %s: %v", base, q.Mesh, err)
			}
			for _, uv := range m.UV {
				if uv[0] < 0 || uv[0] > 1 || uv[1] < 0 || uv[1] > 1 {
					t.Fatalf("%s %s: UV %v outside the atlas", base, q.Mesh, uv)
				}
			}
		}
		// Preview: header, sizes, every vertex inside the atlas and inside a view of the net, depth 0..1.
		bin, err := os.ReadFile(filepath.Join(out, FurnitureDir, tpl.Preview))
		if err != nil {
			t.Fatal(err)
		}
		if string(bin[:4]) != "TCGP" {
			t.Fatalf("%s: bad preview header", base)
		}
		nv, ni := int(binary.LittleEndian.Uint32(bin[8:])), int(binary.LittleEndian.Uint32(bin[12:]))
		if len(bin) != 16+44*nv+4*ni || ni%3 != 0 {
			t.Fatalf("%s: preview size %d for %d vertices / %d indices", base, len(bin), nv, ni)
		}
		fl := func(i int) float64 { return float64(math.Float32frombits(binary.LittleEndian.Uint32(bin[16+4*i:]))) }
		for v := 0; v < nv; v++ {
			au, av, nx, ny, d := fl(v*11+6), fl(v*11+7), fl(v*11+8), fl(v*11+9), fl(v*11+10)
			in := false
			for _, w := range tpl.Model.Views {
				in = in || nx >= w.Net[0]-1e-4 && nx <= w.Net[0]+w.Net[2]+1e-4 && ny >= w.Net[1]-1e-4 && ny <= w.Net[1]+w.Net[3]+1e-4
			}
			if au < 0 || au > 1 || av < 0 || av > 1 || d < -1e-4 || d > 1+1e-4 || !in {
				t.Fatalf("%s: preview vertex %d out of range (uv %v,%v net %v,%v depth %v)", base, v, au, av, nx, ny, d)
			}
		}
		t.Logf("%s: %d parts, %d preview vertices, %d triangles, %d views, preview %d KB in %v", base, len(tpl.Parts), nv, ni/3,
			len(tpl.Model.Views), len(bin)/1024, time.Since(start).Round(time.Millisecond))
	}
}

// An own model (here a textured cube) gets a paint template too: one part (renderer "") = the model with the painting UVs.
func TestPaintTemplateFromMesh(t *testing.T) {
	m := &unityfs.Mesh{Name: "cube"}
	for _, f := range [][4][3]float32{
		{{0, 0, 0}, {1, 0, 0}, {1, 1, 0}, {0, 1, 0}}, {{1, 0, 1}, {0, 0, 1}, {0, 1, 1}, {1, 1, 1}},
		{{0, 0, 1}, {0, 0, 0}, {0, 1, 0}, {0, 1, 1}}, {{1, 0, 0}, {1, 0, 1}, {1, 1, 1}, {1, 1, 0}},
		{{0, 1, 0}, {1, 1, 0}, {1, 1, 1}, {0, 1, 1}}, {{0, 0, 1}, {1, 0, 1}, {1, 0, 0}, {0, 0, 0}},
	} {
		b := uint32(len(m.Pos))
		e1 := [3]float32{f[1][0] - f[0][0], f[1][1] - f[0][1], f[1][2] - f[0][2]}
		e2 := [3]float32{f[3][0] - f[0][0], f[3][1] - f[0][1], f[3][2] - f[0][2]}
		n := [3]float32{e1[1]*e2[2] - e1[2]*e2[1], e1[2]*e2[0] - e1[0]*e2[2], e1[0]*e2[1] - e1[1]*e2[0]}
		for k, p := range f {
			m.Pos = append(m.Pos, p)
			m.Normal = append(m.Normal, n)
			m.UV = append(m.UV, [2]float32{float32(k & 1), float32(k >> 1)})
		}
		if len(m.Subs) == 0 {
			m.Subs = [][]uint32{nil}
		}
		m.Subs[0] = append(m.Subs[0], b, b+1, b+2, b, b+2, b+3)
	}
	tex := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for i := range tex.Pix {
		tex.Pix[i] = 200
	}
	dir := t.TempDir()
	file, err := PaintTemplateFromMesh(dir, "own_test", m, tex)
	if err != nil {
		t.Fatal(err)
	}
	var tpl paintTemplate
	b, _ := os.ReadFile(filepath.Join(dir, FurnitureDir, file))
	if err := json.Unmarshal(b, &tpl); err != nil {
		t.Fatal(err)
	}
	if len(tpl.Parts) != 1 || tpl.Parts[0].Renderer != "" || len(tpl.Model.Views) != 6 {
		t.Fatalf("own template: %d parts (renderer %q), %d views", len(tpl.Parts), tpl.Parts[0].Renderer, len(tpl.Model.Views))
	}
	om, err := uvmap.LoadOBJ(filepath.Join(dir, FurnitureDir, tpl.Parts[0].Mesh))
	if err != nil || len(om.Tris) != 12 {
		t.Fatalf("own model mesh: %v, %d triangles", err, len(om.Tris))
	}
	for _, uv := range om.UV {
		if uv[0] < 0 || uv[0] > 1 || uv[1] < 0 || uv[1] > 1 {
			t.Fatalf("UV %v outside the atlas", uv)
		}
	}
}

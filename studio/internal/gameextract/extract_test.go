package gameextract

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"strconv"
	"strings"
	"tcgstudio/internal/packmesh"
	"tcgstudio/internal/unityfs"
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
	checkClothes(t, out)
	checkCharacters(t, out)
	checkPackOpen(t, out)
}

// The pack models: the fixed shelf pack, and both pack-opening packs with every list as long as its vertex count says, bone
// weights summing to 1, the 7 bones of the game's opening model, more triangles than the model had (the outside is cut along
// the shelf pack's).
func checkPackOpen(t *testing.T, out string) {
	if _, err := os.Stat(filepath.Join(out, "accessories", "CardPackStaticMesh_low"+FixedShelfSuffix+".obj")); err != nil {
		t.Fatalf("fixed shelf pack: %v", err)
	}
	for _, file := range []string{PackOpenFile, PackOpenFixedFile} {
		checkPackOpenFile(t, out, file)
	}
}

func checkPackOpenFile(t *testing.T, out, file string) {
	var f PackOpen
	b, err := os.ReadFile(filepath.Join(out, "accessories", file))
	if err != nil || json.Unmarshal(b, &f) != nil {
		t.Fatalf("%s: %v", file, err)
	}
	n := len(f.Positions) / 3
	if f.Version != PackOpenVersion || len(f.Bones) != 7 || len(f.BoneParents) != 7 || len(f.BindPoses) != 16*7 ||
		len(f.Normals) != 3*n || len(f.UVs) != 2*n || len(f.BoneIndices) != 4*n || len(f.BoneWeights) != 4*n || len(f.Triangles) < 3*1950 {
		t.Fatalf("%s: version %d, %d bones, %d vertices, %d triangles", file, f.Version, len(f.Bones), n, len(f.Triangles)/3)
	}
	for i := 0; i < n; i++ {
		sum := float32(0)
		for k := 0; k < 4; k++ {
			if j := f.BoneIndices[4*i+k]; j < 0 || j >= len(f.Bones) {
				t.Fatalf("vertex %d: bone %d", i, j)
			}
			sum += f.BoneWeights[4*i+k]
		}
		if sum < 0.999 || sum > 1.001 {
			t.Fatalf("vertex %d: weights sum to %v", i, sum)
		}
	}
	for _, x := range f.Triangles {
		if x < 0 || x >= n {
			t.Fatalf("triangle index %d of %d vertices", x, n)
		}
	}
	checkCrimpsDontCross(t, file, f)
	t.Logf("%s (from %s): %d vertices, %d triangles, bones %v", file, f.Shelf, n, len(f.Triangles)/3, f.Bones)
}

// checkCrimpsDontCross: folded shut, the closed pack's crimps show their printed layer from the front and the back, never the
// inside layer (packmesh.Unpinch: in the open sheet the crimps step back further than the closed pack is thick, so they crossed
// and the crimp tips showed the other half's grey inside).
func checkCrimpsDontCross(t *testing.T, file string, f PackOpen) {
	var m packmesh.Mesh
	for i := 0; i < len(f.Positions)/3; i++ {
		m.Pos = append(m.Pos, packmesh.Vec3{float64(f.Positions[3*i]), float64(f.Positions[3*i+1]), float64(f.Positions[3*i+2])})
		m.Norm = append(m.Norm, packmesh.Vec3{float64(f.Normals[3*i]), float64(f.Normals[3*i+1]), float64(f.Normals[3*i+2])})
		m.Joints = append(m.Joints, [4]int{f.BoneIndices[4*i], f.BoneIndices[4*i+1], f.BoneIndices[4*i+2], f.BoneIndices[4*i+3]})
		var w [4]float64
		for k := range 4 {
			w[k] = float64(f.BoneWeights[4*i+k])
		}
		m.Weights = append(m.Weights, w)
	}
	for i := 0; i+2 < len(f.Triangles); i += 3 {
		m.Tris = append(m.Tris, [3]int{f.Triangles[i], f.Triangles[i+1], f.Triangles[i+2]})
	}
	bones := make([]packmesh.Bone, len(f.Bones))
	for i, name := range f.Bones {
		bones[i] = packmesh.Bone{Name: name, Parent: f.BoneParents[i]}
		for k := range 16 {
			bones[i].Bind[k] = float64(f.BindPoses[16*i+k])
		}
	}
	pos, nrm, err := packmesh.Fold(m, bones)
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	// The print faces +z at rest: the closed pack's front is seen from +z, its back from -z.
	for _, side := range []float64{1, -1} {
		for _, y := range []float64{0.55, 0.6, 0.64, 0.66, -0.55, -0.6, -0.64, -0.66} {
			for _, x := range []float64{-0.25, -0.1, 0.1, 0.25} {
				best, bz := -1, math.Inf(-1)
				for ti, tr := range m.Tris {
					a, b, c := pos[tr[0]], pos[tr[1]], pos[tr[2]]
					den := (b[1]-c[1])*(a[0]-c[0]) + (c[0]-b[0])*(a[1]-c[1])
					if math.Abs(den) < 1e-12 || side*(nrm[tr[0]][2]+nrm[tr[1]][2]+nrm[tr[2]][2]) <= 0 {
						continue // edge-on, or facing away (culled)
					}
					w0 := ((b[1]-c[1])*(x-c[0]) + (c[0]-b[0])*(y-c[1])) / den
					w1 := ((c[1]-a[1])*(x-c[0]) + (a[0]-c[0])*(y-c[1])) / den
					w2 := 1 - w0 - w1
					if w0 < 0 || w1 < 0 || w2 < 0 {
						continue
					}
					if z := side * (w0*a[2] + w1*b[2] + w2*c[2]); z > bz {
						bz, best = z, ti
					}
				}
				if best < 0 {
					continue
				}
				tr := m.Tris[best]
				if m.Norm[tr[0]][2]+m.Norm[tr[1]][2]+m.Norm[tr[2]][2] < 0 {
					t.Errorf("%s: the closed pack shows its inside layer at x %+.2f y %+.2f from the %s (the crimps cross)",
						file, x, y, map[float64]string{1: "front", -1: "back"}[side])
				}
			}
		}
	}
}

// The face editor's characters: both bodies with their parts, hair slots and presets, and the face sliders in the body mesh.
func checkCharacters(t *testing.T, out string) {
	var f CharactersFile
	b, err := os.ReadFile(filepath.Join(out, CharactersDir, "characters.json"))
	if err != nil || json.Unmarshal(b, &f) != nil {
		t.Fatalf("characters.json: %v", err)
	}
	for body, slots := range map[string]int{"male": 3, "female": 2} {
		c := f.Bodies[body]
		if len(c.Parts) != 6 || len(c.Hair) != slots || len(c.Presets) == 0 || len(c.Presets[0].Shapes) < 60 {
			t.Fatalf("%s: %d parts, %d hair slots, %d presets", body, len(c.Parts), len(c.Hair), len(c.Presets))
		}
		mesh, err := os.ReadFile(filepath.Join(out, CharactersDir, c.Mesh))
		if err != nil || string(mesh[:4]) != "TCGM" || !bytes.Contains(mesh, []byte("mod_jaw_width")) {
			t.Fatalf("%s mesh %s: %v", body, c.Mesh, err)
		}
	}
	for _, name := range []string{"T_Skin_01", "T_Sclera", "T_Iris_Mask", "T_Lips", "T_Eyebrows_01", "T_Hair_Bob_01_AO"} {
		if f.Textures[name] == "" {
			t.Errorf("texture %s missing", name)
		}
	}
}

// The customer clothes: both bodies, the presets, and the male T-shirt's four panels named by where they sit.
func checkClothes(t *testing.T, out string) {
	var f ClothesFile
	b, err := os.ReadFile(filepath.Join(out, ClothesDir, "clothes.json"))
	if err != nil || json.Unmarshal(b, &f) != nil {
		t.Fatalf("clothes.json: %v", err)
	}
	for _, body := range []string{"male", "female"} {
		if len(f.Bodies[body].Slots) != 4 || len(f.Bodies[body].Presets) == 0 {
			t.Fatalf("%s: %d slots, %d presets", body, len(f.Bodies[body].Slots), len(f.Bodies[body].Presets))
		}
	}
	for _, it := range f.Bodies["male"].Slots[0].Items {
		if it.Name != "TShirt_01" {
			continue
		}
		got := map[string]bool{}
		for _, is := range it.Islands[0] {
			got[is.Label] = true
		}
		for _, want := range []string{"Front", "Back", "Left side", "Right side"} {
			if !got[want] {
				t.Errorf("T-shirt: no %q island (got %v)", want, got)
			}
		}
		if _, err := os.Stat(filepath.Join(out, ClothesDir, it.Mesh)); err != nil || len(it.Variants) != 7 {
			t.Errorf("T-shirt: mesh %q (%v), %d variants", it.Mesh, err, len(it.Variants))
		}
		return
	}
	t.Error("no male T-shirt")
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
			m, err := readOBJ(filepath.Join(out, FurnitureDir, q.Mesh))
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
	om, err := readOBJ(filepath.Join(dir, FurnitureDir, tpl.Parts[0].Mesh))
	if err != nil || om.Tris != 12 {
		t.Fatalf("own model mesh: %v, %d triangles", err, om.Tris)
	}
	for _, uv := range om.UV {
		if uv[0] < 0 || uv[0] > 1 || uv[1] < 0 || uv[1] > 1 {
			t.Fatalf("UV %v outside the atlas", uv)
		}
	}
}

// readOBJ reads what these tests check of an exported mesh: its UVs and how many triangles it has.
func readOBJ(path string) (m struct {
	UV   [][2]float64
	Tris int
}, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) >= 3 && f[0] == "vt" {
			u, _ := strconv.ParseFloat(f[1], 64)
			v, _ := strconv.ParseFloat(f[2], 64)
			m.UV = append(m.UV, [2]float64{u, v})
		} else if len(f) >= 4 && f[0] == "f" {
			m.Tris++
		}
	}
	return m, nil
}

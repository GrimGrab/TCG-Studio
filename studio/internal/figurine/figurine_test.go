package figurine

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// cube returns a unit cube (−0.5..0.5) as 24 vertices / 12 triangles, counter-clockwise from outside (glTF convention).
func cube() (pos [][3]float32, nrm [][3]float32, uv [][2]float32, idx []uint16) {
	faces := []struct{ n, u, v [3]float32 }{
		{[3]float32{1, 0, 0}, [3]float32{0, 0, -1}, [3]float32{0, 1, 0}},
		{[3]float32{-1, 0, 0}, [3]float32{0, 0, 1}, [3]float32{0, 1, 0}},
		{[3]float32{0, 1, 0}, [3]float32{1, 0, 0}, [3]float32{0, 0, -1}},
		{[3]float32{0, -1, 0}, [3]float32{1, 0, 0}, [3]float32{0, 0, 1}},
		{[3]float32{0, 0, 1}, [3]float32{1, 0, 0}, [3]float32{0, 1, 0}},
		{[3]float32{0, 0, -1}, [3]float32{-1, 0, 0}, [3]float32{0, 1, 0}},
	}
	for _, f := range faces {
		base := uint16(len(pos))
		for _, c := range [][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
			var p [3]float32
			for q := 0; q < 3; q++ {
				p[q] = 0.5*f.n[q] + 0.5*c[0]*f.u[q] + 0.5*c[1]*f.v[q]
			}
			pos = append(pos, p)
			nrm = append(nrm, f.n)
			uv = append(uv, [2]float32{(c[0] + 1) / 2, (1 - c[1]) / 2})
		}
		idx = append(idx, base, base+1, base+2, base, base+2, base+3)
	}
	return
}

func pngBytes(t *testing.T, c color.NRGBA) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for i := 0; i < 16*16; i++ {
		img.SetNRGBA(i%16, i/16, c)
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// writeGLB builds a GLB with two cube primitives: one textured (red image), one untextured (blue factor), under a node
// tree with translation + scale, and a second, mirrored node (negative scale) reusing the mesh.
func writeGLB(t *testing.T, path string) {
	pos, nrm, uv, idx := cube()
	var bin bytes.Buffer
	view := func(data any) (int, int) {
		for bin.Len()%4 != 0 {
			bin.WriteByte(0)
		}
		off := bin.Len()
		_ = binary.Write(&bin, binary.LittleEndian, data)
		return off, bin.Len() - off
	}
	pOff, pLen := view(pos)
	nOff, nLen := view(nrm)
	uOff, uLen := view(uv)
	iOff, iLen := view(idx)
	img := pngBytes(t, color.NRGBA{255, 0, 0, 255})
	imOff, imLen := view(img)
	for bin.Len()%4 != 0 {
		bin.WriteByte(0)
	}
	doc := map[string]any{
		"asset":  map[string]any{"version": "2.0"},
		"scene":  0,
		"scenes": []any{map[string]any{"nodes": []int{0, 2}}},
		"nodes": []any{
			map[string]any{"translation": []float64{0, 1, 0}, "children": []int{1}},
			map[string]any{"mesh": 0, "scale": []float64{2, 2, 2}},
			map[string]any{"mesh": 0, "translation": []float64{5, 0, 0}, "scale": []float64{-1, 1, 1}},
		},
		"meshes": []any{map[string]any{"primitives": []any{
			map[string]any{"attributes": map[string]int{"POSITION": 0, "NORMAL": 1, "TEXCOORD_0": 2}, "indices": 3, "material": 0},
			map[string]any{"attributes": map[string]int{"POSITION": 0, "NORMAL": 1}, "indices": 3, "material": 1},
		}}},
		"materials": []any{
			map[string]any{"name": "red", "pbrMetallicRoughness": map[string]any{"baseColorTexture": map[string]any{"index": 0}}},
			map[string]any{"name": "blue", "pbrMetallicRoughness": map[string]any{"baseColorFactor": []float64{0, 0, 1, 1}}},
		},
		"textures": []any{map[string]any{"source": 0}},
		"images":   []any{map[string]any{"bufferView": 4, "mimeType": "image/png"}},
		"accessors": []any{
			map[string]any{"bufferView": 0, "componentType": 5126, "count": len(pos), "type": "VEC3"},
			map[string]any{"bufferView": 1, "componentType": 5126, "count": len(nrm), "type": "VEC3"},
			map[string]any{"bufferView": 2, "componentType": 5126, "count": len(uv), "type": "VEC2"},
			map[string]any{"bufferView": 3, "componentType": 5123, "count": len(idx), "type": "SCALAR"},
		},
		"bufferViews": []any{
			map[string]any{"buffer": 0, "byteOffset": pOff, "byteLength": pLen},
			map[string]any{"buffer": 0, "byteOffset": nOff, "byteLength": nLen},
			map[string]any{"buffer": 0, "byteOffset": uOff, "byteLength": uLen},
			map[string]any{"buffer": 0, "byteOffset": iOff, "byteLength": iLen},
			map[string]any{"buffer": 0, "byteOffset": imOff, "byteLength": imLen},
		},
		"buffers": []any{map[string]any{"byteLength": bin.Len()}},
	}
	js, _ := json.Marshal(doc)
	for len(js)%4 != 0 {
		js = append(js, ' ')
	}
	var out bytes.Buffer
	out.WriteString("glTF")
	_ = binary.Write(&out, binary.LittleEndian, []uint32{2, uint32(12 + 8 + len(js) + 8 + bin.Len())})
	_ = binary.Write(&out, binary.LittleEndian, []uint32{uint32(len(js)), 0x4E4F534A})
	out.Write(js)
	_ = binary.Write(&out, binary.LittleEndian, []uint32{uint32(bin.Len()), 0x004E4942})
	out.Write(bin.Bytes())
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// outwardShare is the fraction of triangles whose (b−a)×(c−a) points along the stored normal and away from the centre of
// the triangle's own cube.
func outward(m *Mesh, center func([3]float64) [3]float64) (alongNormal, outwards float64) {
	n := m.Triangles()
	var a1, a2 int
	for t := 0; t < n; t++ {
		a, b, c := m.Pos[m.Idx[t*3]], m.Pos[m.Idx[t*3+1]], m.Pos[m.Idx[t*3+2]]
		cr := cross(sub(b, a), sub(c, a))
		if dot(cr, m.Nrm[m.Idx[t*3]]) > 0 {
			a1++
		}
		mid := [3]float64{(a[0] + b[0] + c[0]) / 3, (a[1] + b[1] + c[1]) / 3, (a[2] + b[2] + c[2]) / 3}
		if dot(cr, sub(mid, center(mid))) > 0 {
			a2++
		}
	}
	return float64(a1) / float64(n), float64(a2) / float64(n)
}

func TestGLBImportAndBake(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cubes.glb")
	writeGLB(t, p)
	m, img, warns, err := Import(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("warnings: %v", warns)
	// 2 nodes × 2 primitives × 12 triangles.
	if m.Triangles() != 48 {
		t.Fatalf("triangles = %d, want 48", m.Triangles())
	}
	lo, hi := m.Bounds()
	// Node 0/1: cube scaled 2 around (0,1,0) → x −1..1, y 0..2. Node 2: mirrored unit cube at (5,0,0) → x 4.5..5.5, y −0.5..0.5.
	want := [2][3]float64{{-1, -0.5, -1}, {5.5, 2, 1}}
	for q := 0; q < 3; q++ {
		if math.Abs(lo[q]-want[0][q]) > 1e-5 || math.Abs(hi[q]-want[1][q]) > 1e-5 {
			t.Fatalf("bounds %v..%v, want %v..%v", lo, hi, want[0], want[1])
		}
	}
	center := func(v [3]float64) [3]float64 {
		if v[0] > 3 {
			return [3]float64{5, 0, 0}
		}
		return [3]float64{0, 1, 0}
	}
	if n, o := outward(m, center); n != 1 || o != 1 {
		t.Fatalf("imported winding: along normal %.2f, outwards %.2f (want 1, 1; the mirrored node must be flipped)", n, o)
	}
	// Atlas: the red texture and a blue swatch.
	var red, blue bool
	for _, uv := range m.UV {
		c := img.NRGBAAt(int(uv[0]*float64(img.Bounds().Dx())), int(uv[1]*float64(img.Bounds().Dy())))
		red = red || (c.R > 200 && c.B < 50)
		blue = blue || (c.B > 200 && c.R < 50)
	}
	if !red || !blue {
		t.Fatalf("atlas lookups: red %v blue %v", red, blue)
	}

	// Source round trip.
	base := filepath.Join(dir, "src", "cubes.fig")
	if err := WriteSource(base, m, img); err != nil {
		t.Fatal(err)
	}
	src, err := ReadSource(base + ".obj")
	if err != nil {
		t.Fatal(err)
	}
	if src.Triangles() != 48 {
		t.Fatalf("source triangles = %d", src.Triangles())
	}
	for i := range m.UV {
		if math.Abs(src.UV[i][0]-m.UV[i][0]) > 1e-5 || math.Abs(src.UV[i][1]-m.UV[i][1]) > 1e-5 {
			t.Fatalf("uv %d: %v != %v", i, src.UV[i], m.UV[i])
		}
	}

	// Bake: 1.3 units tall, bottom on the anchor; in Unity space the game's convention holds (cross along the normal).
	g, err := Place(src, Placement{Height: 1.3, Anchor: [3]float64{0.1, -0.5, 0.2}})
	if err != nil {
		t.Fatal(err)
	}
	glo, ghi := g.Bounds()
	if math.Abs(ghi[1]-glo[1]-1.3) > 1e-6 || math.Abs(glo[1]+0.5) > 1e-6 {
		t.Fatalf("baked y %v..%v", glo[1], ghi[1])
	}
	if cx := (glo[0] + ghi[0]) / 2; math.Abs(cx-0.1) > 1e-6 {
		t.Fatalf("baked centre x %v", cx)
	}
	if cz := (glo[2] + ghi[2]) / 2; math.Abs(cz-0.2) > 1e-6 {
		t.Fatalf("baked centre z %v", cz)
	}
	if n, _ := outward(g, func(v [3]float64) [3]float64 { return v }); n != 1 {
		t.Fatalf("baked: cross·normal > 0 for %.2f of triangles, want all (vanilla meshes: all)", n)
	}
	// The mirrored-node cube (+x in glTF) stays at +x; front (+z in glTF) becomes −z in Unity (vanilla toys face −z).
	if err := WriteGame(filepath.Join(dir, "game.obj"), g); err != nil {
		t.Fatal(err)
	}
}

func TestRotationOrder(t *testing.T) {
	// 90° about X turns +y into +z (right-handed); then 90° about Y turns +z into +x.
	v := apply3(rotation(90, 90, 0), [3]float64{0, 1, 0})
	if math.Abs(v[0]-1) > 1e-9 || math.Abs(v[1]) > 1e-9 || math.Abs(v[2]) > 1e-9 {
		t.Fatalf("got %v, want (1,0,0)", v)
	}
}

func TestOBJWithMTL(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tex.png"), pngBytes(t, color.NRGBA{0, 255, 0, 255}), 0o644); err != nil {
		t.Fatal(err)
	}
	mtl := "newmtl green\nKd 1 1 1\nmap_Kd -s 1 1 1 C:\\somewhere\\else\\tex.png\nnewmtl plain\nKd 1 0.5 0\n"
	obj := "mtllib m.mtl\nv 0 0 0\nv 1 0 0\nv 1 1 0\nv 0 1 0\nvt 0 0\nvt 1 0\nvt 1 1\nvt 0 1\n" +
		"usemtl green\nf 1/1 2/2 3/3 4/4\nusemtl plain\nf -4 -2 -3\n"
	_ = os.WriteFile(filepath.Join(dir, "m.mtl"), []byte(mtl), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "m.obj"), []byte(obj), 0o644)
	m, img, warns, err := Import(filepath.Join(dir, "m.obj"))
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("warnings: %v", warns)
	}
	if m.Triangles() != 3 {
		t.Fatalf("triangles = %d, want 3 (quad fan + triangle)", m.Triangles())
	}
	var green, orange bool
	for _, uv := range m.UV {
		c := img.NRGBAAt(int(uv[0]*float64(img.Bounds().Dx()-1)), int(uv[1]*float64(img.Bounds().Dy()-1)))
		green = green || (c.G > 200 && c.R < 50)
		orange = orange || (c.R > 200 && c.G > 100 && c.G < 160)
	}
	if !green || !orange {
		t.Fatalf("green %v orange %v", green, orange)
	}
}

// TestSample imports a real model: FIG_SAMPLE=<model file> FIG_OUT=<folder> go test -run TestSample ./internal/figurine/
func TestSample(t *testing.T) {
	src := os.Getenv("FIG_SAMPLE")
	if src == "" {
		t.Skip("FIG_SAMPLE not set")
	}
	m, img, warns, err := Import(src)
	if err != nil {
		t.Fatal(err)
	}
	lo, hi := m.Bounds()
	t.Logf("%s: %d vertices, %d triangles, texture %v, bounds %v..%v, warnings %v", filepath.Base(src), len(m.Pos), m.Triangles(), img.Bounds().Size(), lo, hi, warns)
	if out := os.Getenv("FIG_OUT"); out != "" {
		base := filepath.Join(out, "sample.fig")
		if err := WriteSource(base, m, img); err != nil {
			t.Fatal(err)
		}
		// PiggyA's slot: 1.3 units tall, footprint centre (−0.044, −0.011), bottom −0.54.
		g, err := Place(m, Placement{Height: 1.3, Anchor: [3]float64{-0.0443, 0.1115 - 1.3077/2, -0.0112}})
		if err != nil {
			t.Fatal(err)
		}
		if err := WriteGame(filepath.Join(out, "sample_model.obj"), g); err != nil {
			t.Fatal(err)
		}
	}
}

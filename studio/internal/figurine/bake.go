package figurine

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Import reads a model file (.glb, .gltf, .obj) and combines it into one mesh + one texture.
func Import(path string) (*Mesh, *image.NRGBA, []string, error) { return ImportWith(path, nil) }

// TextureSet is a model's textures chosen by the user (a PBR set shared next to the model, e.g. an OBJ without its .mtl). The game
// draws one colour texture, so the normal map and ambient occlusion are baked into it as shading; other maps aren't used.
type TextureSet struct {
	Color         image.Image // base colour / albedo (nil = keep the model's own colours)
	Normal        image.Image // tangent-space normal map (or a grey height map)
	NormalDirectX bool        // green channel points down (DirectX convention)
	AO            image.Image // ambient occlusion (grey, multiplied)
}

// ImportWith is Import with a texture set chosen by the user laid over the whole model: every part uses it through its own UVs;
// parts without UVs keep their colour.
func ImportWith(path string, texture *TextureSet) (*Mesh, *image.NRGBA, []string, error) {
	var s *Scene
	var err error
	switch strings.ToLower(filepath.Ext(path)) {
	case ".glb", ".gltf":
		s, err = LoadGLTF(path)
	case ".obj":
		s, err = LoadOBJ(path)
	default:
		return nil, nil, nil, fmt.Errorf("unsupported model type %q (use .glb, .gltf or .obj)", filepath.Ext(path))
	}
	if err != nil {
		return nil, nil, nil, err
	}
	if texture != nil && (texture.Color != nil || texture.Normal != nil || texture.AO != nil) {
		useTexture(s, texture)
	}
	m, img, err := Combine(s)
	if err != nil {
		return nil, nil, s.Warnings, err
	}
	if m.Triangles() > WarnTriangles {
		s.warn(fmt.Sprintf("%d triangles is a lot for a shop item (a full shelf shows dozens of copies); under %d is recommended", m.Triangles(), WarnTriangles))
	}
	return m, img, s.Warnings, nil
}

// ---- source files (what the editor shows): right-handed, Y up, UV origin bottom-left

// WriteSource writes <base>.obj and <base>.png.
func WriteSource(base string, m *Mesh, img image.Image) error {
	if err := os.MkdirAll(filepath.Dir(base), 0o755); err != nil {
		return err
	}
	if err := writePNG(base+".png", img); err != nil {
		return err
	}
	return writeOBJ(base+".obj", sourceHeader, m)
}

// ReadSource reads a source OBJ written by WriteSource (one index per vertex, "f a/a/a"), keeping the vertex order; UVs go
// back to image space.
func ReadSource(path string) (*Mesh, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	m := &Mesh{}
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "v":
			if len(f) >= 4 {
				m.Pos = append(m.Pos, [3]float64{num(f[1]), num(f[2]), num(f[3])})
			}
		case "vt":
			if len(f) >= 3 {
				m.UV = append(m.UV, [2]float64{num(f[1]), 1 - num(f[2])})
			}
		case "vn":
			if len(f) >= 4 {
				m.Nrm = append(m.Nrm, [3]float64{num(f[1]), num(f[2]), num(f[3])})
			}
		case "f":
			if len(f) != 4 {
				return nil, fmt.Errorf("%s: not a TCG Studio figurine source (non-triangle face)", filepath.Base(path))
			}
			for _, c := range f[1:] {
				i, err := strconv.Atoi(strings.SplitN(c, "/", 2)[0])
				if err != nil || i < 1 {
					return nil, fmt.Errorf("%s: bad face", filepath.Base(path))
				}
				m.Idx = append(m.Idx, uint32(i-1))
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(m.UV) != len(m.Pos) || len(m.Nrm) != len(m.Pos) {
		return nil, fmt.Errorf("%s: not a TCG Studio figurine source (vertex/uv/normal counts differ)", filepath.Base(path))
	}
	for _, i := range m.Idx {
		if int(i) >= len(m.Pos) {
			return nil, fmt.Errorf("%s: face refers to a missing vertex", filepath.Base(path))
		}
	}
	if len(m.Idx) == 0 {
		return nil, fmt.Errorf("%s: no faces", filepath.Base(path))
	}
	return m, nil
}

// Placement turns a source model into the base toy's mesh space.
type Placement struct {
	RotX   float64    `json:"rotX"` // degrees, applied Z, then X, then Y
	RotY   float64    `json:"rotY"`
	RotZ   float64    `json:"rotZ"`
	Height float64    `json:"height"` // model height in mesh units (1 unit = 10 cm in game, before the shelf's box scale)
	Anchor [3]float64 `json:"anchor"` // Unity mesh space: centre x/z of the footprint and bottom y
}

// Rotation matrix R = Ry · Rx · Rz (right-handed, degrees).
func rotation(rx, ry, rz float64) [9]float64 {
	r := func(d float64) (float64, float64) { a := d * math.Pi / 180; return math.Cos(a), math.Sin(a) }
	cx, sx := r(rx)
	cy, sy := r(ry)
	cz, sz := r(rz)
	X := [9]float64{1, 0, 0, 0, cx, -sx, 0, sx, cx}
	Y := [9]float64{cy, 0, sy, 0, 1, 0, -sy, 0, cy}
	Z := [9]float64{cz, -sz, 0, sz, cz, 0, 0, 0, 1}
	return mul3(Y, mul3(X, Z))
}

func mul3(a, b [9]float64) [9]float64 {
	var o [9]float64
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			for k := 0; k < 3; k++ {
				o[r*3+c] += a[r*3+k] * b[k*3+c]
			}
		}
	}
	return o
}

func apply3(m [9]float64, v [3]float64) [3]float64 {
	return [3]float64{
		m[0]*v[0] + m[1]*v[1] + m[2]*v[2],
		m[3]*v[0] + m[4]*v[1] + m[5]*v[2],
		m[6]*v[0] + m[7]*v[1] + m[8]*v[2],
	}
}

// Place returns the model in Unity mesh space: rotated, scaled to the height, bottom-centre on the anchor, z mirrored
// (right- to left-handed) with the winding swapped so faces keep pointing outwards.
func Place(src *Mesh, p Placement) (*Mesh, error) {
	if p.Height <= 0 {
		return nil, fmt.Errorf("height must be > 0")
	}
	R := rotation(p.RotX, p.RotY, p.RotZ)
	rot := make([][3]float64, len(src.Pos))
	lo := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	hi := [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for i, v := range src.Pos {
		rot[i] = apply3(R, v)
		for q := 0; q < 3; q++ {
			lo[q] = math.Min(lo[q], rot[i][q])
			hi[q] = math.Max(hi[q], rot[i][q])
		}
	}
	h := hi[1] - lo[1]
	if h <= 0 {
		return nil, fmt.Errorf("the model is flat")
	}
	s := p.Height / h
	c := [3]float64{(lo[0] + hi[0]) / 2, lo[1], (lo[2] + hi[2]) / 2}
	out := &Mesh{Pos: make([][3]float64, len(rot)), Nrm: make([][3]float64, len(rot)), UV: src.UV}
	for i, v := range rot {
		gl := [3]float64{(v[0] - c[0]) * s, (v[1] - c[1]) * s, (v[2] - c[2]) * s}
		out.Pos[i] = [3]float64{gl[0] + p.Anchor[0], gl[1] + p.Anchor[1], -gl[2] + p.Anchor[2]}
		n := apply3(R, src.Nrm[i])
		out.Nrm[i] = normalize([3]float64{n[0], n[1], -n[2]})
	}
	out.Idx = make([]uint32, len(src.Idx))
	for t := 0; t+2 < len(src.Idx); t += 3 {
		out.Idx[t], out.Idx[t+1], out.Idx[t+2] = src.Idx[t], src.Idx[t+2], src.Idx[t+1]
	}
	return out, nil
}

// WriteGame writes the placed model for the mod (Unity space, UV origin bottom-left).
func WriteGame(path string, m *Mesh) error {
	return writeOBJ(path, gameHeader, m)
}

const (
	sourceHeader = "TCG Studio figurine source (right-handed, Y up, UV origin bottom-left)"
	gameHeader   = "TCG Studio figurine for TCG Custom Cards (Unity mesh space: left-handed, Y up, UV origin bottom-left)"
)

// SourceOBJ is a source model as WriteSource writes it (for the shared asset store).
func SourceOBJ(m *Mesh) []byte { return encodeOBJ(sourceHeader, m) }

// GameOBJ is a placed model as WriteGame writes it (for the shared asset store).
func GameOBJ(m *Mesh) []byte { return encodeOBJ(gameHeader, m) }

// PNG encodes an image as WriteSource writes its texture.
func PNG(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&b, img)
	return b.Bytes(), err
}

func encodeOBJ(header string, m *Mesh) []byte {
	var b bytes.Buffer
	_ = writeOBJTo(&b, header, m)
	return b.Bytes()
}

func writeOBJ(path, header string, m *Mesh) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := writeOBJTo(fh, header, m); err != nil {
		fh.Close()
		return err
	}
	return fh.Close()
}

func writeOBJTo(out io.Writer, header string, m *Mesh) error {
	w := bufio.NewWriterSize(out, 1<<20)
	f := func(v float64) string { return strconv.FormatFloat(v, 'g', 7, 64) }
	fmt.Fprintf(w, "# %s\n# vertices %d, triangles %d\n", header, len(m.Pos), m.Triangles())
	for _, v := range m.Pos {
		fmt.Fprintf(w, "v %s %s %s\n", f(v[0]), f(v[1]), f(v[2]))
	}
	for _, t := range m.UV {
		fmt.Fprintf(w, "vt %s %s\n", f(t[0]), f(1-t[1])) // image space → origin bottom-left
	}
	for _, n := range m.Nrm {
		fmt.Fprintf(w, "vn %s %s %s\n", f(n[0]), f(n[1]), f(n[2]))
	}
	for t := 0; t+2 < len(m.Idx); t += 3 {
		a, b, c := m.Idx[t]+1, m.Idx[t+1]+1, m.Idx[t+2]+1
		fmt.Fprintf(w, "f %d/%d/%d %d/%d/%d %d/%d/%d\n", a, a, a, b, b, b, c, c, c)
	}
	return w.Flush()
}

func writePNG(path string, img image.Image) error {
	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(fh, img); err != nil {
		fh.Close()
		return err
	}
	return fh.Close()
}

// useTexture puts the set on every part that has UVs, replacing the materials (and the "not found" warnings about them).
func useTexture(s *Scene, t *TextureSet) {
	img := t.Color
	if t.Normal != nil {
		n := t.Normal
		if t.NormalDirectX {
			n = flipGreen(n)
		}
		img = withDetail(img, shadeFromNormal(n, 1))
	}
	if t.AO != nil {
		img = withDetail(img, aoShade(t.AO))
	}
	if t.Color == nil {
		// Only shading maps: lay them over the model's own materials (their colours stay).
		for i := range s.Materials {
			if s.Materials[i].Image == nil {
				s.Materials[i].Image = img
			} else {
				s.Materials[i].Image = multiplyImages(s.Materials[i].Image, img)
			}
		}
		for i := range s.Parts {
			if s.Parts[i].UV != nil && s.Parts[i].Mat < 0 {
				s.Parts[i].Mat = len(s.Materials)
			}
		}
		s.Materials = append(s.Materials, Material{Name: "shading", Image: img, Factor: [4]float64{1, 1, 1, 1}})
		return
	}
	mi := len(s.Materials)
	s.Materials = append(s.Materials, Material{Name: "texture", Image: img, Factor: [4]float64{1, 1, 1, 1}})
	noUV := false
	for i := range s.Parts {
		if s.Parts[i].UV != nil {
			s.Parts[i].Mat = mi
			s.Parts[i].Color = nil
		} else {
			noUV = true
		}
	}
	kept := s.Warnings[:0]
	for _, w := range s.Warnings {
		if !strings.Contains(w, "not found") && !strings.Contains(w, "isn't in the .mtl") && !strings.Contains(w, "couldn't be read") {
			kept = append(kept, w)
		}
	}
	s.Warnings = kept
	if noUV {
		s.warn("some parts have no texture coordinates (UVs), so the texture can't be laid on them; they keep their colour")
	}
}

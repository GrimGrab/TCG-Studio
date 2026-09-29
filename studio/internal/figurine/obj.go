package figurine

import (
	"bufio"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
)

// LoadOBJ reads a Wavefront OBJ with its MTL (usemtl groups, Kd colours, map_Kd textures). OBJ has no unit or axis convention;
// it's taken as right-handed Y-up (Blender's default export), and the editor can turn it if needed.
func LoadOBJ(path string) (*Scene, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	dir := filepath.Dir(path)
	s := &Scene{}
	var v, vn [][3]float64
	var vt [][2]float64
	var vc [][4]float64 // "v x y z r g b" vertex colours (common extension)
	anyColor := false
	matIndex := map[string]int{}
	mtl := map[string]objMtl{}

	type key struct{ v, t, n int }
	type group struct {
		part Part
		idx  map[key]uint32
	}
	groups := map[int]*group{} // by material index (-1 = none)
	var order []int
	cur := -1

	getGroup := func(m int) *group {
		g, ok := groups[m]
		if !ok {
			g = &group{part: Part{Mat: m}, idx: map[key]uint32{}}
			groups[m] = g
			order = append(order, m)
		}
		return g
	}

	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	line := 0
	for sc.Scan() {
		line++
		t := strings.TrimSpace(sc.Text())
		if t == "" || t[0] == '#' {
			continue
		}
		f := strings.Fields(t)
		switch f[0] {
		case "v":
			if len(f) < 4 {
				return nil, fmt.Errorf("line %d: bad vertex", line)
			}
			p := [3]float64{num(f[1]), num(f[2]), num(f[3])}
			v = append(v, p)
			c := [4]float64{1, 1, 1, 1}
			if len(f) >= 7 {
				c = [4]float64{num(f[4]), num(f[5]), num(f[6]), 1}
				anyColor = true
			}
			vc = append(vc, c)
		case "vt":
			if len(f) < 2 {
				continue
			}
			uv := [2]float64{num(f[1]), 0}
			if len(f) > 2 {
				uv[1] = num(f[2])
			}
			vt = append(vt, uv)
		case "vn":
			if len(f) < 4 {
				continue
			}
			vn = append(vn, [3]float64{num(f[1]), num(f[2]), num(f[3])})
		case "mtllib":
			for _, name := range f[1:] {
				for k, m := range loadMTL(filepath.Join(dir, name), s) {
					mtl[k] = m
				}
			}
			// Some exporters put spaces in the file name.
			if len(f) > 2 {
				for k, m := range loadMTL(filepath.Join(dir, strings.Join(f[1:], " ")), s) {
					mtl[k] = m
				}
			}
		case "usemtl":
			name := strings.TrimSpace(strings.TrimPrefix(t, "usemtl"))
			i, ok := matIndex[name]
			if !ok {
				m := Material{Name: name, Factor: [4]float64{1, 1, 1, 1}}
				if d, ok := mtl[name]; ok {
					m.Factor = d.kd
					if d.mapKd != "" {
						m.Image = loadImage(dir, d.mapKd, s)
					}
				} else {
					s.warn(fmt.Sprintf("material %q isn't in the .mtl file (drawn white)", name))
				}
				i = len(s.Materials)
				s.Materials = append(s.Materials, m)
				matIndex[name] = i
			}
			cur = i
		case "f":
			g := getGroup(cur)
			var poly []uint32
			for _, c := range f[1:] {
				parts := strings.Split(c, "/")
				k := key{idx(parts[0], len(v)), -1, -1}
				if len(parts) > 1 && parts[1] != "" {
					k.t = idx(parts[1], len(vt))
				}
				if len(parts) > 2 && parts[2] != "" {
					k.n = idx(parts[2], len(vn))
				}
				if k.v < 0 || k.v >= len(v) || k.t >= len(vt) || k.n >= len(vn) {
					return nil, fmt.Errorf("line %d: face refers to a missing vertex", line)
				}
				vi, ok := g.idx[k]
				if !ok {
					vi = uint32(len(g.part.Pos))
					g.idx[k] = vi
					g.part.Pos = append(g.part.Pos, v[k.v])
					if anyColor {
						g.part.Color = append(g.part.Color, vc[k.v])
					}
					if k.t >= 0 {
						g.part.UV = append(g.part.UV, [2]float64{vt[k.t][0], 1 - vt[k.t][1]}) // OBJ: origin bottom-left
					} else {
						g.part.UV = append(g.part.UV, [2]float64{0, 0})
					}
					if k.n >= 0 {
						g.part.Nrm = append(g.part.Nrm, normalize(vn[k.n]))
					} else {
						g.part.Nrm = append(g.part.Nrm, [3]float64{})
					}
				}
				poly = append(poly, vi)
			}
			for i := 1; i+1 < len(poly); i++ {
				g.part.Idx = append(g.part.Idx, poly[0], poly[i], poly[i+1])
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	for _, m := range order {
		p := groups[m].part
		if len(p.Idx) == 0 {
			continue
		}
		// Fill missing normals.
		missing := false
		for _, n := range p.Nrm {
			if n == ([3]float64{}) {
				missing = true
				break
			}
		}
		if missing {
			sn := smoothNormals(p.Pos, p.Idx)
			for i, n := range p.Nrm {
				if n == ([3]float64{}) {
					p.Nrm[i] = sn[i]
				}
			}
		}
		s.Parts = append(s.Parts, p)
	}
	if len(s.Parts) == 0 {
		return nil, fmt.Errorf("the OBJ file has no faces")
	}
	return s, nil
}

type objMtl struct {
	kd    [4]float64
	mapKd string
}

func loadMTL(path string, s *Scene) map[string]objMtl {
	out := map[string]objMtl{}
	fh, err := os.Open(path)
	if err != nil {
		s.warn(fmt.Sprintf("material file %s not found next to the model (drawn white)", filepath.Base(path)))
		return out
	}
	defer fh.Close()
	sc := bufio.NewScanner(fh)
	cur := ""
	for sc.Scan() {
		t := strings.TrimSpace(sc.Text())
		f := strings.Fields(t)
		if len(f) == 0 || f[0][0] == '#' {
			continue
		}
		switch strings.ToLower(f[0]) {
		case "newmtl":
			cur = strings.TrimSpace(t[len(f[0]):])
			out[cur] = objMtl{kd: [4]float64{1, 1, 1, 1}}
		case "kd":
			if len(f) >= 4 && cur != "" {
				m := out[cur]
				m.kd = [4]float64{num(f[1]), num(f[2]), num(f[3]), m.kd[3]}
				out[cur] = m
			}
		case "d":
			if len(f) >= 2 && cur != "" {
				m := out[cur]
				m.kd[3] = num(f[1])
				out[cur] = m
			}
		case "map_kd":
			if cur != "" {
				// Options (-s, -o, -bm …) come before the file name; the name is the rest after the last option value.
				m := out[cur]
				m.mapKd = mapFile(f[1:])
				out[cur] = m
			}
		}
	}
	return out
}

// mapFile skips "-opt value…" pairs and returns the (possibly space-containing) file name.
func mapFile(f []string) string {
	args := map[string]int{"-blendu": 1, "-blendv": 1, "-bm": 1, "-boost": 1, "-cc": 1, "-clamp": 1, "-imfchan": 1,
		"-mm": 2, "-o": 3, "-s": 3, "-t": 3, "-texres": 1}
	i := 0
	for i < len(f) {
		n, ok := args[strings.ToLower(f[i])]
		if !ok {
			break
		}
		i++
		// -o/-s/-t take 1–3 numbers.
		for k := 0; k < n && i < len(f); k++ {
			if _, err := strconv.ParseFloat(f[i], 64); err != nil && k > 0 {
				break
			}
			i++
		}
	}
	return strings.Join(f[i:], " ")
}

func loadImage(dir, name string, s *Scene) image.Image {
	p := name
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, filepath.FromSlash(strings.ReplaceAll(name, `\`, "/")))
	}
	fh, err := os.Open(p)
	if err != nil {
		// Exporters often write absolute paths from another machine: try the bare file name next to the model.
		fh, err = os.Open(filepath.Join(dir, filepath.Base(strings.ReplaceAll(name, `\`, "/"))))
	}
	if err != nil {
		s.warn(fmt.Sprintf("texture %q not found next to the model (drawn in its plain colour)", name))
		return nil
	}
	defer fh.Close()
	img, _, err := image.Decode(fh)
	if err != nil {
		s.warn(fmt.Sprintf("texture %q couldn't be read: %v", name, err))
		return nil
	}
	return img
}

func num(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// idx resolves a 1-based (or negative, relative) OBJ index to 0-based; -1 when empty.
func idx(s string, count int) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return -2
	}
	if i < 0 {
		return count + i
	}
	return i - 1
}

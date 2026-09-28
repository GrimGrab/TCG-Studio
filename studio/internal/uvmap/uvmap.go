// Package uvmap reads the meshes the mod exports (templates\accessories\*.obj: Unity space, UV origin bottom-left) and
// splits them into panels: connected UV islands with one dominant facing (front/back/left/right/top/bottom). The accessory
// editor uses panels to show how a model's texture is laid out and to map face-plane artwork into texture space.
package uvmap

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

type Vec3 [3]float64
type Vec2 [2]float64

type Mesh struct {
	Pos  []Vec3
	UV   []Vec2
	Norm []Vec3
	Tris [][3]int // indices into Pos/UV/Norm (the exporter writes one index per vertex)
}

// LoadOBJ parses the subset of OBJ the mod writes (v/vt/vn and f a/a/a with equal indices).
func LoadOBJ(path string) (*Mesh, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	m := &Mesh{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	num := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) == 0 {
			continue
		}
		switch fs[0] {
		case "v":
			m.Pos = append(m.Pos, Vec3{num(fs[1]), num(fs[2]), num(fs[3])})
		case "vt":
			m.UV = append(m.UV, Vec2{num(fs[1]), num(fs[2])})
		case "vn":
			m.Norm = append(m.Norm, Vec3{num(fs[1]), num(fs[2]), num(fs[3])})
		case "f":
			if len(fs) < 4 {
				continue
			}
			var t [3]int
			for k := 0; k < 3; k++ {
				i, err := strconv.Atoi(strings.SplitN(fs[1+k], "/", 2)[0])
				if err != nil {
					return nil, fmt.Errorf("bad face %q", sc.Text())
				}
				t[k] = i - 1
			}
			m.Tris = append(m.Tris, t)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(m.UV) != len(m.Pos) {
		return nil, fmt.Errorf("mesh has no per-vertex UVs")
	}
	return m, nil
}

// Face names by dominant object-space normal. Unity: +Z = forward (the side facing away from the viewer by default),
// so which one is the printed "front" of a model is decided per model by the caller.
var faceNames = map[[2]int]string{
	{0, 1}: "right", {0, -1}: "left", {1, 1}: "top", {1, -1}: "bottom", {2, 1}: "back", {2, -1}: "front",
}

// Panel is one UV island with a consistent facing.
type Panel struct {
	Face    string        `json:"face"`
	Axis    int           `json:"axis"` // 0 x, 1 y, 2 z
	Sign    int           `json:"sign"`
	Tris    int           `json:"tris"`
	UVMin   Vec2          `json:"uvMin"` // UV bounds (0..1, origin bottom-left)
	UVMax   Vec2          `json:"uvMax"`
	PosMin  Vec3          `json:"posMin"` // object-space bounds
	PosMax  Vec3          `json:"posMax"`
	AreaUV  float64       `json:"areaUV"`
	Area3D  float64       `json:"area3D"`
	Affine  [2][3]float64 `json:"affine"`  // (a,b,1) face-plane coords → (u,v); see Plane
	Plane   [2]int        `json:"plane"`   // which object axes are the panel's (a,b) plane coords
	Outline [][2]Vec2     `json:"outline"` // boundary edges in UV space
	Flat    bool          `json:"flat"`    // all triangles within ~15° of the facing (false = curved/rounded part)
}

func sub(a, b Vec3) Vec3 { return Vec3{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func cross(a, b Vec3) Vec3 {
	return Vec3{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}
func length(a Vec3) float64 { return math.Sqrt(a[0]*a[0] + a[1]*a[1] + a[2]*a[2]) }

func (m *Mesh) triNormal(t [3]int) (Vec3, float64) {
	n := cross(sub(m.Pos[t[1]], m.Pos[t[0]]), sub(m.Pos[t[2]], m.Pos[t[0]]))
	l := length(n)
	if l == 0 {
		return Vec3{}, 0
	}
	// Winding can differ from the stored normals; trust the stored vertex normals for the direction.
	if len(m.Norm) == len(m.Pos) {
		avg := Vec3{}
		for _, i := range t {
			for k := 0; k < 3; k++ {
				avg[k] += m.Norm[i][k]
			}
		}
		if avg[0]*n[0]+avg[1]*n[1]+avg[2]*n[2] < 0 {
			l = -l
		}
	}
	return Vec3{n[0] / l, n[1] / l, n[2] / l}, math.Abs(l) / 2
}

func dominant(n Vec3) (axis, sign int) {
	axis = 0
	for k := 1; k < 3; k++ {
		if math.Abs(n[k]) > math.Abs(n[axis]) {
			axis = k
		}
	}
	sign = 1
	if n[axis] < 0 {
		sign = -1
	}
	return
}

func uvArea(a, b, c Vec2) float64 {
	return math.Abs((b[0]-a[0])*(c[1]-a[1])-(c[0]-a[0])*(b[1]-a[1])) / 2
}

// Panels groups triangles into UV islands (triangles sharing a UV-space edge) that also share a dominant facing,
// biggest first. Tiny islands (< minArea of the UV square) are dropped.
func (m *Mesh) Panels(minArea float64) []Panel {
	n := len(m.Tris)
	facing := make([][2]int, n)
	normals := make([]Vec3, n)
	area3 := make([]float64, n)
	for i, t := range m.Tris {
		nn, a := m.triNormal(t)
		normals[i], area3[i] = nn, a
		ax, sg := dominant(nn)
		facing[i] = [2]int{ax, sg}
	}
	// Edge key in UV space (quantized) so seams split islands even when positions coincide.
	type ekey [4]int32
	q := func(v Vec2) (int32, int32) { return int32(math.Round(v[0] * 1e5)), int32(math.Round(v[1] * 1e5)) }
	key := func(a, b int) ekey {
		ax, ay := q(m.UV[a])
		bx, by := q(m.UV[b])
		if ax > bx || (ax == bx && ay > by) {
			ax, ay, bx, by = bx, by, ax, ay
		}
		return ekey{ax, ay, bx, by}
	}
	edges := map[ekey][]int{}
	for i, t := range m.Tris {
		for k := 0; k < 3; k++ {
			e := key(t[k], t[(k+1)%3])
			edges[e] = append(edges[e], i)
		}
	}
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	for _, ts := range edges {
		for j := 1; j < len(ts); j++ {
			if facing[ts[0]] == facing[ts[j]] {
				parent[find(ts[0])] = find(ts[j])
			}
		}
	}
	groups := map[int][]int{}
	for i := 0; i < n; i++ {
		groups[find(i)] = append(groups[find(i)], i)
	}

	var out []Panel
	for _, g := range groups {
		f := facing[g[0]]
		p := Panel{Axis: f[0], Sign: f[1], Face: faceNames[f], Tris: len(g), Flat: true}
		p.UVMin, p.UVMax = Vec2{math.Inf(1), math.Inf(1)}, Vec2{math.Inf(-1), math.Inf(-1)}
		p.PosMin, p.PosMax = Vec3{math.Inf(1), math.Inf(1), math.Inf(1)}, Vec3{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
		edgeCount := map[ekey]int{}
		edgeUV := map[ekey][2]Vec2{}
		for _, ti := range g {
			t := m.Tris[ti]
			p.AreaUV += uvArea(m.UV[t[0]], m.UV[t[1]], m.UV[t[2]])
			p.Area3D += area3[ti]
			if math.Abs(normals[ti][f[0]]) < 0.966 {
				p.Flat = false
			}
			for k := 0; k < 3; k++ {
				v := t[k]
				for c := 0; c < 2; c++ {
					p.UVMin[c] = math.Min(p.UVMin[c], m.UV[v][c])
					p.UVMax[c] = math.Max(p.UVMax[c], m.UV[v][c])
				}
				for c := 0; c < 3; c++ {
					p.PosMin[c] = math.Min(p.PosMin[c], m.Pos[v][c])
					p.PosMax[c] = math.Max(p.PosMax[c], m.Pos[v][c])
				}
				e := key(t[k], t[(k+1)%3])
				edgeCount[e]++
				edgeUV[e] = [2]Vec2{m.UV[t[k]], m.UV[t[(k+1)%3]]}
			}
		}
		if p.AreaUV < minArea {
			continue
		}
		for e, c := range edgeCount {
			if c == 1 {
				p.Outline = append(p.Outline, edgeUV[e])
			}
		}
		p.Plane = planeAxes(f[0])
		p.Affine = m.fitAffine(g, p.Plane)
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AreaUV > out[j].AreaUV })
	return out
}

// planeAxes: the two object axes spanning a face with normal along axis (horizontal first, then vertical).
func planeAxes(axis int) [2]int {
	switch axis {
	case 0:
		return [2]int{2, 1} // side faces: z across, y up
	case 1:
		return [2]int{0, 2} // top/bottom: x across, z depth
	default:
		return [2]int{0, 1} // front/back: x across, y up
	}
}

// fitAffine least-squares fits uv = A·(a, b, 1) over the island's vertices (a, b = object coords on the plane axes).
func (m *Mesh) fitAffine(tris []int, plane [2]int) [2][3]float64 {
	var ata [3][3]float64
	var atu, atv [3]float64
	seen := map[int]bool{}
	for _, ti := range tris {
		for _, v := range m.Tris[ti] {
			if seen[v] {
				continue
			}
			seen[v] = true
			x := [3]float64{m.Pos[v][plane[0]], m.Pos[v][plane[1]], 1}
			for r := 0; r < 3; r++ {
				for c := 0; c < 3; c++ {
					ata[r][c] += x[r] * x[c]
				}
				atu[r] += x[r] * m.UV[v][0]
				atv[r] += x[r] * m.UV[v][1]
			}
		}
	}
	return [2][3]float64{solve3(ata, atu), solve3(ata, atv)}
}

func solve3(a [3][3]float64, b [3]float64) [3]float64 {
	// Gaussian elimination with partial pivoting; degenerate systems give zeros.
	m := [3][4]float64{}
	for r := 0; r < 3; r++ {
		copy(m[r][:3], a[r][:])
		m[r][3] = b[r]
	}
	for c := 0; c < 3; c++ {
		p := c
		for r := c + 1; r < 3; r++ {
			if math.Abs(m[r][c]) > math.Abs(m[p][c]) {
				p = r
			}
		}
		if math.Abs(m[p][c]) < 1e-12 {
			return [3]float64{}
		}
		m[c], m[p] = m[p], m[c]
		for r := 0; r < 3; r++ {
			if r == c {
				continue
			}
			f := m[r][c] / m[c][c]
			for k := c; k < 4; k++ {
				m[r][k] -= f * m[c][k]
			}
		}
	}
	return [3]float64{m[0][3] / m[0][0], m[1][3] / m[1][1], m[2][3] / m[2][2]}
}

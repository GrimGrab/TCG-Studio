// Package figurine imports 3D models (glTF/GLB, OBJ+MTL) for custom figurines and bakes them for the mod.
//
// Pipeline:
//
//	Import (glTF / OBJ) → Scene (parts with materials, right-handed Y-up, UVs with a top-left origin)
//	Combine             → one Mesh + one texture (materials packed into an atlas, untextured colours as swatches)
//	WriteSource         → images/src/<name>.fig.obj + .png (right-handed, Y up, UV origin bottom-left): what the editor shows
//	Bake                → images/<id>_model.obj in Unity space (left-handed: z mirrored, winding swapped), placed in the base
//	                      toy's mesh space; the mod's Runtime/MeshLoader reads it as-is.
package figurine

import (
	"image"
	"image/color"
	"math"
)

// Material of a part: an optional base-colour image (tinted by Factor) or just a colour.
type Material struct {
	Name   string
	Image  image.Image // nil = untextured
	Factor [4]float64  // base colour factor (RGBA, linear multiplier on the image)
}

// Part is one drawable piece (a glTF primitive or an OBJ material group), already in model space.
type Part struct {
	Pos   [][3]float64
	Nrm   [][3]float64 // may be nil (computed)
	UV    [][2]float64 // image space: origin top-left, v down. May be nil.
	Color [][4]float64 // vertex colours, may be nil
	Idx   []uint32     // triangles
	Mat   int          // index into Scene.Materials, -1 = none
}

// Scene is a whole imported model before combining.
type Scene struct {
	Parts     []Part
	Materials []Material
	Warnings  []string
}

func (s *Scene) warn(msg string) {
	for _, w := range s.Warnings {
		if w == msg {
			return
		}
	}
	s.Warnings = append(s.Warnings, msg)
}

// Mesh is a combined model: one texture, UVs in image space (origin top-left).
type Mesh struct {
	Pos [][3]float64
	Nrm [][3]float64
	UV  [][2]float64
	Idx []uint32
}

func (m *Mesh) Triangles() int { return len(m.Idx) / 3 }

// Bounds returns min and max corners.
func (m *Mesh) Bounds() (lo, hi [3]float64) {
	lo = [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	hi = [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, p := range m.Pos {
		for q := 0; q < 3; q++ {
			lo[q] = math.Min(lo[q], p[q])
			hi[q] = math.Max(hi[q], p[q])
		}
	}
	return
}

// Limits.
const (
	MaxTriangles  = 150000 // refuse above (the mod's own limit is 200k)
	WarnTriangles = 30000  // a full shelf shows dozens of copies
)

func sub(a, b [3]float64) [3]float64 { return [3]float64{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func cross(a, b [3]float64) [3]float64 {
	return [3]float64{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}
func dot(a, b [3]float64) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
func normalize(a [3]float64) [3]float64 {
	l := math.Sqrt(dot(a, a))
	if l < 1e-20 {
		return [3]float64{0, 1, 0}
	}
	return [3]float64{a[0] / l, a[1] / l, a[2] / l}
}

// smoothNormals computes area-weighted vertex normals for a part without normals.
func smoothNormals(pos [][3]float64, idx []uint32) [][3]float64 {
	n := make([][3]float64, len(pos))
	for i := 0; i+2 < len(idx); i += 3 {
		a, b, c := pos[idx[i]], pos[idx[i+1]], pos[idx[i+2]]
		f := cross(sub(b, a), sub(c, a))
		for _, v := range idx[i : i+3] {
			n[v] = [3]float64{n[v][0] + f[0], n[v][1] + f[1], n[v][2] + f[2]}
		}
	}
	for i := range n {
		n[i] = normalize(n[i])
	}
	return n
}

func toNRGBA(c [4]float64) color.NRGBA {
	b := func(v float64) uint8 { return uint8(math.Round(math.Max(0, math.Min(1, v)) * 255)) }
	return color.NRGBA{b(c[0]), b(c[1]), b(c[2]), b(c[3])}
}

package epl

import (
	"fmt"
	"math"
	"strings"

	"tcgstudio/internal/figurine"
	"tcgstudio/internal/unityfs"
)

// PrefabScene reads a prefab of the bundle (EPL names item models and furniture by their root GameObject) as a
// figurine.Scene: every active GameObject with a MeshFilter and an enabled MeshRenderer, in the root's space, converted to
// the right-handed, top-left-UV form figurine.Combine expects. Each submesh uses its renderer material's main texture.
func (a *Assets) PrefabScene(name string) (*figurine.Scene, error) {
	root := a.GameObject(name)
	if root == nil {
		return nil, fmt.Errorf("model %q not in the bundle", name)
	}
	w := prefabWalker{a: a, sc: &figurine.Scene{}, mats: map[*unityfs.Object]int{}}
	if err := w.walk(root, identity(), true, 0); err != nil {
		return nil, err
	}
	if len(w.sc.Parts) == 0 {
		return nil, fmt.Errorf("model %q has no visible meshes", name)
	}
	return w.sc, nil
}

type prefabWalker struct {
	a    *Assets
	sc   *figurine.Scene
	mats map[*unityfs.Object]int
}

func (w *prefabWalker) walk(o *unityfs.Object, parent mat4, isRoot bool, depth int) error {
	if depth > 64 {
		return fmt.Errorf("prefab hierarchy too deep")
	}
	g, err := unityfs.ReadGameObject(o)
	if err != nil {
		return err
	}
	if !g.Active {
		return nil
	}
	env := w.a.env
	var tr unityfs.Transform
	var mf *unityfs.MeshFilter
	var rend *unityfs.Renderer
	for _, c := range g.Components {
		co, _ := env.Resolve(o.File, c)
		if co == nil {
			continue
		}
		switch co.ClassID {
		case unityfs.ClassTransform, unityfs.ClassRectTransform:
			if t, err := unityfs.ReadTransform(co); err == nil {
				tr = t
			}
		case unityfs.ClassMeshFilter:
			if f, err := unityfs.ReadMeshFilter(co); err == nil {
				mf = &f
			}
		case unityfs.ClassMeshRenderer:
			if r, err := unityfs.ReadRenderer(co); err == nil {
				rend = &r
			}
		}
	}
	world := parent
	if !isRoot { // the prefab's own root placement doesn't matter: the model is placed on its slot afterwards
		world = parent.mul(local(tr))
	}
	if mf != nil && rend != nil && rend.Enabled {
		if err := w.addMesh(o.File, mf, rend, world); err != nil {
			return err
		}
	}
	for _, c := range tr.Children {
		to, _ := env.Resolve(o.File, c)
		if to == nil {
			continue
		}
		ct, err := unityfs.ReadTransform(to)
		if err != nil {
			continue
		}
		child, _ := env.Resolve(to.File, ct.GameObject)
		if child == nil {
			continue
		}
		if err := w.walk(child, world, false, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (w *prefabWalker) addMesh(f *unityfs.File, mf *unityfs.MeshFilter, rend *unityfs.Renderer, world mat4) error {
	env := w.a.env
	mo, _ := env.Resolve(f, mf.Mesh)
	if mo == nil {
		return nil
	}
	m, err := unityfs.ReadMesh(env, mo)
	if err != nil {
		return err
	}
	flip := world.det3() < 0 // a mirrored transform flips the winding once more
	for si, sub := range m.Subs {
		p := figurine.Part{Mat: -1}
		remap := map[uint32]uint32{}
		vert := func(v uint32) uint32 {
			if i, ok := remap[v]; ok {
				return i
			}
			i := uint32(len(p.Pos))
			remap[v] = i
			pos := world.point(f64(m.Pos[v]))
			p.Pos = append(p.Pos, [3]float64{pos[0], pos[1], -pos[2]}) // left- → right-handed
			if int(v) < len(m.Normal) {
				n := world.vector(f64(m.Normal[v]))
				p.Nrm = append(p.Nrm, [3]float64{n[0], n[1], -n[2]})
			}
			if int(v) < len(m.UV) {
				p.UV = append(p.UV, [2]float64{float64(m.UV[v][0]), 1 - float64(m.UV[v][1])})
			}
			return i
		}
		for t := 0; t+2 < len(sub); t += 3 {
			a, b, c := vert(sub[t]), vert(sub[t+1]), vert(sub[t+2])
			if flip {
				p.Idx = append(p.Idx, a, b, c)
			} else { // mirroring z swaps the winding
				p.Idx = append(p.Idx, a, c, b)
			}
		}
		if len(p.Nrm) != len(p.Pos) { // no normals in the mesh: smooth ones from the faces
			p.Nrm = smoothNormals(p.Pos, p.Idx)
		}
		if len(p.UV) != len(p.Pos) {
			p.UV = nil
		}
		if len(rend.Materials) > 0 {
			mi := si
			if mi >= len(rend.Materials) {
				mi = len(rend.Materials) - 1
			}
			p.Mat = w.material(f, rend.Materials[mi])
		}
		if len(p.Idx) > 0 {
			w.sc.Parts = append(w.sc.Parts, p)
		}
	}
	return nil
}

// material adds a renderer material to the scene once (its main texture, or plain white).
func (w *prefabWalker) material(f *unityfs.File, ref unityfs.PPtr) int {
	mo, _ := w.a.env.Resolve(f, ref)
	if mo == nil {
		return -1
	}
	if i, ok := w.mats[mo]; ok {
		return i
	}
	m := figurine.Material{Factor: [4]float64{1, 1, 1, 1}}
	if mat, err := unityfs.ReadMaterial(mo); err == nil {
		m.Name = mat.Name
		if te, ok := mat.MainTexture(); ok {
			if to, _ := w.a.env.Resolve(mo.File, te.Texture); to != nil {
				if img, err := w.a.texture(to); err == nil {
					m.Image = img
				} else {
					w.sc.Warnings = append(w.sc.Warnings, fmt.Sprintf("%s: %v", strings.TrimSpace(mat.Name), err))
				}
			}
		}
	}
	w.mats[mo] = len(w.sc.Materials)
	w.sc.Materials = append(w.sc.Materials, m)
	return w.mats[mo]
}

// PrefabScripts lists the script classes on a prefab's GameObjects (the game components a mod's furniture is built on).
func (a *Assets) PrefabScripts(name string) []string {
	root := a.GameObject(name)
	if root == nil {
		return nil
	}
	var out []string
	var walk func(o *unityfs.Object, depth int)
	walk = func(o *unityfs.Object, depth int) {
		g, err := unityfs.ReadGameObject(o)
		if err != nil || depth > 64 {
			return
		}
		for _, c := range g.Components {
			co, _ := a.env.Resolve(o.File, c)
			if co == nil {
				continue
			}
			switch co.ClassID {
			case unityfs.ClassMonoBehaviour:
				if s := a.env.ScriptClass(co); s != "" {
					out = append(out, s)
				}
			case unityfs.ClassTransform, unityfs.ClassRectTransform:
				t, err := unityfs.ReadTransform(co)
				if err != nil {
					continue
				}
				for _, ch := range t.Children {
					if to, _ := a.env.Resolve(co.File, ch); to != nil {
						if ct, err := unityfs.ReadTransform(to); err == nil {
							if child, _ := a.env.Resolve(to.File, ct.GameObject); child != nil {
								walk(child, depth+1)
							}
						}
					}
				}
			}
		}
	}
	walk(root, 0)
	return out
}

// ---- Unity transform math (column vectors, M = T·R·S, quaternions x y z w)

type mat4 [4][4]float64

func identity() mat4 { return mat4{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}, {0, 0, 0, 1}} }

func local(t unityfs.Transform) mat4 {
	x, y, z, w := float64(t.Rotation[0]), float64(t.Rotation[1]), float64(t.Rotation[2]), float64(t.Rotation[3])
	if x == 0 && y == 0 && z == 0 && w == 0 {
		w = 1
	}
	r := [3][3]float64{
		{1 - 2*(y*y+z*z), 2 * (x*y - z*w), 2 * (x*z + y*w)},
		{2 * (x*y + z*w), 1 - 2*(x*x+z*z), 2 * (y*z - x*w)},
		{2 * (x*z - y*w), 2 * (y*z + x*w), 1 - 2*(x*x+y*y)},
	}
	s := f64(t.Scale)
	if s == ([3]float64{}) {
		s = [3]float64{1, 1, 1}
	}
	var m mat4
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			m[i][j] = r[i][j] * s[j]
		}
		m[i][3] = float64(t.Position[i])
	}
	m[3] = [4]float64{0, 0, 0, 1}
	return m
}

func (a mat4) mul(b mat4) mat4 {
	var m mat4
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 4; k++ {
				m[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return m
}

func (m mat4) point(v [3]float64) [3]float64 {
	return [3]float64{
		m[0][0]*v[0] + m[0][1]*v[1] + m[0][2]*v[2] + m[0][3],
		m[1][0]*v[0] + m[1][1]*v[1] + m[1][2]*v[2] + m[1][3],
		m[2][0]*v[0] + m[2][1]*v[1] + m[2][2]*v[2] + m[2][3],
	}
}

func (m mat4) vector(v [3]float64) [3]float64 {
	return [3]float64{
		m[0][0]*v[0] + m[0][1]*v[1] + m[0][2]*v[2],
		m[1][0]*v[0] + m[1][1]*v[1] + m[1][2]*v[2],
		m[2][0]*v[0] + m[2][1]*v[1] + m[2][2]*v[2],
	}
}

func (m mat4) det3() float64 {
	return m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) - m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) +
		m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
}

func f64(v [3]float32) [3]float64 { return [3]float64{float64(v[0]), float64(v[1]), float64(v[2])} }

func smoothNormals(pos [][3]float64, idx []uint32) [][3]float64 {
	n := make([][3]float64, len(pos))
	for i := 0; i+2 < len(idx); i += 3 {
		a, b, c := pos[idx[i]], pos[idx[i+1]], pos[idx[i+2]]
		u := [3]float64{b[0] - a[0], b[1] - a[1], b[2] - a[2]}
		v := [3]float64{c[0] - a[0], c[1] - a[1], c[2] - a[2]}
		f := [3]float64{u[1]*v[2] - u[2]*v[1], u[2]*v[0] - u[0]*v[2], u[0]*v[1] - u[1]*v[0]}
		for _, k := range idx[i : i+3] {
			n[k] = [3]float64{n[k][0] + f[0], n[k][1] + f[1], n[k][2] + f[2]}
		}
	}
	for i, v := range n {
		if l := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2]); l > 0 {
			n[i] = [3]float64{v[0] / l, v[1] / l, v[2] / l}
		} else {
			n[i] = [3]float64{0, 1, 0}
		}
	}
	return n
}

package gameextract

import (
	"math"
	"strings"

	"tcgstudio/internal/unityfs"
)

// node is a GameObject in a prefab hierarchy with its transform and components.
type node struct {
	obj      *unityfs.Object // the GameObject
	name     string
	active   bool
	tr       unityfs.Transform
	parent   *node
	children []*node
	comps    []*unityfs.Object
	scripts  []string // script classes of its MonoBehaviours
}

// graph indexes the GameObject/Transform hierarchy of one serialized file.
type graph struct {
	env  *unityfs.Env
	file *unityfs.File
	byGO map[int64]*node // GameObject path id → node
	byTr map[int64]*node // Transform path id → node
}

func newGraph(env *unityfs.Env, f *unityfs.File) *graph {
	g := &graph{env: env, file: f, byGO: map[int64]*node{}, byTr: map[int64]*node{}}
	for _, o := range f.Objects {
		if o.ClassID != unityfs.ClassGameObject {
			continue
		}
		gobj, err := unityfs.ReadGameObject(o)
		if err != nil {
			continue
		}
		n := &node{obj: o, name: gobj.Name, active: gobj.Active}
		for _, c := range gobj.Components {
			co, _ := env.Resolve(f, c)
			if co == nil {
				continue
			}
			n.comps = append(n.comps, co)
			switch co.ClassID {
			case unityfs.ClassTransform, unityfs.ClassRectTransform:
				if t, err := unityfs.ReadTransform(co); err == nil {
					n.tr = t
					g.byTr[co.PathID] = n
				}
			case unityfs.ClassMonoBehaviour:
				n.scripts = append(n.scripts, env.ScriptClass(co))
			}
		}
		g.byGO[o.PathID] = n
	}
	for _, n := range g.byGO {
		if n.tr.Father.FileID == 0 {
			n.parent = g.byTr[n.tr.Father.PathID]
		}
		for _, c := range n.tr.Children {
			if c.FileID == 0 {
				if ch := g.byTr[c.PathID]; ch != nil {
					n.children = append(n.children, ch)
				}
			}
		}
	}
	return g
}

func (g *graph) nodeOf(gameObject unityfs.PPtr) *node {
	if gameObject.FileID != 0 {
		return nil
	}
	return g.byGO[gameObject.PathID]
}

func (g *graph) transformNode(tr unityfs.PPtr) *node {
	if tr.FileID != 0 {
		return nil
	}
	return g.byTr[tr.PathID]
}

// has: a script of this class, or a subclass named "<class>_…" (the game's convention, e.g. InteractablePackagingBox_Item).
func (n *node) has(class string) bool {
	for _, s := range n.scripts {
		if s == class || strings.HasPrefix(s, class+"_") {
			return true
		}
	}
	return false
}

// hasInParent is GetComponentInParent != null (itself or any ancestor).
func (n *node) hasInParent(classes ...string) bool {
	for p := n; p != nil; p = p.parent {
		for _, c := range classes {
			if p.has(c) {
				return true
			}
		}
	}
	return false
}

// activeInHierarchy: this GameObject and all its ancestors are active.
func (n *node) activeInHierarchy() bool {
	for p := n; p != nil; p = p.parent {
		if !p.active {
			return false
		}
	}
	return true
}

// path is Transform path from the hierarchy root ("Root/Child/...").
func (n *node) path() string {
	parts := []string{}
	for p := n; p != nil; p = p.parent {
		parts = append([]string{p.name}, parts...)
	}
	return strings.Join(parts, "/")
}

// walk visits n and its descendants depth first in child order (GetComponentsInChildren order).
func (n *node) walk(f func(*node)) {
	f(n)
	for _, c := range n.children {
		c.walk(f)
	}
}

// ---- math (Unity conventions: column vectors, M = T·R·S, quaternions x y z w)

type mat [4][4]float64
type quat [4]float64

func identity() mat { return mat{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}, {0, 0, 0, 1}} }

func (q quat) mat3() [3][3]float64 {
	x, y, z, w := q[0], q[1], q[2], q[3]
	return [3][3]float64{
		{1 - 2*(y*y+z*z), 2 * (x*y - z*w), 2 * (x*z + y*w)},
		{2 * (x*y + z*w), 1 - 2*(x*x+z*z), 2 * (y*z - x*w)},
		{2 * (x*z - y*w), 2 * (y*z + x*w), 1 - 2*(x*x+y*y)},
	}
}

func (a quat) mul(b quat) quat {
	return quat{
		a[3]*b[0] + a[0]*b[3] + a[1]*b[2] - a[2]*b[1],
		a[3]*b[1] - a[0]*b[2] + a[1]*b[3] + a[2]*b[0],
		a[3]*b[2] + a[0]*b[1] - a[1]*b[0] + a[2]*b[3],
		a[3]*b[3] - a[0]*b[0] - a[1]*b[1] - a[2]*b[2],
	}
}

func (q quat) inverse() quat { return quat{-q[0], -q[1], -q[2], q[3]} }

func (q quat) rotate(v [3]float64) [3]float64 {
	m := q.mat3()
	return [3]float64{
		m[0][0]*v[0] + m[0][1]*v[1] + m[0][2]*v[2],
		m[1][0]*v[0] + m[1][1]*v[1] + m[1][2]*v[2],
		m[2][0]*v[0] + m[2][1]*v[1] + m[2][2]*v[2],
	}
}

func f64(v [3]float32) [3]float64 { return [3]float64{float64(v[0]), float64(v[1]), float64(v[2])} }

func (n *node) localQuat() quat {
	r := n.tr.Rotation
	return quat{float64(r[0]), float64(r[1]), float64(r[2]), float64(r[3])}
}

func (n *node) local() mat {
	r := n.localQuat().mat3()
	p, s := f64(n.tr.Position), f64(n.tr.Scale)
	var m mat
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			m[i][j] = r[i][j] * s[j]
		}
		m[i][3] = p[i]
	}
	m[3] = [4]float64{0, 0, 0, 1}
	return m
}

func (n *node) world() mat {
	if n.parent == nil {
		return n.local()
	}
	return mul(n.parent.world(), n.local())
}

func (n *node) worldQuat() quat {
	if n.parent == nil {
		return n.localQuat()
	}
	return n.parent.worldQuat().mul(n.localQuat())
}

func mul(a, b mat) mat {
	var m mat
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 4; k++ {
				m[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return m
}

func (m mat) point(v [3]float64) [3]float64 {
	return [3]float64{
		m[0][0]*v[0] + m[0][1]*v[1] + m[0][2]*v[2] + m[0][3],
		m[1][0]*v[0] + m[1][1]*v[1] + m[1][2]*v[2] + m[1][3],
		m[2][0]*v[0] + m[2][1]*v[1] + m[2][2]*v[2] + m[2][3],
	}
}

func (m mat) vector(v [3]float64) [3]float64 {
	return [3]float64{
		m[0][0]*v[0] + m[0][1]*v[1] + m[0][2]*v[2],
		m[1][0]*v[0] + m[1][1]*v[1] + m[1][2]*v[2],
		m[2][0]*v[0] + m[2][1]*v[1] + m[2][2]*v[2],
	}
}

func (m mat) det3() float64 {
	return m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) - m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) + m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
}

// inverse of an affine matrix.
func (m mat) inverse() mat {
	d := m.det3()
	if d == 0 {
		return identity()
	}
	var r mat
	r[0][0] = (m[1][1]*m[2][2] - m[1][2]*m[2][1]) / d
	r[0][1] = (m[0][2]*m[2][1] - m[0][1]*m[2][2]) / d
	r[0][2] = (m[0][1]*m[1][2] - m[0][2]*m[1][1]) / d
	r[1][0] = (m[1][2]*m[2][0] - m[1][0]*m[2][2]) / d
	r[1][1] = (m[0][0]*m[2][2] - m[0][2]*m[2][0]) / d
	r[1][2] = (m[0][2]*m[1][0] - m[0][0]*m[1][2]) / d
	r[2][0] = (m[1][0]*m[2][1] - m[1][1]*m[2][0]) / d
	r[2][1] = (m[0][1]*m[2][0] - m[0][0]*m[2][1]) / d
	r[2][2] = (m[0][0]*m[1][1] - m[0][1]*m[1][0]) / d
	for i := 0; i < 3; i++ {
		r[i][3] = -(r[i][0]*m[0][3] + r[i][1]*m[1][3] + r[i][2]*m[2][3])
	}
	r[3] = [4]float64{0, 0, 0, 1}
	return r
}

func sub(a, b [3]float64) [3]float64 { return [3]float64{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func length(v [3]float64) float64    { return math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2]) }

func normalize(v [3]float64) [3]float64 {
	l := length(v)
	if l == 0 {
		return v
	}
	return [3]float64{v[0] / l, v[1] / l, v[2] / l}
}

func f32s(v [3]float64) [3]float32 { return [3]float32{float32(v[0]), float32(v[1]), float32(v[2])} }

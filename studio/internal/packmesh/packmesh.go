// Package packmesh builds the pack the game shows in its pack opening: the game's own opening model (CardPack_Mesh, a
// flat sheet that its bones fold shut and the opening animation tears open) with its outside re-textured so that it shows
// what the shelf pack (the model Studio paints) shows at the same place. Pack art then looks the same in the opening as on
// the shelf, while the game's animation still tears the pack.
//
// How: the sheet is folded shut with its bones (Fold), every triangle seen from outside in that pose is cut along the shelf
// pack's triangles as seen from the same side (front/back along z, sides along x), and each piece takes the shelf's UVs
// there (Remap). Positions, normals and bone weights of the pieces come from the opening model, so the animation is untouched.
package packmesh

import (
	"fmt"
	"math"
	"sort"
)

type Vec3 [3]float64
type Vec2 [2]float64

// Mesh is a triangle mesh in Unity mesh space (left-handed, Y up; UV origin bottom-left). Joints/Weights are per vertex
// (skinned meshes only), indices into the renderer's bones.
type Mesh struct {
	Pos     []Vec3
	Norm    []Vec3
	UV      []Vec2
	Joints  [][4]int
	Weights [][4]float64
	Tris    [][3]int
}

// Bone is one of the opening model's bones: its name, its parent's index in the same list (-1 = none) and its bind pose
// (mesh space → bone space, row-major 4×4).
type Bone struct {
	Name   string
	Parent int
	Bind   [16]float64
}

// hinges: how far each bone of the opening model turns about the vertical axis to fold the sheet shut (degrees; the flaps go
// behind the printed side). The two bends on each side make the pack's rounded edges; L_Bend3 (seam fin) and L_Bend4 (the
// fin's torn corner) stay flat, as the fin lies on the back of the closed pack.
var hinges = map[string]float64{"Root": 0, "L_Bend": -90, "L_Bend2": -90, "L_Bend3": 0, "L_Bend4": 0, "R_Bend": 90, "R_Bend2": 90}

// rigid is p' = R p + T (3×4).
type rigid [3][4]float64

func identity() rigid { return rigid{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}} }

func (a rigid) point(p Vec3) Vec3 {
	var o Vec3
	for r := range 3 {
		o[r] = a[r][0]*p[0] + a[r][1]*p[1] + a[r][2]*p[2] + a[r][3]
	}
	return o
}

func (a rigid) vector(p Vec3) Vec3 {
	var o Vec3
	for r := range 3 {
		o[r] = a[r][0]*p[0] + a[r][1]*p[1] + a[r][2]*p[2]
	}
	return o
}

// after is a then b.
func (a rigid) after(b rigid) rigid {
	var o rigid
	for r := range 3 {
		for c := range 4 {
			v := b[r][0]*a[0][c] + b[r][1]*a[1][c] + b[r][2]*a[2][c]
			if c == 3 {
				v += b[r][3]
			}
			o[r][c] = v
		}
	}
	return o
}

// turnY: a turn by deg about the vertical line through (x, z).
func turnY(x, z, deg float64) rigid {
	a := deg * math.Pi / 180
	c, s := math.Cos(a), math.Sin(a)
	r := rigid{{c, 0, s, 0}, {0, 1, 0, 0}, {-s, 0, c, 0}}
	r[0][3] = x - (c*x + s*z)
	r[2][3] = z - (-s*x + c*z)
	return r
}

// restPosition: where a bone sits in mesh space at rest (the inverse bind pose's translation: -Rᵀ t).
func restPosition(b [16]float64) Vec3 {
	var p Vec3
	for c := range 3 {
		p[c] = -(b[0*4+c]*b[3] + b[1*4+c]*b[7] + b[2*4+c]*b[11])
	}
	return p
}

// boneMoves: each bone's move from rest to the closed pack (its hinge turn after its parent's).
func boneMoves(bones []Bone) ([]rigid, error) {
	moves := make([]rigid, len(bones))
	done := make([]bool, len(bones))
	var place func(i int) error
	place = func(i int) error {
		if done[i] {
			return nil
		}
		deg, ok := hinges[bones[i].Name]
		if !ok {
			return fmt.Errorf("the opening pack has a bone %q this Studio version doesn't know", bones[i].Name)
		}
		b := bones[i].Bind
		if b[12] != 0 || b[13] != 0 || b[14] != 0 || math.Abs(b[15]-1) > 1e-6 {
			return fmt.Errorf("bone %s: unexpected bind pose", bones[i].Name)
		}
		parent := identity()
		if p := bones[i].Parent; p >= 0 {
			if err := place(p); err != nil {
				return err
			}
			parent = moves[p]
		}
		at := restPosition(b)
		moves[i] = turnY(at[0], at[2], deg).after(parent)
		done[i] = true
		return nil
	}
	for i := range bones {
		if err := place(i); err != nil {
			return nil, err
		}
	}
	return moves, nil
}

// Fold poses the opening model shut: positions and normals with every bone turned by its hinge angle. An error when the model
// has a bone the hinge table doesn't know (the game's model changed).
func Fold(m Mesh, bones []Bone) (pos, norm []Vec3, err error) {
	if len(m.Joints) != len(m.Pos) || len(m.Weights) != len(m.Pos) || len(m.Norm) != len(m.Pos) {
		return nil, nil, fmt.Errorf("the opening pack has no skin weights or normals")
	}
	moves, err := boneMoves(bones)
	if err != nil {
		return nil, nil, err
	}
	pos, norm = make([]Vec3, len(m.Pos)), make([]Vec3, len(m.Pos))
	for i, p := range m.Pos {
		var acc, nacc Vec3
		for k := range 4 {
			w := m.Weights[i][k]
			if w == 0 {
				continue
			}
			j := m.Joints[i][k]
			if j < 0 || j >= len(moves) {
				return nil, nil, fmt.Errorf("vertex %d: bone %d out of range", i, j)
			}
			q, n := moves[j].point(p), moves[j].vector(m.Norm[i])
			for c := range 3 {
				acc[c] += w * q[c]
				nacc[c] += w * n[c]
			}
		}
		pos[i], norm[i] = acc, unit(nacc)
	}
	return pos, norm, nil
}

// Unpinch keeps the crimps from passing through each other when the pack is folded shut. In the open sheet both crimps step
// back from the print by up to about 0.058, but the closed pack is only as thick as its bends (about 0.039), so the back
// crimp came out in front of the front crimp and the closed pack's crimp tips showed the back half's inside layer. Every
// vertex of the front half (its weights turn it less than 90°) is kept in front of the closed pack's middle, every vertex of
// the back half behind it, the inside layer just inside the print layer: the halves then meet in the middle. A vertex's closed
// depth is linear in its rest depth, so its rest depth is solved for; the bends (turned about 90°) step back no further than
// the flat parts' new limit at rest, so the crimp corners stay joined. Returns how many vertices moved.
func Unpinch(m *Mesh, bones []Bone) (int, error) {
	moves, err := boneMoves(bones)
	if err != nil {
		return 0, err
	}
	at := map[string]Vec3{}
	for _, b := range bones {
		at[b.Name] = restPosition(b.Bind)
	}
	a, okA := at["L_Bend"]
	b, okB := at["L_Bend2"]
	if !okA || !okB {
		return 0, fmt.Errorf("the opening pack has no L_Bend / L_Bend2 bones")
	}
	mid := -math.Abs(b[0]-a[0]) / 2 // the closed pack's middle, behind the print (+z) side
	const printGap, insideGap = 0.002, 0.001
	moved := 0
	for i, p := range m.Pos {
		var depth, slope float64 // closed depth, and how it follows the rest depth
		for k := range 4 {
			w, j := m.Weights[i][k], m.Joints[i][k]
			if w == 0 || j < 0 || j >= len(moves) {
				continue
			}
			depth += w * moves[j].point(p)[2]
			slope += w * moves[j][2][2]
		}
		gap := printGap
		if m.Norm[i][2] < 0 {
			gap = insideGap
		}
		if math.Abs(slope) < 0.3 {
			// A bend: its rest depth hardly moves its closed depth. Its crimp corners step back no further than their
			// neighbours on the flat parts, so the corners stay joined to them.
			if p[2] < mid+gap {
				m.Pos[i][2] = mid + gap
				moved++
			}
			continue
		}
		var want float64
		switch {
		case slope > 0 && depth < mid+gap: // front half behind the middle
			want = mid + gap
		case slope < 0 && depth > mid-gap: // back half in front of it
			want = mid - gap
		default:
			continue
		}
		m.Pos[i][2] += (want - depth) / slope
		moved++
	}
	return moved, nil
}

// Stats says what Remap did (for the extraction log and tests).
type Stats struct {
	Outside, Inside int // opening triangles re-textured / left as they are (inside of the pack or covered)
	Pieces          int // triangles the outside ones were cut into
	Uncovered       int // pieces outside every shelf triangle (took the nearest shelf point's UVs)
	// Area and UncoveredArea: the outside triangles' area as seen (fitted to the shelf pack) and the part of it outside every
	// shelf triangle.
	Area, UncoveredArea float64
}

// edgeBones: the opening model's bones of the pack's rounded side edges (the first bend on each side).
var edgeBones = map[string]bool{"L_Bend": true, "R_Bend": true}

// Remap returns the opening model re-textured from the shelf pack: m is the opening model at rest (what the game skins),
// pos/norm the same model folded shut (Fold), bones its bones, shelf the shelf pack, and shelfEdge tells whether a shelf UV is
// edge art (the pack's Edges face). The opening model prints on +z, the shelf pack on −z, so the folded model is turned half
// round the vertical axis first (a turn, never a mirror), then scaled to the shelf pack's box.
//
// Faces go to faces: the opening model's side edges (edgeBones) show only the shelf pack's edge art, seen from the side;
// everything else shows only the rest, seen from the front or back. The shelf pack bulges, so part of its edge art faces
// forward; the opening pack's front is flat up to its bend and would otherwise show that edge art face-on along its borders.
func Remap(m Mesh, pos, norm []Vec3, bones []Bone, shelf Mesh, shelfEdge func(Vec2) bool) (Mesh, Stats) {
	var st Stats
	turned := make([]Vec3, len(pos))
	tnorm := make([]Vec3, len(pos))
	for i := range pos {
		turned[i] = Vec3{-pos[i][0], pos[i][1], -pos[i][2]}
		tnorm[i] = Vec3{-norm[i][0], norm[i][1], -norm[i][2]}
	}
	olo, ohi := bounds(turned)
	slo, shi := bounds(shelf.Pos)
	q := make([]Vec3, len(turned)) // folded, in the shelf pack's box
	for i, p := range turned {
		for k := range 3 {
			if ohi[k]-olo[k] < 1e-9 { // flat along this axis: its middle
				q[i][k] = (slo[k] + shi[k]) / 2
				continue
			}
			q[i][k] = slo[k] + (p[k]-olo[k])/(ohi[k]-olo[k])*(shi[k]-slo[k])
		}
	}
	shelfN := make([]Vec3, len(shelf.Tris))
	for i, t := range shelf.Tris {
		shelfN[i] = faceNormal(shelf.Norm, t)
	}

	// Each view (front, back, left edge, right edge) on its own: the opening pack's part seen from it is fitted to the shelf
	// pack's part seen from it, and its triangles meet only those shelf triangles.
	type view struct {
		a   int
		s   float64
		olo Vec2 // the opening pack's triangles seen from this side (2D, projected)
		ohi Vec2
		slo Vec2 // the shelf pack's triangles facing this way
		shi Vec2
	}
	views := map[[2]int]*view{}
	key := func(a int, s float64) [2]int { return [2]int{a, int(s)} }
	viewOf := func(a int, s float64) *view {
		k := key(a, s)
		if views[k] == nil {
			inf := math.Inf(1)
			views[k] = &view{a: a, s: s, olo: Vec2{inf, inf}, ohi: Vec2{-inf, -inf}, slo: Vec2{inf, inf}, shi: Vec2{-inf, -inf}}
		}
		return views[k]
	}
	grow := func(lo, hi *Vec2, p Vec2) {
		lo[0], lo[1] = math.Min(lo[0], p[0]), math.Min(lo[1], p[1])
		hi[0], hi[1] = math.Max(hi[0], p[0]), math.Max(hi[1], p[1])
	}
	shelfView := make([][2]int, len(shelf.Tris))
	for si, sn := range shelfN {
		t := shelf.Tris[si]
		uv := Vec2{(shelf.UV[t[0]][0] + shelf.UV[t[1]][0] + shelf.UV[t[2]][0]) / 3, (shelf.UV[t[0]][1] + shelf.UV[t[1]][1] + shelf.UV[t[2]][1]) / 3}
		cx := (shelf.Pos[t[0]][0] + shelf.Pos[t[1]][0] + shelf.Pos[t[2]][0]) / 3
		a, s := faceView(sn, cx, shelfEdge(uv))
		shelfView[si] = key(a, s)
		v, u2, v2 := viewOf(a, s), (a+1)%3, (a+2)%3
		for _, i := range shelf.Tris[si] {
			grow(&v.slo, &v.shi, Vec2{shelf.Pos[i][u2], shelf.Pos[i][v2]})
		}
	}
	type seen struct {
		a    int
		s    float64
		tri2 [3]Vec2
	}
	seenAs := make([]*seen, len(m.Tris))
	for ti, t := range m.Tris {
		// The printed side only: at rest the sheet's outside faces +z and its inside layer -z. Folded shut, the crimps are
		// pinched flat and parts of the inside layer would pass as seen from outside, then show outside art once torn open.
		if faceNormal(m.Norm, t)[2] <= 0 {
			continue
		}
		a, s := faceView(faceNormal(tnorm, t), (q[t[0]][0]+q[t[1]][0]+q[t[2]][0])/3, onEdge(m, bones, t))
		if !seenFromOutside(q, m.Tris, ti, a, s) {
			continue
		}
		u, v := (a+1)%3, (a+2)%3
		tri2 := [3]Vec2{{q[t[0]][u], q[t[0]][v]}, {q[t[1]][u], q[t[1]][v]}, {q[t[2]][u], q[t[2]][v]}}
		if math.Abs(area2(tri2[:])) < 1e-10 {
			continue // seen edge-on
		}
		vw := viewOf(a, s)
		for _, p := range tri2 {
			grow(&vw.olo, &vw.ohi, p)
		}
		seenAs[ti] = &seen{a, s, tri2}
	}
	fit := func(vw *view, p Vec2) Vec2 {
		var o Vec2
		for k := range 2 {
			if vw.ohi[k]-vw.olo[k] < 1e-12 || vw.shi[k] < vw.slo[k] {
				o[k] = p[k]
				continue
			}
			o[k] = vw.slo[k] + (p[k]-vw.olo[k])/(vw.ohi[k]-vw.olo[k])*(vw.shi[k]-vw.slo[k])
		}
		return o
	}

	out := newBuilder(m)
	for ti, t := range m.Tris {
		sa := seenAs[ti]
		if sa == nil {
			out.keep(t)
			st.Inside++
			continue
		}
		st.Outside++
		a, s := sa.a, sa.s
		vw := views[key(a, s)]
		u, v := (a+1)%3, (a+2)%3
		var tri2 [3]Vec2
		for k := range 3 {
			tri2[k] = fit(vw, sa.tri2[k])
		}
		st.Area += math.Abs(area2(tri2[:]))
		// Shelf triangles facing the same way, the outermost first: they cover what lies behind them.
		type cand struct {
			i     int
			depth float64
			poly  []Vec2
		}
		var cands []cand
		lo2, hi2 := bounds2(tri2[:])
		for si := range shelfN {
			if shelfView[si] != key(a, s) {
				continue
			}
			st3 := shelf.Tris[si]
			p := []Vec2{{shelf.Pos[st3[0]][u], shelf.Pos[st3[0]][v]}, {shelf.Pos[st3[1]][u], shelf.Pos[st3[1]][v]}, {shelf.Pos[st3[2]][u], shelf.Pos[st3[2]][v]}}
			l, h := bounds2(p)
			if h[0] < lo2[0] || l[0] > hi2[0] || h[1] < lo2[1] || l[1] > hi2[1] || math.Abs(area2(p)) < 1e-12 {
				continue
			}
			d := s * (shelf.Pos[st3[0]][a] + shelf.Pos[st3[1]][a] + shelf.Pos[st3[2]][a]) / 3
			cands = append(cands, cand{si, d, ccw(p)})
		}
		sort.SliceStable(cands, func(i, j int) bool { return cands[i].depth > cands[j].depth })
		rest := [][]Vec2{tri2[:]}
		for _, c := range cands {
			if len(rest) == 0 {
				break
			}
			var next [][]Vec2
			for _, r := range rest {
				in, outside := split(r, c.poly)
				if len(in) >= 3 && math.Abs(area2(in)) > 1e-12 {
					st.Pieces += out.piece(t, tri2, in, func(p Vec2) Vec2 { return shelfUV(shelf, c.i, u, v, p) })
				}
				next = append(next, outside...)
			}
			rest = next
		}
		for _, r := range rest { // outside every shelf triangle: the nearest shelf point's UVs
			if math.Abs(area2(r)) < 1e-12 {
				continue
			}
			st.Uncovered++
			st.UncoveredArea += math.Abs(area2(r))
			want := key(a, s)
			st.Pieces += out.piece(t, tri2, r, func(p Vec2) Vec2 { return nearestUV(shelf, shelfView, want, u, v, p) })
		}
	}
	return out.mesh(), st
}

// faceView: the side a triangle is seen from: edges along x (s = the side it is on: its normal, or where it lies when the
// normal hardly points sideways), everything else along z (front or back by its normal). s is +1 or -1.
func faceView(n Vec3, x float64, edge bool) (axis int, sign float64) {
	if edge {
		if math.Abs(n[0]) > 0.05 {
			return 0, math.Copysign(1, n[0])
		}
		return 0, math.Copysign(1, x)
	}
	return 2, math.Copysign(1, n[2])
}

// onEdge: the triangle hangs mostly on an edge bone (its corners' weights added up).
func onEdge(m Mesh, bones []Bone, t [3]int) bool {
	sum := map[int]float64{}
	for _, i := range t {
		for k := range 4 {
			sum[m.Joints[i][k]] += m.Weights[i][k]
		}
	}
	best, bw := -1, 0.0
	for j, w := range sum {
		if w > bw || w == bw && j < best {
			best, bw = j, w
		}
	}
	return best >= 0 && best < len(bones) && edgeBones[bones[best].Name]
}

// seenFromOutside: nothing of the folded pack lies in front of the triangle's middle along its view ray.
func seenFromOutside(q []Vec3, tris [][3]int, self, a int, s float64) bool {
	t := tris[self]
	var c Vec3
	for k := range 3 {
		c[k] = (q[t[0]][k] + q[t[1]][k] + q[t[2]][k]) / 3
	}
	u, v := (a+1)%3, (a+2)%3
	for i, o := range tris {
		if i == self {
			continue
		}
		A, B, C := q[o[0]], q[o[1]], q[o[2]]
		w, ok := bary2(Vec2{A[u], A[v]}, Vec2{B[u], B[v]}, Vec2{C[u], C[v]}, Vec2{c[u], c[v]})
		if !ok || w[0] < 1e-4 || w[1] < 1e-4 || w[2] < 1e-4 {
			continue
		}
		if s*(w[0]*A[a]+w[1]*B[a]+w[2]*C[a]-c[a]) > 0.002 {
			return false
		}
	}
	return true
}

// shelfUV: the shelf pack's UV at 2D point p inside its triangle si (projected on axes u, v).
func shelfUV(shelf Mesh, si, u, v int, p Vec2) Vec2 {
	t := shelf.Tris[si]
	A, B, C := shelf.Pos[t[0]], shelf.Pos[t[1]], shelf.Pos[t[2]]
	w, _ := bary2(Vec2{A[u], A[v]}, Vec2{B[u], B[v]}, Vec2{C[u], C[v]}, p)
	return Vec2{w[0]*shelf.UV[t[0]][0] + w[1]*shelf.UV[t[1]][0] + w[2]*shelf.UV[t[2]][0], w[0]*shelf.UV[t[0]][1] + w[1]*shelf.UV[t[1]][1] + w[2]*shelf.UV[t[2]][1]}
}

// nearestUV: the UV at the nearest point of the shelf triangles seen from the same side (2D, projected).
func nearestUV(shelf Mesh, shelfView [][2]int, want [2]int, u, v int, p Vec2) Vec2 {
	best, bd := Vec2{}, math.Inf(1)
	for si := range shelf.Tris {
		if shelfView[si] != want {
			continue
		}
		t := shelf.Tris[si]
		A, B, C := shelf.Pos[t[0]], shelf.Pos[t[1]], shelf.Pos[t[2]]
		cp, w := closest2(Vec2{A[u], A[v]}, Vec2{B[u], B[v]}, Vec2{C[u], C[v]}, p)
		if d := math.Hypot(cp[0]-p[0], cp[1]-p[1]); d < bd {
			bd = d
			best = Vec2{w[0]*shelf.UV[t[0]][0] + w[1]*shelf.UV[t[1]][0] + w[2]*shelf.UV[t[2]][0], w[0]*shelf.UV[t[0]][1] + w[1]*shelf.UV[t[1]][1] + w[2]*shelf.UV[t[2]][1]}
		}
	}
	return best
}

// ---- output

type builder struct {
	src  Mesh
	out  Mesh
	kept map[int]int      // source vertex → output vertex (triangles left as they are)
	made map[[6]int64]int // new vertex (position + UV, rounded) → output vertex
}

func newBuilder(m Mesh) *builder {
	return &builder{src: m, kept: map[int]int{}, made: map[[6]int64]int{}}
}

func (b *builder) add(p, n Vec3, uv Vec2, j [4]int, w [4]float64) int {
	b.out.Pos = append(b.out.Pos, p)
	b.out.Norm = append(b.out.Norm, n)
	b.out.UV = append(b.out.UV, uv)
	b.out.Joints = append(b.out.Joints, j)
	b.out.Weights = append(b.out.Weights, w)
	return len(b.out.Pos) - 1
}

func (b *builder) keep(t [3]int) {
	var o [3]int
	for k, i := range t {
		if at, ok := b.kept[i]; ok {
			o[k] = at
			continue
		}
		at := b.add(b.src.Pos[i], b.src.Norm[i], b.src.UV[i], b.src.Joints[i], b.src.Weights[i])
		b.kept[i] = at
		o[k] = at
	}
	b.out.Tris = append(b.out.Tris, o)
}

// piece adds polygon poly (2D, inside source triangle t whose 2D corners are tri2) as a fan of triangles with UVs from uvAt;
// it returns how many triangles it added. The source triangle's winding is kept.
func (b *builder) piece(t [3]int, tri2 [3]Vec2, poly []Vec2, uvAt func(Vec2) Vec2) int {
	if (area2(poly) > 0) != (area2(tri2[:]) > 0) {
		poly = reversed(poly)
	}
	idx := make([]int, len(poly))
	for k, p := range poly {
		w, _ := bary2(tri2[0], tri2[1], tri2[2], p)
		var pos, nrm Vec3
		for c := range 3 {
			pos[c] = w[0]*b.src.Pos[t[0]][c] + w[1]*b.src.Pos[t[1]][c] + w[2]*b.src.Pos[t[2]][c]
			nrm[c] = w[0]*b.src.Norm[t[0]][c] + w[1]*b.src.Norm[t[1]][c] + w[2]*b.src.Norm[t[2]][c]
		}
		j, wt := blendWeights(b.src, t, w)
		uv := uvAt(p)
		key := [6]int64{round(pos[0]), round(pos[1]), round(pos[2]), round(uv[0]), round(uv[1]), 0}
		if at, ok := b.made[key]; ok {
			idx[k] = at
			continue
		}
		at := b.add(pos, unit(nrm), uv, j, wt)
		b.made[key] = at
		idx[k] = at
	}
	n := 0
	for k := 1; k+1 < len(idx); k++ {
		if idx[0] == idx[k] || idx[k] == idx[k+1] || idx[0] == idx[k+1] {
			continue
		}
		b.out.Tris = append(b.out.Tris, [3]int{idx[0], idx[k], idx[k+1]})
		n++
	}
	return n
}

func (b *builder) mesh() Mesh { return b.out }

func round(f float64) int64 { return int64(math.Round(f * 1e6)) }

// blendWeights: the corners' bone weights mixed by w, the 4 strongest kept and normalised.
func blendWeights(m Mesh, t [3]int, w [3]float64) ([4]int, [4]float64) {
	sum := map[int]float64{}
	for k := range 3 {
		for c := range 4 {
			if m.Weights[t[k]][c] > 0 {
				sum[m.Joints[t[k]][c]] += w[k] * m.Weights[t[k]][c]
			}
		}
	}
	type bw struct {
		j int
		w float64
	}
	list := make([]bw, 0, len(sum))
	for j, x := range sum {
		if x > 1e-6 {
			list = append(list, bw{j, x})
		}
	}
	sort.Slice(list, func(a, b int) bool {
		if list[a].w != list[b].w {
			return list[a].w > list[b].w
		}
		return list[a].j < list[b].j
	})
	var j [4]int
	var wt [4]float64
	total := 0.0
	for k := 0; k < 4 && k < len(list); k++ {
		j[k], wt[k] = list[k].j, list[k].w
		total += list[k].w
	}
	for k := range 4 {
		if total > 0 {
			wt[k] /= total
		}
	}
	return j, wt
}

// ---- 2D geometry

func area2(p []Vec2) float64 {
	a := 0.0
	for i := range p {
		q, r := p[i], p[(i+1)%len(p)]
		a += q[0]*r[1] - r[0]*q[1]
	}
	return a / 2
}

func reversed(p []Vec2) []Vec2 {
	o := make([]Vec2, len(p))
	for i := range p {
		o[i] = p[len(p)-1-i]
	}
	return o
}

func ccw(p []Vec2) []Vec2 {
	if area2(p) < 0 {
		return reversed(p)
	}
	return p
}

func bounds2(p []Vec2) (lo, hi Vec2) {
	lo, hi = Vec2{math.Inf(1), math.Inf(1)}, Vec2{math.Inf(-1), math.Inf(-1)}
	for _, q := range p {
		lo[0], lo[1] = math.Min(lo[0], q[0]), math.Min(lo[1], q[1])
		hi[0], hi[1] = math.Max(hi[0], q[0]), math.Max(hi[1], q[1])
	}
	return
}

// split cuts convex polygon r by convex CCW polygon c: the part inside c, and the parts outside it (convex pieces).
func split(r, c []Vec2) (in []Vec2, outside [][]Vec2) {
	cur := r
	for i := range c {
		a, b := c[i], c[(i+1)%len(c)]
		inside, out := halfPlanes(cur, a, b)
		if len(out) >= 3 && math.Abs(area2(out)) > 1e-14 {
			outside = append(outside, out)
		}
		cur = inside
		if len(cur) < 3 { // nothing of r inside c: the pieces cut off so far are all of r
			if len(outside) == 0 {
				outside = [][]Vec2{r}
			}
			return nil, outside
		}
	}
	return cur, outside
}

// halfPlanes clips p by the line a→b: the part on its left (inside a CCW polygon) and the part on its right.
func halfPlanes(p []Vec2, a, b Vec2) (left, right []Vec2) {
	side := func(q Vec2) float64 { return (b[0]-a[0])*(q[1]-a[1]) - (b[1]-a[1])*(q[0]-a[0]) }
	for i := range p {
		q, r := p[i], p[(i+1)%len(p)]
		sq, sr := side(q), side(r)
		if sq >= 0 {
			left = append(left, q)
		}
		if sq <= 0 {
			right = append(right, q)
		}
		if (sq > 0 && sr < 0) || (sq < 0 && sr > 0) {
			t := sq / (sq - sr)
			x := Vec2{q[0] + t*(r[0]-q[0]), q[1] + t*(r[1]-q[1])}
			left = append(left, x)
			right = append(right, x)
		}
	}
	return left, right
}

func bary2(a, b, c, p Vec2) ([3]float64, bool) {
	den := (b[1]-c[1])*(a[0]-c[0]) + (c[0]-b[0])*(a[1]-c[1])
	if math.Abs(den) < 1e-14 {
		return [3]float64{}, false
	}
	w0 := ((b[1]-c[1])*(p[0]-c[0]) + (c[0]-b[0])*(p[1]-c[1])) / den
	w1 := ((c[1]-a[1])*(p[0]-c[0]) + (a[0]-c[0])*(p[1]-c[1])) / den
	return [3]float64{w0, w1, 1 - w0 - w1}, true
}

// closest2: the closest point of triangle abc to p, with its barycentric weights.
func closest2(a, b, c, p Vec2) (Vec2, [3]float64) {
	if w, ok := bary2(a, b, c, p); ok && w[0] >= 0 && w[1] >= 0 && w[2] >= 0 {
		return p, w
	}
	best, bw, bd := Vec2{}, [3]float64{}, math.Inf(1)
	edge := func(x, y Vec2, ix, iy int) {
		dx, dy := y[0]-x[0], y[1]-x[1]
		l := dx*dx + dy*dy
		t := 0.0
		if l > 0 {
			t = math.Max(0, math.Min(1, ((p[0]-x[0])*dx+(p[1]-x[1])*dy)/l))
		}
		q := Vec2{x[0] + t*dx, x[1] + t*dy}
		if d := math.Hypot(q[0]-p[0], q[1]-p[1]); d < bd {
			bd, best = d, q
			bw = [3]float64{}
			bw[ix], bw[iy] = 1-t, t
		}
	}
	edge(a, b, 0, 1)
	edge(b, c, 1, 2)
	edge(c, a, 2, 0)
	return best, bw
}

// ---- 3D helpers

func bounds(ps []Vec3) (lo, hi Vec3) {
	lo, hi = Vec3{math.Inf(1), math.Inf(1), math.Inf(1)}, Vec3{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, p := range ps {
		for k := range 3 {
			lo[k], hi[k] = math.Min(lo[k], p[k]), math.Max(hi[k], p[k])
		}
	}
	return
}

// faceNormal: the triangle's normal from its corners' normals (the meshes' own shading normals, so the side is right
// whichever way the triangle is wound).
func faceNormal(norm []Vec3, t [3]int) Vec3 {
	var n Vec3
	for _, i := range t {
		for c := range 3 {
			n[c] += norm[i][c]
		}
	}
	return unit(n)
}

func unit(v Vec3) Vec3 {
	l := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	if l == 0 {
		return v
	}
	return Vec3{v[0] / l, v[1] / l, v[2] / l}
}

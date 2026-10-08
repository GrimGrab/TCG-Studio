package gameextract

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/unityfs"
	"tcgstudio/internal/uvmap"
)

// Paint templates (furniture/<base>_paint.*): every piece unfolded for the face editor, whatever its game texture (a real
// unwrap, a two-colour strip or plain material colours). The body renderers get new UVs into one atlas:
//   - triangles are grouped into charts (connected surfaces facing the same way: front/back/left/right/top/bottom, front = +z,
//     the aisle side customers stand on), each projected flat onto its view plane and packed into the atlas;
//   - the vanilla look (texture × material colour) is baked into the atlas (<base>_paint.png);
//   - the editor model (uvmap.Model, Projected) is the six views laid out like a box net; <base>_paint.bin is every body triangle
//     with its atlas UV and its place in that net, so the editor's GPU painter (frontend lib/projectPaint.ts) draws an image placed on
//     "Front" onto everything seen from the front — in the 3D preview, the net view and the saved atlas;
//   - <base>_paint_<n>.obj are the body renderers' meshes with the new UVs in their own mesh space (the mod swaps them in,
//     Runtime/FurniturePaint).

// PaintSize is the atlas size (pixels).
const PaintSize = 2048

// paintVersion changes whenever the paint template output changes (cached templates are then rebuilt).
const paintVersion = 8

// PaintTemplate makes the paint template of one furniture piece (base = EObjectType name) in <templatesDir>\furniture, on
// demand (only pieces someone paints; kept until the next extraction replaces the folder), and returns its JSON file name.
func PaintTemplate(gameDir, templatesDir, base string) (string, error) {
	file := safe(base) + "_paint.json"
	dir := filepath.Join(templatesDir, FurnitureDir)
	var have struct{ Version int }
	if b, err := os.ReadFile(filepath.Join(dir, file)); err == nil && json.Unmarshal(b, &have) == nil && have.Version == paintVersion {
		return file, nil
	}
	data, err := DataDir(gameDir)
	if err != nil {
		return "", err
	}
	enums, err := unityfs.ReadEnums(filepath.Join(data, "Managed", "Assembly-CSharp.dll"), "EObjectType")
	if err != nil {
		return "", err
	}
	env, err := unityfs.Open(data)
	if err != nil {
		return "", err
	}
	defer env.Close()
	x := &extractor{env: env, dir: templatesDir, objNames: invert(enums["EObjectType"]), texNames: map[*unityfs.Object]unityfs.Texture2D{}}
	sd, err := x.shelfData()
	if err != nil {
		return "", err
	}
	for _, od := range sd.Objects {
		if x.objNames[od.ObjectType] != base {
			continue
		}
		po, _ := env.Resolve(sd.File, od.Prefab)
		if po == nil {
			break
		}
		io, err := unityfs.ReadInteractableObject(po)
		if err != nil {
			return "", err
		}
		g := newGraph(env, po.File)
		root := g.nodeOf(io.GameObject)
		if root == nil {
			break
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		// Working parts (cash drawer with its notes, card machine, signs, the prize shelf's TV) keep their own look: not painted.
		look, parts := lookSkip(g, io), map[*node]bool{}
		class := env.ScriptClass(po)
		if rps, err := x.rolePoints(g, po, class, furnitureTypes[class]); err == nil {
			for _, rp := range rps {
				if rp.role.Kind == "part" {
					parts[rp.n] = true
				}
			}
		}
		skip := func(n *node) bool {
			for p := n; p != nil; p = p.parent {
				if parts[p] {
					return true
				}
			}
			return look(n)
		}
		if !x.paintTemplate(g, root, base, root.world().inverse(), x.newMeshSet(""), skip, dir) {
			return "", fmt.Errorf("%s can't be painted: %s", base, strings.Join(x.warnings, "; "))
		}
		return file, nil
	}
	return "", fmt.Errorf("furniture piece %s wasn't found in the game files", base)
}

// paintTemplate is <base>_paint.json.
type paintTemplate struct {
	Version int                `json:"version"`
	Base    string             `json:"base"`
	Vanilla string             `json:"vanilla"` // baked vanilla look
	Preview string             `json:"preview"` // <base>_paint.bin (see paintPreview)
	Parts   []setfmt.PaintPart `json:"parts"`   // meshes = template file names (Studio stores them with the piece)
	Model   uvmap.Model        `json:"model"`
}

// Groups the game fills or swaps at runtime: never repainted (their UVs pick textures/digits).
var paintSkipNames = []string{"tablegameitemset", "playtablenumbergrp"}

type paintMat struct {
	img    *image.NRGBA // nil = colour only
	scale  [2]float64
	offset [2]float64
	color  [4]float64
}

type paintRenderer struct {
	n    *node // nil for an own model
	m    *unityfs.Mesh
	xf   mat // mesh space → root space
	mats []*paintMat
}

type paintTri struct {
	r, sub int
	v      [3]uint32     // vertex indices in the renderer's mesh
	p      [3][3]float64 // root space
	view   int
	chart  int
	uv     [3][2]float64 // atlas pixels (top-left origin)
}

// The six views: outward normal, image-up, label. Image-right = up × (−normal) (Unity is left-handed).
var paintViews = []struct {
	n, up [3]float64
	label string
}{
	{[3]float64{0, 0, 1}, [3]float64{0, 1, 0}, "Front"},
	{[3]float64{0, 0, -1}, [3]float64{0, 1, 0}, "Back"},
	{[3]float64{1, 0, 0}, [3]float64{0, 1, 0}, "Left"},
	{[3]float64{-1, 0, 0}, [3]float64{0, 1, 0}, "Right"},
	{[3]float64{0, 1, 0}, [3]float64{0, 0, -1}, "Top"},
	{[3]float64{0, -1, 0}, [3]float64{0, 0, 1}, "Bottom"},
}

func cross3(a, b [3]float64) [3]float64 {
	return [3]float64{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}
func dot3(a, b [3]float64) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

func viewAxes(v int) (right, up [3]float64) {
	up = paintViews[v].up
	n := paintViews[v].n
	return cross3(up, [3]float64{-n[0], -n[1], -n[2]}), up
}

// chart: surfaces of one view sharing one atlas rectangle — its bounding box in view space (metres: u right, v up), mapped
// linearly into the atlas.
type chart struct {
	view           int
	u0, v0, u1, v1 float64
	fill           float64 // area its surfaces' own boxes cover (≤ its box area)
	tris           []int
	x, y, w, h     int // atlas rect (pixels, without padding)
}

// area of the chart's box; edge-on parts count as 2 mm thick.
func (c *chart) area() float64 { return math.Max(c.u1-c.u0, 0.002) * math.Max(c.v1-c.v0, 0.002) }

// mergeCharts joins charts of the same view into one atlas rectangle when their boxes don't overlap (so no two surfaces read the
// same texels) and the joined box wastes little room (≤ 25 % + 1 cm² empty) — e.g. post segments between shelf boards, board edges
// in a row. Fewer, larger faces: faster editing and less atlas padding. Repeats until nothing merges.
func mergeCharts(cs []*chart) []*chart {
	const eps = 1e-5
	overlap := func(a, b *chart) bool {
		return a.u0 < b.u1-eps && b.u0 < a.u1-eps && a.v0 < b.v1-eps && b.v0 < a.v1-eps
	}
	for changed := true; changed; {
		changed = false
		for i := 0; i < len(cs); i++ {
			a := cs[i]
			for j := i + 1; j < len(cs); j++ {
				b := cs[j]
				if a.view != b.view || overlap(a, b) {
					continue
				}
				u := chart{view: a.view, u0: math.Min(a.u0, b.u0), v0: math.Min(a.v0, b.v0), u1: math.Max(a.u1, b.u1), v1: math.Max(a.v1, b.v1)}
				if fill := a.fill + b.fill; u.area() > fill*1.25+1e-4 {
					continue
				}
				a.u0, a.v0, a.u1, a.v1 = u.u0, u.v0, u.u1, u.v1
				a.fill += b.fill
				a.tris = append(a.tris, b.tris...)
				cs[j] = cs[len(cs)-1]
				cs = cs[:len(cs)-1]
				j = i // a grew: check every other chart again
				changed = true
			}
		}
	}
	return cs
}

// paintTemplate builds and writes a piece's paint template; false when it has no paintable body.
func (x *extractor) paintTemplate(g *graph, root *node, name string, inv mat, meshes *meshSet, skip func(*node) bool, dir string) bool {
	rs := x.paintRenderers(g, root, inv, meshes, skip)
	return x.buildPaint(name, rs, func(r paintRenderer) string { return indexPath(r.n, root) }, dir)
}

// PaintTemplateFromMesh makes the paint template of a piece's own model (Unity root space, UV origin bottom-left, one texture) as
// <dir>\furniture\<name>_paint.*: its one part (renderer "") is the model with the painting UVs, for the furniture's mesh field.
// Kept until the folder is replaced; returns the JSON file name.
func PaintTemplateFromMesh(templatesDir, name string, m *unityfs.Mesh, tex *image.NRGBA) (string, error) {
	file := safe(name) + "_paint.json"
	dir := filepath.Join(templatesDir, FurnitureDir)
	var have struct{ Version int }
	if b, err := os.ReadFile(filepath.Join(dir, file)); err == nil && json.Unmarshal(b, &have) == nil && have.Version == paintVersion {
		return file, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	x := &extractor{}
	pm := &paintMat{img: tex, scale: [2]float64{1, 1}, color: [4]float64{1, 1, 1, 1}}
	if tex == nil {
		pm.color = [4]float64{0.8, 0.8, 0.8, 1}
	}
	rs := []paintRenderer{{m: m, xf: identity(), mats: []*paintMat{pm}}}
	if !x.buildPaint(name, rs, func(paintRenderer) string { return "" }, dir) {
		return "", fmt.Errorf("the model can't be painted: %s", strings.Join(x.warnings, "; "))
	}
	return file, nil
}

// buildPaint makes and writes the paint template of the given parts (rendererPath names a part for the mod).
func (x *extractor) buildPaint(name string, rs []paintRenderer, rendererPath func(paintRenderer) string, dir string) bool {
	if len(rs) == 0 {
		x.warn("furniture %s: no paintable parts", name)
		return false
	}
	// Triangles with their view (dominant direction of the outward normal).
	var tris []paintTri
	for ri, r := range rs {
		mirrored := r.xf.det3() < 0
		hasN := len(r.m.Normal) == len(r.m.Pos)
		for si, sub := range r.m.Subs {
			for i := 0; i+2 < len(sub); i += 3 {
				t := paintTri{r: ri, sub: si, v: [3]uint32{sub[i], sub[i+1], sub[i+2]}}
				for k := 0; k < 3; k++ {
					t.p[k] = r.xf.point(f64(r.m.Pos[t.v[k]]))
				}
				nrm := cross3(sub3(t.p[1], t.p[0]), sub3(t.p[2], t.p[0]))
				if hasN { // trust the stored normals for the side
					var avg [3]float64
					for k := 0; k < 3; k++ {
						nv := r.xf.vector(f64(r.m.Normal[t.v[k]]))
						for c := 0; c < 3; c++ {
							avg[c] += nv[c]
						}
					}
					if dot3(avg, nrm) < 0 {
						nrm = [3]float64{-nrm[0], -nrm[1], -nrm[2]}
					}
				} else if mirrored {
					nrm = [3]float64{-nrm[0], -nrm[1], -nrm[2]}
				}
				best, nn := -2.0, normalize(nrm)
				for v := range paintViews {
					if d := dot3(nn, paintViews[v].n); d > best {
						best, t.view = d, v
					}
				}
				tris = append(tris, t)
			}
		}
	}
	charts := paintCharts(tris)

	// Chart rectangles in view space (metres): u right, v up.
	cs := make([]*chart, len(charts))
	for ci, ts := range charts {
		c := &chart{view: tris[ts[0]].view, u0: math.Inf(1), v0: math.Inf(1), u1: math.Inf(-1), v1: math.Inf(-1), tris: ts}
		right, up := viewAxes(c.view)
		for _, ti := range ts {
			for _, p := range tris[ti].p {
				u, v := dot3(p, right), dot3(p, up)
				c.u0, c.u1, c.v0, c.v1 = math.Min(c.u0, u), math.Max(c.u1, u), math.Min(c.v0, v), math.Max(c.v1, v)
			}
		}
		c.fill = c.area()
		cs[ci] = c
	}
	cs = mergeCharts(cs)
	for ci, c := range cs {
		for _, ti := range c.tris {
			tris[ti].chart = ci
		}
	}

	// Pack: the largest scale (px per metre) at which every chart fits, shelf packing tallest first.
	const pad = 4
	order := make([]*chart, len(cs))
	copy(order, cs)
	sort.Slice(order, func(i, j int) bool { return order[i].v1-order[i].v0 > order[j].v1-order[j].v0 })
	pack := func(s float64) bool {
		x0, y0, rowH := pad, pad, 0
		for _, c := range order {
			w, h := max(1, int(math.Ceil((c.u1-c.u0)*s))), max(1, int(math.Ceil((c.v1-c.v0)*s)))
			if x0+w+pad > PaintSize {
				x0, y0, rowH = pad, y0+rowH+2*pad, 0
			}
			if w+2*pad > PaintSize || y0+h+pad > PaintSize {
				return false
			}
			c.x, c.y, c.w, c.h = x0, y0, w, h
			x0 += w + 2*pad
			rowH = max(rowH, h)
		}
		return true
	}
	lo, hi := 1.0, 8192.0
	for i := 0; i < 30; i++ {
		if mid := (lo + hi) / 2; pack(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	scale := lo
	if !pack(scale) {
		x.warn("furniture %s: paint atlas doesn't fit", name)
		return false
	}

	// Atlas UVs of every triangle corner.
	for i := range tris {
		t := &tris[i]
		c := cs[t.chart]
		right, up := viewAxes(c.view)
		sx, sy := float64(c.w)/math.Max(c.u1-c.u0, 1e-9), float64(c.h)/math.Max(c.v1-c.v0, 1e-9)
		for k := 0; k < 3; k++ {
			u, v := dot3(t.p[k], right), dot3(t.p[k], up)
			t.uv[k] = [2]float64{float64(c.x) + (u-c.u0)*sx, float64(c.y) + (c.v1-v)*sy}
		}
	}

	// Editor model: views laid out like a box net (Top over Front, Bottom under it; Left | Front | Right | Back).
	var bmin, bmax [3]float64
	for k := 0; k < 3; k++ {
		bmin[k], bmax[k] = math.Inf(1), math.Inf(-1)
	}
	for _, t := range tris {
		for _, p := range t.p {
			for k := 0; k < 3; k++ {
				bmin[k], bmax[k] = math.Min(bmin[k], p[k]), math.Max(bmax[k], p[k])
			}
		}
	}
	vb := make([]viewBox, len(paintViews))
	for v := range paintViews {
		right, up := viewAxes(v)
		b := viewBox{u0: math.Inf(1), v0: math.Inf(1), u1: math.Inf(-1), v1: math.Inf(-1), d0: math.Inf(1), d1: math.Inf(-1)}
		for i := 0; i < 8; i++ {
			p := [3]float64{pick(i&1, bmin[0], bmax[0]), pick(i&2, bmin[1], bmax[1]), pick(i&4, bmin[2], bmax[2])}
			u, w, d := dot3(p, right), dot3(p, up), dot3(p, paintViews[v].n)
			b.u0, b.u1, b.v0, b.v1 = math.Min(b.u0, u), math.Max(b.u1, u), math.Min(b.v0, w), math.Max(b.v1, w)
			b.d0, b.d1 = math.Min(b.d0, d), math.Max(b.d1, d)
		}
		vb[v] = b
	}
	gap := 0.08 * math.Max(bmax[0]-bmin[0], math.Max(bmax[1]-bmin[1], bmax[2]-bmin[2]))
	W, H, D := bmax[0]-bmin[0], bmax[1]-bmin[1], bmax[2]-bmin[2]
	left := D + gap
	vb[2].x, vb[2].y = 0, D+gap              // Left
	vb[0].x, vb[0].y = left, D+gap           // Front
	vb[3].x, vb[3].y = left+W+gap, D+gap     // Right
	vb[1].x, vb[1].y = left+W+D+2*gap, D+gap // Back
	vb[4].x, vb[4].y = left, 0               // Top
	vb[5].x, vb[5].y = left, D+gap+H+gap     // Bottom

	base := safe(name) + "_paint"
	model := uvmap.Model{Kind: "Furniture", Mesh: name + "_paint", TextureSize: PaintSize, Size: [3]float64{W, H, D}, Icon: "mesh",
		Projected: true, Turn: math.Pi, Density: scale, Faces: []uvmap.Face{},
		Preview: []uvmap.PreviewPart{{Mesh: name + "_paint", Texture: "main", URL: "/furntemplates/" + base + ".bin"}}}
	used := map[int]bool{}
	for _, c := range cs {
		used[c.view] = true
	}
	for v, view := range paintViews {
		if used[v] {
			b := vb[v]
			model.Views = append(model.Views, uvmap.View{Label: view.label, Net: [4]float64{b.x, b.y, b.u1 - b.u0, b.v1 - b.v0}})
		}
	}

	// Bake the vanilla look, then the meshes.
	atlas := x.bakePaint(rs, tris, pad)
	if err := savePNG(filepath.Join(dir, base+".png"), atlas); err != nil {
		x.warn("furniture %s: %v", name, err)
		return false
	}
	tpl := paintTemplate{Version: paintVersion, Base: name, Vanilla: base + ".png", Preview: base + ".bin", Model: model}
	for ri, r := range rs {
		file := fmt.Sprintf("%s_%d.obj", base, ri)
		if err := os.WriteFile(filepath.Join(dir, file), []byte(paintOBJ(r, tris, ri)), 0o644); err != nil {
			x.warn("furniture %s: %v", name, err)
			return false
		}
		tpl.Parts = append(tpl.Parts, setfmt.PaintPart{Renderer: rendererPath(r), Mesh: file})
	}
	if err := os.WriteFile(filepath.Join(dir, base+".bin"), paintPreview(rs, tris, cs, vb), 0o644); err != nil {
		x.warn("furniture %s: %v", name, err)
		return false
	}
	if err := writeJSON(filepath.Join(dir, base+".json"), tpl); err != nil {
		x.warn("furniture %s: %v", name, err)
		return false
	}
	return true
}

// viewBox: a view's extent (view space: u right, v up, d towards the viewer) and where it sits in the net (x, y: top-left).
type viewBox struct{ u0, v0, u1, v1, d0, d1, x, y float64 }

// paintPreview is <base>_paint.bin for the editor's GPU painter (frontend lib/projectPaint.ts), little-endian:
// "TCGP", u32 version (1), u32 vertex count, u32 index count, then per vertex 11 float32 — position xyz and normal xyz (piece root,
// Unity space), atlas uv (origin bottom-left), net xy (net units, y down), depth (0 = nearest to its view's viewer, 1 = farthest) —
// then u32 triangle indices.
func paintPreview(rs []paintRenderer, tris []paintTri, cs []*chart, vb []viewBox) []byte {
	type vk struct {
		r     int
		v     uint32
		chart int
	}
	index := map[vk]uint32{}
	var verts []float32
	var idx []uint32
	for _, t := range tris {
		r := rs[t.r]
		c := cs[t.chart]
		b := vb[c.view]
		right, up := viewAxes(c.view)
		order := [3]int{0, 1, 2}
		if r.xf.det3() < 0 { // a mirrored part: the same front side in root space (the editor culls back faces like the game)
			order = [3]int{0, 2, 1}
		}
		for _, k := range order {
			key := vk{t.r, t.v[k], t.chart}
			id, ok := index[key]
			if !ok {
				id = uint32(len(verts) / 11)
				index[key] = id
				p := t.p[k]
				n := [3]float64{0, 1, 0}
				if len(r.m.Normal) == len(r.m.Pos) {
					n = normalize(r.xf.vector(f64(r.m.Normal[t.v[k]])))
				}
				u, w, d := dot3(p, right), dot3(p, up), dot3(p, paintViews[c.view].n)
				depth := (b.d1 - d) / math.Max(b.d1-b.d0, 1e-9)
				verts = append(verts, float32(p[0]), float32(p[1]), float32(p[2]), float32(n[0]), float32(n[1]), float32(n[2]),
					float32(t.uv[k][0]/PaintSize), float32(1-t.uv[k][1]/PaintSize),
					float32(b.x+(u-b.u0)), float32(b.y+(b.v1-w)), float32(depth))
			}
			idx = append(idx, id)
		}
	}
	out := make([]byte, 16+4*len(verts)+4*len(idx))
	copy(out, "TCGP")
	binary.LittleEndian.PutUint32(out[4:], 1)
	binary.LittleEndian.PutUint32(out[8:], uint32(len(verts)/11))
	binary.LittleEndian.PutUint32(out[12:], uint32(len(idx)))
	o := 16
	for _, f := range verts {
		binary.LittleEndian.PutUint32(out[o:], math.Float32bits(f))
		o += 4
	}
	for _, i := range idx {
		binary.LittleEndian.PutUint32(out[o:], i)
		o += 4
	}
	return out
}

func pick(bit int, a, b float64) float64 {
	if bit != 0 {
		return b
	}
	return a
}

func sub3(a, b [3]float64) [3]float64 { return [3]float64{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }

// paintRenderers: the body renderers (the look without spots, price tags, helpers or runtime-swapped parts), with materials.
func (x *extractor) paintRenderers(g *graph, root *node, inv mat, meshes *meshSet, skip func(*node) bool) []paintRenderer {
	var out []paintRenderer
	images := map[*unityfs.Object]*image.NRGBA{}
	root.walk(func(n *node) {
		ro := g.comp(n, unityfs.ClassMeshRenderer, "")
		mfo := g.comp(n, unityfs.ClassMeshFilter, "")
		if ro == nil || mfo == nil || skip(n) || n.hasInParent("Item", "InteractablePackagingBox", "ShelfCompartment", "InteractableCardCompartment") {
			return
		}
		for p := n; p != nil && p != root; p = p.parent {
			for _, s := range paintSkipNames {
				if strings.ToLower(p.name) == s {
					return
				}
			}
		}
		r, err := unityfs.ReadRenderer(ro)
		if err != nil || !r.Enabled {
			return
		}
		mf, err := unityfs.ReadMeshFilter(mfo)
		if err != nil {
			return
		}
		mo, _ := x.env.Resolve(g.file, mf.Mesh)
		if mo == nil || mo.ClassID != unityfs.ClassMesh {
			return
		}
		m := meshes.mesh(mo)
		if m == nil || len(m.Pos) == 0 || len(m.Subs) == 0 {
			return
		}
		pr := paintRenderer{n: n, m: m, xf: mul(inv, n.world())}
		for si := range m.Subs {
			pm := &paintMat{scale: [2]float64{1, 1}, color: [4]float64{0.6, 0.6, 0.6, 1}}
			if si < len(r.Materials) {
				if mato, mat := x.material(g.file, r.Materials[si]); mat != nil {
					c := mat.MainColor()
					pm.color = [4]float64{float64(c[0]), float64(c[1]), float64(c[2]), float64(c[3])}
					if te, ok := mat.MainTexture(); ok {
						if to, t, ok := x.texture(mato.File, te.Texture); ok {
							img, seen := images[to]
							if !seen {
								img, _ = x.env.TextureImage(t)
								images[to] = img
							}
							pm.img = img
							if te.Scale != [2]float32{} {
								pm.scale = [2]float64{float64(te.Scale[0]), float64(te.Scale[1])}
							}
							pm.offset = [2]float64{float64(te.Offset[0]), float64(te.Offset[1])}
						}
					}
				}
			}
			pr.mats = append(pr.mats, pm)
		}
		out = append(out, pr)
	})
	return out
}

// paintCharts groups triangles into charts: same view and sharing an edge (positions quantized to 0.1 mm).
func paintCharts(tris []paintTri) [][]int {
	type ekey [6]int64
	q := func(p [3]float64) [3]int64 {
		return [3]int64{int64(math.Round(p[0] * 1e4)), int64(math.Round(p[1] * 1e4)), int64(math.Round(p[2] * 1e4))}
	}
	key := func(a, b [3]float64) ekey {
		qa, qb := q(a), q(b)
		if qa[0] > qb[0] || qa[0] == qb[0] && (qa[1] > qb[1] || qa[1] == qb[1] && qa[2] > qb[2]) {
			qa, qb = qb, qa
		}
		return ekey{qa[0], qa[1], qa[2], qb[0], qb[1], qb[2]}
	}
	parent := make([]int, len(tris))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	edges := map[ekey]int{}
	for i, t := range tris {
		for k := 0; k < 3; k++ {
			e := key(t.p[k], t.p[(k+1)%3])
			if j, ok := edges[e]; ok {
				if tris[j].view == t.view {
					parent[find(i)] = find(j)
				}
			} else {
				edges[e] = i
			}
		}
	}
	groups := map[int][]int{}
	var roots []int
	for i := range tris {
		r := find(i)
		if _, ok := groups[r]; !ok {
			roots = append(roots, r)
		}
		groups[r] = append(groups[r], i)
	}
	out := make([][]int, len(roots))
	for i, r := range roots {
		out[i] = groups[r]
	}
	return out
}

// bakePaint draws the vanilla look (main texture × colour) of every triangle into the atlas, then bleeds the edges.
func (x *extractor) bakePaint(rs []paintRenderer, tris []paintTri, pad int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, PaintSize, PaintSize))
	filled := make([]bool, PaintSize*PaintSize)
	for _, t := range tris {
		r := rs[t.r]
		pm := r.mats[t.sub]
		var src [3][2]float64
		if len(r.m.UV) == len(r.m.Pos) {
			for k := 0; k < 3; k++ {
				uv := r.m.UV[t.v[k]]
				src[k] = [2]float64{float64(uv[0])*pm.scale[0] + pm.offset[0], float64(uv[1])*pm.scale[1] + pm.offset[1]}
			}
		}
		a, b, c := t.uv[0], t.uv[1], t.uv[2]
		den := (b[1]-c[1])*(a[0]-c[0]) + (c[0]-b[0])*(a[1]-c[1])
		if math.Abs(den) < 1e-12 {
			continue
		}
		x0 := int(math.Floor(math.Min(a[0], math.Min(b[0], c[0]))))
		x1 := int(math.Ceil(math.Max(a[0], math.Max(b[0], c[0]))))
		y0 := int(math.Floor(math.Min(a[1], math.Min(b[1], c[1]))))
		y1 := int(math.Ceil(math.Max(a[1], math.Max(b[1], c[1]))))
		for py := max(0, y0); py <= min(PaintSize-1, y1); py++ {
			for px := max(0, x0); px <= min(PaintSize-1, x1); px++ {
				fx, fy := float64(px)+0.5, float64(py)+0.5
				w0 := ((b[1]-c[1])*(fx-c[0]) + (c[0]-b[0])*(fy-c[1])) / den
				w1 := ((c[1]-a[1])*(fx-c[0]) + (a[0]-c[0])*(fy-c[1])) / den
				w2 := 1 - w0 - w1
				const e = -0.02 // a hair outside: no gaps on shared edges
				if w0 < e || w1 < e || w2 < e {
					continue
				}
				col := pm.color
				if pm.img != nil {
					u := w0*src[0][0] + w1*src[1][0] + w2*src[2][0]
					v := w0*src[0][1] + w1*src[1][1] + w2*src[2][1]
					s := sampleWrap(pm.img, u, v)
					for k := 0; k < 3; k++ {
						col[k] *= s[k]
					}
				}
				i := py*PaintSize + px
				filled[i] = true
				img.SetNRGBA(px, py, color.NRGBA{to8(col[0]), to8(col[1]), to8(col[2]), 255})
			}
		}
	}
	// Bleed: grow the painted areas into the padding so filtering at seams never picks up the background.
	for pass := 0; pass < pad+2; pass++ {
		var add []int
		for py := 0; py < PaintSize; py++ {
			for px := 0; px < PaintSize; px++ {
				i := py*PaintSize + px
				if filled[i] {
					continue
				}
				for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, ny := px+d[0], py+d[1]
					if nx >= 0 && ny >= 0 && nx < PaintSize && ny < PaintSize && filled[ny*PaintSize+nx] {
						img.SetNRGBA(px, py, img.NRGBAAt(nx, ny))
						add = append(add, i)
						break
					}
				}
			}
		}
		for _, i := range add {
			filled[i] = true
		}
	}
	return img
}

func to8(v float64) uint8 { return uint8(math.Max(0, math.Min(255, math.Round(v*255)))) }

// sampleWrap reads a texture at UV (origin bottom-left, repeat), as 0..1 RGB.
func sampleWrap(img *image.NRGBA, u, v float64) [3]float64 {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	u, v = u-math.Floor(u), v-math.Floor(v)
	px := min(w-1, int(u*float64(w)))
	py := min(h-1, int((1-v)*float64(h)))
	c := img.NRGBAAt(img.Rect.Min.X+px, img.Rect.Min.Y+py)
	return [3]float64{float64(c.R) / 255, float64(c.G) / 255, float64(c.B) / 255}
}

// paintOBJ writes renderer ri's triangles with the atlas UVs in its own mesh space, one "usemtl" group per submesh (the mod's mesh).
func paintOBJ(r paintRenderer, tris []paintTri, ri int) string {
	f := func(v float64) string { return strconv.FormatFloat(v, 'g', 6, 64) }
	var sb strings.Builder
	sb.WriteString("# TCG Studio furniture paint mesh (mesh space, Unity: left-handed, Y up; UV origin bottom-left)\n")
	type vk struct {
		v     uint32
		chart int
	}
	index := map[vk]int{}
	count := 0
	hasN := len(r.m.Normal) == len(r.m.Pos)
	faces := make([][]string, len(r.m.Subs))
	for _, t := range tris {
		if t.r != ri {
			continue
		}
		var ids [3]int
		for k := 0; k < 3; k++ {
			key := vk{t.v[k], t.chart}
			id, ok := index[key]
			if !ok {
				id = 1 + count
				count++
				index[key] = id
				p, n := f64(r.m.Pos[t.v[k]]), [3]float64{0, 1, 0}
				if hasN {
					n = f64(r.m.Normal[t.v[k]])
				}
				uv := t.uv[k]
				fmt.Fprintf(&sb, "v %s %s %s\nvt %s %s\nvn %s %s %s\n", f(p[0]), f(p[1]), f(p[2]),
					f(uv[0]/PaintSize), f(1-uv[1]/PaintSize), f(n[0]), f(n[1]), f(n[2]))
			}
			ids[k] = id
		}
		faces[t.sub] = append(faces[t.sub], fmt.Sprintf("f %d/%d/%d %d/%d/%d %d/%d/%d\n", ids[0], ids[0], ids[0], ids[1], ids[1], ids[1], ids[2], ids[2], ids[2]))
	}
	for si, fs := range faces {
		fmt.Fprintf(&sb, "usemtl sub%d\n", si)
		for _, l := range fs {
			sb.WriteString(l)
		}
	}
	return sb.String()
}

// indexPath is a node's path under root as "<sibling index>:<name>/…" (the mod walks the indices and checks the names).
func indexPath(n, root *node) string {
	var parts []string
	for p := n; p != nil && p != root && p.parent != nil; p = p.parent {
		idx := 0
		for i, c := range p.parent.children {
			if c == p {
				idx = i
				break
			}
		}
		parts = append([]string{strconv.Itoa(idx) + ":" + p.name}, parts...)
	}
	return strings.Join(parts, "/")
}

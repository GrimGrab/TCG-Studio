package gameextract

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"tcgstudio/internal/unityfs"
)

// Prefab files: shelves and the shop item prefab live here. Shelves in a save's shop are clones of these, so they aren't
// needed (and would only describe one player's shop).
var prefabFiles = []string{"sharedassets1.assets", "sharedassets0.assets", "resources.assets"}

type compartment struct {
	Path               string     `json:"path"`
	Start              [3]float32 `json:"start"`
	EndWidth           [3]float32 `json:"endWidth"`
	EndDepth           [3]float32 `json:"endDepth"`
	EndHeight          [3]float32 `json:"endHeight"`
	Right              [3]float32 `json:"right"`
	Up                 [3]float32 `json:"up"`
	Forward            [3]float32 `json:"forward"`
	Width              float32    `json:"width"`
	Depth              float32    `json:"depth"`
	Height             float32    `json:"height"`
	SizeX              int32      `json:"sizeX"`
	SizeY              int32      `json:"sizeY"`
	SizeZ              int32      `json:"sizeZ"`
	CanPutItem         bool       `json:"m_CanPutItem"`
	CanPutBox          bool       `json:"m_CanPutBox"`
	ApplyScaleOffset   bool       `json:"m_ApplyScaleOffset"`
	HeightGoesUp       bool       `json:"m_HeightGoesUp"`
	AffectedByTallItem bool       `json:"m_AffectedByTallItem"`
	ItemNotForSale     bool       `json:"m_ItemNotForSale"`
	PosCount           int        `json:"posCount"`
}

type shelf struct {
	Name           string        `json:"name"`
	IsPrefab       bool          `json:"isPrefab"`
	ItemNotForSale bool          `json:"m_ItemNotForSale"`
	RootLossyScale [3]float32    `json:"rootLossyScale"`
	Mesh           *string       `json:"mesh"`
	Compartments   []compartment `json:"compartments"`
}

func lossy(m mat) [3]float32 {
	return f32s([3]float64{length([3]float64{m[0][0], m[1][0], m[2][0]}), length([3]float64{m[0][1], m[1][1], m[2][1]}), length([3]float64{m[0][2], m[1][2], m[2][2]})})
}

func rows(m mat) [][4]float32 {
	out := make([][4]float32, 4)
	for i := range out {
		for j := 0; j < 4; j++ {
			out[i][j] = float32(m[i][j])
		}
	}
	return out
}

// comp returns the first component of a class (or MonoBehaviour script class) on a node.
func (g *graph) comp(n *node, classID int32, script string) *unityfs.Object {
	for _, c := range n.comps {
		if c.ClassID == classID && (script == "" || g.env.ScriptClass(c) == script) {
			return c
		}
	}
	return nil
}

// shelves: former ShelfExport (item prefab + shelf prefabs with compartments and a merged model).
func (x *extractor) shelves() (prefab any, out []shelf, tables []any, err error) {
	tables = []any{}
	seen := map[string]bool{}
	meshes := x.newMeshSet("")
	for _, name := range prefabFiles {
		f, err := x.env.File(name)
		if err != nil || f == nil {
			continue
		}
		items := x.env.FindScripts(f, "Item")
		shelves := x.env.FindScripts(f, "Shelf")
		sets := x.env.FindScripts(f, "TableGameItemSet")
		if len(items) == 0 && len(shelves) == 0 && len(sets) == 0 {
			continue
		}
		g := newGraph(x.env, f)
		if prefab == nil {
			for _, it := range items {
				if p := x.itemPrefab(g, it); p != nil {
					prefab = p
					break
				}
			}
		}
		for _, so := range sets {
			tables = append(tables, x.tables(g, so)...)
		}
		for _, so := range shelves {
			sh, err := unityfs.ReadShelf(so)
			if err != nil {
				x.warn("%v", err)
				continue
			}
			root := g.nodeOf(sh.GameObject)
			if root == nil {
				continue
			}
			sname := strings.TrimSpace(strings.ReplaceAll(root.name, "(Clone)", ""))
			if seen[sname] {
				continue
			}
			s := x.shelf(g, root, sname, sh, meshes)
			if len(s.Compartments) == 0 {
				continue
			}
			seen[sname] = true
			out = append(out, s)
		}
	}
	if prefab == nil {
		x.warn("shop item prefab not found")
	}
	if len(out) == 0 {
		return prefab, nil, nil, fmt.Errorf("no shelves found")
	}
	return prefab, out, tables, nil
}

func (x *extractor) itemPrefab(g *graph, item *unityfs.Object) any {
	goRef, mfRef, err := unityfs.ReadItemMeshFilter(item)
	if err != nil || mfRef.Null() {
		return nil
	}
	root := g.nodeOf(goRef)
	mfo, _ := x.env.Resolve(g.file, mfRef)
	if root == nil || mfo == nil {
		return nil
	}
	mf, err := unityfs.ReadMeshFilter(mfo)
	if err != nil {
		return nil
	}
	mn := g.nodeOf(mf.GameObject)
	if mn == nil {
		return nil
	}
	rw, mw := root.world(), mn.world()
	m := mul(rw.inverse(), mw)
	return map[string]any{
		"name": root.name, "meshPath": mn.path(), "meshMatrix": rows(m),
		"meshLocalPosition": f32s(rw.inverse().point([3]float64{mw[0][3], mw[1][3], mw[2][3]})),
		"meshLossyScale":    lossy(mw), "rootLossyScale": lossy(rw),
	}
}

func (x *extractor) shelf(g *graph, root *node, name string, sh unityfs.Shelf, meshes *meshSet) shelf {
	rw := root.world()
	inv := rw.inverse()
	invRot := root.worldQuat().inverse()
	s := shelf{Name: name, IsPrefab: true, ItemNotForSale: sh.ItemNotForSale, RootLossyScale: lossy(rw), Compartments: []compartment{}}
	// Like the mod: each group's direct children, in list order.
	var nodes []*node
	for _, grp := range sh.CompartmentGroups {
		if gn := g.transformNode(grp); gn != nil {
			nodes = append(nodes, gn.children...)
		}
	}
	for _, n := range nodes {
		co := g.comp(n, unityfs.ClassMonoBehaviour, "ShelfCompartment")
		if co == nil {
			continue
		}
		c, err := unityfs.ReadShelfCompartment(co)
		if err != nil {
			x.warn("%s: %v", name, err)
			continue
		}
		st, ew, ed, eh := g.transformNode(c.StartLoc), g.transformNode(c.EndWidthLoc), g.transformNode(c.EndDepthLoc), g.transformNode(c.EndHeightLoc)
		if st == nil || ew == nil || ed == nil || eh == nil {
			continue
		}
		pos := func(n *node) [3]float64 { w := n.world(); return [3]float64{w[0][3], w[1][3], w[2][3]} }
		ps, pw, pd, ph := pos(st), pos(ew), pos(ed), pos(eh)
		rel := invRot.mul(st.worldQuat())
		cc := compartment{
			Path: n.path(), Start: f32s(inv.point(ps)), EndWidth: f32s(inv.point(pw)), EndDepth: f32s(inv.point(pd)), EndHeight: f32s(inv.point(ph)),
			Right: f32s(rel.rotate([3]float64{1, 0, 0})), Up: f32s(rel.rotate([3]float64{0, 1, 0})), Forward: f32s(rel.rotate([3]float64{0, 0, 1})),
			Width: float32(length(sub(pw, ps))), Depth: float32(length(sub(pd, ps))), Height: float32(length(sub(ph, ps))),
			SizeX: c.SizeX, SizeY: c.SizeY, SizeZ: c.SizeZ,
			CanPutItem: c.CanPutItem, CanPutBox: c.CanPutBox, ApplyScaleOffset: c.ApplyScaleOffset, HeightGoesUp: c.HeightGoesUp,
			AffectedByTallItem: c.AffectedByTallItem, ItemNotForSale: c.ItemNotForSale,
		}
		if pl := g.transformNode(c.PosListGrp); pl != nil {
			cc.PosCount = len(pl.children)
		}
		s.Compartments = append(s.Compartments, cc)
	}
	if len(s.Compartments) > 0 {
		if file := x.shelfModel(g, root, name, inv, meshes); file != "" {
			s.Mesh = &file
		}
	}
	return s
}

// shelfModel merges the shelf's static renderers (not items or boxes on it) into one OBJ in the shelf's root space.
func (x *extractor) shelfModel(g *graph, root *node, name string, inv mat, meshes *meshSet) string {
	file := "Shelf_" + safe(name) + ".obj"
	if !x.mergedModel(g, root, "Shelf "+name, filepath.Join(x.acc, file), inv, meshes, nil) {
		return ""
	}
	return file
}

// mergedModel writes a piece's static renderers (not items or boxes on it, nor nodes skip rejects) as one OBJ in the space of inv
// (the root's inverse world matrix). Groups are named after the renderer's path. False when nothing was written.
func (x *extractor) mergedModel(g *graph, root *node, title, path string, inv mat, meshes *meshSet, skip func(*node) bool) bool {
	var sb strings.Builder
	sb.WriteString("# " + title + " — extracted by TCG Studio from the game files (root space, Unity: left-handed, Y up)\n")
	f := func(v float64) string { return strconv.FormatFloat(float64(float32(v)), 'g', -1, 32) }
	base, parts, tris := 1, 0, 0
	stop := false
	root.walk(func(n *node) {
		if stop {
			return
		}
		if parts >= 80 || tris > 60000 {
			stop = true
			return
		}
		ro := g.comp(n, unityfs.ClassMeshRenderer, "")
		mfo := g.comp(n, unityfs.ClassMeshFilter, "")
		if ro == nil || mfo == nil {
			return
		}
		r, err := unityfs.ReadRenderer(ro)
		if err != nil || !r.Enabled || n.hasInParent("Item", "InteractablePackagingBox") || (skip != nil && skip(n)) {
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
		if m == nil || len(m.Pos) == 0 {
			return
		}
		xf := mul(inv, n.world())
		mirrored := xf.det3() < 0
		for _, v := range m.Pos {
			p := xf.point(f64(v))
			sb.WriteString("v " + f(p[0]) + " " + f(p[1]) + " " + f(p[2]) + "\n")
		}
		hasN := len(m.Normal) == len(m.Pos)
		if hasN {
			for _, v := range m.Normal {
				d := normalize(xf.vector(f64(v)))
				sb.WriteString("vn " + f(d[0]) + " " + f(d[1]) + " " + f(d[2]) + "\n")
			}
		}
		sb.WriteString("g " + strings.ReplaceAll(n.path(), " ", "_") + "\n")
		for _, sub := range m.Subs {
			for i := 0; i+2 < len(sub); i += 3 {
				a, b, c := int(sub[i])+base, int(sub[i+1])+base, int(sub[i+2])+base
				if mirrored {
					b, c = c, b
				}
				if hasN {
					fmt.Fprintf(&sb, "f %d//%d %d//%d %d//%d\n", a, a, b, b, c, c)
				} else {
					fmt.Fprintf(&sb, "f %d %d %d\n", a, b, c)
				}
				tris++
			}
		}
		base += len(m.Pos)
		parts++
	})
	if parts == 0 {
		return false
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		x.warn("%s: %v", title, err)
		return false
	}
	return true
}

// tables: the play table's playmat / deck box renderers ("tables" of the former in-game export); their meshes go through the
// accessory mesh set, like the mod.
func (x *extractor) tables(g *graph, set *unityfs.Object) []any {
	pm, db, err := unityfs.ReadTableGameItemSet(set)
	if err != nil {
		x.warn("%v", err)
		return nil
	}
	owner := "TableGameItemSet"
	if gref, err := unityfs.ComponentGameObject(set); err == nil {
		if n := g.nodeOf(gref); n != nil {
			owner += "(" + n.name + ")"
		}
	}
	var out []any
	for _, t := range []struct {
		field string
		ref   unityfs.PPtr
	}{{"m_PlayMatMesh", pm}, {"m_DeckBoxMesh", db}} {
		ro, _ := x.env.Resolve(g.file, t.ref)
		if ro == nil {
			continue
		}
		r, err := unityfs.ReadRenderer(ro)
		if err != nil {
			continue
		}
		n := g.nodeOf(r.GameObject)
		if n == nil {
			continue
		}
		mfo := g.comp(n, unityfs.ClassMeshFilter, "")
		if mfo == nil {
			continue
		}
		mf, err := unityfs.ReadMeshFilter(mfo)
		if err != nil {
			continue
		}
		file, m := x.accMeshes.export(g.file, mf.Mesh)
		if m == nil {
			continue
		}
		var mat any
		if len(r.Materials) > 0 {
			mat = x.matInfo(g.file, r.Materials[0])
		}
		out = append(out, map[string]any{"owner": owner + "." + t.field, "path": n.path(), "mesh": strOrNil(file), "meshName": m.Name,
			"bounds": m.Bounds(), "lossyScale": lossy(n.world()), "material": mat})
	}
	return out
}

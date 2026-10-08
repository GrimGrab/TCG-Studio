package gameextract

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/unityfs"
)

// Furniture templates (furniture/furniture.json): every piece sold in the furniture shop with its behaviour (script class →
// the mod's furniture type), shop data, icon, a merged model in the piece's root space and its spots in the accessory library's
// "spots" format — the same numbers the mod's furniture-templates.txt dump prints (Runtime/FurnitureTemplateDump).

// FurnitureDir is the templates sub-folder.
const FurnitureDir = "furniture"

// furnitureTypes maps game script classes to the mod's FurnitureType names (Core/FurnitureKinds.cs).
var furnitureTypes = map[string]string{
	"Shelf": "Shelf", "CardShelf": "CardShelf", "InteractablePlayTable": "PlayTable",
	"WarehouseShelf": "WarehouseShelf", "TournamentPrizeShelf": "TournamentPrizeShelf",
	"InteractableBulkDonationBox": "BulkDonationBox", "InteractableTrashBin": "TrashBin",
	"InteractableEmptyBoxStorage": "EmptyBoxStorage", "InteractableCardStorageShelf": "CardStorageShelf",
	"InteractableAutoPackOpener": "AutoPackOpener", "InteractableAutoCleanser": "AutoCleanser",
	"InteractableWorkbench": "Workbench", "InteractableCashierCounter": "CashCounter",
}

// FurnitureTypeOfScript is the furniture type a game script class gives a piece ("" = none), with the game's subclass
// convention ("Shelf_…" is a Shelf). Also used for mod prefabs built on the game's components (EPL furniture).
func FurnitureTypeOfScript(class string) string {
	for c, t := range furnitureTypes {
		if class == c || strings.HasPrefix(class, c+"_") {
			return t
		}
	}
	return ""
}

type furnitureSpot struct {
	Kind     string      `json:"kind"`
	Pos      [3]float32  `json:"pos"`
	Rot      [3]float32  `json:"rot"`
	Size     *[3]float32 `json:"size,omitempty"`
	Grid     *[3]int32   `json:"grid,omitempty"`
	Customer *[2]float32 `json:"customer,omitempty"`
	PriceTag *[3]float32 `json:"priceTag,omitempty"`
	Boxes    *bool       `json:"boxes,omitempty"`
}

type furniturePiece struct {
	Base      string            `json:"base"`   // EObjectType name
	Object    int32             `json:"object"` // EObjectType value
	Class     string            `json:"class"`  // game script class
	Type      string            `json:"type"`   // mod furniture type ("" = not usable as a base)
	NameTerm  string            `json:"nameTerm"`
	Price     float32           `json:"price"`
	Level     int32             `json:"level"`
	DecoBonus float32           `json:"decoBonus"`
	Icon      string            `json:"icon,omitempty"`
	IconSize  [2]float32        `json:"iconSize"`
	Model     string            `json:"model,omitempty"`
	Bounds    *[2][3]float32    `json:"bounds,omitempty"` // model min / max (root space)
	Area      *furnitureArea    `json:"area,omitempty"`   // placement area (m_MoveStateValidArea)
	Points    []furniturePoint  `json:"points"`
	Screens   []furnitureScreen `json:"screens,omitempty"` // the game's UI screens on the piece (editor previews)           // seats / stand / worker positions by role (see ReadFurniturePoints)
	// Flags of the base's first item compartment: custom item spots are clones of it (Runtime/FurnitureSpots).
	HeightGoesUp       bool            `json:"heightGoesUp"`
	ApplyScaleOffset   bool            `json:"applyScaleOffset"`
	AffectedByTallItem bool            `json:"affectedByTallItem"`
	Spots              []furnitureSpot `json:"spots"`
}

// furnitureArea is the box the game keeps free of other furniture/walls when placing a piece (InteractableObject
// m_MoveStateValidArea: an OverlapBox at its position with its lossy scale as size), in the piece's root space.
// furniturePoint is a position the game uses on a piece (a seat, where the cashier stands…), in the piece's root space.
// furnitureScreen is a world-UI screen the game shows on a piece (till, card reader, pack opener progress), for the editor's preview:
// its size in metres at the point's scale 1, and its place in the frame of the point it follows (role). Its face is seen from the
// frame's −z side, like any world UI. Look picks the editor's mock picture.
type furnitureScreen struct {
	Role string     `json:"role"`
	Look string     `json:"look"`
	Size [2]float32 `json:"size"`
	Pos  [3]float32 `json:"pos"`
	Rot  [3]float32 `json:"rot"`
}

// furniturePoint is a vanilla piece's point as the furniture def has it (setfmt.FurniturePoint) plus, for working parts, the
// part's size around it (min, max in the point's frame: its position and rotation) for the editor.
type furniturePoint struct {
	setfmt.FurniturePoint
	Box *[2][3]float32 `json:"box,omitempty"`
}

type furnitureArea struct {
	Pos  [3]float32 `json:"pos"`
	Size [3]float32 `json:"size"` // width, height, depth
	Rot  [3]float32 `json:"rot"`
}

// shelfData reads the game's furniture list (ShelfData_ScriptableObject).
func (x *extractor) shelfData() (unityfs.ShelfData, error) {
	for _, name := range prefabFiles {
		f, err := x.env.File(name)
		if err != nil || f == nil {
			continue
		}
		if found := x.env.FindScripts(f, "ShelfData_ScriptableObject"); len(found) > 0 {
			return unityfs.ReadShelfData(found[0])
		}
	}
	return unityfs.ShelfData{}, fmt.Errorf("the game's furniture list (ShelfData) wasn't found")
}

// lookSkip: nodes that aren't part of a piece's look — inactive parts, the placement area, highlight shells, nav-mesh cutters,
// price-tag placeholders (merged preview model and paint template).
func lookSkip(g *graph, io unityfs.InteractableObject) func(*node) bool {
	helpers := map[*node]bool{}
	for _, ref := range []unityfs.PPtr{io.Highlight, io.NavMeshCut} {
		if n := g.nodeOf(ref); n != nil {
			helpers[n] = true
		}
	}
	if n := g.transformNode(io.ValidArea); n != nil {
		helpers[n] = true
	}
	return func(n *node) bool {
		if !n.activeInHierarchy() || n.hasInParent("InteractablePriceTag", "InteractableCardPriceTag") {
			return true
		}
		for p := n; p != nil; p = p.parent {
			if helpers[p] {
				return true
			}
			for _, w := range []string{"highlight", "hightlight", "movestatevalidarea", "dashedline"} {
				if strings.Contains(strings.ToLower(p.name), w) {
					return true
				}
			}
		}
		return false
	}
}

func (x *extractor) furniture() ([]furniturePiece, error) {
	sd, err := x.shelfData()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(x.dir, FurnitureDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	uis := x.uiScreens()
	graphs := map[*unityfs.File]*graph{}
	meshes := x.newMeshSet("")
	var out []furniturePiece
	for _, buy := range sd.Purchases {
		name := x.objNames[buy.ObjectType]
		if name == "" {
			x.warn("furniture type %d has no name", buy.ObjectType)
			continue
		}
		var od *unityfs.ObjectData
		for i := range sd.Objects {
			if sd.Objects[i].ObjectType == buy.ObjectType {
				od = &sd.Objects[i]
				break
			}
		}
		p := furniturePiece{Base: name, Object: buy.ObjectType, NameTerm: buy.Name, Price: buy.Price, Level: buy.Level, Spots: []furnitureSpot{}, Points: []furniturePoint{}}
		if od == nil {
			x.warn("furniture %s: no object data", name)
			continue
		}
		p.DecoBonus = od.DecoBonus
		if rect, ok := x.saveSprite(sd.File, buy.Icon, filepath.Join(dir, safe(name)+"_icon.png")); ok {
			p.Icon, p.IconSize = safe(name)+"_icon.png", rect
		}
		po, _ := x.env.Resolve(sd.File, od.Prefab)
		if po == nil {
			x.warn("furniture %s: prefab not found", name)
			out = append(out, p)
			continue
		}
		p.Class = x.env.ScriptClass(po)
		p.Type = furnitureTypes[p.Class]
		g := graphs[po.File]
		if g == nil {
			g = newGraph(x.env, po.File)
			graphs[po.File] = g
		}
		io, err := unityfs.ReadInteractableObject(po)
		if err != nil {
			x.warn("furniture %s: %v", name, err)
			out = append(out, p)
			continue
		}
		root := g.nodeOf(io.GameObject)
		if root == nil {
			out = append(out, p)
			continue
		}
		inv := root.world().inverse()
		invRot := root.worldQuat().inverse()
		if sg, err := unityfs.ReadSpotGroups(po, p.Class); err == nil {
			p.Spots = append(p.Spots, x.itemSpots(g, sg.Items, inv, invRot)...)
			p.Spots = append(p.Spots, x.cardSpots(g, sg.Cards, inv, invRot)...)
			if c := x.firstCompartment(g, sg.Items); c != nil {
				p.HeightGoesUp, p.ApplyScaleOffset, p.AffectedByTallItem = c.HeightGoesUp, c.ApplyScaleOffset, c.AffectedByTallItem
			}
		} else {
			x.warn("furniture %s: %v", name, err)
		}
		if refs, err := x.refNodes(g, po, p.Class); err == nil {
			p.Screens = x.pieceScreens(p.Type, refs, uis)
		}
		if rps, err := x.rolePoints(g, po, p.Class, p.Type); err == nil {
			for _, rp := range rps {
				pt := furniturePoint{FurniturePoint: setfmt.FurniturePoint{Role: rp.role.Role, Pos: r3x(inv.point(wpos(rp.n))), Rot: r3x(f64(euler(invRot.mul(rp.n.worldQuat()))))}}
				if rp.role.Kind == "part" {
					pt.Box = x.partBox(g, rp.n, meshes)
				}
				p.Points = append(p.Points, pt)
			}
		} else {
			x.warn("furniture %s points: %v", name, err)
		}
		if an := g.transformNode(io.ValidArea); an != nil {
			w := mul(inv, an.world())
			p.Area = &furnitureArea{Pos: r3(w.point([3]float64{})), Size: r3(f64(lossy(w))), Rot: euler(invRot.mul(an.worldQuat()))}
		}
		skip := lookSkip(g, io)
		file := safe(name) + ".obj"
		if x.mergedModel(g, root, "Furniture "+name, filepath.Join(dir, file), inv, meshes, skip) {
			p.Model = file
			if lo, hi, ok := objBounds(filepath.Join(dir, file)); ok {
				p.Bounds = &[2][3]float32{lo, hi}
			}
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no furniture found")
	}
	return out, writeJSON(filepath.Join(dir, "furniture.json"), map[string]any{"version": 1, "pieces": out})
}

// itemSpots: shelf compartments as the mod's item spots (centre of the box, m_StartLoc rotation, size, grid).
func (x *extractor) itemSpots(g *graph, groups []unityfs.PPtr, inv mat, invRot quat) []furnitureSpot {
	var out []furnitureSpot
	for _, grp := range groups {
		gn := g.transformNode(grp)
		if gn == nil {
			continue
		}
		for _, n := range gn.children {
			co := g.comp(n, unityfs.ClassMonoBehaviour, "ShelfCompartment")
			if co == nil {
				continue
			}
			c, err := unityfs.ReadShelfCompartment(co)
			if err != nil {
				x.warn("%v", err)
				continue
			}
			st, ew, ed, eh := g.transformNode(c.StartLoc), g.transformNode(c.EndWidthLoc), g.transformNode(c.EndDepthLoc), g.transformNode(c.EndHeightLoc)
			if st == nil || ew == nil || ed == nil || eh == nil {
				continue
			}
			ps, pw, pd, ph := wpos(st), wpos(ew), wpos(ed), wpos(eh)
			w, d, h := length(sub(pw, ps)), length(sub(pd, ps)), length(sub(ph, ps))
			q := st.worldQuat()
			right, up, fwd := q.rotate([3]float64{1, 0, 0}), q.rotate([3]float64{0, 1, 0}), q.rotate([3]float64{0, 0, 1})
			sign := -1.0
			if c.HeightGoesUp {
				sign = 1
			}
			var centre [3]float64
			for i := 0; i < 3; i++ {
				centre[i] = ps[i] - right[i]*w/2 + fwd[i]*d/2 + sign*up[i]*h/2
			}
			size := [3]float32{round3(w), round3(d), round3(h)}
			grid := [3]int32{c.SizeX, c.SizeY, c.SizeZ}
			boxes := c.CanPutBox
			s := furnitureSpot{Kind: "items", Pos: r3(inv.point(centre)), Rot: euler(invRot.mul(q)), Size: &size, Grid: &grid, Boxes: &boxes}
			if cn := g.transformNode(c.CustomerStandLoc); cn != nil {
				l := inv.point(wpos(cn))
				s.Customer = &[2]float32{round3(l[0]), round3(l[2])}
			}
			if len(c.PriceTags) > 0 {
				if tn := x.componentNode(g, c.PriceTags[0]); tn != nil {
					t := r3(inv.point(wpos(tn)))
					s.PriceTag = &t
				}
			}
			out = append(out, s)
		}
	}
	return out
}

// firstCompartment is the compartment custom item spots are cloned from (the first child with a ShelfCompartment, like the mod).
func (x *extractor) firstCompartment(g *graph, groups []unityfs.PPtr) *unityfs.ShelfCompartment {
	for _, grp := range groups {
		gn := g.transformNode(grp)
		if gn == nil {
			continue
		}
		for _, n := range gn.children {
			if co := g.comp(n, unityfs.ClassMonoBehaviour, "ShelfCompartment"); co != nil {
				if c, err := unityfs.ReadShelfCompartment(co); err == nil {
					return &c
				}
			}
		}
	}
	return nil
}

// cardSpots: card compartments as the mod's card spots (where the card sits).
func (x *extractor) cardSpots(g *graph, groups []unityfs.PPtr, inv mat, invRot quat) []furnitureSpot {
	var out []furnitureSpot
	for _, grp := range groups {
		gn := g.transformNode(grp)
		if gn == nil {
			continue
		}
		for _, n := range gn.children {
			co := g.comp(n, unityfs.ClassMonoBehaviour, "InteractableCardCompartment")
			if co == nil {
				continue
			}
			c, err := unityfs.ReadCardCompartment(co)
			if err != nil {
				x.warn("%v", err)
				continue
			}
			at := g.transformNode(c.PutCardLoc)
			if at == nil {
				at = n
			}
			s := furnitureSpot{Kind: "card", Pos: r3(inv.point(wpos(at))), Rot: euler(invRot.mul(at.worldQuat()))}
			if cn := g.transformNode(c.CustomerStandLoc); cn != nil {
				l := inv.point(wpos(cn))
				s.Customer = &[2]float32{round3(l[0]), round3(l[2])}
			}
			if len(c.PriceTags) > 0 {
				if tn := x.componentNode(g, c.PriceTags[0]); tn != nil {
					t := r3(inv.point(wpos(tn)))
					s.PriceTag = &t
				}
			}
			out = append(out, s)
		}
	}
	return out
}

// componentNode is the node a component (e.g. a price tag script) sits on.
func (x *extractor) componentNode(g *graph, p unityfs.PPtr) *node {
	o, _ := x.env.Resolve(g.file, p)
	if o == nil || o.File != g.file {
		return nil
	}
	gref, err := unityfs.ComponentGameObject(o)
	if err != nil {
		return nil
	}
	return g.nodeOf(gref)
}

func wpos(n *node) [3]float64 { w := n.world(); return [3]float64{w[0][3], w[1][3], w[2][3]} }

func round3(v float64) float32 { return float32(math.Round(v*1000) / 1000) }

func r3(v [3]float64) [3]float32 { return [3]float32{round3(v[0]), round3(v[1]), round3(v[2])} }

// r3x rounds to millimetres as float64 (setfmt's number type), without float32 noise.
func r3x(v [3]float64) [3]float64 {
	return [3]float64{math.Round(v[0]*1000) / 1000, math.Round(v[1]*1000) / 1000, math.Round(v[2]*1000) / 1000}
}

// euler converts a rotation to Unity's Euler angles in degrees (Quaternion.Euler(x, y, z) = Ry · Rx · Rz), each in (-180, 180].
func euler(q quat) [3]float32 {
	m := q.mat3()
	sx := -m[1][2]
	if sx > 1 {
		sx = 1
	} else if sx < -1 {
		sx = -1
	}
	x := math.Asin(sx)
	var y, z float64
	if math.Abs(math.Cos(x)) > 1e-6 {
		y = math.Atan2(m[0][2], m[2][2])
		z = math.Atan2(m[1][0], m[1][1])
	} else {
		y = math.Atan2(-m[2][0], m[0][0])
	}
	deg := func(a float64) float32 {
		d := a * 180 / math.Pi
		if math.Abs(d) < 0.005 {
			d = 0
		}
		return round3(d)
	}
	return [3]float32{deg(x), deg(y), deg(z)}
}

// objBounds reads the min/max of an OBJ's vertices.
func objBounds(path string) (lo, hi [3]float32, ok bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return lo, hi, false
	}
	lo = [3]float32{math.MaxFloat32, math.MaxFloat32, math.MaxFloat32}
	hi = [3]float32{-math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "v ") {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		for i := 0; i < 3; i++ {
			v, err := strconv.ParseFloat(f[i+1], 32)
			if err != nil {
				continue
			}
			lo[i] = float32(math.Min(float64(lo[i]), v))
			hi[i] = float32(math.Max(float64(hi[i]), v))
			ok = true
		}
	}
	return lo, hi, ok
}

// rolePoint is a point of a furniture piece: its role and the node it moves (setfmt.PointRoles, mirrors the mod's FurnitureKinds).
type rolePoint struct {
	role setfmt.PointRole
	n    *node
}

// rolePoints finds the nodes of a piece's point roles: the role's object (a Transform, a GameObject or a component's GameObject),
// or its parent for Parent roles (the cash drawer).
func (x *extractor) rolePoints(g *graph, po *unityfs.Object, class, typ string) ([]rolePoint, error) {
	refs, err := x.refNodes(g, po, class)
	if err != nil {
		return nil, err
	}
	var out []rolePoint
	for _, role := range setfmt.PointRoles(typ) {
		for _, n := range refs[role.Role] {
			if role.Parent {
				n = n.parent
			}
			if n != nil {
				out = append(out, rolePoint{role, n})
			}
		}
	}
	return out, nil
}

// refNodes resolves a furniture script's point objects (unityfs.ReadFurniturePoints, roles and helpers like the card screen) to nodes.
func (x *extractor) refNodes(g *graph, po *unityfs.Object, class string) (map[string][]*node, error) {
	refs, err := unityfs.ReadFurniturePoints(po, class)
	if err != nil {
		return nil, err
	}
	out := map[string][]*node{}
	for name, rs := range refs {
		for _, ref := range rs {
			var n *node
			switch ref.Of {
			case unityfs.RefTransform:
				n = g.transformNode(ref.Ptr)
			case unityfs.RefGameObject:
				n = g.nodeOf(ref.Ptr)
			case unityfs.RefComponent:
				if o, _ := x.env.Resolve(g.file, ref.Ptr); o != nil && o.File == g.file {
					if gp, err := unityfs.ComponentGameObject(o); err == nil {
						n = g.nodeOf(gp)
					}
				}
			}
			if n != nil {
				out[name] = append(out[name], n)
			}
		}
	}
	return out, nil
}

// pointFrame is a node's position and rotation (not its scale) in world space: the frame a point's pos/rot describe.
func pointFrame(n *node) mat {
	q, p := n.worldQuat().mat3(), wpos(n)
	var m mat
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			m[r][c] = q[r][c]
		}
		m[r][3] = p[r]
	}
	m[3][3] = 1
	return m
}

// uiScreen: a world-UI prefab's picture size in its own units (the largest image, anchors resolved) and its root scale.
type uiScreen struct{ extent, scale [2]float64 }

// uiScreens reads the till, card and pack-opener screen prefabs (WorldCanvasUIManager / AutoCardOpenerUISpawner in the shop scene).
func (x *extractor) uiScreens() map[string]uiScreen {
	out := map[string]uiScreen{}
	f, _ := x.env.File("level1")
	if f == nil {
		x.warn("shop scene not found: no screen previews")
		return out
	}
	add := func(name string, from *unityfs.File, ptr unityfs.PPtr) {
		o, _ := x.env.Resolve(from, ptr)
		if o == nil {
			return
		}
		gp, err := unityfs.ComponentGameObject(o)
		if err != nil {
			return
		}
		g := newGraph(x.env, o.File)
		if n := g.nodeOf(gp); n != nil && n.tr.Rect != nil {
			out[name] = uiScreen{extent: uiExtent(n), scale: [2]float64{float64(n.tr.Scale[0]), float64(n.tr.Scale[1])}}
		}
	}
	for _, o := range x.env.FindScripts(f, "WorldCanvasUIManager") {
		if ps, err := unityfs.ReadUIPrefabs(o, 2); err == nil {
			add("till", o.File, ps[0])
			add("card", o.File, ps[1])
		}
	}
	for _, o := range x.env.FindScripts(f, "AutoCardOpenerUISpawner") {
		if ps, err := unityfs.ReadUIPrefabs(o, 1); err == nil {
			add("progress", o.File, ps[0])
		}
	}
	return out
}

// uiExtent is the largest image under a UI root, in the root's units (stretched rects resolved against their parents).
func uiExtent(root *node) [2]float64 {
	var best [2]float64
	var walk func(n *node, parent, scale [2]float64)
	walk = func(n *node, parent, scale [2]float64) {
		r := n.tr.Rect
		if r == nil {
			return
		}
		size := [2]float64{float64(r.SizeDelta[0]), float64(r.SizeDelta[1])}
		if n != root {
			for k := 0; k < 2; k++ {
				size[k] += parent[k] * float64(r.AnchorMax[k]-r.AnchorMin[k])
			}
			scale = [2]float64{scale[0] * float64(n.tr.Scale[0]), scale[1] * float64(n.tr.Scale[1])}
		}
		if n.has("Image") {
			best = [2]float64{math.Max(best[0], math.Abs(size[0]*scale[0])), math.Max(best[1], math.Abs(size[1]*scale[1]))}
		}
		for _, c := range n.children {
			walk(c, size, scale)
		}
	}
	walk(root, [2]float64{}, [2]float64{1, 1})
	return best
}

// pieceScreens: the screens a piece type shows. Till and card screens keep their prefab scale (they only follow their point's
// position and rotation); the pack opener's takes its point's scale (InteractableAutoPackOpener copies m_UIPos.localScale).
func (x *extractor) pieceScreens(typ string, refs map[string][]*node, uis map[string]uiScreen) []furnitureScreen {
	size := func(u uiScreen, sx, sy float64) [2]float32 {
		return [2]float32{round3(u.extent[0] * sx), round3(u.extent[1] * sy)}
	}
	var out []furnitureScreen
	switch typ {
	case "CashCounter":
		if u, ok := uis["till"]; ok && len(refs["screen"]) > 0 {
			out = append(out, furnitureScreen{Role: "screen", Look: "till", Size: size(u, u.scale[0], u.scale[1])})
		}
		if u, ok := uis["card"]; ok && len(refs["cardMachine"]) > 0 && len(refs["cardScreen"]) > 0 {
			m, c := refs["cardMachine"][0], refs["cardScreen"][0]
			rel := mul(pointFrame(m).inverse(), pointFrame(c))
			out = append(out, furnitureScreen{Role: "cardMachine", Look: "card", Size: size(u, u.scale[0], u.scale[1]),
				Pos: r3(rel.point([3]float64{})), Rot: euler(m.worldQuat().inverse().mul(c.worldQuat()))})
		}
	case "AutoPackOpener":
		if u, ok := uis["progress"]; ok && len(refs["screen"]) > 0 {
			sc := refs["screen"][0].tr.Scale
			// Faces the frame's +z side (checked against the vanilla machine's screen frame).
			out = append(out, furnitureScreen{Role: "screen", Look: "progress", Size: size(u, float64(sc[0]), float64(sc[1])), Rot: [3]float32{0, 180, 0}})
		}
	}
	return out
}

// partBox is the extent of the visible meshes under a working part, in the part's frame (its position and rotation, not its scale).
func (x *extractor) partBox(g *graph, n *node, meshes *meshSet) *[2][3]float32 {
	inv := pointFrame(n).inverse()
	lo := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
	hi := [3]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	n.walk(func(c *node) {
		if !c.activeInHierarchy() || g.comp(c, unityfs.ClassMeshRenderer, "") == nil {
			return
		}
		mfo := g.comp(c, unityfs.ClassMeshFilter, "")
		if mfo == nil {
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
		if m == nil {
			return
		}
		xf := mul(inv, c.world())
		for i := 0; i < 8; i++ {
			var v [3]float64
			for k := 0; k < 3; k++ {
				e := float64(m.Extent[k])
				if i&(1<<k) == 0 {
					e = -e
				}
				v[k] = float64(m.Center[k]) + e
			}
			w := xf.point(v)
			for k := 0; k < 3; k++ {
				lo[k], hi[k] = math.Min(lo[k], w[k]), math.Max(hi[k], w[k])
			}
		}
	})
	if math.IsInf(lo[0], 1) {
		return nil
	}
	return &[2][3]float32{r3(lo), r3(hi)}
}

package gameextract

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"tcgstudio/internal/unityfs"
)

// Furniture templates (furniture/furniture.json): every piece sold in the furniture shop with its behaviour (script class →
// the mod's furniture type), shop data, icon, a merged model in the piece's root space and its spots in the accessory library's
// "spots" format — the same numbers the mod's furniture-templates.txt dump prints (Runtime/FurnitureTemplateDump).

// FurnitureDir is the templates sub-folder.
const FurnitureDir = "furniture"

// furnitureTypes maps game script classes to the mod's FurnitureType names (Core/FurnitureKinds.cs). Classes not listed
// (WarehouseShelf, TournamentPrizeShelf) can't be used as a base yet.
var furnitureTypes = map[string]string{
	"Shelf": "Shelf", "CardShelf": "CardShelf", "InteractablePlayTable": "PlayTable",
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
	Base      string           `json:"base"`   // EObjectType name
	Object    int32            `json:"object"` // EObjectType value
	Class     string           `json:"class"`  // game script class
	Type      string           `json:"type"`   // mod furniture type ("" = not usable as a base)
	NameTerm  string           `json:"nameTerm"`
	Price     float32          `json:"price"`
	Level     int32            `json:"level"`
	DecoBonus float32          `json:"decoBonus"`
	Icon      string           `json:"icon,omitempty"`
	IconSize  [2]float32       `json:"iconSize"`
	Model     string           `json:"model,omitempty"`
	Bounds    *[2][3]float32   `json:"bounds,omitempty"` // model min / max (root space)
	Area      *furnitureArea   `json:"area,omitempty"`   // placement area (m_MoveStateValidArea)
	Points    []furniturePoint `json:"points"`           // seats / stand / worker positions by role (see ReadFurniturePoints)
	// Flags of the base's first item compartment: custom item spots are clones of it (Runtime/FurnitureSpots).
	HeightGoesUp       bool            `json:"heightGoesUp"`
	ApplyScaleOffset   bool            `json:"applyScaleOffset"`
	AffectedByTallItem bool            `json:"affectedByTallItem"`
	Spots              []furnitureSpot `json:"spots"`
}

// furnitureArea is the box the game keeps free of other furniture/walls when placing a piece (InteractableObject
// m_MoveStateValidArea: an OverlapBox at its position with its lossy scale as size), in the piece's root space.
// furniturePoint is a position the game uses on a piece (a seat, where the cashier stands…), in the piece's root space.
type furniturePoint struct {
	Role string     `json:"role"`
	Pos  [3]float32 `json:"pos"`
	Rot  [3]float32 `json:"rot"`
}

type furnitureArea struct {
	Pos  [3]float32 `json:"pos"`
	Size [3]float32 `json:"size"` // width, height, depth
	Rot  [3]float32 `json:"rot"`
}

func (x *extractor) furniture() ([]furniturePiece, error) {
	var soObj *unityfs.Object
	for _, name := range prefabFiles {
		f, err := x.env.File(name)
		if err != nil || f == nil {
			continue
		}
		if found := x.env.FindScripts(f, "ShelfData_ScriptableObject"); len(found) > 0 {
			soObj = found[0]
			break
		}
	}
	if soObj == nil {
		return nil, fmt.Errorf("the game's furniture list (ShelfData) wasn't found")
	}
	sd, err := unityfs.ReadShelfData(soObj)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(x.dir, FurnitureDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
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
		switch p.Class {
		case "Shelf":
			if sh, err := unityfs.ReadShelf(po); err == nil {
				p.Spots = append(p.Spots, x.itemSpots(g, sh.CompartmentGroups, inv, invRot)...)
				if c := x.firstCompartment(g, sh.CompartmentGroups); c != nil {
					p.HeightGoesUp, p.ApplyScaleOffset, p.AffectedByTallItem = c.HeightGoesUp, c.ApplyScaleOffset, c.AffectedByTallItem
				}
			} else {
				x.warn("furniture %s: %v", name, err)
			}
		case "CardShelf":
			if cs, err := unityfs.ReadCardShelf(po); err == nil {
				p.Spots = append(p.Spots, x.cardSpots(g, cs.CompartmentGroups, inv, invRot)...)
			} else {
				x.warn("furniture %s: %v", name, err)
			}
		}
		if roles, err := unityfs.ReadFurniturePoints(po, p.Class); err == nil {
			for _, role := range []string{"sit", "stand", "standB", "cashier", "queue", "placeItems", "trade", "worker", "player", "customer"} {
				for _, ref := range roles[role] {
					if n := g.transformNode(ref); n != nil {
						p.Points = append(p.Points, furniturePoint{Role: role, Pos: r3(inv.point(wpos(n))), Rot: euler(invRot.mul(n.worldQuat()))})
					}
				}
			}
		} else {
			x.warn("furniture %s points: %v", name, err)
		}
		if an := g.transformNode(io.ValidArea); an != nil {
			w := mul(inv, an.world())
			p.Area = &furnitureArea{Pos: r3(w.point([3]float64{})), Size: r3(f64(lossy(w))), Rot: euler(invRot.mul(an.worldQuat()))}
		}
		// The look only: no placement area, highlight shells, nav-mesh cutters, price-tag placeholders or hidden parts.
		helpers := map[*node]bool{}
		for _, ref := range []unityfs.PPtr{io.Highlight, io.NavMeshCut} {
			if n := g.nodeOf(ref); n != nil {
				helpers[n] = true
			}
		}
		if n := g.transformNode(io.ValidArea); n != nil {
			helpers[n] = true
		}
		skip := func(n *node) bool {
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
	gref, err := unityfs.MonoBehaviourGameObject(o)
	if err != nil {
		return nil
	}
	return g.nodeOf(gref)
}

func wpos(n *node) [3]float64 { w := n.world(); return [3]float64{w[0][3], w[1][3], w[2][3]} }

func round3(v float64) float32 { return float32(math.Round(v*1000) / 1000) }

func r3(v [3]float64) [3]float32 { return [3]float32{round3(v[0]), round3(v[1]), round3(v[2])} }

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

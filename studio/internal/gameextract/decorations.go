package gameextract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/unityfs"
)

// Decoration templates (decorations/decorations.json): the vanilla "Buy Decoration" app — wall / floor / ceiling looks and
// placeable decorations (posters, fan art, statues, signs, plants) with shop name, price and icon — plus the two prefabs the
// mod builds custom decorations on (Runtime/DecorationInjector: Poster 1 on walls, plant 1 on the floor) with their merged
// model, so Studio can show a new decoration next to a vanilla one at the same scale.

// DecorationsDir is the templates sub-folder.
const DecorationsDir = "decorations"

// Base decorations of the mod (DecorationInjector.WallBase / FloorBase).
const (
	decoWallBase  = 1
	decoFloorBase = 121
)

type decoSurface struct {
	Index    int        `json:"index"`
	Name     string     `json:"name"` // the game's internal name ("Concrete", "Wood_Planks")
	Price    float32    `json:"price"`
	Icon     string     `json:"icon,omitempty"`
	IconSize [2]float32 `json:"iconSize"`
	ShowBar  bool       `json:"showBar,omitempty"`
}

type decoObject struct {
	Object   int        `json:"object"` // EDecoObject value
	Enum     string     `json:"enum"`   // EDecoObject name
	Name     string     `json:"name"`   // shop name ("Poster 12", "Dracunix Statue")
	Tab      string     `json:"tab"`    // Poster | Other
	Mount    string     `json:"mount"`  // Wall | Floor (InteractableObject.m_IsDecorationVertical)
	Price    float32    `json:"price"`
	Icon     string     `json:"icon,omitempty"`
	IconSize [2]float32 `json:"iconSize"`
}

type decoBase struct {
	Object int            `json:"object"`
	Model  string         `json:"model,omitempty"`
	Bounds *[2][3]float32 `json:"bounds,omitempty"` // model min / max (root space)
}

// decoName is the shop name the game builds from DecoPurchaseData (MainName "XXX Statue" / "Poster YYY").
func decoName(d unityfs.DecoPurchase) string {
	if d.MainName == "" {
		return d.Name
	}
	return strings.ReplaceAll(strings.ReplaceAll(d.MainName, "XXX", d.ReplaceXXX), "YYY", d.ReplaceYYY)
}

func (x *extractor) decorations(decoNames map[int32]string) error { // decoNames: EDecoObject value → name
	sd, err := x.shelfData()
	if err != nil {
		return err
	}
	if len(sd.Decos) == 0 || len(sd.Decos) != len(sd.DecoPurchases) {
		return fmt.Errorf("decoration lists look wrong (%d decorations, %d shop entries)", len(sd.Decos), len(sd.DecoPurchases))
	}
	dir := filepath.Join(x.dir, DecorationsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	surfaces := map[string][]decoSurface{}
	for kind, list := range map[string][]unityfs.ShopDeco{"Wall": sd.Walls, "Floor": sd.Floors, "Ceiling": sd.Ceil} {
		out := []decoSurface{}
		for i, s := range list {
			e := decoSurface{Index: i, Name: s.Name, Price: s.Price, ShowBar: s.ShowBar}
			file := fmt.Sprintf("%s_%d_icon.png", strings.ToLower(kind), i)
			if rect, ok := x.saveSprite(sd.File, s.Icon, filepath.Join(dir, file)); ok {
				e.Icon, e.IconSize = file, rect
			}
			out = append(out, e)
		}
		surfaces[kind] = out
	}

	tab := map[int32]string{}
	for _, v := range sd.PosterList {
		tab[v] = "Poster"
	}
	for _, v := range sd.OtherList {
		tab[v] = "Other"
	}
	graphs := map[*unityfs.File]*graph{}
	meshes := x.newMeshSet("")
	objects := []decoObject{}
	bases := map[string]decoBase{}
	for i := 1; i < len(sd.Decos); i++ {
		d, buy := sd.Decos[i], sd.DecoPurchases[i]
		o := decoObject{Object: i, Enum: decoNames[int32(i)], Name: decoName(buy), Tab: tab[int32(i)], Price: buy.Price, Mount: "Floor"}
		if o.Tab == "" {
			continue // not sold
		}
		file := fmt.Sprintf("deco_%d_icon.png", i)
		if rect, ok := x.saveSprite(sd.File, buy.Icon, filepath.Join(dir, file)); ok {
			o.Icon, o.IconSize = file, rect
		}
		po, _ := x.env.Resolve(sd.File, d.Prefab)
		if po == nil {
			x.warn("decoration %d: prefab not found", i)
			objects = append(objects, o)
			continue
		}
		io, err := unityfs.ReadInteractableObject(po)
		if err != nil {
			x.warn("decoration %d: %v", i, err)
			objects = append(objects, o)
			continue
		}
		if io.IsDecorationVertical {
			o.Mount = "Wall"
		}
		objects = append(objects, o)
		if i != decoWallBase && i != decoFloorBase {
			continue
		}
		g := graphs[po.File]
		if g == nil {
			g = newGraph(x.env, po.File)
			graphs[po.File] = g
		}
		root := g.nodeOf(io.GameObject)
		if root == nil {
			continue
		}
		b := decoBase{Object: i}
		model := fmt.Sprintf("base_%s.obj", strings.ToLower(o.Mount))
		if x.mergedModel(g, root, "Decoration "+o.Name, filepath.Join(dir, model), root.world().inverse(), meshes, lookSkip(g, io)) {
			b.Model = model
			if lo, hi, ok := objBounds(filepath.Join(dir, model)); ok {
				b.Bounds = &[2][3]float32{lo, hi}
			}
		}
		bases[o.Mount] = b
	}
	return writeJSON(filepath.Join(dir, "decorations.json"), map[string]any{"version": 1, "surfaces": surfaces, "objects": objects, "bases": bases})
}

package importer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"path/filepath"
	"strings"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/epl"
	"tcgstudio/internal/figurine"
	"tcgstudio/internal/gameextract"
	"tcgstudio/internal/setfmt"
)

// EPLItem is a mod item Studio converts into the accessory library (playmats, sleeves, deck boxes, comics, battle
// decks on the game's own models: their textures are already in those models' layouts).
type EPLItem struct {
	Key    string `json:"key"` // descriptor path in the mod | item name
	ID     string `json:"id"`  // accessory id it gets (eplmod-…)
	Name   string `json:"name"`
	Kind   string `json:"kind"` // accessory kind
	Base   string `json:"base"`
	Exists bool   `json:"exists"` // set by the app: already in the library
	Note   string `json:"note,omitempty"`
}

// eplItemKind decides what a mod item converts to: an accessory kind + base, or why it isn't converted.
func eplItemKind(it *epl.Item) (kind, base, reason string) {
	mesh := strings.TrimSpace(it.MeshToUse)
	switch {
	case it.AddItemAsBoardGame || strings.HasPrefix(strings.ToLower(mesh), "boardgame"):
		return "", "", "board games aren't supported by TCG Custom Cards"
	case mesh == "" && it.Mesh != "" && (it.AddItemAsFigurine || strings.EqualFold(it.ItemCategory, "Figurine")):
		if toy := pickToy(it); toy != nil {
			return "Figurine", toy.Type, ""
		}
		return "", "", "figurine sizes aren't known"
	case mesh == "" && it.Mesh != "":
		return "", "", "has its own 3D model for an item type that only figurines support"
	case mesh == "":
		return "", "", "no game model named"
	}
	for _, k := range setfmt.AccessoryKinds {
		if k.Model {
			continue
		}
		for _, b := range k.Bases {
			if strings.EqualFold(b, mesh) {
				return k.Kind, b, ""
			}
		}
	}
	return "", "", fmt.Sprintf("uses the game model %q, which has no accessory kind here", mesh)
}

// eplItemID is the accessory id of a converted item: the prefix for converted content, the bundle and the item name.
func eplItemID(bundlePath, name string) string {
	cut := func(s string, n int) string {
		s = slug(s)
		if len(s) > n {
			s = strings.TrimSuffix(s[:n], "-")
		}
		return s
	}
	return accessories.ConvertedPrefix + cut(filepath.Base(bundlePath), 20) + "-" + cut(name, 32)
}

func eplItemKey(rel, name string) string { return rel + "|" + name }
func eplVariantKey(rel, name string, n int) string {
	return fmt.Sprintf("%s|%s#%d", rel, name, n)
}

// variantID is the id of texture variant n (1-based): the first keeps the item's id, so an earlier single import is
// replaced in place.
func variantID(id string, n int) string {
	if n <= 1 {
		return id
	}
	return fmt.Sprintf("%s-%d", id, n)
}

// itemMaterials lists an item's distinct textures (materials) in order: MaterialList when given (EPL fills the game's
// per-submesh material list from it), else Material. Mods use the list for several designs of one item (comic covers,
// playmat designs); the game's comic and playmat models show one at a time, so each becomes its own accessory.
func itemMaterials(it *epl.Item) []string {
	list := it.MaterialList
	if len(list) == 0 && it.Material != "" {
		list = []string{it.Material}
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range list {
		if m = strings.TrimSpace(m); m != "" && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

// IconJob is a converted accessory whose shop icon Studio still has to render from its texture (texture variants: the
// mod's own icon shows several designs at once).
type IconJob struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Base    string `json:"base"`
	Texture string `json:"texture"` // library-relative
}

func eplPrefabKey(rel, name string) string { return rel + "|prefab:" + name }

// furnitureType is the game furniture type a mod prefab is built on (its first game furniture component), "" = none.
func furnitureType(a *epl.Assets, p *epl.Prefab) string {
	if a == nil {
		return ""
	}
	for _, s := range a.PrefabScripts(p.PrefabName) {
		if t := gameextract.FurnitureTypeOfScript(s); t != "" {
			return t
		}
	}
	return ""
}

// eplItems sorts a descriptor's items (other than packs and boxes) into convertible accessories and skipped content.
func eplItems(b *epl.Bundle, a *epl.Assets, strip bool) (items []EPLItem, skipped []EPLSkipped) {
	for i := range b.Desc.Items {
		it := &b.Desc.Items[i]
		if it.IsCardPack || it.IsCardBox {
			continue
		}
		kind, base, reason := eplItemKind(it)
		if kind == "" {
			k := it.ItemCategory
			if k == "" || k == "None" {
				k = "Item"
			}
			skipped = append(skipped, EPLSkipped{Name: it.Name, Kind: k, Reason: reason})
			continue
		}
		id, name := eplItemID(b.Path, it.Name), modName(it.Name, strip)
		if mats := itemMaterials(it); kind != "Figurine" && len(mats) > 1 {
			for n := range mats {
				items = append(items, EPLItem{Key: eplVariantKey(b.Rel, it.Name, n+1), ID: variantID(id, n+1),
					Name: fmt.Sprintf("%s (%d of %d)", name, n+1, len(mats)), Kind: kind, Base: base})
			}
			continue
		}
		items = append(items, EPLItem{Key: eplItemKey(b.Rel, it.Name), ID: id, Name: name, Kind: kind, Base: base})
	}
	for i := range b.Desc.Prefabs {
		p := &b.Desc.Prefabs[i]
		if !p.AddItemToFurnitureShop {
			skipped = append(skipped, EPLSkipped{Name: p.Name, Kind: "Deco", Reason: "posters and decorations aren't supported by TCG Custom Cards"})
			continue
		}
		ftype := furnitureType(a, p)
		if ftype == "" {
			skipped = append(skipped, EPLSkipped{Name: p.Name, Kind: "Furniture",
				Reason: "isn't built on one of the game's furniture types (its behaviour comes from the mod's own code)"})
			continue
		}
		t, _ := setfmt.FurnitureTypeInfo(ftype)
		items = append(items, EPLItem{Key: eplPrefabKey(b.Rel, p.Name), ID: eplItemID(b.Path, p.Name), Name: modName(p.Name, strip),
			Kind: "Furniture", Base: ftype, Note: "works as a " + t.One + "; extra behaviour from the mod's own code isn't carried over"})
	}
	if len(b.Desc.CustomShops) > 0 {
		skipped = append(skipped, EPLSkipped{Name: "Phone shop app", Kind: "Shop", Reason: "items go to the game's own shop tabs"})
	}
	return items, skipped
}

// ImportEPLItems converts the chosen mod items (by EPLItem.Key) into the accessory library: texture = the item's
// material, icon = its sprite, price and license from the item. Items already converted are replaced. Returns how many
// were converted and the ones that failed (with why).
func ImportEPLItems(path string, keys []string, opt Options, lib *accessories.Library, report func(Progress)) (int, []IconJob, []string, error) {
	strip := opt.StripNumbers
	want := map[string]bool{}
	for _, k := range keys {
		want[k] = true
	}
	mod, err := epl.Open(path, EPLCacheDir, nil)
	if err != nil {
		return 0, nil, nil, err
	}
	n, done := 0, 0
	var failed []string
	var icons []IconJob
	for _, b := range mod.Bundles {
		type job struct {
			it      *epl.Item
			variant int // 0 = the item as a whole, else texture variant n of its materials
		}
		var todo []job
		for i := range b.Desc.Items {
			it := &b.Desc.Items[i]
			if want[eplItemKey(b.Rel, it.Name)] {
				todo = append(todo, job{it, 0})
			}
			for v := range itemMaterials(it) {
				if want[eplVariantKey(b.Rel, it.Name, v+1)] {
					todo = append(todo, job{it, v + 1})
				}
			}
		}
		var prefabs []*epl.Prefab
		for i := range b.Desc.Prefabs {
			if p := &b.Desc.Prefabs[i]; want[eplPrefabKey(b.Rel, p.Name)] {
				prefabs = append(prefabs, p)
			}
		}
		if len(todo)+len(prefabs) == 0 {
			continue
		}
		assets, err := epl.OpenAssets(b.Path)
		if err != nil {
			return n, icons, failed, err
		}
		for _, j := range todo {
			done++
			report(Progress{Stage: "items", Done: done, Total: len(keys), Message: "Converting " + j.it.Name})
			icon, err := convertItem(assets, b, j.it, j.variant, strip, lib)
			if err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", j.it.Name, err))
				continue
			}
			if icon != nil {
				icons = append(icons, *icon)
			}
			if kind, _, _ := eplItemKind(j.it); kind == "Figurine" || j.variant == 0 {
				lib.Meta.Origins[eplItemID(b.Path, j.it.Name)] = *eplOrigin(mod, &b, j.it.Name, opt)
			} else {
				lib.Meta.Origins[variantID(eplItemID(b.Path, j.it.Name), j.variant)] = *eplOrigin(mod, &b, j.it.Name, opt)
			}
			n++
		}
		for _, p := range prefabs {
			done++
			report(Progress{Stage: "items", Done: done, Total: len(keys), Message: "Converting " + p.Name})
			if err := convertFurniture(assets, b, p, strip, lib); err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", p.Name, err))
				continue
			}
			lib.Meta.Origins[eplItemID(b.Path, p.Name)] = *eplOrigin(mod, &b, p.Name, opt)
			n++
		}
		assets.Close()
	}
	return n, icons, failed, nil
}

// convertItem converts an accessory item, or one texture variant of it (variant n >= 1). A variant's shop icon has to be
// rendered from its texture (the mod's icon shows several designs): that is returned as an IconJob.
func convertItem(a *epl.Assets, b epl.Bundle, it *epl.Item, variant int, strip bool, lib *accessories.Library) (*IconJob, error) {
	kind, base, reason := eplItemKind(it)
	if kind == "" {
		return nil, fmt.Errorf("%s", reason)
	}
	if kind == "Figurine" {
		return nil, convertFigurine(a, b, it, base, strip, lib)
	}
	mats := itemMaterials(it)
	if len(mats) == 0 {
		return nil, fmt.Errorf("no texture named")
	}
	mat, id, name := mats[0], eplItemID(b.Path, it.Name), modName(it.Name, strip)
	if variant > 0 {
		if variant > len(mats) {
			return nil, fmt.Errorf("texture %d not found", variant)
		}
		mat, id = mats[variant-1], variantID(id, variant)
		if len(mats) > 1 {
			name = fmt.Sprintf("%s (%d of %d)", name, variant, len(mats))
		}
	}
	tex, err := a.MaterialTexture(mat)
	if err != nil {
		return nil, err
	}
	acc := setfmt.Accessory{ID: id, Kind: kind, Name: name, Base: base,
		License: setfmt.AccessoryLicense{Level: max(1, it.LicenseLevelRequirement), Price: it.LicensePrice}}
	if it.BaseCost > 0 {
		c := it.BaseCost
		acc.Cost = &c
	}
	data, err := encodePNG(tex)
	if err != nil {
		return nil, err
	}
	if acc.Texture, err = lib.WriteImage(id, "texture", data); err != nil {
		return nil, err
	}
	// The Accessory editor shows the mod's texture as a painted texture (layers added later go on top of it), instead of
	// starting from the vanilla art. A re-import reuses the earlier copy.
	src := acc.Texture // same bytes: the store keeps one file for both
	lib.Meta.Layouts[id], _ = json.Marshal(map[string]any{"version": 2, "layers": []any{}, "base": "vanilla",
		"baseColor": "#3a3a3a", "textureFile": src})

	var job *IconJob
	if len(mats) > 1 {
		job = &IconJob{ID: id, Kind: kind, Base: base, Texture: acc.Texture}
	} else if it.SpriteName != "" {
		if icon, err := a.Image(it.SpriteName); err == nil {
			if acc.Icon, err = writePNG(lib, id, "icon", icon); err != nil {
				return nil, err
			}
		}
	}
	lib.Put(acc)
	return job, nil
}

// storeSource stores a converted model's editable source (what the Figurine/Furniture editor opens): its OBJ and texture.
func storeSource(lib *accessories.Library, m *figurine.Mesh, tex image.Image) (model, texture string, err error) {
	if model, err = lib.PutBytes(figurine.SourceOBJ(m), ".obj"); err != nil {
		return "", "", err
	}
	png, err := figurine.PNG(tex)
	if err != nil {
		return "", "", err
	}
	texture, err = lib.PutBytes(png, ".png")
	return model, texture, err
}

func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img)
	return buf.Bytes(), err
}

func writePNG(lib *accessories.Library, id, suffix string, img image.Image) (string, error) {
	b, err := encodePNG(img)
	if err != nil {
		return "", err
	}
	return lib.WriteImage(id, suffix, b)
}

// ToySize is a vanilla toy's shelf slot (ItemData.itemDimension, isTallItem) and its mesh bounds (Unity mesh space).
type ToySize struct {
	Type   string     `json:"type"`
	Dim    [3]float64 `json:"itemDimension"`
	Tall   bool       `json:"isTallItem"`
	Bounds struct {
		Center [3]float64 `json:"center"`
		Size   [3]float64 `json:"size"`
	} `json:"bounds"`
}

// FigurineToys are the toys a converted figurine can stand in for (set by the app from its built-in measurements).
var FigurineToys []ToySize

// pickToy finds the figurine base whose shelf slot is closest to the item's (EPL items name their own dimension).
func pickToy(it *epl.Item) *ToySize {
	k, _ := setfmt.KindInfo("Figurine")
	var best *ToySize
	bestD := math.Inf(1)
	for i := range FigurineToys {
		t := &FigurineToys[i]
		ok := false
		for _, b := range k.Bases {
			ok = ok || b == t.Type
		}
		if !ok {
			continue
		}
		d := math.Hypot(t.Dim[0]-it.ItemDeminsion.X, t.Dim[1]-it.ItemDeminsion.Y)
		if t.Tall != it.IsTallItem {
			d += 0.5
		}
		if d < bestD {
			best, bestD = t, d
		}
	}
	return best
}

// convertFigurine bakes an item's own model like the Figurine editor: source files for re-editing (images/src), the
// model placed on the base toy's footprint at its own size (within reason), its texture and icon.
func convertFigurine(a *epl.Assets, b epl.Bundle, it *epl.Item, base string, strip bool, lib *accessories.Library) error {
	scene, err := a.PrefabScene(it.Mesh) // EPL names the model's prefab
	if err != nil {
		return err
	}
	// EPL paints the item's own material over the model (its renderers often keep a placeholder such as "defaultmat"
	// from the model import): do the same when the item names one the bundle has.
	if it.Material != "" {
		if img, err := a.MaterialTexture(it.Material); err == nil {
			scene.Materials = []figurine.Material{{Name: it.Material, Image: img, Factor: [4]float64{1, 1, 1, 1}}}
			for i := range scene.Parts {
				scene.Parts[i].Mat = 0
			}
		}
	}
	src, tex, err := figurine.Combine(scene)
	if err != nil {
		return err
	}
	if src.Triangles() == 0 {
		return fmt.Errorf("model %q is empty", it.Mesh)
	}
	toy := pickToy(it)
	if toy == nil {
		return fmt.Errorf("figurine sizes aren't known")
	}
	id := eplItemID(b.Path, it.Name)
	srcModel, srcTex, err := storeSource(lib, src, tex)
	if err != nil {
		return err
	}
	lo, hi := src.Bounds()
	height := hi[1] - lo[1] // EPL shows the model at its own size in the toy's place; keep it unless far off
	if bh := toy.Bounds.Size[1]; bh > 0 && (height < bh*0.3 || height > bh*3) {
		height = bh
	}
	c, sz := toy.Bounds.Center, toy.Bounds.Size
	p := figurine.Placement{Height: height, Anchor: [3]float64{c[0], c[1] - sz[1]/2, c[2]}}
	g, err := figurine.Place(src, p)
	if err != nil {
		return err
	}
	meshRel, err := lib.PutBytes(figurine.GameOBJ(g), ".obj")
	if err != nil {
		return err
	}
	acc := setfmt.Accessory{ID: id, Kind: "Figurine", Name: modName(it.Name, strip), Base: base, Mesh: meshRel,
		License: setfmt.AccessoryLicense{Level: max(1, it.LicenseLevelRequirement), Price: it.LicensePrice}}
	if it.BaseCost > 0 {
		cost := it.BaseCost
		acc.Cost = &cost
	}
	if acc.Texture, err = writePNG(lib, id, "texture", tex); err != nil {
		return err
	}
	if it.SpriteName != "" {
		if icon, err := a.Image(it.SpriteName); err == nil {
			if acc.Icon, err = writePNG(lib, id, "icon", icon); err != nil {
				return err
			}
		}
	}
	// The Figurine editor's layout, so the model can be turned or resized there later.
	layout, _ := json.Marshal(map[string]any{"version": "fig1", "model": srcModel, "texture": srcTex,
		"sourceName": strings.TrimSpace(it.Name), "rotX": 0, "rotY": 0, "rotZ": 0, "height": height,
		"triangles": src.Triangles(), "warnings": []string{}})
	lib.Meta.Layouts[id] = layout
	lib.Put(acc)
	return nil
}

// convertFurniture converts a furniture prefab: its model as-is (in the piece's own space), its game behaviour as the
// type, name/description/price/level/deco bonus and shop icon. Item spots and stand points stay the base piece's — check
// them on the Furniture page.
func convertFurniture(a *epl.Assets, b epl.Bundle, p *epl.Prefab, strip bool, lib *accessories.Library) error {
	ftype := furnitureType(a, p)
	t, ok := setfmt.FurnitureTypeInfo(ftype)
	if !ok {
		return fmt.Errorf("isn't built on one of the game's furniture types")
	}
	scene, err := a.PrefabScene(p.PrefabName)
	if err != nil {
		return err
	}
	src, tex, err := figurine.Combine(scene)
	if err != nil {
		return err
	}
	id := eplItemID(b.Path, p.Name)
	srcModel, srcTex, err := storeSource(lib, src, tex)
	if err != nil {
		return err
	}
	// Keep the model where the prefab has it: same height, bottom-centre back on its own spot (Unity space, z mirrored).
	lo, hi := src.Bounds()
	height := hi[1] - lo[1]
	anchor := [3]float64{(lo[0] + hi[0]) / 2, lo[1], -(lo[2] + hi[2]) / 2}
	g, err := figurine.Place(src, figurine.Placement{Height: height, Anchor: anchor})
	if err != nil {
		return err
	}
	meshRel, err := lib.PutBytes(figurine.GameOBJ(g), ".obj")
	if err != nil {
		return err
	}
	// The mod's description may promise behaviour from its own code: say what the piece does here.
	desc := strings.TrimSpace(strings.TrimSpace(p.Description) + " (Works as a " + t.One + " here.)")
	f := setfmt.Furniture{ID: id, Type: ftype, Name: modName(p.Name, strip), Description: desc, Base: t.DefaultBase, Mesh: meshRel}
	if p.Price > 0 {
		price := p.Price
		f.Price = &price
	}
	if p.LevelRequirement > 0 {
		lvl := p.LevelRequirement
		f.Level = &lvl
	}
	if p.DecoBonus > 0 {
		d := p.DecoBonus
		f.DecoBonus = &d
	}
	if f.Texture, err = writePNG(lib, id, "texture", tex); err != nil {
		return err
	}
	if p.SpriteName != "" {
		if icon, err := a.Image(p.SpriteName); err == nil {
			if f.Icon, err = writePNG(lib, id, "icon", icon); err != nil {
				return err
			}
		}
	}
	layout, _ := json.Marshal(map[string]any{"version": "fur1", "model": srcModel, "texture": srcTex,
		"sourceName": strings.TrimSpace(p.Name), "rotX": 0, "rotY": 0, "rotZ": 0, "height": height,
		"triangles": src.Triangles(), "warnings": []string{}, "autoIcon": false})
	lib.Meta.Layouts[id] = layout
	lib.PutFurniture(f)
	return nil
}

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
// decks, board games and card supplies on the game's own models: their textures are already in those models' layouts).
type EPLItem struct {
	Key    string `json:"key"` // descriptor path in the mod | item name
	ID     string `json:"id"`  // accessory id it gets (eplmod-…)
	Name   string `json:"name"`
	Kind   string `json:"kind"` // accessory kind
	Base   string `json:"base"`
	Exists bool   `json:"exists"` // set by the app: already in the library
	Note   string `json:"note,omitempty"`
}

// eplItemKind decides what a mod item converts to: an accessory kind + base, or why it isn't converted. A game model
// (MeshToUse) decides the kind, except a board game or card supply the mod sells as a figurine: that becomes a figurine
// with the game model as its own model. An own model is a figurine, or a board game when the mod sells it as one.
func eplItemKind(it *epl.Item) (kind, base, reason string) {
	mesh := strings.TrimSpace(it.MeshToUse)
	ownModel := mesh == "" && it.Mesh != ""
	if game, _ := figurineModel(it); game != "" {
		ownModel = true
	}
	switch {
	case ownModel && (it.AddItemAsFigurine || strings.EqualFold(it.ItemCategory, "Figurine")):
		if toy := pickBase("Figurine", it); toy != nil {
			return "Figurine", toy.Type, ""
		}
		return "", "", "figurine sizes aren't known"
	case ownModel && (it.AddItemAsBoardGame || strings.EqualFold(it.ItemCategory, "Boardgame")):
		if b := pickBase("BoardGame", it); b != nil {
			return "BoardGame", b.Type, ""
		}
		return "", "", "board game sizes aren't known"
	case ownModel:
		return "", "", "has its own 3D model for an item type that only figurines and board games support"
	case mesh == "":
		return "", "", "no game model named"
	}
	if kind, base = gameModelKind(mesh); kind != "" {
		return kind, base, ""
	}
	return "", "", fmt.Sprintf("uses the game model %q, which has no accessory kind here", mesh)
}

// gameModelKind is the accessory kind and base of a game model an item names (MeshToUse), "" when none has it.
func gameModelKind(mesh string) (kind, base string) {
	mesh = strings.TrimSpace(mesh)
	if m := strings.ToLower(mesh); strings.HasPrefix(m, "up_playmat") { // one tube base for all colours (setfmt.AccessoryKinds)
		mesh = "UP_Playmat1"
	}
	for _, k := range setfmt.AccessoryKinds {
		if k.Model {
			continue
		}
		for _, b := range k.Bases {
			if strings.EqualFold(b, mesh) {
				return k.Kind, b
			}
		}
	}
	return "", ""
}

// figurineModel tells whether an item is converted from a model of its own (baked like the Figurine editor): its own
// prefab (own), or the game's board game / card supply model the mod sells as a figurine (game = that base; only these
// kinds have one model of their own per base).
func figurineModel(it *epl.Item) (game string, own bool) {
	mesh := strings.TrimSpace(it.MeshToUse)
	if mesh == "" {
		return "", it.Mesh != ""
	}
	if !it.AddItemAsFigurine {
		return "", false
	}
	if kind, base := gameModelKind(mesh); kind == "BoardGame" || kind == "CardSupply" {
		return base, true
	}
	return "", false
}

// GameModel finds the OBJ file of a game item's model in the game templates (set by the app), or says why there is none.
var GameModel func(base string) (string, error)

// gameModelScene reads a game item's model from the game templates as a figurine scene with one material (the item's
// texture). The templates are in Unity's mesh space (left-handed), figurine scenes are right-handed: z is mirrored and the
// winding swapped here, and figurine.Place turns both back.
func gameModelScene(base string, tex image.Image) (*figurine.Scene, error) {
	if GameModel == nil {
		return nil, fmt.Errorf("the game's models aren't available")
	}
	path, err := GameModel(base)
	if err != nil {
		return nil, err
	}
	scene, err := figurine.LoadOBJ(path)
	if err != nil {
		return nil, err
	}
	scene.Warnings = nil // the templates have no .mtl: the texture comes from the item
	scene.Materials = []figurine.Material{{Name: base, Image: tex, Factor: [4]float64{1, 1, 1, 1}}}
	for i := range scene.Parts {
		p := &scene.Parts[i]
		p.Mat = 0
		for j := range p.Pos {
			p.Pos[j][2] = -p.Pos[j][2]
		}
		for j := range p.Nrm {
			p.Nrm[j][2] = -p.Nrm[j][2]
		}
		for t := 0; t+2 < len(p.Idx); t += 3 {
			p.Idx[t+1], p.Idx[t+2] = p.Idx[t+2], p.Idx[t+1]
		}
	}
	return scene, nil
}

// kindShop is the shop list a kind's vanilla items are sold from (setfmt.AccessoryShops).
func kindShop(kind string) string {
	switch kind {
	case "BoardGame", "CardSupply":
		return "tabletop"
	case "Figurine":
		return "figurines"
	case "BattleDeck":
		return "items"
	}
	return "accessories"
}

// eplShop: the shop EPL sells an item from (its AddItemAs… flags; EPL's restock entries carry them), "" when that is
// where the base item is sold anyway or no flag is set. A mod may sell a deck box as a board game.
func eplShop(it *epl.Item, kind string) string {
	shop := ""
	switch {
	case it.AddItemAsBoardGame:
		shop = "tabletop"
	case it.AddItemAsFigurine:
		shop = "figurines"
	case it.AddItemAsAccessory:
		shop = "accessories"
	}
	if shop == kindShop(kind) {
		return ""
	}
	return shop
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
		_, ownModel := figurineModel(it)
		if mats := itemMaterials(it); !ownModel && len(mats) > 1 {
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
		if kinds := eplDecoKinds(p); len(kinds) > 0 {
			for _, k := range kinds {
				it := EPLItem{Key: eplDecoKey(b.Rel, p.Name, k), ID: eplDecoID(b.Path, p, k), Name: modName(p.Name, strip), Kind: "Decoration", Base: k}
				if k == "Object" {
					it.Note = "decoration from the mod's model"
				}
				items = append(items, it)
			}
			continue
		}
		if !p.AddItemToFurnitureShop {
			skipped = append(skipped, EPLSkipped{Name: p.Name, Kind: "Prefab", Reason: "isn't sold in the furniture shop or as a decoration"})
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
		type decoJob struct {
			p    *epl.Prefab
			kind string
		}
		var decos []decoJob
		for i := range b.Desc.Prefabs {
			p := &b.Desc.Prefabs[i]
			if want[eplPrefabKey(b.Rel, p.Name)] {
				prefabs = append(prefabs, p)
			}
			for _, k := range eplDecoKinds(p) {
				if want[eplDecoKey(b.Rel, p.Name, k)] {
					decos = append(decos, decoJob{p, k})
				}
			}
		}
		if len(todo)+len(prefabs)+len(decos) == 0 {
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
			if _, own := figurineModel(j.it); j.variant == 0 || own {
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
		for _, j := range decos {
			done++
			report(Progress{Stage: "items", Done: done, Total: len(keys), Message: "Converting " + j.p.Name})
			if err := convertDecoration(assets, b, j.p, j.kind, strip, lib); err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", j.p.Name, err))
				continue
			}
			lib.Meta.Origins[eplDecoID(b.Path, j.p, j.kind)] = *eplOrigin(mod, &b, j.p.Name, opt)
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
	if _, own := figurineModel(it); own { // own model or a game model sold as a figurine
		return nil, convertFigurine(a, b, it, kind, base, strip, lib)
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
	acc := setfmt.Accessory{ID: id, Kind: kind, Name: name, Base: base, Shop: eplShop(it, kind),
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

// ToySize is a vanilla toy's (or board game's / card supply's) shelf slot (ItemData.itemDimension, isTallItem) and its mesh
// bounds (Unity mesh space).
type ToySize struct {
	Type   string     `json:"type"`
	Dim    [3]float64 `json:"itemDimension"`
	Tall   bool       `json:"isTallItem"`
	Bounds struct {
		Center [3]float64 `json:"center"`
		Size   [3]float64 `json:"size"`
	} `json:"bounds"`
}

// FigurineToys are the vanilla items a converted own model can stand in for: toys, board games and card supplies (set by
// the app from its built-in measurements).
var FigurineToys []ToySize

// pickBase finds the base of a kind whose shelf slot is closest to the item's (EPL items name their own dimension). A game
// model sold as a figurine keeps that model's own slot instead (mods leave the dimension at 1×1): the closest one at least
// as wide, so the model doesn't reach into its neighbours.
func pickBase(kind string, it *epl.Item) *ToySize {
	k, _ := setfmt.KindInfo(kind)
	x, y, tall, minX := it.ItemDeminsion.X, it.ItemDeminsion.Y, it.IsTallItem, 0.0
	if game, _ := figurineModel(it); game != "" {
		for _, t := range FigurineToys {
			if t.Type == game {
				x, y, tall, minX = t.Dim[0], t.Dim[1], t.Tall, t.Dim[0]
			}
		}
	}
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
		d := math.Hypot(t.Dim[0]-x, t.Dim[1]-y)
		if t.Tall != tall {
			d += 0.5
		}
		if t.Dim[0] < minX {
			d += 100 // only when no slot is wide enough
		}
		if d < bestD {
			best, bestD = t, d
		}
	}
	return best
}

// convertFigurine bakes an item's own model like the Figurine editor: source files for re-editing (images/src), the
// model placed on the base item's footprint at its own size (within reason), its texture and icon. kind = Figurine or
// BoardGame (own models of both are baked the same way). A game model sold as a figurine comes from the game templates
// with the item's texture, which is painted for that model's UVs.
func convertFigurine(a *epl.Assets, b epl.Bundle, it *epl.Item, kind, base string, strip bool, lib *accessories.Library) error {
	var scene *figurine.Scene
	game, _ := figurineModel(it)
	if game != "" {
		mats := itemMaterials(it)
		if len(mats) == 0 {
			return fmt.Errorf("no texture named")
		}
		img, err := a.MaterialTexture(mats[0])
		if err != nil {
			return err
		}
		if scene, err = gameModelScene(game, img); err != nil {
			return err
		}
	} else {
		var err error
		if scene, err = a.PrefabScene(it.Mesh); err != nil { // EPL names the model's prefab
			return err
		}
	}
	// EPL paints the item's own material over the model (its renderers often keep a placeholder such as "defaultmat"
	// from the model import): do the same when the item names one the bundle has.
	if game == "" && it.Material != "" {
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
	toy := pickBase(kind, it)
	if toy == nil {
		return fmt.Errorf("%s sizes aren't known", strings.ToLower(kind))
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
	acc := setfmt.Accessory{ID: id, Kind: kind, Name: modName(it.Name, strip), Base: base, Mesh: meshRel, Shop: eplShop(it, kind),
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

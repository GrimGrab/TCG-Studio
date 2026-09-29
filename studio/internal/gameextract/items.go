package gameextract

import (
	"fmt"
	"path/filepath"
	"strings"
)

func isPackOrBox(name string) bool {
	return strings.Contains(name, "CardPack") || strings.Contains(name, "CardBox") || strings.HasSuffix(name, "Pack")
}

// packTemplates: <type>_texture.png and <type>_icon.png per pack/box item in the templates root (former TemplateExport).
func (x *extractor) packTemplates() error {
	f := x.so.File
	n := 0
	for i := 0; i < x.count(); i++ {
		name := x.itemNames[int32(i)]
		if !isPackOrBox(name) {
			continue
		}
		if _, t, ok := x.mainTexture(f, x.so.Meshes[i].Material); ok && x.saveTexture(t, filepath.Join(x.dir, name+"_texture.png")) {
			n++
		}
		if _, ok := x.saveSprite(f, x.so.Items[i].Icon, filepath.Join(x.dir, name+"_icon.png")); ok {
			n++
		}
	}
	if n == 0 {
		return fmt.Errorf("no pack/box art could be read")
	}
	return nil
}

// kindOf mirrors the mod's AccessoryKinds.KindOf (true = an item Studio can make custom versions of).
func kindOf(name string) bool {
	for _, p := range []string{"DeckBox", "Playmat", "PlayMat", "CardSleeve_", "D20DiceBox", "Manga", "BinderBook", "PreconDeck_", "Toy_"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

type restockRow struct {
	Index  int     `json:"index"`
	Big    bool    `json:"big"`
	Level  int32   `json:"level"`
	Price  float32 `json:"price"`
	Amount int32   `json:"amount"`
}

type accItem struct {
	Type                  string       `json:"type"`
	ID                    int          `json:"id"`
	Category              string       `json:"category"`
	Name                  string       `json:"name"`
	BaseCost              float32      `json:"baseCost"`
	MarketPriceMinPercent float32      `json:"marketPriceMinPercent"`
	MarketPriceMaxPercent float32      `json:"marketPriceMaxPercent"`
	IsTallItem            bool         `json:"isTallItem"`
	ItemDimension         [3]float32   `json:"itemDimension"`
	PosYOffsetInBox       float32      `json:"posYOffsetInBox"`
	ScaleOffsetInBox      float32      `json:"scaleOffsetInBox"`
	ItemHandScaleOffset   float32      `json:"itemHandScaleOffset"`
	IconScale             float32      `json:"iconScale"`
	ColliderPosOffset     [3]float32   `json:"colliderPosOffset"`
	ColliderScale         [3]float32   `json:"colliderScale"`
	Texture               *string      `json:"texture"`
	TextureName           *string      `json:"textureName"`
	TextureSize           *[2]int      `json:"textureSize"`
	Icon                  *string      `json:"icon"`
	IconRect              *[2]float32  `json:"iconRect"`
	Mesh                  *string      `json:"mesh"`
	MeshName              *string      `json:"meshName"`
	Bounds                any          `json:"bounds"`
	MeshSecondary         *string      `json:"meshSecondary"`
	MeshSecondaryFile     *string      `json:"meshSecondaryFile"`
	BoundsSecondary       any          `json:"boundsSecondary"`
	Texture2              *string      `json:"texture2"`
	MaterialListTextures  []*string    `json:"materialListTextures"`
	ShownIn               []string     `json:"shownIn"`
	Material              any          `json:"material"`
	MaterialSecondary     any          `json:"materialSecondary"`
	MaterialList          []any        `json:"materialList"`
	RestockRows           []restockRow `json:"restockRows"`
}

func ptr[T any](v T) *T { return &v }

func strOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func has(list []int32, v int) bool {
	for _, x := range list {
		if int(x) == v {
			return true
		}
	}
	return false
}

// accessories: former AccessoryTemplateExport (items part): textures, icons, meshes and accessories.json items.
func (x *extractor) accessories() ([]accItem, error) {
	f := x.so.File
	x.accMeshes = x.newMeshSet(x.acc) // shared with the table meshes, like the mod
	meshes := x.accMeshes
	var items []accItem
	for i := 0; i < x.count(); i++ {
		name := x.itemNames[int32(i)]
		d := x.so.Items[i]
		cat := x.catNames[d.Category]
		if !kindOf(name) && cat != "Sleeve" && cat != "Dice" {
			continue
		}
		md := x.so.Meshes[i]
		it := accItem{Type: name, ID: i, Category: cat, Name: d.Name, BaseCost: d.BaseCost,
			MarketPriceMinPercent: d.MarketPriceMinPercent, MarketPriceMaxPercent: d.MarketPriceMaxPercent, IsTallItem: d.IsTallItem,
			ItemDimension: d.ItemDimension, PosYOffsetInBox: d.PosYOffsetInBox, ScaleOffsetInBox: d.ScaleOffsetInBox,
			ItemHandScaleOffset: d.ItemHandScaleOffset, IconScale: d.IconScale, ColliderPosOffset: d.ColliderPosOffset, ColliderScale: d.ColliderScale,
			MaterialListTextures: []*string{}, ShownIn: []string{}, MaterialList: []any{}, RestockRows: []restockRow{}}

		texObj, tex, hasTex := x.mainTexture(f, md.Material)
		if hasTex && x.saveTexture(tex, filepath.Join(x.acc, name+"_texture.png")) {
			it.Texture, it.TextureName, it.TextureSize = ptr(name+"_texture.png"), ptr(tex.Name), &[2]int{tex.Width, tex.Height}
		}
		if rect, ok := x.saveSprite(f, d.Icon, filepath.Join(x.acc, name+"_icon.png")); ok {
			it.Icon, it.IconRect = ptr(name+"_icon.png"), &rect
		}
		file, m := meshes.export(f, md.Mesh)
		it.Mesh, it.Bounds = strOrNil(file), bounds(m)
		if m != nil {
			it.MeshName = ptr(m.Name)
		}
		file2, m2 := meshes.export(f, md.MeshSecondary)
		it.MeshSecondaryFile, it.BoundsSecondary = strOrNil(file2), bounds(m2)
		if m2 != nil {
			it.MeshSecondary = ptr(m2.Name)
		}
		if o2, t2, ok := x.mainTexture(f, md.MaterialSecondary); ok && o2 != texObj && x.saveTexture(t2, filepath.Join(x.acc, name+"_texture2.png")) {
			it.Texture2 = ptr(name + "_texture2.png")
		}
		for k, mp := range md.MaterialList {
			if _, lt, ok := x.mainTexture(f, mp); ok && x.saveTexture(lt, filepath.Join(x.acc, fmt.Sprintf("%s_list%d_texture.png", name, k))) {
				it.MaterialListTextures = append(it.MaterialListTextures, ptr(fmt.Sprintf("%s_list%d_texture.png", name, k)))
			} else {
				it.MaterialListTextures = append(it.MaterialListTextures, nil)
			}
			it.MaterialList = append(it.MaterialList, x.matInfo(f, mp))
		}
		for _, s := range []struct {
			list []int32
			name string
		}{{x.so.Shown, "items"}, {x.so.ShownAccessory, "accessories"}, {x.so.ShownFigurine, "figurines"}, {x.so.ShownBoardGame, "boardgames"}, {x.so.ShownAll, "all"}} {
			if has(s.list, i) {
				it.ShownIn = append(it.ShownIn, s.name)
			}
		}
		it.Material, it.MaterialSecondary = x.matInfo(f, md.Material), x.matInfo(f, md.MaterialSecondary)
		for r, rd := range x.so.Restock {
			if int(rd.ItemType) == i {
				it.RestockRows = append(it.RestockRows, restockRow{Index: r, Big: rd.IsBigBox, Level: rd.Level, Price: rd.LicensePrice, Amount: rd.Amount})
			}
		}
		items = append(items, it)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no accessories found in the item list")
	}
	return items, nil
}

type packItem struct {
	Type              string  `json:"type"`
	ID                int     `json:"id"`
	Kind              string  `json:"kind"`
	Mesh              *string `json:"mesh"`
	MeshName          string  `json:"meshName"`
	Bounds            any     `json:"bounds"`
	MeshSecondary     *string `json:"meshSecondary"`
	MeshSecondaryFile *string `json:"meshSecondaryFile"`
	TextureName       *string `json:"textureName"`
	Texture           *string `json:"texture"`
	Material          any     `json:"material"`
	MaterialSecondary any     `json:"materialSecondary"`
}

// packMeshes: former PackTemplateExport (items part): pack/box meshes into accessories\ and packs.json.
func (x *extractor) packMeshes() ([]packItem, error) {
	f := x.so.File
	meshes := x.newMeshSet(x.acc)
	var out []packItem
	for i := 0; i < x.count(); i++ {
		name := x.itemNames[int32(i)]
		pack := strings.Contains(name, "CardPack") || strings.HasSuffix(name, "Pack")
		box := strings.Contains(name, "CardBox")
		if !pack && !box {
			continue
		}
		md := x.so.Meshes[i]
		file, m := meshes.export(f, md.Mesh)
		if m == nil {
			continue
		}
		it := packItem{Type: name, ID: i, Kind: map[bool]string{true: "box", false: "pack"}[box], Mesh: strOrNil(file), MeshName: m.Name, Bounds: m.Bounds(),
			Material: x.matInfo(f, md.Material), MaterialSecondary: x.matInfo(f, md.MaterialSecondary)}
		file2, m2 := meshes.export(f, md.MeshSecondary)
		it.MeshSecondaryFile = strOrNil(file2)
		if m2 != nil {
			it.MeshSecondary = ptr(m2.Name)
		}
		if _, t, ok := x.mainTexture(f, md.Material); ok {
			it.TextureName, it.Texture = ptr(t.Name), ptr(name+"_texture.png")
		}
		out = append(out, it)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no pack/box meshes found")
	}
	return out, nil
}

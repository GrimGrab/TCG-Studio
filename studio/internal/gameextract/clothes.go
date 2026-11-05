package gameextract

import (
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"tcgstudio/internal/unityfs"
)

// Customer clothes (ClothesDir): the game's customers are "CC" characters (Assembly-CSharp\CC\CharacterCustomization):
// a preset per character (Male0…, Female0…) names one apparel piece per slot (upper body, lower body, footwear, misc) and
// which colour variant of it. Each variant sets the piece's shader colours (_Tint, _Tint_R/G/B) and a _Print texture laid
// on the piece's UVs (scale 1, no offset). clothes.json lists every piece with its variants, an OBJ of its LOD0 mesh and
// the UV islands per material (the face editor's guides); the vanilla prints are saved as PNGs.

// ClothesDir is the templates subfolder of the customer clothes.
const ClothesDir = "clothes"

// ClothesTextureSize is the print texture size Studio paints.
const ClothesTextureSize = 1024

type ClothesFile struct {
	Version int                    `json:"version"`
	Bodies  map[string]ClothesBody `json:"bodies"` // "male", "female"
}

type ClothesBody struct {
	Slots   []ClothesSlot `json:"slots"`   // CharacterCustomization.ApparelTables order
	Presets []BodyPreset  `json:"presets"` // the game finds a character's by name ("Male" + model index)
}

type ClothesSlot struct {
	Table string          `json:"table"` // UpperBody_M…
	Label string          `json:"label"` // "Upper Body"
	Items []ClothingPiece `json:"items"`
}

type ClothingPiece struct {
	Name      string `json:"name"`    // CC item name (presets and the mod refer to it)
	Display   string `json:"display"` // "T-Shirt"
	Materials int    `json:"materials"`
	// Morph is the same mesh with the character sliders (the game sets them on clothes too, CharacterCustomization.setApparel), as characters' .bin.
	Morph    string            `json:"morph,omitempty"`
	Mesh     string            `json:"mesh,omitempty"` // OBJ (LOD0), relative to ClothesDir; "" = nothing to paint
	Variants []ClothesVariant  `json:"variants"`
	Islands  [][]ClothesIsland `json:"islands,omitempty"` // per material: its UV islands
	// PrintScale per material: the material's _Print_Scale (the print repeats that many times over the UVs; 1 = once).
	PrintScale []float64 `json:"printScale,omitempty"`
}

// PrintUVOffset: the apparel shader reads _Print at the mesh's UV + (0.5, 0.5), wrapping (measured in game 2026-10-26 with
// a labelled grid on the male TShirt_01; every apparel material's _Print scale/offset is 1/0, so it's in the compiled
// shader). Studio paints on the mesh's UVs and bakes the saved print into this space (BakeClothesPrint).
const PrintUVOffset = 0.5

// BakeClothesPrint turns a print painted on a piece's UVs (print scale 1) into the texture the game samples: pixel (x, y)
// takes the painting at (x, y) − PrintUVOffset, wrapping. The same move turns a game print back into UV space, since the
// offset is half the texture.
func BakeClothesPrint(src image.Image) *image.NRGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dx, dy := int(math.Round(float64(w)*PrintUVOffset)), int(math.Round(float64(h)*PrintUVOffset))
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, src.At(b.Min.X+(x+w-dx)%w, b.Min.Y+(y+h-dy)%h))
		}
	}
	return dst
}

type ClothesVariant struct {
	Label string          `json:"label"`
	Defs  []ClothesMatDef `json:"defs"` // per material
}

// ClothesMatDef: colours as #RRGGBBAA (the shader's linear values as stored), print = PNG relative to ClothesDir.
type ClothesMatDef struct {
	Tint  string `json:"tint"`
	TintR string `json:"tintR"`
	TintG string `json:"tintG"`
	TintB string `json:"tintB"`
	Print string `json:"print,omitempty"`
}

// ClothesIsland is one UV island: its rect in texture pixels (top-left origin, ClothesTextureSize) and where it sits.
type ClothesIsland struct {
	ID    string     `json:"id"`
	Label string     `json:"label"`
	Rect  [4]float64 `json:"rect"`
	Small bool       `json:"small,omitempty"` // seams, trims: shown as guides, not offered first
}

type BodyPreset struct {
	Name     string   `json:"name"`
	Apparel  []string `json:"apparel"`  // piece name per slot ("" / "None" = nothing)
	Variants []int32  `json:"variants"` // variant per slot
}

func colorHex(c [4]float32) string {
	b := func(v float32) int { return int(math.Round(math.Max(0, math.Min(1, float64(v))) * 255)) }
	return fmt.Sprintf("#%02x%02x%02x%02x", b(c[0]), b(c[1]), b(c[2]), b(c[3]))
}

// customerBody is the prefab the game's customers of one body are made from.
type customerBody struct {
	obj *unityfs.Object
	cc  unityfs.CharacterCustomization
}

// customerBodies finds the customer prefabs ("male", "female"). Since the 2026-10-27 update there are several of each. The game runs
// on the "_HP Variant" ones on PC (PlatformManager.IsHighEndConsole is true there: CustomerManager swaps them in), and those are drawn
// normally, so they're the ones that count; anything else only when a body has none.
func (x *extractor) customerBodies() map[string]customerBody {
	out := map[string]customerBody{}
	hp := map[string]bool{}
	for _, name := range x.assetFiles() {
		f, err := x.env.File(name)
		if err != nil || f == nil {
			continue
		}
		for _, o := range x.env.FindScripts(f, "CharacterCustomization") {
			cc, err := unityfs.ReadCharacterCustomization(o)
			if err != nil {
				continue
			}
			body := map[string]string{"Male": "male", "Female": "female"}[cc.CharacterName]
			if body == "" || hp[body] {
				continue
			}
			goName := ""
			if gob, _ := x.env.Resolve(o.File, cc.GameObject); gob != nil {
				if g, err := unityfs.ReadGameObject(gob); err == nil {
					goName = g.Name
				}
			}
			if _, have := out[body]; !have || strings.Contains(goName, "_HP Variant") {
				out[body], hp[body] = customerBody{o, cc}, strings.Contains(goName, "_HP Variant")
			}
		}
	}
	return out
}

func (x *extractor) clothes() error {
	dir := filepath.Join(x.dir, ClothesDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	out := ClothesFile{Version: 1, Bodies: map[string]ClothesBody{}}
	prints := map[unityfs.PPtr]string{}
	for body, cb := range x.customerBodies() {
		o, cc := cb.obj, cb.cc
		var b ClothesBody
		for _, tp := range cc.ApparelTables {
			to, err := x.env.Resolve(o.File, tp)
			if err != nil || to == nil {
				return fmt.Errorf("%s apparel table: %v", body, err)
			}
			tab, err := unityfs.ReadApparelTable(to)
			if err != nil {
				return err
			}
			slot := ClothesSlot{Table: tab.Name, Label: tab.Label}
			for _, it := range tab.Items {
				p := ClothingPiece{Name: it.Name, Display: it.DisplayName}
				for _, v := range it.Variants {
					cv := ClothesVariant{Label: v.Label}
					for _, d := range v.Defs {
						md := ClothesMatDef{Tint: colorHex(d.MainTint), TintR: colorHex(d.TintR), TintG: colorHex(d.TintG), TintB: colorHex(d.TintB)}
						if !d.Print.Null() {
							name, ok := prints[d.Print]
							if !ok {
								name = fmt.Sprintf("print_%d.png", len(prints))
								if _, t, ok := x.texture(to.File, d.Print); ok && x.saveTexture(t, filepath.Join(dir, name)) {
									prints[d.Print] = name
								} else {
									name = ""
								}
							}
							md.Print = name
						}
						cv.Defs = append(cv.Defs, md)
					}
					if len(cv.Defs) > p.Materials {
						p.Materials = len(cv.Defs)
					}
					p.Variants = append(p.Variants, cv)
				}
				if !it.Mesh.Null() && p.Materials > 0 {
					if m, scales := x.clothingMesh(to.File, it.Mesh); m != nil {
						for j := 0; j < p.Materials; j++ {
							s := 1.0
							if j < len(scales) {
								s = scales[j]
							}
							p.PrintScale = append(p.PrintScale, s)
						}
						file := body + "_" + safeName(it.Name) + ".obj"
						if err := os.WriteFile(filepath.Join(dir, file), []byte(m.OBJ("extracted by TCG Studio from the game files")), 0o644); err == nil {
							p.Mesh = file
						}
						morph := body + "_" + safeName(it.Name) + ".bin"
						if err := writeCharMesh(filepath.Join(dir, morph), m, isPreviewShape, nil); err == nil {
							p.Morph = morph
						}
						for s := range m.Subs {
							if s < p.Materials {
								p.Islands = append(p.Islands, uvIslands(m, s, ClothesTextureSize))
							}
						}
					}
				}
				slot.Items = append(slot.Items, p)
			}
			b.Slots = append(b.Slots, slot)
		}
		if po, err := x.env.Resolve(o.File, cc.Presets); err == nil && po != nil {
			if _, ps, err := unityfs.ReadCharacterPresets(po); err == nil {
				for _, p := range ps {
					b.Presets = append(b.Presets, BodyPreset{Name: p.Name, Apparel: p.ApparelNames, Variants: p.ApparelMaterials})
				}
			} else {
				x.warn("%s presets: %v", body, err)
			}
		}
		out.Bodies[body] = b
	}
	if len(out.Bodies) == 0 {
		return fmt.Errorf("no customer bodies found")
	}
	return writeJSON(filepath.Join(dir, "clothes.json"), out)
}

func safeName(s string) string {
	b := []byte(s)
	for i, c := range b {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			b[i] = '_'
		}
	}
	return string(b)
}

// clothingMesh finds the first SkinnedMeshRenderer under a piece's prefab (its LOD0: the LODs are siblings in order) and
// its materials' _Print_Scale (1 when unset).
func (x *extractor) clothingMesh(from *unityfs.File, prefab unityfs.PPtr) (*unityfs.Mesh, []float64) {
	var scales []float64
	var walk func(gob *unityfs.Object, depth int) *unityfs.Mesh
	walk = func(gob *unityfs.Object, depth int) *unityfs.Mesh {
		if depth > 12 {
			return nil
		}
		g, err := unityfs.ReadGameObject(gob)
		if err != nil {
			return nil
		}
		var kids []unityfs.PPtr
		for _, cp := range g.Components {
			co, _ := x.env.Resolve(gob.File, cp)
			if co == nil {
				continue
			}
			switch co.ClassID {
			case unityfs.ClassSkinnedMesh:
				rend, mp, err := unityfs.ReadSkinnedMesh(co)
				if err != nil {
					continue
				}
				if mo, _ := x.env.Resolve(co.File, mp); mo != nil {
					if m, err := unityfs.ReadMesh(x.env, mo); err == nil && len(m.UV) == len(m.Pos) {
						yUp(m)
						for _, mat := range rend.Materials {
							s := 1.0
							if _, md := x.material(co.File, mat); md != nil {
								if v, ok := md.Floats["_Print_Scale"]; ok && v > 0 {
									s = float64(v)
								}
							}
							scales = append(scales, s)
						}
						return m
					}
				}
			case unityfs.ClassTransform:
				if tr, err := unityfs.ReadTransform(co); err == nil {
					kids = tr.Children
				}
			}
		}
		for _, ch := range kids {
			cto, _ := x.env.Resolve(gob.File, ch)
			if cto == nil {
				continue
			}
			ctr, err := unityfs.ReadTransform(cto)
			if err != nil {
				continue
			}
			if cgo, _ := x.env.Resolve(cto.File, ctr.GameObject); cgo != nil {
				if m := walk(cgo, depth+1); m != nil {
					return m
				}
			}
		}
		return nil
	}
	gob, err := x.env.Resolve(from, prefab)
	if err != nil || gob == nil {
		return nil, nil
	}
	m := walk(gob, 0)
	return m, scales
}

// uvIslands splits a submesh into UV islands (triangles connected through shared vertices; Unity splits vertices at UV
// seams) and names them by where they sit on the body: the big ones facing forward / back are Front / Back, sideways
// ones Left / Right (the character's own left), the rest Part n; tiny islands are marked Small.
func uvIslands(m *unityfs.Mesh, sub, size int) []ClothesIsland {
	tris := m.Subs[sub]
	parent := make([]int, len(m.Pos))
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
	for i := 0; i+2 < len(tris); i += 3 {
		a, b, c := find(int(tris[i])), find(int(tris[i+1])), find(int(tris[i+2]))
		parent[b] = a
		parent[find(c)] = a
	}
	type acc struct {
		x0, y0, x1, y1 float64
		area           float64
		n, c           [3]float64
	}
	groups := map[int]*acc{}
	total := 0.0
	for i := 0; i+2 < len(tris); i += 3 {
		r := find(int(tris[i]))
		g := groups[r]
		if g == nil {
			g = &acc{x0: math.Inf(1), y0: math.Inf(1), x1: math.Inf(-1), y1: math.Inf(-1)}
			groups[r] = g
		}
		var uv [3][2]float64
		for k := 0; k < 3; k++ {
			v := int(tris[i+k])
			u := m.UV[v]
			uv[k] = [2]float64{float64(u[0]), 1 - float64(u[1])}
			g.x0, g.x1 = math.Min(g.x0, uv[k][0]), math.Max(g.x1, uv[k][0])
			g.y0, g.y1 = math.Min(g.y0, uv[k][1]), math.Max(g.y1, uv[k][1])
		}
		ar := math.Abs((uv[1][0]-uv[0][0])*(uv[2][1]-uv[0][1])-(uv[2][0]-uv[0][0])*(uv[1][1]-uv[0][1])) / 2
		g.area += ar
		total += ar
		for k := 0; k < 3; k++ {
			v := int(tris[i+k])
			for j := 0; j < 3; j++ {
				g.c[j] += float64(m.Pos[v][j]) * ar / 3
				if len(m.Normal) == len(m.Pos) {
					g.n[j] += float64(m.Normal[v][j]) * ar / 3
				}
			}
		}
	}
	list := make([]*acc, 0, len(groups))
	for _, g := range groups {
		list = append(list, g)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].area > list[j].area })
	var out []ClothesIsland
	used := map[string]int{}
	S := float64(size)
	for _, g := range list {
		if g.area <= 0 {
			continue
		}
		small := g.area < total*0.03
		label := "Part"
		if !small {
			nx, nz := g.n[0], g.n[forwardAxis]
			if math.Abs(nz) >= math.Abs(nx) {
				if nz*forwardSign > 0 {
					label = "Front"
				} else {
					label = "Back"
				}
			} else if g.c[0]*rightSign > 0 {
				label = "Right side"
			} else {
				label = "Left side"
			}
		} else {
			label = "Small part"
		}
		used[label]++
		id := fmt.Sprintf("%s%d", map[string]string{"Front": "front", "Back": "back", "Right side": "right", "Left side": "left", "Small part": "small", "Part": "part"}[label], used[label])
		if used[label] > 1 {
			label = fmt.Sprintf("%s %d", label, used[label])
		}
		out = append(out, ClothesIsland{ID: id, Label: label, Small: small,
			Rect: [4]float64{math.Floor(g.x0 * S), math.Floor(g.y0 * S), math.Ceil(g.x1 * S), math.Ceil(g.y1 * S)}})
	}
	return out
}

// yUp turns a CC piece from its mesh space (Z up, front towards −y: the T-shirt's deep-neck panel faces −y) to the
// previews' frame (Y up, front towards +z, right = +x): (x, y, z) → (x, z, −y), a plain rotation.
func yUp(m *unityfs.Mesh) {
	turn := func(v [3]float32) [3]float32 { return [3]float32{v[0], v[2], -v[1]} }
	for i := range m.Pos {
		m.Pos[i] = turn(m.Pos[i])
	}
	for i := range m.Normal {
		m.Normal[i] = turn(m.Normal[i])
	}
	m.Center = turn(m.Center)
	m.Extent = [3]float32{m.Extent[0], m.Extent[2], m.Extent[1]}
}

// After yUp: the front faces +z, the character's right is +x.
const (
	forwardAxis = 2
	forwardSign = 1.0
	rightSign   = 1.0
)

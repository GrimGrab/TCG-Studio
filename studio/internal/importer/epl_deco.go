package importer

import (
	"encoding/json"
	"fmt"
	"image"
	"math"
	"strings"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/epl"
	"tcgstudio/internal/figurine"
	"tcgstudio/internal/setfmt"
)

// EPL decorations (PrefabInfo with IsDecoObject / IsDeco{Wall,Floor,Ceiling}Texture; prefabloader
// LoadingPipeline/Processors/PrefabProcessor.cs): placeable ones become "Object" decorations with the mod's model kept in place
// (EPL spawns the prefab itself, so its root is where the game snaps it), surfaces become Wall / Floor / Ceiling looks from the
// named textures. One prefab may be flagged for several surfaces: each becomes its own look.

// eplDecoKinds lists the decoration kinds a prefab converts to (none = not a decoration).
func eplDecoKinds(p *epl.Prefab) []string {
	var out []string
	if p.IsDecoObject {
		out = append(out, "Object")
	}
	if p.IsDecoWallTexture {
		out = append(out, "Wall")
	}
	if p.IsDecoFloorTexture {
		out = append(out, "Floor")
	}
	if p.IsDecoCeilingTexture {
		out = append(out, "Ceiling")
	}
	return out
}

func eplDecoKey(rel, name, kind string) string { return rel + "|deco:" + kind + ":" + name }

// eplDecoID: the prefab's id; a prefab flagged for more than one kind gets the kind appended for the 2nd… ones.
func eplDecoID(bundlePath string, p *epl.Prefab, kind string) string {
	id := eplItemID(bundlePath, p.Name)
	if kinds := eplDecoKinds(p); len(kinds) > 1 && kinds[0] != kind {
		id += "-" + strings.ToLower(kind)
	}
	return id
}

func convertDecoration(a *epl.Assets, b epl.Bundle, p *epl.Prefab, kind string, strip bool, lib *accessories.Library) error {
	id := eplDecoID(b.Path, p, kind)
	d := setfmt.Decoration{ID: id, Kind: kind, Name: modName(p.Name, strip), Price: math.Max(0, p.Price)}
	if d.Price == 0 {
		d.Price = setfmt.NewDecoration(kind, "", "").Price
	}
	var err error
	if kind == "Object" {
		err = convertDecoObject(a, p, &d, lib)
	} else {
		err = convertDecoSurface(a, p, &d, lib)
	}
	if err != nil {
		return err
	}
	if p.SpriteName != "" {
		if icon, err := a.Image(p.SpriteName); err == nil {
			if d.Icon, err = writePNG(lib, id, "icon", icon); err != nil {
				return err
			}
		}
	}
	lib.PutDecoration(d)
	return nil
}

func convertDecoSurface(a *epl.Assets, p *epl.Prefab, d *setfmt.Decoration, lib *accessories.Library) error {
	if strings.TrimSpace(p.Texture) == "" {
		return fmt.Errorf("no texture named")
	}
	tex, err := a.Image(p.Texture)
	if err != nil {
		return fmt.Errorf("texture %q: %w", p.Texture, err)
	}
	if d.Texture, err = writePNG(lib, d.ID, "texture", tex); err != nil {
		return err
	}
	if p.NormalMap != "" {
		if img, err := a.Image(p.NormalMap); err == nil {
			if d.NormalMap, err = writePNG(lib, d.ID, "normal", unswizzleNormal(img)); err != nil {
				return err
			}
		}
	}
	if p.RoughnessMap != "" {
		if img, err := a.Image(p.RoughnessMap); err == nil {
			if d.RoughnessMap, err = writePNG(lib, d.ID, "metallic", img); err != nil {
				return err
			}
		}
	}
	// EPL passes the colour straight to the material (0–1); a mod that leaves it out would get black, keep white then.
	if c := p.Color; c != nil && c.R+c.G+c.B > 0 {
		ch := func(v float64) int { return int(math.Round(math.Max(0, math.Min(1, v)) * 255)) }
		if s := fmt.Sprintf("#%02X%02X%02X", ch(c.R), ch(c.G), ch(c.B)); s != "#FFFFFF" {
			d.Color = s
		}
	}
	sm := math.Max(0, math.Min(1, p.Smoothness))
	d.Smoothness = &sm
	return nil
}

func convertDecoObject(a *epl.Assets, p *epl.Prefab, d *setfmt.Decoration, lib *accessories.Library) error {
	scene, err := a.PrefabScene(p.PrefabName)
	if err != nil {
		return err
	}
	src, tex, err := figurine.Combine(scene)
	if err != nil {
		return err
	}
	srcModel, srcTex, err := storeSource(lib, src, tex)
	if err != nil {
		return err
	}
	wall, _ := a.PrefabWallMounted(p.PrefabName)
	d.Mount = "Floor"
	if wall {
		d.Mount = "Wall"
	}
	if strings.EqualFold(p.DecoType, "Poster") {
		d.Tab = "Poster"
	}
	// Keep the model where the prefab has it (Unity space, z mirrored), like converted furniture.
	lo, hi := src.Bounds()
	height := hi[1] - lo[1]
	anchor := [3]float64{(lo[0] + hi[0]) / 2, lo[1], -(lo[2] + hi[2]) / 2}
	g, err := figurine.Place(src, figurine.Placement{Height: height, Anchor: anchor})
	if err != nil {
		return err
	}
	if d.Mesh, err = lib.PutBytes(figurine.GameOBJ(g), ".obj"); err != nil {
		return err
	}
	if d.Texture, err = writePNG(lib, d.ID, "texture", tex); err != nil {
		return err
	}
	layout, _ := json.Marshal(map[string]any{"version": "deco1", "mode": "kept", "model": srcModel, "texture": srcTex,
		"sourceName": strings.TrimSpace(p.Name), "rotX": 0, "rotY": 0, "rotZ": 0, "height": height,
		"triangles": src.Triangles(), "warnings": []string{}, "autoIcon": false})
	lib.Meta.Layouts[d.ID] = layout
	return nil
}

// unswizzleNormal turns a normal map stored the way Unity packs them (DXT5nm: x in alpha, y in green, red/blue unused) back into
// a plain RGB normal map; ordinary normal maps (blue ≈ 1) are kept.
func unswizzleNormal(img *image.NRGBA) *image.NRGBA {
	var blue, n float64
	alphaVaries := false
	pix := img.Pix
	for i := 0; i+3 < len(pix); i += 4 * 17 {
		blue += float64(pix[i+2])
		n++
		if pix[i+3] != 255 {
			alphaVaries = true
		}
	}
	if n == 0 || blue/n > 128 || !alphaVaries {
		return img
	}
	out := image.NewNRGBA(img.Bounds())
	for i := 0; i+3 < len(pix); i += 4 {
		x := float64(pix[i+3])/127.5 - 1
		y := float64(pix[i+1])/127.5 - 1
		z := math.Sqrt(math.Max(0, 1-x*x-y*y))
		out.Pix[i] = pix[i+3]
		out.Pix[i+1] = pix[i+1]
		out.Pix[i+2] = uint8(math.Round((z + 1) * 127.5))
		out.Pix[i+3] = 255
	}
	return out
}

package setfmt

// Decorations (accessory library "decorations"): mirrors DecorationDef in the mod's Core/Defs.cs and
// AccessoryLoader.LoadDecorations. See docs/set-format.md "Decorations".

import (
	"fmt"
	"os"
	"strings"
)

// DecorationKind describes one kind of decoration for the Studio page (tabs) and validation.
type DecorationKind struct {
	Kind    string `json:"kind"`    // Wall | Floor | Ceiling | Poster | Object (JSON value in accessories.json)
	Title   string `json:"title"`   // "Walls"
	One     string `json:"one"`     // "wall look"
	Toggle  string `json:"toggle"`  // mod config key in [Content - Decorations] that hides the vanilla ones
	Surface bool   `json:"surface"` // a tiling look for the shop's walls/floor/ceiling (else a placeable model)
}

// DecorationToggleSection is the mod config section of the hide-vanilla toggles.
const DecorationToggleSection = "Content - Decorations"

var DecorationKinds = []DecorationKind{
	{Kind: "Wall", Title: "Walls", One: "wall look", Toggle: "ShowVanillaSurfaces", Surface: true},
	{Kind: "Floor", Title: "Floors", One: "floor look", Toggle: "ShowVanillaSurfaces", Surface: true},
	{Kind: "Ceiling", Title: "Ceilings", One: "ceiling look", Toggle: "ShowVanillaSurfaces", Surface: true},
	{Kind: "Poster", Title: "Posters", One: "poster", Toggle: "ShowVanillaPosters"},
	{Kind: "Object", Title: "Objects", One: "decoration", Toggle: "ShowVanillaDecoObjects"},
}

func DecorationKindInfo(kind string) (DecorationKind, bool) {
	for _, k := range DecorationKinds {
		if k.Kind == kind {
			return k, true
		}
	}
	return DecorationKind{}, false
}

// Decoration is one entry of the library's "decorations" list.
type Decoration struct {
	ID    string  `json:"id"`
	Kind  string  `json:"kind"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Icon  string  `json:"icon,omitempty"`
	// Surface: tiling colour texture. Placeable: the model's texture.
	Texture string `json:"texture,omitempty"`
	// Surfaces only.
	NormalMap    string   `json:"normalMap,omitempty"`
	RoughnessMap string   `json:"roughnessMap,omitempty"` // metallic (R) / smoothness (A)
	Color        string   `json:"color,omitempty"`        // #RRGGBB multiplied onto the texture
	Smoothness   *float64 `json:"smoothness,omitempty"`   // 0–1, mod default 0.2
	// Placeable only: baked OBJ in the decoration's root space (wall: wall plane z = 0, faces +z; floor: stands on y = 0).
	Mesh  string `json:"mesh,omitempty"`
	Mount string `json:"mount,omitempty"` // Object: "Wall" | "Floor" (posters always hang on a wall)
	Tab   string `json:"tab,omitempty"`   // "Poster" | "Other" (default: Poster for posters, Other for objects)
}

func (d Decoration) IsSurface() bool {
	k, _ := DecorationKindInfo(d.Kind)
	return k.Surface
}

// EffectiveMount is where the decoration goes ("Wall" / "Floor"), as the mod reads it.
func (d Decoration) EffectiveMount() string {
	if d.Kind == "Poster" || d.Mount == "Wall" {
		return "Wall"
	}
	return "Floor"
}

// NewDecoration: vanilla-like starting prices (posters 150–1000, statues 6000+, looks 1000–10000).
func NewDecoration(kind, id, name string) Decoration {
	price := map[string]float64{"Wall": 2500, "Floor": 2500, "Ceiling": 2500, "Poster": 300, "Object": 1000}[kind]
	d := Decoration{ID: id, Kind: kind, Name: name, Price: price}
	if kind == "Object" {
		d.Mount = "Floor"
	}
	return d
}

// FileRefs points at every file field (texture, maps, icon, mesh), for code that lists, copies or moves its files.
func (d *Decoration) FileRefs() []*string {
	return []*string{&d.Texture, &d.NormalMap, &d.RoughnessMap, &d.Icon, &d.Mesh}
}

// Files lists the decoration's file paths (empty ones included).
func (d Decoration) Files() []string {
	var out []string
	for _, p := range d.FileRefs() {
		out = append(out, *p)
	}
	return out
}

func (d Decoration) Clone() Decoration {
	if d.Smoothness != nil {
		v := *d.Smoothness
		d.Smoothness = &v
	}
	return d
}

// ValidateDecorations mirrors AccessoryLoader.LoadDecorations: errors reject the decoration in game, warnings don't.
func (l *AccessoryLibrary) ValidateDecorations(resolve func(rel string) string) (errs, warns []string) {
	ids := map[string]bool{}
	exists := func(rel string) bool {
		if rel == "" {
			return false
		}
		_, err := os.Stat(resolve(rel))
		return err == nil
	}
	for i, d := range l.Decorations {
		where := fmt.Sprintf("decoration %q", d.ID)
		if strings.TrimSpace(d.ID) == "" {
			errs = append(errs, fmt.Sprintf("decoration #%d: missing id", i+1))
			continue
		}
		if strings.ContainsAny(d.ID, " \t:/|") {
			errs = append(errs, where+": id may not contain spaces, ':', '/' or '|'")
		}
		if ids[d.ID] {
			errs = append(errs, where+": duplicate id")
		}
		ids[d.ID] = true
		k, ok := DecorationKindInfo(d.Kind)
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: unknown kind %q", where, d.Kind))
			continue
		}
		if d.Price < 0 {
			errs = append(errs, where+": price must be ≥ 0")
		}
		if d.Icon != "" && !exists(d.Icon) {
			warns = append(warns, fmt.Sprintf("%s: icon not found %q", where, d.Icon))
		}
		if k.Surface {
			if !exists(d.Texture) {
				errs = append(errs, fmt.Sprintf("%s: texture not found %q", where, d.Texture))
			}
			for _, m := range []string{d.NormalMap, d.RoughnessMap} {
				if m != "" && !exists(m) {
					warns = append(warns, fmt.Sprintf("%s: map not found %q (left out)", where, m))
				}
			}
			if d.Color != "" && !tintRe.MatchString(d.Color) {
				errs = append(errs, fmt.Sprintf("%s: color %q is not a colour (#RRGGBB)", where, d.Color))
			}
			if d.Smoothness != nil && (*d.Smoothness < 0 || *d.Smoothness > 1) {
				errs = append(errs, where+": smoothness must be between 0 and 1")
			}
			continue
		}
		if !exists(d.Mesh) {
			errs = append(errs, fmt.Sprintf("%s: model not found %q", where, d.Mesh))
		}
		if d.Texture != "" && !exists(d.Texture) {
			warns = append(warns, fmt.Sprintf("%s: texture not found %q (white used)", where, d.Texture))
		}
		if d.Mount != "" && d.Mount != "Wall" && d.Mount != "Floor" {
			errs = append(errs, fmt.Sprintf("%s: mount %q must be Wall or Floor", where, d.Mount))
		}
		if d.Tab != "" && d.Tab != "Poster" && d.Tab != "Other" {
			errs = append(errs, fmt.Sprintf("%s: tab %q must be Poster or Other", where, d.Tab))
		}
	}
	return
}

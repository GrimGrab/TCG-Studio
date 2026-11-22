package setfmt

import (
	"fmt"
	"sort"
	"strings"
)

// BorderSlot is a set's own take on one border slot (set.json "borders"). Mirrors the mod's BorderSlotDef.
type BorderSlot struct {
	// Look is the library border of the slot's normal version; empty = the game's rim.
	Look string `json:"look,omitempty"`
	// FoilLook is the library border of the slot's foil version; nil = Look, "" = the game's rim.
	FoilLook *string `json:"foilLook,omitempty"`
}

// Border layer kinds.
const (
	BorderImageLayer  = "image"
	BorderShaderLayer = "shader"
)

// BorderStockPrefix starts the ids of borders that come with the mod (mod Runtime/CardBorders.StockPrefix).
const BorderStockPrefix = "tcgcc-border-"

// Border is a card border in the accessory library: a stack of layers drawn over a full-image card, back to front.
// Mirrors the mod's BorderDef.
type Border struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
	// Window is where the card's picture goes: x, y, width, height as fractions of the border from its top left. Empty =
	// the picture covers the whole card under the border.
	Window []float64     `json:"window,omitempty"`
	Layers []BorderLayer `json:"layers"`
}

// BorderLayer is an image (a card-shaped picture, see-through where the card shows) or a card shader drawn on the image
// layer under it (art() = that picture; the card picture when there is none), optionally clipped to its shape.
type BorderLayer struct {
	Kind    string               `json:"kind"`
	File    string               `json:"file,omitempty"`
	Opacity float64              `json:"opacity"`
	Shader  string               `json:"shader,omitempty"`
	Clip    bool                 `json:"clip,omitempty"`
	Params  map[string][]float64 `json:"params,omitempty"`
}

// FoilLookOf is the border of the slot's foil version (FoilLook, else Look).
func (b BorderSlot) FoilLookOf() string {
	if b.FoilLook != nil {
		return *b.FoilLook
	}
	return b.Look
}

// BorderOf is the library border a card's version is drawn with: the card's own entry, else the set's slot ("" = none).
func (s *Set) BorderOf(c *Card, border string, foil bool) string {
	key := border
	if foil {
		key += "_foil"
	}
	if c != nil {
		for k, v := range c.VersionBorders {
			if strings.EqualFold(k, key) {
				return v
			}
		}
	}
	for k, slot := range s.Borders {
		if strings.EqualFold(k, border) {
			if foil {
				return slot.FoilLookOf()
			}
			return slot.Look
		}
	}
	return ""
}

func isBorder(name string) bool {
	for _, b := range Borders {
		if strings.EqualFold(b, name) {
			return true
		}
	}
	return false
}

func sortedSlotKeys(m map[string]BorderSlot) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// BorderIDs lists the library borders the set and its cards name (each once, stable order; "" left out).
func (s *Set) BorderIDs() []string {
	var out []string
	seen := map[string]bool{}
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, k := range sortedSlotKeys(s.Borders) {
		add(s.Borders[k].Look)
		add(s.Borders[k].FoilLookOf())
	}
	for i := range s.Cards {
		for _, k := range sortedKeys(s.Cards[i].VersionBorders) {
			add(s.Cards[i].VersionBorders[k])
		}
	}
	return out
}

// RenameBorders points the set's and its cards' border choices at new ids (old id → new id).
func (s *Set) RenameBorders(renamed map[string]string) {
	if len(renamed) == 0 {
		return
	}
	for k, slot := range s.Borders {
		if to, ok := renamed[slot.Look]; ok {
			slot.Look = to
		}
		if slot.FoilLook != nil {
			if to, ok := renamed[*slot.FoilLook]; ok {
				v := to
				slot.FoilLook = &v
			}
		}
		s.Borders[k] = slot
	}
	for i := range s.Cards {
		for k, id := range s.Cards[i].VersionBorders {
			if to, ok := renamed[id]; ok {
				s.Cards[i].VersionBorders[k] = to
			}
		}
	}
}

// ShaderIDs lists the card shaders the border's shader layers use (each once).
func (b *Border) ShaderIDs() []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range b.Layers {
		if l.Kind == BorderShaderLayer && l.Shader != "" && !seen[l.Shader] {
			seen[l.Shader] = true
			out = append(out, l.Shader)
		}
	}
	return out
}

// ValidateBorders checks the library's borders (the mod's AccessoryLoader.LoadBorders skips what this refuses).
func ValidateBorders(list []Border) []Issue {
	var out []Issue
	add := func(where, format string, args ...any) {
		out = append(out, Issue{Level: "error", Where: where, Message: fmt.Sprintf(format, args...)})
	}
	ids := map[string]bool{}
	for i, b := range list {
		w := fmt.Sprintf("border %q", b.Name)
		if b.Name == "" {
			w = fmt.Sprintf("border #%d", i+1)
			add(w, "needs a name")
		}
		if b.ID == "" {
			add(w, "missing id")
		} else if ids[b.ID] {
			add(w, "duplicate id %q", b.ID)
		}
		ids[b.ID] = true
		if len(b.Window) != 0 && len(b.Window) != 4 {
			add(w, "window needs 4 numbers (x, y, width, height)")
		}
		for j, l := range b.Layers {
			lw := fmt.Sprintf("%s layer %d", w, j+1)
			switch l.Kind {
			case BorderImageLayer:
				if l.File == "" {
					add(lw, "no picture")
				}
			case BorderShaderLayer:
				if l.Shader == "" {
					add(lw, "no shader chosen")
				}
				for name, v := range l.Params {
					if n := len(v); n != 1 && n != 2 && n != 4 {
						add(lw, "%q needs 1, 2 or 4 numbers", name)
					}
				}
			default:
				add(lw, "unknown kind %q", l.Kind)
			}
		}
	}
	return out
}

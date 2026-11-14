package setfmt

import (
	"fmt"
	"strings"
)

// Card shaders (accessory library "shaders", content kind "shader"; the Shaders page): GLSL written by the player that
// draws a card's picture, given to card versions by a set's and a card's "versionShaders". Code and Params are what the
// player wrote; Bundle is what Studio compiled from them for the game (internal/cardshader), the only part of a shader
// besides the parameter values and images that the mod reads (Runtime/CardShaders).

// Shader parameter kinds (the same names as internal/cardshader's).
const (
	ShaderNumber = "number"
	ShaderVec2   = "vec2"
	ShaderColour = "colour"
	ShaderImage  = "image"
)

// What a shader draws on (Shader.Draws).
const (
	DrawsPicture    = ""           // the card's picture; the rest of the card stays as the game draws it
	DrawsSeeThrough = "seeThrough" // the picture, with the rest of the card front hidden: see-through parts show what is behind the card
	DrawsOver       = "over"       // on top of the whole card front (frame, text, stats); the picture under it stays
)

type Shader struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Code   string        `json:"code"`
	Params []ShaderParam `json:"params,omitempty"`
	// Draws: one of the Draws… values. Read by the mod.
	Draws string `json:"draws,omitempty"`
	// Mix, MixStrength: Shadertoy code only (a mainImage and no card function): how its picture is mixed with the card's
	// (internal/cardshader's Mix… values, "" = add) and how strongly (0 = fully). Compiled into the bundle.
	Mix         string  `json:"mix,omitempty"`
	MixStrength float64 `json:"mixStrength,omitempty"`
	// Bundle: the compiled shader ("assets/<hash>.shaderbundle"); "" = never compiled (the game draws the card normally).
	// Built tells which code and parameters it was compiled from.
	Bundle string `json:"bundle,omitempty"`
	Built  string `json:"built,omitempty"`
	Icon   string `json:"icon,omitempty"` // a frame of the preview, for lists
}

// ShaderParam is a value the code uses by name. In the game the n-th number/2D/colour parameter is value slot n and the
// n-th image parameter is image slot n.
type ShaderParam struct {
	Name  string    `json:"name"`
	Kind  string    `json:"kind"`
	Value []float64 `json:"value,omitempty"` // number: 1 value, vec2: 2, colour: 4 (rgba, 0..1)
	Min   float64   `json:"min,omitempty"`   // number: the slider's range in Studio
	Max   float64   `json:"max,omitempty"`
	Image string    `json:"image,omitempty"` // image: the picture (shared store)
}

// ShaderValueCount is how many numbers a parameter of a kind holds (0 for images and unknown kinds).
func ShaderValueCount(kind string) int {
	return map[string]int{ShaderNumber: 1, ShaderVec2: 2, ShaderColour: 4}[kind]
}

// ValidateShaders: errors refuse a save. Parameter names and counts are checked where the code is compiled.
func (l *AccessoryLibrary) ValidateShaders() (errs []string) {
	ids := map[string]bool{}
	for i, s := range l.Shaders {
		where := fmt.Sprintf("shader %q", s.Name)
		switch {
		case !SafeID(s.ID):
			errs = append(errs, fmt.Sprintf("shader #%d: bad id", i+1))
			continue
		case ids[s.ID]:
			errs = append(errs, where+": duplicate id")
			continue
		case strings.TrimSpace(s.Name) == "":
			errs = append(errs, fmt.Sprintf("shader #%d: no name", i+1))
		}
		ids[s.ID] = true
		if s.Draws != DrawsPicture && s.Draws != DrawsSeeThrough && s.Draws != DrawsOver {
			errs = append(errs, fmt.Sprintf("%s: unknown \"draws\" %q", where, s.Draws))
		}
		for _, p := range s.Params {
			n := ShaderValueCount(p.Kind)
			switch {
			case p.Kind != ShaderImage && n == 0:
				errs = append(errs, fmt.Sprintf("%s: parameter %q has an unknown kind %q", where, p.Name, p.Kind))
			case p.Kind != ShaderImage && len(p.Value) != n:
				errs = append(errs, fmt.Sprintf("%s: parameter %q needs %d value(s)", where, p.Name, n))
			}
		}
	}
	return errs
}

// Package cardshader turns a card shader written by a player (GLSL, one function "vec4 card(vec2 uv)") into a shader
// bundle the game can load. The game cannot compile shaders, so Studio does it:
//
//	player's GLSL + prelude  ->  HLSL (glslang + SPIRV-Cross, glsl2hlsl.wasm)  ->  DXBC (Direct3D's compiler)
//	->  a copy of the template bundle with its pixel programs swapped (patch.go)
//
// The template is shaders\Assets\CardShader\TCGCC_CardShader.shader, built by the Unity editor. The prelude below and
// that file describe the same inputs, constants and textures; change them together.
package cardshader

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// TemplateShader is the name of the shader in the template bundle (and in every bundle made from it).
const TemplateShader = "TCGCC/CardShader"

const (
	MaxValues = 16 // value slots (_P) a shader's parameters can use
	MaxImages = 4  // image slots (_Tex0.._Tex3)
)

// Parameter kinds.
const (
	KindNumber = "number" // float
	KindVec2   = "vec2"
	KindColour = "colour" // vec4, rgba
	KindImage  = "image"  // sampler2D
)

// Param is a value the shader's author declared: usable by name in the code, set in Studio.
type Param struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// Slot is where a parameter lives in the template: a value slot (_P[n]) or an image slot (_Tex<n>).
type Slot struct {
	Param
	Index int `json:"index"`
}

var identifier = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// Names the prelude or GLSL itself already use.
var taken = map[string]bool{"card": true, "art": true, "main": true, "iTime": true, "iTilt": true, "iFacing": true, "iResolution": true,
	"mainImage": true, "iMouse": true, "iChannel0": true, "iChannel1": true, "iChannel2": true, "iChannel3": true, "iChannelResolution": true,
	"iFrame": true, "iTimeDelta": true, "iFrameRate": true, "iDate": true,
	"iSeed": true, "iCard": true, "iShown": true, "iHover": true, "iBorder": true, "iFoil": true, "iRarity": true, "iGrade": true,
	"cardGlow": true, "cardMask": true,
	"uv": true, "float": true, "int": true, "bool": true, "vec2": true, "vec3": true, "vec4": true, "mat2": true, "mat3": true, "mat4": true,
	"sampler2D": true, "texture": true, "in": true, "out": true, "inout": true, "uniform": true, "const": true, "if": true, "else": true,
	"for": true, "while": true, "do": true, "return": true, "break": true, "continue": true, "discard": true, "struct": true, "void": true,
	"true": true, "false": true, "precision": true, "highp": true, "mediump": true, "lowp": true, "layout": true, "switch": true, "case": true,
	"default": true}

// Slots gives every parameter its slot, in order, or says what is wrong with the list.
func Slots(params []Param) ([]Slot, error) {
	var out []Slot
	values, images := 0, 0
	seen := map[string]bool{}
	for _, p := range params {
		switch {
		case !identifier.MatchString(p.Name) || strings.Contains(p.Name, "__") || strings.HasPrefix(p.Name, "gl_"):
			return nil, fmt.Errorf("parameter name %q: use letters, digits and single underscores, starting with a letter", p.Name)
		case taken[p.Name]:
			return nil, fmt.Errorf("parameter name %q is already used by the shader language", p.Name)
		case seen[p.Name]:
			return nil, fmt.Errorf("two parameters are named %q", p.Name)
		}
		seen[p.Name] = true
		switch p.Kind {
		case KindNumber, KindVec2, KindColour:
			out = append(out, Slot{p, values})
			values++
		case KindImage:
			out = append(out, Slot{p, images})
			images++
		default:
			return nil, fmt.Errorf("parameter %q: unknown kind %q", p.Name, p.Kind)
		}
	}
	if values > MaxValues {
		return nil, fmt.Errorf("a shader can have %d number, 2D and colour parameters; this one has %d", MaxValues, values)
	}
	if images > MaxImages {
		return nil, fmt.Errorf("a shader can have %d image parameters; this one has %d", MaxImages, images)
	}
	return out, nil
}

// The inputs as the game's template declares them: registers and TEXCOORD numbers come from the layout numbers.
const gameInputs = `layout(std140, binding = 0) uniform TCGCCCard { vec4 _TextureSampleAdd; vec4 _ClipRect; vec4 _P[16]; };
layout(binding = 0) uniform sampler2D _MainTex;
layout(binding = 1) uniform sampler2D _Tex0;
layout(binding = 2) uniform sampler2D _Tex1;
layout(binding = 3) uniform sampler2D _Tex2;
layout(binding = 4) uniform sampler2D _Tex3;
layout(binding = 5) uniform sampler2D _Mask;
layout(location = 0) in vec4 _uvWorld;
layout(location = 1) in vec4 _view;
layout(location = 2) in vec4 _art;
layout(location = 3) in vec4 _misc;
layout(location = 4) in vec4 _card;
layout(location = 5) in vec4 _facts;
layout(location = 0) out vec4 _colour;
`

// The same inputs for Studio's preview (WebGL 2): plain uniforms set by the preview, one varying.
const previewInputs = `uniform vec4 _TextureSampleAdd;
uniform vec4 _P[16];
uniform sampler2D _MainTex;
uniform sampler2D _Tex0;
uniform sampler2D _Tex1;
uniform sampler2D _Tex2;
uniform sampler2D _Tex3;
uniform sampler2D _Mask;
uniform vec4 _view;
uniform vec4 _art;
uniform vec4 _misc;
uniform vec4 _card;
uniform vec4 _facts;
in vec4 _uvWorld;
out vec4 _colour;
`

// What a shader's author can use, the same in the game and in the preview.
const authorAPI = `#define iTime (_view.w)
#define iTilt (_view.xy)
#define iFacing (_view.z)
#define iResolution vec3(_misc.yz, 1.0)
// About the card being drawn. iSeed: a random number (0..1) of this copy on screen, so copies side by side don't move in
// step; iCard: a random number of the card itself, the same for every copy. iShown: seconds since the card appeared;
// iHover: seconds the pointer has been on it (0 while it is not). iRarity: the rarity's place in the set's list (0 = first).
// iGrade: 0 = not graded, else 1..10.
#define iSeed (_card.x)
#define iCard (_card.y)
#define iShown (_card.z)
#define iHover (_card.w)
#define iBorder int(_facts.x + 0.5)
#define iFoil (_facts.y > 0.5)
#define iRarity int(_facts.z + 0.5)
#define iGrade int(_facts.w + 0.5)
// Light added on top of whatever card() returns, also where that is see-through: add to it inside card().
vec3 cardGlow = vec3(0.0);
// What Shadertoy shaders expect, so one pasted from shadertoy.com runs as it is: the first channel is the card's picture, the
// others are the shader's picture parameters in order, and the mouse is where the card's tilt points.
#define iChannel0 _MainTex
#define iChannel1 _Tex0
#define iChannel2 _Tex1
#define iChannel3 _Tex2
#define iChannelResolution vec3[4](vec3(_misc.yz, 1.0), vec3(256.0, 256.0, 1.0), vec3(256.0, 256.0, 1.0), vec3(256.0, 256.0, 1.0))
#define iMouse vec4((_view.xy + 0.5) * _misc.yz, 0.0, 0.0)
#define iFrame int(_view.w * 60.0)
#define iTimeDelta (1.0 / 60.0)
#define iFrameRate 60.0
#define iDate vec4(2026.0, 1.0, 1.0, _view.w)
vec4 art(vec2 uv) { return texture(_MainTex, _art.xy + clamp(uv, 0.0, 1.0) * _art.zw) + _TextureSampleAdd; }
// The card's own mask picture (set.json "shaderMask"): 1 where effects belong, 0 where they don't; 1 everywhere on a card
// without one. The mask is painted over the whole card picture, so it is looked up where the art is.
float cardMask(vec2 uv) { return texture(_Mask, _art.xy + clamp(uv, 0.0, 1.0) * _art.zw).r; }
`

const tail = `
void main() {
    vec4 c = card(_uvWorld.xy);
    c.a = clamp(c.a, 0.0, 1.0);
    // Blending is "source + destination * (1 - alpha)": the glow is light with no alpha of its own.
    c.rgb = c.rgb * c.a + max(cardGlow, vec3(0.0));
    c *= _misc.x;
    // Every input is read here, so the converted shader keeps them all, in the template's order.
    c.rgb += (_view.x + _art.x + _card.x + _facts.x) * 0.0;
#ifdef UNITY_UI_CLIP_RECT
    vec2 inside = step(_ClipRect.xy, _uvWorld.zw) * step(_uvWorld.zw, _ClipRect.zw);
    c *= inside.x * inside.y;
#endif
#ifdef UNITY_UI_ALPHACLIP
    if (c.a < 0.001) discard;
#endif
    _colour = c;
}
`

// How a Shadertoy shader's picture is mixed with the card's (Options.Mix).
const (
	MixAdd      = "add"      // its light on top of the picture (the default)
	MixScreen   = "screen"   // like add, but never brighter than white
	MixMultiply = "multiply" // darkens the picture
	MixOverlay  = "overlay"  // brightens the bright parts, darkens the dark
	MixReplace  = "replace"  // only the Shadertoy picture shows
)

var mixCode = map[string]string{
	MixAdd:      "p + fx",
	MixScreen:   "1.0 - (1.0 - p) * (1.0 - fx)",
	MixMultiply: "p * fx",
	MixOverlay:  "mix(2.0 * p * fx, 1.0 - 2.0 * (1.0 - p) * (1.0 - fx), step(0.5, p))",
	MixReplace:  "fx",
}

// Options are a shader's settings that change the compiled code.
type Options struct {
	// Mix and Strength: Shadertoy shaders only (IsShadertoy). "" = MixAdd; 0 = full strength.
	Mix      string  `json:"mix"`
	Strength float64 `json:"strength"`
}

// Check says what is wrong with the options.
func (o Options) Check() error {
	if o.Mix != "" && mixCode[o.Mix] == "" {
		return fmt.Errorf("unknown mix %q", o.Mix)
	}
	if o.Strength < 0 || o.Strength > 1 {
		return fmt.Errorf("mix strength %v: must be between 0 and 1", o.Strength)
	}
	return nil
}

// shadertoyBridge goes between the code and the tail when the code is a Shadertoy shader (IsShadertoy): its picture
// is mixed with the card's, in the card's shape, where the card's mask allows.
func shadertoyBridge(o Options) string {
	mix, strength := o.Mix, o.Strength
	if mixCode[mix] == "" {
		mix = MixAdd
	}
	if strength <= 0 {
		strength = 1
	}
	return fmt.Sprintf(`
vec4 card(vec2 uv) {
    vec4 colour = vec4(0.0, 0.0, 0.0, 1.0);
    mainImage(colour, uv * _misc.yz);
    vec4 picture = art(uv);
    vec3 p = picture.rgb;
    vec3 fx = clamp(colour.rgb, 0.0, 1.0);
    return vec4(mix(p, %s, %s * cardMask(uv)), picture.a);
}
`, mixCode[mix], strconv.FormatFloat(strength, 'f', 4, 64))
}

var (
	hasMainImage = regexp.MustCompile(`\bvoid\s+mainImage\s*\(`)
	hasCard      = regexp.MustCompile(`\bvec4\s+card\s*\(`)
)

// IsShadertoy reports code written for shadertoy.com: it has mainImage(...) and no card(...) of its own. (The page's
// preview decides the same way: frontend lib/cardShaderView.ts isShadertoy.)
func IsShadertoy(code string) bool {
	return hasMainImage.MatchString(code) && !hasCard.MatchString(code)
}

// Wrapping is the text put before and after a shader's code to make a complete fragment shader. The code's own lines
// keep their numbers in compiler messages. Bridge goes right after the code when it is a Shadertoy shader.
type Wrapping struct {
	Head   string `json:"head"`
	Tail   string `json:"tail"`
	Bridge string `json:"bridge"`
}

// around returns the complete shader for a piece of code.
func (w Wrapping) around(code string) string {
	if IsShadertoy(code) {
		return w.Head + code + "\n" + w.Bridge + w.Tail
	}
	return w.Head + code + "\n" + w.Tail
}

func wrapping(version, inputs string, slots []Slot, keywords []string, o Options) Wrapping {
	var b strings.Builder
	b.WriteString(version + "\nprecision highp float;\nprecision highp int;\n")
	for _, k := range keywords {
		b.WriteString("#define " + k + " 1\n")
	}
	b.WriteString(inputs + authorAPI)
	for _, s := range slots {
		switch s.Kind {
		case KindNumber:
			fmt.Fprintf(&b, "#define %s (_P[%d].x)\n", s.Name, s.Index)
		case KindVec2:
			fmt.Fprintf(&b, "#define %s (_P[%d].xy)\n", s.Name, s.Index)
		case KindColour:
			fmt.Fprintf(&b, "#define %s (_P[%d])\n", s.Name, s.Index)
		case KindImage:
			fmt.Fprintf(&b, "#define %s _Tex%d\n", s.Name, s.Index)
		}
	}
	b.WriteString("#line 1\n")
	return Wrapping{Head: b.String(), Tail: tail, Bridge: shadertoyBridge(o)}
}

// PreviewWrapping is the wrapping Studio's WebGL preview compiles a shader with.
func PreviewWrapping(params []Param, o Options) (Wrapping, error) {
	slots, err := Slots(params)
	if err == nil {
		err = o.Check()
	}
	if err != nil {
		return Wrapping{}, err
	}
	return wrapping("#version 300 es", previewInputs, slots, nil, o), nil
}

// gameHLSL converts a shader to the HLSL of one keyword variant of the template.
func gameHLSL(source string, slots []Slot, keywords []string, o Options) (string, error) {
	w := wrapping("#version 310 es", gameInputs, slots, keywords, o)
	return toHLSL(w.around(source))
}

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}

// sampleLevels rewrites every "tex.Sample(sampler, uv)" of converted HLSL to "tex.SampleLevel(sampler, uv, 0.0)".
func sampleLevels(hlsl string) string {
	const call = ".Sample("
	var b strings.Builder
	for {
		i := strings.Index(hlsl, call)
		if i < 0 {
			break
		}
		depth, end := 1, -1
		for j := i + len(call); j < len(hlsl); j++ {
			switch hlsl[j] {
			case '(':
				depth++
			case ')':
				depth--
			}
			if depth == 0 {
				end = j
				break
			}
		}
		if end < 0 {
			break
		}
		b.WriteString(hlsl[:i])
		b.WriteString(".SampleLevel(")
		b.WriteString(hlsl[i+len(call) : end])
		b.WriteString(", 0.0)")
		hlsl = hlsl[end+1:]
	}
	b.WriteString(hlsl)
	return b.String()
}

// Build makes the game's bundle for a shader from the template bundle. Mistakes in the shader's code come back as a
// *SourceError with the code's own line numbers.
func Build(template []byte, id, source string, params []Param, o Options) ([]byte, error) {
	slots, err := Slots(params)
	if err == nil {
		err = o.Check()
	}
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte("tcgcc card shader " + id))
	return swapPrograms(template, TemplateShader, "CAB-"+hex.EncodeToString(sum[:16]), func(keywords []string) ([]byte, error) {
		hlsl, err := gameHLSL(source, slots, keywords, o)
		if err != nil {
			return nil, err
		}
		dxbc, err := compileHLSL(hlsl, "main", "ps_4_0", nil)
		if err != nil && strings.Contains(err.Error(), "gradient") {
			// Direct3D refuses a plain texture lookup inside a loop whose length it can't know (WebGL allows it; common in
			// Shadertoy shaders). Looking the texture up at its full size instead is what such code means.
			dxbc, err = compileHLSL(sampleLevels(hlsl), "main", "ps_4_0", nil)
		}
		if err != nil {
			return nil, &SourceError{Messages: "ERROR: the game's graphics compiler (Direct3D) refused this shader, though the preview runs it: " + firstLine(err.Error())}
		}
		return dxbc, nil
	})
}

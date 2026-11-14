package cardshader

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"strings"
)

// Stock shaders: the holo foils that come with the mod, one per border. They are card shaders like the player's own,
// written in the same language, and are what a foil version without a shader of its own is drawn with ([Foil] HoloFoil).
// They share one piece of code and differ in their parameter values, so the mod carries one compiled bundle for all.
//
// This file is the source. stock/stock.json (the list as the mod and Studio's pages read it) and stock/tcgcc_stock (the
// compiled bundle) are made from it by `go run ./cmd/stockshaders` and committed; both are built into the mod's DLL.
// TestStockFilesCurrent fails when they are out of step.

// StockPrefix starts every stock shader's id; no player shader has such an id (theirs are random).
const StockPrefix = "tcgcc-holo-"

// StockShader is one stock shader.
type StockShader struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Border string         `json:"border"` // the border whose foil version it is the default of
	Code   string         `json:"code"`
	Params []StarterParam `json:"params"`
}

// IsStock reports a stock shader's id.
func IsStock(id string) bool { return strings.HasPrefix(id, StockPrefix) }

const holoCode = `// Holo foil: bands of colour that drift over the picture, a light sweep that crosses the card, and an
// optional pattern (sparkles, etched lines or a cosmos of clouds and stars).
//   artMask   0 = everywhere, 1 = stronger on bright parts of the picture, 2 = stronger on dark parts
//   pattern   0 = none, 1 = sparkle, 2 = etched lines, 3 = cosmos
//   viewReact 0 = the look doesn't change when you move around the card, 1 = it shifts with the view
float hash21(vec2 p) {
    p = fract(p * vec2(123.34, 456.21));
    p += dot(p, p + 45.32);
    return fract(p.x * p.y);
}

float valueNoise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    vec2 u = f * f * (3.0 - 2.0 * f);
    float a = hash21(i);
    float b = hash21(i + vec2(1.0, 0.0));
    float c = hash21(i + vec2(0.0, 1.0));
    float d = hash21(i + vec2(1.0, 1.0));
    return mix(mix(a, b, u.x), mix(c, d, u.x), u.y);
}

vec3 rgb2hsv(vec3 c) {
    vec4 k = vec4(0.0, -1.0 / 3.0, 2.0 / 3.0, -1.0);
    vec4 p = mix(vec4(c.bg, k.wz), vec4(c.gb, k.xy), step(c.b, c.g));
    vec4 q = mix(vec4(p.xyw, c.r), vec4(c.r, p.yzx), step(p.x, c.r));
    float d = q.x - min(q.w, q.y);
    return vec3(abs(q.z + (q.w - q.y) / (6.0 * d + 1e-10)), d / (q.x + 1e-10), q.x);
}

vec3 hsv2rgb(vec3 c) {
    vec3 p = abs(fract(c.xxx + vec3(1.0, 2.0 / 3.0, 1.0 / 3.0)) * 6.0 - 3.0);
    return c.z * mix(vec3(1.0), clamp(p - 1.0, 0.0, 1.0), c.y);
}

// The foil's colours around the tint: the hue wanders by hueSpread, brightness and saturation pulse.
vec3 ramp(float t) {
    vec3 hsv = rgb2hsv(tint.rgb);
    float a = t * 6.2831853;
    float hue = fract(hsv.x + hueSpread * sin(a));
    float sat = clamp(hsv.y * saturation * (0.75 + 0.25 * cos(a * 2.0)), 0.0, 1.0);
    float val = clamp(max(hsv.z, 0.35) * (0.55 + 0.45 * (0.5 + 0.5 * sin(a * 2.0 + 1.0))), 0.0, 1.0);
    return hsv2rgb(vec3(hue, sat, val));
}

// Twinkling points in a jittered grid.
float sparkles(vec2 uv, vec2 tilt, float scale, float amount) {
    vec2 grid = uv * vec2(scale, scale * 1.4);
    vec2 cell = floor(grid);
    float h = hash21(cell);
    float h2 = hash21(cell + 17.13);
    vec2 centre = vec2(0.2 + 0.6 * h, 0.2 + 0.6 * h2);
    float star = clamp(1.0 - length(fract(grid) - centre) * 6.0, 0.0, 1.0);
    star = star * star * star;
    float present = step(1.0 - amount * 0.45, hash21(cell + 3.7));
    float phase = h * 43.0 + dot(tilt, vec2(21.0, 13.0)) + iTime * (0.6 + h2);
    return star * present * pow(clamp(sin(phase) * 0.5 + 0.5, 0.0, 1.0), 6.0);
}

vec4 card(vec2 uv) {
    vec4 picture = art(uv);
    vec2 tilt = iTilt * viewReact;

    // Bands: a place on the colour ramp from the position, the view and a little noise, drifting with time.
    float distortion = valueNoise(uv * 5.0) * 0.6 + valueNoise(uv * 13.0) * 0.4;
    float coord = (uv.x * 0.6 + uv.y) * tiling + (tilt.x * 0.9 + tilt.y * 0.6) * parallax
        + distortion * 0.25 + iTime * 0.06 * motion;
    vec3 holo = ramp(fract(coord));
    float band = pow(clamp(sin(coord * 6.2831853 * 1.5) * 0.5 + 0.5, 0.0, 1.0), 2.0);
    float intensity = mix(1.0, 0.25 + 0.75 * band, bands);

    float lum = dot(picture.rgb, vec3(0.299, 0.587, 0.114));
    float mask = 1.0;
    if (artMask > 0.5 && artMask < 1.5) mask = smoothstep(0.3, 0.85, lum);
    else if (artMask > 1.5) mask = 1.0 - smoothstep(0.15, 0.6, lum);

    vec3 col = holo * intensity * mask;
    vec3 extra = vec3(0.0);
    if (pattern > 0.5 && pattern < 1.5) {
        extra += sparkles(uv, tilt, patternScale, sparkle) * (holo + 0.6);
    } else if (pattern > 1.5 && pattern < 2.5) {
        float lines = fract((uv.x + uv.y * 1.4) * patternScale * 0.25);
        extra += smoothstep(0.85, 1.0, lines) * (0.4 + 0.6 * band) * holo * 1.2;
        extra += sparkles(uv, tilt, patternScale * 0.5, sparkle * 0.5) * (holo + 0.5);
    } else if (pattern > 2.5) {
        float cloud = valueNoise(uv * 3.0 + tilt * 0.8) * valueNoise(uv * 7.0 - tilt * 0.5);
        col *= 0.6 + 1.2 * cloud;
        extra += sparkles(uv, tilt, patternScale, sparkle) * (holo + 0.8);
        extra += sparkles(uv + 0.37, tilt * 1.3, patternScale * 1.7, sparkle * 0.7) * holo;
    }

    // A glow at grazing angles, and a soft light that crosses the card again and again.
    float glow = pow(1.0 - iFacing, 2.0) * edgeGlow * viewReact;
    float sweepAt = fract((uv.x * 0.6 + uv.y) * 0.5 - iTime * 0.15 * motion);
    float d = (sweepAt - 0.5) / 0.07;
    vec3 sweepLight = (holo * 0.7 + 0.3) * exp(-d * d) * sweep * mix(0.5, 1.0, mask);

    // A card with a mask picture has the foil only where the mask is white.
    vec3 light = (col + extra + holo * glow + sweepLight) * strength * cardMask(uv);
    return vec4(picture.rgb + light, picture.a);
}
`

// holoLook is one border's values, in the order of the parameters below.
type holoLook struct {
	border, name                                string
	r, g, b                                     int
	hue, sat, strength, tiling, parallax, bands float64
	mask, pattern, sparkle, patternScale, edge  float64
}

// The six looks (the mod's holo foil as tuned in game, 2026-09-24).
var holoLooks = []holoLook{
	{"Base", "Dark teal", 0x37, 0x67, 0x61, 0.00, 0.3, 1.50, 1.2, 1.2, 0.7, 0, 0, 0.2, 60, 0.10},
	{"FirstEdition", "Cyan", 0x00, 0xF3, 0xFF, 0.50, 0.9, 1.50, 1.4, 1.6, 0.5, 1, 0, 0.4, 70, 0.20},
	{"Silver", "Brushed silver", 0xC7, 0xD9, 0xF7, 0.04, 0.5, 0.29, 1.0, 1.8, 0.8, 0, 2, 0.3, 90, 0.25},
	{"Gold", "Gold leaf", 0xFF, 0xD9, 0x2E, 0.05, 1.0, 0.31, 1.2, 1.6, 0.7, 0, 2, 0.5, 80, 0.30},
	{"EX", "Emerald", 0x00, 0xFF, 0x67, 0.08, 1.0, 1.50, 1.5, 1.8, 0.6, 1, 0, 0.5, 70, 0.30},
	{"FullArt", "Violet", 0x9E, 0x6A, 0xFF, 0.15, 1.0, 0.30, 1.3, 1.6, 0.5, 0, 1, 0.8, 50, 0.35},
}

var borderLabels = map[string]string{"Base": "Base", "FirstEdition": "First Edition", "Silver": "Silver", "Gold": "Gold", "EX": "EX", "FullArt": "Full Art"}

// Stock returns the stock shaders, in border order.
func Stock() []StockShader {
	num := func(name string, v, min, max float64) StarterParam {
		return StarterParam{Param: Param{name, KindNumber}, Value: []float64{v}, Min: min, Max: max}
	}
	var out []StockShader
	for _, l := range holoLooks {
		out = append(out, StockShader{
			ID: StockPrefix + strings.ToLower(l.border), Name: "Holo: " + l.name + " (" + borderLabels[l.border] + " foil)", Border: l.border, Code: holoCode,
			Params: []StarterParam{
				{Param: Param{"tint", KindColour}, Value: []float64{round3(float64(l.r) / 255), round3(float64(l.g) / 255), round3(float64(l.b) / 255), 1}},
				num("hueSpread", l.hue, 0, 0.5), num("saturation", l.sat, 0, 1), num("strength", l.strength, 0, 1.5),
				num("tiling", l.tiling, 0.2, 4), num("parallax", l.parallax, 0, 3), num("bands", l.bands, 0, 1),
				num("artMask", l.mask, 0, 2), num("pattern", l.pattern, 0, 3), num("sparkle", l.sparkle, 0, 1),
				num("patternScale", l.patternScale, 10, 150), num("edgeGlow", l.edge, 0, 1), num("viewReact", 0, 0, 1),
				num("sweep", 0.6, 0, 2), num("motion", 1.49, 0, 3),
			},
		})
	}
	return out
}

func round3(v float64) float64 { return float64(int(v*1000+0.5)) / 1000 }

// StockJSON is stock/stock.json: the list for the mod and for Studio's pages.
func StockJSON() []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	_ = enc.Encode(Stock())
	return b.Bytes()
}

// StockBundle compiles the stock shaders' code into the bundle the mod carries (all share the code and the
// parameters' names and kinds, so one bundle serves them all).
func StockBundle(template []byte) ([]byte, error) {
	s := Stock()[0]
	params := make([]Param, len(s.Params))
	for i, p := range s.Params {
		params[i] = p.Param
	}
	return Build(template, "stock", s.Code, params, Options{})
}

//go:embed stock/stock.json
var stockJSONFile []byte

//go:embed stock/tcgcc_stock
var stockBundleFile []byte

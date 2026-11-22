package cardshader

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Stock shaders: the holo foils that come with the mod, one per border. They are card shaders like the player's own,
// written in the same language, and are what a foil version without a shader of its own is drawn with ([Foil] HoloFoil).
// The holo looks share one piece of code and differ in their parameter values, so they share one compiled bundle; a
// stock shader with code of its own (the EX foil's emboss) has a bundle of its own.
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
	Border string         `json:"border"` // the border whose foil version it is the default of ("" = none: a card border's shader)
	Code   string         `json:"code"`
	Params []StarterParam `json:"params"`
	Bundle string         `json:"bundle"` // its compiled bundle: stock/<Bundle>, in the mod's DLL as TCGCustomCards.stockbundle.<Bundle>
}

// The stock shaders' bundles.
const (
	holoBundle   = "tcgcc_stock"
	embossBundle = "tcgcc_stock_emboss"
)

// IsStock reports a stock shader's id.
func IsStock(id string) bool { return strings.HasPrefix(id, StockPrefix) }

const holoCode = `float hash21(vec2 p) {
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

vec3 ramp(float t) {
    vec3 hsv = rgb2hsv(tint.rgb);
    float a = t * 6.2831853;
    float hue = fract(hsv.x + hueSpread * sin(a));
    float sat = clamp(hsv.y * saturation * (0.75 + 0.25 * cos(a * 2.0)), 0.0, 1.0);
    float val = clamp(max(hsv.z, 0.35) * (0.55 + 0.45 * (0.5 + 0.5 * sin(a * 2.0 + 1.0))), 0.0, 1.0);
    return hsv2rgb(vec3(hue, sat, val));
}

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
    float shine = 1.0;
    if (pattern > 0.5 && pattern < 1.5) {
        extra += sparkles(uv, tilt, patternScale, sparkle) * (holo + 0.6);
    } else if (pattern > 1.5 && pattern < 2.5) {
        float lines = fract((uv.x + uv.y * 1.4) * patternScale * 0.25);
        extra += smoothstep(0.85, 1.0, lines) * (0.4 + 0.6 * band) * holo * 1.2;
        extra += sparkles(uv, tilt, patternScale * 0.5, sparkle * 0.5) * (holo + 0.5);
    } else if (pattern > 3.5) {
        vec3 bronze = vec3(dot(holo, vec3(0.333))) * vec3(1.10, 0.92, 0.66);
        vec3 aged = mix(bronze, holo, 0.45);
        float grain = 0.7 + 0.3 * valueNoise(uv * 240.0) * valueNoise(uv * 97.0 + 5.1);
        float edge = min(min(uv.x, 1.0 - uv.x), min(uv.y, 1.0 - uv.y));
        float wear = smoothstep(0.0, 0.07, edge + (valueNoise(uv * 18.0) - 0.5) * 0.05)
            * (0.55 + 0.45 * smoothstep(0.25, 0.6, valueNoise(uv * 4.0 + 11.0)));
        col = aged * (0.35 + 0.65 * band) * mask * grain * wear;
        float scratch = 0.0;
        for (int k = 0; k < 2; k++) {
            float a = k == 0 ? 0.45 : -1.1;
            vec2 r = vec2(cos(a) * uv.x - sin(a) * uv.y, sin(a) * uv.x + cos(a) * uv.y) * vec2(3.0, patternScale);
            vec2 cell = floor(r);
            float on = step(0.82, hash21(cell + float(k) * 7.3));
            float across = abs(fract(r.y) - 0.5);
            scratch += on * (1.0 - smoothstep(0.02, 0.07, across)) * smoothstep(0.0, 0.3, fract(r.x)) * smoothstep(1.0, 0.7, fract(r.x));
        }
        extra += clamp(scratch, 0.0, 1.0) * vec3(0.55, 0.52, 0.46) * sparkle * (0.4 + 0.6 * band);
        shine = 0.3;
        picture.rgb = mix(picture.rgb, picture.rgb * vec3(1.04, 0.95, 0.78) + vec3(0.03, 0.02, 0.0), 0.6);
    } else if (pattern > 2.5) {
        float cloud = valueNoise(uv * 3.0 + tilt * 0.8) * valueNoise(uv * 7.0 - tilt * 0.5);
        col *= 0.6 + 1.2 * cloud;
        extra += sparkles(uv, tilt, patternScale, sparkle) * (holo + 0.8);
        extra += sparkles(uv + 0.37, tilt * 1.3, patternScale * 1.7, sparkle * 0.7) * holo;
    }

    float glow = pow(1.0 - iFacing, 2.0) * edgeGlow * viewReact * shine;
    float sweepAt = fract((uv.x * 0.6 + uv.y) * 0.5 - iTime * 0.15 * motion);
    float d = (sweepAt - 0.5) / 0.07;
    vec3 sweepLight = (holo * 0.7 + 0.3) * exp(-d * d) * sweep * shine * mix(0.5, 1.0, mask);

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
	view                                        float64 // viewReact
	motion                                      float64
}

// The holo looks (EX has the embossed foil instead) (the mod's holo foil as tuned in game, 2026-09-24).
var holoLooks = []holoLook{
	// Base and First Edition: the user's values, tuned on the Shaders page 2026-11-17.
	{"Base", "Dark teal", 0x37, 0x67, 0x61, 0.00, 0.3, 0.41, 2.882, 1.2, 0.7, 0, 0, 0.714, 67.06, 0.466, 0.273, 2.105},
	{"FirstEdition", "Antique", 0xD8, 0xB0, 0x70, 0.122, 0.412, 0.80, 1.046, 1.2, 1, 2, 4, 0, 150, 0.697, 0.6, 1.739},
	{"Silver", "Brushed silver", 0xC7, 0xD9, 0xF7, 0.04, 0.5, 0.29, 1.0, 1.8, 0.8, 0, 2, 0.3, 90, 0.25, 0.6, 1.49},
	{"Gold", "Gold leaf", 0xFF, 0xD9, 0x2E, 0.05, 1.0, 0.31, 1.2, 1.6, 0.7, 0, 2, 0.5, 80, 0.30, 0.6, 1.49},
	{"FullArt", "Violet", 0x9E, 0x6A, 0xFF, 0.15, 1.0, 0.30, 1.3, 1.6, 0.5, 0, 1, 0.8, 50, 0.35, 0.6, 1.49},
}

// Stock returns the stock shaders, in border order.
func Stock() []StockShader {
	order := map[string]int{"Base": 0, "FirstEdition": 1, "Silver": 2, "Gold": 3, "EX": 4, "FullArt": 5, "": 6} // borders' own shaders last
	num := func(name string, v, min, max float64) StarterParam {
		return StarterParam{Param: Param{name, KindNumber}, Value: []float64{v}, Min: min, Max: max}
	}
	var out []StockShader
	for _, l := range holoLooks {
		out = append(out, StockShader{
			ID: StockPrefix + strings.ToLower(l.border), Name: "Holo: " + l.name, Border: l.border, Code: holoCode, Bundle: holoBundle,
			Params: []StarterParam{
				{Param: Param{"tint", KindColour}, Value: []float64{round3(float64(l.r) / 255), round3(float64(l.g) / 255), round3(float64(l.b) / 255), 1}},
				num("hueSpread", l.hue, 0, 0.5), num("saturation", l.sat, 0, 1), num("strength", l.strength, 0, 1.5),
				num("tiling", l.tiling, 0.2, 4), num("parallax", l.parallax, 0, 3), num("bands", l.bands, 0, 1),
				num("artMask", l.mask, 0, 2), num("pattern", l.pattern, 0, 4), num("sparkle", l.sparkle, 0, 1),
				num("patternScale", l.patternScale, 10, 150), num("edgeGlow", l.edge, 0, 1), num("viewReact", l.view, 0, 1),
				num("sweep", 0.6, 0, 2), num("motion", l.motion, 0, 3),
			},
		})
	}
	out = append(out, embossStock(), leafStock())
	out = append(out, rimStocks()...)
	sort.SliceStable(out, func(i, j int) bool { return order[out[i].Border] < order[out[j].Border] })
	return out
}

// CleanStockValues keeps of a setup's own stock values (accessory library "stockValues") only what the mod can use and
// what differs from the stock shader: known stock shaders, their number/2D/colour parameters, the right count of values.
// Nil when nothing is left.
func CleanStockValues(in map[string]map[string][]float64) map[string]map[string][]float64 {
	out := map[string]map[string][]float64{}
	for _, s := range Stock() {
		given := in[s.ID]
		if len(given) == 0 {
			continue
		}
		own := map[string][]float64{}
		for _, p := range s.Params {
			v, ok := given[p.Name]
			n := map[string]int{KindNumber: 1, KindVec2: 2, KindColour: 4}[p.Kind]
			if !ok || n == 0 || len(v) != n || sameValues(v, p.Value) {
				continue
			}
			own[p.Name] = append([]float64(nil), v...)
		}
		if len(own) > 0 {
			out[s.ID] = own
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// WithValues returns the stock shader with a setup's own values (CleanStockValues' form, one shader's part) in place of its own.
func (s StockShader) WithValues(own map[string][]float64) StockShader {
	params := make([]StarterParam, len(s.Params))
	copy(params, s.Params)
	for i, p := range params {
		if v, ok := own[p.Name]; ok && len(v) == len(p.Value) {
			params[i].Value = append([]float64(nil), v...)
		}
	}
	s.Params = params
	return s
}

func sameValues(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if d := a[i] - b[i]; d > 1e-6 || d < -1e-6 {
			return false
		}
	}
	return true
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

// StockBundles compiles the stock shaders' code into the bundles the mod carries, by bundle name (shaders that share a
// bundle share the code and the parameters' names and kinds).
func StockBundles(template []byte) (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, s := range Stock() {
		if _, done := out[s.Bundle]; done {
			continue
		}
		params := make([]Param, len(s.Params))
		for i, p := range s.Params {
			params[i] = p.Param
		}
		id := "stock"
		if s.Bundle != holoBundle {
			id = s.Bundle // the holo bundle keeps its old id
		}
		b, err := Build(template, id, s.Code, params, Options{})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.Name, err)
		}
		out[s.Bundle] = b
	}
	return out, nil
}

//go:embed stock/stock.json
var stockJSONFile []byte

//go:embed stock/tcgcc_stock*
var stockBundleFiles embed.FS

package cardshader

// Two more shaders for the stock borders (internal/cardborder), like the leaf shimmer: drawn on a border's picture, keeping
// its alpha, no border's foil default (Border "").
//   - Rainbow rim: a rainbow that slides along the rim as the card tilts, over the picture's own shading, with a few
//     twinkling points (the Rainbow holo border).
//   - Gloss streak: one soft band of light that follows the tilt and drifts slowly, like a reflection on glass or polished
//     metal (the Obsidian and Etched silver borders).

const (
	rainbowBundle = "tcgcc_stock_rainbow"
	glossBundle   = "tcgcc_stock_gloss"
)

const rainbowCode = `vec3 rimHue(float h) {
    return clamp(abs(mod(h * 6.0 + vec3(0.0, 4.0, 2.0), 6.0) - 3.0) - 1.0, 0.0, 1.0);
}

vec4 card(vec2 uv) {
    vec4 rim = art(uv);
    // The colour at a point depends on where it is and on the angle the card is seen from, so the bands slide as it tilts.
    float h = (uv.x + uv.y * 0.6) * bands + iTilt.x * 2.0 - iTilt.y * 1.2 + iTime * speed * 0.05 + iCard;
    vec3 rainbow = mix(vec3(1.0), rimHue(fract(h)), saturation);
    float shade = dot(rim.rgb, vec3(0.299, 0.587, 0.114));
    vec3 col = mix(rim.rgb, rainbow * (0.35 + shade), strength);

    // Twinkles: a few cells light up and fade, each on its own beat.
    vec2 cell = floor(uv * vec2(90.0, 126.0));
    float r = fract(sin(dot(cell, vec2(12.9898, 78.233))) * 43758.5453);
    float twinkle = step(1.0 - sparkle * 0.05, r) * max(0.0, sin(iTime * 3.0 + r * 40.0));
    col += vec3(twinkle) * rim.a;
    return vec4(col, rim.a);
}
`

const glossCode = `vec4 card(vec2 uv) {
    vec4 base = art(uv);
    // Where the light falls: moved by the tilt, drifting a little by itself.
    vec2 light = vec2(0.5) + iTilt * 1.4 + vec2(sin(iTime * speed * 0.4 + iSeed * 6.0), cos(iTime * speed * 0.3)) * 0.15;
    float d = dot(uv - light, normalize(vec2(1.0, 0.7)));
    float streak = exp(-d * d * sharpness);
    return vec4(base.rgb + tint.rgb * streak * shine * base.a, base.a);
}
`

func rimStocks() []StockShader {
	num := func(name string, v, min, max float64) StarterParam {
		return StarterParam{Param: Param{name, KindNumber}, Value: []float64{v}, Min: min, Max: max}
	}
	return []StockShader{
		{
			ID: StockPrefix + "rainbowrim", Name: "Rainbow rim", Code: rainbowCode, Bundle: rainbowBundle,
			Params: []StarterParam{
				num("bands", 1.2, 0.2, 4), num("saturation", 0.75, 0, 1), num("strength", 0.7, 0, 1),
				num("sparkle", 0.5, 0, 1), num("speed", 1, 0, 3),
			},
		},
		{
			ID: StockPrefix + "gloss", Name: "Gloss streak", Code: glossCode, Bundle: glossBundle,
			Params: []StarterParam{
				{Param: Param{"tint", KindColour}, Value: []float64{1, 1, 1, 1}},
				num("shine", 0.6, 0, 1.5), num("sharpness", 40, 4, 200), num("speed", 1, 0, 3),
			},
		},
	}
}

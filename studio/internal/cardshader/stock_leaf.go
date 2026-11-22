package cardshader

// The metal leaf shimmer: the shader layer of the stock gold and silver leaf borders (internal/cardborder). It draws on
// the border's leaf picture: every flake of leaf catches the light at its own angle as the card tilts, and a soft sheen
// sweeps across now and then. It keeps the picture's alpha, so it stays on the leaf. No border's foil default (Border "").
// Flakes smaller than about a pixel (a card further away) fade to their average: hard-edged flakes switching on and off
// below pixel size flickered while the camera moved (2026-11-22).

const leafBundle = "tcgcc_stock_leaf"

const leafCode = `float leafHash(vec2 p) {
    p = fract(p * vec2(234.34, 435.345));
    p += dot(p, p + 34.23);
    return fract(p.x * p.y);
}

vec4 card(vec2 uv) {
    vec4 leaf = art(uv);

    // Flakes of leaf, laid in rows like real gilding: each a little brighter or darker, each tilted its own way.
    vec2 grid = vec2(flakes, flakes * 1.4);
    vec2 p = uv * grid;
    p.x += leafHash(vec2(floor(p.y), 3.0)) * 0.7;
    vec2 cell = floor(p);
    // How many flakes one screen pixel covers: 1 below about half a flake per pixel, 0 from a flake and a half.
    float footprint = max(fwidth(p.x), fwidth(p.y));
    float detail = 1.0 - smoothstep(0.5, 1.5, footprint);
    float tone = mix(0.5, leafHash(cell + iCard * 17.0), detail);
    vec2 facing = vec2(leafHash(cell + 3.1), leafHash(cell + 7.7)) - 0.5;

    // The light moves with the card's tilt and drifts slowly by itself.
    vec2 light = iTilt * 1.6 + vec2(sin(iTime * speed * 0.7 + iSeed * 6.0), cos(iTime * speed * 0.5)) * 0.3;
    float glint = mix(0.04, pow(max(0.0, 1.0 - length(facing - light * 0.5) * 2.2), 6.0), detail); // 0.04 ≈ its average

    // A broad sheen crossing the card.
    float band = uv.x * 0.8 + uv.y * 0.6 - fract(iTime * speed * 0.12 + iSeed) * 2.6 + 0.6 + iTilt.x;
    float sheen = exp(-band * band * 16.0);

    // The glint takes the leaf's own colour, lighter.
    vec3 glow = mix(vec3(1.0), leaf.rgb / max(0.001, max(leaf.r, max(leaf.g, leaf.b))), 0.55);
    vec3 col = leaf.rgb * (1.0 - variation * 0.5 + variation * tone) * tint.rgb;
    col += glow * (glint * shine + sheen * shine * 0.5) * leaf.a * cardMask(uv);
    return vec4(col, leaf.a);
}
`

func leafStock() StockShader {
	num := func(name string, v, min, max float64) StarterParam {
		return StarterParam{Param: Param{name, KindNumber}, Value: []float64{v}, Min: min, Max: max}
	}
	return StockShader{
		ID: StockPrefix + "leaf", Name: "Metal leaf shimmer", Code: leafCode, Bundle: leafBundle,
		Params: []StarterParam{
			{Param: Param{"tint", KindColour}, Value: []float64{1, 1, 1, 1}},
			num("flakes", 60, 4, 60), num("shine", 0.55, 0, 1.5), num("variation", 0.35, 0, 1), num("speed", 1, 0, 3),
		},
	}
}

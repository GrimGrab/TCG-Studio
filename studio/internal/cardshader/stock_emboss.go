package cardshader

// The EX foil: an embossed card. The picture is pressed into the card as a low relief, a fine diamond texture is
// stamped into the foil, and a light that circles slowly (and follows the view) catches the raised edges, its highlights
// split into a prism of colours. It has no tint: the card keeps its own colours.

const embossCode = `vec3 hsv2rgb(vec3 c) {
    vec3 p = abs(fract(c.xxx + vec3(1.0, 2.0 / 3.0, 1.0 / 3.0)) * 6.0 - 3.0);
    return c.z * mix(vec3(1.0), clamp(p - 1.0, 0.0, 1.0), c.y);
}

float luma(vec3 c) { return dot(c, vec3(0.299, 0.587, 0.114)); }

float diamonds(vec2 uv) {
    vec2 p = uv * vec2(diamondScale, diamondScale * 1.4);
    vec2 r = vec2(p.x + p.y, p.x - p.y) * 0.7071;
    vec2 f = abs(fract(r) - 0.5);
    return 1.0 - 2.0 * max(f.x, f.y);
}

float height(vec2 uv) {
    vec2 o = vec2(0.002, 0.0015);
    float pic = (luma(art(uv).rgb) * 2.0 + luma(art(uv + o).rgb) + luma(art(uv - o).rgb)) * 0.25;
    return pic * relief + diamonds(uv) * diamond * 0.12;
}

vec4 card(vec2 uv) {
    vec4 picture = art(uv);

    vec2 e = vec2(0.003, 0.0022);
    float gx = height(uv + vec2(e.x, 0.0)) - height(uv - vec2(e.x, 0.0));
    float gy = height(uv + vec2(0.0, e.y)) - height(uv - vec2(0.0, e.y));
    vec3 n = normalize(vec3(-gx * 3.0, -gy * 3.0, 1.0));

    float t = iTime * 0.5 * lightSpeed;
    vec2 tilt = iTilt * viewReact;
    vec3 l = normalize(vec3(vec2(cos(t), sin(t) * 0.7) * 0.6 + tilt * 3.0, 1.0));
    vec3 h = normalize(l + vec3(0.0, 0.0, 1.0));

    float lit = dot(n, l) - l.z;
    vec3 base = picture.rgb * (1.0 + lit * shading * 2.0);

    float spec = max(pow(max(dot(n, h), 0.0), sharpness) - pow(h.z, sharpness) * 0.85, 0.0);
    float hue = fract(atan(n.y, n.x) / 6.2831853 + uv.y * 0.4 + t * 0.05 + (tilt.x * 1.6 + tilt.y * 1.1));
    vec3 prism = hsv2rgb(vec3(hue, 0.65 * clamp(hueSpread * 2.0, 0.0, 1.0), 1.0));
    vec3 light = prism * spec * shine * 4.0;

    vec2 dir = normalize(vec2(0.8, 0.6));
    float along = dot(uv, dir);
    float every = max(sweepEvery, 0.5);
    float at = -0.3 + mod(iTime + iSeed * every, every) * sweepSpeed * 0.8;
    float d = (along - at) / max(sweepWidth, 0.005);
    float band = exp(-d * d);
    float facing = clamp(dot(n.xy, dir) * -6.0, 0.0, 1.0) + clamp(length(n.xy) * 3.0, 0.0, 1.0) * 0.5;
    vec3 rake = mix(vec3(1.0), prism, 0.5) * band * (facing + 0.12) * sweep;

    float m = cardMask(uv);
    return vec4(mix(picture.rgb, base, m) + (light * strength + rake) * m, picture.a);
}
`

func embossStock() StockShader {
	num := func(name string, v, min, max float64) StarterParam {
		return StarterParam{Param: Param{name, KindNumber}, Value: []float64{v}, Min: min, Max: max}
	}
	return StockShader{
		ID: StockPrefix + "ex", Name: "Holo: Embossed prism (EX foil)", Border: "EX", Code: embossCode, Bundle: embossBundle,
		Params: []StarterParam{
			// The user's values, tuned on the Shaders page 2026-11-16.
			num("relief", 1.03, 0, 3), num("diamond", 0.5, 0, 1), num("diamondScale", 47.6, 10, 150),
			num("shading", 0.16, 0, 1.5), num("shine", 0.82, 0, 2), num("sharpness", 18.2, 2, 96),
			num("hueSpread", 0.53, 0, 1), num("lightSpeed", 1.5, 0, 3), num("viewReact", 0.66, 0, 1), num("strength", 0.18, 0, 1.5),
			num("sweep", 0.6, 0, 2), num("sweepWidth", 0.05, 0.01, 0.3), num("sweepSpeed", 1, 0.2, 3), num("sweepEvery", 4, 1, 15),
		},
	}
}

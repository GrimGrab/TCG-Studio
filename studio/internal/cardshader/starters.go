package cardshader

// Starter is a shader to begin a new one from.
type Starter struct {
	Name   string         `json:"name"`
	About  string         `json:"about"`
	Code   string         `json:"code"`
	Params []StarterParam `json:"params"`
	// Draws, Mix, Strength: the shader's settings to start with (setfmt.Shader's fields of the same names).
	Draws    string  `json:"draws,omitempty"`
	Mix      string  `json:"mix,omitempty"`
	Strength float64 `json:"mixStrength,omitempty"`
}

// StarterParam is a starter's parameter with its first value (and slider range for numbers).
type StarterParam struct {
	Param
	Value []float64 `json:"value,omitempty"`
	Min   float64   `json:"min,omitempty"`
	Max   float64   `json:"max,omitempty"`
}

// Starters: the first one is what "New shader" makes (the plain picture); the others are the page's examples.
var Starters = []Starter{
	{Name: "Plain picture", About: "Just the card's picture: a blank page.", Code: `// uv is the place on the card: (0, 0) is the bottom left corner, (1, 1) the top right.
// art(uv) is the card's picture there. Return the colour to draw; keep the picture's alpha for the rounded corners.
// A shader from shadertoy.com? Select everything here and paste it over: it is then mixed with the card's picture.
vec4 card(vec2 uv) {
    vec4 picture = art(uv);
    return picture;
}
`},
	{Name: "Rainbow holo", About: "Rainbow bands that slide over the picture as the card tilts.", Code: `// Rainbow bands that slide over the picture as the card tilts.
vec3 rainbow(float t) {
    return 0.5 + 0.5 * cos(6.2831853 * (t + vec3(0.0, 0.33, 0.67)));
}

vec4 card(vec2 uv) {
    vec4 picture = art(uv);
    float t = (uv.x * 0.6 + uv.y) * bands + iTilt.x * 1.5 + iTilt.y + iTime * speed;
    float brightness = dot(picture.rgb, vec3(0.299, 0.587, 0.114));
    vec3 holo = rainbow(t) * strength * mix(0.4, 1.0, brightness);
    return vec4(picture.rgb + holo, picture.a);
}
`, Params: []StarterParam{
		{Param: Param{"bands", KindNumber}, Value: []float64{1.5}, Min: 0, Max: 6},
		{Param: Param{"speed", KindNumber}, Value: []float64{0.05}, Min: 0, Max: 1},
		{Param: Param{"strength", KindNumber}, Value: []float64{0.35}, Min: 0, Max: 1},
	}},
	{Name: "Light sweep", About: "A soft bar of light that crosses the card again and again.", Code: `// A soft bar of light that crosses the card again and again.
vec4 card(vec2 uv) {
    vec4 picture = art(uv);
    float where = fract(iTime * speed) * 1.6 - 0.3;
    float d = (uv.x + uv.y * 0.4) / 1.4 - where;
    float light = exp(-d * d / (width * width));
    return vec4(picture.rgb + tint.rgb * tint.a * light, picture.a);
}
`, Params: []StarterParam{
		{Param: Param{"speed", KindNumber}, Value: []float64{0.25}, Min: 0, Max: 2},
		{Param: Param{"width", KindNumber}, Value: []float64{0.08}, Min: 0.01, Max: 0.5},
		{Param: Param{"tint", KindColour}, Value: []float64{1, 1, 1, 0.6}},
	}},
	{Name: "Sparkles", About: "Small stars that twinkle as time passes and the card tilts.", Code: `// Small stars that twinkle as time passes and the card tilts.
float hash(vec2 p) {
    p = fract(p * vec2(123.34, 456.21));
    p += dot(p, p + 45.32);
    return fract(p.x * p.y);
}

vec4 card(vec2 uv) {
    vec4 picture = art(uv);
    vec2 grid = uv * vec2(count, count * 1.4);
    vec2 cell = floor(grid);
    vec2 centre = 0.2 + 0.6 * vec2(hash(cell), hash(cell + 17.13));
    float star = pow(clamp(1.0 - length(fract(grid) - centre) * 6.0, 0.0, 1.0), 3.0);
    float phase = hash(cell + 3.7) * 43.0 + iTime * 2.0 + dot(iTilt, vec2(21.0, 13.0));
    float twinkle = pow(sin(phase) * 0.5 + 0.5, 6.0);
    return vec4(picture.rgb + sparkle.rgb * star * twinkle, picture.a);
}
`, Params: []StarterParam{
		{Param: Param{"count", KindNumber}, Value: []float64{40}, Min: 5, Max: 120},
		{Param: Param{"sparkle", KindColour}, Value: []float64{1, 1, 0.9, 1}},
	}},
	{Name: "Crystal shards", About: "The picture behind cracked crystal: shards bend it and flash as the card tilts.", Code: `// Crystal shards: the picture seen through cracked crystal. Every shard bends the picture a little
// its own way and flashes in its own colour when the card tilts towards it.
vec2 hash2(vec2 p) {
    p = vec2(dot(p, vec2(127.1, 311.7)), dot(p, vec2(269.5, 183.3)));
    return fract(sin(p) * 43758.5453);
}

vec3 rainbow(float t) {
    return 0.5 + 0.5 * cos(6.2831853 * (t + vec3(0.0, 0.33, 0.67)));
}

vec4 card(vec2 uv) {
    // Find the shard this pixel is in (the nearest point of a jittered grid) and how close it is to a crack.
    vec2 p = uv * vec2(shards, shards * 1.4);
    vec2 cell = floor(p);
    vec2 f = fract(p);
    float nearest = 8.0;
    float second = 8.0;
    vec2 id = vec2(0.0);
    for (int y = -1; y <= 1; y++) {
        for (int x = -1; x <= 1; x++) {
            vec2 g = vec2(float(x), float(y));
            float d = length(g + hash2(cell + g) - f);
            if (d < nearest) { second = nearest; nearest = d; id = cell + g; }
            else if (d < second) { second = d; }
        }
    }
    float crack = 1.0 - smoothstep(0.0, 0.06, second - nearest);

    // Each shard is a tiny tilted mirror: it has its own direction, which also bends the picture behind it.
    vec2 facet = hash2(id + 7.3) - 0.5;
    vec4 picture = art(uv + facet * bend);

    // A shard flashes when the view lines up with its direction, and breathes slowly on its own.
    float lined = 1.0 - clamp(length(facet * 1.2 + iTilt * 2.0), 0.0, 1.0);
    float flash = pow(lined, 3.0);
    float breathe = 0.5 + 0.5 * sin(iTime * 0.7 + hash2(id).x * 6.2831853);
    vec3 colour = rainbow(hash2(id + 3.1).x + iTilt.x + iTilt.y * 0.5);

    vec3 col = picture.rgb + colour * (flash * 0.9 + breathe * 0.08) * shine;
    col = mix(col, edge.rgb, crack * edge.a);
    return vec4(col, art(uv).a);
}
`, Params: []StarterParam{
		{Param: Param{"shards", KindNumber}, Value: []float64{9}, Min: 3, Max: 30},
		{Param: Param{"bend", KindNumber}, Value: []float64{0.012}, Min: 0, Max: 0.05},
		{Param: Param{"shine", KindNumber}, Value: []float64{0.6}, Min: 0, Max: 1.5},
		{Param: Param{"edge", KindColour}, Value: []float64{1, 1, 1, 0.35}},
	}},
	{Name: "Dissolve", About: "The card fades out in burning patches and comes back; you see what is behind it.", Draws: "seeThrough", Code: `// Dissolve: the card burns away in patches and comes back, showing what is behind it.
// Needs "Draws: the whole card, see-through" (beside the preview), or the game's card frame shows in the holes.
float hash(vec2 p) {
    p = fract(p * vec2(123.34, 456.21));
    p += dot(p, p + 45.32);
    return fract(p.x * p.y);
}

float noise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    vec2 u = f * f * (3.0 - 2.0 * f);
    return mix(mix(hash(i), hash(i + vec2(1.0, 0.0)), u.x), mix(hash(i + vec2(0.0, 1.0)), hash(i + vec2(1.0, 1.0)), u.x), u.y);
}

vec4 card(vec2 uv) {
    vec4 picture = art(uv);
    // Every copy on screen starts at its own moment (iSeed), so a page of them doesn't fade in step.
    float t = iTime * speed + iSeed * 10.0;
    float gone = smoothstep(0.2, 0.8, 0.5 + 0.5 * sin(t));
    float patches = noise(uv * vec2(patchSize, patchSize * 1.4) + iSeed * 31.0) * 0.7 + noise(uv * 40.0) * 0.3;
    float here = smoothstep(-0.04, 0.04, patches - gone * 1.08 + 0.04);
    // A burning edge where the card is about to go: light on top, also where the card is already gone.
    float rim = 1.0 - abs(here * 2.0 - 1.0);
    cardGlow += burn.rgb * burn.a * rim * picture.a;
    return vec4(picture.rgb, picture.a * here);
}
`, Params: []StarterParam{
		{Param: Param{"speed", KindNumber}, Value: []float64{0.6}, Min: 0, Max: 3},
		{Param: Param{"patchSize", KindNumber}, Value: []float64{6}, Min: 1, Max: 30},
		{Param: Param{"burn", KindColour}, Value: []float64{1, 0.55, 0.15, 1}},
	}},
	{Name: "Glitch", About: "Now and then the picture tears into shifted strips with split colours.", Code: `// Glitch: now and then the picture tears into strips that jump sideways, with the colours pulled apart.
float hash(float n) { return fract(sin(n) * 43758.5453); }

vec4 card(vec2 uv) {
    // Time in steps; each copy on screen has its own rhythm (iSeed). Most steps are quiet.
    float t = floor((iTime + iSeed * 20.0) * rate);
    float on = step(1.0 - amount, hash(t * 1.7 + iSeed * 9.0));
    float strip = floor(uv.y * strips);
    float jump = (hash(strip * 3.1 + t) - 0.5) * 2.0 * shift * on * step(0.5, hash(strip + t * 0.7));
    vec2 at = uv + vec2(jump, 0.0);
    vec4 picture = art(at);
    float r = art(at + vec2(split * on, 0.0)).r;
    float b = art(at - vec2(split * on, 0.0)).b;
    // A few bright scan lines while it glitches.
    cardGlow += vec3(0.2, 0.9, 1.0) * on * step(0.97, hash(strip * 7.3 + t)) * 0.5 * picture.a;
    return vec4(r, picture.g, b, art(uv).a);
}
`, Params: []StarterParam{
		{Param: Param{"rate", KindNumber}, Value: []float64{8}, Min: 1, Max: 30},
		{Param: Param{"amount", KindNumber}, Value: []float64{0.25}, Min: 0, Max: 1},
		{Param: Param{"strips", KindNumber}, Value: []float64{24}, Min: 4, Max: 80},
		{Param: Param{"shift", KindNumber}, Value: []float64{0.08}, Min: 0, Max: 0.3},
		{Param: Param{"split", KindNumber}, Value: []float64{0.012}, Min: 0, Max: 0.05},
	}},
	{Name: "Rank aura", About: "A glow that grows with the card: rarer, fancier borders, foil and high grades shine more.", Code: `// Rank aura: the card glows around its edge, and the more special the card is, the more it shows.
// It reads what the card is: iRarity (place in the set's rarity list), iBorder (0 Base .. 5 Full Art),
// iFoil and iGrade (0 = not graded). Change them with "Shown as" under the preview.
vec4 card(vec2 uv) {
    vec4 picture = art(uv);

    // How special the card is, 0..1 and beyond: each step of rarity and border adds a little, a grade of 9 or 10 a lot.
    float rank = float(iRarity) * rarityStep + float(iBorder) * borderStep;
    if (iGrade >= 9) rank += 0.3 * float(iGrade - 8);

    // The colour goes from the common colour to the rare one as the rarity rises.
    vec3 colour = mix(common.rgb, rare.rgb, clamp(float(iRarity) / 3.0, 0.0, 1.0));

    // A soft frame of light just inside the card's edge, breathing slowly; each copy in its own rhythm.
    vec2 fromEdge = min(uv, 1.0 - uv);
    float frame = smoothstep(width, 0.0, min(fromEdge.x, fromEdge.y * 1.4));
    float breathe = 0.75 + 0.25 * sin(iTime * 1.5 + iSeed * 6.2831853);
    cardGlow += colour * frame * rank * breathe * picture.a;

    // Foil versions get a band of light that crosses the card.
    if (iFoil) {
        float d = (uv.x + uv.y * 0.5) / 1.5 - fract(iTime * 0.2 + iSeed);
        cardGlow += colour * exp(-d * d / 0.004) * 0.5 * picture.a;
    }
    return picture;
}
`, Params: []StarterParam{
		{Param: Param{"rarityStep", KindNumber}, Value: []float64{0.25}, Min: 0, Max: 1},
		{Param: Param{"borderStep", KindNumber}, Value: []float64{0.1}, Min: 0, Max: 0.5},
		{Param: Param{"width", KindNumber}, Value: []float64{0.12}, Min: 0.02, Max: 0.4},
		{Param: Param{"common", KindColour}, Value: []float64{0.6, 0.8, 1, 1}},
		{Param: Param{"rare", KindColour}, Value: []float64{1, 0.75, 0.2, 1}},
	}},
	{Name: "Graded gem", About: "Only graded cards change: the higher the grade, the more sparkles; a 10 gets a crown of light.", Code: `// Graded gem: ungraded cards are drawn as they are. Graded ones sparkle, more with every grade,
// and a 10 gets a slow ring of light. Set "grade" in "Shown as" under the preview to see it (0 = not graded).
float hash(vec2 p) {
    p = fract(p * vec2(123.34, 456.21));
    p += dot(p, p + 45.32);
    return fract(p.x * p.y);
}

vec4 card(vec2 uv) {
    vec4 picture = art(uv);
    if (iGrade == 0) return picture;

    // Sparkles: more cells light up as the grade rises.
    float share = float(iGrade) / 10.0;
    vec2 grid = uv * vec2(count, count * 1.4);
    vec2 cell = floor(grid);
    vec2 centre = 0.2 + 0.6 * vec2(hash(cell), hash(cell + 17.13));
    float star = pow(clamp(1.0 - length(fract(grid) - centre) * 6.0, 0.0, 1.0), 3.0);
    float lit = step(1.0 - share * 0.5, hash(cell + 3.7));
    float twinkle = pow(sin(hash(cell + 9.1) * 43.0 + iTime * 2.0 + iSeed * 20.0) * 0.5 + 0.5, 6.0);
    cardGlow += sparkle.rgb * sparkle.a * star * lit * twinkle * picture.a;

    // A perfect 10: a ring of light that slowly opens from the middle.
    if (iGrade >= 10) {
        float r = length((uv - 0.5) * vec2(1.0, 1.4));
        float ring = exp(-pow((r - fract(iTime * 0.15 + iSeed) * 0.9) / 0.03, 2.0));
        cardGlow += sparkle.rgb * ring * 0.4 * picture.a;
    }
    return picture;
}
`, Params: []StarterParam{
		{Param: Param{"count", KindNumber}, Value: []float64{30}, Min: 5, Max: 100},
		{Param: Param{"sparkle", KindColour}, Value: []float64{1, 0.95, 0.7, 1}},
	}},
}

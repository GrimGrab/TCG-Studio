package cardshader

import "strings"

// TutorialStep is one step of the Shaders page's guided tutorial. Every step adds one idea to the same shader, so the
// steps' codes build on each other and the last one is a finished effect.
type TutorialStep struct {
	Title string `json:"title"`
	// Why: what the step is about, in a sentence or two. Try: something to play with once it works.
	Why string `json:"why"`
	Try string `json:"try,omitempty"`
	// Add: the lines to type (shown as they are); Where: where they go.
	Add   string `json:"add,omitempty"`
	Where string `json:"where,omitempty"`
	// Code: the whole shader once the step is done ("Do it for me" puts it in the editor). Params: the parameters the
	// code uses so far, with the values "Do it for me" gives new ones. Draws: the shader's Draws setting from this step
	// on ("" = unchanged).
	Code   string         `json:"code"`
	Params []StarterParam `json:"params,omitempty"`
	Draws  string         `json:"draws,omitempty"`
	// Uses: what the code has once the step is done and did not have before; the page ticks the step off when it sees it.
	Uses string `json:"uses"`
}

// The tutorial's lines, in the versions the steps go through.
const (
	tutTint      = `    picture.rgb *= vec3(1.0, 0.85, 0.6);`
	tutTintParam = `    picture.rgb *= tint.rgb;`

	tutBand      = `    float band = sin((uv.x + uv.y) * 10.0 - iTime * 2.0) * 0.5 + 0.5;`
	tutBandTilt  = `    float band = sin((uv.x + uv.y) * 10.0 + iTilt.x * 8.0 - iTime * 2.0) * 0.5 + 0.5;`
	tutBandSpeed = `    float band = sin((uv.x + uv.y) * 10.0 + iTilt.x * 8.0 - iTime * speed) * 0.5 + 0.5;`
	tutBandSeed  = `    float band = sin((uv.x + uv.y) * 10.0 + iTilt.x * 8.0 - iTime * speed + iSeed * 6.28) * 0.5 + 0.5;`

	tutLight      = `    picture.rgb += band * 0.2;`
	tutLightParam = `    picture.rgb += band * strength;`
	tutRainbow    = `    vec3 rainbow = 0.5 + 0.5 * cos(6.28 * (band + vec3(0.0, 0.33, 0.67)));`
	tutGlow       = `    cardGlow += rainbow * band * strength * picture.a;`
	tutGrain      = `    float grain = texture(pattern, uv * 3.0).r;`
	tutGlowGrain  = `    cardGlow += rainbow * band * strength * grain * picture.a;`

	tutFade = `    picture.a *= 0.5 + 0.5 * band;`
)

func tutorialCode(lines ...string) string {
	return "vec4 card(vec2 uv) {\n    vec4 picture = art(uv);\n" + strings.Join(lines, "\n") + "\n    return picture;\n}\n"
}

// The tutorial's parameters, with the values "Do it for me" starts them at.
var (
	tutStrength = StarterParam{Param: Param{"strength", KindNumber}, Value: []float64{0.3}, Min: 0, Max: 1}
	tutTintP    = StarterParam{Param: Param{"tint", KindColour}, Value: []float64{1, 0.85, 0.6, 1}}
	tutSpeed    = StarterParam{Param: Param{"speed", KindNumber}, Value: []float64{2}, Min: 0, Max: 5}
	tutPattern  = StarterParam{Param: Param{"pattern", KindImage}}
)

// Tutorial is the guided tutorial: from the plain picture to a rainbow holo that fades, with the Parameters panel on
// the way (a number, a colour, a slider's range, a picture).
var Tutorial = []TutorialStep{
	{
		Title: "The picture",
		Why: "A shader is one function, card, that runs for every pixel of the card and answers with that pixel's colour. " +
			"uv is where the pixel is (0, 0 = bottom left, 1, 1 = top right) and art(uv) is the card's picture there. " +
			"Right now it just hands the picture back, so the card looks normal.",
		Try:  "Change art(uv) to art(uv * 0.5) and the picture zooms in. Put it back before going on.",
		Code: tutorialCode(),
		Uses: "art(",
	},
	{
		Title: "Change the colours",
		Why: "A colour is three numbers: red, green, blue, each from 0 to 1. Multiplying the picture by a colour tints it: " +
			"here red stays, green drops a little and blue drops more, which warms the card up.",
		Try:   "Try vec3(0.6, 0.8, 1.0) for a cold card, or vec3(1.0, 0.3, 0.3) for a red one.",
		Add:   tutTint,
		Where: "After the line with art(uv), before return.",
		Code:  tutorialCode(tutTint),
		Uses:  "vec3(",
	},
	{
		Title: "Make it move",
		Why: "iTime is a clock: the seconds that have passed. sin() of something that grows swings up and down for ever, so " +
			"mixing the pixel's place with the clock gives stripes of light that travel across the card.",
		Try:   "Change 10.0 for more or fewer stripes.",
		Add:   tutBand + "\n" + tutLight,
		Where: "After the tint line, before return.",
		Code:  tutorialCode(tutTint, tutBand, tutLight),
		Uses:  "iTime",
	},
	{
		Title: "React to the view",
		Why: "iTilt says how far the card is seen from the side. Adding it to the stripes makes them slide when the card " +
			"turns, which is what makes a holo card look like foil instead of a film playing on it.",
		Try:   "Move the mouse over the preview card: the stripes follow.",
		Add:   tutBandTilt,
		Where: "Replace the line that starts with float band.",
		Code:  tutorialCode(tutTint, tutBandTilt, tutLight),
		Uses:  "iTilt",
	},
	{
		Title: "Your first parameter",
		Why: "Numbers typed into the code can only be changed in the code. Use a name instead: a name the code doesn't know " +
			"turns into a parameter in the Parameters list on the right about a second after you stop typing (until then the " +
			"editor says the name is not known). Each parameter has its name, its kind, and a control for its value: for a " +
			"number, a slider and a box to type in.",
		Try: "Under Parameters, drag the strength slider and watch the stripes get brighter and dimmer. The code didn't change: " +
			"only the value did. A card can later have its own value for it (the card's Shader tab).",
		Add:    tutLightParam,
		Where:  "Replace the line picture.rgb += band * 0.2;",
		Code:   tutorialCode(tutTint, tutBandTilt, tutLightParam),
		Params: []StarterParam{tutStrength},
		Uses:   "strength",
	},
	{
		Title: "A colour to pick",
		Why: "How the code uses a name decides the parameter's kind. Using it with .rgb makes it a Colour: it gets a colour " +
			"picker instead of a slider. A new colour starts white, which leaves the picture as it is, so the tint disappears " +
			"until you pick one.",
		Try: "Click the colour box next to tint and pick a colour: the whole card takes it. The slider beside it is the " +
			"colour's alpha, which code reads as tint.a (this shader doesn't use it).",
		Add:    tutTintParam,
		Where:  "Replace the line picture.rgb *= vec3(1.0, 0.85, 0.6);",
		Code:   tutorialCode(tutTintParam, tutBandTilt, tutLightParam),
		Params: []StarterParam{tutStrength, tutTintP},
		Uses:   "tint.rgb",
	},
	{
		Title: "A slider with more room",
		Why: "A new number starts at 0.5 with a slider from 0 to 1, which is slow for a speed. Under every number's slider is " +
			"\"Slider from … to …\": that is the slider's range, yours to set. The value box beside the slider also takes any " +
			"number you type, inside the range or not.",
		Try: "Under speed, change \"to\" from 1 to 5, then drag the slider up: the stripes race. You can also make a parameter " +
			"first with Add parameter, name it, choose its kind from the list, and then use the name in the code; ✕ removes one.",
		Add:    tutBandSpeed,
		Where:  "Replace the line that starts with float band (the 2.0 becomes speed).",
		Code:   tutorialCode(tutTintParam, tutBandSpeed, tutLightParam),
		Params: []StarterParam{tutStrength, tutTintP, tutSpeed},
		Uses:   "speed",
	},
	{
		Title: "Light in colour",
		Why: "cardGlow is light added on top of the card. Three cos() waves, one per colour and a third of a turn apart, " +
			"make a rainbow; multiplying by picture.a keeps the light inside the card's rounded corners.",
		Try:    "strength now sets how bright the rainbow is, and tint still colours the picture under it: tune both on the right.",
		Add:    tutRainbow + "\n" + tutGlow,
		Where:  "Replace the line picture.rgb += band * strength;",
		Code:   tutorialCode(tutTintParam, tutBandSpeed, tutRainbow, tutGlow),
		Params: []StarterParam{tutStrength, tutTintP, tutSpeed},
		Uses:   "cardGlow",
	},
	{
		Title: "Every copy its own",
		Why: "On a binder page every card with this shader would move exactly in step. iSeed is a random number each copy " +
			"on screen gets when it appears; adding it shifts the stripes differently on every card.",
		Try:    "Press Restart under the preview: the card appears again with another number, and the stripes start elsewhere.",
		Add:    tutBandSeed,
		Where:  "Replace the line that starts with float band.",
		Code:   tutorialCode(tutTintParam, tutBandSeed, tutRainbow, tutGlow),
		Params: []StarterParam{tutStrength, tutTintP, tutSpeed},
		Uses:   "iSeed",
	},
	{
		Title: "A picture of your own",
		Why: "A parameter can also be a picture: looking a name up with texture(name, place) makes it a Picture parameter with " +
			"a \"Choose picture…\" button. Until you choose one it is white, which changes nothing. Here its brightness " +
			"decides where the rainbow shows; uv * 3.0 repeats the picture three times across the card.",
		Try: "Under pattern, click Choose picture… and pick any picture with light and dark parts (a texture, a logo, stars): " +
			"the rainbow now only shines where the picture is bright.",
		Add:    tutGrain + "\n" + tutGlowGrain,
		Where:  "Replace the line that starts with cardGlow.",
		Code:   tutorialCode(tutTintParam, tutBandSeed, tutRainbow, tutGrain, tutGlowGrain),
		Params: []StarterParam{tutStrength, tutTintP, tutSpeed, tutPattern},
		Uses:   "texture(pattern",
	},
	{
		Title: "See through it",
		Why: "The fourth number of a colour is alpha: how solid the pixel is. Lowering it makes the card see-through. For that " +
			"to show what is behind the card, set Draws (right of the editor, under the preview) to \"The whole card, " +
			"see-through\"; otherwise the game's card frame shows in its place.",
		Try:    "That's a finished shader. Save, then give it to a card version on a set's Set tab or a card's Shader tab.",
		Add:    tutFade,
		Where:  "Before return. Then set Draws to \"The whole card, see-through\".",
		Code:   tutorialCode(tutTintParam, tutBandSeed, tutRainbow, tutGrain, tutGlowGrain, tutFade),
		Params: []StarterParam{tutStrength, tutTintP, tutSpeed, tutPattern},
		Draws:  "seeThrough",
		Uses:   "picture.a *=",
	},
}

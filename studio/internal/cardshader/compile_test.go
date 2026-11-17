package cardshader

import (
	"errors"
	"os"
	"strings"
	"testing"
)

const sampleShader = `// bands of light that follow the view
float band(float x) { return pow(sin(x * 6.2831) * 0.5 + 0.5, 4.0); }
vec4 card(vec2 uv) {
    vec4 a = art(uv + vec2(sin(uv.y * 20.0 + iTime) * wobble, 0.0));
    float b = band(uv.x + uv.y + iTilt.x + iTime * 0.1);
    vec3 col = a.rgb + glow.rgb * b * texture(pattern, uv * 4.0).r * iFacing;
    return vec4(col, a.a);
}`

// Everything a shader can know about its card, a see-through part and added light.
const cardFactsShader = `vec4 card(vec2 uv) {
    vec4 a = art(uv);
    float gone = step(fract(iSeed + iCard + iShown * 0.2 + iHover), 0.5) * cardMask(uv);
    cardGlow += vec3(0.1) * float(iBorder + iRarity + iGrade) * (iFoil ? 1.0 : 0.5);
    return vec4(a.rgb, a.a * (1.0 - gone));
}`

var sampleParams = []Param{{"wobble", KindNumber}, {"glow", KindColour}, {"pattern", KindImage}}

func TestGameHLSL(t *testing.T) {
	slots, err := Slots(sampleParams)
	if err != nil {
		t.Fatal(err)
	}
	for _, kw := range [][]string{nil, {"UNITY_UI_ALPHACLIP"}, {"UNITY_UI_CLIP_RECT"}, {"UNITY_UI_ALPHACLIP", "UNITY_UI_CLIP_RECT"}} {
		hlsl, err := gameHLSL(sampleShader, slots, kw, Options{})
		if err != nil {
			t.Fatalf("%v: %v", kw, err)
		}
		if os.Getenv("CARDSHADER_SHOW") != "" && len(kw) == 2 {
			t.Log("\n" + hlsl)
		}
		// The registers and semantics the template's vertex program and stored bindings expect.
		for _, want := range []string{"register(b0)", "_MainTex : register(t0)", "register(s0)", "_Tex0 : register(t1)", "register(s1)",
			"TEXCOORD0", "TEXCOORD1", "TEXCOORD2", "TEXCOORD3", "TEXCOORD4", "TEXCOORD5", "SV_Target0", "packoffset(c2)"} {
			if !strings.Contains(hlsl, want) {
				t.Errorf("%v: converted shader lacks %q", kw, want)
			}
		}
		if strings.Contains(hlsl, "SV_Position") {
			t.Errorf("%v: converted shader reads SV_Position, which would shift its inputs", kw)
		}
		dxbc, err := compileHLSL(hlsl, "main", "ps_4_0", nil)
		if err != nil {
			t.Fatalf("%v: %v", kw, err)
		}
		asm, err := disassemble(dxbc)
		if err != nil {
			t.Fatal(err)
		}
		if os.Getenv("CARDSHADER_SHOW") != "" && len(kw) == 2 {
			t.Log("\n" + asm)
		}
		if strings.Contains(asm, "discard") != (len(kw) > 0 && kw[0] == "UNITY_UI_ALPHACLIP") {
			t.Errorf("%v: alpha clip is in the wrong variant", kw)
		}
	}
}

func TestCardInputsCompile(t *testing.T) {
	slots, _ := Slots(nil)
	hlsl, err := gameHLSL(cardFactsShader, slots, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(hlsl, "_Mask : register(t5)") {
		t.Error("the mask is not texture 5")
	}
	if _, err := compileHLSL(hlsl, "main", "ps_4_0", nil); err != nil {
		t.Fatal(err)
	}
}

func TestSourceErrorsKeepTheirLines(t *testing.T) {
	slots, _ := Slots(nil)
	_, err := gameHLSL("vec4 card(vec2 uv) {\n    return vec4(nothing, 1.0);\n}", slots, nil, Options{})
	var se *SourceError
	if !errors.As(err, &se) {
		t.Fatalf("want a SourceError, got %v", err)
	}
	if !strings.Contains(se.Messages, "0:2:") || !strings.Contains(se.Messages, "nothing") {
		t.Errorf("message does not point at line 2: %s", se.Messages)
	}
}

func TestStartersCompile(t *testing.T) {
	for _, s := range Starters {
		var params []Param
		for _, p := range s.Params {
			params = append(params, p.Param)
		}
		slots, err := Slots(params)
		if err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
		hlsl, err := gameHLSL(s.Code, slots, []string{"UNITY_UI_ALPHACLIP", "UNITY_UI_CLIP_RECT"}, Options{Mix: s.Mix, Strength: s.Strength})
		if err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
		if _, err := compileHLSL(hlsl, "main", "ps_4_0", nil); err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
	}
}

// Every step of the tutorial compiles for the game, and has what it says it adds while the step before it does not
// (the page ticks a step off when it sees that).
func TestTutorialSteps(t *testing.T) {
	for i, s := range Tutorial {
		var params []Param
		for _, p := range s.Params {
			params = append(params, p.Param)
		}
		slots, err := Slots(params)
		if err != nil {
			t.Fatalf("step %d: %v", i+1, err)
		}
		hlsl, err := gameHLSL(s.Code, slots, nil, Options{})
		if err != nil {
			t.Fatalf("step %d: %v", i+1, err)
		}
		if _, err := compileHLSL(hlsl, "main", "ps_4_0", nil); err != nil {
			t.Fatalf("step %d: %v", i+1, err)
		}
		if !strings.Contains(s.Code, s.Uses) || (i > 0 && strings.Contains(Tutorial[i-1].Code, s.Uses)) {
			t.Errorf("step %d: %q does not tell it from the step before", i+1, s.Uses)
		}
		if s.Add != "" && !strings.Contains(s.Code, s.Add) {
			t.Errorf("step %d: the lines to type are not in its code", i+1)
		}
	}
}

// The template bundle and the prelude must describe the same shader: its four pixel programs, one constant buffer of
// 18 registers (b0), six textures and six inputs. Skipped until the template has been built (tools\build-shaders.ps1).
func TestTemplateMatchesPrelude(t *testing.T) {
	template, err := Template()
	if err != nil {
		t.Skip(err)
	}
	progs, err := shaderPrograms(template, TemplateShader)
	if err != nil {
		t.Fatal(err)
	}
	if len(progs) != 4 {
		t.Fatalf("template has %d pixel programs, want 4 (clip rect × alpha clip)", len(progs))
	}
	for _, p := range progs {
		asm, err := disassemble(p.DXBC)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"dcl_constantbuffer CB0[18]", "dcl_resource_texture2d (float,float,float,float) t5", "dcl_sampler s5",
			"dcl_input_ps linear v0", "dcl_input_ps linear v5"} {
			if !strings.Contains(asm, want) {
				t.Errorf("%v: template program lacks %q", p.Keywords, want)
			}
		}
		for _, not := range []string{"CB1", "t6", "dcl_input_ps linear v6", "SV_Position"} {
			if strings.Contains(asm, not) {
				t.Errorf("%v: template program has %q, which the prelude doesn't describe", p.Keywords, not)
			}
		}
	}
	out, err := Build(template, "test", sampleShader, sampleParams, Options{})
	if err != nil {
		t.Fatal(err)
	}
	again, err := shaderPrograms(out, TemplateShader)
	if err != nil || len(again) != 4 {
		t.Fatalf("built bundle: %d programs, %v", len(again), err)
	}
}

// The stock shaders' committed files are what stock.go and the template give now.
func TestStockFilesCurrent(t *testing.T) {
	template, err := Template()
	if err != nil {
		t.Skip(err)
	}
	bundles, err := StockBundles(template)
	if err != nil {
		t.Fatal(err)
	}
	same := string(StockJSON()) == string(stockJSONFile)
	for name, b := range bundles {
		if f, err := stockBundleFiles.ReadFile("stock/" + name); err != nil || string(f) != string(b) {
			same = false
		}
	}
	if files, _ := stockBundleFiles.ReadDir("stock"); len(files) != len(bundles) {
		same = false // a bundle no stock shader uses any more
	}
	if !same {
		t.Fatal("internal/cardshader/stock is out of date: run `go run ./cmd/stockshaders` in studio\\ and rebuild the mod")
	}
	for _, s := range Stock() {
		if !IsStock(s.ID) || len(s.Params) > MaxValues {
			t.Errorf("%s: bad id or too many parameters", s.ID)
		}
	}
}

// A shader pasted from shadertoy.com (mainImage, iChannel0, iMouse, a texture lookup in a loop) compiles for the game.
func TestShadertoyCode(t *testing.T) {
	const toy = `float ring(vec2 p, float r) { return smoothstep(0.02, 0.0, abs(length(p) - r)); }
void mainImage(out vec4 fragColor, in vec2 fragCoord) {
    vec2 uv = fragCoord / iResolution.xy;
    vec2 p = (fragCoord - 0.5 * iResolution.xy) / iResolution.y;
    vec3 col = texture(iChannel0, uv).rgb;
    float steps = 4.0 + floor(iMouse.x / iResolution.x * 8.0);
    for (float i = 0.0; i < steps; i += 1.0) {
        col += texture(iChannel0, uv + 0.01 * i * vec2(cos(iTime), sin(iTime))).rgb * 0.05;
        col += ring(p, 0.1 * i + fract(iTime * 0.2)) * vec3(0.2, 0.5, 1.0);
    }
    fragColor = vec4(col / (1.0 + float(iFrame % 2) * 0.0), 1.0);
}`
	if !IsShadertoy(toy) || IsShadertoy(sampleShader) {
		t.Fatal("Shadertoy code is not told apart from card code")
	}
	template, err := Template()
	if err != nil {
		t.Skip(err)
	}
	for mix := range mixCode {
		if _, err := Build(template, "toy", toy, nil, Options{Mix: mix, Strength: 0.5}); err != nil {
			t.Fatalf("%s: %v", mix, err)
		}
	}
	if _, err := Build(template, "toy", toy, nil, Options{Mix: "sideways"}); err == nil {
		t.Fatal("an unknown mix was accepted")
	}
}

func TestSlots(t *testing.T) {
	for _, bad := range [][]Param{
		{{"iTime", KindNumber}}, {{"cardGlow", KindColour}}, {{"iSeed", KindNumber}}, {{"a", KindNumber}, {"a", KindColour}}, {{"2x", KindNumber}}, {{"a__b", KindNumber}}, {{"x", "matrix"}},
		{{"a", KindImage}, {"b", KindImage}, {"c", KindImage}, {"d", KindImage}, {"e", KindImage}},
	} {
		if _, err := Slots(bad); err == nil {
			t.Errorf("%v: accepted", bad)
		}
	}
	s, err := Slots([]Param{{"a", KindNumber}, {"img", KindImage}, {"b", KindVec2}, {"img2", KindImage}})
	if err != nil || s[0].Index != 0 || s[1].Index != 0 || s[2].Index != 1 || s[3].Index != 1 {
		t.Errorf("slots %v, %v", s, err)
	}
}

package share

import (
	"path/filepath"
	"testing"

	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

// A set brings the borders its slots and cards use, and those borders bring their shaders. Where both ids are taken,
// the set points at the border's new id and the border at the shader's new id.
func TestSetBringsItsBorders(t *testing.T) {
	h1, a1 := workspaceWith(t)
	ws1 := workspace.Dir(h1.Root)
	l, err := ws1.Items(a1)
	if err != nil {
		t.Fatal(err)
	}
	bundle, _ := l.PutBytes([]byte("compiled"), ".shaderbundle")
	pic, _ := l.PutBytes([]byte("frame"), ".png")
	l.Lib.Shaders = []setfmt.Shader{{ID: "shine", Name: "Shine", Code: "vec4 card(vec2 uv) { return art(uv); }", Bundle: bundle}}
	l.Lib.Borders = []setfmt.Border{{ID: "leaf", Name: "Leaf", Layers: []setfmt.BorderLayer{
		{Kind: setfmt.BorderImageLayer, File: pic, Opacity: 1},
		{Kind: setfmt.BorderShaderLayer, Shader: "shine", Opacity: 1, Clip: true},
	}}}
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}
	p, _ := ws1.Projects(a1).Load("dom")
	p.Set.Borders = map[string]setfmt.BorderSlot{"Gold": {Look: "leaf"}}
	p.Set.Cards[0].VersionBorders = map[string]string{"Silver_foil": "leaf"}
	if err := ws1.Projects(a1).Save(p); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "set"+ItemsExt)
	if _, err := ExportItems(h1, a1, []Pick{{"set", "dom"}}, out, "1.0", nil); err != nil {
		t.Fatal(err)
	}

	// Another PC, where "leaf" and "shine" are already other items.
	h2 := setups.Home{Root: t.TempDir()}
	if _, err := h2.Ensure("1.0"); err != nil {
		t.Fatal(err)
	}
	a2 := active(t, h2)
	ws2 := workspace.Dir(h2.Root)
	l2, _ := ws2.Items(a2)
	l2.Lib.Shaders = []setfmt.Shader{{ID: "shine", Name: "Mine"}}
	l2.Lib.Borders = []setfmt.Border{{ID: "leaf", Name: "Mine", Layers: []setfmt.BorderLayer{}}}
	if err := l2.Save(); err != nil {
		t.Fatal(err)
	}
	res, err := ImportItems(h2, a2, out, Selection{AddHere: true}, nil)
	if err != nil || len(res.Items) != 3 {
		t.Fatalf("import: %+v %v", res, err)
	}
	ids := map[string]string{}
	for _, it := range res.Items {
		ids[it.Kind] = it.ID
	}
	border, shader := ids["border"], ids["shader"]
	if border == "" || border == "leaf" || shader == "" || shader == "shine" {
		t.Fatalf("border and shader did not come along under new ids: %+v", res.Items)
	}
	q, err := ws2.Projects(a2).Load("dom")
	if err != nil {
		t.Fatal(err)
	}
	if q.Set.Borders["Gold"].Look != border || q.Set.Cards[0].VersionBorders["Silver_foil"] != border {
		t.Fatalf("set borders: %+v, card %v (border now %s)", q.Set.Borders, q.Set.Cards[0].VersionBorders, border)
	}
	l2, _ = ws2.Items(a2)
	i := l2.BorderIndex(border)
	if i < 0 || len(l2.Lib.Borders[i].Layers) != 2 || l2.Lib.Borders[i].Layers[1].Shader != shader {
		t.Fatalf("imported border: %+v", l2.Lib.Borders)
	}
}

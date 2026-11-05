package designs

import (
	"encoding/json"
	"math"
	"testing"

	"tcgstudio/internal/uvmap"
)

// A pack layout made before the front and back grew: layers move with their faces (by their centre), fills stay, the layout
// is stamped and a second run changes nothing. The shown part of the grown faces keeps the old net scale, so art on the pack
// stays where it was.
// A box design from when front & back and the sides were one face each: every layer shows on both again, once.
func TestMigrateBoxLayoutSplitsFaces(t *testing.T) {
	old := `{"version":2,"net":2,"layers":[
		{"id":"a","name":"Art","kind":"image","x":0.7,"y":0.5,"w":1,"h":1},
		{"id":"b","name":"Side","kind":"image","x":2,"y":0.5,"w":0.5,"h":0.5},
		{"id":"c","name":"Lid","kind":"image","x":0.7,"y":-0.5,"w":1,"h":0.5},
		{"id":"d","name":"Tint","kind":"fill","face":"right","x":0,"y":0,"w":0,"h":0},
		{"id":"e","name":"Map","kind":"fill","space":"texture","x":0.7,"y":0.5,"w":9,"h":9},
		{"id":"f","name":"All faces","kind":"fill","face":"","x":0,"y":0,"w":0,"h":0}]}`
	out, ok := Layout("Box", json.RawMessage(old))
	if !ok {
		t.Fatal("not migrated")
	}
	var l struct {
		Layers []map[string]any `json:"layers"`
	}
	if err := json.Unmarshal(out, &l); err != nil {
		t.Fatal(err)
	}
	// Whole-face colours are saved at 0, 0, inside the front's area: they must not be copied for that, or a solid colour lands
	// on top of the art. Only the right face's tint gets a copy, on the left.
	if len(l.Layers) != 10 {
		t.Fatalf("%d layers, want 6 + 4 copies: %v", len(l.Layers), l.Layers)
	}
	for _, ly := range l.Layers[6:] {
		if ly["kind"] == "fill" && ly["face"] != "left" {
			t.Errorf("a colour copied onto face %v: %v", ly["face"], ly["name"])
		}
	}
	for _, c := range []struct {
		name string
		x, y float64
	}{{"Art (back)", 0.7 + 2.476, 0.5}, {"Side (left)", 2 + 2.476, 0.5}, {"Lid (bottom)", 0.7, -0.5 + 2.052}} {
		found := false
		for _, ly := range l.Layers {
			if ly["name"] == c.name {
				found = true
				if math.Abs(ly["x"].(float64)-c.x) > 1e-9 || math.Abs(ly["y"].(float64)-c.y) > 1e-9 {
					t.Errorf("%s at %v, %v", c.name, ly["x"], ly["y"])
				}
			}
		}
		if !found {
			t.Errorf("no %s", c.name)
		}
	}
	if _, again := Layout("Box", out); again {
		t.Error("migrated twice")
	}
}

func TestMigratePackLayout(t *testing.T) {
	old := `{"version":2,"layers":[{"kind":"image","x":0.3},{"kind":"image","x":1.0},{"kind":"text","x":1.6},{"kind":"fill","face":"back"},{"kind":"fill","space":"texture","x":900}]}`
	out, ok := Layout("Pack", json.RawMessage(old))
	if !ok {
		t.Fatal("not migrated")
	}
	var l struct {
		Net    int              `json:"net"`
		Layers []map[string]any `json:"layers"`
	}
	if err := json.Unmarshal(out, &l); err != nil {
		t.Fatal(err)
	}
	want := []float64{0.3, 1.0 + 0.0954, 1.6 + 0.0954 + 0.0497}
	for i, w := range want {
		if x := l.Layers[i]["x"].(float64); math.Abs(x-w) > 1e-9 {
			t.Errorf("layer %d at %v, want %v", i, x, w)
		}
	}
	if _, has := l.Layers[3]["x"]; has || l.Net != uvmap.NetVersion {
		t.Errorf("fill got a place or net %d", l.Net)
	}
	if x := l.Layers[4]["x"].(float64); x != 900 {
		t.Errorf("map layer moved to %v (it is in texture pixels)", x)
	}
	if _, again := Layout("Pack", out); again {
		t.Error("migrated twice")
	}
	if _, other := Layout("Sleeve", json.RawMessage(old)); other {
		t.Error("a model without net moves changed")
	}

	// Each face's old net width = its shown part now; faces after it moved by the net moves.
	m, _ := uvmap.ModelFor("Pack")
	faces := map[string]uvmap.Face{}
	for _, f := range m.Faces {
		faces[f.ID] = f
	}
	for id, oldW := range map[string]float64{"front": 0.678, "back": 0.678} {
		f := faces[id]
		if w := f.Net[2] * f.Shown[2]; math.Abs(w-oldW) > 0.001 {
			t.Errorf("%s shows %v of the net, was %v", id, w, oldW)
		}
	}
	for id, oldX := range map[string]float64{"edges": 0.678, "back": 0.788, "seam": 1.566} {
		nx := oldX
		for _, mv := range m.NetMoves {
			if oldX >= mv.From {
				nx += mv.DX
			}
		}
		if math.Abs(faces[id].Net[0]-nx) > 0.001 {
			t.Errorf("%s at %v, the moves put it at %v", id, faces[id].Net[0], nx)
		}
	}
}

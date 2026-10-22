package uvmap

import (
	"encoding/json"
	"math"
	"testing"
)

// A pack layout made before the front and back grew: layers move with their faces (by their centre), fills stay, the layout
// is stamped and a second run changes nothing. The shown part of the grown faces keeps the old net scale, so art on the pack
// stays where it was.
func TestMigratePackLayout(t *testing.T) {
	old := `{"version":2,"layers":[{"kind":"image","x":0.3},{"kind":"image","x":1.0},{"kind":"text","x":1.6},{"kind":"fill","face":"back"},{"kind":"fill","space":"texture","x":900}]}`
	out, ok := MigrateLayout("Pack", json.RawMessage(old))
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
	if _, has := l.Layers[3]["x"]; has || l.Net != NetVersion {
		t.Errorf("fill got a place or net %d", l.Net)
	}
	if x := l.Layers[4]["x"].(float64); x != 900 {
		t.Errorf("map layer moved to %v (it is in texture pixels)", x)
	}
	if _, again := MigrateLayout("Pack", out); again {
		t.Error("migrated twice")
	}
	if _, box := MigrateLayout("Box", json.RawMessage(old)); box {
		t.Error("a model without net moves changed")
	}

	// Each face's old net width = its shown part now; faces after it moved by the net moves.
	m, _ := ModelFor("Pack")
	faces := map[string]Face{}
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

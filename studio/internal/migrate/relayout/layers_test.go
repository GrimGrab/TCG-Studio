package relayout

import (
	"encoding/json"
	"math"
	"testing"

	"tcgstudio/internal/uvmap"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

// TestLayerIdentity: between identical layouts a map layer comes back as it was (any turn, mirror, skew), clipped to its area.
func TestLayerIdentity(t *testing.T) {
	m := uvmap.Model{TextureSize: 1024, Faces: []uvmap.Face{
		{ID: "front", Targets: []uvmap.Target{{Rect: [4]float64{100, 100, 500, 400}, Src: [4]float64{0, 0, 1, 1}, Transpose: true, FlipX: true}}}}}
	for _, l := range []layer{
		{"space": "texture", "x": 300.0, "y": 250.0, "w": 120.0, "h": 80.0, "rot": 0.0},
		{"space": "texture", "x": 300.0, "y": 250.0, "w": 120.0, "h": 80.0, "rot": 33.0, "mirror": true},
		{"space": "texture", "x": 300.0, "y": 250.0, "w": 120.0, "h": 80.0, "rot": -90.0, "skew": 0.25},
	} {
		out := movePieces(m, m, l)
		if len(out) != 1 {
			t.Fatalf("%v: %d pieces", l, len(out))
		}
		o := out[0]
		for _, k := range []string{"x", "y", "w", "h", "rot"} {
			if !near(o[k].(float64), l[k].(float64)) {
				t.Errorf("%v: %s = %v", l, k, o[k])
			}
		}
		lm, _ := l["mirror"].(bool)
		om, _ := o["mirror"].(bool)
		ls, _ := l["skew"].(float64)
		os, _ := o["skew"].(float64)
		if lm != om || !near(ls, os) {
			t.Errorf("%v: mirror %v skew %v", l, om, os)
		}
	}
}

// TestLayerOldBoxFront: a picture on the old box front lands on the new front and back, each clipped to its area.
func TestLayerOldBoxFront(t *testing.T) {
	from := uvmap.Model{TextureSize: 1024, Faces: []uvmap.Face{
		{ID: "front", Label: "Front", Targets: []uvmap.Target{{Rect: [4]float64{202, 153, 561, 374}, Src: [4]float64{0, 0, 1, 1}}}}}}
	to := uvmap.Model{TextureSize: 2048, Faces: []uvmap.Face{
		{ID: "front", Targets: []uvmap.Target{
			{Rect: [4]float64{645, 1024, 1401, 1573}, Src: [4]float64{0, 0, 1, 1}},
			{Rect: [4]float64{645, 21, 1401, 570}, Src: [4]float64{0, 0, 1, 1}, FlipX: true, FlipY: true}}}}}
	raw, _ := json.Marshal(map[string]any{"version": 2, "layers": []layer{
		{"id": "a", "name": "Logo", "kind": "image", "space": "texture", "x": 381.5, "y": 263.5, "w": 359.0, "h": 221.0, "rot": 0.0},
		{"id": "b", "kind": "fill", "face": "top", "x": 0.0, "y": 0.0, "w": 0.0, "h": 0.0, "rot": 0.0}, // a face layer: unchanged
	}})
	got, err := Layout(from, to, raw)
	if err != nil {
		t.Fatal(err)
	}
	var l struct {
		Layers []layer `json:"layers"`
	}
	_ = json.Unmarshal(got, &l)
	if len(l.Layers) != 3 {
		t.Fatalf("%d layers: %v", len(l.Layers), l.Layers)
	}
	front, back := l.Layers[0], l.Layers[1]
	if !near(front["x"].(float64), 1023) || !near(front["y"].(float64), 1298.5) || !near(front["w"].(float64), 756) || !near(front["h"].(float64), 549) || !near(front["rot"].(float64), 0) {
		t.Errorf("front piece %v", front)
	}
	if r := math.Abs(back["rot"].(float64)); !near(back["x"].(float64), 1023) || !near(back["y"].(float64), 295.5) || !near(r, 180) {
		t.Errorf("back piece (turned 180°) %v", back)
	}
	if l.Layers[2]["id"] != "b" {
		t.Errorf("face layer changed: %v", l.Layers[2])
	}
}

// TestLayoutWithoutLayers: a design with no layers keeps an empty list (the editor reads layout.layers as a list, never null).
func TestLayoutWithoutLayers(t *testing.T) {
	m := uvmap.Model{TextureSize: 1024}
	for _, raw := range []string{`{"version":2,"layers":[]}`, `{"version":2}`} {
		got, err := Layout(m, m, json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		var l map[string]json.RawMessage
		_ = json.Unmarshal(got, &l)
		if string(l["layers"]) != "[]" {
			t.Errorf("%s → layers %s", raw, l["layers"])
		}
	}
}

// TestLayoutDropsRemovedFaces: a face layer on a face the new layout no longer has goes (the dice palette's swatches); one
// on a face both have stays.
func TestLayoutDropsRemovedFaces(t *testing.T) {
	from := uvmap.Model{Faces: []uvmap.Face{{ID: "box"}, {ID: "frame"}}}
	to := uvmap.Model{Faces: []uvmap.Face{{ID: "box"}, {ID: "top"}}}
	raw, _ := json.Marshal(map[string]any{"version": 2, "layers": []layer{
		{"id": "a", "kind": "fill", "face": "box", "color": "#ff0000"},
		{"id": "b", "kind": "fill", "face": "frame", "color": "#00ff00"},
	}})
	got, err := Layout(from, to, raw)
	if err != nil {
		t.Fatal(err)
	}
	var l struct {
		Layers []layer `json:"layers"`
	}
	_ = json.Unmarshal(got, &l)
	if len(l.Layers) != 1 || l.Layers[0]["id"] != "a" {
		t.Errorf("layers %v, want only a", l.Layers)
	}
}

// Package designs brings saved editor designs up to the models' current nets when a set loads: layers move with their faces,
// and a face that was split gets a copy of its layers on the new face. The moves themselves are on the models (uvmap NetMoves,
// NetCopies).
package designs

import (
	"encoding/json"

	"tcgstudio/internal/uvmap"
)

// migration:net-v2-pack
// migration:net-v3-box

// packArtKinds maps studio.json packArt entries to their editor models.
var packArtKinds = map[string]string{"pack": "Pack", "box": "Box"}

// PackArt brings a set's pack and box designs (studio.json packArt, by pack id) up to date; saved with the set's next save.
func PackArt(art map[string]map[string]json.RawMessage) {
	for _, layouts := range art {
		for which, raw := range layouts {
			if kind, ok := packArtKinds[which]; ok {
				if out, ok := Layout(kind, raw); ok {
					layouts[which] = out
				}
			}
		}
	}
}

// Layout moves the layers of a layout made for an older net of the model with the faces they sit on (a layer belongs
// to the face under its centre) and stamps the current uvmap.NetVersion; ok = changed. Other layouts come back unchanged.
func Layout(kind string, raw json.RawMessage) (json.RawMessage, bool) {
	m, found := uvmap.ModelFor(kind)
	var l map[string]json.RawMessage
	if !found || len(m.NetMoves)+len(m.NetCopies) == 0 || len(raw) == 0 || json.Unmarshal(raw, &l) != nil {
		return raw, false
	}
	net := 1
	if v, ok := l["net"]; ok && json.Unmarshal(v, &net) != nil {
		return raw, false
	}
	if net >= uvmap.NetVersion {
		return raw, false
	}
	var layers []map[string]any
	if v, ok := l["layers"]; ok && json.Unmarshal(v, &layers) != nil {
		return raw, false
	}
	for _, layer := range layers {
		x, ok := layer["x"].(float64)
		if !ok || layer["space"] == "texture" {
			continue // fills on the faces have no place; map layers are in texture pixels
		}
		nx := x
		for _, mv := range m.NetMoves {
			if mv.Version > net && x >= mv.From {
				nx += mv.DX
			}
		}
		layer["x"] = nx
	}
	for _, c := range m.NetCopies {
		if c.Version > net {
			layers = append(layers, copiesOn(layers, c)...)
		}
	}
	b, err := json.Marshal(layers)
	if err != nil {
		return raw, false
	}
	if layers != nil {
		l["layers"] = b
	}
	l["net"], _ = json.Marshal(uvmap.NetVersion)
	out, err := json.Marshal(l)
	if err != nil {
		return raw, false
	}
	return out, true
}

// copiesOn: copies of the layers on c's old face for its new face, in the same order.
func copiesOn(layers []map[string]any, c uvmap.NetCopy) []map[string]any {
	var out []map[string]any
	for _, layer := range layers {
		if layer["space"] == "texture" {
			continue // in texture pixels: stays where it is
		}
		n := map[string]any{}
		for k, v := range layer {
			n[k] = v
		}
		x, placed := layer["x"].(float64)
		y, _ := layer["y"].(float64)
		w, _ := layer["w"].(float64)
		h, _ := layer["h"].(float64)
		wholeFace := layer["kind"] == "fill" && !(w > 0 && h > 0) // a colour over its face: its x, y mean nothing (saved as 0, 0)
		switch {
		case wholeFace && layer["face"] == c.From:
			n["face"] = c.To
		case wholeFace: // another face's, or every face's (which covers the new faces already)
			continue
		case placed && x >= c.Area[0] && x < c.Area[0]+c.Area[2] && y >= c.Area[1] && y < c.Area[1]+c.Area[3]:
			n["x"], n["y"] = x+c.DX, y+c.DY
		default:
			continue
		}
		if id, ok := layer["id"].(string); ok {
			n["id"] = id + "_" + c.To
		}
		if name, ok := layer["name"].(string); ok {
			n["name"] = name + " (" + c.To + ")"
		}
		out = append(out, n)
	}
	return out
}

package uvmap

import "encoding/json"

// NetVersion is the net version of layouts made now (Layout "net" field; missing = 1). Bump it with a model's NetMoves.
const NetVersion = 2

// NetMove: from layout net Version on, layers centred at net x >= From (in the older net) sit DX further right.
type NetMove struct {
	Version  int
	From, DX float64
}

// MigrateLayout moves the layers of a layout made for an older net of the model with the faces they sit on (a layer belongs
// to the face under its centre) and stamps the current NetVersion; ok = changed. Other layouts come back unchanged.
func MigrateLayout(kind string, raw json.RawMessage) (json.RawMessage, bool) {
	m, found := ModelFor(kind)
	var l map[string]json.RawMessage
	if !found || len(m.NetMoves) == 0 || len(raw) == 0 || json.Unmarshal(raw, &l) != nil {
		return raw, false
	}
	net := 1
	if v, ok := l["net"]; ok && json.Unmarshal(v, &net) != nil {
		return raw, false
	}
	if net >= NetVersion {
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
	b, err := json.Marshal(layers)
	if err != nil {
		return raw, false
	}
	if layers != nil {
		l["layers"] = b
	}
	l["net"], _ = json.Marshal(NetVersion)
	out, err := json.Marshal(l)
	if err != nil {
		return raw, false
	}
	return out, true
}

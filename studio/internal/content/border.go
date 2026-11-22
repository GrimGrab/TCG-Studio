package content

import (
	"bytes"
	"encoding/json"
)

// renameBorderShaders points an imported border's shader layers at the new ids of shaders imported with it. Every other
// field is kept as it is (the entry is handled as a JSON object, like all entries).
func renameBorderShaders(raw []byte, renamed map[string]string) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	m := map[string]any{}
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	layers, _ := m["layers"].([]any)
	changed := false
	for _, l := range layers {
		layer, ok := l.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := layer["shader"].(string); ok {
			if to, ok := renamed[id]; ok {
				layer["shader"] = to
				changed = true
			}
		}
	}
	if !changed {
		return raw, nil
	}
	return json.Marshal(m)
}

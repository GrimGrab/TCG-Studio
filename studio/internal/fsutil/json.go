package fsutil

import "encoding/json"

// WalkStrings calls f for every string value in a decoded JSON tree (any depth).
func WalkStrings(v any, f func(string)) {
	switch t := v.(type) {
	case string:
		f(t)
	case []any:
		for _, x := range t {
			WalkStrings(x, f)
		}
	case map[string]any:
		for _, x := range t {
			WalkStrings(x, f)
		}
	}
}

// MapStrings replaces every string value in a decoded JSON tree by f's result (in place; returns the new root).
func MapStrings(v any, f func(string) string) any {
	switch t := v.(type) {
	case string:
		return f(t)
	case []any:
		for i := range t {
			t[i] = MapStrings(t[i], f)
		}
	case map[string]any:
		for k := range t {
			t[k] = MapStrings(t[k], f)
		}
	}
	return v
}

// MapJSONStrings decodes doc, maps its string values through f and encodes it again. On a decode error doc is returned
// unchanged.
func MapJSONStrings(doc []byte, f func(string) string) []byte {
	var v any
	if json.Unmarshal(doc, &v) != nil {
		return doc
	}
	b, err := json.Marshal(MapStrings(v, f))
	if err != nil {
		return doc
	}
	return b
}

// JSONStrings returns every string value of doc (any depth), in document-walk order, or nil when doc isn't JSON.
func JSONStrings(doc []byte) []string {
	var v any
	if json.Unmarshal(doc, &v) != nil {
		return nil
	}
	var out []string
	WalkStrings(v, func(s string) { out = append(out, s) })
	return out
}

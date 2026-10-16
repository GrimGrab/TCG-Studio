package accessories

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"tcgstudio/internal/setfmt"
)

// Raw access to the library's lists by their JSON name ("accessories", "furniture", "decorations", …), for code that
// handles every kind of entry the same way (internal/content: catalog, sharing, setup files). Entries are JSON objects
// with an "id"; the typed lists stay the source of truth (Save writes them).

// ListNames is the JSON name of every entry list of setfmt.AccessoryLibrary, in field order.
func ListNames() []string {
	t := reflect.TypeOf(setfmt.AccessoryLibrary{})
	var out []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Type.Kind() != reflect.Slice {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name != "" && name != "-" {
			out = append(out, name)
		}
	}
	return out
}

// RawList returns list name's entries as JSON objects, in list order.
func (l *Library) RawList(name string) ([]json.RawMessage, error) {
	lists, err := l.rawLists()
	if err != nil {
		return nil, err
	}
	raw, ok := lists[name]
	if !ok || string(raw) == "null" {
		if !hasList(name) {
			return nil, fmt.Errorf("the accessory library has no list %q", name)
		}
		return nil, nil
	}
	var out []json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SetRawList replaces list name's entries (JSON objects); fields the typed entry doesn't know are dropped.
func (l *Library) SetRawList(name string, entries []json.RawMessage) error {
	if !hasList(name) {
		return fmt.Errorf("the accessory library has no list %q", name)
	}
	lists, err := l.rawLists()
	if err != nil {
		return err
	}
	if entries == nil {
		entries = []json.RawMessage{}
	}
	b, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	lists[name] = b
	all, err := json.Marshal(lists)
	if err != nil {
		return err
	}
	lib := &setfmt.AccessoryLibrary{}
	if err := json.Unmarshal(all, lib); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if lib.Accessories == nil {
		lib.Accessories = []setfmt.Accessory{}
	}
	l.Lib = lib
	return nil
}

func (l *Library) rawLists() (map[string]json.RawMessage, error) {
	b, err := json.Marshal(l.Lib)
	if err != nil {
		return nil, err
	}
	m := map[string]json.RawMessage{}
	return m, json.Unmarshal(b, &m)
}

func hasList(name string) bool {
	for _, n := range ListNames() {
		if n == name {
			return true
		}
	}
	return false
}

// EntryID reads the "id" of a raw entry ("" when it has none).
func EntryID(raw json.RawMessage) string {
	var e struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &e)
	return e.ID
}

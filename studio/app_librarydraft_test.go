package main

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"tcgstudio/internal/accessories"
	astore "tcgstudio/internal/assets"
	"tcgstudio/internal/setfmt"
)

func draftLibrary(t *testing.T) *accessories.Library {
	t.Helper()
	ws := t.TempDir()
	l, err := accessories.Open(filepath.Join(ws, "setup"), astore.For(ws))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a1", "a2", "a3"} {
		l.Put(setfmt.NewAccessory("Playmat", id, id, ""))
	}
	l.PutFurniture(setfmt.Furniture{ID: "f1", Type: "Shelf", Name: "F1"})
	return l
}

func draftItem(t *testing.T, v any, layout, icon string) LibraryDraftItem {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return LibraryDraftItem{Item: b, Layout: layout, Icon: icon}
}

func TestApplyLibraryDraft(t *testing.T) {
	l := draftLibrary(t)
	changed := setfmt.NewAccessory("Playmat", "a2", "Renamed", "")
	added := setfmt.NewAccessory("Deckbox", "n1", "New", "")
	png := base64.StdEncoding.EncodeToString([]byte("\x89PNG fake"))
	d := LibraryDraft{
		Order: []string{"n1", "a3", "a2", "a1"},
		Items: []LibraryDraftItem{draftItem(t, changed, `{"version":2}`, ""), draftItem(t, added, "", png)},
	}
	if err := applyLibraryDraft(l, listAccessories, d); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, a := range l.Lib.Accessories {
		got = append(got, a.ID)
	}
	if want := []string{"n1", "a3", "a2", "a1"}; len(got) != 4 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Fatalf("order = %v", got)
	}
	if a := l.Lib.Accessories[2]; a.Name != "Renamed" || string(l.Meta.Layouts["a2"]) != `{"version":2}` {
		t.Fatalf("changed entry = %+v, layout %s", a, l.Meta.Layouts["a2"])
	}
	if a := l.Lib.Accessories[0]; !astore.IsAsset(a.Icon) {
		t.Fatalf("new entry's icon = %q, want a store path", a.Icon)
	}
}

func TestApplyLibraryDraftRejects(t *testing.T) {
	for name, d := range map[string]LibraryDraft{
		"id of another list": {Items: []LibraryDraftItem{{Item: json.RawMessage(`{"id":"f1","kind":"Playmat"}`)}}},
		"bad id":             {Items: []LibraryDraftItem{{Item: json.RawMessage(`{"id":"../x","kind":"Playmat"}`)}}},
		"unknown kind":       {Items: []LibraryDraftItem{{Item: json.RawMessage(`{"id":"z1","kind":"Nope"}`)}}},
		"delete missing":     {Deleted: []string{"nothere"}},
		"bad layout":         {Items: []LibraryDraftItem{{Item: json.RawMessage(`{"id":"a1","kind":"Playmat"}`), Layout: "{"}}},
	} {
		if err := applyLibraryDraft(draftLibrary(t), listAccessories, d); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := applyLibraryDraft(draftLibrary(t), "sets", LibraryDraft{}); err == nil {
		t.Error("unknown list accepted")
	}
}

func TestFreshName(t *testing.T) {
	a := freshName("images/p1_pack_texture.png")
	if !strings.HasPrefix(a, "images/p1_pack_texture.v") || !strings.HasSuffix(a, ".png") {
		t.Fatalf("freshName = %q", a)
	}
	if b := freshName(a); strings.Count(b, ".v") != 1 || !strings.HasPrefix(b, "images/p1_pack_texture.v") {
		t.Fatalf("a second freshName stacks marks: %q", b)
	}
}

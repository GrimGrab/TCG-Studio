package main

import (
	"testing"

	"tcgstudio/internal/origin"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

// Converted sets are grouped by the mod they came from (as the Catalog names it), database imports by their game; a
// collection the player typed is kept and image-folder sets stay empty.
func TestFillCollection(t *testing.T) {
	a := NewApp()
	cases := []struct {
		name, typed string
		meta        project.Meta
		want        string
	}{
		{"epl origin", "", project.Meta{Source: "epl", SourceDir: `C:\mods\Pack.zip`, Origin: &origin.Origin{Mod: "Gundam Cards"}}, "Gundam Cards"},
		{"epl before origins", "", project.Meta{Source: "epl", SourceDir: `C:\mods\RetroMon.zip`}, "RetroMon"},
		{"typed kept", "Mine", project.Meta{Source: "epl", Origin: &origin.Origin{Mod: "Gundam Cards"}}, "Mine"},
		{"image folder", "", project.Meta{Source: "folder", SourceDir: `C:\pics`}, ""},
		{"scryfall", "", project.Meta{Source: "scryfall"}, "Magic: The Gathering"},
	}
	for _, c := range cases {
		m := c.meta
		p := &project.Project{Set: &setfmt.Set{Collection: c.typed}, Meta: &m}
		a.fillCollection(p)
		if p.Set.Collection != c.want {
			t.Errorf("%s: collection %q, want %q", c.name, p.Set.Collection, c.want)
		}
	}
}

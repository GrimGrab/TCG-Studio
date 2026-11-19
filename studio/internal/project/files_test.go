package project

import (
	"encoding/json"
	"slices"
	"testing"

	"tcgstudio/internal/setfmt"
)

// A font a pack/box text layer names is one of the set's files (Storage keeps it, setup exports carry it), like the images.
func TestFilesCountLayerFonts(t *testing.T) {
	layout := json.RawMessage(`{"layers":[{"kind":"text","text":"BOOSTER","font":"fonts/almendra-regular.ttf"},
		{"kind":"image","src":"images/auto_front.png"}]}`)
	p := &Project{Set: &setfmt.Set{}, Meta: &Meta{PackArt: map[string]map[string]json.RawMessage{"booster": {"pack": layout}}}}
	files := p.Files()
	for _, want := range []string{"fonts/almendra-regular.ttf", "images/auto_front.png"} {
		if !slices.Contains(files, want) {
			t.Errorf("Files() = %v, want it to include %s", files, want)
		}
	}
	if IsRelFile(`C:\Windows\Fonts\arial.ttf`) || IsRelFile("fonts/notes.txt") {
		t.Error("only relative image and font paths are set files")
	}
}

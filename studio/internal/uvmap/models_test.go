package uvmap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tcgstudio/internal/setfmt"
)

// templatesDir holds the game templates' accessories folder (TCG_TEMPLATES, or TCG Studio's extraction cache). Tests skip without it.
func templatesDir(t *testing.T) string {
	dir := os.Getenv("TCG_TEMPLATES")
	if dir == "" {
		cache, _ := os.UserCacheDir()
		dir = filepath.Join(cache, "TCG Studio", "game-templates", "accessories")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Skip("no game templates (run TCG Studio once, or set TCG_TEMPLATES): ", dir)
	}
	return dir
}

// notMovedYet reports whether a layout is a hand-measured one of game 1.02 whose model these templates no longer have
// (the game replaced it and the layout hasn't moved to the new model yet): logged, not failed.
func notMovedYet(t *testing.T, dir, kind, mesh string) bool {
	if _, err := os.Stat(filepath.Join(dir, mesh+".obj")); err == nil {
		return false
	}
	if _, current := faceMapOf(kind); current {
		return false // a derived layout's model must be there
	}
	t.Logf("%s: model %s isn't in these templates (layout not moved to the game's new model yet)", kind, mesh)
	return true
}

// tabletopBases lists the items with their own 1.02 layout.
func tabletopBases() []string {
	var out []string
	for b := range tabletop {
		out = append(out, b)
	}
	return out
}

func TestPreviewMeshesExported(t *testing.T) {
	dir := templatesDir(t)
	for _, kind := range append([]string{"Deckbox", "Playmat", "Sleeve", "Dice", "Comic", "Binder", "BattleDeck"}, tabletopBases()...) {
		m, ok := ModelFor(kind)
		if !ok {
			m, ok = tabletop[kind]
		}
		if !ok || len(m.Preview) == 0 {
			t.Fatalf("%s: no preview parts", kind)
		}
		for _, p := range m.Preview {
			if notMovedYet(t, dir, kind, p.Mesh) {
				continue
			}
			if _, err := os.Stat(filepath.Join(dir, p.Mesh+".obj")); err != nil {
				t.Errorf("%s: preview mesh %s not exported", kind, p.Mesh)
			}
		}
	}
}

// sharesLayout: item meshes that read the same texture areas as another model's layout (the rolled playmat sold in the shop
// shows the table mat's printed surface).
var sharesLayout = map[string]string{"PlayMatMesh_low": "PlayMatFlatMesh"}

// TestLayoutsAreTheItemsModels checks that every accessory base's layout is made for the model the game draws that item with
// (its mesh in the templates): a game update that swaps a model (the 2026-10-27 *_low meshes) otherwise goes unnoticed while
// the old mesh is still in the files. A derived layout must match; a hand-measured one still on an old model is logged.
func TestLayoutsAreTheItemsModels(t *testing.T) {
	dir := templatesDir(t)
	var tpl struct {
		Items []struct {
			Type, Mesh, MeshSecondaryFile string
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, "accessories.json"))
	if err == nil {
		err = json.Unmarshal(b, &tpl)
	}
	if err != nil {
		t.Fatal(err)
	}
	meshOf := map[string][2]string{}
	for _, it := range tpl.Items {
		meshOf[it.Type] = [2]string{strings.TrimSuffix(it.Mesh, ".obj"), strings.TrimSuffix(it.MeshSecondaryFile, ".obj")}
	}
	for _, k := range setfmt.AccessoryKinds {
		for _, base := range k.Bases {
			m, ok := ModelForBase(k.Kind, base)
			have, found := meshOf[base]
			if !ok || !found || m.Mesh == have[0] || m.Mesh == have[1] || sharesLayout[have[0]] == m.Mesh {
				continue // no painted layout (figurines), or not in these templates
			}
			if _, current := faceMapOf(k.Kind); current {
				t.Errorf("%s %s: layout made for %s, the game draws %s", k.Kind, base, m.Mesh, have[0])
			} else {
				t.Logf("%s %s: layout still made for %s, the game draws %s (not moved to the new model yet)", k.Kind, base, m.Mesh, have[0])
			}
		}
	}
}

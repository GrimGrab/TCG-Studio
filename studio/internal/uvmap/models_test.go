package uvmap

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// templatesDir holds the mod's export (set TCG_TEMPLATES, or the default Steam install path). Tests skip without it.
func templatesDir(t *testing.T) string {
	dir := os.Getenv("TCG_TEMPLATES")
	if dir == "" {
		dir = `D:\SteamLibrary\steamapps\common\TCG Card Shop Simulator\BepInEx\plugins\TCGCustomCards\templates\accessories`
	}
	if _, err := os.Stat(dir); err != nil {
		t.Skip("accessory templates not exported: ", dir)
	}
	return dir
}

// TestModelsMatchMeshes checks every non-bleed target against the exported mesh: each target rectangle must be covered by
// UV islands (within a few pixels) — so a game update that changes the models is noticed.
func TestModelsMatchMeshes(t *testing.T) {
	dir := templatesDir(t)
	type check struct {
		kind, mesh string
		faces      []Face
	}
	var checks []check
	for _, kind := range []string{"Deckbox", "Playmat", "Sleeve", "Comic", "Binder", "BattleDeck", "Pack", "Box"} { // Dice is a palette (swatches, not UV islands)
		m, _ := ModelFor(kind)
		checks = append(checks, check{kind, m.Mesh, m.Faces})
	}
	// Box bases read other atlas areas through the Rare/Epic/Legendary box meshes.
	for i, b := range box.Bases[1:4] {
		var faces []Face
		for _, f := range box.Faces {
			faces = append(faces, Face{ID: f.ID, Targets: b.Targets[f.ID]})
		}
		checks = append(checks, check{b.ID, fmt.Sprintf("CardBoxMesh_%d", i+2), faces})
	}
	for _, c := range checks {
		kind := c.kind
		m := Model{Faces: c.faces, TextureSize: 1024}
		mesh, err := LoadOBJ(filepath.Join(dir, c.mesh+".obj"))
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		panels := mesh.Panels(0.002)
		px := func(p Panel) [4]float64 {
			s := float64(m.TextureSize)
			return [4]float64{p.UVMin[0] * s, (1 - p.UVMax[1]) * s, p.UVMax[0] * s, (1 - p.UVMin[1]) * s}
		}
		for _, f := range m.Faces {
			for _, tg := range f.Targets {
				if tg.Bleed {
					continue
				}
				// Union of the panels overlapping the target must match the target within tolerance.
				u := [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
				for _, p := range panels {
					r := px(p)
					if r[0] < tg.Rect[2]-4 && r[2] > tg.Rect[0]+4 && r[1] < tg.Rect[3]-4 && r[3] > tg.Rect[1]+4 {
						u = [4]float64{math.Min(u[0], r[0]), math.Min(u[1], r[1]), math.Max(u[2], r[2]), math.Max(u[3], r[3])}
					}
				}
				for k := 0; k < 4; k++ {
					// Neighbouring islands may extend the union; the target must lie inside it and touch it on each side
					// within 30 px (strip faces share their height with neighbours).
					inside := (k < 2 && u[k] <= tg.Rect[k]+6) || (k >= 2 && u[k] >= tg.Rect[k]-6)
					if !inside {
						t.Errorf("%s/%s target %v not covered by mesh UVs (union %v)", kind, f.ID, tg.Rect, u)
						break
					}
				}
			}
		}
	}
}

func TestDeckboxOrientation(t *testing.T) {
	dir := templatesDir(t)
	mesh, err := LoadOBJ(filepath.Join(dir, "DeckBox.obj"))
	if err != nil {
		t.Fatal(err)
	}
	// The outer front cover: u grows with +x (viewer's right), v with +y (up) — i.e. upright, not mirrored.
	for _, p := range mesh.Panels(0.002) {
		if p.Face == "front" && p.PosMin[2] < -0.45 && p.AreaUV > 0.1 {
			if p.Affine[0][0] <= 0 || p.Affine[1][1] <= 0 {
				t.Fatalf("front cover mirrored: affine %v", p.Affine)
			}
			return
		}
	}
	t.Fatal("front cover panel not found")
}

// TestBattleDeckOrientation: the front is upright; the back (incl. the tab's back) lies transposed in the texture — seen from
// behind, the face's top (+y) runs to the texture's left (u falls) and the face's left (+x) to the texture's top (v rises).
func TestBattleDeckOrientation(t *testing.T) {
	dir := templatesDir(t)
	mesh, err := LoadOBJ(filepath.Join(dir, "PerconDeck_Mesh.obj"))
	if err != nil {
		t.Fatal(err)
	}
	var front, back bool
	for _, p := range mesh.Panels(0.02) {
		switch {
		case p.Face == "front" && p.AreaUV > 0.3:
			front = true
			if p.Affine[0][0] <= 0 || p.Affine[1][1] <= 0 {
				t.Errorf("front mirrored: affine %v", p.Affine)
			}
		case p.Face == "back" && p.AreaUV > 0.05:
			back = true
			if p.Affine[0][1] >= 0 || p.Affine[1][0] <= 0 {
				t.Errorf("back not transposed as modelled: affine %v", p.Affine)
			}
		}
	}
	if !front || !back {
		t.Fatalf("panels not found (front %v, back %v)", front, back)
	}
}

func TestPreviewMeshesExported(t *testing.T) {
	dir := templatesDir(t)
	for _, kind := range []string{"Deckbox", "Playmat", "Sleeve", "Dice", "Comic", "Binder", "BattleDeck"} {
		m, ok := ModelFor(kind)
		if !ok || len(m.Preview) == 0 {
			t.Fatalf("%s: no preview parts", kind)
		}
		for _, p := range m.Preview {
			if _, err := os.Stat(filepath.Join(dir, p.Mesh+".obj")); err != nil {
				t.Errorf("%s: preview mesh %s not exported", kind, p.Mesh)
			}
		}
	}
}

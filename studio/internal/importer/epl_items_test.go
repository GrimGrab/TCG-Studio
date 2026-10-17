package importer

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/epl"
)

// Kind and shop of EPL items on the game's tabletop models: the model decides the kind, EPL's AddItemAs… flags the shop;
// a board game or card supply sold as a figurine becomes a figurine with that model (Marvel and board game mods do this).
func TestEPLTabletopItems(t *testing.T) {
	FigurineToys = []ToySize{{Type: "Boardgame_Speedrobo_Mafia", Dim: [3]float64{3, 4, 1}, Tall: true},
		{Type: "Boardgame_Speedrobo_Claim", Dim: [3]float64{1.5, 4, 1}}, {Type: "Toy_GolemA", Dim: [3]float64{3, 4, 1}, Tall: true}}
	defer func() { FigurineToys = nil }()
	for _, c := range []struct {
		name             string
		it               epl.Item
		kind, base, shop string
	}{
		{"board game in the figurine tab", epl.Item{ItemCategory: "Figurine", MeshToUse: "Boardgame_Speedrobo_Mafia", AddItemAsFigurine: true,
			ItemDeminsion: epl.Vec3{X: 3, Y: 4, Z: 1}, IsTallItem: true}, "Figurine", "Toy_GolemA", ""},
		{"board game category sold as a figurine", epl.Item{ItemCategory: "Boardgame", MeshToUse: "Boardgame_Speedrobo_Mafia", AddItemAsFigurine: true},
			"Figurine", "Toy_GolemA", ""},
		{"card supply sold as a figurine", epl.Item{MeshToUse: "UP_Toploader", AddItemAsFigurine: true}, "Figurine", "Toy_GolemA", ""},
		{"sleeve sold as a figurine keeps its kind", epl.Item{MeshToUse: "CardSleeve_Clear", AddItemAsFigurine: true}, "Sleeve", "CardSleeve_Clear", "figurines"},
		{"board game in its own shop", epl.Item{ItemCategory: "Boardgame", MeshToUse: "Boardgame_Speedrobo_Claim", AddItemAsBoardGame: true},
			"BoardGame", "Boardgame_Speedrobo_Claim", ""},
		{"no flags: the base's shop", epl.Item{MeshToUse: "boardgame_speedrobo_claim"}, "BoardGame", "Boardgame_Speedrobo_Claim", ""},
		{"card supply", epl.Item{MeshToUse: "UP_Toploader", AddItemAsBoardGame: true}, "CardSupply", "UP_Toploader", ""},
		{"playmat tube of another colour", epl.Item{MeshToUse: "UP_Playmat3", AddItemAsBoardGame: true}, "CardSupply", "UP_Playmat1", ""},
		{"deck box sold as a board game", epl.Item{MeshToUse: "DeckBox1", AddItemAsBoardGame: true}, "Deckbox", "DeckBox1", "tabletop"},
		{"own model board game", epl.Item{Mesh: "MyGame", AddItemAsBoardGame: true, ItemDeminsion: epl.Vec3{X: 3, Y: 4, Z: 1}, IsTallItem: true},
			"BoardGame", "Boardgame_Speedrobo_Mafia", ""},
	} {
		kind, base, reason := eplItemKind(&c.it)
		if kind != c.kind || base != c.base {
			t.Errorf("%s: got %q %q (%s), want %q %q", c.name, kind, base, reason, c.kind, c.base)
			continue
		}
		if shop := eplShop(&c.it, kind); shop != c.shop {
			t.Errorf("%s: shop %q, want %q", c.name, shop, c.shop)
		}
	}
}

// A game model from the templates (Unity's left-handed mesh space) is mirrored into a right-handed scene once, so the
// bake (figurine.Place mirrors back) gives the game's own orientation and winding; UVs keep the template's.
func TestGameModelScene(t *testing.T) {
	obj := filepath.Join(t.TempDir(), "m.obj")
	os.WriteFile(obj, []byte("g submesh0\nv 0 0 1\nv 1 0 1\nv 0 1 2\nvt 0 0\nvt 1 0\nvt 0 1\nvn 0 0 1\nvn 0 0 1\nvn 0 0 1\nf 1/1/1 2/2/2 3/3/3\n"), 0o644)
	GameModel = func(string) (string, error) { return obj, nil }
	defer func() { GameModel = nil }()
	s, err := gameModelScene("Boardgame_Speedrobo_Mafia", image.NewNRGBA(image.Rect(0, 0, 2, 2)))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Parts) != 1 || len(s.Materials) != 1 || s.Parts[0].Mat != 0 {
		t.Fatalf("want one part on the item's texture, got %d parts, %d materials", len(s.Parts), len(s.Materials))
	}
	p := s.Parts[0]
	if p.Pos[p.Idx[0]][2] != -1 || p.Nrm[0][2] != -1 {
		t.Errorf("z not mirrored: %v %v", p.Pos, p.Nrm)
	}
	// Winding swapped: the second index now points at the file's third vertex.
	if p.Pos[p.Idx[1]] != [3]float64{0, 1, -2} {
		t.Errorf("winding not swapped: idx %v pos %v", p.Idx, p.Pos)
	}
	if uv := p.UV[p.Idx[0]]; uv != [2]float64{0, 1} {
		t.Errorf("uv %v, want the template's flipped to image space (0,1)", uv)
	}
}

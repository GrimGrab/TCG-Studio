package unityfs

import (
	"os"
	"path/filepath"
	"testing"
)

// gameData is the game's data folder (TCG_GAME = game folder, or the default Steam path). Tests skip without it.
func gameData(t *testing.T) string {
	dir := os.Getenv("TCG_GAME")
	if dir == "" {
		dir = `D:\SteamLibrary\steamapps\common\TCG Card Shop Simulator`
	}
	data := filepath.Join(dir, "Card Shop Simulator_Data")
	if _, err := os.Stat(data); err != nil {
		t.Skip("game not installed: ", dir)
	}
	return data
}

func TestEnums(t *testing.T) {
	data := gameData(t)
	e, err := ReadEnums(filepath.Join(data, "Managed", "Assembly-CSharp.dll"), "EItemType", "EItemCategory")
	if err != nil {
		t.Fatal(err)
	}
	it := e["EItemType"]
	if it["BasicCardPack"] != 0 || it["Toy_PiggyA"] != 33 || it["Max"] < 130 {
		t.Fatalf("EItemType: BasicCardPack=%d Toy_PiggyA=%d Max=%d", it["BasicCardPack"], it["Toy_PiggyA"], it["Max"])
	}
	if e["EItemCategory"]["Figurine"] != 7 {
		t.Fatalf("EItemCategory.Figurine = %d", e["EItemCategory"]["Figurine"])
	}
}

func TestReadBasics(t *testing.T) {
	env, err := Open(gameData(t))
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	f, err := env.File("sharedassets1.assets")
	if err != nil {
		t.Fatal(err)
	}
	var meshes, texs int
	for _, o := range f.Objects {
		name, _ := o.Name()
		switch {
		case o.ClassID == ClassMesh && name == "PiggyA_Mesh":
			m, err := ReadMesh(env, o)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("mesh %s: %d vertices, %d uv, %d normals, submeshes %d, bounds %v", m.Name, len(m.Pos), len(m.UV), len(m.Normal), len(m.Subs), m.Bounds())
			meshes++
		case o.ClassID == ClassTexture2D && name == "T_PiggyA":
			tx, err := ReadTexture2D(o)
			if err != nil {
				t.Fatal(err)
			}
			img, err := env.TextureImage(tx)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("texture %s %dx%d fmt %d colorspace %d, pixel(1024,1024)=%v", tx.Name, tx.Width, tx.Height, tx.Format, tx.ColorSpace, img.NRGBAAt(1024, 1024))
			texs++
		}
	}
	if meshes == 0 || texs == 0 {
		t.Fatalf("found %d meshes, %d textures", meshes, texs)
	}
	for _, name := range []string{"globalgamemanagers.assets", "sharedassets0.assets", "sharedassets1.assets", "resources.assets"} {
		f, err := env.File(name)
		if err != nil {
			t.Fatal(err)
		}
		n := env.FindScripts(f, "StockItemData_ScriptableObject", "Shelf", "ShelfCompartment", "Item")
		counts := map[string]int{}
		for _, o := range n {
			counts[env.ScriptClass(o)]++
		}
		t.Logf("%s: %v", name, counts)
	}
}

func TestStockItemData(t *testing.T) {
	env, err := Open(gameData(t))
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	f, _ := env.File("sharedassets1.assets")
	so := env.FindScripts(f, "StockItemData_ScriptableObject")
	if len(so) != 1 {
		t.Fatalf("found %d item databases", len(so))
	}
	s, err := env.ReadStockItemData(so[0])
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("items %d, meshes %d, restock %d, shown figurines %v", len(s.Items), len(s.Meshes), len(s.Restock), s.ShownFigurine)
	for _, i := range []int{0, 33, 70, 127} {
		d := s.Items[i]
		t.Logf("%d %q cat %d cost %v market %v-%v tall %v dim %v posY %v scale %v mesh %+v", i, d.Name, d.Category, d.BaseCost,
			d.MarketPriceMinPercent, d.MarketPriceMaxPercent, d.IsTallItem, d.ItemDimension, d.PosYOffsetInBox, d.ScaleOffsetInBox, s.Meshes[i].Name)
	}
	if len(s.Restock) > 0 {
		t.Logf("restock[0] %+v, last %+v", s.Restock[0], s.Restock[len(s.Restock)-1])
	}
}

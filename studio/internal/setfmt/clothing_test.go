package setfmt

import "testing"

func TestValidateClothes(t *testing.T) {
	ok := Clothing{ID: "a1", Name: "Shop tee", Body: "male", Piece: "TShirt_01", Parts: []ClothingPart{{Tint: "#ffffffff"}}, Worn: 10}
	bad := Clothing{ID: "a1", Name: "", Body: "robot", Piece: "", Parts: []ClothingPart{{Tint: "white"}}, Worn: 101}
	l := AccessoryLibrary{Clothes: []Clothing{ok}, Customers: []Customer{{ID: "c", Name: "Gary", Outfit: []OutfitSlot{{Slot: 0, Piece: "TShirt_01", Clothing: "a1"}, {Slot: 1, Piece: "Jeans_01", Clothing: "gone"}}}}}
	errs, warns := l.ValidateClothes()
	if len(errs) != 0 || len(warns) != 1 {
		t.Fatalf("good list: errs %v warns %v", errs, warns)
	}
	l.Clothes = append(l.Clothes, bad)
	errs, _ = l.ValidateClothes()
	// duplicate id, name, body, piece, colour, worn
	if len(errs) != 6 {
		t.Fatalf("bad look: %d errors %v", len(errs), errs)
	}
}

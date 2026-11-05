package main

import (
	"testing"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/setfmt"
)

func TestSyncFaces(t *testing.T) {
	old := setfmt.CustomerFace{HairColor: "#000000"}
	edited := setfmt.CustomerFace{HairColor: "#ffffff"}
	lib := &setfmt.AccessoryLibrary{
		Faces: []setfmt.SavedFace{{ID: "f", Body: "male", Model: 3, Face: old, Portrait: "assets/face.png"}},
		Customers: []setfmt.Customer{
			{ID: "same", FaceID: "f", Model: 3, Face: &setfmt.CustomerFace{HairColor: "#000000"}, Portrait: "assets/own-outfit.png"},
			{ID: "stale", FaceID: "f", Model: 3, Face: &edited, Portrait: "assets/stale.png"},
			{ID: "gone", FaceID: "deleted", Model: 1, Face: &edited, Portrait: "assets/kept.png"},
		},
		Employees: []setfmt.Employee{{ID: "e", FaceID: "f", Model: 9, Portrait: "assets/old.png"}},
	}
	syncFaces(&accessories.Library{Lib: lib})
	c := lib.Customers
	if c[0].Portrait != "assets/own-outfit.png" {
		t.Errorf("an unchanged face replaced their own portrait: %q", c[0].Portrait)
	}
	if c[1].Portrait != "assets/face.png" || c[1].Face.HairColor != "#000000" {
		t.Errorf("someone with an old copy didn't follow the face: %+v", c[1])
	}
	if c[2].FaceID != "" || c[2].Face == nil || c[2].Portrait != "assets/kept.png" {
		t.Errorf("a deleted face should leave them their copy: %+v", c[2])
	}
	if e := lib.Employees[0]; e.Model != 3 || e.Face == nil || e.Portrait != "assets/face.png" {
		t.Errorf("an employee didn't follow the face: %+v", e)
	}
}

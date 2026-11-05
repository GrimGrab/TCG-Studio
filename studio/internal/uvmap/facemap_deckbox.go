package uvmap

// Deck box (DeckBox_low). How to read and edit this: facemap.go.
//
// Parts of the mesh not listed keep the base item's print; paint them with layers on the Texture view.
func faceMapDeckbox() Model {
	return faceMap(Model{Kind: "Deckbox", Mesh: "DeckBox_low", TextureSize: 1024, Icon: "box", Size: deckbox.Size,
		Preview: mainPart("DeckBox_low"), Faces: deckbox.Faces},
		[]Row{
			{Face: "left", Part: "Panel", Rect: [4]float64{422, 467, 631, 1018}, Show: [4]float64{0.022, 0.042, 0.978, 0.955}, Shape: [][2]float64{{423.17, 471.3}, {422, 482.24}, {422, 1018}, {627.64, 1018}, {627.64, 996.12}, {631, 494.98}, {629.83, 477.94}, {628.81, 471.3}, {625.44, 469.17}, {617.85, 467}, {430.8, 469.17}}},
			{Face: "front", Part: "Cover", Rect: [4]float64{6, 760, 205, 1018}, Show: [4]float64{0, 0.087, 1, 0.965}},
			{Face: "front", Part: "Lip", Rect: [4]float64{613, 718, 799, 728}, Show: [4]float64{0.031, 0.965, 0.969, 0.99}},
			{Face: "front", Part: "Strip", Rect: [4]float64{623, 1005, 820, 1018}, Show: [4]float64{0, 0.99, 1, 1}},
			{Face: "front", Part: "Cover copy", Rect: [4]float64{211, 773, 420, 1018}, Show: [4]float64{0, 0.087, 1, 0.965}},
			{Face: "right", Part: "Panel", Rect: [4]float64{788, 467, 992, 728}, Show: [4]float64{0.022, 0.042, 0.978, 0.955}, Shape: [][2]float64{{791.6, 717.72}, {788, 480.47}, {789.51, 472.14}, {791.6, 469.12}, {793.62, 467.8}, {799.86, 467}, {983.46, 467.8}, {990.49, 469.12}, {992, 474.28}, {992, 728}, {791.6, 728}}},
			{Face: "right", Part: "Panel copy", Rect: [4]float64{808, 728, 1024, 1018}, Show: [4]float64{0.022, 0.042, 0.978, 0.955}, Shape: [][2]float64{{811.82, 1006.58}, {808, 742.96}, {809.57, 733.72}, {811.82, 730.35}, {813.98, 728.88}, {820.55, 728}, {1014.95, 728.88}, {1022.43, 730.35}, {1024, 736.09}, {1024, 1018}, {811.82, 1018}}},
			{Face: "back", Rect: [4]float64{6, 292, 204, 569}, Flip: "xy"},
			{Face: "back", Part: "Back copy", Rect: [4]float64{210, 338, 420, 589}, Flip: "xy"},
			{Face: "top", Rect: [4]float64{6, 569, 204, 760}},
			{Face: "top", Part: "Lid copy", Rect: [4]float64{209, 589, 419, 773}},
			{Face: "bottom", Rect: [4]float64{6, 95, 204, 292}},
			{Face: "bottom", Part: "Bottom copy", Rect: [4]float64{211, 145, 421, 337}},
		})
}

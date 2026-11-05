package uvmap

// Card pack (CardPackStaticMesh_low). How to read and edit this: facemap.go.
func faceMapPack() Model {
	return faceMap(Model{Kind: "Pack", Mesh: "CardPackStaticMesh_low", TextureSize: 1024, Icon: "pack",
		Size:    [3]float64{0.678, 1.354, 0.148},
		Preview: []PreviewPart{{Mesh: "CardPackStaticMesh_low", Texture: "main"}},
		Bases:   sliceBases(pack.Bases), NetMoves: pack.NetMoves,
		Faces: []Face{
			{ID: "crimpTop", Label: "Top crimp", Net: [4]float64{0, -0.127, 0.678, 0.127}},
			{ID: "front", Label: "Front", Net: [4]float64{0, 0, 0.7734, 1.082}, Shown: &[4]float64{0, 0, 0.8766066838046273, 1}},
			{ID: "edges", Label: "Edges", Net: [4]float64{0.7734, 0, 0.11, 1.082}},
			{ID: "back", Label: "Back", Net: [4]float64{0.8834, 0, 0.7277, 1.082}, Shown: &[4]float64{0, 0, 0.9316939890710383, 1}},
			{ID: "seam", Label: "Back seam", Net: [4]float64{1.7111, 0, 0.125, 1.082}, Hidden: true},
			{ID: "crimpBottom", Label: "Bottom crimp", Net: [4]float64{0, 1.082, 0.678, 0.127}},
		}},
		[]Row{
			{Face: "crimpTop", Rect: [4]float64{583, 32, 985, 110}},
			{Face: "crimpTop", Rect: [4]float64{371, 32, 579, 106}, Show: [4]float64{0.4543, 0, 1, 1}},
			{Face: "crimpTop", Rect: [4]float64{198, 32, 371, 106}, Show: [4]float64{0, 0, 0.4543, 1}},
			{Face: "crimpTop", Rect: [4]float64{91, 710, 189, 782}, Show: [4]float64{0.443, 0, 0.6226, 1}, Flip: "xy"},
			{Face: "front", Rect: [4]float64{579, 110, 984, 723}, Show: [4]float64{0, 0, 0.8766, 1}},
			{Face: "edges", Rect: [4]float64{529, 106, 592, 723}},
			{Face: "edges", Rect: [4]float64{200, 106, 254, 722}},
			{Face: "back", Rect: [4]float64{354, 108, 538, 724}, Show: [4]float64{0.4668, 0, 0.9317, 1}},
			{Face: "back", Rect: [4]float64{246, 106, 406, 723}, Show: [4]float64{0, 0, 0.4035, 1}},
			{Face: "seam", Rect: [4]float64{91, 104, 189, 709}, Flip: "xy"},
			{Face: "crimpBottom", Rect: [4]float64{582, 720, 985, 800}},
			{Face: "crimpBottom", Rect: [4]float64{371, 726, 579, 800}, Show: [4]float64{0.4543, 0, 1, 1}},
			{Face: "crimpBottom", Rect: [4]float64{198, 726, 371, 800}, Show: [4]float64{0, 0, 0.4543, 1}},
			{Face: "crimpBottom", Rect: [4]float64{91, 35, 189, 106}, Show: [4]float64{0.443, 0, 0.6226, 1}, Flip: "xy"},
		})
}

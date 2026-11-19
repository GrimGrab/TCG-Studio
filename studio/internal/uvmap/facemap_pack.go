package uvmap

// Card pack: the game's CardPackStaticMesh_low with its back fixed (gameextract packModels: one continuous back strip; the
// game's own model reads part of it twice). Custom packs use this model in game; vanilla packs keep the game's. How to read and
// edit this: facemap.go.
func faceMapPack() Model {
	return faceMap(Model{Kind: "Pack", Mesh: "CardPackStaticMesh_low.fixed", TextureSize: 1024, Icon: "pack",
		Size:    [3]float64{0.678, 1.354, 0.148},
		Preview: []PreviewPart{{Mesh: "CardPackStaticMesh_low.fixed", Texture: "main"}},
		Bases:   sliceBases(pack.Bases), NetMoves: pack.NetMoves,
		Faces: []Face{
			{ID: "crimpTop", Label: "Top crimp", Net: [4]float64{0, -0.127, 0.678, 0.127}},
			{ID: "front", Label: "Front", Net: [4]float64{0, 0, 0.7734, 1.082}, Shown: &[4]float64{0, 0, 0.8766066838046273, 1}},
			{ID: "edges", Label: "Edges", Net: [4]float64{0.7734, 0, 0.11, 1.082}},
			{ID: "back", Label: "Back", Net: [4]float64{0.8834, 0, 0.678, 1.082}},
			{ID: "seam", Label: "Back seam", Net: [4]float64{1.7111, 0, 0.125, 1.082}, Hidden: true},
			{ID: "crimpBottom", Label: "Bottom crimp", Net: [4]float64{0, 1.082, 0.678, 0.127}},
		}},
		[]Row{
			{Face: "crimpTop", Rect: [4]float64{583, 32, 985, 110}},
			{Face: "crimpTop", Rect: [4]float64{198, 32, 579, 106}},
			{Face: "crimpTop", Rect: [4]float64{91, 710, 189, 782}, Show: [4]float64{0.443, 0, 0.6226, 1}, Flip: "xy"},
			{Face: "front", Rect: [4]float64{579, 110, 984, 723}, Show: [4]float64{0, 0, 0.8766, 1}},
			{Face: "edges", Rect: [4]float64{536, 106, 578, 723}},
			{Face: "edges", Rect: [4]float64{202, 106, 246, 722}},
			// One piece: the back's two halves read one strip of the texture (the seam fin over the middle has its own).
			{Face: "back", Rect: [4]float64{246, 106, 538, 724}},
			{Face: "seam", Rect: [4]float64{91, 104, 189, 709}, Flip: "xy"},
			{Face: "crimpBottom", Rect: [4]float64{582, 720, 985, 800}},
			{Face: "crimpBottom", Rect: [4]float64{198, 726, 579, 800}},
			{Face: "crimpBottom", Rect: [4]float64{91, 35, 189, 106}, Show: [4]float64{0.443, 0, 0.6226, 1}, Flip: "xy"},
		})
}

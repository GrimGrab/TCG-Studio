package uvmap

// Card box (CardBoxMesh_low). How to read and edit this: facemap.go.
func faceMapBox() Model {
	return faceMap(Model{Kind: "Box", Mesh: "CardBoxMesh_low", TextureSize: 2048, Icon: "box",
		Size:    [3]float64{1.547, 1.123, 0.929},
		Preview: []PreviewPart{{Mesh: "CardBoxMesh_low", Texture: "main"}},
		Bases:   sliceBases(box.Bases),
		// Designs from when front & back, the two sides, and top & bottom were one face each show on both again.
		NetCopies: []NetCopy{
			{Version: 3, From: "front", To: "back", Area: [4]float64{0, 0, 1.547, 1.123}, DX: 2.476},
			{Version: 3, From: "right", To: "left", Area: [4]float64{1.547, 0, 0.929, 1.123}, DX: 2.476},
			{Version: 3, From: "top", To: "bottom", Area: [4]float64{0, -0.929, 1.547, 0.929}, DY: 2.052},
		},
		Faces: []Face{
			{ID: "top", Label: "Top", Net: [4]float64{0, -0.929, 1.547, 0.929}},
			{ID: "front", Label: "Front", Net: [4]float64{0, 0, 1.547, 1.123}},
			{ID: "right", Label: "Right", Net: [4]float64{1.547, 0, 0.929, 1.123}},
			{ID: "back", Label: "Back", Net: [4]float64{2.476, 0, 1.547, 1.123}, ArtFrom: "front"},
			{ID: "left", Label: "Left", Net: [4]float64{4.023, 0, 0.929, 1.123}, ArtFrom: "right"},
			{ID: "bottom", Label: "Bottom", Net: [4]float64{0, 1.123, 1.547, 0.929}, ArtFrom: "top"},
		}},
		[]Row{
			{Face: "top", Rect: [4]float64{645, 570, 1401, 1024}},
			{Face: "left", Rect: [4]float64{94, 569, 645, 1025}, Flip: "x", Turn: true},
			{Face: "right", Rect: [4]float64{1401, 570, 1950, 1024}, Flip: "y", Turn: true},
			{Face: "bottom", Rect: [4]float64{645, 1573, 1401, 2027}},
			{Face: "front", Rect: [4]float64{645, 1024, 1401, 1573}},
			{Face: "back", Rect: [4]float64{645, 21, 1401, 570}, Flip: "xy"},
		})
}

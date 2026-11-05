package uvmap

// Playmat (the table mat's mesh; the rolled one in the shop reads the same printed area). How to read and edit this: facemap.go.
func faceMapPlaymat() Model {
	return faceMap(Model{Kind: "Playmat", Mesh: "PlayMatFlatMesh", TextureSize: 1024, Icon: "tilt",
		Size:    [3]float64{4.614, 0.05, 2.825},
		Preview: []PreviewPart{{Mesh: "PlayMatFlatMesh", Texture: "main"}},
		Faces: []Face{
			{ID: "surface", Label: "Play surface", Net: [4]float64{0, 0, 4.614, 2.825}},
			{ID: "bottom", Label: "Underside", Net: [4]float64{0, 2.925, 4.614, 2.825}},
			{ID: "edges", Label: "Rim", Net: [4]float64{0, 5.85, 0.6, 0.6}, Hidden: true},
		}},
		[]Row{
			{Face: "surface", Rect: [4]float64{24, 27, 997, 569}},
			{Face: "bottom", Rect: [4]float64{18, 748, 375, 1007}, Flip: "y"},
			{Face: "edges", Rect: [4]float64{424, 771, 564, 913}},
		})
}

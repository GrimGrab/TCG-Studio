package uvmap

// Sleeve pack (ItemCardSleeveMesh_low). How to read and edit this: facemap.go.
func faceMapSleeve() Model {
	return faceMap(Model{Kind: "Sleeve", Mesh: "ItemCardSleeveMesh_low", TextureSize: 1024, Icon: "tilt",
		Size:    [3]float64{0.693, 1.088, 0.206},
		Preview: []PreviewPart{{Mesh: "ItemCardSleeveMesh_low", Texture: "main"}},
		Faces: []Face{
			{ID: "top", Label: "Hang tab", Net: [4]float64{0, -0.387, 0.7, 0.331}},
			{ID: "front", Label: "Front", Net: [4]float64{0, 0, 0.693, 1.088}},
			{ID: "back", Label: "Back", Net: [4]float64{0.793, 0, 0.693, 1.088}},
			{ID: "edges", Label: "Edges", Net: [4]float64{1.586, 0, 0.206, 1.088}, Hidden: true},
		}},
		[]Row{
			{Face: "top", Rect: [4]float64{10, 565, 728, 877}},
			{Face: "front", Rect: [4]float64{410, 19, 719, 519}},
			{Face: "back", Rect: [4]float64{46, 19, 354, 519}},
			{Face: "edges", Rect: [4]float64{769, 397, 996, 1024}},
			{Face: "edges", Rect: [4]float64{769, 0, 996, 397}, Flip: "x", Turn: true},
		})
}

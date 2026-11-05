package uvmap

// Battle deck (PerconDeck_Mesh_low). How to read and edit this: facemap.go.
func faceMapBattleDeck() Model {
	return faceMap(Model{Kind: "BattleDeck", Mesh: "PerconDeck_Mesh_low", TextureSize: 1024, Icon: "box",
		Size:    [3]float64{1.055, 1.454, 0.471},
		Preview: []PreviewPart{{Mesh: "PerconDeck_Mesh_low", Texture: "main"}},
		Faces: []Face{
			{ID: "tab", Label: "Hang tab", Net: [4]float64{0.471, -0.738, 1.055, 0.267}},
			{ID: "top", Label: "Lid", Net: [4]float64{0.471, -0.471, 1.055, 0.471}},
			{ID: "left", Label: "Left side", Net: [4]float64{0, 0, 0.471, 1.454}},
			{ID: "front", Label: "Front", Net: [4]float64{0.471, 0, 1.055, 1.454}},
			{ID: "right", Label: "Right side", Net: [4]float64{1.526, 0, 0.471, 1.454}},
			{ID: "back", Label: "Back", Net: [4]float64{1.997, -0.267, 1.055, 1.721}},
		}},
		[]Row{
			{Face: "tab", Rect: [4]float64{161, 146, 507, 234}},
			{Face: "top", Rect: [4]float64{161, 234, 507, 386}},
			{Face: "left", Rect: [4]float64{5, 385, 162, 863}},
			{Face: "front", Rect: [4]float64{162, 386, 506, 863}},
			{Face: "right", Rect: [4]float64{506, 385, 663, 863}},
			{Face: "back", Rect: [4]float64{672, 299, 1019, 863}},
		})
}

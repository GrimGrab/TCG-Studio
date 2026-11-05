package uvmap

// Dice box (DiceBox_low; the glass is its own mesh and texture). How to read and edit this: facemap.go.
func faceMapDice() Model {
	return faceMap(Model{Kind: "Dice", Mesh: "DiceBox_low", TextureSize: 1024, Icon: "mesh",
		Size:    [3]float64{0.722, 1.002, 0.799},
		Preview: []PreviewPart{{Mesh: "DiceBox_low", Texture: "main"}, {Mesh: "DiceBoxGlass_low", Glass: true}},
		Faces: []Face{
			{ID: "top", Label: "Lid", Net: [4]float64{0, 0, 0.722, 0.799}},
			{ID: "box", Label: "Sides (lid and box)", Net: [4]float64{0, 0.899, 0.799, 0.17}},
			{ID: "bottom", Label: "Bottom", Net: [4]float64{0, 1.169, 0.722, 0.799}},
			{ID: "die", Label: "Die (its faces and numbers)", Net: [4]float64{0.899, 0, 0.076, 0.882}, Hidden: true},
		}},
		[]Row{
			{Face: "top", Rect: [4]float64{427, 103, 766, 478}, Flip: "xy"},
			{Face: "box", Rect: [4]float64{426, 22, 767, 103}, Flip: "xy"},
			{Face: "box", Rect: [4]float64{346, 102, 427, 480}, Flip: "x", Turn: true},
			{Face: "box", Rect: [4]float64{427, 478, 766, 560}},
			{Face: "box", Rect: [4]float64{766, 102, 848, 480}, Flip: "y", Turn: true},
			{Face: "box", Rect: [4]float64{82, 940, 423, 1023}},
			{Face: "box", Rect: [4]float64{421, 565, 502, 942}, Flip: "y", Turn: true},
			{Face: "box", Rect: [4]float64{82, 485, 423, 567}, Flip: "xy"},
			{Face: "box", Rect: [4]float64{1, 565, 84, 942}, Flip: "x", Turn: true},
			{Face: "bottom", Rect: [4]float64{502, 565, 842, 942}, Flip: "xy"},
			{Face: "die", Rect: [4]float64{945, 139, 1023, 1023}},
		})
}

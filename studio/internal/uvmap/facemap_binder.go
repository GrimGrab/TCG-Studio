package uvmap

// Collection book (ItemBinderBookOptimized_low). How to read and edit this: facemap.go.
func faceMapBinder() Model {
	return faceMap(Model{Kind: "Binder", Mesh: "ItemBinderBookOptimized_low", TextureSize: 1024, Icon: "tilt",
		Size:    [3]float64{3.087, 2.889, 0.618},
		Preview: []PreviewPart{{Mesh: "ItemBinderBookOptimized_low", Texture: "main"}},
		Faces: []Face{
			{ID: "back", Label: "Back cover", Net: [4]float64{0, 0, 3.087, 2.889}},
			{ID: "left", Label: "Spine", Net: [4]float64{3.087, 0, 0.618, 2.889}},
			{ID: "front", Label: "Front cover", Net: [4]float64{3.705, 0, 3.087, 2.889}},
			{ID: "inside", Label: "Inside covers", Net: [4]float64{0, 3.089, 6.792, 2.889}, Hidden: true},
		}},
		[]Row{
			{Face: "back", Rect: [4]float64{14, 619, 428, 1009}},
			{Face: "left", Rect: [4]float64{428, 619, 512, 1009}},
			{Face: "front", Rect: [4]float64{512, 619, 926, 1009}},
			{Face: "inside", Rect: [4]float64{19, 214, 433, 604}, Show: [4]float64{0, 0, 0.5, 1}},
			{Face: "inside", Rect: [4]float64{511, 214, 925, 604}, Show: [4]float64{0.5, 0, 1, 1}},
			{Face: "Inside Spine", Part: "Piece 1", Rect: [4]float64{432, 215, 513, 603}},
		})
}

package uvmap

// Comic (Manga_Mesh). How to read and edit this: facemap.go.
func faceMapComic() Model {
	return faceMap(Model{Kind: "Comic", Mesh: "Manga_Mesh", TextureSize: 1024, Icon: "tilt",
		Size:    [3]float64{1.226, 1.699, 0.227},
		Preview: []PreviewPart{{Mesh: "Manga_Mesh", Texture: "main"}},
		Faces: []Face{
			{ID: "back", Label: "Back cover", Net: [4]float64{0, 0, 1.226, 1.699}},
			{ID: "left", Label: "Spine", Net: [4]float64{1.226, 0, 0.227, 1.699}},
			{ID: "front", Label: "Front cover", Net: [4]float64{1.453, 0, 1.226, 1.699}},
			{ID: "pages", Label: "Page edges", Net: [4]float64{0, 1.799, 1.699, 0.443}},
		}},
		[]Row{
			{Face: "back", Rect: [4]float64{14, 18, 450, 736}},
			{Face: "left", Rect: [4]float64{450, 18, 524, 736}},
			{Face: "front", Rect: [4]float64{524, 18, 1004, 736}},
			{Face: "pages", Rect: [4]float64{28, 751, 982, 1003}},
		})
}

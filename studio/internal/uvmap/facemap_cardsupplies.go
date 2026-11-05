package uvmap

// Card supply UP_PennySleeves. How to read and edit this: facemap.go.
func faceMapUPPennySleeves() Model {
	return faceMap(Model{Kind: "CardSupply", Mesh: "PennySleeves_Mesh", TextureSize: 1024, Icon: "mesh",
		Size:    [3]float64{0.726, 1.238, 0.208},
		Preview: []PreviewPart{{Mesh: "PennySleeves_Mesh", Texture: "main"}},
		Faces: []Face{
			{ID: "header", Label: "Bag top (both sides)", Net: [4]float64{0, -0.321, 0.726, 0.221}},
			{ID: "front", Label: "Front", Net: [4]float64{0, 0, 0.693, 0.992}},
			{ID: "back", Label: "Back", Net: [4]float64{0.793, 0, 0.693, 0.992}},
			{ID: "edges", Label: "Stack edges", Net: [4]float64{1.586, 0, 0.206, 0.988}, Hidden: true},
		}},
		[]Row{
			{Face: "header", Rect: [4]float64{10, 573, 733, 873}},
			{Face: "front", Rect: [4]float64{393, 21, 749, 518}},
			{Face: "back", Rect: [4]float64{12, 21, 368, 518}},
			{Face: "edges", Rect: [4]float64{774, 11, 989, 1013}},
		})
}

// Card supply UP_TowerDeckbox. How to read and edit this: facemap.go.
func faceMapUPTowerDeckbox() Model {
	return faceMap(Model{Kind: "CardSupply", Mesh: "TowerDeckbox_Mesh", TextureSize: 1024, Icon: "mesh",
		Size:    [3]float64{0.82, 1.352, 0.82},
		Preview: []PreviewPart{{Mesh: "TowerDeckbox_Mesh", Texture: "main"}},
		Faces: []Face{
			{ID: "left", Label: "Left side", Net: [4]float64{0, 0, 0.821, 1.343}},
			{ID: "front", Label: "Front", Net: [4]float64{0.821, 0, 0.821, 1.346}},
			{ID: "right", Label: "Right side", Net: [4]float64{1.642, 0, 0.821, 1.346}},
			{ID: "back", Label: "Back", Net: [4]float64{2.463, 0, 0.821, 1.346}},
			{ID: "top", Label: "Lid top & bottom", Net: [4]float64{0.821, -0.821, 0.821, 0.821}},
		}},
		[]Row{
			{Face: "left", Rect: [4]float64{6, 51, 371, 559}},
			{Face: "front", Rect: [4]float64{19, 573, 722, 1012}, Flip: "y", Turn: true},
			{Face: "right", Rect: [4]float64{385, 45, 730, 561}},
			{Face: "back", Rect: [4]float64{742, 36, 1013, 468}},
			{Face: "top", Rect: [4]float64{754, 488, 999, 733}, Flip: "xy"},
		})
}

// Card supply UP_MagneticHolder. How to read and edit this: facemap.go.
func faceMapUPMagneticHolder() Model {
	return faceMap(Model{Kind: "CardSupply", Mesh: "MagneticHolder_Mesh", TextureSize: 1024, Icon: "mesh",
		Size:    [3]float64{0.727, 1.141, 0.093},
		Preview: []PreviewPart{{Mesh: "MagneticHolder_Mesh", Texture: "main"}},
		Faces: []Face{
			{ID: "front", Label: "Front", Net: [4]float64{0, 0, 0.727, 1.141}},
			{ID: "back", Label: "Back", Net: [4]float64{0.827, 0, 0.727, 1.141}},
			{ID: "edges", Label: "Rim (all edges)", Net: [4]float64{1.654, 0, 0.6, 0.13}, Hidden: true},
		}},
		[]Row{
			{Face: "front", Rect: [4]float64{528, 27, 992, 745}},
			{Face: "back", Rect: [4]float64{25, 32, 496, 741}},
			{Face: "edges", Rect: [4]float64{69, 808, 936, 994}},
		})
}

// Card supply UP_Toploader. How to read and edit this: facemap.go.
func faceMapUPToploader() Model {
	return faceMap(Model{Kind: "CardSupply", Mesh: "Toploader_Mesh", TextureSize: 1024, Icon: "mesh",
		Size:    [3]float64{0.716, 1.066, 0.382},
		Preview: []PreviewPart{{Mesh: "Toploader_Mesh", Texture: "main"}},
		Faces: []Face{
			{ID: "clearFront", Label: "Toploader front", Net: [4]float64{0.399, 0, 0.682, 1.057}},
			{ID: "clearBack", Label: "Toploader back", Net: [4]float64{1.479, 0, 0.682, 1.057}},
			{ID: "left", Label: "Card left", Net: [4]float64{0, 1.157, 0.382, 0.618}},
			{ID: "front", Label: "Card front", Net: [4]float64{0.382, 1.349, 0.715, 0.426}},
			{ID: "right", Label: "Card right", Net: [4]float64{1.097, 1.157, 0.382, 0.618}},
			{ID: "back", Label: "Card back", Net: [4]float64{1.479, 1.157, 0.715, 0.618}},
			{ID: "edges", Label: "Stack edges", Net: [4]float64{2.294, 0, 0.34, 1.048}, Hidden: true},
		}},
		[]Row{
			{Face: "clearFront", Rect: [4]float64{403, 29, 733, 509}},
			{Face: "clearBack", Rect: [4]float64{28, 30, 362, 507}},
			{Face: "left", Rect: [4]float64{547, 721, 763, 1024}},
			{Face: "front", Rect: [4]float64{14, 515, 460, 740}},
			{Face: "right", Rect: [4]float64{335, 721, 545, 1023}},
			{Face: "back", Rect: [4]float64{9, 768, 324, 1011}},
			{Face: "edges", Rect: [4]float64{774, 11, 989, 1013}},
		})
}

// Card supply UP_CardPreserver. How to read and edit this: facemap.go.
func faceMapUPCardPreserver() Model {
	return faceMap(Model{Kind: "CardSupply", Mesh: "CardPreserverBox_Mesh", TextureSize: 2048, Icon: "mesh",
		Size:    [3]float64{0.847, 1.382, 0.244},
		Preview: []PreviewPart{{Mesh: "CardPreserverBox_Mesh", Texture: "main"}},
		Faces: []Face{
			{ID: "left", Label: "Left side", Net: [4]float64{0, 0, 0.241, 1.167}},
			{ID: "front", Label: "Front", Net: [4]float64{0.241, 0, 0.847, 1.167}},
			{ID: "right", Label: "Right side", Net: [4]float64{1.088, 0, 0.241, 1.167}},
			{ID: "top", Label: "Top", Net: [4]float64{0.241, -0.23, 0.847, 0.23}},
			{ID: "tab", Label: "Hang tab front", Net: [4]float64{0.241, -0.545, 0.847, 0.215}},
			{ID: "back", Label: "Back (with the tab)", Net: [4]float64{1.429, -0.215, 0.844, 1.382}},
			{ID: "bottom", Label: "Bottom", Net: [4]float64{0.241, 1.267, 0.847, 0.234}, Hidden: true},
		}},
		[]Row{
			{Face: "left", Rect: [4]float64{145, 27, 482, 1493}},
			{Face: "front", Rect: [4]float64{485, 29, 1567, 1494}},
			{Face: "right", Rect: [4]float64{1569, 27, 1906, 1493}},
			{Face: "top", Rect: [4]float64{1263, 1512, 2025, 1810}},
			{Face: "tab", Rect: [4]float64{1260, 1841, 2048, 2022}},
			{Face: "back", Rect: [4]float64{49, 1528, 786, 1997}, Turn: true},
			{Face: "bottom", Rect: [4]float64{804, 1535, 1247, 1742}},
		})
}

// Card supply UP_Playmat1. How to read and edit this: facemap.go.
func faceMapUPPlaymat1() Model {
	return faceMap(Model{Kind: "CardSupply", Mesh: "UltraProPlaymatOptimized_low", TextureSize: 1024, Icon: "mesh",
		Size:    [3]float64{0.508, 3.003, 0.415},
		Preview: []PreviewPart{{Mesh: "UltraProPlaymatOptimized_low", Texture: "main"}},
		Faces: []Face{
			{ID: "left", Label: "Left side", Net: [4]float64{0, 0, 0.41, 2.674}},
			{ID: "front", Label: "Front", Net: [4]float64{0.41, 0, 0.507, 2.674}},
			{ID: "right", Label: "Right side", Net: [4]float64{0.917, 0, 0.41, 2.674}},
			{ID: "back", Label: "Back (with the tab)", Net: [4]float64{1.327, -0.329, 0.505, 3.003}},
			{ID: "tab", Label: "Hang tab front", Net: [4]float64{0.41, -0.722, 0.504, 0.329}},
			{ID: "top", Label: "Top end", Net: [4]float64{0.41, -0.393, 0.496, 0.393}},
			{ID: "mat", Label: "Mat inside (its colour)", Net: [4]float64{1.932, 0, 0.3, 0.6}},
		}},
		[]Row{
			{Face: "left", Rect: [4]float64{5, 122, 126, 899}},
			{Face: "front", Rect: [4]float64{126, 123, 274, 899}},
			{Face: "right", Rect: [4]float64{274, 122, 395, 899}},
			{Face: "back", Rect: [4]float64{405, 241, 555, 1019}, Show: [4]float64{0, 0.1095, 1, 1}},
			{Face: "back", Rect: [4]float64{405, 38, 555, 136}, Show: [4]float64{0, 0, 1, 0.1095}, Flip: "xy"},
			{Face: "tab", Rect: [4]float64{405, 137, 555, 234}},
			{Face: "top", Rect: [4]float64{125, 5, 275, 123}},
			{Face: "mat", Rect: [4]float64{579, 404, 680, 887}},
		})
}

// Card supply UP_PlatinumSeriesPocketPages. How to read and edit this: facemap.go.
func faceMapUPPlatinumSeriesPocketPages() Model {
	return faceMap(Model{Kind: "CardSupply", Mesh: "PocketPages_Mesh", TextureSize: 2048, Icon: "mesh",
		Size:    [3]float64{2.196, 2.948, 0.404},
		Preview: []PreviewPart{{Mesh: "PocketPages_Mesh", Texture: "main"}},
		Faces: []Face{
			{ID: "left", Label: "Left side", Net: [4]float64{0, 0, 0.393, 2.923}},
			{ID: "front", Label: "Front", Net: [4]float64{0.393, 0, 2.18, 2.923}},
			{ID: "right", Label: "Right side", Net: [4]float64{2.573, 0, 0.393, 2.923}},
			{ID: "back", Label: "Back label", Net: [4]float64{3.066, 1.476, 1.094, 1.447}},
		}},
		[]Row{
			{Face: "left", Rect: [4]float64{1567, 100, 1741, 1710}},
			{Face: "front", Rect: [4]float64{10, 58, 1321, 1916}},
			{Face: "right", Rect: [4]float64{1338, 73, 1507, 1724}},
			{Face: "back", Rect: [4]float64{1759, 1390, 2043, 2040}},
		})
}

// Card supply UP_SemiRigidCardHolder. How to read and edit this: facemap.go.
func faceMapUPSemiRigidCardHolder() Model {
	return faceMap(Model{Kind: "CardSupply", Mesh: "SemiRigidCardHolder_Mesh", TextureSize: 1024, Icon: "mesh",
		Size:    [3]float64{1.562, 1.153, 0.92},
		Preview: []PreviewPart{{Mesh: "SemiRigidCardHolder_Mesh", Texture: "main"}},
		Faces: []Face{
			{ID: "left", Label: "Sides (both)", Net: [4]float64{0, 0, 0.894, 1.143}},
			{ID: "front", Label: "Front", Net: [4]float64{0.894, 0, 1.551, 1.125}},
			{ID: "back", Label: "Back", Net: [4]float64{2.545, 0, 1.519, 1.116}},
			{ID: "top", Label: "Top & bottom", Net: [4]float64{0.894, -0.889, 1.551, 0.889}},
		}},
		[]Row{
			{Face: "left", Rect: [4]float64{691, 554, 1010, 1002}},
			{Face: "front", Rect: [4]float64{11, 23, 738, 527}},
			{Face: "back", Rect: [4]float64{10, 554, 686, 1002}},
			{Face: "top", Rect: [4]float64{770, 24, 1004, 528}, Flip: "x", Turn: true},
		})
}

// Card supply UP_Album. How to read and edit this: facemap.go.
func faceMapUPAlbum() Model {
	return faceMap(Model{Kind: "CardSupply", Mesh: "ItemBinderBookOptimized_low", TextureSize: 1024, Icon: "mesh",
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
		})
}

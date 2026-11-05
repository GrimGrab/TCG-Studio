package uvmap

// Board game Boardgame_Speedrobo_Claim. How to read and edit this: facemap.go.
func faceMapBoardgameSpeedroboClaim() Model {
	return faceMap(Model{Kind: "BoardGame", Mesh: "BoardGameMesh_Claim", TextureSize: 1024, Icon: "mesh",
		Size:    [3]float64{1.526, 2.091, 0.968},
		Preview: []PreviewPart{{Mesh: "BoardGameMesh_Claim", Texture: "main"}},
		Faces: []Face{
			{ID: "left", Label: "Left side", Net: [4]float64{0, 0, 0.94, 2.075}},
			{ID: "front", Label: "Front", Net: [4]float64{0.94, 0, 1.515, 2.075}},
			{ID: "right", Label: "Right side", Net: [4]float64{2.455, 0, 0.94, 2.075}},
			{ID: "back", Label: "Back", Net: [4]float64{3.395, 0, 1.515, 2.075}},
			{ID: "top", Label: "Top & bottom", Net: [4]float64{0.94, -0.93, 1.515, 0.93}},
		}},
		[]Row{
			{Face: "left", Rect: [4]float64{471, 158, 624, 592}},
			{Face: "front", Rect: [4]float64{152, 160, 473, 592}},
			{Face: "right", Rect: [4]float64{0, 158, 152, 592}, Flip: "x"},
			{Face: "back", Rect: [4]float64{155, 592, 471, 1022}, Flip: "xy"},
			{Face: "top", Rect: [4]float64{154, 0, 472, 160}},
		})
}

// Board game Boardgame_Speedrobo_Mafia. How to read and edit this: facemap.go.
func faceMapBoardgameSpeedroboMafia() Model {
	return faceMap(Model{Kind: "BoardGame", Mesh: "BoardGameMesh_MafiaWorks", TextureSize: 1024, Icon: "mesh",
		Size:    [3]float64{3.042, 1.973, 1.072},
		Preview: []PreviewPart{{Mesh: "BoardGameMesh_MafiaWorks", Texture: "main"}},
		Faces: []Face{
			{ID: "front", Label: "Front", Net: [4]float64{0, 0, 3.021, 1.935}},
			{ID: "top", Label: "Top (also bottom & ends)", Net: [4]float64{0, -1.04, 3.021, 1.04}},
			{ID: "back", Label: "Back", Net: [4]float64{3.121, 0, 3, 1.935}},
		}},
		[]Row{
			{Face: "front", Rect: [4]float64{479, 631, 1017, 1007}},
			{Face: "top", Rect: [4]float64{633, 1, 1023, 168}},
			{Face: "back", Rect: [4]float64{637, 177, 1018, 446}},
		})
}

// Board game Boardgame_Speedrobo_Necro. How to read and edit this: facemap.go.
func faceMapBoardgameSpeedroboNecro() Model {
	return faceMap(Model{Kind: "BoardGame", Mesh: "BoardGameMesh_Necromansters", TextureSize: 1024, Icon: "mesh",
		Size:    [3]float64{1.298, 2.571, 0.822},
		Preview: []PreviewPart{{Mesh: "BoardGameMesh_Necromansters", Texture: "main"}},
		Faces: []Face{
			{ID: "left", Label: "Sides (both)", Net: [4]float64{0, 0, 0.8, 2.55}},
			{ID: "front", Label: "Front", Net: [4]float64{0.8, 0, 1.29, 2.55}},
			{ID: "top", Label: "Top & bottom", Net: [4]float64{0.8, -0.79, 1.29, 0.79}},
			{ID: "back", Label: "Back", Net: [4]float64{2.19, 0, 1.29, 2.55}},
		}},
		[]Row{
			{Face: "left", Rect: [4]float64{491, 27, 694, 846}},
			{Face: "front", Rect: [4]float64{17, 28, 477, 947}},
			{Face: "top", Rect: [4]float64{495, 843, 839, 1013}},
			{Face: "back", Rect: [4]float64{702, 27, 1013, 726}},
		})
}

// Board game Boardgame_Speedrobo_SystemGate1. How to read and edit this: facemap.go.
func faceMapBoardgameSpeedroboSystemGate1() Model {
	return faceMap(Model{Kind: "BoardGame", Mesh: "BoardGameMesh_SystemGate1", TextureSize: 1024, Icon: "mesh",
		Size:    [3]float64{3.042, 1.973, 0.836},
		Preview: []PreviewPart{{Mesh: "BoardGameMesh_SystemGate1", Texture: "main"}},
		Faces: []Face{
			{ID: "left", Label: "Left side", Net: [4]float64{0, 0, 0.813, 1.958}},
			{ID: "front", Label: "Front", Net: [4]float64{0.813, 0, 3.021, 1.958}},
			{ID: "right", Label: "Right side", Net: [4]float64{3.834, 0, 0.813, 1.958}},
			{ID: "back", Label: "Back (lower part = top & bottom)", Net: [4]float64{4.647, 0, 3, 1.958}},
		}},
		[]Row{
			{Face: "left", Rect: [4]float64{16, 347, 123, 667}, Flip: "x"},
			{Face: "front", Rect: [4]float64{145, 347, 652, 668}},
			{Face: "right", Rect: [4]float64{16, 17, 123, 338}},
			{Face: "back", Rect: [4]float64{144, 19, 650, 341}},
		})
}

// Board game Boardgame_Speedrobo_SystemGate2. How to read and edit this: facemap.go.
func faceMapBoardgameSpeedroboSystemGate2() Model {
	return faceMap(Model{Kind: "BoardGame", Mesh: "BoardGameMesh_SystemGate2", TextureSize: 1024, Icon: "mesh",
		Size:    [3]float64{3.042, 1.973, 0.828},
		Preview: []PreviewPart{{Mesh: "BoardGameMesh_SystemGate2", Texture: "main"}},
		Faces: []Face{
			{ID: "left", Label: "Left side", Net: [4]float64{0, 0, 0.805, 1.958}},
			{ID: "front", Label: "Front", Net: [4]float64{0.805, 0, 3.021, 1.958}},
			{ID: "right", Label: "Right side", Net: [4]float64{3.826, 0, 0.805, 1.958}},
			{ID: "back", Label: "Back (lower part = top & bottom)", Net: [4]float64{4.631, 0, 3, 1.958}},
		}},
		[]Row{
			{Face: "left", Rect: [4]float64{786, 360, 1003, 693}},
			{Face: "front", Rect: [4]float64{17, 677, 534, 1007}},
			{Face: "right", Rect: [4]float64{787, 16, 1004, 347}},
			{Face: "back", Rect: [4]float64{548, 705, 1019, 1007}},
		})
}

package uvmap

// Game look box restockBig. How to read and edit this: facemap.go.
func faceMapLookRestockBig() Model {
	return faceMap(Model{Kind: "Box look", Mesh: "CardboardBox_20x20x20_StaticClosed", TextureSize: 2048, Icon: "mesh",
		Size:    [3]float64{1.97, 2.01, 1.97},
		Preview: []PreviewPart{},
		Faces: []Face{
			{ID: "left", Label: "Left side", Net: [4]float64{0, 0, 1.97, 2.01}},
			{ID: "front", Label: "Front", Net: [4]float64{1.97, 0, 1.97, 2.01}},
			{ID: "right", Label: "Right side", Net: [4]float64{3.94, 0, 1.97, 2.01}},
			{ID: "back", Label: "Back", Net: [4]float64{5.91, 0, 1.97, 2.01}},
			{ID: "top", Label: "Top (outer flaps)", Net: [4]float64{1.97, -1.97, 1.97, 1.97}},
			{ID: "bottom", Label: "Bottom", Net: [4]float64{1.97, 2.01, 1.97, 1.97}, Hidden: true},
			{ID: "under", Label: "Flaps under the top", Net: [4]float64{5.91, -1.97, 1.97, 1.97}, Hidden: true},
			{ID: "inside", Label: "Inside walls (all four)", Net: [4]float64{0, 4.279999999999999, 1.97, 2.01}, Hidden: true},
			{ID: "floor", Label: "Inside floor", Net: [4]float64{2.27, 4.279999999999999, 1.97, 1.97}, Hidden: true},
			{ID: "flapsIn", Label: "Flap undersides (all four)", Net: [4]float64{4.54, 4.279999999999999, 1.97, 0.985}, Hidden: true},
		}},
		[]Row{
			{Face: "left", Rect: [4]float64{1027, 1506, 1535, 2041}},
			{Face: "front", Rect: [4]float64{1534, 1506, 2041, 2041}},
			{Face: "right", Rect: [4]float64{23, 1505, 531, 2041}},
			{Face: "back", Rect: [4]float64{530, 1506, 1039, 2041}},
			{Face: "top", Rect: [4]float64{1038, 1245, 1534, 1507}, Show: [4]float64{0, 0, 0.5, 1}, Flip: "y", Turn: true},
			{Face: "top", Rect: [4]float64{24, 1245, 522, 1505}, Show: [4]float64{0.5, 0, 1, 1}, Flip: "x", Turn: true},
			{Face: "bottom", Rect: [4]float64{31, 7, 537, 520}, Turn: true},
			{Face: "under", Rect: [4]float64{1544, 1245, 2041, 1511}, Show: [4]float64{0, 0.5, 1, 1}},
			{Face: "under", Rect: [4]float64{531, 1245, 1027, 1511}, Show: [4]float64{0, 0, 1, 0.5}, Flip: "xy"},
			{Face: "inside", Rect: [4]float64{1266, 769, 1680, 1233}, Flip: "x"},
			{Face: "inside", Rect: [4]float64{441, 769, 855, 1233}, Flip: "x"},
			{Face: "inside", Rect: [4]float64{854, 763, 1267, 1233}, Flip: "x"},
			{Face: "inside", Rect: [4]float64{27, 763, 442, 1233}, Flip: "x"},
			{Face: "floor", Rect: [4]float64{558, 15, 1062, 518}, Flip: "xy"},
			{Face: "flapsIn", Rect: [4]float64{1266, 538, 1680, 764}, Flip: "y"},
			{Face: "flapsIn", Rect: [4]float64{441, 538, 854, 764}, Flip: "x"},
			{Face: "flapsIn", Rect: [4]float64{27, 538, 441, 761}, Turn: true},
			{Face: "flapsIn", Rect: [4]float64{854, 538, 1266, 761}, Flip: "xy", Turn: true},
		})
}

// Game look box restockSmall. How to read and edit this: facemap.go.
func faceMapLookRestockSmall() Model {
	return faceMap(Model{Kind: "Box look", Mesh: "CardboardBox_20x20x10_StaticClosed", TextureSize: 2048, Icon: "mesh",
		Size:    [3]float64{1.97, 1.01, 1.97},
		Preview: []PreviewPart{},
		Faces: []Face{
			{ID: "left", Label: "Left side", Net: [4]float64{0, 0, 1.97, 1.01}},
			{ID: "front", Label: "Front", Net: [4]float64{1.97, 0, 1.97, 1.01}},
			{ID: "right", Label: "Right side", Net: [4]float64{3.94, 0, 1.97, 1.01}},
			{ID: "back", Label: "Back", Net: [4]float64{5.91, 0, 1.97, 1.01}},
			{ID: "top", Label: "Top (outer flaps)", Net: [4]float64{1.97, -1.97, 1.97, 1.97}},
			{ID: "bottom", Label: "Bottom", Net: [4]float64{1.97, 1.01, 1.97, 1.97}, Hidden: true},
			{ID: "under", Label: "Flaps under the top", Net: [4]float64{5.91, -1.97, 1.97, 1.97}, Hidden: true},
			{ID: "inside", Label: "Inside walls (all four)", Net: [4]float64{0, 3.28, 1.97, 1.01}, Hidden: true},
			{ID: "floor", Label: "Inside floor", Net: [4]float64{2.27, 3.28, 1.97, 1.97}, Hidden: true},
			{ID: "flapsIn", Label: "Flap undersides (all four)", Net: [4]float64{4.54, 3.28, 1.97, 0.985}, Hidden: true},
		}},
		[]Row{
			{Face: "left", Rect: [4]float64{1156, 1732, 1728, 2033}},
			{Face: "front", Rect: [4]float64{1739, 303, 2035, 875}, Flip: "y", Turn: true},
			{Face: "right", Rect: [4]float64{26, 1736, 599, 2033}},
			{Face: "back", Rect: [4]float64{598, 1741, 1168, 2033}},
			{Face: "top", Rect: [4]float64{1169, 1448, 1728, 1736}, Show: [4]float64{0, 0, 0.5, 1}, Flip: "y", Turn: true},
			{Face: "top", Rect: [4]float64{26, 1448, 587, 1734}, Show: [4]float64{0.5, 0, 1, 1}, Flip: "x", Turn: true},
			{Face: "bottom", Rect: [4]float64{29, 16, 685, 661}, Turn: true},
			{Face: "under", Rect: [4]float64{1471, 10, 2032, 297}, Show: [4]float64{0, 0.5, 1, 1}},
			{Face: "under", Rect: [4]float64{598, 1448, 1156, 1742}, Show: [4]float64{0, 0, 1, 0.5}, Flip: "xy"},
			{Face: "inside", Rect: [4]float64{1745, 1465, 2036, 2033}, Turn: true},
			{Face: "inside", Rect: [4]float64{593, 1126, 1159, 1417}, Flip: "x"},
			{Face: "inside", Rect: [4]float64{1158, 1117, 1725, 1417}, Flip: "x"},
			{Face: "inside", Rect: [4]float64{28, 1116, 594, 1417}, Flip: "x"},
			{Face: "floor", Rect: [4]float64{698, 15, 1285, 602}, Flip: "xy"},
			{Face: "flapsIn", Rect: [4]float64{1750, 883, 2035, 1450}, Flip: "xy", Turn: true},
			{Face: "flapsIn", Rect: [4]float64{594, 834, 1159, 1120}, Flip: "x"},
			{Face: "flapsIn", Rect: [4]float64{28, 834, 594, 1115}, Turn: true},
			{Face: "flapsIn", Rect: [4]float64{1159, 834, 1723, 1115}, Flip: "xy", Turn: true},
		})
}

// Game look box furnitureBox. How to read and edit this: facemap.go.
func faceMapLookFurnitureBox() Model {
	return faceMap(Model{Kind: "Box look", Mesh: "CardboardBox_30x20x20_StaticClosed", TextureSize: 2048, Icon: "mesh",
		Size:    [3]float64{1.97, 2.03, 3},
		Preview: []PreviewPart{},
		Faces: []Face{
			{ID: "left", Label: "Left side", Net: [4]float64{0, 0, 3, 2.03}},
			{ID: "front", Label: "Front", Net: [4]float64{3, 0, 1.97, 2.03}},
			{ID: "right", Label: "Right side", Net: [4]float64{4.97, 0, 3, 2.03}},
			{ID: "back", Label: "Back", Net: [4]float64{7.97, 0, 1.97, 2.03}},
			{ID: "top", Label: "Top (outer flaps)", Net: [4]float64{3, -3, 1.97, 3}},
			{ID: "bottom", Label: "Bottom", Net: [4]float64{3, 2.03, 1.97, 3}, Hidden: true},
			{ID: "under", Label: "Flaps under the top", Net: [4]float64{7.97, -3, 1.97, 3}, Hidden: true},
			{ID: "inside", Label: "Inside walls (all four)", Net: [4]float64{0, 5.329999999999999, 1.97, 2.03}, Hidden: true},
			{ID: "floor", Label: "Inside floor", Net: [4]float64{2.27, 5.329999999999999, 1.97, 3}, Hidden: true},
			{ID: "flapsIn", Label: "Flap undersides (all four)", Net: [4]float64{4.54, 5.329999999999999, 1.97, 1.5}, Hidden: true},
		}},
		[]Row{
			{Face: "left", Rect: [4]float64{1025, 1599, 1664, 2019}},
			{Face: "front", Rect: [4]float64{1656, 1602, 2033, 2019}},
			{Face: "right", Rect: [4]float64{24, 1599, 657, 2019}},
			{Face: "back", Rect: [4]float64{656, 1605, 1026, 2019}},
			{Face: "top", Rect: [4]float64{1032, 1399, 1656, 1602}, Show: [4]float64{0, 0, 0.5, 1}, Flip: "y", Turn: true},
			{Face: "top", Rect: [4]float64{24, 1399, 650, 1602}, Show: [4]float64{0.5, 0, 1, 1}, Flip: "x", Turn: true},
			{Face: "bottom", Rect: [4]float64{36, 224, 883, 766}, Turn: true},
			{Face: "under", Rect: [4]float64{1664, 1399, 2031, 1606}, Show: [4]float64{0, 0.6667, 1, 1}},
			{Face: "under", Rect: [4]float64{657, 1399, 1026, 1603}, Show: [4]float64{0, 0, 1, 0.3333}, Flip: "xy"},
			{Face: "inside", Rect: [4]float64{1451, 988, 1776, 1375}, Flip: "x"},
			{Face: "inside", Rect: [4]float64{577, 988, 903, 1375}, Flip: "x"},
			{Face: "inside", Rect: [4]float64{902, 982, 1452, 1375}, Flip: "x"},
			{Face: "inside", Rect: [4]float64{28, 982, 578, 1375}, Flip: "x"},
			{Face: "floor", Rect: [4]float64{911, 221, 1769, 767}, Flip: "x", Turn: true},
			{Face: "flapsIn", Rect: [4]float64{28, 795, 578, 979}, Turn: true},
			{Face: "flapsIn", Rect: [4]float64{902, 795, 1451, 980}, Flip: "xy", Turn: true},
			{Face: "flapsIn", Rect: [4]float64{1451, 795, 1776, 986}, Flip: "y"},
			{Face: "flapsIn", Rect: [4]float64{577, 795, 902, 984}, Flip: "x"},
		})
}

// Game look box gradedBox. How to read and edit this: facemap.go.
func faceMapLookGradedBox() Model {
	return faceMap(Model{Kind: "Box look", Mesh: "CardboardBox_20x10x7_Animated_Model", TextureSize: 2048, Icon: "mesh",
		Size:    [3]float64{0.97, 1.18, 2},
		Preview: []PreviewPart{},
		Faces: []Face{
			{ID: "left", Label: "Left side", Net: [4]float64{0, 0, 2, 1.18}},
			{ID: "front", Label: "Front", Net: [4]float64{2, 0, 0.97, 1.18}},
			{ID: "right", Label: "Right side", Net: [4]float64{2.9699999999999998, 0, 2, 1.18}},
			{ID: "back", Label: "Back", Net: [4]float64{4.97, 0, 0.97, 1.18}},
			{ID: "floor", Label: "Inside floor", Net: [4]float64{2, -2, 0.97, 2}},
			{ID: "inside", Label: "Inside walls (all four)", Net: [4]float64{4.97, -2, 0.97, 1.18}},
			{ID: "bottom", Label: "Bottom", Net: [4]float64{2, 1.18, 0.97, 2}, Hidden: true},
		}},
		[]Row{
			{Face: "left", Rect: [4]float64{1198, 1542, 2036, 2034}},
			{Face: "front", Rect: [4]float64{1692, 9, 2039, 502}},
			{Face: "right", Rect: [4]float64{31, 1542, 869, 2034}},
			{Face: "back", Rect: [4]float64{868, 1542, 1199, 2034}},
			{Face: "floor", Rect: [4]float64{25, 517, 950, 999}, Flip: "y", Turn: true},
			{Face: "inside", Rect: [4]float64{1696, 507, 2036, 1002}, Flip: "x"},
			{Face: "inside", Rect: [4]float64{865, 1025, 1203, 1520}, Flip: "x"},
			{Face: "inside", Rect: [4]float64{1201, 1025, 2038, 1520}, Flip: "x"},
			{Face: "inside", Rect: [4]float64{29, 1025, 866, 1520}, Flip: "x"},
			{Face: "bottom", Rect: [4]float64{24, 9, 976, 498}, Turn: true},
		})
}

// Game look box bulk. How to read and edit this: facemap.go.
func faceMapLookBulk() Model {
	return faceMap(Model{Kind: "Box look", Mesh: "JankCardBox_Closed", TextureSize: 512, Icon: "mesh",
		Size:    [3]float64{1.54, 0.8, 0.86},
		Preview: []PreviewPart{},
		Faces: []Face{
			{ID: "front", Label: "Front", Net: [4]float64{0, 0, 1.54, 0.8}},
			{ID: "top", Label: "Lid", Net: [4]float64{0, -0.86, 1.54, 0.86}},
			{ID: "rest", Label: "Back, sides & inside (colour)", Net: [4]float64{1.64, 0, 0.5, 0.5}, Hidden: true},
		}},
		[]Row{
			{Face: "front", Rect: [4]float64{12, 11, 410, 265}},
			{Face: "top", Rect: [4]float64{11, 307, 411, 465}, Flip: "xy"},
			{Face: "rest", Rect: [4]float64{415, 10, 504, 58}},
			{Face: "rest", Rect: [4]float64{413, 399, 506, 501}},
		})
}

// Game look box bag. How to read and edit this: facemap.go.
func faceMapLookBag() Model {
	return faceMap(Model{Kind: "Box look", Mesh: "PaperBagClosed", TextureSize: 1024, Icon: "mesh",
		Size:    [3]float64{3.07, 4, 1.36},
		Preview: []PreviewPart{},
		Faces: []Face{
			{ID: "left", Label: "Left side", Net: [4]float64{0, 0, 1.36, 4}},
			{ID: "front", Label: "Front", Net: [4]float64{1.36, 0, 3.07, 4}},
			{ID: "right", Label: "Right side", Net: [4]float64{4.43, 0, 1.36, 4}},
			{ID: "back", Label: "Back", Net: [4]float64{5.79, 0, 3.07, 4}},
			{ID: "bottom", Label: "Bottom", Net: [4]float64{1.36, 4, 3.07, 1.36}, Hidden: true},
			{ID: "inside", Label: "Inside walls (all four)", Net: [4]float64{0, 5.66, 3.07, 4}, Hidden: true},
			{ID: "floor", Label: "Inside floor", Net: [4]float64{3.3699999999999997, 5.66, 3.07, 1.36}, Hidden: true},
			{ID: "handles", Label: "Handles (colour)", Net: [4]float64{6.739999999999999, 5.66, 1, 2}, Hidden: true},
		}},
		[]Row{
			{Face: "left", Rect: [4]float64{772, 0, 908, 597}},
			{Face: "front", Rect: [4]float64{0, 0, 387, 597}},
			{Face: "right", Rect: [4]float64{886, 0, 1023, 597}, Flip: "xy"},
			{Face: "back", Rect: [4]float64{386, 0, 773, 597}},
			{Face: "bottom", Rect: [4]float64{0, 596, 386, 784}},
			{Face: "inside", Rect: [4]float64{494, 600, 712, 1023}},
			{Face: "inside", Rect: [4]float64{798, 601, 1024, 1023}},
			{Face: "inside", Rect: [4]float64{393, 597, 496, 1023}},
			{Face: "inside", Rect: [4]float64{711, 598, 799, 1023}},
			{Face: "floor", Rect: [4]float64{109, 787, 389, 1020}},
			{Face: "handles", Rect: [4]float64{1, 788, 107, 1019}},
		})
}

// Game look box streetBillboard. How to read and edit this: facemap.go.
func faceMapLookStreetBillboard() Model {
	return faceMap(Model{Kind: "Box look", Mesh: "streetBillboard", TextureSize: 1024, Icon: "mesh",
		Size:    [3]float64{9.47, 7.44, 1},
		Preview: []PreviewPart{},
		Faces: []Face{
			{ID: "sides", Label: "Top board", Net: [4]float64{0, 0, 4.53, 3.1}},
			{ID: "big", Label: "Big board", Net: [4]float64{0, 3.6, 9.47, 3.74}},
		}},
		[]Row{
			{Face: "sides", Rect: [4]float64{29, 37, 484, 349}},
			{Face: "big", Rect: [4]float64{38, 407, 978, 781}},
		})
}

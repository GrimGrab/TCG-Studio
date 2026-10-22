package uvmap

// Game look boxes: the cardboard boxes (restock big/small, the box furniture comes in, the graded-card return box), the
// bulk card box, the paper shopping bag and the street billboards. Painted over the vanilla UVs (the mod swaps only the texture, so the boxes' open/close rigs keep working).
// Measured from the game 1.02 meshes (gameextract boxes\: the closed static mesh, the graded box's animated mesh) with
// uvdebug; flips from each island's UV direction against the face seen from outside (inside faces: seen from inside).
// Cardboard nets are in decimetres (the boxes are 0.2 units), the bulk box in object units. A closed box's top is its two
// outer flaps (left and right half); the two flaps under them, the inside and the bottom are hidden faces.

// cardboard builds a closed box's net: the four walls in a row, top above the front, bottom below it, then the hidden faces
// in a second row (flaps under the top, inside walls, inside floor, flap undersides).
func cardboard(mesh string, w, h, d float64, walls [4][]Target, top, under, bottom, inWalls, floor, flapsIn []Target) Model {
	y2 := h + d + 0.3
	faces := []Face{
		face("left", "Left side", [4]float64{0, 0, d, h}, walls[0]...),
		face("front", "Front", [4]float64{d, 0, w, h}, walls[1]...),
		face("right", "Right side", [4]float64{d + w, 0, d, h}, walls[2]...),
		face("back", "Back", [4]float64{2*d + w, 0, w, h}, walls[3]...),
		face("top", "Top (outer flaps)", [4]float64{d, -d, w, d}, top...),
		hidden(face("bottom", "Bottom", [4]float64{d, h, w, d}, bottom...)),
		hidden(face("under", "Flaps under the top", [4]float64{2*d + w, -d, w, d}, under...)),
		hidden(face("inside", "Inside walls (all four)", [4]float64{0, y2, w, h}, inWalls...)),
		hidden(face("floor", "Inside floor", [4]float64{w + 0.3, y2, w, d}, floor...)),
		hidden(face("flapsIn", "Flap undersides (all four)", [4]float64{2*w + 0.6, y2, w, d / 2}, flapsIn...)),
	}
	return Model{Kind: "Box look", Mesh: mesh, TextureSize: 2048, Icon: "mesh", Size: [3]float64{w, h, d}, Faces: faces}
}

var (
	leftHalf  = [4]float64{0, 0, 0.5, 1}
	rightHalf = [4]float64{0.5, 0, 1, 1}
)

func half(rect [4]float64, src [4]float64, transpose, flipX, flipY bool) Target {
	return Target{Rect: rect, Src: src, Transpose: transpose, FlipX: flipX, FlipY: flipY}
}

func fx(rect [4]float64) Target { return atT(rect, false, true, false) }

var boxLooks = map[string]Model{
	// Restock box, big (CardboardBox_20x20x20, 2048²).
	"restockBig": cardboard("CardboardBox_20x20x20_StaticClosed", 1.97, 2.01, 1.97,
		[4][]Target{{at([4]float64{1027, 1506, 1535, 2040})}, {at([4]float64{1534, 1506, 2040, 2040})},
			{at([4]float64{24, 1506, 531, 2040})}, {at([4]float64{530, 1506, 1039, 2040})}},
		[]Target{half([4]float64{1038, 1246, 1534, 1507}, leftHalf, true, false, true), half([4]float64{25, 1246, 521, 1504}, rightHalf, true, true, false)},
		[]Target{half([4]float64{1544, 1246, 2040, 1511}, [4]float64{0, 0.5, 1, 1}, false, false, false), half([4]float64{531, 1246, 1027, 1511}, [4]float64{0, 0, 1, 0.5}, false, true, true)},
		[]Target{atT([4]float64{32, 8, 536, 519}, true, false, false)},
		[]Target{fx([4]float64{1266, 769, 1679, 1232}), fx([4]float64{441, 769, 855, 1232}), fx([4]float64{854, 763, 1267, 1232}), fx([4]float64{28, 763, 442, 1232})},
		[]Target{atT([4]float64{559, 16, 1061, 517}, false, true, true)},
		[]Target{atT([4]float64{1266, 539, 1679, 764}, false, false, true), atT([4]float64{441, 539, 854, 764}, false, true, false),
			atT([4]float64{28, 539, 441, 760}, true, false, false), atT([4]float64{854, 539, 1266, 760}, true, true, true)}),
	// Restock box, small (CardboardBox_20x20x10, 2048²); its front lies on its side in the texture.
	"restockSmall": cardboard("CardboardBox_20x20x10_StaticClosed", 1.97, 1.01, 1.97,
		[4][]Target{{at([4]float64{1156, 1732, 1727, 2032})}, {atT([4]float64{1740, 304, 2034, 874}, true, false, true)},
			{at([4]float64{27, 1736, 599, 2032})}, {at([4]float64{598, 1741, 1168, 2032})}},
		[]Target{half([4]float64{1169, 1449, 1727, 1736}, leftHalf, true, false, true), half([4]float64{27, 1449, 586, 1733}, rightHalf, true, true, false)},
		[]Target{half([4]float64{1472, 11, 2031, 296}, [4]float64{0, 0.5, 1, 1}, false, false, false), half([4]float64{598, 1449, 1156, 1742}, [4]float64{0, 0, 1, 0.5}, false, true, true)},
		[]Target{atT([4]float64{30, 17, 684, 660}, true, false, false)},
		[]Target{atT([4]float64{1746, 1466, 2035, 2032}, true, false, false), fx([4]float64{593, 1126, 1159, 1416}), fx([4]float64{1158, 1117, 1724, 1416}), fx([4]float64{29, 1117, 594, 1416})},
		[]Target{atT([4]float64{699, 16, 1284, 601}, false, true, true)},
		[]Target{atT([4]float64{1751, 884, 2034, 1449}, true, true, true), atT([4]float64{594, 835, 1159, 1120}, false, true, false),
			atT([4]float64{29, 835, 594, 1114}, true, false, false), atT([4]float64{1159, 835, 1722, 1114}, true, true, true)}),
	// The box furniture comes in (CardboardBox_30x20x20, 2048²); its flaps under the top cover only the front and back third.
	"furnitureBox": cardboard("CardboardBox_30x20x20_StaticClosed", 1.97, 2.03, 3.0,
		[4][]Target{{at([4]float64{1025, 1599, 1664, 2018})}, {at([4]float64{1656, 1602, 2032, 2018})},
			{at([4]float64{25, 1599, 657, 2018})}, {at([4]float64{656, 1605, 1026, 2018})}},
		[]Target{half([4]float64{1032, 1400, 1656, 1602}, leftHalf, true, false, true), half([4]float64{25, 1400, 650, 1602}, rightHalf, true, true, false)},
		[]Target{half([4]float64{1664, 1400, 2031, 1606}, [4]float64{0, 2.0 / 3, 1, 1}, false, false, false), half([4]float64{657, 1400, 1026, 1603}, [4]float64{0, 0, 1, 1.0 / 3}, false, true, true)},
		[]Target{atT([4]float64{37, 225, 882, 765}, true, false, false)},
		[]Target{fx([4]float64{1451, 988, 1775, 1374}), fx([4]float64{577, 988, 903, 1374}), fx([4]float64{902, 982, 1452, 1374}), fx([4]float64{29, 982, 578, 1374})},
		[]Target{atT([4]float64{912, 222, 1768, 766}, true, true, false)},
		[]Target{atT([4]float64{29, 796, 578, 979}, true, false, false), atT([4]float64{902, 796, 1450, 979}, true, true, true),
			atT([4]float64{1451, 796, 1775, 986}, false, false, true), atT([4]float64{577, 796, 902, 984}, false, true, false)}),
}

func init() {
	// The graded-card return box (CardboardBox_20x10x7, 2048², animated mesh only) is an open tray: no top, the inside floor
	// and walls are seen from above.
	w, h, d := 0.97, 1.18, 2.0
	boxLooks["gradedBox"] = Model{Kind: "Box look", Mesh: "CardboardBox_20x10x7_Animated_Model", TextureSize: 2048, Icon: "mesh",
		Size: [3]float64{w, h, d}, Faces: []Face{
			face("left", "Left side", [4]float64{0, 0, d, h}, at([4]float64{1198, 1543, 2035, 2033})),
			face("front", "Front", [4]float64{d, 0, w, h}, at([4]float64{1693, 10, 2038, 501})),
			face("right", "Right side", [4]float64{d + w, 0, d, h}, at([4]float64{32, 1543, 869, 2033})),
			face("back", "Back", [4]float64{2*d + w, 0, w, h}, at([4]float64{868, 1543, 1199, 2033})),
			face("floor", "Inside floor", [4]float64{d, -d, w, d}, atT([4]float64{26, 518, 949, 998}, true, false, true)),
			face("inside", "Inside walls (all four)", [4]float64{2*d + w, -d, w, h},
				fx([4]float64{1697, 508, 2035, 1001}), fx([4]float64{865, 1026, 1203, 1519}), fx([4]float64{1201, 1026, 2037, 1519}), fx([4]float64{30, 1026, 866, 1519})),
			hidden(face("bottom", "Bottom", [4]float64{d, h, w, d}, atT([4]float64{25, 10, 975, 497}, true, false, false))),
		}}
	// Bulk card box (JankCardBox_Closed, 512², one mesh for every variant): the printed front and lid; the back, sides,
	// rims and the cards inside read small swatches (colour only).
	boxLooks["bulk"] = Model{Kind: "Box look", Mesh: "JankCardBox_Closed", TextureSize: 512, Icon: "mesh",
		Size: [3]float64{1.54, 0.80, 0.86}, Faces: []Face{
			face("front", "Front", [4]float64{0, 0, 1.54, 0.80}, at([4]float64{13, 12, 409, 264})),
			face("top", "Lid", [4]float64{0, -0.86, 1.54, 0.86}, atT([4]float64{12, 308, 410, 464}, false, true, true)),
			hidden(face("rest", "Back, sides & inside (colour)", [4]float64{1.64, 0, 0.5, 0.5},
				at([4]float64{416, 11, 503, 57}), at([4]float64{414, 400, 505, 500}))),
		}}
}

func init() {
	// Paper shopping bag (PaperBagClosed / PaperBagOpen, 1024², the same UVs; the open bag's texture has the handles drawn on
	// its sides). Front = the +z side; the side on its right (−x) lies upside down in the texture. The inside walls, both
	// floors and the rope handles are hidden faces.
	w, h, d := 3.07, 4.0, 1.36
	y2 := h + d + 0.3
	boxLooks["bag"] = Model{Kind: "Box look", Mesh: "PaperBagClosed", TextureSize: 1024, Icon: "mesh", Size: [3]float64{w, h, d},
		Faces: []Face{
			face("left", "Left side", [4]float64{0, 0, d, h}, at([4]float64{773, 1, 908, 596})),
			face("front", "Front", [4]float64{d, 0, w, h}, at([4]float64{0, 1, 386, 596})),
			face("right", "Right side", [4]float64{d + w, 0, d, h}, atT([4]float64{886, 1, 1022, 596}, false, true, true)),
			face("back", "Back", [4]float64{2*d + w, 0, w, h}, at([4]float64{387, 1, 772, 596})),
			hidden(face("bottom", "Bottom", [4]float64{d, h, w, d}, at([4]float64{1, 597, 385, 783}))),
			hidden(face("inside", "Inside walls (all four)", [4]float64{0, y2, w, h},
				at([4]float64{494, 600, 712, 1022}), at([4]float64{798, 601, 1023, 1022}), at([4]float64{394, 598, 496, 1022}), at([4]float64{711, 599, 799, 1022}))),
			hidden(face("floor", "Inside floor", [4]float64{w + 0.3, y2, w, d}, at([4]float64{110, 788, 388, 1019}))),
			hidden(face("handles", "Handles (colour)", [4]float64{2*w + 0.6, y2, 1, 2}, at([4]float64{2, 789, 106, 1018}))),
		}}
	// Street billboards on building 32 (mcp_building_32_billboards_d, 1024², the ads' submesh): a big board wrapping three
	// sides of the corner, above it two side boards (one picture) and a front board. Net in texture pixels / 100.
	boxLooks["streetBillboard"] = Model{Kind: "Box look", Mesh: "streetBillboard", TextureSize: 1024, Icon: "mesh", Size: [3]float64{9.47, 7.44, 1},
		Faces: []Face{
			face("sides", "Top boards, left and right", [4]float64{0, 0, 4.53, 3.10}, at([4]float64{30, 38, 483, 348})),
			face("middle", "Top board, middle", [4]float64{4.94, 0, 4.53, 3.10}, at([4]float64{533, 38, 986, 348})),
			face("big", "Big board (all three sides)", [4]float64{0, 3.6, 9.47, 3.74}, at([4]float64{39, 408, 986, 782})),
		}}
}

// BoxLook is the layout of a Game look box: "restockBig", "restockSmall", "furnitureBox", "gradedBox", "bulk", "bag" or
// "streetBillboard".
func BoxLook(id string) (Model, bool) {
	m, ok := boxLooks[id]
	return m, ok
}

// BoxLooks lists the box layouts (tests).
func BoxLooks() []string {
	var out []string
	for id := range boxLooks {
		out = append(out, id)
	}
	return out
}

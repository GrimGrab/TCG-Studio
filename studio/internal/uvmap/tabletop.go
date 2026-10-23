package uvmap

// Tabletop items (the Tabletop Geek phone shop): the Speedrobo board games and the Ultra-PRO card supplies. Unlike the other
// kinds, every vanilla item has its own mesh, so each base has its own layout (ModelForBase). Measured from the game 1.02
// meshes with uvdebug (rects incl. the rounded lid edges next to an island); orientations checked with TestRenderOrientation.
// Where the game's mesh reads one texture area from several sides (a box's top and bottom, both spines), that area is one face.

func ttModel(kind, mesh string, tex int, size [3]float64, faces ...Face) Model {
	return Model{Kind: kind, Mesh: mesh, TextureSize: tex, Icon: "mesh", Preview: mainPart(mesh), Size: size, Faces: faces}
}

func face(id, label string, net [4]float64, targets ...Target) Face {
	return Face{ID: id, Label: label, Net: net, Targets: targets}
}

func hidden(f Face) Face { f.Hidden = true; return f }

func at(rect [4]float64) Target { return Target{Rect: rect, Src: full} }

func atT(rect [4]float64, transpose, flipX, flipY bool) Target {
	return Target{Rect: rect, Src: full, Transpose: transpose, FlipX: flipX, FlipY: flipY}
}

// Board games: cardboard boxes with a lid. W × H × D = the box's width, height and depth (object units).
var tabletop = map[string]Model{
	// Tall box; both spines read one strip (the right one turned upside down), the lid top and the bottom one picture.
	"Boardgame_Speedrobo_Necro": ttModel("BoardGame", "BoardGameMesh_Necromansters", 1024, [3]float64{1.298, 2.571, 0.822},
		face("left", "Sides (both)", [4]float64{0, 0, 0.8, 2.55}, at([4]float64{492, 28, 694, 846})),
		face("front", "Front", [4]float64{0.8, 0, 1.29, 2.55}, at([4]float64{18, 29, 476, 946})),
		face("top", "Top & bottom", [4]float64{0.8, -0.79, 1.29, 0.79}, at([4]float64{495, 843, 838, 1012})),
		face("back", "Back", [4]float64{2.19, 0, 1.29, 2.55}, at([4]float64{703, 28, 1012, 725}))),
	// The back lies upside down in the texture; the right side is mirrored.
	"Boardgame_Speedrobo_Claim": ttModel("BoardGame", "BoardGameMesh_Claim", 1024, [3]float64{1.526, 2.091, 0.968},
		face("left", "Left side", [4]float64{0, 0, 0.94, 2.075}, at([4]float64{471, 159, 623, 591})),
		face("front", "Front", [4]float64{0.94, 0, 1.515, 2.075}, at([4]float64{152, 160, 473, 591})),
		face("right", "Right side", [4]float64{2.455, 0, 0.94, 2.075}, atT([4]float64{0, 159, 152, 591}, false, true, false)),
		face("back", "Back", [4]float64{3.395, 0, 1.515, 2.075}, atT([4]float64{156, 593, 470, 1021}, false, true, true)),
		face("top", "Top & bottom", [4]float64{0.94, -0.93, 1.515, 0.93}, at([4]float64{155, 1, 471, 159}))),
	// Wide box; the lid's top, the bottom and both ends all read the lid picture.
	"Boardgame_Speedrobo_Mafia": ttModel("BoardGame", "BoardGameMesh_MafiaWorks", 1024, [3]float64{3.042, 1.973, 1.072},
		face("front", "Front", [4]float64{0, 0, 3.021, 1.935}, at([4]float64{480, 632, 1016, 1006})),
		face("top", "Top (also bottom & ends)", [4]float64{0, -1.04, 3.021, 1.04}, at([4]float64{634, 2, 1022, 167})),
		face("back", "Back", [4]float64{3.121, 0, 3.0, 1.935}, at([4]float64{638, 178, 1017, 445}))),
	// Wide boxes; the top and bottom read the lower band of the back picture.
	"Boardgame_Speedrobo_SystemGate1": ttModel("BoardGame", "BoardGameMesh_SystemGate1", 1024, [3]float64{3.042, 1.973, 0.836},
		face("left", "Left side", [4]float64{0, 0, 0.813, 1.958}, atT([4]float64{17, 348, 122, 666}, false, true, false)),
		face("front", "Front", [4]float64{0.813, 0, 3.021, 1.958}, at([4]float64{146, 348, 651, 667})),
		face("right", "Right side", [4]float64{3.834, 0, 0.813, 1.958}, at([4]float64{17, 18, 122, 337})),
		face("back", "Back (lower part = top & bottom)", [4]float64{4.647, 0, 3.0, 1.958}, at([4]float64{145, 20, 649, 340}))),
	"Boardgame_Speedrobo_SystemGate2": ttModel("BoardGame", "BoardGameMesh_SystemGate2", 1024, [3]float64{3.042, 1.973, 0.828},
		face("left", "Left side", [4]float64{0, 0, 0.805, 1.958}, at([4]float64{787, 361, 1002, 692})),
		face("front", "Front", [4]float64{0.805, 0, 3.021, 1.958}, at([4]float64{18, 678, 533, 1006})),
		face("right", "Right side", [4]float64{3.826, 0, 0.805, 1.958}, at([4]float64{788, 17, 1003, 346})),
		face("back", "Back (lower part = top & bottom)", [4]float64{4.631, 0, 3.0, 1.958}, at([4]float64{549, 706, 1018, 1006}))),

	// Card supplies (Ultra-PRO).
	// A clear magnetic case; the four rim edges share one strip (colour only).
	"UP_MagneticHolder": ttModel("CardSupply", "MagneticHolder_Mesh", 1024, [3]float64{0.727, 1.141, 0.093},
		face("front", "Front", [4]float64{0, 0, 0.727, 1.141}, at([4]float64{529, 28, 991, 744})),
		face("back", "Back", [4]float64{0.827, 0, 0.727, 1.141}, at([4]float64{26, 33, 495, 740})),
		hidden(face("edges", "Rim (all edges)", [4]float64{1.654, 0, 0.6, 0.13}, at([4]float64{70, 809, 935, 993})))),
	// A stack of clear toploaders in a printed card header; the stack's edges share one strip (colour only).
	"UP_Toploader": ttModel("CardSupply", "Toploader_Mesh", 1024, [3]float64{0.716, 1.066, 0.382},
		face("clearFront", "Toploader front", [4]float64{0.399, 0, 0.682, 1.057}, at([4]float64{404, 30, 732, 508})),
		face("clearBack", "Toploader back", [4]float64{1.479, 0, 0.682, 1.057}, at([4]float64{29, 31, 361, 506})),
		face("left", "Card left", [4]float64{0, 1.157, 0.382, 0.618}, at([4]float64{548, 722, 762, 1023})),
		face("front", "Card front", [4]float64{0.382, 1.349, 0.715, 0.426}, at([4]float64{15, 516, 460, 740})),
		face("right", "Card right", [4]float64{1.097, 1.157, 0.382, 0.618}, at([4]float64{335, 721, 544, 1022})),
		face("back", "Card back", [4]float64{1.479, 1.157, 0.715, 0.618}, at([4]float64{10, 769, 323, 1010})),
		hidden(face("edges", "Stack edges", [4]float64{2.294, 0, 0.34, 1.048}, at([4]float64{775, 12, 988, 1012})))),
	// A bag of sleeves around a printed card; the bag's top (hang hole) reads one area from both sides.
	"UP_PennySleeves": ttModel("CardSupply", "PennySleeves_Mesh", 1024, [3]float64{0.726, 1.238, 0.208},
		face("header", "Bag top (both sides)", [4]float64{0, -0.321, 0.726, 0.221}, at([4]float64{11, 574, 732, 872})),
		face("front", "Front", [4]float64{0, 0, 0.693, 0.992}, at([4]float64{394, 22, 748, 517})),
		face("back", "Back", [4]float64{0.793, 0, 0.693, 0.992}, at([4]float64{13, 22, 367, 517})),
		hidden(face("edges", "Stack edges", [4]float64{1.586, 0, 0.206, 0.988}, at([4]float64{775, 12, 988, 1012})))),
	// Storage box; both sides read one picture, the top and the bottom another (on its side in the texture).
	"UP_SemiRigidCardHolder": ttModel("CardSupply", "SemiRigidCardHolder_Mesh", 1024, [3]float64{1.562, 1.153, 0.92},
		face("left", "Sides (both)", [4]float64{0, 0, 0.894, 1.143}, at([4]float64{692, 555, 1009, 1001})),
		face("front", "Front", [4]float64{0.894, 0, 1.551, 1.125}, at([4]float64{12, 24, 737, 526})),
		face("back", "Back", [4]float64{2.545, 0, 1.519, 1.116}, at([4]float64{11, 555, 685, 1001})),
		face("top", "Top & bottom", [4]float64{0.894, -0.889, 1.551, 0.889}, atT([4]float64{771, 25, 1003, 527}, true, true, false))),
	// Box with a hang tab above its back; the back (incl. the tab's back) lies on its side in the texture. 2048² texture.
	"UP_CardPreserver": ttModel("CardSupply", "CardPreserverBox_Mesh", 2048, [3]float64{0.847, 1.382, 0.244},
		face("left", "Left side", [4]float64{0, 0, 0.241, 1.167}, at([4]float64{146, 28, 481, 1492})),
		face("front", "Front", [4]float64{0.241, 0, 0.847, 1.167}, at([4]float64{486, 30, 1566, 1493})),
		face("right", "Right side", [4]float64{1.088, 0, 0.241, 1.167}, at([4]float64{1570, 28, 1905, 1492})),
		face("top", "Top", [4]float64{0.241, -0.23, 0.847, 0.23}, at([4]float64{1264, 1513, 2024, 1809})),
		face("tab", "Hang tab front", [4]float64{0.241, -0.545, 0.847, 0.215}, at([4]float64{1261, 1842, 2048, 2021})),
		face("back", "Back (with the tab)", [4]float64{1.429, -0.215, 0.844, 1.382}, atT([4]float64{50, 1529, 785, 1996}, true, false, false)),
		hidden(face("bottom", "Bottom", [4]float64{0.241, 1.267, 0.847, 0.234}, at([4]float64{805, 1536, 1246, 1741})))),
	// Open-top box of pages. 2048² texture.
	"UP_PlatinumSeriesPocketPages": ttModel("CardSupply", "PocketPages_Mesh", 2048, [3]float64{2.196, 2.948, 0.404},
		face("left", "Left side", [4]float64{0, 0, 0.393, 2.923}, at([4]float64{1568, 101, 1740, 1709})),
		face("front", "Front", [4]float64{0.393, 0, 2.18, 2.923}, at([4]float64{11, 59, 1320, 1915})),
		face("right", "Right side", [4]float64{2.573, 0, 0.393, 2.923}, at([4]float64{1339, 74, 1506, 1723})),
		face("back", "Back label", [4]float64{3.066, 1.476, 1.094, 1.447}, at([4]float64{1760, 1391, 2042, 2039}))),
	// Tall box; all four sides lie on their side in the texture. Lid top and bottom share one area.
	"UP_TowerDeckbox": ttModel("CardSupply", "TowerDeckbox_Mesh", 1024, [3]float64{0.82, 1.352, 0.82},
		face("left", "Left side", [4]float64{0, 0, 0.821, 1.343}, at([4]float64{7, 52, 370, 558})),
		face("front", "Front", [4]float64{0.821, 0, 0.821, 1.346}, atT([4]float64{20, 574, 721, 1011}, true, false, true)),
		face("right", "Right side", [4]float64{1.642, 0, 0.821, 1.346}, at([4]float64{386, 46, 729, 560})),
		face("back", "Back", [4]float64{2.463, 0, 0.821, 1.346}, at([4]float64{743, 37, 1012, 467})),
		face("top", "Lid top & bottom", [4]float64{0.821, -0.821, 0.821, 0.821}, atT([4]float64{755, 489, 998, 732}, false, true, true))),
}

func init() {
	// UP playmat tube: the four vanilla colours are one box shape (own meshes UltraProPlaymat<Colour>_Mesh, same box areas) and differ
	// only in which swatch the rolled mat inside reads (gray 876–1088, green 64–447, purple 447–636, yellow 636–876 × 1368–1961), so
	// UP_Playmat1 (gray) is the one base and the mat colour is a face. The four long sides lie on their side. 2048² texture.
	tabletop["UP_Playmat1"] = ttModel("CardSupply", "UltraProPlaymatGray_Mesh", 2048, [3]float64{0.508, 3.003, 0.415},
		face("left", "Left side", [4]float64{0, 0, 0.41, 2.674}, atT([4]float64{22, 677, 2035, 946}, true, false, true)),
		face("front", "Front", [4]float64{0.41, 0, 0.507, 2.674}, atT([4]float64{18, 367, 2032, 648}, true, false, true)),
		face("right", "Right side", [4]float64{0.917, 0, 0.41, 2.674}, atT([4]float64{12, 1000, 2037, 1267}, true, true, false)),
		face("back", "Back (with the tab)", [4]float64{1.327, -0.329, 0.505, 3.003}, atT([4]float64{18, 50, 2031, 321}, true, false, true)),
		face("tab", "Hang tab front", [4]float64{0.41, -0.722, 0.504, 0.329}, at([4]float64{1186, 1689, 1984, 1867})),
		face("top", "Top end", [4]float64{0.41, -0.393, 0.496, 0.393}, at([4]float64{1189, 1359, 1949, 1656})),
		face("mat", "Mat inside (its colour)", [4]float64{1.932, 0, 0.3, 0.6}, at([4]float64{876, 1368, 1088, 1961})))
	// Collectors Album: the collection book's mesh (ItemBinderBook + pages) with a 2048² texture — the binder layout at 2×.
	tabletop["UP_Album"] = scaled(binder, "CardSupply", 2)
}

// scaled copies a layout for a texture k times as large (same UVs).
func scaled(m Model, kind string, k float64) Model {
	m.Kind, m.TextureSize = kind, int(float64(m.TextureSize)*k)
	faces := make([]Face, len(m.Faces))
	for i, f := range m.Faces {
		f.Targets = append([]Target(nil), f.Targets...)
		for j := range f.Targets {
			for c := range f.Targets[j].Rect {
				f.Targets[j].Rect[c] *= k
			}
		}
		faces[i] = f
	}
	m.Faces = faces
	return m
}

// ModelForBase returns the layout of an accessory's model: per base for the tabletop kinds, per kind otherwise.
func ModelForBase(kind, base string) (Model, bool) {
	if m, ok := tabletop[base]; ok && m.Kind == kind {
		return inclusive(m), true
	}
	return ModelFor(kind)
}

// TabletopBases lists the vanilla items that have their own layout (tests).
func TabletopBases() []string {
	var out []string
	for b := range tabletop {
		out = append(out, b)
	}
	return out
}

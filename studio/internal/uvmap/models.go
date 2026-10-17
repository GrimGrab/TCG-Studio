package uvmap

// Editable layouts of the game's accessory models (game 1.02). Every vanilla item of a kind shares one mesh (deck boxes
// "DeckBox", playmats "PlayMatMesh" rolled / "PlayMatFlatMesh" on tables, sleeves "ItemCardSleeveMesh", dice "DiceBox",
// comics "Manga_Mesh", collection books "ItemBinderBook", battle decks "PerconDeck_Mesh"); they differ only in texture, so one layout per kind covers all. Measured from the meshes the mod exports (see TestModelsMatchMeshes).
//
// A face is a rectangle in the editor's unfolded "net" (object units, y down) seen from outside, upright. Each target copies a
// part of the face (Src, normalized 0..1 face coords) into a texture rectangle (Rect, pixels in a 1024² texture, origin
// top-left). Bleed targets are painted first and slightly larger so seams never show vanilla pixels.

type Target struct {
	Rect  [4]float64 `json:"rect"` // x0, y0, x1, y1 in texture pixels (top-left origin)
	Src   [4]float64 `json:"src"`  // u0, v0, u1, v1 inside the face (0..1, top-left origin)
	FlipX bool       `json:"flipX,omitempty"`
	FlipY bool       `json:"flipY,omitempty"`
	// Transpose: the face lies on its side in the texture — face x runs along texture y and face y along texture x (applied
	// before FlipX/FlipY, which act in texture space).
	Transpose bool `json:"transpose,omitempty"`
	Bleed     bool `json:"bleed,omitempty"`
}

type Face struct {
	ID     string     `json:"id"`
	Label  string     `json:"label"`
	Net    [4]float64 `json:"net"`              // x, y, w, h in the net (object units)
	Hidden bool       `json:"hidden,omitempty"` // mostly not visible in game (still painted)
	// Shown: the part of the face the item shows (x, y, w, h fractions of Net); nil = all of it. The rest covers texture the
	// meshes don't show (the pack's outer strips): the editor places and fits art on the shown part.
	Shown   *[4]float64 `json:"shown,omitempty"`
	Targets []Target    `json:"targets"`
}

// View is a labelled frame drawn as a guide in the net (furniture: Front, Left, Top… — each holds many faces).
type View struct {
	Label string     `json:"label"`
	Net   [4]float64 `json:"net"`
}

type Model struct {
	Kind        string `json:"kind"`
	Mesh        string `json:"mesh"`
	TextureSize int    `json:"textureSize"`
	// Box size for the 3D preview (object units): width (x), height (y), depth (z).
	Size  [3]float64 `json:"size"`
	Faces []Face     `json:"faces"`
	// Palette: the texture is a colour palette (each face a swatch) — recoloured, not painted.
	Palette bool `json:"palette,omitempty"`
	// Preview: the exported meshes the 3D preview draws (templates\accessories\<mesh>.obj).
	Preview []PreviewPart `json:"preview"`
	// Icon: how the studio renders the shop icon — "box" (front + right side + lid, oblique), "tilt" (front face, tilted like
	// the vanilla playmat/book icons), "recolor" (the vanilla icon with the palette swatches recoloured), "pack" / "mesh" (the
	// preview model rendered with the texture).
	Icon string `json:"icon"`
	// Bases: vanilla items the art can start from (pack/box editor). Each names its texture in the game templates
	// and, when that item's mesh reads a different texture area, where each face is in it (face id → targets; nil = Faces).
	Bases []Base `json:"bases,omitempty"`
	// Projected: no faces — the net is the Views, and the editor's GPU painter projects the layers onto every surface by its
	// place in its view (Preview[0] is the .bin of gameextract/furniture_paint.go). Density: atlas pixels per net unit.
	// NetMoves: how the net changed in a later NetVersion (layouts made before it get their layers moved, MigrateLayout).
	NetMoves  []NetMove `json:"-"`
	Projected bool      `json:"projected,omitempty"`
	Views     []View    `json:"views,omitempty"`
	Density   float64   `json:"density,omitempty"`
	// Turn: extra turn of the 3D preview / icon camera around the vertical axis (radians) — furniture fronts face +z.
	Turn float64 `json:"turn,omitempty"`
}

type Base struct {
	ID      string              `json:"id"`
	Label   string              `json:"label"`
	Texture string              `json:"texture"`
	Targets map[string][]Target `json:"targets,omitempty"`
}

var full = [4]float64{0, 0, 1, 1}

// edgeSwatch: a thin strip at the middle of a face's left edge — the colour that small swatches of a part's card edges take.
var edgeSwatch = [4]float64{0.01, 0.4, 0.05, 0.6}

// PreviewPart is one mesh of the 3D preview. Texture "main" = the texture being edited, "secondary" = the base item's
// second texture (<base>_texture2.png, e.g. binder pages); Glass parts are drawn see-through and untextured.
type PreviewPart struct {
	Mesh    string `json:"mesh"`
	URL     string `json:"url,omitempty"` // where the UI loads it ("" = /acctemplates/<mesh>.obj)
	Texture string `json:"texture,omitempty"`
	Glass   bool   `json:"glass,omitempty"`
}

func mainPart(mesh string) []PreviewPart { return []PreviewPart{{Mesh: mesh, Texture: "main"}} }

// Deck box: a tray (sides/back/inner front share one strip y 44–484, a continuous panorama back½ | left | front | right | back½)
// with a separate front cover (the big print), a lid top and a bottom.
var deckbox = Model{
	Kind: "Deckbox", Mesh: "DeckBox", TextureSize: 1024, Icon: "box",
	Preview: mainPart("DeckBox"),
	Size:    [3]float64{0.914, 1.225, 0.862},
	Faces: []Face{
		{ID: "left", Label: "Left side", Net: [4]float64{0, 0, 0.862, 1.225}, Targets: []Target{
			{Rect: [4]float64{189, 42, 419, 486}, Src: full}}},
		{ID: "front", Label: "Front", Net: [4]float64{0.862, 0, 0.914, 1.225}, Targets: []Target{
			// The tray's own front (under the cover; its top edge shows above the cover). The tray strip (y 44–484) reaches 2 px
			// further up and down into unused texels, so mipmaps don't draw a red line along the top of the sides.
			{Rect: [4]float64{419, 42, 651, 486}, Src: full},
			// The cover spans y −0.550…0.525 of the tray's −0.593…0.632. Its island is x 62–400 × y 538–968; the rect reaches 2 px
			// further into the unused texels around it, so mipmaps don't pull the vanilla red into its edges.
			{Rect: [4]float64{60, 536, 402, 970}, Src: [4]float64{0, 0.087, 1, 0.965}}}},
		{ID: "right", Label: "Right side", Net: [4]float64{1.776, 0, 0.862, 1.225}, Targets: []Target{
			{Rect: [4]float64{651, 42, 881, 486}, Src: full}}},
		{ID: "back", Label: "Back", Net: [4]float64{2.638, 0, 0.862, 1.225}, Targets: []Target{
			{Rect: [4]float64{881, 42, 996, 486}, Src: [4]float64{0, 0, 0.5, 1}},
			{Rect: [4]float64{74, 42, 189, 486}, Src: [4]float64{0.5, 0, 1, 1}}}},
		// Lid island x 731–991 × y 535–781, plus 2 px around it.
		{ID: "top", Label: "Lid", Net: [4]float64{0.862, -0.864, 0.914, 0.864}, Targets: []Target{
			{Rect: [4]float64{729, 533, 993, 783}, Src: full}}},
		{ID: "bottom", Label: "Bottom", Net: [4]float64{0.862, 1.225, 0.914, 0.864}, Hidden: true, Targets: []Target{
			{Rect: [4]float64{457, 557, 689, 790}, Src: full, FlipY: true}}},
	},
}

// Playmat: one printed surface (table mesh top, also wrapped around the rolled shop item), the rubber underside (x 21–372 ×
// y 751–1004, same direction as the top: shown as if the mat were flipped towards you) and the rim (x 427–561 × y 774–910,
// all four edges squashed into it — colour only).
var playmat = Model{
	Kind: "Playmat", Mesh: "PlayMatFlatMesh", TextureSize: 1024, Icon: "tilt",
	Preview: mainPart("PlayMatFlatMesh"),
	Size:    [3]float64{4.614, 0.05, 2.825},
	Faces: []Face{
		{ID: "surface", Label: "Play surface", Net: [4]float64{0, 0, 4.614, 2.825}, Targets: []Target{
			// Bleed: the whole face stretched over the printed area incl. its margin, then the exact mapping on top.
			{Rect: [4]float64{0, 0, 1024, 604}, Src: full, Bleed: true},
			{Rect: [4]float64{28, 27, 996, 569}, Src: full}}},
		{ID: "bottom", Label: "Underside", Net: [4]float64{0, 2.925, 4.614, 2.825}, Targets: []Target{
			{Rect: [4]float64{19, 749, 374, 1006}, Src: full, FlipY: true}}},
		{ID: "edges", Label: "Rim", Net: [4]float64{0, 5.85, 0.6, 0.6}, Hidden: true, Targets: []Target{
			{Rect: [4]float64{425, 772, 563, 912}, Src: full}}},
	},
}

// Sleeve pack: a flat box of sleeves with a printed front (the logo art), a plain back and a hang tab above it (the same
// print on both sides of the tab). The tab's rect runs on over the crimp below it (y 862–906, the fold into the box), so
// the tab's art continues over it. The box edges share one strip (x 774–988, incl. the left edge's rows 988–1012), rotated
// in the texture — colour only.
var sleeve = Model{
	Kind: "Sleeve", Mesh: "ItemCardSleeveMesh", TextureSize: 1024, Icon: "tilt",
	Preview: mainPart("ItemCardSleeveMesh"),
	Size:    [3]float64{0.693, 1.088, 0.206},
	Faces: []Face{
		{ID: "top", Label: "Hang tab", Net: [4]float64{0, -0.387, 0.700, 0.331}, Targets: []Target{
			{Rect: [4]float64{10, 577, 701, 906}, Src: full},
			// The tab's thin card edges (its sides, top and the hang hole's wall) are small colour swatches: the colour at the
			// tab's left edge.
			{Rect: [4]float64{173, 940, 216, 982}, Src: edgeSwatch},
			{Rect: [4]float64{711, 830, 741, 860}, Src: edgeSwatch},
			{Rect: [4]float64{382, 428, 394, 454}, Src: edgeSwatch}}},
		{ID: "front", Label: "Front", Net: [4]float64{0, 0, 0.693, 1.088}, Targets: []Target{
			{Rect: [4]float64{394, 20, 748, 517}, Src: full}}},
		{ID: "back", Label: "Back", Net: [4]float64{0.793, 0, 0.693, 1.088}, Targets: []Target{
			{Rect: [4]float64{12, 20, 367, 517}, Src: full}}},
		{ID: "edges", Label: "Edges", Net: [4]float64{1.586, 0, 0.206, 1.088}, Hidden: true, Targets: []Target{
			{Rect: [4]float64{774, 11, 989, 1012}, Src: full}}},
	},
}

// Comic: one cover wrap back | spine | front (y 19–735), upright. The lower part of the texture (x 30–980 × y 753–1001) is the
// page edges: one strip of page lines that the fore-edge, the top and the bottom all reuse (the book's height runs across it).
var comic = Model{
	Kind: "Comic", Mesh: "Manga_Mesh", TextureSize: 1024, Icon: "tilt",
	Preview: mainPart("Manga_Mesh"),
	Size:    [3]float64{1.226, 1.699, 0.227},
	Faces: []Face{
		{ID: "back", Label: "Back cover", Net: [4]float64{0, 0, 1.226, 1.699}, Targets: []Target{
			{Rect: [4]float64{15, 19, 450, 735}, Src: full}}},
		{ID: "left", Label: "Spine", Net: [4]float64{1.226, 0, 0.227, 1.699}, Targets: []Target{
			{Rect: [4]float64{450, 19, 524, 735}, Src: full}}},
		{ID: "front", Label: "Front cover", Net: [4]float64{1.453, 0, 1.226, 1.699}, Targets: []Target{
			{Rect: [4]float64{524, 19, 1003, 735}, Src: full}}},
		{ID: "pages", Label: "Page edges", Net: [4]float64{0, 1.799, 1.699, 0.443}, Targets: []Target{
			{Rect: [4]float64{29, 752, 981, 1002}, Src: full}}},
	},
}

// Collection book (binder): outer cover wrap back | spine | front (y 526–1008) and the inside of the covers (y 36–518, seen when
// opened). Rings, clips and the pages (own texture) are kept.
var binder = Model{
	Kind: "Binder", Mesh: "ItemBinderBook", TextureSize: 1024, Icon: "tilt",
	Preview: []PreviewPart{{Mesh: "ItemBinderBook", Texture: "main"}, {Mesh: "ItemBinderPage", Texture: "secondary"}},
	Size:    [3]float64{3.087, 2.889, 0.618},
	Faces: []Face{
		{ID: "back", Label: "Back cover", Net: [4]float64{0, 0, 3.087, 2.889}, Targets: []Target{
			{Rect: [4]float64{16, 526, 406, 1008}, Src: full}}},
		{ID: "left", Label: "Spine", Net: [4]float64{3.087, 0, 0.618, 2.889}, Targets: []Target{
			{Rect: [4]float64{406, 526, 481, 1008}, Src: full}}},
		{ID: "front", Label: "Front cover", Net: [4]float64{3.705, 0, 3.087, 2.889}, Targets: []Target{
			{Rect: [4]float64{481, 526, 871, 1008}, Src: full}}},
		{ID: "inside", Label: "Inside covers", Net: [4]float64{0, 3.089, 6.792, 2.889}, Hidden: true, Targets: []Target{
			{Rect: [4]float64{81, 36, 934, 518}, Src: full}}},
	},
}

// Dice box: the 128² texture is a palette — the dice faces use the big top area, the bottom row holds the glass frame, the box
// (top/bottom), the dice edges and the numbers. Net units = texture pixels / 100.
var dice = Model{
	Kind: "Dice", Mesh: "DiceBox", TextureSize: 128, Icon: "recolor", Palette: true,
	Preview: []PreviewPart{{Mesh: "DiceBox", Texture: "main"}, {Mesh: "DiceBoxGlass", Glass: true}},
	Size:    [3]float64{0.722, 1.002, 0.799},
	Faces: []Face{
		{ID: "dice", Label: "Dice", Net: [4]float64{0, 0, 1.28, 0.96}, Targets: []Target{{Rect: [4]float64{0, 0, 128, 96}, Src: full}}},
		{ID: "frame", Label: "Glass frame", Net: [4]float64{0, 1.06, 0.32, 0.32}, Targets: []Target{{Rect: [4]float64{0, 96, 32, 128}, Src: full}}},
		{ID: "box", Label: "Box", Net: [4]float64{0.42, 1.06, 0.32, 0.32}, Targets: []Target{{Rect: [4]float64{32, 96, 64, 128}, Src: full}}},
		{ID: "edges", Label: "Dice edges", Net: [4]float64{0.84, 1.06, 0.32, 0.32}, Targets: []Target{{Rect: [4]float64{64, 96, 96, 128}, Src: full}}},
		{ID: "numbers", Label: "Numbers", Net: [4]float64{1.26, 1.06, 0.32, 0.32}, Targets: []Target{{Rect: [4]float64{96, 96, 128, 128}, Src: full}}},
	},
}

// Battle deck (vanilla PreconDeck_*, mesh "PerconDeck_Mesh" — the game's spelling): a box with a big printed front, two sides,
// a lid, and a hang tab standing up from the back edge. Rects reach 1–2 px past their islands into unused texels (mipmaps). The back and the back of the tab are one panel lying on its side in
// the texture (x 25–392, y 765–998). The grey block around x 400–625 × 765–1010 is the inside (tray, card stack) — kept.
var battleDeck = Model{
	Kind: "BattleDeck", Mesh: "PerconDeck_Mesh", TextureSize: 1024, Icon: "box",
	Preview: mainPart("PerconDeck_Mesh"),
	Size:    [3]float64{1.055, 1.454, 0.471},
	Faces: []Face{
		{ID: "tab", Label: "Hang tab", Net: [4]float64{0.471, -0.738, 1.055, 0.267}, Targets: []Target{
			{Rect: [4]float64{636, 913, 1024, 1018}, Src: full, Bleed: true},
			{Rect: [4]float64{644, 921, 1016, 1010}, Src: full}}},
		{ID: "top", Label: "Lid", Net: [4]float64{0.471, -0.471, 1.055, 0.471}, Targets: []Target{
			{Rect: [4]float64{630, 754, 1014, 907}, Src: full}}},
		{ID: "left", Label: "Left side", Net: [4]float64{0, 0, 0.471, 1.454}, Targets: []Target{
			{Rect: [4]float64{16, 13, 242, 748}, Src: full},
			// The sloped rim on top of the walls around the opening, the lid's thin edge and its underside all read one small
			// swatch (x 417–448 × y 998–1021): the colour at the top of the left side.
			{Rect: [4]float64{416, 996, 449, 1022}, Src: [4]float64{0.2, 0.005, 0.8, 0.03}}}},
		{ID: "front", Label: "Front", Net: [4]float64{0.471, 0, 1.055, 1.454}, Targets: []Target{
			{Rect: [4]float64{242, 14, 784, 748}, Src: full}}},
		{ID: "right", Label: "Right side", Net: [4]float64{1.526, 0, 0.471, 1.454}, Targets: []Target{
			{Rect: [4]float64{784, 13, 1009, 748}, Src: full}}},
		// Back incl. the back of the tab (top 0.267): face top → texture left, face left → texture top.
		{ID: "back", Label: "Back", Net: [4]float64{1.997, -0.267, 1.055, 1.721}, Targets: []Target{
			{Rect: [4]float64{23, 763, 394, 1000}, Src: full, Transpose: true}}},
	},
}

// Card pack (mesh "CardPackStatic_Mesh", 1024² texture per pack type). The texture is one wrap: back (x 217–558) | edge strip
// (561–603, shared by both curved edges) | front (607–948), all y 114–720, with the ridged crimps above (y 41–108) and below
// (726–794) and the back's fin seam at x 80–172. The front is upright; the back is mirrored in the texture (u grows with +x on
// the side facing +z). The pack torn open in the opening sequence ("CardPack_Mesh", skinned) reads the same areas.
//
// Front and back reach on past what the meshes show, over the vanilla print beside them (front to x 996, back from x 192):
// no mesh shows it, but the opening pack's fold edge (x 209–218) sits next to the back's and texture filtering pulls it in, so
// vanilla blue showed along the edge (2026-10-17). Those parts are the faces' own outer strips (same scale as the face, no
// stretch); NetVersion 2 widened the two faces in the net and moved the faces after them.
const (
	packFrontShown = 341.0 / 389 // the part of the front face the meshes show (x 607–948 of 607–996)
	packBackShown  = 341.0 / 366 // the same for the back (x 558–217 of 558–192, mirrored)
)

var pack = Model{
	Kind: "Pack", Mesh: "CardPackStatic_Mesh", TextureSize: 1024, Icon: "pack",
	Preview: mainPart("CardPackStatic_Mesh"),
	Size:    [3]float64{0.678, 1.354, 0.148},
	NetMoves: []NetMove{ // 0.0954 / 0.0497 = 48 / 25 texture px at the faces' 503 px per unit
		{Version: 2, From: 0.678, DX: 0.0954}, // edges, back and seam after the wider front
		{Version: 2, From: 1.466, DX: 0.0497}, // the seam after the wider back
	},
	Faces: []Face{
		{ID: "crimpTop", Label: "Top crimp", Net: [4]float64{0, -0.127, 0.678, 0.127}, Targets: []Target{
			{Rect: [4]float64{588, 41, 967, 108}, Src: full},
			{Rect: [4]float64{199, 41, 577, 108}, Src: full, FlipX: true},
			// the fin seam's crimp end: the middle of the crimp
			{Rect: [4]float64{80, 41, 172, 108}, Src: [4]float64{0.4, 0, 0.6, 1}, FlipX: true}}},
		{ID: "front", Label: "Front", Net: [4]float64{0, 0, 0.7734, 1.082}, Shown: &[4]float64{0, 0, packFrontShown, 1}, Targets: []Target{
			{Rect: [4]float64{600, 108, 996, 724}, Src: full, Bleed: true},
			{Rect: [4]float64{948, 114, 996, 720}, Src: [4]float64{packFrontShown, 0, 1, 1}, Bleed: true}, // the outer strip (unused texels)
			{Rect: [4]float64{607, 114, 948, 720}, Src: [4]float64{0, 0, packFrontShown, 1}}}},
		{ID: "edges", Label: "Edges", Net: [4]float64{0.7734, 0, 0.11, 1.082}, Targets: []Target{
			{Rect: [4]float64{558, 104, 607, 730}, Src: full, Bleed: true},
			{Rect: [4]float64{561, 111, 603, 720}, Src: full}}},
		{ID: "back", Label: "Back", Net: [4]float64{0.8834, 0, 0.7277, 1.082}, Shown: &[4]float64{0, 0, packBackShown, 1}, Targets: []Target{
			{Rect: [4]float64{192, 108, 562, 724}, Src: full, FlipX: true, Bleed: true},
			{Rect: [4]float64{192, 114, 217, 720}, Src: [4]float64{packBackShown, 0, 1, 1}, FlipX: true, Bleed: true}, // the outer strip
			{Rect: [4]float64{217, 114, 558, 720}, Src: [4]float64{0, 0, packBackShown, 1}, FlipX: true}}},
		{ID: "seam", Label: "Back seam", Net: [4]float64{1.7111, 0, 0.125, 1.082}, Hidden: true, Targets: []Target{
			{Rect: [4]float64{80, 114, 172, 713}, Src: full, FlipX: true}}},
		{ID: "crimpBottom", Label: "Bottom crimp", Net: [4]float64{0, 1.082, 0.678, 0.127}, Targets: []Target{
			{Rect: [4]float64{588, 726, 967, 794}, Src: full},
			{Rect: [4]float64{199, 726, 577, 794}, Src: full, FlipX: true},
			{Rect: [4]float64{80, 723, 172, 791}, Src: [4]float64{0.4, 0, 0.6, 1}, FlipX: true}}},
	},
	Bases: []Base{
		{ID: "BasicCardPack", Label: "Basic pack", Texture: "BasicCardPack_texture.png"},
		{ID: "RareCardPack", Label: "Rare pack", Texture: "RareCardPack_texture.png"},
		{ID: "EpicCardPack", Label: "Epic pack", Texture: "EpicCardPack_texture.png"},
		{ID: "LegendaryCardPack", Label: "Legendary pack", Texture: "LegendaryCardPack_texture.png"},
		{ID: "DestinyBasicCardPack", Label: "Destiny basic pack", Texture: "DestinyBasicCardPack_texture.png"},
		{ID: "DestinyRareCardPack", Label: "Destiny rare pack", Texture: "DestinyRareCardPack_texture.png"},
		{ID: "DestinyEpicCardPack", Label: "Destiny epic pack", Texture: "DestinyEpicCardPack_texture.png"},
		{ID: "DestinyLegendaryCardPack", Label: "Destiny legendary pack", Texture: "DestinyLegendaryCardPack_texture.png"},
		{ID: "GhostPack", Label: "Ghost / Megabot / other pack", Texture: "GhostPack_texture.png"},
		{ID: "AscensionCardPack", Label: "Ascension pack", Texture: "AscensionCardPack_texture.png"},
	},
}

// Card box (mesh "CardBoxMesh" — custom boxes clone the Basic box). A plain cuboid whose opposite faces share a texture area:
// front & back (x 202–561, y 153–374), top & bottom (201–510 × 14–143), left & right (11–171 × 18–266), all upright. The
// vanilla box texture is an atlas of four boxes (T_CardBox; Destiny boxes T_CardBox_Destiny, same layout): Rare/Epic/Legendary
// boxes use the meshes CardBoxMesh_2/_3/_4 with the areas in their Base targets (the Legendary side lies on its side).
var box = Model{
	Kind: "Box", Mesh: "CardBoxMesh", TextureSize: 1024, Icon: "box",
	Preview: mainPart("CardBoxMesh"),
	Size:    [3]float64{1.547, 1.123, 0.929},
	Faces: []Face{
		{ID: "top", Label: "Top & bottom", Net: [4]float64{0, -0.929, 1.547, 0.929}, Targets: []Target{
			{Rect: [4]float64{196, 9, 515, 148}, Src: full, Bleed: true},
			{Rect: [4]float64{201, 14, 510, 143}, Src: full}}},
		{ID: "front", Label: "Front & back", Net: [4]float64{0, 0, 1.547, 1.123}, Targets: []Target{
			{Rect: [4]float64{197, 148, 566, 379}, Src: full, Bleed: true},
			{Rect: [4]float64{202, 153, 561, 374}, Src: full}}},
		{ID: "right", Label: "Sides", Net: [4]float64{1.547, 0, 0.929, 1.123}, Targets: []Target{
			{Rect: [4]float64{6, 13, 176, 271}, Src: full, Bleed: true},
			{Rect: [4]float64{11, 18, 171, 266}, Src: full}}},
	},
	Bases: boxBases(),
}

func boxBases() []Base {
	type area struct{ front, top, side [4]float64 }
	areas := []struct {
		id, label string
		a         *area
		sideT     bool
	}{
		{"Basic", "basic", nil, false},
		{"Rare", "rare", &area{[4]float64{630, 153, 988, 374}, [4]float64{628, 14, 937, 143}, [4]float64{7, 332, 167, 581}}, false},
		{"Epic", "epic", &area{[4]float64{221, 577, 578, 797}, [4]float64{220, 438, 529, 567}, [4]float64{7, 670, 167, 918}}, false},
		{"Legendary", "legendary", &area{[4]float64{642, 568, 1001, 788}, [4]float64{641, 428, 951, 557}, [4]float64{682, 829, 931, 989}}, true},
	}
	var out []Base
	for _, line := range []struct{ prefix, label string }{{"", "Box"}, {"Destiny", "Destiny box"}} {
		for _, a := range areas {
			id := line.prefix + a.id + "CardBox"
			b := Base{ID: id, Label: line.label + " (" + a.label + ")", Texture: id + "_texture.png"}
			if a.a != nil {
				b.Targets = map[string][]Target{
					"front": {{Rect: a.a.front, Src: full}},
					"top":   {{Rect: a.a.top, Src: full}},
					// Legendary: face x (front → back) runs down the texture, face top → texture right.
					"right": {{Rect: a.a.side, Src: full, Transpose: a.sideT, FlipX: a.sideT}},
				}
			}
			out = append(out, b)
		}
	}
	return out
}

// ModelFor returns the editable layout for an accessory kind (see setfmt.AccessoryKinds).
func ModelFor(kind string) (Model, bool) {
	switch kind {
	case "Deckbox":
		return deckbox, true
	case "Playmat":
		return playmat, true
	case "Sleeve":
		return sleeve, true
	case "Comic":
		return comic, true
	case "Binder":
		return binder, true
	case "Dice":
		return dice, true
	case "BattleDeck":
		return battleDeck, true
	case "Pack":
		return pack, true
	case "Box":
		return box, true
	}
	return Model{}, false
}

// Package uvmap holds the layouts of every painted model: where each face lies on its texture (face maps), and the game
// layouts older art is converted from.
package uvmap

import "sync"

// Face maps: a model's faces and where each one lands on the UV texture, written out as plain numbers, one row per piece
// (facemap_*.go). They are the current game's layouts; the hand-measured ones of game 1.02 in models.go and tabletop.go stay only
// so older art can be converted from them. The numbers are used exactly as written: Exact switches off everything that grows or
// adds areas (faceArea in the editor).
//
// To edit one: change a number, rebuild, re-save the item in the editor. Settings → Developer → Layout tuner does it by dragging
// and copies the rows. Rows:
//
//	Face   which face of the editor's art the piece shows; a name the model's Faces don't have makes a new face with that name
//	Part   its name in the editor's guides
//	Rect   x0, y0, x1, y1 in texture pixels, top-left origin: the art is stretched over this rectangle
//	Show   the part of the face it shows: left, top, right, bottom as 0..1 (omit = all of it)
//	Flip   "x", "y" or "xy": the piece lies mirrored or upside down in the texture
//	Turn   true: the piece lies on its side in the texture (the face's width runs along the texture's y)
//	Shape  the piece's real outline when it isn't a plain rectangle: only inside it is painted (omit = all of Rect)

// Row is one piece of a face.
type Row struct {
	Face, Part string
	Rect, Show [4]float64
	Flip       string
	Turn       bool
	Shape      [][2]float64
}

// faceMaps: every face map, by kind, by base for the kinds with one model per item (board games, card supplies), and by look
// id for the Game look boxes.
var faceMaps = sync.OnceValue(func() map[string]Model {
	return map[string]Model{
		"Pack":                            faceMapPack(),
		"Box":                             faceMapBox(),
		"Sleeve":                          faceMapSleeve(),
		"BattleDeck":                      faceMapBattleDeck(),
		"Binder":                          faceMapBinder(),
		"Dice":                            faceMapDice(),
		"Playmat":                         faceMapPlaymat(),
		"Comic":                           faceMapComic(),
		"Boardgame_Speedrobo_Claim":       faceMapBoardgameSpeedroboClaim(),
		"Boardgame_Speedrobo_Mafia":       faceMapBoardgameSpeedroboMafia(),
		"Boardgame_Speedrobo_Necro":       faceMapBoardgameSpeedroboNecro(),
		"Boardgame_Speedrobo_SystemGate1": faceMapBoardgameSpeedroboSystemGate1(),
		"Boardgame_Speedrobo_SystemGate2": faceMapBoardgameSpeedroboSystemGate2(),
		"UP_PennySleeves":                 faceMapUPPennySleeves(),
		"UP_TowerDeckbox":                 faceMapUPTowerDeckbox(),
		"UP_MagneticHolder":               faceMapUPMagneticHolder(),
		"UP_Toploader":                    faceMapUPToploader(),
		"UP_CardPreserver":                faceMapUPCardPreserver(),
		"UP_Playmat1":                     faceMapUPPlaymat1(),
		"UP_PlatinumSeriesPocketPages":    faceMapUPPlatinumSeriesPocketPages(),
		"UP_SemiRigidCardHolder":          faceMapUPSemiRigidCardHolder(),
		"UP_Album":                        faceMapUPAlbum(),
		"restockBig":                      faceMapLookRestockBig(),
		"restockSmall":                    faceMapLookRestockSmall(),
		"furnitureBox":                    faceMapLookFurnitureBox(),
		"gradedBox":                       faceMapLookGradedBox(),
		"bulk":                            faceMapLookBulk(),
		"bag":                             faceMapLookBag(),
		"streetBillboard":                 faceMapLookStreetBillboard(),
		"Deckbox":                         faceMapDeckbox(),
	}
})

// faceMapOf is the face map of a kind, base or look id.
func faceMapOf(key string) (Model, bool) {
	m, ok := faceMaps()[key]
	return m, ok
}

// NetPerPixel is how big a new face is in the editor's net, per texture pixel of its first piece, in a model with no other face
// to take the scale from (about the deck box's scale); the tuner uses the same (lib/accessoryArt.ts newFaceNet).
const NetPerPixel = 0.0046

// netPerPixel: a model's net units per texture pixel, from its largest piece, so a new face matches the faces around it.
func netPerPixel(faces []Face) float64 {
	best, area := NetPerPixel, 0.0
	for _, f := range faces {
		for _, t := range f.Targets {
			w, h := t.Rect[2]-t.Rect[0], t.Rect[3]-t.Rect[1]
			if w*h > area && w > 0 && h > 0 {
				if t.Transpose {
					w = h
				}
				best, area = f.Net[2]*(t.Src[2]-t.Src[0])/w, (t.Rect[2]-t.Rect[0])*(t.Rect[3]-t.Rect[1])
			}
		}
	}
	return best
}

// faceMap builds a model from its rows; m gives everything else, and the faces' labels and net places. A row naming a face m
// doesn't have makes a new face (the name is its label), placed below the others in the net. Faces without a row are left out.
func faceMap(m Model, rows []Row) Model {
	target := func(r Row) Target {
		t := Target{Rect: r.Rect, Src: r.Show, Part: r.Part, Shape: r.Shape, FlipX: r.Flip == "x" || r.Flip == "xy", FlipY: r.Flip == "y" || r.Flip == "xy", Transpose: r.Turn}
		if t.Src == ([4]float64{}) {
			t.Src = full
		}
		return t
	}
	faces := []Face{}
	at := map[string]int{}
	bottom := 0.0
	for _, ff := range m.Faces {
		f := ff
		f.Targets = nil
		for _, r := range rows {
			if r.Face == f.ID {
				f.Targets = append(f.Targets, target(r))
			}
		}
		if len(f.Targets) > 0 {
			at[f.ID] = len(faces)
			faces = append(faces, f)
			bottom = max(bottom, f.Net[1]+f.Net[3])
		}
	}
	known := map[string]bool{}
	for _, ff := range m.Faces {
		known[ff.ID] = true
	}
	scale := netPerPixel(faces)
	for _, r := range rows {
		if known[r.Face] {
			continue
		}
		if i, ok := at[r.Face]; ok {
			faces[i].Targets = append(faces[i].Targets, target(r))
			continue
		}
		w, h := (r.Rect[2]-r.Rect[0])*scale, (r.Rect[3]-r.Rect[1])*scale
		if r.Turn {
			w, h = h, w
		}
		at[r.Face] = len(faces)
		faces = append(faces, Face{ID: r.Face, Label: r.Face, Net: [4]float64{0, bottom + 0.1, w, h}, Targets: []Target{target(r)}})
		bottom += 0.1 + h
	}
	m.Faces, m.Exact = faces, true
	return m
}

// sliceBases: the same bases, each now its own texture in the model's layout (game 1.03 texture arrays: one slice per item).
func sliceBases(legacy []Base) []Base {
	out := make([]Base, len(legacy))
	for i, b := range legacy {
		out[i] = Base{ID: b.ID, Label: b.Label, Texture: b.Texture}
	}
	return out
}

// Paints reports whether texture point x, y belongs to the piece: inside its rectangle and, when it has one, its outline.
func (t Target) Paints(x, y float64) bool {
	if x < t.Rect[0] || x >= t.Rect[2] || y < t.Rect[1] || y >= t.Rect[3] {
		return false
	}
	if len(t.Shape) == 0 {
		return true
	}
	in := false
	for i, j := 0, len(t.Shape)-1; i < len(t.Shape); j, i = i, i+1 {
		a, b := t.Shape[i], t.Shape[j]
		if (a[1] > y) != (b[1] > y) && x < a[0]+(y-a[1])*(b[0]-a[0])/(b[1]-a[1]) {
			in = !in
		}
	}
	return in
}

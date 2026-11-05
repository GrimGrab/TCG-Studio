package relayout

import (
	"crypto/rand"
	"encoding/json"
	"math"

	"tcgstudio/internal/uvmap"
)

// A design (frontend lib/accessoryArt.ts Layout) keeps layers either on the faces (net units: they follow any layout as
// they are) or on the texture map ("space": "texture", pixels of the layout they were made on). Map layers are moved here:
// each piece of a layer on an old face area becomes a layer on each new area of that face, mapped like the face, clipped to
// it. Layers are kept as JSON objects so fields this package doesn't know survive.

type layer = map[string]any

// Layout moves a design's map layers from layout from to layout to (a raw Layout; other fields unchanged). Converting the
// picture files it names (baseFile, textureFile) is the caller's job.
func Layout(from, to uvmap.Model, raw json.RawMessage) (json.RawMessage, error) {
	var l map[string]json.RawMessage
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, err
	}
	var layers []layer
	if v, ok := l["layers"]; ok {
		if err := json.Unmarshal(v, &layers); err != nil {
			return nil, err
		}
	}
	gone := map[string]bool{} // faces the new layout no longer has (the dice palette's swatches)
	for _, f := range from.Faces {
		gone[f.ID] = true
	}
	for _, f := range to.Faces {
		delete(gone, f.ID)
	}
	out := []layer{} // never null: the editor reads layout.layers as a list
	for _, ly := range layers {
		if ly["space"] != "texture" {
			if face, _ := ly["face"].(string); gone[face] {
				continue
			}
			out = append(out, ly)
			continue
		}
		out = append(out, movePieces(from, to, ly)...)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	l["layers"] = b
	return json.Marshal(l)
}

// affine maps texture points: p' = (a·x + b·y + c, d·x + e·y + f).
type affine [6]float64

func (m affine) at(x, y float64) (float64, float64) {
	return m[0]*x + m[1]*y + m[2], m[3]*x + m[4]*y + m[5]
}

func (m affine) vec(x, y float64) (float64, float64) { return m[0]*x + m[1]*y, m[3]*x + m[4]*y }

// through is the mapping of old target from (a face area of the old layout) onto new target to of the same face.
func through(f uvmap.Face, from, to uvmap.Target) affine {
	p := func(x, y float64) (float64, float64) {
		s, r := faceAt(from, f, x, y)
		return targetPixel(to, s, r)
	}
	ox, oy := p(0, 0)
	xx, xy := p(1, 0)
	yx, yy := p(0, 1)
	return affine{xx - ox, yx - ox, ox, xy - oy, yy - oy, oy}
}

// targetPixel: the texture pixel of target t showing face point (s, r) (no clamping: points past the area map past it).
func targetPixel(t uvmap.Target, s, r float64) (float64, float64) {
	a := (s - t.Src[0]) / (t.Src[2] - t.Src[0])
	b := (r - t.Src[1]) / (t.Src[3] - t.Src[1])
	if t.Transpose {
		a, b = b, a
	}
	if t.FlipX {
		a = 1 - a
	}
	if t.FlipY {
		b = 1 - b
	}
	return t.Rect[0] + a*(t.Rect[2]-t.Rect[0]), t.Rect[1] + b*(t.Rect[3]-t.Rect[1])
}

// movePieces: a map layer's pieces on the new layout (none when it touches no face area both layouts have).
func movePieces(from, to uvmap.Model, ly layer) []layer {
	num := func(k string) float64 { v, _ := ly[k].(float64); return v }
	x, y, w, h, rot, skew := num("x"), num("y"), num("w"), num("h"), num("rot"), num("skew")
	mirror, _ := ly["mirror"].(bool)
	// The layer's frame: point = c + A·(lx, ly) for lx, ly in ±w/2, ±h/2 (drawMapLayer: rotate, mirror, skew).
	cs, sn := math.Cos(rot*math.Pi/180), math.Sin(rot*math.Pi/180)
	m := 1.0
	if mirror {
		m = -1
	}
	A := [4]float64{cs * m, -sn, sn * m, cs}                       // R·Mirror
	A = [4]float64{A[0], A[0]*skew + A[1], A[2], A[2]*skew + A[3]} // ·Skew
	corners := [][2]float64{}
	for _, c := range [][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
		lx, lyv := c[0]*w/2, c[1]*h/2
		corners = append(corners, [2]float64{x + A[0]*lx + A[1]*lyv, y + A[2]*lx + A[3]*lyv})
	}
	box := bounds(corners)
	if c, ok := clipOf(ly); ok {
		box = intersect(box, c)
	}
	type piece struct {
		l     layer
		label string
	}
	var pieces []piece
	for _, of := range from.Faces {
		for _, nf := range to.Faces {
			if nf.ID != of.ID && nf.ArtFrom != of.ID { // a face split off since gets the pieces of the one it shared a print with
				continue
			}
			for _, ot := range of.Targets {
				if ot.Bleed || empty(intersect(box, ot.Rect)) {
					continue
				}
				for _, nt := range nf.Targets {
					if nt.Bleed {
						continue
					}
					M := through(of, ot, nt)
					clip := mapRect(M, intersect(box, ot.Rect))
					pieces = append(pieces, piece{transformed(ly, M, A, w, h), nf.Label}) // label for names
					pieces[len(pieces)-1].l["clip"] = []float64{clip[0], clip[1], clip[2], clip[3]}
					if len(nt.Shape) > 0 {
						pieces[len(pieces)-1].l["shape"] = nt.Shape
					}
				}
			}
		}
	}
	out := make([]layer, len(pieces))
	for i, p := range pieces {
		if len(pieces) > 1 {
			p.l["id"] = newID()
			if name, _ := ly["name"].(string); name != "" && p.label != "" {
				p.l["name"] = name + " – " + p.label
			}
		}
		out[i] = p.l
	}
	return out
}

// transformed: a copy of layer ly drawn through M (frame A, size w×h): new centre, size, rotation, mirror and skew.
func transformed(ly layer, M affine, A [4]float64, w, h float64) layer {
	n := layer{}
	for k, v := range ly {
		n[k] = v
	}
	x, y := ly["x"].(float64), ly["y"].(float64)
	cx, cy := M.at(x, y)
	ux, uy := M.vec(A[0]*w, A[2]*w) // the layer's full x axis
	vx, vy := M.vec(A[1]*h, A[3]*h) // and y axis
	mirror := ux*vy-uy*vx < 0
	m := 1.0
	if mirror {
		m = -1
	}
	rr := math.Atan2(m*uy, m*ux)
	c, s := math.Cos(rr), math.Sin(rr)
	// V in the turned (and mirrored) frame: (skew·h', h').
	tx, ty := c*vx+s*vy, -s*vx+c*vy
	tx *= m
	n["x"], n["y"] = cx, cy
	n["w"], n["h"] = math.Hypot(ux, uy), ty
	n["rot"] = math.Round(rr*180/math.Pi*1e6) / 1e6
	if mirror {
		n["mirror"] = true
	} else {
		delete(n, "mirror")
	}
	if sk := math.Round(tx/ty*1e9) / 1e9; sk != 0 {
		n["skew"] = sk
	} else {
		delete(n, "skew")
	}
	return n
}

func clipOf(ly layer) ([4]float64, bool) {
	c, ok := ly["clip"].([]any)
	if !ok || len(c) != 4 {
		return [4]float64{}, false
	}
	var r [4]float64
	for i := range r {
		r[i], _ = c[i].(float64)
	}
	return r, true
}

func bounds(ps [][2]float64) [4]float64 {
	r := [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for _, p := range ps {
		r = [4]float64{math.Min(r[0], p[0]), math.Min(r[1], p[1]), math.Max(r[2], p[0]), math.Max(r[3], p[1])}
	}
	return r
}

func intersect(a, b [4]float64) [4]float64 {
	return [4]float64{math.Max(a[0], b[0]), math.Max(a[1], b[1]), math.Min(a[2], b[2]), math.Min(a[3], b[3])}
}

func empty(r [4]float64) bool { return r[2]-r[0] <= 1e-6 || r[3]-r[1] <= 1e-6 }

// mapRect: a rectangle through M (targets flip, turn and scale, so rectangles stay rectangles).
func mapRect(M affine, r [4]float64) [4]float64 {
	x0, y0 := M.at(r[0], r[1])
	x1, y1 := M.at(r[2], r[3])
	return [4]float64{math.Min(x0, x1), math.Min(y0, y1), math.Max(x0, x1), math.Max(y0, y1)}
}

// newID is a layer id like the editor's (10 lowercase letters/digits).
func newID() string {
	const abc = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 10)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = abc[int(b[i])%len(abc)]
	}
	return string(b)
}

package art

import (
	"image"
	"math"
	"sort"
)

// ---------------------------------------------------------------- booster display photos
// Product photos of booster boxes (TCGplayer) are usually open displays: the lid art standing up at the top, a row of packs,
// and the box's front panel at the bottom — head-on or at a 3/4 angle, on white. DetectDisplay finds the front panel and the
// lid as quads the editor straightens (lib/perspective.ts) into the box faces.

// Quad is four corners in image pixels: top-left, top-right, bottom-right, bottom-left (x, y each).
type Quad [8]float64

// DisplayFaces is what DetectDisplay found; Confident is false when a fallback guess was used for either face.
type DisplayFaces struct {
	Front     Quad `json:"front"`
	Lid       Quad `json:"lid"`
	Confident bool `json:"confident"`
}

// line is y = a·x + b over the columns x0..x1 (working-image pixels).
type line struct{ a, b, x0, x1 float64 }

func (l line) y(x float64) float64 { return l.a*x + l.b }

// DetectDisplay finds the front panel and the lid of an open booster display in a photo (white background allowed).
func DetectDisplay(img image.Image) DisplayFaces {
	g := newGray(img, 400)
	if g.w < 40 || g.h < 40 {
		return fallbackDisplay(img.Bounds(), g)
	}
	bottom, top := g.contours()

	// Bottom edge of the front panel: the longest straight run of the mask's bottom contour.
	base, ok := longestLine(bottom, float64(g.h)*0.015, 0.6)
	if !ok || base.x1-base.x0 < float64(g.w)*0.35 {
		return fallbackDisplay(img.Bounds(), g)
	}
	H := float64(g.h)
	// Panel top: going up from the bottom edge (8–55 % of the photo height; Pokémon displays have panels of 12–17 %), the first edge about as strong as the strongest —
	// the pack tops above it are often just as strong (measured: BLB display 541235 both peak at 1.1).
	confident := true
	pt, ok := firstStrong(g.edgeProfile(base, H*0.08, H*0.55), 0.9)
	if !ok || pt.score < 0.5 {
		confident = false
		pt = edgeScore{l: line{base.a, base.b - H*0.35, base.x0, base.x1}}
	}
	panelTop := pt.l

	// Lid top: a quarter of the panel's columns reach higher than this line (keeps most of a header bump; the corners
	// outside the lid are white and made transparent by the editor). Lid bottom: the lid/packs seam has no reliable edge
	// (the packs carry the same logos), so 72 % of the way down to the panel top — measured BLB 82 %, Lorcana TFC 66 %,
	// Dominaria 85 %.
	var tops []float64
	for x := int(base.x0); x <= int(base.x1); x++ {
		if top[x] >= 0 {
			tops = append(tops, float64(top[x])-panelTop.y(float64(x)))
		}
	}
	lidTopOff := -H * 0.6
	if len(tops) > 0 {
		sort.Float64s(tops)
		lidTopOff = tops[len(tops)/4]
	}
	if lidTopOff > -H*0.15 {
		confident, lidTopOff = false, -H*0.6
	}
	// The lid's columns: those reaching at least halfway from the panel top up to the lid top (a lid can be narrower than
	// the box front, e.g. the Dominaria display).
	lx0, lx1 := base.x1, base.x0
	for x := int(base.x0); x <= int(base.x1); x++ {
		if top[x] >= 0 && float64(top[x])-panelTop.y(float64(x)) < lidTopOff/2 {
			lx0, lx1 = math.Min(lx0, float64(x)), math.Max(lx1, float64(x))
		}
	}
	if lx1-lx0 < (base.x1-base.x0)*0.3 {
		lx0, lx1 = base.x0, base.x1
	}
	lidTop := line{panelTop.a, panelTop.b + lidTopOff, lx0, lx1}
	lidBottom := line{panelTop.a, panelTop.b + lidTopOff*0.28, lx0, lx1}
	// Better: the lid found on its own, by the jumps in the top outline where it stands up from the box rim (a 3/4 view puts
	// the lid far off the front panel's columns, e.g. the One Piece OP-02 display).
	if t, b, ok := findLid(top, H); ok {
		// A lid mostly above the front panel (front-facing display): its visible part often continues below the rims behind
		// the packs (Lorcana TFC: rims at the side flaps cut off the wordmark), so the 72 % rule is a floor. A lid off to the
		// side (strong 3/4 view, One Piece): the rim decides. (Dropping the floor didn't help the Pokémon lids — survey
		// 2026-10-03 — their errors come from the jumps picked.)
		over := math.Min(t.x1, base.x1) - math.Max(t.x0, base.x0)
		if over >= (t.x1-t.x0)*0.5 {
			f := line{lidBottom.a, lidBottom.b, t.x0, t.x1}
			y0, y1 := math.Max(b.y(t.x0), f.y(t.x0)), math.Max(b.y(t.x1), f.y(t.x1))
			a := (y1 - y0) / (t.x1 - t.x0)
			b = line{a, y0 - a*t.x0, t.x0, t.x1}
		}
		lidTop, lidBottom = t, b
	}

	k := g.scale
	quad := func(t, b line) Quad {
		x0, x1 := t.x0, t.x1
		return Quad{x0 * k, t.y(x0) * k, x1 * k, t.y(x1) * k, x1 * k, b.y(x1) * k, x0 * k, b.y(x0) * k}
	}
	off := img.Bounds().Min
	f := DisplayFaces{Front: quad(panelTop, base), Lid: quad(lidTop, lidBottom), Confident: confident}
	f.Front.shift(off)
	f.Lid.shift(off)
	f.Front.clamp(img.Bounds())
	f.Lid.clamp(img.Bounds())
	return f
}

// findLid finds a display's lid from the top outline (per column, -1 = empty): its sides are the biggest jumps up and
// down (≥ 12 % of the height), the rim beside a jump is where the lid's visible part ends, and its top edge is the longest
// straight run of the outline between the sides. One side may be the photo's edge (lid reaching to the box side). ok is
// false without a clear jump (closed boxes, head-on shots without side flaps).
func findLid(top []int, H float64) (lidTop, lidBottom line, ok bool) {
	n := len(top)
	w := max(2, n/100)
	minJump := H * 0.12
	xl, xr, upL, downR := -1, -1, 0.0, 0.0
	for x := w; x < n; x++ {
		if top[x] < 0 || top[x-w] < 0 {
			continue
		}
		if d := float64(top[x-w] - top[x]); d > upL { // outline rises going right: the lid's left side
			upL, xl = d, x
		}
		if d := float64(top[x] - top[x-w]); d > downR { // outline falls: the lid's right side
			downR, xr = d, x-w
		}
	}
	first, last := -1, -1
	for x, t := range top {
		if t >= 0 {
			if first < 0 {
				first = x
			}
			last = x
		}
	}
	hasL, hasR := upL >= minJump, downR >= minJump
	if !hasL && !hasR {
		return line{}, line{}, false
	}
	x0, x1 := float64(first), float64(last)
	if hasL {
		x0 = float64(xl)
	}
	if hasR {
		x1 = float64(xr)
	}
	if x1-x0 < float64(n)*0.25 {
		return line{}, line{}, false
	}
	// Top edge: the longest straight run of the outline between the sides.
	c := make([]int, n)
	for x := range c {
		c[x] = -1
		if float64(x) >= x0 && float64(x) <= x1 {
			c[x] = top[x]
		}
	}
	t, found := longestLine(c, H*0.015, 0.6)
	if !found {
		return line{}, line{}, false
	}
	// A quarter of the lid's columns reach higher than its top line (a header bump keeps most of its art).
	var offs []float64
	for x := int(x0); x <= int(x1); x++ {
		if top[x] >= 0 {
			offs = append(offs, float64(top[x])-t.y(float64(x)))
		}
	}
	sort.Float64s(offs)
	t = line{t.a, t.b + math.Min(0, offs[len(offs)/4]), x0, x1}
	// Bottom edge: through the lower rim beside a jump (the outline just outside the lid), with the top's slope. The higher
	// rim is usually a side wall's top, further back (Dominaria display: right rim 65 px above the front one).
	rim := func(x int) (float64, bool) {
		if x >= 0 && x < n && top[x] >= 0 {
			return float64(top[x]), true
		}
		return 0, false
	}
	rl, okL := rim(int(x0) - w - 1)
	rr, okR := rim(int(x1) + w + 1)
	okL, okR = okL && hasL, okR && hasR
	var b line
	switch {
	case okL && (!okR || rl >= rr):
		b = line{t.a, rl - t.a*x0, x0, x1}
	case okR:
		b = line{t.a, rr - t.a*x1, x0, x1}
	default:
		return line{}, line{}, false
	}
	if b.y(x0)-t.y(x0) < H*0.15 || b.y(x1)-t.y(x1) < H*0.15 {
		return line{}, line{}, false
	}
	return t, b, true
}

func (q *Quad) shift(p image.Point) {
	for i := 0; i < 8; i += 2 {
		q[i] += float64(p.X)
		q[i+1] += float64(p.Y)
	}
}

func (q *Quad) clamp(r image.Rectangle) {
	for i := 0; i < 8; i += 2 {
		q[i] = math.Max(float64(r.Min.X), math.Min(float64(r.Max.X), q[i]))
		q[i+1] = math.Max(float64(r.Min.Y), math.Min(float64(r.Max.Y), q[i+1]))
	}
}

// fallbackDisplay guesses from the photo's shape: the bottom 35 % is the front panel, the top 45 % the lid.
func fallbackDisplay(r image.Rectangle, g *gray) DisplayFaces {
	b := r
	if g != nil && g.w > 0 {
		if m := g.maskBounds(); !m.Empty() {
			b = image.Rect(r.Min.X+int(float64(m.Min.X)*g.scale), r.Min.Y+int(float64(m.Min.Y)*g.scale),
				r.Min.X+int(float64(m.Max.X)*g.scale), r.Min.Y+int(float64(m.Max.Y)*g.scale))
		}
	}
	x0, x1, y0, y1 := float64(b.Min.X), float64(b.Max.X), float64(b.Min.Y), float64(b.Max.Y)
	h := y1 - y0
	return DisplayFaces{
		Front: Quad{x0, y1 - h*0.35, x1, y1 - h*0.35, x1, y1, x0, y1},
		Lid:   Quad{x0, y0, x1, y0, x1, y0 + h*0.45, x0, y0 + h*0.45},
	}
}

// ---------------------------------------------------------------- working image

type gray struct {
	w, h  int
	scale float64   // source pixels per working pixel
	lum   []float64 // 0..1
	mask  []bool    // part of the product (not white background)
}

func newGray(img image.Image, width int) *gray {
	b := img.Bounds()
	if b.Dx() < width {
		width = b.Dx()
	}
	if width <= 0 {
		return &gray{}
	}
	scale := float64(b.Dx()) / float64(width)
	g := &gray{w: width, h: int(float64(b.Dy()) / scale), scale: scale}
	g.lum = make([]float64, g.w*g.h)
	g.mask = make([]bool, g.w*g.h)
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			// Box average of the source cell.
			sx0, sy0 := b.Min.X+int(float64(x)*scale), b.Min.Y+int(float64(y)*scale)
			sx1, sy1 := max(sx0+1, b.Min.X+int(float64(x+1)*scale)), max(sy0+1, b.Min.Y+int(float64(y+1)*scale))
			var r, gg, bb, a, n float64
			for yy := sy0; yy < sy1; yy++ {
				for xx := sx0; xx < sx1; xx++ {
					cr, cg, cb, ca := img.At(xx, yy).RGBA()
					r, gg, bb, a, n = r+float64(cr), gg+float64(cg), bb+float64(cb), a+float64(ca), n+1
				}
			}
			r, gg, bb, a = r/n/65535, gg/n/65535, bb/n/65535, a/n/65535
			i := y*g.w + x
			g.lum[i] = 0.299*r + 0.587*gg + 0.114*bb
			g.mask[i] = a > 0.5 && !(r > 0.93 && gg > 0.93 && bb > 0.93)
		}
	}
	return g
}

func (g *gray) at(x, y float64) float64 {
	xi, yi := int(x), int(y)
	if xi < 0 || yi < 0 || xi >= g.w || yi >= g.h {
		return 1
	}
	return g.lum[yi*g.w+xi]
}

func (g *gray) in(x, y float64) bool {
	xi, yi := int(x), int(y)
	return xi >= 0 && yi >= 0 && xi < g.w && yi < g.h && g.mask[yi*g.w+xi]
}

// contours returns per column the lowest and highest product pixel (-1 = empty column). Specks (runs shorter than 2 % of
// the height) are ignored.
func (g *gray) contours() (bottom, top []int) {
	bottom, top = make([]int, g.w), make([]int, g.w)
	minRun := max(2, g.h/50)
	for x := 0; x < g.w; x++ {
		bottom[x], top[x] = -1, -1
		run := 0
		for y := 0; y < g.h; y++ {
			if g.mask[y*g.w+x] {
				if run++; run == minRun && top[x] < 0 {
					top[x] = y - minRun + 1
				}
			} else {
				run = 0
			}
		}
		run = 0
		for y := g.h - 1; y >= 0; y-- {
			if g.mask[y*g.w+x] {
				if run++; run == minRun {
					bottom[x] = y + minRun - 1
					break
				}
			} else {
				run = 0
			}
		}
	}
	return bottom, top
}

func (g *gray) maskBounds() image.Rectangle {
	r := image.Rectangle{}
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			if g.mask[y*g.w+x] {
				r = r.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return r
}

// longestLine fits the longest straight run of a contour (y per column, -1 = none): the candidate lines through pairs of
// points, scored by the longest stretch of columns within tol (gaps of up to 3 % of the width allowed), |slope| ≤ maxSlope.
func longestLine(c []int, tol, maxSlope float64) (line, bool) {
	n := len(c)
	var pts []int
	for x, y := range c {
		if y >= 0 {
			pts = append(pts, x)
		}
	}
	if len(pts) < n/4 {
		return line{}, false
	}
	gap := max(3, n*3/100)
	best, bestLen := line{}, 0.0
	step := max(1, len(pts)/60)
	for i := 0; i < len(pts); i += step {
		for j := i + step; j < len(pts); j += step {
			x1, x2 := pts[i], pts[j]
			if x2-x1 < n/10 {
				continue
			}
			a := float64(c[x2]-c[x1]) / float64(x2-x1)
			if math.Abs(a) > maxSlope {
				continue
			}
			b := float64(c[x1]) - a*float64(x1)
			// Longest run of inliers containing both points.
			last, runStart := -gap-1, -1
			bs, be := -1, -1
			for x := 0; x < n; x++ {
				if c[x] < 0 || math.Abs(float64(c[x])-(a*float64(x)+b)) > tol {
					continue
				}
				if x-last > gap {
					runStart = x
				}
				last = x
				if x-runStart > be-bs {
					bs, be = runStart, x
				}
			}
			if l := float64(be - bs); l > bestLen {
				bestLen, best = l, line{a, b, float64(bs), float64(be)}
			}
		}
	}
	if bestLen <= 0 {
		return line{}, false
	}
	// Least-squares refit over the run's inliers.
	var sx, sy, sxx, sxy, m float64
	for x := int(best.x0); x <= int(best.x1); x++ {
		if c[x] >= 0 && math.Abs(float64(c[x])-best.y(float64(x))) <= tol {
			fx, fy := float64(x), float64(c[x])
			sx, sy, sxx, sxy, m = sx+fx, sy+fy, sxx+fx*fx, sxy+fx*fy, m+1
		}
	}
	if d := m*sxx - sx*sx; m > 2 && d != 0 {
		best.a = (m*sxy - sx*sy) / d
		best.b = (sy - best.a*sx) / m
	}
	return best, true
}

// edgeScore is one candidate line of an edge search with its offset above the reference (working pixels).
type edgeScore struct {
	l     line
	d     float64
	score float64
}

// edgeProfile scores lines above ref (offsets lo..hi working pixels up, slope ref.a ± 0.06; the best slope per offset):
// the share of the columns (over ref's span, inside the product) where brightness changes clearly across the line, plus a
// little of the mean change (0..~1.2).
func (g *gray) edgeProfile(ref line, lo, hi float64) []edgeScore {
	var out []edgeScore
	mid := (ref.x0 + ref.x1) / 2
	for d := lo; d <= hi; d++ {
		best := edgeScore{score: -1}
		for da := -0.06; da <= 0.0601; da += 0.02 {
			a := ref.a + da
			// Rotate around the middle column so the offset stays the offset there.
			l := line{a, ref.y(mid) - d - a*mid, ref.x0, ref.x1}
			hits, sum, n := 0.0, 0.0, 0.0
			for x := ref.x0 + 2; x <= ref.x1-2; x += 2 {
				y := l.y(x)
				if !g.in(x, y) {
					continue
				}
				diff := math.Abs(g.at(x, y-2) - g.at(x, y+2))
				if diff > 0.08 {
					hits++
				}
				sum += diff
				n++
			}
			if n < (ref.x1-ref.x0)/4 {
				continue
			}
			if s := hits/n + sum/n; s > best.score {
				best = edgeScore{l, d, s}
			}
		}
		if best.score >= 0 {
			out = append(out, best)
		}
	}
	return out
}

// firstStrong picks, going up from the reference, the first local peak scoring at least frac of the best one (the panel's
// top edge comes before the brighter pack tops above it). ok is false when nothing was scored.
func firstStrong(p []edgeScore, frac float64) (edgeScore, bool) {
	best := -1.0
	for _, e := range p {
		best = math.Max(best, e.score)
	}
	for i, e := range p {
		if e.score < best*frac {
			continue
		}
		if i+1 < len(p) && p[i+1].score > e.score {
			continue // still climbing to the peak
		}
		return e, true
	}
	return edgeScore{}, false
}

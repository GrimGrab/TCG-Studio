package packmesh

import (
	"fmt"
	"math"
)

// FixBack gives the shelf pack's back one continuous texture strip. The game's model maps its two back halves to strips that
// overlap: the half that ends under the seam fin reads part of the texture the other half shows beside the fin, so art there
// showed twice (seen from an angle, under the slanted fin). Every vertex whose UV lies in the back's texture area (inBack) gets
// its u from its x, in one straight line from the back's left edge to its right edge, taken from the model's own outermost back
// vertices (where the back meets the edges, so the edges still join). v stays. The fin, edges, crimps and front keep their UVs.
// Returns the fixed copy and how many vertices changed.
func FixBack(shelf Mesh, inBack func(Vec2) bool) (Mesh, int, error) {
	var sel []int
	for i, uv := range shelf.UV {
		if inBack(uv) {
			sel = append(sel, i)
		}
	}
	if len(sel) < 4 {
		return shelf, 0, fmt.Errorf("the shelf pack has no back (%d vertices in its texture area)", len(sel))
	}
	// The outermost back vertices on each side: x and their u (averaged over the column).
	xL, xR := math.Inf(1), math.Inf(-1)
	for _, i := range sel {
		xL, xR = math.Min(xL, shelf.Pos[i][0]), math.Max(xR, shelf.Pos[i][0])
	}
	if xR-xL < 1e-6 {
		return shelf, 0, fmt.Errorf("the shelf pack's back has no width")
	}
	edgeU := func(x float64) float64 {
		sum, n := 0.0, 0
		for _, i := range sel {
			if math.Abs(shelf.Pos[i][0]-x) < 1e-4 {
				sum += shelf.UV[i][0]
				n++
			}
		}
		return sum / float64(n)
	}
	uL, uR := edgeU(xL), edgeU(xR)
	out := shelf
	out.UV = append([]Vec2(nil), shelf.UV...)
	changed := 0
	for _, i := range sel {
		u := uR + (xR-shelf.Pos[i][0])/(xR-xL)*(uL-uR)
		if math.Abs(u-out.UV[i][0]) > 1e-6 {
			out.UV[i][0] = u
			changed++
		}
	}
	return out, changed, nil
}

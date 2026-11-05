package uvmap

// migration:art-layouts-1.03
// The hand-measured layouts of game 1.02 (models.go, tabletop.go), only for converting art from before the 2026-10-27 update:
// that art was painted into these areas, grown by inclusive, so it has to be read back from the same ones.

// LegacyModelFor is a kind's layout of game 1.02.
func LegacyModelFor(kind string) (Model, bool) {
	m, ok := legacyModel(kind)
	return inclusive(m), ok
}

// LegacyModelForBase is ModelForBase's layout of game 1.02.
func LegacyModelForBase(kind, base string) (Model, bool) {
	if m, ok := tabletop[base]; ok && m.Kind == kind {
		return inclusive(m), true
	}
	return LegacyModelFor(kind)
}

func legacyModel(kind string) (Model, bool) {
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

// inclusive returns a copy of m whose target rects take in one more texel on every side: at a UV edge the GPU blends the
// texels on both sides of it (more at smaller mip levels), so a texel left unpainted next to an area showed as a 1 px seam
// of vanilla colour. A side stays where another target covers that texel (a neighbouring area keeps its own first/last
// column or row) or at the texture's edge. Palettes stay exact: their swatches touch. Bases (where vanilla art is read
// from) are unchanged.
func inclusive(m Model) Model {
	if m.Palette || len(m.Faces) == 0 {
		return m
	}
	var all [][4]float64
	for _, f := range m.Faces {
		for _, t := range f.Targets {
			all = append(all, t.Rect)
		}
	}
	// taken reports whether any other target covers part of the strip x0..x1 × y0..y1.
	taken := func(self [4]float64, x0, y0, x1, y1 float64) bool {
		for _, r := range all {
			if r != self && r[0] < x1 && r[2] > x0 && r[1] < y1 && r[3] > y0 {
				return true
			}
		}
		return false
	}
	size := float64(m.TextureSize)
	faces := make([]Face, len(m.Faces))
	for i, f := range m.Faces {
		f.Targets = append([]Target(nil), f.Targets...)
		for j, t := range f.Targets {
			r := t.Rect
			if r[0] > 0 && !taken(r, r[0]-1, r[1], r[0], r[3]) {
				t.Rect[0] = r[0] - 1
			}
			if r[1] > 0 && !taken(r, r[0], r[1]-1, r[2], r[1]) {
				t.Rect[1] = r[1] - 1
			}
			if r[2] < size && !taken(r, r[2], r[1], r[2]+1, r[3]) {
				t.Rect[2] = r[2] + 1
			}
			if r[3] < size && !taken(r, r[0], r[3], r[2], r[3]+1) {
				t.Rect[3] = r[3] + 1
			}
			f.Targets[j] = t
		}
		faces[i] = f
	}
	m.Faces = faces
	return m
}

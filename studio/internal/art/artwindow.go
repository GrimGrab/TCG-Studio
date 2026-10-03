package art

// ArtWindow is where the artwork sits on a card scan of an import source, as fractions of the image (x0, y0, x1, y1).
// Measured 2026-10-03 on sample scans (Dominaria + Alpha, Pokémon SV ex, Dark Magician, Lorcana TFC, OP-02, FaB WTR,
// SWU SOR leader). Hand-made sets use the whole image (their pictures are often art already).
func ArtWindow(source string, w, h int) [4]float64 {
	landscape := w > h
	switch source {
	case "scryfall":
		return [4]float64{0.085, 0.11, 0.915, 0.545}
	case "tcgdex":
		return [4]float64{0.08, 0.105, 0.92, 0.495}
	case "ygoprodeck":
		return [4]float64{0.12, 0.18, 0.88, 0.72}
	case "lorcast":
		return [4]float64{0, 0, 1, 0.5}
	case "optcg":
		return [4]float64{0.05, 0.05, 0.95, 0.62}
	case "fab":
		return [4]float64{0.08, 0.1, 0.92, 0.52}
	case "swudb":
		if landscape { // leaders and bases: art on the left, text on the right
			return [4]float64{0.03, 0.05, 0.45, 0.95}
		}
		return [4]float64{0.05, 0.05, 0.95, 0.55}
	}
	return [4]float64{0, 0, 1, 1}
}

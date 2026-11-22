package setfmt

import "fmt"

// EditionLabel is a set's or card's "editionLabel": the edition text (1st Edition, Silver, Gold, EX) the mod draws on
// full-image cards. Every field is optional: a card's left-out field is the set's, the set's is the default (shown, at
// X 0 / Y -24.882, size 1). X and Y move the label from the game's own spot in % of the card's width and height (+ = right,
// up). Mirrors the mod's EditionLabelDef.
type EditionLabel struct {
	Show *bool    `json:"show,omitempty"`
	X    *float64 `json:"x,omitempty"`
	Y    *float64 `json:"y,omitempty"`
	Size *float64 `json:"size,omitempty"`
}

// Ranges Studio offers: sideways ±50 %, up / down from just above the bottom edge (-40) to the top (65; user's range
// 2026-11-22), size 0.3–4.
const (
	EditionLabelMaxX    = 50.0
	EditionLabelMinY    = -40.0
	EditionLabelMaxY    = 65.0
	EditionLabelMinSize = 0.3
	EditionLabelMaxSize = 4.0
)

func (l *EditionLabel) problem() string {
	if l == nil {
		return ""
	}
	out := func(v *float64, lo, hi float64) bool { return v != nil && (*v < lo || *v > hi) }
	if out(l.X, -EditionLabelMaxX, EditionLabelMaxX) {
		return fmt.Sprintf("x must be between %g and %g", -EditionLabelMaxX, EditionLabelMaxX)
	}
	if out(l.Y, EditionLabelMinY, EditionLabelMaxY) {
		return fmt.Sprintf("y must be between %g and %g", EditionLabelMinY, EditionLabelMaxY)
	}
	if l.Size != nil && (*l.Size < EditionLabelMinSize || *l.Size > EditionLabelMaxSize) {
		return fmt.Sprintf("size must be between %g and %g", EditionLabelMinSize, EditionLabelMaxSize)
	}
	return ""
}

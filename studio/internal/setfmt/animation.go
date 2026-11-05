package setfmt

import (
	"fmt"
	"math"
	"strings"
)

// Customer animations (accessory library "animations", content kind "animation"; Customers → Animations): keyframed bone
// rotations of the customers' skeleton (both bodies share it; templates characters.json `skeleton`), and Moves = which
// animation plays at which moment. Mirrors the mod's Runtime/Customers/Moves.cs.

// Animation masks: the bones an animation may move; the rest stays on the game's animation. While a customer walks, "full"
// plays as "upper" (the legs keep walking).
var AnimationMasks = []string{"full", "upper", "arms"}

// AnimationMoments are the moments with no customer line of their own (the line situations are moments too).
var AnimationMoments = []Moment{
	{ID: "ripPack", Label: "Opens each pack (pack ripping)"},
	{ID: "ripLook", Label: "Looks at the cards from a pack"},
	{ID: "ripBigHit", Label: "Pulls a big hit (3× the hit price)"},
	{ID: "ripHugeHit", Label: "Pulls a huge hit (10× the hit price)"},
	{ID: "ripBrickStack", Label: "Opens 3+ packs without a hit"},
}

type Moment struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type Animation struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Length float64 `json:"length"` // seconds
	Mask   string  `json:"mask"`   // AnimationMasks
	// Bones: per bone name, keys in time order.
	Bones map[string][]AnimKey `json:"bones"`
	// FemaleBones: the female body's own keys, for built-in animations whose hands must meet (the bodies' sizes differ).
	FemaleBones map[string][]AnimKey `json:"femaleBones,omitempty"`
}

// AnimKey is a bone's local rotation (Unity x, y, z, w) at a time; between keys the rotation eases (smooth in and out).
type AnimKey struct {
	T float64    `json:"t"`
	R [4]float64 `json:"r"`
}

const (
	AnimMinLength = 0.1
	AnimMaxLength = 30.0
)

// ValidateAnimations mirrors Moves.Load in the mod: errors make the game skip an animation (or a moment).
func (l *AccessoryLibrary) ValidateAnimations() (errs, warns []string) {
	ids := map[string]bool{}
	for i, a := range l.Animations {
		where := fmt.Sprintf("animation %q", a.Name)
		switch {
		case !SafeID(a.ID):
			errs = append(errs, fmt.Sprintf("animation #%d: bad id", i+1))
			continue
		case ids[a.ID]:
			errs = append(errs, where+": duplicate id")
			continue
		case strings.TrimSpace(a.Name) == "":
			errs = append(errs, fmt.Sprintf("animation #%d: no name", i+1))
		case a.Length < AnimMinLength || a.Length > AnimMaxLength:
			errs = append(errs, fmt.Sprintf("%s: length %.2f s (%.1f–%.0f s)", where, a.Length, AnimMinLength, AnimMaxLength))
		case !contains(AnimationMasks, a.Mask):
			errs = append(errs, fmt.Sprintf("%s: unknown mask %q", where, a.Mask))
		}
		ids[a.ID] = true
		for bone, keys := range a.Bones {
			last := -1.0
			for _, k := range keys {
				n := math.Sqrt(k.R[0]*k.R[0] + k.R[1]*k.R[1] + k.R[2]*k.R[2] + k.R[3]*k.R[3])
				if k.T < 0 || k.T > a.Length+1e-6 || k.T <= last || math.Abs(n-1) > 0.01 {
					errs = append(errs, fmt.Sprintf("%s: bad key on %s at %.2f s", where, bone, k.T))
					break
				}
				last = k.T
			}
		}
		if len(a.Bones) == 0 {
			warns = append(warns, where+": no keys yet (it does nothing)")
		}
	}
	return errs, warns
}

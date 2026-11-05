package setfmt

import (
	"fmt"
	"strings"
)

// Employees (accessory library "employees", content kind "employee"): hireable employees made on the Employees page. Mirrors the mod's
// Core/Defs.cs EmployeeDef; outfits and faces work as a customer's (Runtime/Clothes/Wardrobe). Speeds are seconds per action as the game's WorkerData:
// lower is faster.
type Employee struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Female bool   `json:"female,omitempty"`
	// Model is the customer character they look like; -1 = one of the game's worker looks.
	Model         int     `json:"model"`
	Description   string  `json:"description,omitempty"`
	RestockSpeed  float64 `json:"restockSpeed"`
	CheckoutSpeed float64 `json:"checkoutSpeed"`
	// WalkSpeed is added to their walking speed: 0 = normal, 0.5 = half as fast again.
	WalkSpeed float64             `json:"walkSpeed,omitempty"`
	Wage      float64             `json:"wage"`
	HireCost  float64             `json:"hireCost"`
	Level     int                 `json:"level"`
	Perks     []string            `json:"perks"`
	Voice     *CustomerVoice      `json:"voice,omitempty"`
	Lines     map[string][]string `json:"lines,omitempty"`
	Removed   map[string][]string `json:"removed,omitempty"`
	// Anims are their own animations per moment, on top of the ones they inherit; AnimsOff are inherited ones they never play.
	Anims    map[string][]string `json:"anims,omitempty"`
	AnimsOff map[string][]string `json:"animsOff,omitempty"`
	Outfit   []OutfitSlot        `json:"outfit,omitempty"`
	Face     *CustomerFace       `json:"face,omitempty"`
	FaceID   string              `json:"faceId,omitempty"`
	Portrait string              `json:"portrait,omitempty"`
}

// ValidateEmployees: errors refuse the save, warnings are shown.
func (l *AccessoryLibrary) ValidateEmployees(knownPerks map[string]bool) (errs, warns []string) {
	ids := map[string]bool{}
	for i, e := range l.Employees {
		where := fmt.Sprintf("employee %q", e.Name)
		if strings.TrimSpace(e.ID) == "" {
			errs = append(errs, fmt.Sprintf("employee #%d: missing id", i+1))
			continue
		}
		if ids[e.ID] {
			errs = append(errs, where+": duplicate id")
		}
		ids[e.ID] = true
		if strings.TrimSpace(e.Name) == "" {
			errs = append(errs, fmt.Sprintf("employee #%d: needs a name", i+1))
		}
		if e.Model >= CustomerModels[e.Female] {
			errs = append(errs, fmt.Sprintf("%s: look %d doesn't exist (0–%d)", where, e.Model, CustomerModels[e.Female]-1))
		}
		if e.Face != nil && e.Model < 0 {
			errs = append(errs, where+": a face of their own needs a character to start from")
		}
		if e.RestockSpeed <= 0 || e.CheckoutSpeed <= 0 {
			errs = append(errs, where+": speeds must be above 0")
		}
		if e.Wage < 0 || e.HireCost < 0 || e.Level < 0 {
			errs = append(errs, where+": wage, hiring cost and level can't be negative")
		}
		if v := e.Voice; v != nil && (v.Pitch < 0.5 || v.Pitch > 2 || v.Tone < 0 || v.Tone > 1) {
			errs = append(errs, where+": a voice setting is out of range")
		}
		for _, p := range e.Perks {
			if knownPerks != nil && !knownPerks[p] {
				warns = append(warns, fmt.Sprintf("%s: unknown perk %q (ignored in game)", where, p))
			}
		}
	}
	return errs, warns
}

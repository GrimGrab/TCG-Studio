package setfmt

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Customers (accessory library "customers"): regulars made on the Customers page. Mirrors Persona in the mod's
// Runtime/Customers/Persona.cs (definition fields only; what happened in a save stays in the save) and Cast.Authored.

// CustomerModels is how many character models each body has (vanilla CustomerManager m_MaxMaleModelIndex 35 /
// m_MaxFemaleModelIndex 15, inclusive).
var CustomerModels = map[bool]int{false: 36, true: 16}

type Customer struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Female bool   `json:"female,omitempty"`
	// Model is the character model index; -1 = the look of their first visit (then kept).
	Model  int      `json:"model"`
	Traits []string `json:"traits"`
	// Skill is the table skill 1–100; -1 = rolled on their first visit.
	Skill int `json:"skill"`
	// Lines are their own lines per situation, said on top of the ones they inherit (built-in, the setup's, their traits').
	Lines map[string][]string `json:"lines,omitempty"`
	// Removed are inherited lines they never say, per situation.
	Removed map[string][]string `json:"removed,omitempty"`
	// Nickname is what they're known as (the game shows it under their name; they don't earn another).
	Nickname string `json:"nickname,omitempty"`
	// Anims are their own animations per moment, on top of the ones they inherit; AnimsOff are inherited ones they never play.
	Anims    map[string][]string `json:"anims,omitempty"`
	AnimsOff map[string][]string `json:"animsOff,omitempty"`
	// Outfit replaces pieces of their character's clothes (slots not named keep the character's own).
	Outfit []OutfitSlot `json:"outfit,omitempty"`
	// Face is their own face and body, made in the face editor from their character's (nil = the character's own).
	Face *CustomerFace `json:"face,omitempty"`
	// Portrait is their portrait rendered from Face ("assets/<hash>.png" in the shared store; the mod shows it instead of
	// their character's on the tournament screen and in the profile panel).
	Portrait string `json:"portrait,omitempty"`
	// Voice is their talking sound (nil = worked out from their id).
	Voice *CustomerVoice `json:"voice,omitempty"`
	// FaceID is the saved face (faces[]) their face was made as or copied from (Studio only: editing it updates that one).
	FaceID string `json:"faceId,omitempty"`
}

// CustomerVoice is a talking voice: Pitch 0.5–2 (how fast the letter recordings are read: 1 = as recorded, 2 = an octave up), Tone 0
// (muffled) – 0.5 (as recorded) – 1 (crisp).
type CustomerVoice struct {
	Pitch float64 `json:"pitch"`
	Tone  float64 `json:"tone"`
}

// UnmarshalJSON: what a file doesn't name keeps its middle (voices saved before these settings).
func (v *CustomerVoice) UnmarshalJSON(b []byte) error {
	type plain CustomerVoice
	p := plain{Pitch: 1, Tone: 0.5}
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*v = CustomerVoice(p)
	return nil
}

// SavedFace (accessory library "faces", content kind "face"): a face made in the face editor, kept to give to any regular
// of that body (each gets a copy as their Face). Model is the character it was made from (their clothes / body).
type SavedFace struct {
	ID       string       `json:"id"`
	Body     string       `json:"body"` // "male" | "female"
	Model    int          `json:"model"`
	Face     CustomerFace `json:"face"`
	Portrait string       `json:"portrait,omitempty"`
	// NoWalkIns keeps it out of the pool walk-ins roll from.
	NoWalkIns bool `json:"noWalkIns,omitempty"`
}

// CustomerFace is what a game character preset (CC_CharacterData) sets on the head and body, as the mod applies it with
// the game's CharacterCustomization: Shapes = face sliders (setBlendshapeByName, mod_* −3…3, shp_* −1…3, body_* 0…1, and
// the resting expression: mouthSmileLeft, jawOpen, … 0…1; the mod zeroes those on the next visitor), Hair =
// hair / eyebrows / beard piece names by hair slot (setHairByName), HairColor (setHairColor on every slot), Floats
// (_Skin_Tint_Alpha, _Freckles, _Wrinkles, _Make_Up_Alpha, _Lipstick_Alpha, _Skin_Smoothness_Offset, Height, Width),
// Textures (_Skin_Texture, _Make_Up: texture names) and Colors (_Skin_Tint, _Eye_Color, _Lips_Tint), colours #RRGGBB[AA].
type CustomerFace struct {
	Shapes    map[string]float64 `json:"shapes,omitempty"`
	Hair      []string           `json:"hair,omitempty"`
	HairColor string             `json:"hairColor,omitempty"`
	Floats    map[string]float64 `json:"floats,omitempty"`
	Textures  map[string]string  `json:"textures,omitempty"`
	Colors    map[string]string  `json:"colors,omitempty"`
}

// NewCustomer is a customer with a random look and skill (rolled by the game).
func NewCustomer(id, name string) Customer {
	return Customer{ID: id, Name: name, Model: -1, Skill: -1, Traits: []string{}}
}

// ValidateCustomers mirrors Cast.Authored: errors make the game skip a customer.
func (l *AccessoryLibrary) ValidateCustomers(knownTraits map[string]bool) (errs, warns []string) {
	ids := map[string]bool{}
	for i, c := range l.Customers {
		where := fmt.Sprintf("customer %q", c.Name)
		if strings.TrimSpace(c.ID) == "" {
			errs = append(errs, fmt.Sprintf("customer #%d: missing id", i+1))
			continue
		}
		if ids[c.ID] {
			errs = append(errs, where+": duplicate id")
		}
		ids[c.ID] = true
		if strings.TrimSpace(c.Name) == "" {
			errs = append(errs, fmt.Sprintf("customer #%d: needs a name", i+1))
		}
		if c.Model >= CustomerModels[c.Female] {
			errs = append(errs, fmt.Sprintf("%s: look %d doesn't exist (0–%d)", where, c.Model, CustomerModels[c.Female]-1))
		}
		if f := c.Face; f != nil {
			if c.Model < 0 {
				errs = append(errs, where+": a face of their own needs a character to start from (Pick one)")
			}
			cols := []string{f.HairColor}
			for _, v := range f.Colors {
				cols = append(cols, v)
			}
			for _, col := range cols {
				if col != "" && !hexColor.MatchString(col) {
					errs = append(errs, fmt.Sprintf("%s: bad colour %q", where, col))
				}
			}
		}
		if v := c.Voice; v != nil && (v.Pitch < 0.5 || v.Pitch > 2 || v.Tone < 0 || v.Tone > 1) {
			errs = append(errs, where+": a voice setting is out of range")
		}
		if c.Skill > 100 {
			errs = append(errs, where+": table skill goes up to 100")
		}
		for _, t := range c.Traits {
			if knownTraits != nil && !knownTraits[t] {
				warns = append(warns, fmt.Sprintf("%s: unknown trait %q (ignored in game)", where, t))
			}
		}
	}
	return errs, warns
}

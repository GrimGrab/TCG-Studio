package setfmt

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Clothes (accessory library "clothes"): new looks of the game's customer clothing pieces, made on Game look → Clothes.
// Mirrors ClothingDef in the mod's Runtime/Clothes/Wardrobe.cs. A look recolours a vanilla piece (the shader's _Tint,
// _Tint_R/G/B, as the game's own colour variants do) and lays a print on its UVs (_Print).

// ClothingBodies are the two customer bodies.
var ClothingBodies = []string{"male", "female"}

type Clothing struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Body  string `json:"body"`  // "male" | "female"
	Piece string `json:"piece"` // the game's piece name (TShirt_01)
	// Parts: one per material of the piece, in the piece's order.
	Parts []ClothingPart `json:"parts"`
	// Worn: of the customers dressed in this piece, how many in 100 wear this look instead (0 = only regulars' outfits).
	Worn int `json:"worn"`
	// Replaces: a vanilla colour variant of the piece (its label); everyone dressed in it wears this look instead.
	Replaces string `json:"replaces,omitempty"`
}

type ClothingPart struct {
	// Colours #RRGGBB or #RRGGBBAA: Tint colours the whole part, TintR/G/B the areas of the piece's tint mask.
	Tint  string `json:"tint"`
	TintR string `json:"tintR,omitempty"`
	TintG string `json:"tintG,omitempty"`
	TintB string `json:"tintB,omitempty"`
	// Print: the picture on the part's UVs, a PNG with transparency ("assets/<hash>.png"); "" = none.
	Print string `json:"print,omitempty"`
	// Design: the editor layout the print was painted with (Studio only).
	Design json.RawMessage `json:"design,omitempty"`
}

// OutfitSlot dresses a regular in a piece: Slot = the body's apparel slot (0 upper body, 1 lower body, 2 footwear,
// 3 misc); Clothing = one of the clothes' ids (a look of that piece), else Variant = a vanilla colour variant label
// ("" = the piece's first).
type OutfitSlot struct {
	Slot     int    `json:"slot"`
	Piece    string `json:"piece"`
	Variant  string `json:"variant,omitempty"`
	Clothing string `json:"clothing,omitempty"`
}

var hexColor = regexp.MustCompile(`^#([0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

// ValidateClothes mirrors Wardrobe.Load: errors make the game skip a look.
func (l *AccessoryLibrary) ValidateClothes() (errs, warns []string) {
	ids := map[string]bool{}
	for i, c := range l.Clothes {
		where := fmt.Sprintf("clothes %q", c.Name)
		if strings.TrimSpace(c.ID) == "" {
			errs = append(errs, fmt.Sprintf("clothes #%d: missing id", i+1))
			continue
		}
		if ids[c.ID] {
			errs = append(errs, where+": duplicate id")
		}
		ids[c.ID] = true
		if strings.TrimSpace(c.Name) == "" {
			errs = append(errs, fmt.Sprintf("clothes #%d: needs a name", i+1))
		}
		if c.Body != "male" && c.Body != "female" {
			errs = append(errs, where+`: body must be "male" or "female"`)
		}
		if strings.TrimSpace(c.Piece) == "" {
			errs = append(errs, where+": no piece of clothing chosen")
		}
		if len(c.Parts) == 0 {
			errs = append(errs, where+": no parts")
		}
		for j, p := range c.Parts {
			for _, col := range []string{p.Tint, p.TintR, p.TintG, p.TintB} {
				if col != "" && !hexColor.MatchString(col) {
					errs = append(errs, fmt.Sprintf("%s part %d: bad colour %q", where, j+1, col))
				}
			}
		}
		if c.Worn < 0 || c.Worn > 100 {
			errs = append(errs, where+": worn goes from 0 to 100")
		}
	}
	for _, cu := range l.Customers {
		for _, o := range cu.Outfit {
			if o.Slot < 0 || o.Slot > 3 {
				errs = append(errs, fmt.Sprintf("customer %q: outfit slot %d doesn't exist", cu.Name, o.Slot))
			}
			if o.Clothing != "" && !ids[o.Clothing] {
				warns = append(warns, fmt.Sprintf("customer %q: an outfit look isn't in the clothes list any more (the piece's own colours are used)", cu.Name))
			}
		}
	}
	return errs, warns
}

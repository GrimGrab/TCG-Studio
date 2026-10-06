package setfmt

// Furniture: mirrors FurnitureDef/FurnitureSpotDef in the mod's Core/Defs.cs and AccessoryLoader.LoadFurniture (same file as the
// accessories). See docs/set-format.md "Furniture".

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// FurnitureType is the behaviour of a piece (the base piece's game component). Mirrors Core/FurnitureKinds.cs.
type FurnitureType struct {
	Type        string      `json:"type"`        // JSON value
	Title       string      `json:"title"`       // "Shelves"
	One         string      `json:"one"`         // "shelf"
	DefaultBase string      `json:"defaultBase"` // vanilla EObjectType used when base is empty
	Spots       string      `json:"spots"`       // "items" | "card" | "" (fixed)
	Points      []PointRole `json:"points"`      // position roles (seats, stand points…); see PointRoles
}

var FurnitureTypes = []FurnitureType{
	{Type: "Shelf", Title: "Shelves", One: "shelf", DefaultBase: "Shelf", Spots: "items"},
	{Type: "CardShelf", Title: "Card shelves", One: "card shelf", DefaultBase: "CardShelf", Spots: "card"},
	{Type: "PlayTable", Title: "Play tables", One: "play table", DefaultBase: "PlayTable"},
	{Type: "BulkDonationBox", Title: "Bulk donation bins", One: "bulk donation bin", DefaultBase: "BulkDonationBox"},
	{Type: "TrashBin", Title: "Trash bins", One: "trash bin", DefaultBase: "Trashbin"},
	{Type: "EmptyBoxStorage", Title: "Empty box storage", One: "empty box storage", DefaultBase: "EmptyBoxStorage"},
	{Type: "CardStorageShelf", Title: "Card storage shelves", One: "card storage shelf", DefaultBase: "CardStorageShelf"},
	{Type: "AutoPackOpener", Title: "Auto pack openers", One: "auto pack opener", DefaultBase: "AutoPackOpener1"},
	{Type: "AutoCleanser", Title: "Auto cleansers", One: "auto cleanser", DefaultBase: "AutoCleanser1"},
	{Type: "Workbench", Title: "Workbenches", One: "workbench", DefaultBase: "Workbench"},
	{Type: "CashCounter", Title: "Cash counters", One: "cash counter", DefaultBase: "CashCounter"},
}

func FurnitureTypeInfo(t string) (FurnitureType, bool) {
	for _, k := range FurnitureTypes {
		if k.Type == t {
			k.Points = PointRoles(t)
			return k, true
		}
	}
	return FurnitureType{}, false
}

// FurnitureTypesWithPoints is FurnitureTypes with each type's point roles filled in (for the editor).
func FurnitureTypesWithPoints() []FurnitureType {
	out := make([]FurnitureType, len(FurnitureTypes))
	for i, k := range FurnitureTypes {
		k.Points = PointRoles(k.Type)
		out[i] = k
	}
	return out
}

type Furniture struct {
	ID          string           `json:"id"`
	Type        string           `json:"type"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Base        string           `json:"base,omitempty"`
	Price       *float64         `json:"price,omitempty"`
	Level       *int             `json:"level,omitempty"`
	DecoBonus   *float64         `json:"decoBonus,omitempty"`
	Icon        string           `json:"icon,omitempty"`
	Texture     string           `json:"texture,omitempty"`
	Tint        string           `json:"tint,omitempty"`
	Mesh        string           `json:"mesh,omitempty"`
	Spots       []FurnitureSpot  `json:"spots,omitempty"`
	Area        *FurnitureArea   `json:"area,omitempty"`
	Points      []FurniturePoint `json:"points,omitempty"`
}

// FurniturePoint is a position the game uses on a piece (seat, where the cashier or a worker stands, customer stand points…), in
// piece space (metres, degrees). Points of a role replace the base piece's in order; roles not listed stay the base's.
type FurniturePoint struct {
	Role string     `json:"role"`
	Pos  [3]float64 `json:"pos"`
	Rot  [3]float64 `json:"rot"`
}

// PointRole describes a role of a furniture type. Resizable roles may have more or fewer points than the base (lists the game
// picks from); the others keep the base piece's count (the game's logic expects exactly those). Mirrors Core/FurnitureKinds.cs.
type PointRole struct {
	Role      string `json:"role"`
	Label     string `json:"label"`
	Tip       string `json:"tip"`
	Resizable bool   `json:"resizable"`
}

var furniturePointRoles = map[string][]PointRole{
	"PlayTable": {
		{Role: "sit", Label: "Seat", Tip: "Where a player sits (one per seat)."},
		{Role: "stand", Label: "Seat stand point", Tip: "Where a player stands to sit down / get up at that seat."},
		{Role: "standB", Label: "Waiting point", Tip: "Where a customer waits for that seat."},
	},
	"CashCounter": {
		{Role: "cashier", Label: "Cashier", Tip: "Where you (or a worker) stand to run the till."},
		{Role: "queue", Label: "Queue start", Tip: "Where the checkout queue starts; it grows along this point's forward direction."},
		{Role: "placeItems", Label: "Item drop", Tip: "Where customers put their items on the counter."},
		{Role: "trade", Label: "Trade stand point", Tip: "Where customers stand to trade cards at the counter.", Resizable: true},
	},
	"AutoPackOpener":   {{Role: "worker", Label: "Worker", Tip: "Where a worker stands to use the machine."}},
	"AutoCleanser":     {{Role: "worker", Label: "Worker", Tip: "Where a worker stands to refill the machine."}},
	"Workbench":        {{Role: "player", Label: "Player", Tip: "Where you stand to use the workbench."}},
	"BulkDonationBox":  {{Role: "customer", Label: "Customer stand point", Tip: "Where customers stand to use the bin (one is picked at random).", Resizable: true}},
	"CardStorageShelf": {{Role: "customer", Label: "Stand point", Tip: "Where workers/customers stand at the shelf (one is picked at random).", Resizable: true}},
	"EmptyBoxStorage":  {{Role: "customer", Label: "Stand point", Tip: "Where people stand to drop empty boxes (one is picked at random).", Resizable: true}},
}

// PointRoles lists a furniture type's point roles (nil = none).
func PointRoles(furnitureType string) []PointRole { return furniturePointRoles[furnitureType] }

// FurnitureArea is the placement area (piece space, metres): centre x/z and width/depth; nil = the base piece's.
type FurnitureArea struct {
	Pos  [2]float64 `json:"pos"`
	Size [2]float64 `json:"size"`
}

type FurnitureSpot struct {
	Kind     string      `json:"kind"` // items | card
	Pos      [3]float64  `json:"pos"`
	Rot      [3]float64  `json:"rot"`
	Size     *[3]float64 `json:"size,omitempty"`
	Grid     *[3]int     `json:"grid,omitempty"`
	Customer *[2]float64 `json:"customer,omitempty"`
	PriceTag *[3]float64 `json:"priceTag,omitempty"`
	Boxes    *bool       `json:"boxes,omitempty"`
}

// DefaultBase is the vanilla piece used when Base is empty (same default as the mod).
func (f Furniture) DefaultBase() string {
	if f.Base != "" {
		return f.Base
	}
	if k, ok := FurnitureTypeInfo(f.Type); ok {
		return k.DefaultBase
	}
	return ""
}

var tintRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// ValidateFurniture mirrors AccessoryLoader.LoadFurniture. bases maps a vanilla EObjectType name to its furniture type (from the
// game templates; nil = not known yet, the base isn't checked).
func (l *AccessoryLibrary) ValidateFurniture(resolve func(rel string) string, bases map[string]string) (errs, warns []string) {
	ids := map[string]bool{}
	for i, f := range l.Furniture {
		where := fmt.Sprintf("furniture %q", f.ID)
		if strings.TrimSpace(f.ID) == "" {
			errs = append(errs, fmt.Sprintf("furniture #%d: missing id", i+1))
			continue
		}
		if strings.ContainsAny(f.ID, " \t:/|") {
			errs = append(errs, where+": id may not contain spaces, ':', '/' or '|'")
		}
		if ids[f.ID] {
			errs = append(errs, where+": duplicate id")
		}
		ids[f.ID] = true
		k, ok := FurnitureTypeInfo(f.Type)
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: unknown type %q", where, f.Type))
			continue
		}
		if bases != nil {
			if t, known := bases[f.DefaultBase()]; !known {
				errs = append(errs, fmt.Sprintf("%s: base %q is not a vanilla furniture piece", where, f.DefaultBase()))
			} else if t != f.Type {
				errs = append(errs, fmt.Sprintf("%s: base %q is not a %s", where, f.DefaultBase(), k.One))
			}
		}
		if f.Price != nil && *f.Price <= 0 {
			errs = append(errs, where+": price must be > 0")
		}
		if f.Level != nil && *f.Level < 0 {
			errs = append(errs, where+": level must be ≥ 0")
		}
		if f.Tint != "" && !tintRe.MatchString(f.Tint) {
			errs = append(errs, fmt.Sprintf("%s: tint %q is not a colour (#RRGGBB)", where, f.Tint))
		}
		for _, file := range []string{f.Texture, f.Icon, f.Mesh} {
			if file != "" {
				if _, err := os.Stat(resolve(file)); err != nil {
					warns = append(warns, fmt.Sprintf("%s: file not found %q (base piece's used)", where, file))
				}
			}
		}
		for j, pt := range f.Points {
			ok := false
			for _, r := range PointRoles(f.Type) {
				ok = ok || r.Role == pt.Role
			}
			if !ok {
				errs = append(errs, fmt.Sprintf("%s point %d: a %s has no %q points", where, j+1, k.One, pt.Role))
			}
		}
		if f.Area != nil && (f.Area.Size[0] <= 0 || f.Area.Size[1] <= 0) {
			errs = append(errs, where+": placement area width and depth must be > 0")
		}
		if len(f.Spots) > 64 {
			errs = append(errs, where+": at most 64 spots")
		}
		for j, s := range f.Spots {
			sw := fmt.Sprintf("%s spot %d", where, j+1)
			switch s.Kind {
			case "items", "card":
			default:
				errs = append(errs, fmt.Sprintf("%s: unknown kind %q", sw, s.Kind))
				continue
			}
			if s.Customer == nil {
				errs = append(errs, sw+": no customer point (customers stand there to take from the spot)")
			}
			if s.Kind != k.Spots {
				errs = append(errs, fmt.Sprintf("%s: a %s has no %s spots", sw, k.One, s.Kind))
			}
			if s.Kind == "items" {
				if s.Size == nil || s.Size[0] <= 0 || s.Size[1] <= 0 || s.Size[2] < 0 {
					errs = append(errs, sw+": size needs width, depth > 0 and height ≥ 0")
				}
				if s.Grid != nil && (s.Grid[0] < 1 || s.Grid[1] < 1 || s.Grid[2] < 1 || s.Grid[0]*s.Grid[1]*s.Grid[2] > 512) {
					errs = append(errs, sw+": grid needs 3 whole numbers ≥ 1 (at most 512 units)")
				}
			}
		}
	}
	return
}

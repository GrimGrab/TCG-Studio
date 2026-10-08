package setfmt

// Furniture: mirrors FurnitureDef/FurnitureSpotDef in the mod's Core/Defs.cs and AccessoryLoader.LoadFurniture (same file as the
// accessories). See docs/set-format.md "Furniture".

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

// FurnitureType is the behaviour of a piece (the base piece's game component). Mirrors Core/FurnitureKinds.cs.
type FurnitureType struct {
	Type        string      `json:"type"`        // JSON value
	Title       string      `json:"title"`       // "Shelves"
	One         string      `json:"one"`         // "shelf"
	DefaultBase string      `json:"defaultBase"` // vanilla EObjectType used when base is empty
	Spots       []string    `json:"spots"`       // spot kinds it can have: "items", "card" (none = fixed)
	Points      []PointRole `json:"points"`      // position roles (seats, stand points…); see PointRoles
}

var FurnitureTypes = []FurnitureType{
	{Type: "Shelf", Title: "Shelves", One: "shelf", DefaultBase: "Shelf", Spots: []string{"items"}},
	{Type: "CardShelf", Title: "Card shelves", One: "card shelf", DefaultBase: "CardShelf", Spots: []string{"card"}},
	{Type: "WarehouseShelf", Title: "Warehouse shelves", One: "warehouse shelf", DefaultBase: "WarehouseShelf", Spots: []string{"items"}},
	{Type: "TournamentPrizeShelf", Title: "Tournament prize shelves", One: "tournament prize shelf", DefaultBase: "TournamentPrizeShelf", Spots: []string{"items", "card"}},
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
	Paint       *FurniturePaint  `json:"paint,omitempty"`
	Spots       []FurnitureSpot  `json:"spots,omitempty"`
	Area        *FurnitureArea   `json:"area,omitempty"`
	Points      []FurniturePoint `json:"points,omitempty"`
}

// FurniturePaint is a piece painted in the face editor: the painted atlas and the base's body renderers with meshes that read
// it (the base's paint template, gameextract/furniture_paint.go). Replaces texture/tint; not with an own mesh.
type FurniturePaint struct {
	Texture string      `json:"texture"`
	Parts   []PaintPart `json:"parts"`
}

// PaintPart: a body renderer ("<sibling index>:<name>/…" under the piece root) and its mesh with the painting UVs.
type PaintPart struct {
	Renderer string `json:"renderer"`
	Mesh     string `json:"mesh"`
}

// FileRefs points at every file field of the piece (texture, icon, mesh, paint texture and meshes), for code that lists,
// copies or moves its files.
func (f *Furniture) FileRefs() []*string {
	out := []*string{&f.Texture, &f.Icon, &f.Mesh}
	if f.Paint != nil {
		out = append(out, &f.Paint.Texture)
		for i := range f.Paint.Parts {
			out = append(out, &f.Paint.Parts[i].Mesh)
		}
	}
	return out
}

// Files lists the piece's file paths (empty ones included).
func (f Furniture) Files() []string {
	var out []string
	for _, p := range f.FileRefs() {
		out = append(out, *p)
	}
	return out
}

// Clone is a deep copy (Paint is shared by plain copies).
func (f Furniture) Clone() Furniture {
	if f.Paint != nil {
		p := *f.Paint
		p.Parts = append([]PaintPart(nil), f.Paint.Parts...)
		f.Paint = &p
	}
	f.Spots = append([]FurnitureSpot(nil), f.Spots...)
	f.Points = append([]FurniturePoint(nil), f.Points...)
	return f
}

// FurniturePoint is a position the game uses on a piece (seat, where the cashier or a worker stands, customer stand points…), in
// piece space (metres, degrees). Points of a role replace the base piece's in order; roles not listed stay the base's.
type FurniturePoint struct {
	Role string     `json:"role"`
	Pos  [3]float64 `json:"pos"`
	Rot  [3]float64 `json:"rot"`
	// Scale: size of a spot or working part against the vanilla piece's (screens, drawer, signs…); 0 = 1.
	Scale float64 `json:"scale,omitempty"`
}

// PointRole describes a role of a furniture type. Resizable roles may have more or fewer points than the base (lists the game
// picks from); the others keep the base piece's count (the game's logic expects exactly those). Mirrors Core/FurnitureKinds.cs.
type PointRole struct {
	Role      string `json:"role"`
	Label     string `json:"label"`
	Tip       string `json:"tip"`
	Resizable bool   `json:"resizable"`
	// Kind: "person" (someone stands/sits there), "spot" (a place the game uses: screen UI, where money lands…) or "part" (a working
	// part with its own look — drawer, card machine, signs, TV — kept visible under an own model and not painted). Mirrors
	// FurnitureKinds.PointKind.
	Kind string `json:"kind"`
	// Parent: the point is the parent of the field's object (the drawer's animation plays on its own transform).
	Parent bool `json:"parent,omitempty"`
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
		{Role: "placeItems", Label: "Item drop", Tip: "Where customers put their items on the counter.", Kind: "spot"},
		{Role: "trade", Label: "Trade stand point", Tip: "Where customers stand to trade cards at the counter.", Resizable: true},
		{Role: "screen", Label: "Till screen", Tip: "Where the till's screen (totals, change) is shown — put it on your model's monitor.", Kind: "spot"},
		{Role: "scanItem", Label: "Scanned items", Tip: "Where scanned items fly to (into the bag).", Kind: "spot"},
		{Role: "money", Label: "Paid notes", Tip: "Where customers put their notes.", Kind: "spot"},
		{Role: "coin", Label: "Paid coins", Tip: "Where customers put their coins.", Kind: "spot"},
		{Role: "drawer", Label: "Cash drawer", Tip: "The till's drawer with the notes and coins you give change from; it slides open when a customer pays cash.", Kind: "part", Parent: true},
		{Role: "cardMachine", Label: "Card machine", Tip: "The card reader (its screen moves with it).", Kind: "part"},
		{Role: "cardPay", Label: "Card machine (paying)", Tip: "Where the card reader is held up while a customer pays by card.", Kind: "spot"},
		{Role: "cardLook", Label: "Card payment view", Tip: "Where you look while entering a card payment.", Kind: "spot"},
		{Role: "bag", Label: "Shopping bag", Tip: "The open paper bag scanned items go into.", Kind: "part"},
		{Role: "closedSign", Label: "Closed sign", Tip: "The sign shown when the counter is closed.", Kind: "part"},
		{Role: "tradeSign", Label: "No trading sign", Tip: "The sign shown when trading is off.", Kind: "part"},
	},
	"AutoPackOpener": {
		{Role: "worker", Label: "Worker", Tip: "Where a worker stands to use the machine."},
		{Role: "screen", Label: "Progress bar", Tip: "Where the machine's progress bar is shown (its size and facing follow this point).", Kind: "spot"},
		{Role: "packIn", Label: "Pack slot", Tip: "Where packs go when they are put into the machine.", Kind: "spot"},
		{Role: "packInside", Label: "Packs inside", Tip: "Where packs wait inside the machine.", Kind: "spot"},
	},
	"AutoCleanser":     {{Role: "worker", Label: "Worker", Tip: "Where a worker stands to refill the machine."}},
	"Workbench":        {{Role: "player", Label: "Player", Tip: "Where you stand to use the workbench."}},
	"BulkDonationBox":  {{Role: "customer", Label: "Customer stand point", Tip: "Where customers stand to use the bin (one is picked at random).", Resizable: true}},
	"CardStorageShelf": {{Role: "customer", Label: "Stand point", Tip: "Where workers/customers stand at the shelf (one is picked at random).", Resizable: true}},
	"EmptyBoxStorage":  {{Role: "customer", Label: "Stand point", Tip: "Where people stand to drop empty boxes (one is picked at random).", Resizable: true}},
	"TournamentPrizeShelf": {
		{Role: "customer", Label: "Viewer stand point", Tip: "Where customers stand to look at the prizes (one is picked at random).", Resizable: true},
		{Role: "winner", Label: "Winner pose point", Tip: "Where a tournament winner poses with their prize, by placement (1st, 2nd…)."},
		{Role: "screen", Label: "Tournament screen", Tip: "The TV that switches on on tournament days.", Kind: "part"},
	},
}

// PointRoles lists a furniture type's point roles (nil = none); Kind defaults to "person".
func PointRoles(furnitureType string) []PointRole {
	rs := furniturePointRoles[furnitureType]
	out := make([]PointRole, len(rs))
	for i, r := range rs {
		if r.Kind == "" {
			r.Kind = "person"
		}
		out[i] = r
	}
	return out
}

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
		for _, file := range f.Files() {
			if file != "" {
				if _, err := os.Stat(resolve(file)); err != nil {
					warns = append(warns, fmt.Sprintf("%s: file not found %q (base piece's used)", where, file))
				}
			}
		}
		if f.Paint != nil && f.Mesh != "" {
			errs = append(errs, where+": a painted piece can't also have its own model")
		}
		if f.Paint != nil && (f.Paint.Texture == "" || len(f.Paint.Parts) == 0) {
			errs = append(errs, where+": paint needs a texture and its parts")
		}
		for j, pt := range f.Points {
			ok := false
			for _, r := range PointRoles(f.Type) {
				ok = ok || r.Role == pt.Role
			}
			if !ok {
				errs = append(errs, fmt.Sprintf("%s point %d: a %s has no %q points", where, j+1, k.One, pt.Role))
			}
			if pt.Scale < 0 || pt.Scale > 20 {
				errs = append(errs, fmt.Sprintf("%s point %d: size must be between 0 and 20 times the vanilla size", where, j+1))
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
			if !slices.Contains(k.Spots, s.Kind) {
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

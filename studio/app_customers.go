package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/customers"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/game"
	"tcgstudio/internal/gameextract"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
)

// Customers page: regulars made in Studio (the active setup's item library, content kind "customer"), the setup's own
// lines (<setup>\game\customer-lines.json) and the regulars the game made in this setup's saves, to adopt. The mod reads
// both files when the game starts (Runtime/Customers/Cast).

// CustomersPage is everything the page shows.
type CustomersPage struct {
	Customers []setfmt.Customer    `json:"customers"`
	Lines     customers.SetupLines `json:"lines"`
	Defaults  customers.Defaults   `json:"defaults"`
	Saved     []customers.Saved    `json:"saved"`
	Models    map[string]int       `json:"models"` // looks per body: "male", "female"
	// Outfits: the clothes made on Game look → Clothes and the game's pieces (templates clothes.json URL, "" = not read yet).
	Clothes          []setfmt.Clothing `json:"clothes"`
	ClothesTemplates string            `json:"clothesTemplates"`
	// The face editor's characters (templates characters.json URL, "" = not read yet) and the saved faces.
	CharactersTemplates string             `json:"charactersTemplates"`
	Faces               []setfmt.SavedFace `json:"faces"`
	// Customer animations (Animations tab), which plays at which moment, and the moments with no line of their own (the line
	// situations in Defaults are moments too).
	Animations []setfmt.Animation `json:"animations"`
	// The built-in ones and what the setup changes (added / switched off per moment); customers add theirs on top.
	DefaultAnimations []setfmt.Animation  `json:"defaultAnimations"`
	DefaultMoves      map[string][]string `json:"defaultMoves"`
	MomentAnims       map[string][]string `json:"momentAnims"`
	MomentOff         map[string][]string `json:"momentOff"`
	Moments           []setfmt.Moment     `json:"moments"`
}

// knownAnims is the map without animations that don't exist and without empty moments; nil when nothing is left.
func knownAnims(m map[string][]string, known map[string]bool) map[string][]string {
	var out map[string][]string
	for moment, ids := range m {
		for _, id := range ids {
			if known[id] {
				if out == nil {
					out = map[string][]string{}
				}
				out[moment] = append(out[moment], id)
			}
		}
	}
	return out
}

// LoadCustomersPage reads the page's data for the active setup.
func (a *App) LoadCustomersPage() (CustomersPage, error) {
	var page CustomersPage
	d, err := customers.LoadDefaults()
	if err != nil {
		return page, err
	}
	page.Defaults = d
	page.Models = map[string]int{"male": setfmt.CustomerModels[false], "female": setfmt.CustomerModels[true]}
	l, err := a.items()
	if err != nil {
		return page, err
	}
	page.Customers = append([]setfmt.Customer{}, l.Lib.Customers...)
	page.Clothes = append([]setfmt.Clothing{}, l.Lib.Clothes...)
	page.Faces = append([]setfmt.SavedFace{}, l.Lib.Faces...)
	page.Animations = append([]setfmt.Animation{}, l.Lib.Animations...)
	da, err := customers.LoadDefaultAnimations()
	if err != nil {
		return page, err
	}
	page.DefaultAnimations, page.DefaultMoves = da.Animations, da.Moves
	page.MomentAnims, page.MomentOff = l.Lib.MomentAnims, l.Lib.MomentOff
	page.Moments = setfmt.AnimationMoments
	if fsutil.Exists(a.templateFile(gameextract.ClothesDir, "clothes.json")) {
		page.ClothesTemplates = "/clothtemplates/clothes.json"
	}
	if fsutil.Exists(a.templateFile(gameextract.CharactersDir, "characters.json")) {
		page.CharactersTemplates = "/chartemplates/characters.json"
	}
	have := map[string]bool{}
	for _, c := range page.Customers {
		have[c.ID] = true
	}
	if page.Lines, err = customers.LoadLines(filepath.Join(a.root(), setups.GameStateDir)); err != nil {
		return page, err
	}
	// The active setup's saves are in the game's save folder while it is active.
	page.Saved = customers.FromSaves(filepath.Join(game.SaveDir(), "TCGCustomCards"), have)
	if page.Saved == nil {
		page.Saved = []customers.Saved{}
	}
	return page, nil
}

// NewCustomerID is a fresh random id for a customer made on the page.
func (a *App) NewCustomerID() (string, error) {
	l, err := a.items()
	if err != nil {
		return "", err
	}
	return l.NewID(), nil
}

// SaveCustomers stores the page (Save): the customers list and the setup's lines. Returns warnings; errors refuse the
// save so the game never skips a customer silently. The game reads the change at its next start.
func (a *App) SaveCustomers(list []setfmt.Customer, lines customers.SetupLines, faces []setfmt.SavedFace, anims []setfmt.Animation, momentAnims, momentOff map[string][]string) ([]string, error) {
	d, err := customers.LoadDefaults()
	if err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Name = strings.TrimSpace(list[i].Name)
		list[i].Lines = customers.Clean(list[i].Lines)
		list[i].Removed = customers.Clean(list[i].Removed)
		if list[i].Face == nil {
			list[i].Portrait = ""
		}
		if list[i].Traits == nil {
			list[i].Traits = []string{}
		}
	}
	l, err := a.items()
	if err != nil {
		return nil, err
	}
	store := portraitStore(l)
	for i := range list {
		if err := store(list[i].Name, &list[i].Portrait); err != nil {
			return nil, err
		}
	}
	if err := checkFaces(l, faces, store); err != nil {
		return nil, err
	}
	for i := range anims {
		anims[i].Name = strings.TrimSpace(anims[i].Name)
		if anims[i].Bones == nil {
			anims[i].Bones = map[string][]setfmt.AnimKey{}
		}
		if l.AnimationIndex(anims[i].ID) < 0 && l.HasItem(anims[i].ID) {
			return nil, fmt.Errorf("animation %q: its id is used by another item", anims[i].Name)
		}
	}
	// An animation that was deleted leaves every list it was in.
	da, err := customers.LoadDefaultAnimations()
	if err != nil {
		return nil, err
	}
	animIDs := map[string]bool{}
	for _, x := range da.Animations {
		animIDs[x.ID] = true
	}
	for _, x := range anims {
		animIDs[x.ID] = true
	}
	momentAnims, momentOff = knownAnims(momentAnims, animIDs), knownAnims(momentOff, animIDs)
	for i := range list {
		list[i].Anims, list[i].AnimsOff = knownAnims(list[i].Anims, animIDs), knownAnims(list[i].AnimsOff, animIDs)
	}
	for i := range l.Lib.Employees {
		e := &l.Lib.Employees[i]
		e.Anims, e.AnimsOff = knownAnims(e.Anims, animIDs), knownAnims(e.AnimsOff, animIDs)
	}
	check := setfmt.AccessoryLibrary{Customers: list, Clothes: l.Lib.Clothes, Animations: anims}
	errs, warns := check.ValidateCustomers(d.TraitIDs())
	cerrs, cwarns := check.ValidateClothes()
	aerrs, awarns := check.ValidateAnimations()
	errs, warns = append(append(errs, cerrs...), aerrs...), append(append(warns, cwarns...), awarns...)
	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "\n"))
	}
	l.Lib.Customers = list
	l.Lib.Faces = faces
	syncFaces(l)
	l.Lib.Animations = anims
	l.Lib.MomentAnims, l.Lib.MomentOff = momentAnims, momentOff
	if err := l.Save(); err != nil {
		return nil, err
	}
	// The file also holds the employees' lines, from the Employees page.
	if old, err := customers.LoadLines(filepath.Join(a.root(), setups.GameStateDir)); err == nil {
		lines.EmployeeLines, lines.EmployeeRemoved = old.EmployeeLines, old.EmployeeRemoved
	}
	if err := customers.SaveLines(filepath.Join(a.root(), setups.GameStateDir), lines); err != nil {
		return nil, err
	}
	if game.IsRunning() {
		warns = append(warns, "The game is running: it picks up the change at its next start.")
	}
	return warns, nil
}

// portraitStore puts a portrait that came as a PNG data URL into the shared store (never the game folder).
func portraitStore(l *accessories.Library) func(name string, p *string) error {
	return func(name string, p *string) error {
		if !strings.HasPrefix(*p, "data:") {
			return nil
		}
		png, err := decodeDataURL(*p)
		if err != nil || len(png) < 8 || string(png[1:4]) != "PNG" {
			return fmt.Errorf("%s: bad portrait", name)
		}
		*p, err = l.PutBytes(png, ".png")
		return err
	}
}

// checkFaces validates the saved faces and stores their portraits.
func checkFaces(l *accessories.Library, faces []setfmt.SavedFace, store func(string, *string) error) error {
	ids := map[string]bool{}
	for i := range faces {
		f := &faces[i]
		switch {
		case !setfmt.SafeID(f.ID) || ids[f.ID]:
			return fmt.Errorf("saved face %q: bad or repeated id", f.ID)
		case f.Body != "male" && f.Body != "female":
			return fmt.Errorf("saved face %q: unknown body", f.ID)
		case l.FaceIndex(f.ID) < 0 && l.HasItem(f.ID):
			return fmt.Errorf("saved face %q: its id is used by another item", f.ID)
		}
		ids[f.ID] = true
		if err := store("saved face", &f.Portrait); err != nil {
			return err
		}
	}
	return nil
}

// syncFaces: faces belong to the whole setup. Everyone given a saved face follows its edits, whichever page made them; someone whose face was deleted keeps
// their copy.
func syncFaces(l *accessories.Library) {
	byID := map[string]*setfmt.SavedFace{}
	for i := range l.Lib.Faces {
		byID[l.Lib.Faces[i].ID] = &l.Lib.Faces[i]
	}
	follow := func(id *string, face **setfmt.CustomerFace, model *int, portrait *string) {
		if *id == "" {
			return
		}
		f := byID[*id]
		if f == nil {
			*id = ""
			return
		}
		if *face != nil && *model == f.Model && reflect.DeepEqual(**face, f.Face) {
			return // nothing changed: their portrait shows their own outfit
		}
		copied := f.Face
		*face, *model, *portrait = &copied, f.Model, f.Portrait
	}
	for i := range l.Lib.Customers {
		c := &l.Lib.Customers[i]
		follow(&c.FaceID, &c.Face, &c.Model, &c.Portrait)
	}
	for i := range l.Lib.Employees {
		e := &l.Lib.Employees[i]
		follow(&e.FaceID, &e.Face, &e.Model, &e.Portrait)
	}
}

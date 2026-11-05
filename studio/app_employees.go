package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"tcgstudio/internal/customers"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/game"
	"tcgstudio/internal/gameextract"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
)

// Employees page: hireable employees made in Studio (the active setup's item library, content kind "employee") and the setup's employee lines (kept in
// customer-lines.json beside the customers'). The mod reads both when the game starts (Runtime/Employees).

// EmployeesPage is everything the page shows.
type EmployeesPage struct {
	Employees         []setfmt.Employee   `json:"employees"`
	Lines             map[string][]string `json:"lines"`
	Removed           map[string][]string `json:"removed"`
	Defaults          customers.Defaults  `json:"defaults"`
	Models            map[string]int      `json:"models"`
	Clothes           []setfmt.Clothing   `json:"clothes"`
	ClothesTemplates  string              `json:"clothesTemplates"`
	Faces             []setfmt.SavedFace  `json:"faces"`
	Animations        []setfmt.Animation  `json:"animations"`
	DefaultAnimations []setfmt.Animation  `json:"defaultAnimations"`
	DefaultMoves      map[string][]string `json:"defaultMoves"`
	MomentAnims       map[string][]string `json:"momentAnims"`
	MomentOff         map[string][]string `json:"momentOff"`
	// The face editor's characters (templates characters.json URL, "" = not read yet).
	CharactersTemplates string `json:"charactersTemplates"`
}

// LoadEmployeesPage reads the page's data for the active setup.
func (a *App) LoadEmployeesPage() (EmployeesPage, error) {
	var page EmployeesPage
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
	page.Employees = append([]setfmt.Employee{}, l.Lib.Employees...)
	page.Clothes = append([]setfmt.Clothing{}, l.Lib.Clothes...)
	page.Faces = append([]setfmt.SavedFace{}, l.Lib.Faces...)
	page.Animations = append([]setfmt.Animation{}, l.Lib.Animations...)
	da, err := customers.LoadDefaultAnimations()
	if err != nil {
		return page, err
	}
	page.DefaultAnimations, page.DefaultMoves = da.Animations, da.Moves
	page.MomentAnims, page.MomentOff = l.Lib.MomentAnims, l.Lib.MomentOff
	if fsutil.Exists(a.templateFile(gameextract.ClothesDir, "clothes.json")) {
		page.ClothesTemplates = "/clothtemplates/clothes.json"
	}
	if fsutil.Exists(a.templateFile(gameextract.CharactersDir, "characters.json")) {
		page.CharactersTemplates = "/chartemplates/characters.json"
	}
	lines, err := customers.LoadLines(filepath.Join(a.root(), setups.GameStateDir))
	if err != nil {
		return page, err
	}
	page.Lines, page.Removed = lines.EmployeeLines, lines.EmployeeRemoved
	return page, nil
}

// SaveEmployees stores the page (Save): the employees and the setup's employee lines. Returns warnings; errors refuse the save. The game reads the change
// at its next start.
func (a *App) SaveEmployees(list []setfmt.Employee, lines, removed map[string][]string, faces []setfmt.SavedFace, anims []setfmt.Animation, momentAnims, momentOff map[string][]string) ([]string, error) {
	d, err := customers.LoadDefaults()
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, p := range d.EmployeePerks {
		known[p.ID] = true
	}
	l, err := a.items()
	if err != nil {
		return nil, err
	}
	store := portraitStore(l)
	if err := checkFaces(l, faces, store); err != nil {
		return nil, err
	}
	for i := range list {
		e := &list[i]
		e.Name = strings.TrimSpace(e.Name)
		e.Lines = customers.Clean(e.Lines)
		e.Removed = customers.Clean(e.Removed)
		if e.Perks == nil {
			e.Perks = []string{}
		}
		if e.Face == nil {
			e.Portrait = ""
		}
		if err := store(e.Name, &e.Portrait); err != nil {
			return nil, err
		}
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
	// An animation that was deleted leaves every list it was in, on either page.
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
	for i := range l.Lib.Customers {
		c := &l.Lib.Customers[i]
		c.Anims, c.AnimsOff = knownAnims(c.Anims, animIDs), knownAnims(c.AnimsOff, animIDs)
	}
	check := setfmt.AccessoryLibrary{Employees: list, Animations: anims}
	errs, warns := check.ValidateEmployees(known)
	aerrs, awarns := check.ValidateAnimations()
	errs, warns = append(errs, aerrs...), append(warns, awarns...)
	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "\n"))
	}
	dir := filepath.Join(a.root(), setups.GameStateDir)
	setup, err := customers.LoadLines(dir)
	if err != nil {
		return nil, err
	}
	setup.EmployeeLines, setup.EmployeeRemoved = lines, removed
	l.Lib.Employees = list
	l.Lib.Faces = faces
	syncFaces(l)
	l.Lib.Animations = anims
	l.Lib.MomentAnims, l.Lib.MomentOff = momentAnims, momentOff
	if err := l.Save(); err != nil {
		return nil, err
	}
	if err := customers.SaveLines(dir, setup); err != nil {
		return nil, err
	}
	if game.IsRunning() {
		warns = append(warns, "The game is running: it picks up the change at its next start.")
	}
	return warns, nil
}

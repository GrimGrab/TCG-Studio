package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/cardborder"
	"tcgstudio/internal/game"
	"tcgstudio/internal/gameextract"
	"tcgstudio/internal/ids"
	"tcgstudio/internal/setfmt"
)

// Borders page: card borders (the active setup's item library, content kind "border"): stacks of image and shader layers
// that sets give their border slots (set.json "borders", a card's "versionBorders"). The mod draws them over full-image
// cards (Runtime/CardBorders, Patches/CardBorderView).

// BordersPage is everything the page shows.
type BordersPage struct {
	Borders []setfmt.Border `json:"borders"`
	// Stock: the borders that come with the mod (read-only; a copy can be edited). Their pictures are served under
	// /stockborder/ (cardborder.StockURL).
	Stock []setfmt.Border `json:"stock"`
}

// BorderSaveResult is what Save reports.
type BorderSaveResult struct {
	Borders  []setfmt.Border `json:"borders"`
	Warnings []string        `json:"warnings"`
}

// LoadBordersPage reads the page's data for the active setup.
func (a *App) LoadBordersPage() (BordersPage, error) {
	page := BordersPage{Borders: []setfmt.Border{}, Stock: cardborder.Stock()}
	l, err := a.items()
	if err != nil {
		return page, err
	}
	page.Borders = append(page.Borders, l.Lib.Borders...)
	return page, nil
}

// BorderChoice is a border a slot or card version can be given: one of the setup's or a stock one (BuiltIn).
type BorderChoice struct {
	setfmt.Border
	BuiltIn bool `json:"builtIn"`
}

// ListBorders lists the borders the set editor's pickers and previews offer: the active setup's, then the stock ones.
func (a *App) ListBorders() ([]BorderChoice, error) {
	l, err := a.items()
	if err != nil {
		return nil, err
	}
	out := []BorderChoice{}
	for _, b := range l.Lib.Borders {
		out = append(out, BorderChoice{Border: b})
	}
	for _, b := range cardborder.Stock() {
		out = append(out, BorderChoice{Border: b, BuiltIn: true})
	}
	return out, nil
}

// NewBorderID returns an id no item of the setup uses.
func (a *App) NewBorderID() (string, error) {
	l, err := a.items()
	if err != nil {
		return "", err
	}
	return ids.Free(l.HasItem), nil
}

// PickBorderImage lets the player choose a picture for an image layer and copies it into the shared store. Returns its
// reference, "" when cancelled.
func (a *App) PickBorderImage() (string, error) {
	path, err := a.pickPNGFile("Choose a border picture (PNG with a see-through middle)")
	if err != nil || path == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(path), ".png") {
		return "", fmt.Errorf("border pictures must be PNG files: they need transparency for the card picture to show through")
	}
	l, err := a.items()
	if err != nil {
		return "", err
	}
	return l.Assets.PutFile(path)
}

// SaveBorders stores the page (Save): the whole list. Borders that left the list are kept in the shared catalog. Pictures
// sent as data: URLs (icons, painted layers) go into the shared store.
func (a *App) SaveBorders(list []setfmt.Border) (BorderSaveResult, error) {
	var res BorderSaveResult
	if a.artConverting.Load() {
		return res, errArtConverting
	}
	l, err := a.items()
	if err != nil {
		return res, err
	}
	store := func(what string, s *string) error {
		if !strings.HasPrefix(*s, "data:") {
			return nil
		}
		png, err := decodeDataURL(*s)
		if err != nil || len(png) < 8 || string(png[1:4]) != "PNG" {
			return fmt.Errorf("bad %s picture", what)
		}
		*s, err = l.PutBytes(png, ".png")
		return err
	}
	for i := range list {
		b := &list[i]
		b.Name = strings.TrimSpace(b.Name)
		if strings.HasPrefix(b.ID, setfmt.BorderStockPrefix) {
			return res, fmt.Errorf("border %q: its id belongs to the borders that come with the mod", b.Name)
		}
		if l.BorderIndex(b.ID) < 0 && l.HasItem(b.ID) {
			return res, fmt.Errorf("border %q: its id is used by another item", b.Name)
		}
		if err := store("preview", &b.Icon); err != nil {
			return res, fmt.Errorf("border %q: %w", b.Name, err)
		}
		if b.Layers == nil {
			b.Layers = []setfmt.BorderLayer{}
		}
		for j := range b.Layers {
			if err := store(fmt.Sprintf("layer %d", j+1), &b.Layers[j].File); err != nil {
				return res, fmt.Errorf("border %q: %w", b.Name, err)
			}
		}
	}
	if issues := setfmt.ValidateBorders(list); len(issues) > 0 {
		var msgs []string
		for _, is := range issues {
			msgs = append(msgs, is.Where+": "+is.Message)
		}
		return res, errors.New(strings.Join(msgs, "\n"))
	}

	kept := map[string]bool{}
	for _, b := range list {
		kept[b.ID] = true
	}
	var gone []string
	for _, b := range l.Lib.Borders {
		if !kept[b.ID] {
			gone = append(gone, b.ID)
		}
	}
	if len(gone) > 0 {
		if err := a.keepInCatalog("", gone, l); err != nil {
			return res, err
		}
	}
	l.Lib.Borders = list
	if _, err := a.saveLibrary(l); err != nil {
		return res, err
	}
	if game.IsRunning() {
		res.Warnings = append(res.Warnings, "The game is running: it picks up the change at its next start.")
	}
	res.Borders = list
	return res, nil
}

// CardFrames lists the game's border rims read from the game files (templates "cardframes"), for the previews of slots
// without a border of their own. Empty while Studio hasn't read the game files (or with an older read).
func (a *App) CardFrames() []gameextract.CardFrame {
	out := []gameextract.CardFrame{}
	b, err := os.ReadFile(filepath.Join(a.templatesDir(), gameextract.CardFramesDir, gameextract.CardFramesFile))
	if err == nil {
		_ = json.Unmarshal(b, &out)
	}
	return out
}

// EditionLabelLayout is where and how the game prints a card's edition label (templates "cardframes"), for the card
// previews; nil while Studio hasn't read the game files (or with an older read).
func (a *App) EditionLabelLayout() *gameextract.EditionLabel {
	b, err := os.ReadFile(filepath.Join(a.templatesDir(), gameextract.CardFramesDir, gameextract.EditionLabelFile))
	if err != nil {
		return nil
	}
	var l gameextract.EditionLabel
	if json.Unmarshal(b, &l) != nil {
		return nil
	}
	return &l
}

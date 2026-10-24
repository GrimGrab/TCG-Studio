package main

import (
	"errors"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/accessories"
	astore "tcgstudio/internal/assets"
	"tcgstudio/internal/game"
	"tcgstudio/internal/gamify"
	"tcgstudio/internal/origin"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/uvmap"
)

// ---------------------------------------------------------------- accessory library (custom deck boxes / playmats)
//
// The library belongs to the setup (not to a set). The game reads it from the active setup at its next start, so saving is
// all a change takes (saveLibrary).

// AccessoryView is the library as the frontend sees it; layouts are the editor's own JSON, kept as strings.
type AccessoryView struct {
	Accessories []setfmt.Accessory       `json:"accessories"`
	Furniture   []setfmt.Furniture       `json:"furniture"`
	Decorations []setfmt.Decoration      `json:"decorations"`
	Layouts     map[string]string        `json:"layouts"`
	Origins     map[string]origin.Origin `json:"origins"` // converted accessories/furniture: where they came from
	Installed   bool                     `json:"installed"`
	Warnings    []string                 `json:"warnings"`
	Errors      []string                 `json:"errors"`
}

// store is the workspace's shared asset store (accessory and furniture files; setups keep only the game info).
func (a *App) store() astore.Store { return a.wsDir().Store() }

func (a *App) accView(l *accessories.Library) AccessoryView {
	v := AccessoryView{Accessories: l.Lib.Accessories, Furniture: l.Lib.Furniture, Decorations: l.Lib.Decorations, Layouts: map[string]string{},
		Origins: l.Meta.Origins}
	if v.Furniture == nil {
		v.Furniture = []setfmt.Furniture{}
	}
	if v.Decorations == nil {
		v.Decorations = []setfmt.Decoration{}
	}
	for id, raw := range l.Meta.Layouts {
		v.Layouts[id] = string(raw)
	}
	v.Errors, v.Warnings = l.Lib.Validate(l.Resolve)
	fe, fw := l.Lib.ValidateFurniture(l.Resolve, a.furnitureBases())
	v.Errors, v.Warnings = append(v.Errors, fe...), append(v.Warnings, fw...)
	de, dw := l.Lib.ValidateDecorations(l.Resolve)
	v.Errors, v.Warnings = append(v.Errors, de...), append(v.Warnings, dw...)
	if gameDir := a.gameDir(); game.IsGameDir(gameDir) { // the game reads this setup's library and the store (game.StudioPathsFile)
		sp := game.ReadStudioPaths(gameDir)
		v.Installed = game.SameDir(sp.Setup, a.root()) && game.SameDir(sp.Assets, a.store().Dir)
	}
	if v.Warnings == nil {
		v.Warnings = []string{}
	}
	if v.Errors == nil {
		v.Errors = []string{}
	}
	return v
}

// saveLibrary writes the library; the game reads it from the active setup (pointGameAtWorkspace keeps it pointed there).
func (a *App) saveLibrary(l *accessories.Library) (AccessoryView, error) {
	if err := l.Save(); err != nil {
		return AccessoryView{}, err
	}
	a.pointGameAtWorkspace()
	return a.accView(l), nil
}

func (a *App) Accessories() (AccessoryView, error) {
	l, err := a.items()
	if err != nil {
		return AccessoryView{}, err
	}
	return a.accView(l), nil
}

// AccessoryKinds lists the accessory kinds (tabs), their vanilla bases and mod toggles.
func (a *App) AccessoryKinds() []setfmt.AccessoryKind { return setfmt.AccessoryKinds }

// AccessoryShops lists the shop lists an accessory can be sold from instead of its base item's (value → label).
func (a *App) AccessoryShops() map[string]string { return setfmt.AccessoryShops }

// AccessoryTemplates returns accessories.json of the game templates (items, meshes, textures, license rows, shelves; see
// app_templates.go) as raw JSON, or "" while they aren't available.
func (a *App) AccessoryTemplates() string { return a.templateJSON("accessories", "accessories.json") }

// AccessoryModel returns the editable face layout (net + texture mapping) of a kind's model.
func (a *App) AccessoryModel(kind string) (uvmap.Model, error) {
	m, ok := uvmap.ModelFor(kind)
	if !ok {
		return m, errors.New("unknown accessory kind " + kind)
	}
	return m, nil
}

// AccessoryModelFor returns the layout of an accessory's model: per base for the tabletop kinds (every board game and card supply
// has its own mesh), the kind's otherwise.
func (a *App) AccessoryModelFor(kind, base string) (uvmap.Model, error) {
	if base == "" {
		if k, ok := setfmt.KindInfo(kind); ok && len(k.Bases) > 0 {
			base = k.Bases[0]
		}
	}
	m, ok := uvmap.ModelForBase(kind, base)
	if !ok {
		return m, errors.New("no editable layout for " + kind + " " + base)
	}
	return m, nil
}

// NewAccessory makes a draft with a fresh id; nothing is stored until the page's Save (SaveLibraryDraft).
func (a *App) NewAccessory(kind, name, base string) (setfmt.Accessory, error) {
	l, err := a.items()
	if err != nil {
		return setfmt.Accessory{}, err
	}
	k, ok := setfmt.KindInfo(kind)
	if !ok {
		return setfmt.Accessory{}, errors.New("unknown accessory kind " + kind)
	}
	if strings.TrimSpace(name) == "" {
		name = "Custom " + strings.Title(k.One) //nolint:staticcheck // ASCII names
	}
	return setfmt.NewAccessory(kind, l.NewID(), name, base), nil
}

func (a *App) MoveAccessory(id string, to int) (AccessoryView, error) {
	l, err := a.items()
	if err != nil {
		return AccessoryView{}, err
	}
	l.Move(id, to)
	return a.saveLibrary(l)
}

// PickAccessoryImage lets the user choose an image for the editor and copies it into the library (images/src/…).
func (a *App) PickAccessoryImage() (string, error) {
	file, err := a.pickImageFile("Choose an image")
	if err != nil || file == "" {
		return "", err
	}
	l, err := a.items()
	if err != nil {
		return "", err
	}
	return l.CopySource(file)
}

// SaveTemplateImage asks where to save a painting template (PNG data URL from the editor) and writes it. Returns the path,
// "" when cancelled.
func (a *App) SaveTemplateImage(name, dataURL string) (string, error) {
	b, err := decodeDataURL(dataURL)
	if err != nil {
		return "", err
	}
	file, err := a.saveFileAs("Save painting template", safeFileName(strings.TrimSuffix(name, ".png"))+".png", ".png",
		[]runtime.FileFilter{{DisplayName: "PNG image (*.png)", Pattern: "*.png"}})
	if err != nil || file == "" {
		return "", err
	}
	return file, os.WriteFile(file, b, 0o644)
}

// SaveAccessorySourceImage stores an image made in the studio (data URL, e.g. a straightened photo) with the library's source
// images; returns its path.
func (a *App) SaveAccessorySourceImage(name, dataURL string) (string, error) {
	b, err := decodeDataURL(dataURL)
	if err != nil {
		return "", err
	}
	l, err := a.items()
	if err != nil {
		return "", err
	}
	return l.WriteSource(name, b)
}

func (a *App) InstallAccessories() (AccessoryView, error) {
	l, err := a.items()
	if err != nil {
		return AccessoryView{}, err
	}
	if !game.IsGameDir(a.gameDir()) {
		return a.accView(l), errNoGame
	}
	return a.saveLibrary(l)
}

// ---------------------------------------------------------------- accessory gamify (one ladder, library order)

func (a *App) GamifyAccessoryDefaults() gamify.AccessorySettings {
	return gamify.DefaultAccessorySettings()
}

func (a *App) GamifyAccessoryPreview(s gamify.AccessorySettings) ([]gamify.AccessoryPreview, error) {
	l, err := a.items()
	if err != nil {
		return nil, err
	}
	list := append([]setfmt.Accessory(nil), l.Lib.Accessories...)
	return gamify.ApplyAccessories(list, s), nil
}

func (a *App) GamifyAccessoryApply(s gamify.AccessorySettings) ([]gamify.AccessoryPreview, error) {
	l, err := a.items()
	if err != nil {
		return nil, err
	}
	rows := gamify.ApplyAccessories(l.Lib.Accessories, s)
	_, err = a.saveLibrary(l)
	return rows, err
}

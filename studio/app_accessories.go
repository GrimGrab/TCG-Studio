package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	astore "tcgstudio/internal/assets"
	"tcgstudio/internal/origin"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/game"
	"tcgstudio/internal/gamify"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/uvmap"
)

// ---------------------------------------------------------------- accessory library (custom deck boxes / playmats)
//
// The library is global (not part of a set). Every change is saved to the workspace and, when the game folder is set,
// installed to <plugin>\Accessories right away (the mod reads it at game start).

// AccessoryView is the library as the frontend sees it; layouts are the editor's own JSON, kept as strings.
type AccessoryView struct {
	Accessories []setfmt.Accessory       `json:"accessories"`
	Furniture   []setfmt.Furniture       `json:"furniture"`
	Layouts     map[string]string        `json:"layouts"`
	Origins     map[string]origin.Origin `json:"origins"` // converted accessories/furniture: where they came from
	Installed   bool                     `json:"installed"`
	Warnings    []string                 `json:"warnings"`
	Errors      []string                 `json:"errors"`
}

func (a *App) accLib() (*accessories.Library, error) { return accessories.Open(a.root(), a.store()) }

// store is the workspace's shared asset store (accessory and furniture files; setups keep only the game info).
func (a *App) store() astore.Store { return astore.For(a.settings.Workspace) }

func (a *App) accView(l *accessories.Library) AccessoryView {
	v := AccessoryView{Accessories: l.Lib.Accessories, Furniture: l.Lib.Furniture, Layouts: map[string]string{},
		Origins: map[string]origin.Origin{}}
	for id, o := range l.Meta.Origins {
		v.Origins[id] = o
	}
	// Converted before origins were recorded: at least say they came from a mod.
	for _, a := range l.Lib.Accessories {
		if _, ok := v.Origins[a.ID]; !ok && accessories.IsConverted(a.ID) {
			v.Origins[a.ID] = origin.Origin{Kind: "EPL mod", Mod: "an EPL mod"}
		}
	}
	for _, f := range l.Lib.Furniture {
		if _, ok := v.Origins[f.ID]; !ok && accessories.IsConverted(f.ID) {
			v.Origins[f.ID] = origin.Origin{Kind: "EPL mod", Mod: "an EPL mod"}
		}
	}
	if v.Furniture == nil {
		v.Furniture = []setfmt.Furniture{}
	}
	for id, raw := range l.Meta.Layouts {
		v.Layouts[id] = string(raw)
	}
	v.Errors, v.Warnings = l.Lib.Validate(l.Resolve)
	fe, fw := l.Lib.ValidateFurniture(l.Resolve, a.furnitureBases())
	v.Errors, v.Warnings = append(v.Errors, fe...), append(v.Warnings, fw...)
	if game.IsGameDir(a.settings.GameDir) {
		_, err := os.Stat(filepath.Join(accessories.InstalledDir(a.settings.GameDir), accessories.LibraryFile))
		v.Installed = err == nil
	}
	if v.Warnings == nil {
		v.Warnings = []string{}
	}
	if v.Errors == nil {
		v.Errors = []string{}
	}
	return v
}

// saveAndInstall writes the library and mirrors it into the game when a game folder is known.
func (a *App) saveAndInstall(l *accessories.Library) (AccessoryView, error) {
	if err := l.Save(); err != nil {
		return AccessoryView{}, err
	}
	if game.IsGameDir(a.settings.GameDir) {
		if err := l.Install(a.settings.GameDir); err != nil {
			return a.accView(l), errors.New("saved, but installing into the game failed: " + err.Error())
		}
	}
	return a.accView(l), nil
}

func (a *App) Accessories() (AccessoryView, error) {
	l, err := a.accLib()
	if err != nil {
		return AccessoryView{}, err
	}
	return a.accView(l), nil
}

// AccessoryKinds lists the accessory kinds (tabs), their vanilla bases and mod toggles.
func (a *App) AccessoryKinds() []setfmt.AccessoryKind { return setfmt.AccessoryKinds }

// AccessoryTemplates returns accessories.json of the game templates (items, meshes, textures, license rows, shelves; see
// app_templates.go) as raw JSON, or "" while they aren't available.
func (a *App) AccessoryTemplates() string {
	if !game.IsGameDir(a.settings.GameDir) {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(a.templatesDir(), "accessories", "accessories.json"))
	if err != nil {
		return ""
	}
	return string(b)
}

// AccessoryModel returns the editable face layout (net + texture mapping) of a kind's model.
func (a *App) AccessoryModel(kind string) (uvmap.Model, error) {
	m, ok := uvmap.ModelFor(kind)
	if !ok {
		return m, errors.New("unknown accessory kind " + kind)
	}
	return m, nil
}

func (a *App) NewAccessory(kind, name, base string) (setfmt.Accessory, error) {
	l, err := a.accLib()
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
	acc := setfmt.NewAccessory(kind, l.NewID(name), name, base)
	l.Put(acc)
	_, err = a.saveAndInstall(l)
	return acc, err
}

// SaveAccessory stores an accessory's fields and editor layout. texturePNG / iconPNG are data URLs (or raw base64) of the
// composed images from the editor; empty keeps the current files.
func (a *App) SaveAccessory(acc setfmt.Accessory, layout string, texturePNG, iconPNG string) (AccessoryView, error) {
	l, err := a.accLib()
	if err != nil {
		return AccessoryView{}, err
	}
	if !setfmt.SafeID(acc.ID) {
		return AccessoryView{}, errors.New("bad accessory id")
	}
	for _, img := range []struct {
		data, suffix string
		dst          *string
	}{{texturePNG, "texture", &acc.Texture}, {iconPNG, "icon", &acc.Icon}} {
		if img.data == "" {
			continue
		}
		b, err := decodeDataURL(img.data)
		if err != nil {
			return AccessoryView{}, err
		}
		rel, err := l.WriteImage(acc.ID, img.suffix, b)
		if err != nil {
			return AccessoryView{}, err
		}
		*img.dst = rel
	}
	if layout != "" {
		if !json.Valid([]byte(layout)) {
			return AccessoryView{}, errors.New("layout is not valid JSON")
		}
		l.Meta.Layouts[acc.ID] = json.RawMessage(layout)
	}
	l.Put(acc)
	return a.saveAndInstall(l)
}

func (a *App) DeleteAccessory(id string) (AccessoryView, error) {
	l, err := a.accLib()
	if err != nil {
		return AccessoryView{}, err
	}
	if err := a.keepInCatalog("", []string{id}, l); err != nil {
		return AccessoryView{}, err
	}
	l.Delete(id)
	return a.saveAndInstall(l)
}

// DeleteAccessories removes several accessories at once (one save and install).
func (a *App) DeleteAccessories(ids []string) (AccessoryView, error) {
	l, err := a.accLib()
	if err != nil {
		return AccessoryView{}, err
	}
	if err := a.keepInCatalog("", ids, l); err != nil {
		return AccessoryView{}, err
	}
	for _, id := range ids {
		l.Delete(id)
	}
	return a.saveAndInstall(l)
}

func (a *App) MoveAccessory(id string, to int) (AccessoryView, error) {
	l, err := a.accLib()
	if err != nil {
		return AccessoryView{}, err
	}
	l.Move(id, to)
	return a.saveAndInstall(l)
}

// PickAccessoryImage lets the user choose an image for the editor and copies it into the library (images/src/…).
func (a *App) PickAccessoryImage() (string, error) {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose an image", Filters: imageFilters})
	if err != nil || file == "" {
		return "", err
	}
	l, err := a.accLib()
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
	file, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save painting template",
		Filters:         []runtime.FileFilter{{DisplayName: "PNG image (*.png)", Pattern: "*.png"}},
		DefaultFilename: safeFileName(strings.TrimSuffix(name, ".png")) + ".png",
	})
	if err != nil || file == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(file), ".png") {
		file += ".png"
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
	l, err := a.accLib()
	if err != nil {
		return "", err
	}
	return l.WriteSource(name, b)
}

func (a *App) InstallAccessories() (AccessoryView, error) {
	l, err := a.accLib()
	if err != nil {
		return AccessoryView{}, err
	}
	if !game.IsGameDir(a.settings.GameDir) {
		return a.accView(l), errors.New("game folder not set (Settings → Game)")
	}
	return a.saveAndInstall(l)
}

func (a *App) OpenAccessoriesFolder() {
	l, err := a.accLib()
	if err == nil {
		_ = os.MkdirAll(l.Folder, 0o755)
		_ = exec.Command("explorer", l.Folder).Start()
	}
}

// ---------------------------------------------------------------- accessory gamify (one ladder, library order)

func (a *App) GamifyAccessoryDefaults() gamify.AccessorySettings {
	return gamify.DefaultAccessorySettings()
}

func (a *App) GamifyAccessoryPreview(s gamify.AccessorySettings) ([]gamify.AccessoryPreview, error) {
	l, err := a.accLib()
	if err != nil {
		return nil, err
	}
	list := append([]setfmt.Accessory(nil), l.Lib.Accessories...)
	return gamify.ApplyAccessories(list, s), nil
}

func (a *App) GamifyAccessoryApply(s gamify.AccessorySettings) ([]gamify.AccessoryPreview, error) {
	l, err := a.accLib()
	if err != nil {
		return nil, err
	}
	rows := gamify.ApplyAccessories(l.Lib.Accessories, s)
	_, err = a.saveAndInstall(l)
	return rows, err
}

func decodeDataURL(s string) ([]byte, error) {
	if i := strings.Index(s, ","); strings.HasPrefix(s, "data:") && i >= 0 {
		s = s[i+1:]
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, errors.New("bad image data: " + err.Error())
	}
	return b, nil
}

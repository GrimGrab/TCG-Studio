package main

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/fontname"
	"tcgstudio/internal/game"
	"tcgstudio/internal/gameextract"
	"tcgstudio/internal/modconfig"
)

// ---------------------------------------------------------------- shop sign (Settings page)
//
// The sign above the shop entrance. Stored as mod settings ([Visuals] ShopSignImage = a file name in <plugin>\ShopSign,
// ShopSignCrop = the part shown, ShowShopNameOnSign), so a running game applies changes live; settings-meta.json
// "elsewhere" keeps them off the Mod settings page.

const (
	shopSignSection  = "Visuals"
	shopSignImageKey = "ShopSignImage"
	shopSignNameKey  = "ShowShopNameOnSign"
	shopSignCropKey  = "ShopSignCrop"
	shopSignAspect   = 4.11 // the sign face, measured from the game files (docs: runtime-facts "Shop sign")

	// [Visuals - Shop sign text]: the shop name's font, colour, size and outline.
	shopSignTextSection = "Visuals - Shop sign text"
	shopSignFontKey     = "ShopSignFont"
	shopSignColorKey    = "ShopSignTextColor"
	shopSignSizeKey     = "ShopSignTextSize"
	shopSignOutlineKey  = "ShopSignOutline"
	shopSignOutColorKey = "ShopSignOutlineColor"
)

// ShopSignView is the sign's current state for the Settings page.
type ShopSignView struct {
	Image    string       `json:"image"` // URL of the chosen image, "" = the vanilla sign
	Crop     []float64    `json:"crop"`  // x, y, w, h as fractions of the image (top-left origin); nil = the middle
	ShowName bool         `json:"showName"`
	Aspect   float64      `json:"aspect"` // width ÷ height of the sign face (mod: ShopSign.FaceAspect)
	Text     ShopSignText `json:"text"`
	Layout   string       `json:"layout"`   // URL of the game's sign.json (preview layout, default font, game fonts); "" = not read yet
	ShopName string       `json:"shopName"` // the shop's name in the newest save, for the preview
}

// ShopSignText is the shop name's style ([Visuals - Shop sign text]).
type ShopSignText struct {
	Font         string  `json:"font"`    // display name of the chosen font, "" = the game's own
	FontURL      string  `json:"fontUrl"` // the chosen font file for the preview
	Color        string  `json:"color"`   // #RRGGBB, "" = the game's gold
	Size         float64 `json:"size"`
	Outline      float64 `json:"outline"` // fraction of the font size
	OutlineColor string  `json:"outlineColor"`
}

func (a *App) shopSignDir() string {
	return filepath.Join(game.PluginDir(a.settings.GameDir), modconfig.ShopSignFolder)
}

func (a *App) ShopSign() (ShopSignView, error) {
	p, err := a.modConfigPath()
	if err != nil {
		return ShopSignView{}, err
	}
	img, err := modconfig.Get(p, shopSignSection, shopSignImageKey)
	if err != nil {
		return ShopSignView{}, err
	}
	name, err := modconfig.Get(p, shopSignSection, shopSignNameKey)
	if err != nil {
		return ShopSignView{}, err
	}
	v := ShopSignView{ShowName: name.Value != "false", Aspect: shopSignAspect}
	if c, err := modconfig.Get(p, shopSignSection, shopSignCropKey); err == nil {
		v.Crop = parseCrop(c.Value)
	}
	v.Image = a.shopSignURL(img.Value)
	get := func(key string) string {
		e, _ := modconfig.Get(p, shopSignTextSection, key)
		return e.Value
	}
	v.Text = ShopSignText{Color: get(shopSignColorKey), OutlineColor: get(shopSignOutColorKey), Size: 1}
	if f, err := strconv.ParseFloat(get(shopSignSizeKey), 64); err == nil {
		v.Text.Size = f
	}
	if f, err := strconv.ParseFloat(get(shopSignOutlineKey), 64); err == nil {
		v.Text.Outline = f
	}
	if font := get(shopSignFontKey); font != "" {
		if v.Text.FontURL = a.shopSignURL(font); v.Text.FontURL != "" {
			v.Text.Font = fontname.File(filepath.Join(a.shopSignDir(), font))
		}
		if v.Text.Font == "" {
			v.Text.Font = strings.TrimSuffix(font, filepath.Ext(font))
		}
	}
	if exists(filepath.Join(a.templatesDir(), gameextract.ShopSignDir, "sign.json")) {
		v.Layout = "/signtemplates/sign.json"
	}
	v.ShopName = newestShopName()
	return v, nil
}

// shopSignURL serves a file of <plugin>\ShopSign to the page ("" when missing); the mtime query busts the cache on a replace.
func (a *App) shopSignURL(name string) string {
	if name == "" || filepath.Base(name) != name {
		return ""
	}
	info, err := os.Stat(filepath.Join(a.shopSignDir(), name))
	if err != nil {
		return ""
	}
	return "/shopsign/" + url.PathEscape(name) + "?v=" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
}

// newestShopName is m_PlayerName (the shop's name) of the most recently written save, "" without saves.
func newestShopName() string {
	files, _ := filepath.Glob(filepath.Join(game.SaveDir(), "savedGames_Release*.json"))
	var newest string
	var at time.Time
	for _, f := range files {
		if info, err := os.Stat(f); err == nil && info.ModTime().After(at) {
			newest, at = f, info.ModTime()
		}
	}
	if newest == "" {
		return ""
	}
	b, err := os.ReadFile(newest)
	if err != nil {
		return ""
	}
	var save struct {
		Name string `json:"m_PlayerName"`
	}
	if json.Unmarshal(b, &save) != nil {
		return ""
	}
	return save.Name
}

// ShopSignFont is one font offered for the shop name.
type ShopSignFont struct {
	Source string `json:"source"` // "game" (from the game files) or "windows" (installed)
	ID     string `json:"id"`     // game: file under the templates' shopsign folder; windows: full path
	Name   string `json:"name"`
}

// ShopSignFonts lists the game's fonts and the fonts installed on Windows (.ttf/.otf; collections can't be loaded by the mod).
func (a *App) ShopSignFonts() []ShopSignFont {
	out := []ShopSignFont{}
	var lay struct {
		Fonts []struct{ Name, File string } `json:"fonts"`
	}
	if b, err := os.ReadFile(filepath.Join(a.templatesDir(), gameextract.ShopSignDir, "sign.json")); err == nil && json.Unmarshal(b, &lay) == nil {
		for _, f := range lay.Fonts {
			out = append(out, ShopSignFont{Source: "game", ID: f.File, Name: f.Name})
		}
	}
	for _, f := range installedFonts() {
		out = append(out, ShopSignFont{Source: "windows", ID: f.path, Name: f.name})
	}
	return out
}

// SetShopSignFont picks the shop name's font: source "" = the game's own, "game"/"windows" = an entry of ShopSignFonts,
// "file" = a .ttf/.otf the user chooses. The font is copied into <plugin>\ShopSign so it travels with setups.
func (a *App) SetShopSignFont(source, id string) (ShopSignView, error) {
	p, err := a.modConfigPath()
	if err != nil {
		return ShopSignView{}, err
	}
	var from string
	switch source {
	case "":
	case "game":
		if strings.Contains(id, "..") || !strings.HasPrefix(id, "fonts/") {
			return ShopSignView{}, errors.New("unknown game font")
		}
		from = filepath.Join(a.templatesDir(), gameextract.ShopSignDir, filepath.FromSlash(id))
	case "windows":
		for _, f := range installedFonts() {
			if strings.EqualFold(f.path, id) {
				from = f.path
			}
		}
		if from == "" {
			return ShopSignView{}, errors.New("that font isn't installed")
		}
	case "file":
		from, err = runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose a font for the shop sign",
			Filters: []runtime.FileFilter{{DisplayName: "Fonts (*.ttf, *.otf)", Pattern: "*.ttf;*.otf"}}})
		if err != nil || from == "" {
			return a.ShopSign()
		}
	default:
		return ShopSignView{}, errors.New("unknown font source")
	}
	name := ""
	if from != "" {
		b, err := os.ReadFile(from)
		if err != nil {
			return ShopSignView{}, err
		}
		if err := fontname.Check(b); err != nil {
			return ShopSignView{}, err
		}
		if err := os.MkdirAll(a.shopSignDir(), 0o755); err != nil {
			return ShopSignView{}, err
		}
		name = "font_" + filepath.Base(from)
		if err := os.WriteFile(filepath.Join(a.shopSignDir(), name), b, 0o644); err != nil {
			return ShopSignView{}, err
		}
	}
	if old, err := modconfig.Get(p, shopSignTextSection, shopSignFontKey); err == nil && old.Value != "" && old.Value != name && filepath.Base(old.Value) == old.Value {
		_ = os.Remove(filepath.Join(a.shopSignDir(), old.Value)) // the previous font
	}
	if _, err := modconfig.Set(p, shopSignTextSection, shopSignFontKey, name); err != nil {
		return ShopSignView{}, err
	}
	return a.ShopSign()
}

// SetShopSignText stores the shop name's colour ("" = the game's gold), size, outline and outline colour.
func (a *App) SetShopSignText(t ShopSignText) (ShopSignView, error) {
	p, err := a.modConfigPath()
	if err != nil {
		return ShopSignView{}, err
	}
	hex := func(s string) (string, error) {
		s = strings.TrimSpace(s)
		if s == "" {
			return "", nil
		}
		if !strings.HasPrefix(s, "#") {
			s = "#" + s
		}
		if len(s) != 7 || strings.Trim(strings.ToLower(s[1:]), "0123456789abcdef") != "" {
			return "", errors.New("colours are #RRGGBB")
		}
		return strings.ToUpper(s), nil
	}
	color, err := hex(t.Color)
	if err != nil {
		return ShopSignView{}, err
	}
	outColor, err := hex(t.OutlineColor)
	if err != nil {
		return ShopSignView{}, err
	}
	if outColor == "" {
		outColor = "#000000"
	}
	num := func(f, lo, hi float64) string { return strconv.FormatFloat(min(max(f, lo), hi), 'f', -1, 64) }
	for _, kv := range [][2]string{{shopSignColorKey, color}, {shopSignSizeKey, num(t.Size, 0.5, 2)},
		{shopSignOutlineKey, num(t.Outline, 0, 0.3)}, {shopSignOutColorKey, outColor}} {
		if _, err := modconfig.Set(p, shopSignTextSection, kv[0], kv[1]); err != nil {
			return ShopSignView{}, err
		}
	}
	return a.ShopSign()
}

// ChooseShopSign lets the user pick a banner image, copies it into <plugin>\ShopSign and selects it.
func (a *App) ChooseShopSign() (ShopSignView, error) {
	p, err := a.modConfigPath()
	if err != nil {
		return ShopSignView{}, err
	}
	file, err := a.pickImageFile("Choose the shop sign image")
	if err != nil || file == "" {
		return a.ShopSign()
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return ShopSignView{}, err
	}
	if err := os.MkdirAll(a.shopSignDir(), 0o755); err != nil {
		return ShopSignView{}, err
	}
	name := filepath.Base(file)
	if err := os.WriteFile(filepath.Join(a.shopSignDir(), name), b, 0o644); err != nil {
		return ShopSignView{}, err
	}
	if old, err := modconfig.Get(p, shopSignSection, shopSignImageKey); err == nil && old.Value != "" && old.Value != name && filepath.Base(old.Value) == old.Value {
		_ = os.Remove(filepath.Join(a.shopSignDir(), old.Value)) // the previous image
	}
	if _, err := modconfig.Set(p, shopSignSection, shopSignCropKey, ""); err != nil { // a new picture starts centred
		return ShopSignView{}, err
	}
	if _, err := modconfig.Set(p, shopSignSection, shopSignImageKey, name); err != nil {
		return ShopSignView{}, err
	}
	return a.ShopSign()
}

// RemoveShopSign goes back to the vanilla sign.
func (a *App) RemoveShopSign() (ShopSignView, error) {
	p, err := a.modConfigPath()
	if err != nil {
		return ShopSignView{}, err
	}
	old, _ := modconfig.Get(p, shopSignSection, shopSignImageKey)
	if _, err := modconfig.Set(p, shopSignSection, shopSignImageKey, ""); err != nil {
		return ShopSignView{}, err
	}
	if _, err := modconfig.Set(p, shopSignSection, shopSignCropKey, ""); err != nil {
		return ShopSignView{}, err
	}
	if old.Value != "" && filepath.Base(old.Value) == old.Value {
		_ = os.Remove(filepath.Join(a.shopSignDir(), old.Value)) // the image only: a chosen font stays
	}
	return a.ShopSign()
}

func (a *App) SetShopSignShowName(show bool) (ShopSignView, error) {
	p, err := a.modConfigPath()
	if err != nil {
		return ShopSignView{}, err
	}
	if _, err := modconfig.Set(p, shopSignSection, shopSignNameKey, strconv.FormatBool(show)); err != nil {
		return ShopSignView{}, err
	}
	return a.ShopSign()
}

// SetShopSignCrop stores the part of the image shown on the sign (fractions of the image, top-left origin; the crop box keeps
// the sign's shape). An empty slice goes back to the middle.
func (a *App) SetShopSignCrop(crop []float64) (ShopSignView, error) {
	p, err := a.modConfigPath()
	if err != nil {
		return ShopSignView{}, err
	}
	value := ""
	if len(crop) == 4 {
		parts := make([]string, 4)
		for i, f := range crop {
			parts[i] = strconv.FormatFloat(min(max(f, 0), 1), 'f', 4, 64)
		}
		value = strings.Join(parts, ",")
	} else if len(crop) != 0 {
		return ShopSignView{}, errors.New("crop needs x, y, width and height")
	}
	if _, err := modconfig.Set(p, shopSignSection, shopSignCropKey, value); err != nil {
		return ShopSignView{}, err
	}
	return a.ShopSign()
}

// parseCrop reads "x,y,w,h" (nil when empty or invalid — the mod then uses the middle too).
func parseCrop(s string) []float64 {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return nil
	}
	out := make([]float64, 4)
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil
		}
		out[i] = f
	}
	return out
}

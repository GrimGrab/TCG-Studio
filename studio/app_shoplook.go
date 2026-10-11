package main

import (
	"encoding/base64"
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

// ---------------------------------------------------------------- Shop look page
//
// The shop sign above the entrance, the phone wallpaper and the open/closed sign at the door. Stored as mod settings (a picture
// = an image file name in <plugin>\ShopSign + the part shown), so a running game applies changes live; settings-meta.json
// "elsewhere" keeps them off the Mod settings page.

const (
	shopSignSection = "Visuals"
	shopSignNameKey = "ShowShopNameOnSign"
	shopLookSection = "Visuals - Shop look"

	// [Visuals - Shop sign text]: the shop name's font, colour, size and outline.
	shopSignTextSection = "Visuals - Shop sign text"
	shopSignFontKey     = "ShopSignFont"
	shopSignColorKey    = "ShopSignTextColor"
	shopSignSizeKey     = "ShopSignTextSize"
	shopSignOutlineKey  = "ShopSignOutline"
	shopSignOutColorKey = "ShopSignOutlineColor"
)

// shopPicture is one picture slot of the page. Aspects (width ÷ height of what the game shows) are measured from the game files
// (docs: runtime-facts "Shop sign", "Phone wallpaper and open/closed sign"; mod ShopSign.FaceAspect, PhoneWallpaper.ScreenAspect,
// DoorSign.BoardAspect).
type shopPicture struct {
	section, imageKey, cropKey string
	prefix                     string // of the copied file, so slots never share one
	title                      string // of the file dialog
	aspect                     float64
}

var shopPictures = map[string]shopPicture{
	"sign":      {shopSignSection, "ShopSignImage", "ShopSignCrop", "sign_", "Choose the shop sign image", 4.11},
	"wallpaper": {shopLookSection, "PhoneWallpaperImage", "PhoneWallpaperCrop", "wallpaper_", "Choose the phone wallpaper", 24.63 / 41.896},
	"open":      {shopLookSection, "OpenSignImage", "OpenSignCrop", "open_", "Choose the OPEN side of the door sign", 328.0 / 206},
	"closed":    {shopLookSection, "ClosedSignImage", "ClosedSignCrop", "closed_", "Choose the CLOSED side of the door sign", 328.0 / 206},
}

// ShopLookView is the page's state.
type ShopLookView struct {
	Pictures map[string]ShopPictureView `json:"pictures"` // by slot (shopPictures)
	Sign     ShopSignView               `json:"sign"`
	// URLs of the game's layouts for the previews (templates shopsign/); "" = not read yet.
	SignLayout  string `json:"signLayout"`
	PhoneLayout string `json:"phoneLayout"`
	DoorLayout  string `json:"doorLayout"`
}

type ShopPictureView struct {
	Image  string    `json:"image"`  // URL of the chosen image, "" = vanilla
	Crop   []float64 `json:"crop"`   // x, y, w, h as fractions of the image (top-left origin); nil = the middle
	Aspect float64   `json:"aspect"` // width ÷ height of the crop box
	Design string    `json:"design"` // a side drawn from text: its design (JSON, see SaveShopPictureText); "" = a picture
}

// ShopSignView is the shop name on the sign.
type ShopSignView struct {
	ShowName bool         `json:"showName"`
	Text     ShopSignText `json:"text"`
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

func (a *App) ShopLook() (ShopLookView, error) {
	p, err := a.modConfigPath()
	if err != nil {
		return ShopLookView{}, err
	}
	v := ShopLookView{Pictures: map[string]ShopPictureView{}}
	for slot, pic := range shopPictures {
		img, err := modconfig.Get(p, pic.section, pic.imageKey)
		if err != nil {
			return ShopLookView{}, err
		}
		pv := ShopPictureView{Image: a.shopSignURL(img.Value), Aspect: pic.aspect}
		if pv.Image != "" {
			if b, err := os.ReadFile(filepath.Join(a.shopSignDir(), designFile(img.Value))); err == nil {
				pv.Design = string(b)
			}
		}
		if c, err := modconfig.Get(p, pic.section, pic.cropKey); err == nil {
			pv.Crop = parseCrop(c.Value)
		}
		v.Pictures[slot] = pv
	}
	name, err := modconfig.Get(p, shopSignSection, shopSignNameKey)
	if err != nil {
		return ShopLookView{}, err
	}
	get := func(key string) string {
		e, _ := modconfig.Get(p, shopSignTextSection, key)
		return e.Value
	}
	v.Sign = ShopSignView{ShowName: name.Value != "false", ShopName: newestShopName(),
		Text: ShopSignText{Color: get(shopSignColorKey), OutlineColor: get(shopSignOutColorKey), Size: 1}}
	if f, err := strconv.ParseFloat(get(shopSignSizeKey), 64); err == nil {
		v.Sign.Text.Size = f
	}
	if f, err := strconv.ParseFloat(get(shopSignOutlineKey), 64); err == nil {
		v.Sign.Text.Outline = f
	}
	if font := get(shopSignFontKey); font != "" {
		if v.Sign.Text.FontURL = a.shopSignURL(font); v.Sign.Text.FontURL != "" {
			v.Sign.Text.Font = fontname.File(filepath.Join(a.shopSignDir(), font))
		}
		if v.Sign.Text.Font == "" {
			v.Sign.Text.Font = strings.TrimSuffix(font, filepath.Ext(font))
		}
	}
	for file, url := range map[string]*string{"sign.json": &v.SignLayout, "phone.json": &v.PhoneLayout, "door.json": &v.DoorLayout} {
		if exists(filepath.Join(a.templatesDir(), gameextract.ShopSignDir, file)) {
			*url = "/signtemplates/" + file
		}
	}
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

// ---------------------------------------------------------------- pictures

func pictureSlot(slot string) (shopPicture, error) {
	pic, ok := shopPictures[slot]
	if !ok {
		return pic, errors.New("unknown picture " + slot)
	}
	return pic, nil
}

// setPicture stores a slot's file ("" = vanilla), starts it centred and deletes the slot's previous file (and its design).
func (a *App) setPicture(p string, pic shopPicture, name string) error {
	old, _ := modconfig.Get(p, pic.section, pic.imageKey)
	if _, err := modconfig.Set(p, pic.section, pic.cropKey, ""); err != nil {
		return err
	}
	if _, err := modconfig.Set(p, pic.section, pic.imageKey, name); err != nil {
		return err
	}
	if old.Value != "" && old.Value != name && filepath.Base(old.Value) == old.Value {
		_ = os.Remove(filepath.Join(a.shopSignDir(), old.Value))
		_ = os.Remove(filepath.Join(a.shopSignDir(), designFile(old.Value)))
	}
	a.pruneFiles()
	return nil
}

// designFile is the design kept next to a picture drawn from text (e.g. open_text.png.json).
func designFile(image string) string { return image + ".json" }

// SaveShopPictureText stores a door sign side designed in the page: the PNG (base64) becomes the side's picture and the
// design (JSON: text, colours, font {file, name}, background {file, crop}) is kept next to it, so the page can reopen and edit
// it. The mod only sees the picture.
func (a *App) SaveShopPictureText(slot, pngBase64, design string) (ShopLookView, error) {
	pic, err := pictureSlot(slot)
	if err != nil {
		return ShopLookView{}, err
	}
	if slot != "open" && slot != "closed" {
		return ShopLookView{}, errors.New("only the door sign takes text")
	}
	var d struct{}
	if len(design) > 64<<10 || json.Unmarshal([]byte(design), &d) != nil {
		return ShopLookView{}, errors.New("bad text design")
	}
	b, err := base64.StdEncoding.DecodeString(pngBase64)
	if err != nil || len(b) < 8 || string(b[1:4]) != "PNG" {
		return ShopLookView{}, errors.New("bad picture")
	}
	p, err := a.modConfigPath()
	if err != nil {
		return ShopLookView{}, err
	}
	if err := os.MkdirAll(a.shopSignDir(), 0o755); err != nil {
		return ShopLookView{}, err
	}
	name := pic.prefix + "text.png"
	if err := os.WriteFile(filepath.Join(a.shopSignDir(), name), b, 0o644); err != nil {
		return ShopLookView{}, err
	}
	if err := os.WriteFile(filepath.Join(a.shopSignDir(), designFile(name)), []byte(design), 0o644); err != nil {
		return ShopLookView{}, err
	}
	if err := a.setPicture(p, pic, name); err != nil {
		return ShopLookView{}, err
	}
	return a.ShopLook()
}

// ChooseShopPicture lets the user pick an image for a picture slot, copies it into <plugin>\ShopSign and selects it.
func (a *App) ChooseShopPicture(slot string) (ShopLookView, error) {
	pic, err := pictureSlot(slot)
	if err != nil {
		return ShopLookView{}, err
	}
	p, err := a.modConfigPath()
	if err != nil {
		return ShopLookView{}, err
	}
	f, err := a.importPicture(pic.title, pic.prefix)
	if err != nil || f.File == "" {
		if err != nil {
			return ShopLookView{}, err
		}
		return a.ShopLook()
	}
	if err := a.setPicture(p, pic, f.File); err != nil {
		return ShopLookView{}, err
	}
	return a.ShopLook()
}

// ShopFile is a file copied into <plugin>\ShopSign (a picture or a font).
type ShopFile struct {
	File string `json:"file"` // file name in the folder, "" = none chosen (cancelled)
	Name string `json:"name"` // display name
	URL  string `json:"url"`  // for the page's previews
}

// importPicture lets the user pick an image and copies it into <plugin>\ShopSign as prefix + its name (File "" = cancelled).
func (a *App) importPicture(title, prefix string) (ShopFile, error) {
	file, err := a.pickImageFile(title)
	if err != nil || file == "" {
		return ShopFile{}, err
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return ShopFile{}, err
	}
	if err := os.MkdirAll(a.shopSignDir(), 0o755); err != nil {
		return ShopFile{}, err
	}
	name := prefix + filepath.Base(file)
	if err := os.WriteFile(filepath.Join(a.shopSignDir(), name), b, 0o644); err != nil {
		return ShopFile{}, err
	}
	return ShopFile{File: name, Name: filepath.Base(file), URL: a.shopSignURL(name)}, nil
}

// ImportDoorBackground lets the user pick a background picture for a door sign side's design (drawn under its text). It is
// only copied: SaveShopPictureText makes it part of the side; unused backgrounds are pruned later.
func (a *App) ImportDoorBackground(slot string) (ShopFile, error) {
	if slot != "open" && slot != "closed" {
		return ShopFile{}, errors.New("only the door sign has text designs")
	}
	return a.importPicture(shopPictures[slot].title, slot+"_bg_")
}

// RemoveShopPicture goes back to a slot's vanilla picture.
func (a *App) RemoveShopPicture(slot string) (ShopLookView, error) {
	pic, err := pictureSlot(slot)
	if err != nil {
		return ShopLookView{}, err
	}
	p, err := a.modConfigPath()
	if err != nil {
		return ShopLookView{}, err
	}
	if err := a.setPicture(p, pic, ""); err != nil {
		return ShopLookView{}, err
	}
	return a.ShopLook()
}

// SetShopPictureCrop stores the part of a slot's image that is shown (fractions of the image, top-left origin; the crop box
// keeps the slot's shape). An empty slice goes back to the middle.
func (a *App) SetShopPictureCrop(slot string, crop []float64) (ShopLookView, error) {
	pic, err := pictureSlot(slot)
	if err != nil {
		return ShopLookView{}, err
	}
	p, err := a.modConfigPath()
	if err != nil {
		return ShopLookView{}, err
	}
	value := ""
	if len(crop) == 4 {
		parts := make([]string, 4)
		for i, f := range crop {
			parts[i] = strconv.FormatFloat(min(max(f, 0), 1), 'f', 4, 64)
		}
		value = strings.Join(parts, ",")
	} else if len(crop) != 0 {
		return ShopLookView{}, errors.New("crop needs x, y, width and height")
	}
	if _, err := modconfig.Set(p, pic.section, pic.cropKey, value); err != nil {
		return ShopLookView{}, err
	}
	return a.ShopLook()
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

// ---------------------------------------------------------------- the shop name on the sign

func (a *App) SetShopSignShowName(show bool) (ShopLookView, error) {
	p, err := a.modConfigPath()
	if err != nil {
		return ShopLookView{}, err
	}
	if _, err := modconfig.Set(p, shopSignSection, shopSignNameKey, strconv.FormatBool(show)); err != nil {
		return ShopLookView{}, err
	}
	return a.ShopLook()
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

// ImportShopFont copies a font into <plugin>\ShopSign, so it travels with setups: source "game"/"windows" = an entry of
// ShopSignFonts, "file" = a .ttf/.otf the user chooses (File "" when cancelled). Unused fonts are pruned later.
func (a *App) ImportShopFont(source, id string) (ShopFile, error) {
	var from string
	switch source {
	case "game":
		if strings.Contains(id, "..") || !strings.HasPrefix(id, "fonts/") {
			return ShopFile{}, errors.New("unknown game font")
		}
		from = filepath.Join(a.templatesDir(), gameextract.ShopSignDir, filepath.FromSlash(id))
	case "windows":
		for _, f := range installedFonts() {
			if strings.EqualFold(f.path, id) {
				from = f.path
			}
		}
		if from == "" {
			return ShopFile{}, errors.New("that font isn't installed")
		}
	case "file":
		var err error
		from, err = runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose a font",
			Filters: []runtime.FileFilter{{DisplayName: "Fonts (*.ttf, *.otf)", Pattern: "*.ttf;*.otf"}}})
		if err != nil || from == "" {
			return ShopFile{}, nil
		}
	default:
		return ShopFile{}, errors.New("unknown font source")
	}
	b, err := os.ReadFile(from)
	if err != nil {
		return ShopFile{}, err
	}
	if err := fontname.Check(b); err != nil {
		return ShopFile{}, err
	}
	if err := os.MkdirAll(a.shopSignDir(), 0o755); err != nil {
		return ShopFile{}, err
	}
	name := "font_" + filepath.Base(from)
	path := filepath.Join(a.shopSignDir(), name)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return ShopFile{}, err
	}
	display := fontname.File(path)
	if display == "" {
		display = strings.TrimSuffix(filepath.Base(from), filepath.Ext(from))
	}
	return ShopFile{File: name, Name: display, URL: a.shopSignURL(name)}, nil
}

// pruneFiles deletes the fonts and door sign backgrounds that neither the shop name nor a door sign design uses any more.
func (a *App) pruneFiles() {
	sign := ""
	if p, err := a.modConfigPath(); err == nil {
		if e, err := modconfig.Get(p, shopSignTextSection, shopSignFontKey); err == nil {
			sign = e.Value
		}
	}
	pruneFiles(a.shopSignDir(), sign)
}

// pruneFiles deletes the files we copy for designs (font_*, open_bg_*, closed_bg_*) from dir, except signFont and the
// font/background each design (*.png.json) names.
func pruneFiles(dir, signFont string) {
	used := map[string]bool{signFont: true}
	designs, _ := filepath.Glob(filepath.Join(dir, "*.png.json"))
	for _, f := range designs {
		var d struct {
			Font       struct{ File string } `json:"font"`
			Background struct{ File string } `json:"background"`
		}
		if b, err := os.ReadFile(f); err == nil && json.Unmarshal(b, &d) == nil {
			used[d.Font.File], used[d.Background.File] = true, true
		}
	}
	for _, pattern := range []string{"font_*", "open_bg_*", "closed_bg_*"} {
		files, _ := filepath.Glob(filepath.Join(dir, pattern))
		for _, f := range files {
			if !used[filepath.Base(f)] {
				_ = os.Remove(f)
			}
		}
	}
}

// SetShopSignFont picks the shop name's font: source "" = the game's own, otherwise as ImportShopFont.
func (a *App) SetShopSignFont(source, id string) (ShopLookView, error) {
	p, err := a.modConfigPath()
	if err != nil {
		return ShopLookView{}, err
	}
	name := ""
	if source != "" {
		f, err := a.ImportShopFont(source, id)
		if err != nil {
			return ShopLookView{}, err
		}
		if f.File == "" { // cancelled
			return a.ShopLook()
		}
		name = f.File
	}
	if _, err := modconfig.Set(p, shopSignTextSection, shopSignFontKey, name); err != nil {
		return ShopLookView{}, err
	}
	a.pruneFiles()
	return a.ShopLook()
}

// SetShopSignText stores the shop name's colour ("" = the game's gold), size, outline and outline colour.
func (a *App) SetShopSignText(t ShopSignText) (ShopLookView, error) {
	p, err := a.modConfigPath()
	if err != nil {
		return ShopLookView{}, err
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
		return ShopLookView{}, err
	}
	outColor, err := hex(t.OutlineColor)
	if err != nil {
		return ShopLookView{}, err
	}
	if outColor == "" {
		outColor = "#000000"
	}
	num := func(f, lo, hi float64) string { return strconv.FormatFloat(min(max(f, lo), hi), 'f', -1, 64) }
	for _, kv := range [][2]string{{shopSignColorKey, color}, {shopSignSizeKey, num(t.Size, 0.5, 2)},
		{shopSignOutlineKey, num(t.Outline, 0, 0.3)}, {shopSignOutColorKey, outColor}} {
		if _, err := modconfig.Set(p, shopSignTextSection, kv[0], kv[1]); err != nil {
			return ShopLookView{}, err
		}
	}
	return a.ShopLook()
}

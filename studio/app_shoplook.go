package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/art"
	"tcgstudio/internal/fontname"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/game"
	"tcgstudio/internal/gameextract"
	"tcgstudio/internal/modconfig"
	"tcgstudio/internal/setups"
)

// ---------------------------------------------------------------- Shop look page
//
// The shop sign above the entrance, the phone wallpaper, the open/closed sign at the door and the global card back. Stored as mod settings (a picture
// = an image file name in the setup's ShopSign folder + the part shown), so a running game applies changes live; settings-meta.json
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
// (the mod's ShopSign.FaceAspect, PhoneWallpaper.ScreenAspect, DoorSign.BoardAspect).
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
	CardBack    CardBackView `json:"cardBack"`
}

// CardBackView is the global card back: the picture it was cut from and the crop (to edit it again), and the result.
type CardBackView struct {
	Image  string    `json:"image"`  // URL of the card back the game uses, "" = none (each set's frame back)
	File   string    `json:"file"`   // its source picture in the ShopSign folder, "" = unknown (made before crops were kept)
	Source string    `json:"source"` // URL of that picture
	Crop   []float64 `json:"crop"`   // x, y, w, h fractions of the picture; nil = the middle
	Aspect float64   `json:"aspect"` // width ÷ height of the crop box (the card back art)
}

// cardBackDesign is kept next to the setup's card_back.png: what Save made it from.
type cardBackDesign struct {
	File string    `json:"file"`
	Crop []float64 `json:"crop,omitempty"`
}

func (a *App) cardBackDesignPath() string { return a.globalBackPath() + ".json" }

func (a *App) loadCardBackDesign() (cardBackDesign, bool) {
	var d cardBackDesign
	b, err := os.ReadFile(a.cardBackDesignPath())
	if err != nil || json.Unmarshal(b, &d) != nil || d.File == "" || filepath.Base(d.File) != d.File {
		return cardBackDesign{}, false
	}
	return d, true
}

type ShopPictureView struct {
	Image  string    `json:"image"`  // URL of the chosen image, "" = vanilla
	File   string    `json:"file"`   // its file name in the setup's ShopSign folder (for SaveShopLook), "" = vanilla
	Crop   []float64 `json:"crop"`   // x, y, w, h as fractions of the image (top-left origin); nil = the middle
	Aspect float64   `json:"aspect"` // width ÷ height of the crop box
	Design string    `json:"design"` // a side drawn from text: its design (JSON, see ShopPictureSave); "" = a picture
}

// ShopSignView is the shop name on the sign.
type ShopSignView struct {
	ShowName bool         `json:"showName"`
	Text     ShopSignText `json:"text"`
	ShopName string       `json:"shopName"` // the shop's name in the newest save ("" = no save yet); Save can rename it
}

// ShopSignText is the shop name's style ([Visuals - Shop sign text]).
type ShopSignText struct {
	Font         string  `json:"font"`     // display name of the chosen font, "" = the game's own
	FontFile     string  `json:"fontFile"` // its file name in the setup's ShopSign folder (for SaveShopLook)
	FontURL      string  `json:"fontUrl"`  // the chosen font file for the preview
	Color        string  `json:"color"`    // #RRGGBB, "" = the game's gold
	Size         float64 `json:"size"`
	Outline      float64 `json:"outline"` // fraction of the font size
	OutlineColor string  `json:"outlineColor"`
}

func (a *App) shopSignDir() string {
	return filepath.Join(a.root(), setups.GameStateDir, modconfig.ShopSignFolder) // read by the mod from the setup
}

func (a *App) ShopLook() (ShopLookView, error) {
	p, err := a.openModConfig()
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
			pv.File = img.Value
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
	v.Sign = ShopSignView{ShowName: name.Value != "false", ShopName: game.ShopName(),
		Text: ShopSignText{Color: get(shopSignColorKey), OutlineColor: get(shopSignOutColorKey), Size: 1}}
	if f, err := strconv.ParseFloat(get(shopSignSizeKey), 64); err == nil {
		v.Sign.Text.Size = f
	}
	if f, err := strconv.ParseFloat(get(shopSignOutlineKey), 64); err == nil {
		v.Sign.Text.Outline = f
	}
	if font := get(shopSignFontKey); font != "" {
		if v.Sign.Text.FontURL = a.shopSignURL(font); v.Sign.Text.FontURL != "" {
			v.Sign.Text.FontFile = font
			v.Sign.Text.Font = fontname.File(filepath.Join(a.shopSignDir(), font))
		}
		if v.Sign.Text.Font == "" {
			v.Sign.Text.Font = strings.TrimSuffix(font, filepath.Ext(font))
		}
	}
	v.CardBack = CardBackView{Image: a.GlobalCardBack(), Aspect: art.CardBackAspect}
	if d, ok := a.loadCardBackDesign(); ok && v.CardBack.Image != "" {
		if src := a.shopSignURL(d.File); src != "" {
			v.CardBack.File, v.CardBack.Source, v.CardBack.Crop = d.File, src, d.Crop
		}
	}
	for file, url := range map[string]*string{"sign.json": &v.SignLayout, "phone.json": &v.PhoneLayout, "door.json": &v.DoorLayout} {
		if exists(a.templateFile(gameextract.ShopSignDir, file)) {
			*url = "/signtemplates/" + file
		}
	}
	return v, nil
}

// shopSignURL serves a file of the setup's ShopSign folder to the page ("" when missing); the mtime query busts the cache on a replace.
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

// ---------------------------------------------------------------- pictures

func pictureSlot(slot string) (shopPicture, error) {
	pic, ok := shopPictures[slot]
	if !ok {
		return pic, errors.New("unknown picture " + slot)
	}
	return pic, nil
}

// designFile is the design kept next to a picture drawn from text (e.g. open_text_1a2b3c4d.png.json).
func designFile(image string) string { return image + ".json" }

// ShopFile is a file copied into the setup's ShopSign folder (a picture or a font).
type ShopFile struct {
	File string `json:"file"` // file name in the folder, "" = none chosen (cancelled)
	Name string `json:"name"` // display name
	URL  string `json:"url"`  // for the page's previews
}

// importPicture lets the user pick an image and copies it into the setup's ShopSign folder as prefix + its name (File "" = cancelled).
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

// ImportShopPicture lets the user pick an image for a picture slot and copies it into the setup's ShopSign folder (File "" =
// cancelled). The game doesn't change until SaveShopLook; a copy the page never saved is pruned by a later save.
func (a *App) ImportShopPicture(slot string) (ShopFile, error) {
	pic, err := pictureSlot(slot)
	if err != nil {
		return ShopFile{}, err
	}
	return a.importPicture(pic.title, pic.prefix)
}

// ImportDoorBackground lets the user pick a background picture for a door sign side's design (drawn under its text). It is
// only copied: SaveShopLook makes it part of the side; unused backgrounds are pruned later.
func (a *App) ImportDoorBackground(slot string) (ShopFile, error) {
	if slot != "open" && slot != "closed" {
		return ShopFile{}, errors.New("only the door sign has text designs")
	}
	return a.importPicture(shopPictures[slot].title, slot+"_bg_")
}

// cardBackPrefix starts the card back's source pictures in the ShopSign folder. Save cuts the picture at its crop into the
// setup's card_back.png (the file the mod reads) and keeps picture + crop (cardBackDesign) to edit it again.
const cardBackPrefix = "back_"

// CardBackSource is a picture for the card back in the draft, with the crop to start from.
type CardBackSource struct {
	File string    `json:"file"` // "" = cancelled
	URL  string    `json:"url"`
	Crop []float64 `json:"crop"`
}

// ImportCardBack lets the user pick a picture for the card back. Its crop starts where the automatic one would cut: inside
// plain borders (a scanned card's padding), the largest card-shaped box in the middle.
func (a *App) ImportCardBack() (CardBackSource, error) {
	file, err := a.pickImageFile("Choose the global card back image")
	if err != nil || file == "" {
		return CardBackSource{}, err
	}
	crop, err := art.ImageCardBackAutoCrop(file)
	if err != nil {
		return CardBackSource{}, err
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return CardBackSource{}, err
	}
	name, err := a.writeCardBackSource(strings.ToLower(filepath.Ext(file)), func(p string) error { return os.WriteFile(p, b, 0o644) })
	if err != nil {
		return CardBackSource{}, err
	}
	return CardBackSource{File: name, URL: a.shopSignURL(name), Crop: crop}, nil
}

// GenerateCardBack makes a card back picture from a colour and an optional title (already card-shaped: the crop is all of it).
func (a *App) GenerateCardBack(color, title string) (CardBackSource, error) {
	name, err := a.writeCardBackSource(".png", func(p string) error { return art.GenerateCardBack("", "", color, title, p) })
	if err != nil {
		return CardBackSource{}, err
	}
	return CardBackSource{File: name, URL: a.shopSignURL(name), Crop: []float64{0, 0, 1, 1}}, nil
}

// writeCardBackSource stores a source picture under a new name (so the preview never shows a cached older one).
func (a *App) writeCardBackSource(ext string, write func(path string) error) (string, error) {
	if err := os.MkdirAll(a.shopSignDir(), 0o755); err != nil {
		return "", err
	}
	name := cardBackPrefix + strconv.FormatInt(time.Now().UnixNano(), 36) + ext
	return name, write(filepath.Join(a.shopSignDir(), name))
}

// applyCardBack cuts a source picture (ImportCardBack / GenerateCardBack) at crop into the setup's global card back and
// remembers both; File "" removes the card back.
func (a *App) applyCardBack(s CardBackSave) error {
	if s.File == "" {
		for _, p := range []string{a.globalBackPath(), a.cardBackDesignPath()} {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}
	if filepath.Base(s.File) != s.File || !strings.HasPrefix(s.File, cardBackPrefix) || !exists(filepath.Join(a.shopSignDir(), s.File)) {
		return errors.New("card back picture not found: " + s.File)
	}
	if len(s.Crop) != 0 && len(s.Crop) != 4 {
		return errors.New("crop needs x, y, width and height")
	}
	if err := art.CardBackFromCrop(filepath.Join(a.shopSignDir(), s.File), s.Crop, a.globalBackPath()); err != nil {
		return err
	}
	b, err := json.Marshal(cardBackDesign{File: s.File, Crop: s.Crop})
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(a.cardBackDesignPath(), b)
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
	if json.Unmarshal([]byte(a.templateJSON(gameextract.ShopSignDir, "sign.json")), &lay) == nil {
		for _, f := range lay.Fonts {
			out = append(out, ShopSignFont{Source: "game", ID: f.File, Name: f.Name})
		}
	}
	for _, f := range installedFonts() {
		out = append(out, ShopSignFont{Source: "windows", ID: f.path, Name: f.name})
	}
	return out
}

// ImportShopFont copies a font into the setup's ShopSign folder, so it travels with setups: source "game"/"windows" = an entry of
// ShopSignFonts, "file" = a .ttf/.otf the user chooses (File "" when cancelled). Unused fonts are pruned later.
func (a *App) ImportShopFont(source, id string) (ShopFile, error) {
	var from string
	switch source {
	case "game":
		if from = a.templateFile(gameextract.ShopSignDir, filepath.FromSlash(id)); from == "" || !strings.HasPrefix(id, "fonts/") {
			return ShopFile{}, errors.New("unknown game font")
		}
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

// ---------------------------------------------------------------- saving the page

// ShopLookSave is the page's draft. Nothing reaches the game until the user clicks Save: SaveShopLook then writes every
// changed setting in one go. Missing pictures / nil fields = unchanged.
type ShopLookSave struct {
	Pictures map[string]ShopPictureSave `json:"pictures"` // by slot (shopPictures)
	ShowName *bool                      `json:"showName"`
	Font     *string                    `json:"font"`     // font file in the setup's ShopSign folder, "" = the game's own
	Text     *ShopSignText              `json:"text"`     // colour, size, outline, outline colour (the font: Font)
	ShopName *string                    `json:"shopName"` // a new name for the shop: written into the game's saves (game closed)
	CardBack *CardBackSave              `json:"cardBack"`
}

// CardBackSave is the card back of the draft: a source picture (ImportCardBack / GenerateCardBack) and its crop; File "" = none.
type CardBackSave struct {
	File string    `json:"file"`
	Crop []float64 `json:"crop"`
}

// ShopPictureSave is one picture slot of the draft.
type ShopPictureSave struct {
	File   string    `json:"file"`   // image file in the setup's ShopSign folder (ImportShopPicture), "" = vanilla
	Crop   []float64 `json:"crop"`   // x, y, w, h as fractions of the image (top-left origin); empty = the middle
	PNG    string    `json:"png"`    // a door sign side drawn in the page (base64 PNG), used instead of File
	Design string    `json:"design"` // that side's design (JSON: text, colours, font {file, name}, background {file, crop})
}

// SaveShopLook stores the page's draft (renaming the shop in the game's saves when the name changed), deletes the
// pictures it replaced and prunes copied files nothing uses any more.
func (a *App) SaveShopLook(s ShopLookSave) (ShopLookView, error) {
	p, err := a.openModConfig()
	if err != nil {
		return ShopLookView{}, err
	}
	// The shop's name lives in the game's saves: renamed first, so a running game (refused) leaves everything unsaved.
	if s.ShopName != nil {
		if old := game.ShopName(); strings.TrimSpace(*s.ShopName) != old {
			if _, err := game.RenameShop(old, *s.ShopName); err != nil {
				return ShopLookView{}, err
			}
		}
	}
	var kvs [][3]string
	var replaced []string
	for slot, ps := range s.Pictures {
		pic, err := pictureSlot(slot)
		if err != nil {
			return ShopLookView{}, err
		}
		name, crop := ps.File, ""
		if ps.PNG != "" {
			if name, err = a.writeDesignedPicture(pic, slot, ps.PNG, ps.Design); err != nil {
				return ShopLookView{}, err
			}
		} else {
			if name != "" && (filepath.Base(name) != name || !exists(filepath.Join(a.shopSignDir(), name))) {
				return ShopLookView{}, errors.New("picture not found: " + name)
			}
			if crop, err = cropValue(ps.Crop); err != nil {
				return ShopLookView{}, err
			}
		}
		if old, err := modconfig.Get(p, pic.section, pic.imageKey); err == nil && old.Value != name {
			replaced = append(replaced, old.Value)
		}
		kvs = append(kvs, [3]string{pic.section, pic.cropKey, crop}, [3]string{pic.section, pic.imageKey, name})
	}
	if s.ShowName != nil {
		kvs = append(kvs, [3]string{shopSignSection, shopSignNameKey, strconv.FormatBool(*s.ShowName)})
	}
	if s.Font != nil {
		if f := *s.Font; f != "" && (filepath.Base(f) != f || !exists(filepath.Join(a.shopSignDir(), f))) {
			return ShopLookView{}, errors.New("font not found: " + f)
		}
		kvs = append(kvs, [3]string{shopSignTextSection, shopSignFontKey, *s.Font})
	}
	if t := s.Text; t != nil {
		color, err := hexColor(t.Color)
		if err != nil {
			return ShopLookView{}, err
		}
		outColor, err := hexColor(t.OutlineColor)
		if err != nil {
			return ShopLookView{}, err
		}
		if outColor == "" {
			outColor = "#000000"
		}
		num := func(f, lo, hi float64) string { return strconv.FormatFloat(min(max(f, lo), hi), 'f', -1, 64) }
		kvs = append(kvs, [3]string{shopSignTextSection, shopSignColorKey, color},
			[3]string{shopSignTextSection, shopSignSizeKey, num(t.Size, 0.5, 2)},
			[3]string{shopSignTextSection, shopSignOutlineKey, num(t.Outline, 0, 0.3)},
			[3]string{shopSignTextSection, shopSignOutColorKey, outColor})
	}
	if len(kvs) > 0 {
		if _, err := modconfig.SetMany(p, kvs); err != nil {
			return ShopLookView{}, err
		}
	}
	if s.CardBack != nil {
		if err := a.applyCardBack(*s.CardBack); err != nil {
			return ShopLookView{}, err
		}
	}
	inUse := a.shopFilesInUse(p)
	for _, f := range replaced {
		if f != "" && filepath.Base(f) == f && !slices.Contains(inUse, f) {
			_ = os.Remove(filepath.Join(a.shopSignDir(), f))
			_ = os.Remove(filepath.Join(a.shopSignDir(), designFile(f)))
		}
	}
	pruneFiles(a.shopSignDir(), inUse)
	return a.ShopLook()
}

// writeDesignedPicture writes a door sign side drawn in the page and its design; returns the picture's file name. A new name
// per design: the mod rebuilds a picture when its setting changes, so rewriting the same file would leave a running game on
// the old one (SaveShopLook deletes the previous file).
func (a *App) writeDesignedPicture(pic shopPicture, slot, pngBase64, design string) (string, error) {
	if slot != "open" && slot != "closed" {
		return "", errors.New("only the door sign takes text")
	}
	var d struct{}
	if len(design) > 64<<10 || json.Unmarshal([]byte(design), &d) != nil {
		return "", errors.New("bad text design")
	}
	b, err := base64.StdEncoding.DecodeString(pngBase64)
	if err != nil || len(b) < 8 || string(b[1:4]) != "PNG" {
		return "", errors.New("bad picture")
	}
	if err := os.MkdirAll(a.shopSignDir(), 0o755); err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	name := pic.prefix + "text_" + hex.EncodeToString(sum[:4]) + ".png"
	if err := os.WriteFile(filepath.Join(a.shopSignDir(), name), b, 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(a.shopSignDir(), designFile(name)), []byte(design), 0o644); err != nil {
		return "", err
	}
	return name, nil
}

// cropValue is the crop setting "x,y,w,h" ("" = the middle).
func cropValue(crop []float64) (string, error) {
	if len(crop) == 0 {
		return "", nil
	}
	if len(crop) != 4 {
		return "", errors.New("crop needs x, y, width and height")
	}
	parts := make([]string, 4)
	for i, f := range crop {
		parts[i] = strconv.FormatFloat(min(max(f, 0), 1), 'f', 4, 64)
	}
	return strings.Join(parts, ","), nil
}

// hexColor normalises a #RRGGBB colour ("" stays "").
func hexColor(s string) (string, error) {
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

// shopFilesInUse are the setup's ShopSign folder files the settings name: every slot's picture and the shop name's
// font, plus the card back's source picture.
func (a *App) shopFilesInUse(p string) []string {
	var out []string
	if d, ok := a.loadCardBackDesign(); ok {
		out = append(out, d.File)
	}
	for _, pic := range shopPictures {
		if e, err := modconfig.Get(p, pic.section, pic.imageKey); err == nil && e.Value != "" {
			out = append(out, e.Value)
		}
	}
	if e, err := modconfig.Get(p, shopSignTextSection, shopSignFontKey); err == nil && e.Value != "" {
		out = append(out, e.Value)
	}
	return out
}

// pruneFiles deletes the files the page copies (font_*, sign_*, wallpaper_*, open_*, closed_*: pictures, door sign designs and
// their backgrounds; back_*: card back pictures) from dir unless they're in use: named in inUse, the design of such a picture, or the font/background
// that design names. Other files (e.g. a picture set by hand) stay.
func pruneFiles(dir string, inUse []string) {
	used := map[string]bool{}
	for _, f := range inUse {
		used[f] = true
		b, err := os.ReadFile(filepath.Join(dir, designFile(f)))
		if err != nil {
			continue
		}
		used[designFile(f)] = true
		var d struct {
			Font       struct{ File string } `json:"font"`
			Background struct{ File string } `json:"background"`
		}
		if json.Unmarshal(b, &d) == nil {
			used[d.Font.File], used[d.Background.File] = true, true
		}
	}
	for _, pattern := range []string{"font_*", "sign_*", "wallpaper_*", "open_*", "closed_*", cardBackPrefix + "*"} {
		files, _ := filepath.Glob(filepath.Join(dir, pattern))
		for _, f := range files {
			if !used[filepath.Base(f)] {
				_ = os.Remove(f)
			}
		}
	}
}

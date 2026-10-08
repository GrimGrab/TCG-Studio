package main

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"tcgstudio/internal/game"
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
)

// ShopSignView is the sign's current state for the Settings page.
type ShopSignView struct {
	Image    string    `json:"image"` // URL of the chosen image, "" = the vanilla sign
	Crop     []float64 `json:"crop"`  // x, y, w, h as fractions of the image (top-left origin); nil = the middle
	ShowName bool      `json:"showName"`
	Aspect   float64   `json:"aspect"` // width ÷ height of the sign face (mod: ShopSign.FaceAspect)
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
	if img.Value != "" {
		if info, err := os.Stat(filepath.Join(a.shopSignDir(), img.Value)); err == nil {
			v.Image = "/shopsign/" + url.PathEscape(img.Value) + "?v=" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
		}
	}
	return v, nil
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
	if _, err := modconfig.Set(p, shopSignSection, shopSignImageKey, ""); err != nil {
		return ShopSignView{}, err
	}
	if _, err := modconfig.Set(p, shopSignSection, shopSignCropKey, ""); err != nil {
		return ShopSignView{}, err
	}
	_ = os.RemoveAll(a.shopSignDir())
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

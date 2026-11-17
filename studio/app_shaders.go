package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/cardshader"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/game"
	"tcgstudio/internal/ids"
	"tcgstudio/internal/setfmt"
)

// Shaders page: card shaders written by the player (the active setup's item library, content kind "shader"). The page
// previews the code as written (WebGL); Save also compiles every changed shader for the game (internal/cardshader) and
// stores the result in the shared store. The mod reads the library when the game starts (Runtime/CardShaders).

// ShadersPage is everything the page shows.
type ShadersPage struct {
	Shaders   []setfmt.Shader      `json:"shaders"`
	Starters  []cardshader.Starter `json:"starters"`
	// Tutorial: the steps of the guided tutorial.
	Tutorial []cardshader.TutorialStep `json:"tutorial"`
	// Stock: the shaders that come with the mod, with their stock values (the code is read-only; a copy can be edited).
	// StockValues: this setup's own values for them (accessory library "stockValues"), which the game uses instead.
	Stock       []cardshader.StockShader          `json:"stock"`
	StockValues map[string]map[string][]float64 `json:"stockValues"`
	MaxValues int                  `json:"maxValues"`
	MaxImages int                  `json:"maxImages"`
	// NoTemplate: why this build cannot compile shaders for the game ("" = it can).
	NoTemplate string `json:"noTemplate"`
}

// ShaderProblem is a shader whose code could not be compiled for the game: it was saved, and the game keeps drawing
// what its last good version drew (or the plain picture).
type ShaderProblem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Messages string `json:"messages"`
}

// ShaderSaveResult is what Save reports.
type ShaderSaveResult struct {
	Shaders  []setfmt.Shader `json:"shaders"`
	Problems []ShaderProblem `json:"problems"`
	Warnings []string        `json:"warnings"`
}

// refreshShaderBundles compiles again every stored shader whose bundle was made from another template than this
// Studio's (a Studio update changed what the mod's shaders are built on; such a bundle no longer gets its values in the
// game). Shaders are small and this only works when something is stale. Reports whether the library changed.
func refreshShaderBundles(l *accessories.Library) bool {
	template, err := cardshader.Template()
	if err != nil {
		return false
	}
	changed := false
	for i := range l.Lib.Shaders {
		s := &l.Lib.Shaders[i]
		sig := shaderSignature(template, *s)
		if s.Built == sig && s.Bundle != "" && fsutil.Exists(l.Assets.Path(s.Bundle)) {
			continue
		}
		bundle, err := cardshader.Build(template, s.ID, s.Code, shaderParams(s.Params), shaderOptions(*s))
		if err != nil {
			continue // code with mistakes: the Shaders page reports it at the next Save
		}
		ref, err := l.PutBytes(bundle, ".shaderbundle")
		if err != nil {
			continue
		}
		s.Bundle, s.Built = ref, sig
		changed = true
	}
	return changed
}

// RefreshShaders brings the active setup's compiled shaders up to date with this Studio (see refreshShaderBundles).
// Called when Studio starts and whenever shaders are listed. Returns how many setups' libraries were rewritten (0 or 1).
func (a *App) RefreshShaders() (int, error) {
	l, err := a.items()
	if err != nil {
		return 0, err
	}
	if !refreshShaderBundles(l) {
		return 0, nil
	}
	_, err = a.saveLibrary(l)
	return 1, err
}

// LoadShadersPage reads the page's data for the active setup.
func (a *App) LoadShadersPage() (ShadersPage, error) {
	page := ShadersPage{Starters: cardshader.Starters, Tutorial: cardshader.Tutorial, Stock: cardshader.Stock(), MaxValues: cardshader.MaxValues, MaxImages: cardshader.MaxImages}
	if _, err := cardshader.Template(); err != nil {
		page.NoTemplate = err.Error()
	}
	if _, err := a.RefreshShaders(); err != nil {
		return page, err
	}
	l, err := a.items()
	if err != nil {
		return page, err
	}
	page.Shaders = append([]setfmt.Shader{}, l.Lib.Shaders...)
	page.StockValues = cardshader.CleanStockValues(l.Lib.StockValues)
	if page.StockValues == nil {
		page.StockValues = map[string]map[string][]float64{}
	}
	return page, nil
}

// ShaderChoice is a shader a card version can be given: one of the setup's or a stock one (BuiltIn; Border = the
// border whose foil version it is the default of).
type ShaderChoice struct {
	ID      string               `json:"id"`
	Name    string               `json:"name"`
	Code    string               `json:"code"`
	Params  []setfmt.ShaderParam `json:"params"`
	BuiltIn bool                 `json:"builtIn"`
	Border  string               `json:"border,omitempty"`
	// What the previews need besides the code (setfmt.Shader's fields of the same names).
	Draws       string  `json:"draws,omitempty"`
	Mix         string  `json:"mix,omitempty"`
	MixStrength float64 `json:"mixStrength,omitempty"`
}

// ListShaders lists the shaders the pickers and previews of the set editor offer: the active setup's, then the stock ones.
func (a *App) ListShaders() ([]ShaderChoice, error) {
	if _, err := a.RefreshShaders(); err != nil {
		return nil, err
	}
	l, err := a.items()
	if err != nil {
		return nil, err
	}
	out := []ShaderChoice{}
	for _, s := range l.Lib.Shaders {
		out = append(out, ShaderChoice{ID: s.ID, Name: s.Name, Code: s.Code, Params: s.Params, Draws: s.Draws, Mix: s.Mix, MixStrength: s.MixStrength})
	}
	for _, s := range cardshader.Stock() {
		s = s.WithValues(l.Lib.StockValues[s.ID]) // what the game draws this setup's cards with
		c := ShaderChoice{ID: s.ID, Name: s.Name, Code: s.Code, BuiltIn: true, Border: s.Border}
		for _, p := range s.Params {
			c.Params = append(c.Params, setfmt.ShaderParam{Name: p.Name, Kind: p.Kind, Value: p.Value, Min: p.Min, Max: p.Max})
		}
		out = append(out, c)
	}
	return out, nil
}

// NewShaderID returns an id no item of the setup uses.
func (a *App) NewShaderID() (string, error) {
	l, err := a.items()
	if err != nil {
		return "", err
	}
	return ids.Free(l.HasItem), nil
}

func shaderParams(list []setfmt.ShaderParam) []cardshader.Param {
	out := make([]cardshader.Param, len(list))
	for i, p := range list {
		out[i] = cardshader.Param{Name: p.Name, Kind: p.Kind}
	}
	return out
}

// shaderOptions are a shader's settings that are compiled into its code.
func shaderOptions(s setfmt.Shader) cardshader.Options {
	return cardshader.Options{Mix: s.Mix, Strength: s.MixStrength}
}

// ShaderWrapping returns the text the preview puts around a shader's code for these parameters and this mix (Shadertoy
// code: setfmt.Shader's Mix and MixStrength). An error says what is wrong with the parameter list (a bad or repeated
// name, too many) or the mix.
func (a *App) ShaderWrapping(params []setfmt.ShaderParam, mix string, strength float64) (cardshader.Wrapping, error) {
	return cardshader.PreviewWrapping(shaderParams(params), cardshader.Options{Mix: mix, Strength: strength})
}

// PickShaderImage lets the player choose a picture for an image parameter and copies it into the shared store.
// Returns its reference, "" when cancelled.
func (a *App) PickShaderImage() (string, error) {
	path, err := a.pickImageFile("Choose a picture for the shader")
	if err != nil || path == "" {
		return "", err
	}
	l, err := a.items()
	if err != nil {
		return "", err
	}
	return l.Assets.PutFile(path)
}

// shaderSignature identifies what a bundle was compiled from.
func shaderSignature(template []byte, s setfmt.Shader) string {
	h := sha256.New()
	h.Write(template)
	fmt.Fprintf(h, "\x00wrap%d\x00%s\x00%s", cardshader.WrapVersion, s.ID, s.Code)
	for _, p := range s.Params {
		fmt.Fprintf(h, "\x00%s\x00%s", p.Name, p.Kind)
	}
	// The mix is part of the code of Shadertoy shaders only.
	if cardshader.IsShadertoy(s.Code) {
		fmt.Fprintf(h, "\x00mix\x00%s\x00%v", s.Mix, s.MixStrength)
	}
	return hex.EncodeToString(h.Sum(nil)[:12])
}

// SaveShaders stores the page (Save): the whole list. Shaders that left the list are kept in the shared catalog. Every
// new or changed shader is compiled for the game; one whose code has mistakes is saved all the same and reported.
// stockValues: this setup's own values for the stock shaders (stock id → parameter → values; empty = the stock values).
func (a *App) SaveShaders(list []setfmt.Shader, stockValues map[string]map[string][]float64) (ShaderSaveResult, error) {
	var res ShaderSaveResult
	if a.artConverting.Load() {
		return res, errArtConverting
	}
	l, err := a.items()
	if err != nil {
		return res, err
	}
	for i := range list {
		s := &list[i]
		s.Name = strings.TrimSpace(s.Name)
		if cardshader.IsStock(s.ID) {
			return res, fmt.Errorf("shader %q: its id belongs to the shaders that come with the mod", s.Name)
		}
		if l.ShaderIndex(s.ID) < 0 && l.HasItem(s.ID) {
			return res, fmt.Errorf("shader %q: its id is used by another item", s.Name)
		}
		if _, err := cardshader.Slots(shaderParams(s.Params)); err != nil {
			return res, fmt.Errorf("shader %q: %w", s.Name, err)
		}
		if err := shaderOptions(*s).Check(); err != nil {
			return res, fmt.Errorf("shader %q: %w", s.Name, err)
		}
		if strings.HasPrefix(s.Icon, "data:") {
			png, err := decodeDataURL(s.Icon)
			if err != nil || len(png) < 8 || string(png[1:4]) != "PNG" {
				return res, fmt.Errorf("shader %q: bad preview picture", s.Name)
			}
			if s.Icon, err = l.PutBytes(png, ".png"); err != nil {
				return res, err
			}
		}
	}
	check := setfmt.AccessoryLibrary{Shaders: list}
	if errs := check.ValidateShaders(); len(errs) > 0 {
		return res, errors.New(strings.Join(errs, "\n"))
	}

	template, terr := cardshader.Template()
	for i := range list {
		s := &list[i]
		// What the game has stays until a newer version compiles.
		if old := l.ShaderIndex(s.ID); old >= 0 {
			s.Bundle, s.Built = l.Lib.Shaders[old].Bundle, l.Lib.Shaders[old].Built
		} else {
			s.Bundle, s.Built = "", ""
		}
		if terr != nil {
			continue
		}
		sig := shaderSignature(template, *s)
		if s.Built == sig && s.Bundle != "" && fsutil.Exists(l.Assets.Path(s.Bundle)) {
			continue
		}
		bundle, err := cardshader.Build(template, s.ID, s.Code, shaderParams(s.Params), shaderOptions(*s))
		var mistake *cardshader.SourceError
		switch {
		case errors.As(err, &mistake):
			res.Problems = append(res.Problems, ShaderProblem{ID: s.ID, Name: s.Name, Messages: mistake.Messages})
			continue
		case err != nil:
			return res, fmt.Errorf("shader %q: %w", s.Name, err)
		}
		if s.Bundle, err = l.PutBytes(bundle, ".shaderbundle"); err != nil {
			return res, err
		}
		s.Built = sig
	}
	if terr != nil && len(list) > 0 {
		res.Warnings = append(res.Warnings, terr.Error()+".")
	}

	kept := map[string]bool{}
	for _, s := range list {
		kept[s.ID] = true
	}
	var gone []string
	for _, s := range l.Lib.Shaders {
		if !kept[s.ID] {
			gone = append(gone, s.ID)
		}
	}
	if len(gone) > 0 {
		if err := a.keepInCatalog("", gone, l); err != nil {
			return res, err
		}
	}
	l.Lib.Shaders = list
	l.Lib.StockValues = cardshader.CleanStockValues(stockValues)
	if _, err := a.saveLibrary(l); err != nil {
		return res, err
	}
	if game.IsRunning() {
		res.Warnings = append(res.Warnings, "The game is running: it picks up the change at its next start.")
	}
	res.Shaders = list
	return res, nil
}

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/fontname"
	"tcgstudio/internal/gameextract"
	"tcgstudio/internal/project"
)

// Fonts for the font picker (views/FontPicker.svelte): the game's fonts and Windows fonts (ShopSignFonts), font files picked
// this run (PickFontFile) and their previews. A font a design keeps using is copied where that design lives: the shop sign's
// into the setup (ImportShopFont), a pack/box text layer's into the set (ImportSetFont).

// FontRef names a font of the picker: source "game" (id = file in the shop sign templates), "windows" (id = the installed
// font's path) or "file" (id = a font file picked this run).
type FontRef struct {
	Source string `json:"source"`
	ID     string `json:"id"`
}

// pickedFonts are the font files chosen with PickFontFile this run; FontRef "file" ids index it.
var pickedFonts struct {
	mu    sync.Mutex
	paths []string
}

// chooseFontFile asks for a .ttf/.otf ("" when cancelled): the one font file dialog (shop sign fonts, PickFontFile).
func (a *App) chooseFontFile() (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose a font",
		Filters: []runtime.FileFilter{{DisplayName: "Fonts (*.ttf, *.otf)", Pattern: "*.ttf;*.otf"}}})
}

// PickFontFile asks for a .ttf/.otf and offers it like a listed font (Source "file"; ID "" when cancelled).
func (a *App) PickFontFile() (ShopSignFont, error) {
	from, err := a.chooseFontFile()
	if err != nil || from == "" {
		return ShopSignFont{}, nil
	}
	b, err := os.ReadFile(from)
	if err != nil {
		return ShopSignFont{}, err
	}
	if err := fontname.Check(b); err != nil {
		return ShopSignFont{}, err
	}
	name := fontname.File(from)
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(from), filepath.Ext(from))
	}
	pickedFonts.mu.Lock()
	defer pickedFonts.mu.Unlock()
	for i, p := range pickedFonts.paths {
		if strings.EqualFold(p, from) {
			return ShopSignFont{Source: "file", ID: strconv.Itoa(i), Name: name}, nil
		}
	}
	pickedFonts.paths = append(pickedFonts.paths, from)
	return ShopSignFont{Source: "file", ID: strconv.Itoa(len(pickedFonts.paths) - 1), Name: name}, nil
}

// fontPath is the file of a listed or picked font.
func (a *App) fontPath(f FontRef) (string, error) {
	switch f.Source {
	case "game":
		if p := a.templateFile(gameextract.ShopSignDir, filepath.FromSlash(f.ID)); p != "" && strings.HasPrefix(f.ID, "fonts/") {
			return p, nil
		}
	case "windows":
		for _, w := range installedFonts() {
			if strings.EqualFold(w.path, f.ID) {
				return w.path, nil
			}
		}
	case "file":
		if p := pickedFont(f.ID); p != "" {
			return p, nil
		}
	}
	return "", errors.New("that font isn't available any more — choose it again")
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// ImportSetFont copies a font of the picker into the set's files as fonts/<name> (a text layer of the pack/box editor names
// it, so it travels with the set) and returns that path. The same font again reuses its copy; another file with the same
// name gets a new one.
func (a *App) ImportSetFont(id, source, fontID string) (string, error) {
	from, err := a.fontPath(FontRef{Source: source, ID: fontID})
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(from)
	if err != nil {
		return "", err
	}
	if err := fontname.Check(b); err != nil {
		return "", err
	}
	p, err := a.loadProject(id)
	if err != nil {
		return "", err
	}
	rel := "fonts/" + strings.ToLower(unsafeName.ReplaceAllString(filepath.Base(from), "_"))
	if old, err := os.ReadFile(p.ImagePath(rel)); err == nil {
		if bytes.Equal(old, b) {
			return rel, nil
		}
		rel = project.FreshName(rel)
	}
	return p.WriteFile(rel, b)
}

func pickedFont(id string) string {
	i, err := strconv.Atoi(id)
	pickedFonts.mu.Lock()
	defer pickedFonts.mu.Unlock()
	if err != nil || i < 0 || i >= len(pickedFonts.paths) {
		return ""
	}
	return pickedFonts.paths[i]
}

// winFontsByName maps an installed font's file name to its path, for the font list's previews (/winfonts/<file name>).
var winFontsByName struct {
	sync.Mutex
	m map[string]string
}

func winFontFile(name string) string {
	winFontsByName.Lock()
	defer winFontsByName.Unlock()
	if winFontsByName.m == nil {
		winFontsByName.m = map[string]string{}
		for _, f := range installedFonts() {
			winFontsByName.m[strings.ToLower(filepath.Base(f.path))] = f.path
		}
	}
	return winFontsByName.m[strings.ToLower(name)]
}

func init() {
	fileRoutes = append(fileRoutes,
		// previews in the font list: only fonts Windows lists or the user picked
		fileRoute{"winfonts/", 1, func(a *App, rel string) string { return winFontFile(rel) }},
		fileRoute{"pickedfont/", 1, func(a *App, rel string) string { return pickedFont(rel) }},
	)
}

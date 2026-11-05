package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"os"
	"strconv"
	"strings"

	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/game"
	"tcgstudio/internal/gameextract"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/uvmap"
)

// ---------------------------------------------------------------- Game look page: Clothes
//
// New looks of the customers' clothing pieces (accessory library "clothes", content kind "clothing"): colours and a print
// painted in the shared face editor over the piece's own UVs. The pieces, their vanilla colour variants, models and UV
// islands come from the game files (templates clothes\, gameextract/clothes.go). The tab has its own draft; the page's
// Save stores it here (SaveClothes), and the mod's Runtime/Clothes/Wardrobe reads the list when the game starts.

// ClothesPage is the tab's data: the stored looks and the templates' clothes.json URL ("" = not read from the game yet).
type ClothesPage struct {
	Clothes   []setfmt.Clothing `json:"clothes"`
	Templates string            `json:"templates"`
}

func (a *App) LoadClothes() (ClothesPage, error) {
	page := ClothesPage{Clothes: []setfmt.Clothing{}}
	l, err := a.items()
	if err != nil {
		return page, err
	}
	page.Clothes = append(page.Clothes, l.Lib.Clothes...)
	if fsutil.Exists(a.templateFile(gameextract.ClothesDir, "clothes.json")) {
		page.Templates = "/clothtemplates/clothes.json"
	}
	return page, nil
}

// NewClothingID is a fresh random id for a new look.
func (a *App) NewClothingID() (string, error) {
	l, err := a.items()
	if err != nil {
		return "", err
	}
	return l.NewID(), nil
}

func (a *App) clothesTemplates() (gameextract.ClothesFile, error) {
	var f gameextract.ClothesFile
	b, err := os.ReadFile(a.templateFile(gameextract.ClothesDir, "clothes.json"))
	if err != nil {
		return f, errors.New("the clothes haven't been read from the game files yet (Game & mod page)")
	}
	return f, json.Unmarshal(b, &f)
}

func findPiece(f gameextract.ClothesFile, body, piece string) (gameextract.ClothingPiece, bool) {
	for _, s := range f.Bodies[body].Slots {
		for _, it := range s.Items {
			if it.Name == piece {
				return it, true
			}
		}
	}
	return gameextract.ClothingPiece{}, false
}

// ClothingModel is the face editor's model for one material of a piece: a face per UV island (the big ones first, the
// small seams and trims as hidden faces), the piece's model as the 3D preview.
func (a *App) ClothingModel(body, piece string, material int) (uvmap.Model, error) {
	f, err := a.clothesTemplates()
	if err != nil {
		return uvmap.Model{}, err
	}
	p, ok := findPiece(f, body, piece)
	if !ok || p.Mesh == "" || material < 0 || material >= len(p.Islands) {
		return uvmap.Model{}, fmt.Errorf("%s can't be painted", piece)
	}
	var islands []uvmap.Island
	for _, is := range p.Islands[material] {
		islands = append(islands, uvmap.Island{ID: is.ID, Label: is.Label, Rect: is.Rect, Small: is.Small})
	}
	return uvmap.Clothing(p.Mesh, "/clothtemplates/"+p.Mesh, gameextract.ClothesTextureSize, islands), nil
}

// ClothingSave is one look of the tab's draft: the look and the prints painted since the last save (part index → PNG
// data URL, or "tpl:<file>" = a vanilla print from the templates as it is; "" = no print; parts not named keep theirs).
type ClothingSave struct {
	Item   setfmt.Clothing   `json:"item"`
	Prints map[string]string `json:"prints"`
}

// bakeClothesPrint turns a print painted on part j's UVs (PNG data URL) into the PNG the game samples
// (gameextract.BakeClothesPrint). Pieces whose material repeats its print can't take a painted one yet.
func (a *App) bakeClothesPrint(tpl gameextract.ClothesFile, tplErr error, c setfmt.Clothing, j int, dataURL string) ([]byte, error) {
	if tplErr != nil {
		return nil, tplErr
	}
	p, ok := findPiece(tpl, c.Body, c.Piece)
	if !ok {
		return nil, fmt.Errorf("%s: the game has no %s piece %q", c.Name, c.Body, c.Piece)
	}
	if j >= len(p.PrintScale) {
		return nil, fmt.Errorf("%s: the game templates are older than this Studio; open Game & mod so they're read again", c.Name)
	}
	if s := p.PrintScale[j]; s != 1 {
		return nil, fmt.Errorf("%s: the %s repeats its print %g times, so a painted print can't be used on it yet (colours only)", c.Name, p.Display, s)
	}
	raw, err := decodeDataURL(dataURL)
	if err != nil {
		return nil, fmt.Errorf("%s: bad print", c.Name)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: bad print: %v", c.Name, err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, gameextract.BakeClothesPrint(img)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SaveClothes stores the tab's whole list (Save). Errors refuse the save; returns warnings.
func (a *App) SaveClothes(list []ClothingSave) ([]string, error) {
	l, err := a.items()
	if err != nil {
		return nil, err
	}
	tpl, tplErr := a.clothesTemplates()
	out := make([]setfmt.Clothing, 0, len(list))
	for _, s := range list {
		c := s.Item
		c.Name = strings.TrimSpace(c.Name)
		for k, data := range s.Prints {
			j, err := strconv.Atoi(k)
			if err != nil || j < 0 || j >= len(c.Parts) {
				return nil, fmt.Errorf("%s: bad part %q", c.Name, k)
			}
			if data == "" {
				c.Parts[j].Print = ""
				continue
			}
			var data2 []byte
			if name, ok := strings.CutPrefix(data, "tpl:"); ok { // a vanilla print as it is (already the game's texture)
				data2, err = os.ReadFile(a.templateFile(gameextract.ClothesDir, name))
			} else {
				data2, err = a.bakeClothesPrint(tpl, tplErr, c, j, data)
			}
			if err != nil {
				return nil, err
			}
			if len(data2) < 8 || string(data2[1:4]) != "PNG" {
				return nil, fmt.Errorf("%s: bad print", c.Name)
			}
			if c.Parts[j].Print, err = l.PutBytes(data2, ".png"); err != nil {
				return nil, err
			}
		}
		out = append(out, c)
	}
	check := setfmt.AccessoryLibrary{Clothes: out, Customers: l.Lib.Customers}
	errs, warns := check.ValidateClothes()
	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "\n"))
	}
	ids := map[string]bool{}
	for _, c := range out {
		ids[c.ID] = true
		if l.ClothingIndex(c.ID) < 0 && l.HasItem(c.ID) {
			return nil, fmt.Errorf("the id %q is already used by another item", c.ID)
		}
	}
	l.Lib.Clothes = out
	if err := l.Save(); err != nil {
		return nil, err
	}
	if game.IsRunning() {
		warns = append(warns, "The game is running: customers wear the change after its next start.")
	}
	return warns, nil
}

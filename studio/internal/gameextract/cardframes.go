package gameextract

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"

	"golang.org/x/image/draw"

	"tcgstudio/internal/imgfile"
	"tcgstudio/internal/unityfs"
)

// The game's border rims, for Studio's border previews (a set's Borders & versions page shows a slot without a border of
// its own as the game draws it). Per frame template (a vanilla expansion) and border: the style the game picks for that
// border (CardUISetting.GetCardUISettingData, not Destiny: the first style listing the border, else the first style), its
// rim sheet (cardBorderImageList[border], [0] when shorter) clipped to cardBorderMask, cut to the card. The mask is the
// whole card's shape: on a full-image card the sheet is the card under the picture, and the rim is what shows around the
// picture (inset to FullImageInset of the card's height). Measured in the game files 2026-11-20 (game 1.04).

// CardFramesDir is the folder of the rims in the templates; CardFramesFile lists them.
const (
	CardFramesDir  = "cardframes"
	CardFramesFile = "cardframes.json"
)

// rimHeight is the most a rim picture is tall (preview size).
const rimHeight = 720

// CardFrame is one rim picture.
type CardFrame struct {
	Template string `json:"template"` // the frame template (ECardExpansionType name: "Tetramon")
	Border   string `json:"border"`   // the border (ECardBorderType name: "Gold")
	File     string `json:"file"`     // picture in CardFramesDir; "" = the style draws no rim (Full Art)
}

var borderNames = []string{"Base", "FirstEdition", "Silver", "Gold", "EX", "FullArt"}

func (x *extractor) cardFrames(expansions map[int32]string) error {
	var settings []unityfs.CardUISetting
	var from *unityfs.File
	var firstErr error
	for _, name := range x.assetFiles() {
		f, err := x.env.File(name)
		if err != nil || f == nil {
			continue
		}
		for _, o := range x.env.FindScripts(f, "MonsterData_ScriptableObject") {
			s, err := x.env.ReadCardUISettings(o)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if len(s) > len(settings) {
				settings, from = s, f
			}
		}
	}
	if len(settings) == 0 {
		if firstErr != nil {
			return firstErr
		}
		return fmt.Errorf("the game's card data (MonsterData_ScriptableObject) wasn't found")
	}
	dir := filepath.Join(x.dir, CardFramesDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var list []CardFrame
	images := map[[2]unityfs.PPtr]string{} // sheet + mask → file (styles share them)
	done := map[string]bool{}
	for _, s := range settings {
		template, ok := expansions[s.Expansion]
		// The game's list has a second "CatJob" entry (the unused FoodieGO slot): the first of each is the one it uses.
		if !ok || len(s.Styles) == 0 || done[template] {
			continue
		}
		done[template] = true
		for b, border := range borderNames {
			style := s.Styles[0]
			for _, st := range s.Styles {
				if slices.Contains(st.Borders, int32(b)) {
					style = st
					break
				}
			}
			frame := CardFrame{Template: template, Border: border}
			if style.ShowBorder && len(style.BorderImages) > 0 {
				sheet := style.BorderImages[0]
				if b < len(style.BorderImages) {
					sheet = style.BorderImages[b]
				}
				key := [2]unityfs.PPtr{sheet, style.BorderMask}
				file, done := images[key]
				if !done {
					file = fmt.Sprintf("%s_%s.png", template, border)
					if err := x.saveRim(from, sheet, style.BorderMask, filepath.Join(dir, file)); err != nil {
						x.warn("card rim %s %s: %v", template, border, err)
						file = ""
					}
					images[key] = file
				}
				frame.File = file
			}
			list = append(list, frame)
		}
	}
	b, err := json.MarshalIndent(list, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, CardFramesFile), b, 0o644)
}

// saveRim draws the rim sheet clipped to the rim's shape (both stretched over the same rect, as the game lays them out)
// and cuts it to the card (the shape's solid area).
func (x *extractor) saveRim(from *unityfs.File, sheetRef, maskRef unityfs.PPtr, path string) error {
	sheet, err := x.spriteImage(from, sheetRef)
	if err != nil {
		return fmt.Errorf("rim sheet: %w", err)
	}
	mask, err := x.spriteImage(from, maskRef)
	if err != nil {
		return fmt.Errorf("rim shape: %w", err)
	}
	r := mask.Bounds()
	out := image.NewNRGBA(r)
	draw.CatmullRom.Scale(out, r, sheet, sheet.Bounds(), draw.Src, nil)
	box := image.Rectangle{Min: r.Max, Max: r.Min}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := out.NRGBAAt(x, y)
			m := mask.NRGBAAt(x, y).A
			c.A = uint8(uint32(c.A) * uint32(m) / 255)
			out.SetNRGBA(x, y, c)
			if m > 8 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if box.Empty() {
		return fmt.Errorf("the rim shape is empty")
	}
	// Previews are small: at most rimHeight tall.
	w, h := box.Dx(), box.Dy()
	if h > rimHeight {
		w, h = w*rimHeight/h, rimHeight
	}
	card := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(card, card.Bounds(), out, box, draw.Src, nil)
	return imgfile.SaveFast(path, card)
}

// spriteImage reads a sprite's picture (its own rect of its texture or atlas).
func (x *extractor) spriteImage(from *unityfs.File, p unityfs.PPtr) (*image.NRGBA, error) {
	o, err := x.env.Resolve(from, p)
	if err != nil || o == nil || o.ClassID != unityfs.ClassSprite {
		return nil, fmt.Errorf("not a sprite (%v)", err)
	}
	s, err := unityfs.ReadSprite(o)
	if err != nil {
		return nil, err
	}
	img, err := x.env.SpriteImage(s)
	if err != nil {
		return nil, err
	}
	if img.Bounds().Empty() {
		return nil, fmt.Errorf("sprite %s is empty", s.Name)
	}
	return img, nil
}

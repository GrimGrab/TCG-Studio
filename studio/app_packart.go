package main

// Pack / box art editor: the accessory face editor on the card pack and card box models (uvmap "Pack" / "Box"). The editor
// composes the textures in the frontend; these methods store them in the project (same file names as the generator) and
// render the pack's shop icon from the finished texture. Layouts live in studio.json (project.Meta.PackArt).

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path"
	"path/filepath"
	"sort"
	"time"

	"tcgstudio/internal/art"
	"tcgstudio/internal/importer"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/uvmap"
)

// PackArtFiles are project-relative paths of one pack's (or box's) texture and icon.
type PackArtFiles struct {
	Texture string `json:"texture"`
	Icon    string `json:"icon"`
}

// packArtPrefix matches the generator's per-pack file names (images/<packId>_pack_texture.png; the first "booster" pack none).
func packArtPrefix(packID string) (string, error) {
	if packID == "" || packID == "booster" {
		return "images/", nil
	}
	if !setfmt.SafeID(packID) {
		return "", errors.New("bad pack id")
	}
	return "images/" + packID + "_", nil
}

// SavePackArt writes the editor's composed texture and icon (data URLs) for which = "pack" | "box" of a pack and returns their
// paths; the caller assigns them to the pack and saves the project.
func (a *App) SavePackArt(id, packID, which, texturePNG, iconPNG string) (PackArtFiles, error) {
	if !setfmt.SafeID(id) {
		return PackArtFiles{}, errors.New("bad project id")
	}
	if which != "pack" && which != "box" {
		return PackArtFiles{}, errors.New("which must be pack or box")
	}
	pre, err := packArtPrefix(packID)
	if err != nil {
		return PackArtFiles{}, err
	}
	folder := a.ws().Folder(id)
	out := PackArtFiles{Texture: pre + which + "_texture.png", Icon: pre + which + "_icon.png"}
	for _, f := range []struct{ data, rel string }{{texturePNG, out.Texture}, {iconPNG, out.Icon}} {
		b, err := decodeDataURL(f.data)
		if err != nil {
			return PackArtFiles{}, err
		}
		dst := filepath.Join(folder, filepath.FromSlash(f.rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return PackArtFiles{}, err
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return PackArtFiles{}, err
		}
	}
	return out, nil
}

// PackIconFromTexture renders a pack shop icon (data URL) from a composed pack texture (data URL): the vanilla pack icon with
// the front face warped in.
func (a *App) PackIconFromTexture(texturePNG string) (string, error) {
	if !a.templatesReady() {
		return "", errors.New("the game templates aren't available yet — check Settings → Game")
	}
	b, err := decodeDataURL(texturePNG)
	if err != nil {
		return "", err
	}
	tex, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	m, _ := uvmap.ModelFor("Pack")
	var front image.Rectangle
	for _, f := range m.Faces {
		if f.ID != "front" {
			continue
		}
		for _, t := range f.Targets {
			if !t.Bleed {
				front = image.Rect(int(t.Rect[0]), int(t.Rect[1]), int(t.Rect[2]), int(t.Rect[3]))
			}
		}
	}
	icon, err := art.PackIconFromTexture(a.templatesDir(), tex, front)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := art.EncodePNG(&buf, icon); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// SnapshotPackBase copies a project image (the pack's current art) to <prefix><which>_base.png so the editor can start from it
// without reading its own output back. Returns the copy's path.
func (a *App) SnapshotPackBase(id, packID, which, rel string) (string, error) {
	if !setfmt.SafeID(id) || (which != "pack" && which != "box") {
		return "", errors.New("bad arguments")
	}
	pre, err := packArtPrefix(packID)
	if err != nil {
		return "", err
	}
	folder := a.ws().Folder(id)
	src := filepath.Join(folder, filepath.FromSlash(rel))
	if r, err := filepath.Rel(folder, src); err != nil || r == ".." || filepath.IsAbs(r) || len(r) > 2 && r[:3] == ".."+string(filepath.Separator) {
		return "", errors.New("image must be inside the project")
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	out := pre + which + "_base.png"
	return out, os.WriteFile(filepath.Join(folder, filepath.FromSlash(out)), b, 0o644)
}

// ---------------------------------------------------------------- product photos (TCGplayer)

var sealedPhotos = importer.NewSealed()

// ProductPhotoSets is the photo search's start: the games, the TCGplayer sets of one game and the project's own set among
// them (0 = no match, the user picks).
type ProductPhotoSets struct {
	Games    []importer.SealedGame  `json:"games"`
	Category int                    `json:"category"`
	Groups   []importer.SealedGroup `json:"groups"`
	Match    int                    `json:"match"`
}

// ProductPhotoSets lists TCGplayer's sets for a project's game (category 0 = the game it was imported from, Magic for
// hand-made sets) and picks the project's set by its code and name.
func (a *App) ProductPhotoSets(id string, category int) (ProductPhotoSets, error) {
	p, err := a.ws().Load(id)
	if err != nil {
		return ProductPhotoSets{}, err
	}
	if category == 0 {
		if category = importer.SealedCategory(p.Meta.Source); category == 0 {
			category = 1
		}
	}
	ctx, cancel := context.WithTimeout(a.ctx, time.Minute)
	defer cancel()
	gs, match, err := sealedPhotos.Groups(ctx, category, p.Meta.SourceCode(), p.Set.Name)
	if err != nil {
		return ProductPhotoSets{}, err
	}
	return ProductPhotoSets{Games: importer.SealedGames, Category: category, Groups: gs, Match: match}, nil
}

// ProductPhotos lists a TCGplayer set's sealed products with photos (packs, then boxes, then the rest).
func (a *App) ProductPhotos(category, group int) ([]importer.SealedProduct, error) {
	ctx, cancel := context.WithTimeout(a.ctx, time.Minute)
	defer cancel()
	return sealedPhotos.Products(ctx, category, group)
}

// UseProductPhoto downloads a product photo into the project (white background cropped away) and returns its path, ready
// for an image layer. A photo downloaded before is reused.
func (a *App) UseProductPhoto(id string, p importer.SealedProduct) (string, error) {
	if !setfmt.SafeID(id) {
		return "", errors.New("bad project id")
	}
	rel := fmt.Sprintf("images/photo_%d.png", p.ID)
	if _, err := os.Stat(filepath.Join(a.ws().Folder(id), filepath.FromSlash(rel))); err == nil {
		return rel, nil
	}
	ctx, cancel := context.WithTimeout(a.ctx, time.Minute)
	defer cancel()
	img, err := sealedPhotos.Photo(ctx, p)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := art.EncodePNG(&buf, img); err != nil {
		return "", err
	}
	return a.writeIntoProject(id, path.Base(rel), buf.Bytes())
}

// ---------------------------------------------------------------- smart generate

// SmartArtCard is a card whose artwork the generator can use: its image and where the art sits on it (fractions).
type SmartArtCard struct {
	Name   string     `json:"name"`
	Image  string     `json:"image"`
	Window [4]float64 `json:"window"`
}

// SmartArtSources is everything Smart generate builds one pack's art from. Photos are project images (white background
// cropped); Box holds the display's front panel and lid as quads in BoxPhoto's pixels. Cards (best first) are always filled,
// for the no-photo path and the box's lid when there is no box photo.
type SmartArtSources struct {
	PackPhoto string            `json:"packPhoto"`
	PackName  string            `json:"packName"`
	BoxPhoto  string            `json:"boxPhoto"`
	BoxName   string            `json:"boxName"`
	Box       *art.DisplayFaces `json:"box"`
	Cards     []SmartArtCard    `json:"cards"`
	Icon      string            `json:"icon"` // set icon or logo, "" = none
	Notes     []string          `json:"notes"`
}

// SmartArtSources finds the photos for a pack (TCGplayer, pack n ↔ n-th booster kind) and ranks the pack's cards for
// their art. A failed photo lookup is not an error: the notes say why and the card art path is used.
// SmartChoice is what the Smart generate dialog picked. Manual false = choose automatically (imports): the project's set
// on TCGplayer, pack n ↔ n-th booster product. Manual true: Pack/Box as given (nil = none: card art instead).
type SmartChoice struct {
	Manual bool                    `json:"manual"`
	Pack   *importer.SealedProduct `json:"pack"`
	Box    *importer.SealedProduct `json:"box"`
}

// PickedProducts is the automatic choice of pack and box photos among a set's products.
type PickedProducts struct {
	Pack *importer.SealedProduct `json:"pack"`
	Box  *importer.SealedProduct `json:"box"`
}

// PickPackAndBox is the automatic choice for pack number index, for the dialog to start from.
func (a *App) PickPackAndBox(ps []importer.SealedProduct, index int) PickedProducts {
	pack, box := importer.PickPackAndBox(ps, index)
	return PickedProducts{pack, box}
}

// DetectBoxFaces finds the front panel and lid in a project's box photo (the dialog shows them to adjust).
func (a *App) DetectBoxFaces(id, rel string) (art.DisplayFaces, error) {
	if !setfmt.SafeID(id) {
		return art.DisplayFaces{}, errors.New("bad project id")
	}
	img, err := loadProjectImage(a.ws().Folder(id), rel)
	if err != nil {
		return art.DisplayFaces{}, err
	}
	return art.DetectDisplay(img), nil
}

func (a *App) SmartArtSources(id, packID string, choice SmartChoice) (SmartArtSources, error) {
	p, err := a.ws().Load(id)
	if err != nil {
		return SmartArtSources{}, err
	}
	out := SmartArtSources{Cards: []SmartArtCard{}, Notes: []string{}}
	index, cards := 0, map[string]bool{}
	for i, pk := range p.Set.Packs {
		if pk.ID == packID {
			index = i
			for _, c := range pk.Cards {
				cards[c] = true
			}
		}
	}
	for _, rel := range []string{"images/set_icon.svg", "images/set_logo.png"} {
		if _, err := os.Stat(filepath.Join(p.Folder, filepath.FromSlash(rel))); err == nil {
			out.Icon = rel
			break
		}
	}

	// Photos: the dialog's choice, or found automatically.
	use := func(pack, box *importer.SealedProduct) {
		if pack != nil {
			if rel, err := a.UseProductPhoto(id, *pack); err == nil {
				out.PackPhoto, out.PackName = rel, pack.Name
			} else {
				out.Notes = append(out.Notes, "Pack photo: "+err.Error())
			}
		}
		if box != nil {
			rel, err := a.UseProductPhoto(id, *box)
			if err != nil {
				out.Notes = append(out.Notes, "Box photo: "+err.Error())
				return
			}
			if img, err := loadProjectImage(p.Folder, rel); err == nil {
				f := art.DetectDisplay(img)
				out.BoxPhoto, out.BoxName, out.Box = rel, box.Name, &f
				if !f.Confident && !choice.Manual {
					out.Notes = append(out.Notes, "The box photo was hard to read — Smart generate in the pack editor lets you adjust the box faces.")
				}
			}
		}
	}
	if choice.Manual {
		use(choice.Pack, choice.Box)
	} else if cat := importer.SealedCategory(p.Meta.Source); cat != 0 {
		ctx, cancel := context.WithTimeout(a.ctx, 2*time.Minute)
		defer cancel()
		_, group, err := sealedPhotos.Groups(ctx, cat, p.Meta.SourceCode(), p.Set.Name)
		switch {
		case err != nil:
			out.Notes = append(out.Notes, "TCGplayer: "+err.Error())
		case group == 0:
			out.Notes = append(out.Notes, "This set wasn't found on TCGplayer — using card art (Smart generate in the pack editor lets you pick it).")
		default:
			ps, err := sealedPhotos.Products(ctx, cat, group)
			if err != nil {
				out.Notes = append(out.Notes, "TCGplayer: "+err.Error())
				break
			}
			pack, box := importer.PickPackAndBox(ps, index)
			use(pack, box)
			if pack == nil && box == nil {
				out.Notes = append(out.Notes, "TCGplayer has no pack or box photos for this set — using card art.")
			}
		}
	}

	// Cards, best first: real price (the higher of normal and foil), then game rarity.
	rank := map[string]int{}
	for i, r := range setfmt.Rarities {
		rank[r] = i
	}
	price := func(cid string) float64 {
		m, ok := p.Meta.Cards[cid]
		if !ok {
			return 0
		}
		v := 0.0
		for _, f := range []*float64{m.USD, m.USDFoil} {
			if f != nil && *f > v {
				v = *f
			}
		}
		return v
	}
	var list []setfmt.Card
	for _, c := range p.Set.Cards {
		if c.Image != "" && (len(cards) == 0 || cards[c.ID]) {
			list = append(list, c)
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		if pi, pj := price(list[i].ID), price(list[j].ID); pi != pj {
			return pi > pj
		}
		return rank[list[i].Rarity] > rank[list[j].Rarity]
	})
	for _, c := range list {
		if len(out.Cards) == 3 {
			break
		}
		w, h := 0, 0
		if cfg, err := imageConfig(filepath.Join(p.Folder, filepath.FromSlash(c.Image))); err == nil {
			w, h = cfg.Width, cfg.Height
		}
		out.Cards = append(out.Cards, SmartArtCard{Name: c.Name, Image: c.Image, Window: art.ArtWindow(p.Meta.Source, w, h)})
	}
	return out, nil
}

func loadProjectImage(folder, rel string) (image.Image, error) {
	f, err := os.Open(filepath.Join(folder, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func imageConfig(path string) (image.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Config{}, err
	}
	defer f.Close()
	c, _, err := image.DecodeConfig(f)
	return c, err
}

// SaveAutoArt stores an image Smart generate made (data URL) as images/auto_<pack>_<part>_<time>.png and removes the
// earlier versions of that part (a new name each time, so the editor never shows a cached old image). Returns its path.
func (a *App) SaveAutoArt(id, packID, part, dataURL string) (string, error) {
	if !setfmt.SafeID(id) || !setfmt.SafeID(part) || (packID != "" && !setfmt.SafeID(packID)) {
		return "", errors.New("bad arguments")
	}
	b, err := decodeDataURL(dataURL)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(a.ws().Folder(id), "images")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	stem := "auto_" + packID + "_" + part + "_"
	old, _ := filepath.Glob(filepath.Join(dir, stem+"*.png"))
	name := fmt.Sprintf("%s%d.png", stem, time.Now().UnixMilli())
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		return "", err
	}
	for _, f := range old {
		_ = os.Remove(f)
	}
	return "images/" + name, nil
}

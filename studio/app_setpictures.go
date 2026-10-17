package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"strings"

	"tcgstudio/internal/art"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

// ---------------------------------------------------------------- set pictures
//
// The set's card back and its packs' textures and icons can be cut from any picture with the crop box (the same one the Game
// look page uses). studio.json keeps the picture and the crop per slot (project.Meta.Pictures); SaveProject cuts each into
// the file set.json names. The file's name comes from picture + crop, so setups sharing the set's folder with another crop
// never overwrite each other's file, and an unchanged slot isn't cut again.

// setPicture is one kind of set file made from a picture.
type setPicture struct {
	title  string                                              // of the file dialog
	aspect float64                                             // width ÷ height of the crop box
	size   image.Point                                         // the file's pixels; zero = the cropped part's own (at most iconMaxSide)
	ref    func(p *project.Project, pack *setfmt.Pack) *string // the set.json field naming the file
	card   bool                                                // a card back: cut like the global one (art.CardBackFromCrop)
}

// Pack art: 1024² textures in the vanilla layout; icons any size in the vanilla icons' shape (the mod pads them: pack pixels
// 521×1001, box 512×437).
const iconMaxSide = 1024

var setPictures = map[string]setPicture{
	"cardBack": {"Choose a card back image", art.CardBackAspect, image.Point{},
		func(p *project.Project, _ *setfmt.Pack) *string { return &p.Set.CardBack }, true},
	"packTexture": {"Choose the pack texture", 1, image.Pt(1024, 1024),
		func(_ *project.Project, pk *setfmt.Pack) *string { return &pk.PackTexture }, false},
	"packIcon": {"Choose the pack icon", 521.0 / 1001, image.Point{},
		func(_ *project.Project, pk *setfmt.Pack) *string { return &pk.PackIcon }, false},
	"boxTexture": {"Choose the box texture", 1, image.Pt(1024, 1024),
		func(_ *project.Project, pk *setfmt.Pack) *string { return &pk.BoxTexture }, false},
	"boxIcon": {"Choose the box icon", 512.0 / 437, image.Point{},
		func(_ *project.Project, pk *setfmt.Pack) *string { return &pk.BoxIcon }, false},
}

func (sp setPicture) cut(src string, crop []float64, out string) error {
	if sp.card {
		return art.CardBackFromCrop(src, crop, out)
	}
	return art.CropToFile(src, crop, sp.aspect, sp.size, iconMaxSide, out)
}

// SetPictureAspects are the crop box shapes (width ÷ height) by kind: cardBack, packTexture, packIcon, boxTexture, boxIcon.
func (a *App) SetPictureAspects() map[string]float64 {
	out := map[string]float64{}
	for k, sp := range setPictures {
		out[k] = sp.aspect
	}
	return out
}

// parseSetPictureSlot splits "cardBack" / "pack/<pack id>/<field>" into the kind and the pack id ("" for the card back).
func parseSetPictureSlot(slot string) (setPicture, string, error) {
	kind, packID := slot, ""
	if rest, ok := strings.CutPrefix(slot, "pack/"); ok {
		i := strings.LastIndex(rest, "/")
		if i <= 0 {
			return setPicture{}, "", errors.New("bad picture slot " + slot)
		}
		packID, kind = rest[:i], rest[i+1:]
	}
	sp, ok := setPictures[kind]
	if !ok || (packID == "") != (kind == "cardBack") {
		return setPicture{}, "", errors.New("unknown picture slot " + slot)
	}
	return sp, packID, nil
}

// SetPictureSource is a picture chosen for a set slot: its set file and the crop to start from (nil = the middle).
type SetPictureSource struct {
	File string    `json:"file"` // "" = cancelled
	Crop []float64 `json:"crop"`
}

// PickSetPicture lets the user choose a picture for a set slot and copies it into the set's folder. Nothing else changes until
// the set is saved with the picture in studio.json's pictures (SaveProject then cuts it). The card back's crop starts inside
// plain borders (a scanned card's padding), like the global one.
func (a *App) PickSetPicture(id, slot string) (SetPictureSource, error) {
	sp, _, err := parseSetPictureSlot(slot)
	if err != nil {
		return SetPictureSource{}, err
	}
	file, err := a.pickImageFile(sp.title)
	if err != nil || file == "" {
		return SetPictureSource{}, err
	}
	rel, err := a.copyIntoProject(id, file)
	if err != nil {
		return SetPictureSource{}, err
	}
	src := SetPictureSource{File: rel}
	if slot == "cardBack" {
		src.Crop, _ = art.ImageCardBackAutoCrop(file)
	}
	return src, nil
}

// bakeSetPictures cuts every picture slot of the project into its set file and points set.json at it; returns slot → file,
// "" for a dropped slot: its pack no longer exists, or set.json names another file than the one last cut (replaced since).
func bakeSetPictures(p *project.Project) (map[string]string, error) {
	baked := map[string]string{}
	if p.Meta == nil {
		return baked, nil
	}
	for slot, ps := range p.Meta.Pictures {
		sp, packID, err := parseSetPictureSlot(slot)
		if err != nil {
			return nil, err
		}
		var pack *setfmt.Pack
		if packID != "" {
			for i := range p.Set.Packs {
				if p.Set.Packs[i].ID == packID {
					pack = &p.Set.Packs[i]
				}
			}
		}
		if (packID != "" && pack == nil) || (ps.Out != "" && *sp.ref(p, pack) != ps.Out) {
			delete(p.Meta.Pictures, slot)
			baked[slot] = ""
			continue
		}
		if !project.IsRelFile(ps.File) || !fsutil.Exists(p.ImagePath(ps.File)) {
			return nil, fmt.Errorf("the picture for %s is missing: %s", slot, ps.File)
		}
		if len(ps.Crop) != 0 && len(ps.Crop) != 4 {
			return nil, errors.New("crop needs x, y, width and height")
		}
		sum := sha256.Sum256([]byte(fmt.Sprint(slot, ps.File, ps.Crop)))
		name := strings.ReplaceAll(strings.ReplaceAll(slot, "/", "_"), " ", "_")
		rel, out := p.WriteTarget("images/cut_" + name + "_" + hex.EncodeToString(sum[:4]) + ".png")
		if !fsutil.Exists(out) {
			if err := sp.cut(p.ImagePath(ps.File), ps.Crop, out); err != nil {
				return nil, fmt.Errorf("%s: %w", slot, err)
			}
		}
		*sp.ref(p, pack) = rel
		ps.Out = rel
		p.Meta.Pictures[slot] = ps
		baked[slot] = rel
	}
	return baked, nil
}

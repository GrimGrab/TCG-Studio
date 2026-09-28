package main

// Pack / box art editor: the accessory face editor on the card pack and card box models (uvmap "Pack" / "Box"). The editor
// composes the textures in the frontend; these methods store them in the project (same file names as the generator) and
// render the pack's shop icon from the finished texture. Layouts live in studio.json (project.Meta.PackArt).

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	_ "image/png"
	"os"
	"path/filepath"

	"tcgstudio/internal/art"
	"tcgstudio/internal/game"
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
	if !game.GetStatus(a.settings.GameDir).TemplatesFound {
		return "", errors.New("pack templates not found — load a save once with the mod installed (it exports them)")
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
	icon, err := art.PackIconFromTexture(game.TemplatesDir(a.settings.GameDir), tex, front)
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

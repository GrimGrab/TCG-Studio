package relayout

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"tcgstudio/internal/imgfile"
	"tcgstudio/internal/project"
	"tcgstudio/internal/uvmap"
)

// packKinds: the pack editor's two models, with the set.json field holding each one's texture.
var packKinds = []struct {
	which, kind string
	texture     func(*packRef) *string
}{
	{"box", "Box", func(p *packRef) *string { return &p.box }},
	{"pack", "Pack", func(p *packRef) *string { return &p.pack }},
}

type packRef struct{ pack, box string }

// Project converts one set's pack and box art made for the game 1.02 models to the current ones (only kinds whose current
// layout differs: the face maps): each texture is redrawn into a new file (the old one stays — another setup may
// still use it), the design's map layers move, and pictures the design names (baseFile, textureFile) are redrawn too.
// tplDir is the game templates folder (the new vanilla print around the faces); kinds = the uvmap kinds to convert.
// changed = the project needs saving.
func Project(p *project.Project, tplDir string, kinds []string) (changed bool, err error) {
	for i := range p.Set.Packs {
		pk := &p.Set.Packs[i]
		ref := packRef{pack: pk.PackTexture, box: pk.BoxTexture}
		for _, k := range packKinds {
			if !slices.Contains(kinds, k.kind) {
				continue
			}
			from, _ := uvmap.LegacyModelFor(k.kind)
			to, ok := uvmap.ModelFor(k.kind)
			if !ok || to.Mesh == from.Mesh {
				continue // this kind has no new layout (yet)
			}
			var raw json.RawMessage
			if p.Meta != nil && p.Meta.PackArt[pk.ID] != nil {
				raw = p.Meta.PackArt[pk.ID][k.which]
			}
			design := parseDesign(raw)
			tex := k.texture(&ref)
			if project.IsRelFile(*tex) {
				if _, err := os.Stat(p.ImagePath(*tex)); err == nil {
					rel, err := convertFile(p, *tex, from, to, background(to, design, tplDir))
					if err != nil {
						return changed, fmt.Errorf("%s %s: %w", pk.ID, k.which, err)
					}
					*tex, changed = rel, true
				}
			}
			if raw != nil {
				out, err := Layout(from, to, raw)
				if err != nil {
					return changed, fmt.Errorf("%s %s design: %w", pk.ID, k.which, err)
				}
				if out, err = convertDesignFiles(p, out, from, to, background(to, design, tplDir)); err != nil {
					return changed, fmt.Errorf("%s %s design: %w", pk.ID, k.which, err)
				}
				p.Meta.PackArt[pk.ID][k.which] = out
				changed = true
			}
		}
		pk.PackTexture, pk.BoxTexture = ref.pack, ref.box
	}
	return changed, nil
}

// design is the part of a Layout the conversion needs.
type design struct {
	Base        string `json:"base"`      // "vanilla" | "color"
	BaseColor   string `json:"baseColor"` // "#rrggbb"
	BaseItem    string `json:"baseItem"`  // vanilla item the art starts from, or "file"
	BaseFile    string `json:"baseFile"`
	TextureFile string `json:"textureFile"`
}

func parseDesign(raw json.RawMessage) design {
	var d design
	if raw != nil {
		_ = json.Unmarshal(raw, &d)
	}
	return d
}

// background: what shows around the faces in the new layout — the base's colour for a colour design, else the new vanilla
// print of its base item (the model's first base when it names none).
func background(to uvmap.Model, d design, tplDir string) image.Image {
	if d.Base == "color" {
		if c, ok := parseHex(d.BaseColor); ok {
			return &image.Uniform{C: c}
		}
	}
	if len(to.Bases) == 0 || tplDir == "" {
		return nil
	}
	file := to.Bases[0].Texture
	for _, b := range to.Bases {
		if b.ID == d.BaseItem {
			file = b.Texture
		}
	}
	return templateImage(filepath.Join(tplDir, file))
}

// templateImages: decoded vanilla textures by path and version (the same few base textures serve every set of a conversion; workers
// convert sets at the same time).
var (
	templateMu     sync.Mutex
	templateImages = map[string]image.Image{}
)

// templateImage is a game template picture, decoded once per path and version (nil when unreadable; a new extraction of
// the game files gets new modification times).
func templateImage(path string) image.Image {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	key := path + "|" + info.ModTime().String()
	templateMu.Lock()
	defer templateMu.Unlock()
	if img, ok := templateImages[key]; ok {
		return img
	}
	img, err := imgfile.Load(path)
	if err != nil {
		img = nil
	}
	templateImages[key] = img
	return img
}

// convertFile redraws a set picture in the new layout into a new file next to it; returns its set path.
func convertFile(p *project.Project, rel string, from, to uvmap.Model, bg image.Image) (string, error) {
	img, err := imgfile.Load(p.ImagePath(rel))
	if err != nil {
		return "", err
	}
	b, err := imgfile.EncodeFast(Image(from, to, img, bg))
	if err != nil {
		return "", err
	}
	out := project.FreshName(strings.TrimSuffix(rel, filepath.Ext(rel)) + ".png")
	return p.WriteFile(out, b)
}

// convertDesignFiles redraws the whole-texture pictures a design names (its starting snapshot, an imported painted texture)
// over bg like the art file: the editor draws from them, and transparent parts would come out black around the faces.
func convertDesignFiles(p *project.Project, raw json.RawMessage, from, to uvmap.Model, bg image.Image) (json.RawMessage, error) {
	var l map[string]json.RawMessage
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, err
	}
	for _, key := range []string{"baseFile", "textureFile"} {
		var rel string
		if v, ok := l[key]; !ok || json.Unmarshal(v, &rel) != nil || !project.IsRelFile(rel) {
			continue
		}
		if _, err := os.Stat(p.ImagePath(rel)); err != nil {
			continue
		}
		out, err := convertFile(p, rel, from, to, bg)
		if err != nil {
			return nil, err
		}
		l[key], _ = json.Marshal(out)
	}
	return json.Marshal(l)
}

func parseHex(s string) (color.NRGBA, bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return color.NRGBA{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return color.NRGBA{}, false
	}
	return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, true
}

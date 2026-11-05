package relayout

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/imgfile"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/uvmap"
)

// Accessory art keys: an accessory kind, or for the card supplies (one layout per base) the base. These are the keys a
// conversion runs per (Studio records them like set kinds).
var accessoryKeys = []string{"Deckbox", "Sleeve", "BattleDeck", "Binder", "Dice", "UP_Playmat1", "UP_Album"}

// AccessoryKeys lists the accessory art keys in conversion order.
func AccessoryKeys() []string { return slices.Clone(accessoryKeys) }

// KeyOf is the art key of an accessory ("" = its art has no layout to convert: an own model's texture follows that model).
func KeyOf(a setfmt.Accessory) string {
	if a.Mesh != "" {
		return ""
	}
	k := a.Kind
	if a.Kind == "CardSupply" {
		k = a.DefaultBase()
	}
	if !slices.Contains(accessoryKeys, k) {
		return ""
	}
	return k
}

// Models are the earlier (game 1.02) and current layouts of a set kind or accessory art key; ok = the layout changed.
func Models(key string) (from, to uvmap.Model, ok bool) {
	kind, base := key, ""
	if strings.HasPrefix(key, "UP_") {
		kind, base = "CardSupply", key
	}
	from, _ = uvmap.LegacyModelForBase(kind, base)
	to, found := uvmap.ModelForBase(kind, base)
	return from, to, found && to.Mesh != from.Mesh
}

// PendingAccessoryKeys lists those of keys with an accessory whose art is still in the 1.02 layout.
func PendingAccessoryKeys(l *accessories.Library, keys []string) []string {
	var out []string
	for _, a := range l.Lib.Accessories {
		if k := KeyOf(a); k != "" && !l.Meta.ArtCurrent[a.ID] && slices.Contains(keys, k) && !slices.Contains(out, k) {
			out = append(out, k)
		}
	}
	return out
}

// Accessories converts the library's accessory art made for the game 1.02 models (keys = the art keys to convert): each
// texture is redrawn into a new file in the shared store (the old one stays — another setup may still use it), the design's
// map layers move, and pictures the design names are redrawn too. changed = the library needs saving.
// A bad accessory stays as it was (err names it) and doesn't hold up the others: save the library when changed, err or not.
func Accessories(l *accessories.Library, tplDir string, keys []string) (changed bool, err error) {
	var failed []string
	for i := range l.Lib.Accessories {
		a := &l.Lib.Accessories[i]
		key := KeyOf(*a)
		if key == "" || l.Meta.ArtCurrent[a.ID] || !slices.Contains(keys, key) {
			continue
		}
		from, to, ok := Models(key)
		if !ok {
			continue
		}
		if err := convertAccessory(l, a, from, to, tplDir); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", a.Name, err))
			continue
		}
		l.Meta.ArtCurrent[a.ID], changed = true, true
	}
	if len(failed) > 0 {
		return changed, errors.New(strings.Join(failed, ", "))
	}
	return changed, nil
}

// convertAccessory redraws one accessory's art and design; a failure leaves it unchanged.
func convertAccessory(l *accessories.Library, a *setfmt.Accessory, from, to uvmap.Model, tplDir string) error {
	raw := l.Meta.Layouts[a.ID]
	bg := accessoryBackground(to, parseDesign(raw), tplDir, a.DefaultBase())
	texture := a.Texture
	if texture != "" {
		if _, err := os.Stat(l.Resolve(texture)); err == nil {
			if texture, err = convertStored(l, texture, from, to, bg); err != nil {
				return err
			}
		}
	}
	if raw != nil {
		out, err := Layout(from, to, raw)
		if err != nil {
			return fmt.Errorf("design: %w", err)
		}
		if out, err = convertStoredDesignFiles(l, out, from, to, bg); err != nil {
			return fmt.Errorf("design: %w", err)
		}
		l.Meta.Layouts[a.ID] = out
	}
	a.Texture = texture
	return nil
}

// accessoryBackground: the base's colour for a colour design, else the new vanilla texture of the accessory's base item.
func accessoryBackground(to uvmap.Model, d design, tplDir, base string) image.Image {
	if d.Base == "color" || tplDir == "" {
		return background(to, d, "")
	}
	return templateImage(filepath.Join(tplDir, "accessories", base+"_texture.png"))
}

// convertStored redraws a stored picture in the new layout into a new file of the shared store; returns its path.
func convertStored(l *accessories.Library, rel string, from, to uvmap.Model, bg image.Image) (string, error) {
	img, err := imgfile.Load(l.Resolve(rel))
	if err != nil {
		return "", err
	}
	b, err := imgfile.EncodeFast(Image(from, to, img, bg))
	if err != nil {
		return "", err
	}
	return l.PutBytes(b, ".png")
}

// convertStoredDesignFiles is convertDesignFiles for a design whose pictures are in the shared store.
func convertStoredDesignFiles(l *accessories.Library, raw json.RawMessage, from, to uvmap.Model, bg image.Image) (json.RawMessage, error) {
	var d map[string]json.RawMessage
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	for _, key := range []string{"baseFile", "textureFile"} {
		var rel string
		if v, ok := d[key]; !ok || json.Unmarshal(v, &rel) != nil || rel == "" {
			continue
		}
		if _, err := os.Stat(l.Resolve(rel)); err != nil {
			continue
		}
		out, err := convertStored(l, rel, from, to, bg)
		if err != nil {
			return nil, err
		}
		d[key], _ = json.Marshal(out)
	}
	return json.Marshal(d)
}

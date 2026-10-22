package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/gameextract"
	"tcgstudio/internal/modconfig"
	"tcgstudio/internal/uvmap"
)

// ---------------------------------------------------------------- Game look page: Boxes
//
// The cardboard boxes, the bulk card boxes, the shopping bag and the street billboards, painted in the shared face editor
// (AccessoryEditor) over the vanilla UVs. Save writes the whole texture as box_<id>_<hash>.png into the setup's ShopSign
// folder with the editor layout next to it (designFile), and names it in the look's mod setting ([Visuals - Game look -
// Boxes] / [Visuals - Game look - Bags & billboards], read by the mod's Runtime/BoxLook). A look with a twin (the shopping
// bag: closed + open) gets a second texture from the same design over the twin's vanilla texture.
// Images placed in the editor are copied there too (boximg_*), so a setup carries everything; pruneFiles keeps what a
// saved layout names.

const (
	boxLookSection   = "Visuals - Game look - Boxes"
	sceneLookSection = "Visuals - Game look - Bags & billboards"
	boxPrefix        = "box_"    // painted textures
	boxImagePrefix   = "boximg_" // images placed in the box editor
)

// boxLook is one look of the tab: its mod setting, editor layout (uvmap.BoxLook) and vanilla texture (templates
// boxes\<id>.png); twin = a second vanilla texture (template id) painted with the same design, set in twinKey.
type boxLook struct {
	id, key, model, title, group string
	section, twin, twinKey       string
}

// boxLooks in the order the page lists them; the bulk boxes are the game's BulkBox_* items (the mod's BoxLook.BulkBoxes).
var boxLooks = func() []boxLook {
	out := []boxLook{
		{id: "restockBig", key: "RestockBoxBigTexture", model: "restockBig", title: "Restock box (big)", group: "cardboard"},
		{id: "restockSmall", key: "RestockBoxSmallTexture", model: "restockSmall", title: "Restock box (small)", group: "cardboard"},
		{id: "furnitureBox", key: "FurnitureBoxTexture", model: "furnitureBox", title: "Furniture box", group: "cardboard"},
		{id: "gradedBox", key: "GradedBoxTexture", model: "gradedBox", title: "Graded-card box", group: "cardboard"},
		{id: "shoppingBag", key: "ShoppingBagTexture", model: "bag", title: "Shopping bag", group: "bag",
			section: sceneLookSection, twin: "shoppingBagOpen", twinKey: "ShoppingBagOpenTexture"},
		{id: "streetBillboard", key: "StreetBillboardTexture", model: "streetBillboard", title: "Street billboards", group: "street",
			section: sceneLookSection},
	}
	for _, v := range [][2]string{{"Base", "Tetramon"}, {"Destiny", "Destiny"},
		{"BaseGradeHG", "Tetramon HG"}, {"BaseGradeMG", "Tetramon MG"}, {"BaseGradePG", "Tetramon PG"}, {"BaseGradeUG", "Tetramon UG"},
		{"DestinyGradeHG", "Destiny HG"}, {"DestinyGradeMG", "Destiny MG"}, {"DestinyGradePG", "Destiny PG"}, {"DestinyGradeUG", "Destiny UG"}} {
		id := "BulkBox_Tetramon" + v[0]
		out = append(out, boxLook{id: id, key: id + "Texture", model: "bulk", title: "Bulk box: " + v[1], group: "bulk"})
	}
	for i := range out {
		if out[i].section == "" {
			out[i].section = boxLookSection
		}
	}
	return out
}()

func boxLookByID(id string) (boxLook, bool) {
	for _, b := range boxLooks {
		if b.id == id {
			return b, true
		}
	}
	return boxLook{}, false
}

// BoxLookView is one box of the page.
type BoxLookView struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Group   string `json:"group"`   // "cardboard", "bulk", "bag" or "street"
	Model   string `json:"model"`   // editor layout (BoxLookModel)
	Vanilla string `json:"vanilla"` // URL of the vanilla texture, "" = templates not read yet
	File    string `json:"file"`    // the painted texture in the ShopSign folder, "" = vanilla
	Image   string `json:"image"`   // its URL
	Layout  string `json:"layout"`  // the editor layout it was painted with (JSON), "" = none kept
	Twin    string `json:"twin"`    // URL of the twin's vanilla texture (the design is painted over it too), "" = none
}

func (a *App) boxLookViews(p string) []BoxLookView {
	out := []BoxLookView{}
	for _, b := range boxLooks {
		v := BoxLookView{ID: b.id, Title: b.title, Group: b.group, Model: b.model}
		if exists(a.templateFile(gameextract.BoxesDir, b.id+".png")) {
			v.Vanilla = "/boxtemplates/" + b.id + ".png"
		}
		if b.twin != "" && exists(a.templateFile(gameextract.BoxesDir, b.twin+".png")) {
			v.Twin = "/boxtemplates/" + b.twin + ".png"
		}
		if e, err := modconfig.Get(p, b.section, b.key); err == nil && e.Value != "" {
			if v.Image = a.shopSignURL(e.Value); v.Image != "" {
				v.File = e.Value
				if lay, err := os.ReadFile(filepath.Join(a.shopSignDir(), designFile(e.Value))); err == nil {
					v.Layout = string(lay)
				}
			}
		}
		out = append(out, v)
	}
	return out
}

// BoxLookModel is a box's editor layout, its 3D preview served from the templates' boxes folder.
func (a *App) BoxLookModel(model string) (uvmap.Model, error) {
	m, ok := uvmap.BoxLook(model)
	if !ok {
		return m, errors.New("no layout for box " + model)
	}
	m.Preview = []uvmap.PreviewPart{{Mesh: m.Mesh, URL: "/boxtemplates/" + m.Mesh + ".obj", Texture: "main"}}
	return m, nil
}

// ImportBoxImage lets the user pick an image for the box editor and copies it into the ShopSign folder (File "" = cancelled).
func (a *App) ImportBoxImage() (ShopFile, error) {
	return a.importPicture("Choose an image", boxImagePrefix)
}

// SaveBoxImage stores an image the box editor made (a straightened photo); returns its file name.
func (a *App) SaveBoxImage(name, dataURL string) (string, error) {
	b, err := decodeDataURL(dataURL)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(a.shopSignDir(), 0o755); err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	file := boxImagePrefix + hex.EncodeToString(sum[:4]) + "_" + strings.ReplaceAll(safeFileName(name), listSeparator, "_")
	return file, os.WriteFile(filepath.Join(a.shopSignDir(), file), b, 0o644)
}

// BoxLookSave is one look of the draft: the editor's texture (PNG data URL), the same design over the twin's vanilla
// texture (looks with a twin) and the layout; PNG "" = back to vanilla.
type BoxLookSave struct {
	PNG     string `json:"png"`
	TwinPNG string `json:"twinPng"`
	Layout  string `json:"layout"`
}

// saveBoxLooks writes the draft's boxes; returns the settings to set and the files they replace. A new file name per texture:
// the mod reloads a box's texture when its setting changes.
func (a *App) saveBoxLooks(p string, boxes map[string]BoxLookSave) (kvs [][3]string, replaced []string, err error) {
	for id, s := range boxes {
		b, ok := boxLookByID(id)
		if !ok {
			return nil, nil, errors.New("unknown box " + id)
		}
		name, twin := "", ""
		if s.PNG != "" {
			var lay map[string]any
			if s.Layout != "" && json.Unmarshal([]byte(s.Layout), &lay) != nil {
				return nil, nil, errors.New("bad layout for " + b.title)
			}
			if name, err = a.writeBoxTexture(id, s.PNG, b.title); err != nil {
				return nil, nil, err
			}
			if s.Layout != "" {
				if err := os.WriteFile(filepath.Join(a.shopSignDir(), designFile(name)), []byte(s.Layout), 0o644); err != nil {
					return nil, nil, err
				}
			}
			if b.twin != "" {
				if s.TwinPNG == "" {
					return nil, nil, errors.New("missing second texture for " + b.title)
				}
				if twin, err = a.writeBoxTexture(b.twin, s.TwinPNG, b.title); err != nil {
					return nil, nil, err
				}
			}
		}
		set := func(key, value string) {
			if old, err := modconfig.Get(p, b.section, key); err == nil && old.Value != "" && old.Value != value {
				replaced = append(replaced, old.Value)
			}
			kvs = append(kvs, [3]string{b.section, key, value})
		}
		set(b.key, name)
		if b.twinKey != "" {
			set(b.twinKey, twin)
		}
	}
	return kvs, replaced, nil
}

// writeBoxTexture stores one painted texture (PNG data URL) as box_<id>_<hash>.png; returns its file name.
func (a *App) writeBoxTexture(id, dataURL, title string) (string, error) {
	png, err := decodeDataURL(dataURL)
	if err != nil || len(png) < 8 || string(png[1:4]) != "PNG" {
		return "", errors.New("bad texture for " + title)
	}
	if err := os.MkdirAll(a.shopSignDir(), 0o755); err != nil {
		return "", err
	}
	sum := sha256.Sum256(png)
	name := boxPrefix + id + "_" + hex.EncodeToString(sum[:4]) + ".png"
	return name, os.WriteFile(filepath.Join(a.shopSignDir(), name), png, 0o644)
}

// boxFilesInUse are the painted textures the look settings name.
func boxFilesInUse(p string) []string {
	var out []string
	for _, b := range boxLooks {
		for _, key := range []string{b.key, b.twinKey} {
			if key == "" {
				continue
			}
			if e, err := modconfig.Get(p, b.section, key); err == nil && e.Value != "" {
				out = append(out, e.Value)
			}
		}
	}
	return out
}

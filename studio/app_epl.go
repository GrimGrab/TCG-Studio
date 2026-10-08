package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/epl"
	"tcgstudio/internal/importer"
)

// ---------------------------------------------------------------- Enhanced Prefab Loader mods (Import sets → "EPL mod")
// The mod is read where it is (folder) or unpacked once into the cache (zip); ImportSet("epl", set.code, opt) converts
// one of its card sets. See internal/epl and importer/epl.go.

// figurineSizes.json: the vanilla toys' shelf slots and sizes (also used by the Figurine editor), for converted figurines.
//
//go:embed frontend/src/lib/figurineSizes.json
var figurineSizesJSON []byte

func init() {
	var d struct {
		Items []importer.ToySize `json:"items"`
	}
	if json.Unmarshal(figurineSizesJSON, &d) == nil {
		importer.FigurineToys = d.Items
	}
}

// PickEPLMod asks for an EPL mod with one dialog for both kinds (Windows has no file-or-folder dialog): its download
// (.zip/.rar/.7z), or any file inside the unpacked mod's folder — epl.ModPath walks up to the mod. All files are shown first
// so an unpacked folder's contents are clickable. Returns the mod's path for PreviewEPL ("" when cancelled); dropping works too.
func (a *App) PickEPLMod() (string, error) {
	p, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select an Enhanced Prefab Loader mod: its .zip/.rar/.7z, or any .json in its folder",
		Filters: []runtime.FileFilter{{DisplayName: "EPL mod (.zip/.rar/.7z, or a .json in the mod folder)", Pattern: "*.zip;*.rar;*.7z;*.json"},
			{DisplayName: "All files", Pattern: "*.*"}}})
	if err != nil || p == "" {
		return "", err
	}
	return epl.ModPath(p)
}

// PickEPLFolder asks for an unpacked mod's folder ("" when cancelled). Windows has no file-or-folder dialog, so the Import page
// has this next to Choose download… (archives).
func (a *App) PickEPLFolder() (string, error) {
	p, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Select the unpacked Enhanced Prefab Loader mod's folder"})
	if err != nil || p == "" {
		return "", err
	}
	return epl.ModPath(p)
}

// EPLModPath turns a dropped folder or file into the mod's path for PreviewEPL (see epl.ModPath).
func (a *App) EPLModPath(dropped string) (string, error) { return epl.ModPath(dropped) }

// PreviewEPL reads a mod for the Import page (unpacking a zip's bundles first, with "import:progress" events).
func (a *App) PreviewEPL(path string, strip bool) (importer.EPLPreview, error) {
	a.useEPLCache()
	pv, err := importer.ScanEPL(path, strip, func(done, total int64) {
		runtime.EventsEmit(a.ctx, "import:progress", importer.Progress{Stage: "unpack", Done: int(done >> 20),
			Total: int(total >> 20), Message: "Unpacking the mod…"})
	})
	if err != nil {
		return pv, err
	}
	for i := range pv.Sets {
		_, err := os.Stat(a.ws().Folder(pv.Sets[i].ProjectID))
		pv.Sets[i].Imported = err == nil
	}
	if l, err := a.accLib(); err == nil {
		for i := range pv.Items {
			pv.Items[i].Exists = l.HasItem(pv.Items[i].ID)
		}
	}
	return pv, nil
}

// EPLSelection is what the player ticked in the preview of a mod.
type EPLSelection struct {
	Sets    []string                     `json:"sets"`   // set codes
	Rarity  map[string]map[string]string `json:"rarity"` // set code → mod tier → game rarity
	Items   []string                     `json:"items"`  // item keys (accessories)
	Options importer.Options             `json:"options"`
}

type EPLResult struct {
	Sets     []string           `json:"sets"`  // imported project ids
	Items    int                `json:"items"` // accessories converted
	Icons    []importer.IconJob `json:"icons"` // accessories whose shop icon the page renders from their texture
	Problems []string           `json:"problems"`
}

// ImportEPL converts the chosen parts of a mod in one go: its card sets (as ImportSet would, one after another) and its
// accessories (into the accessory library, installed to the game when it is set up). A failed set doesn't stop the rest.
func (a *App) ImportEPL(path string, sel EPLSelection) (EPLResult, error) {
	a.useEPLCache()
	res := EPLResult{Sets: []string{}, Icons: []importer.IconJob{}, Problems: []string{}}
	for _, code := range sel.Sets {
		opt := sel.Options
		opt.RarityMap = sel.Rarity[code]
		id, err := a.ImportSet("epl", code, opt)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return res, err
			}
			_, _, exp := importer.SplitEPLCode(code)
			res.Problems = append(res.Problems, exp+": "+err.Error())
			continue
		}
		res.Sets = append(res.Sets, id)
	}
	if len(sel.Items) > 0 {
		started := time.Now().Add(-time.Second)
		defer a.catalogItemsSince(started) // template what the import made (after saving below)
		l, err := a.accLib()
		if err != nil {
			return res, err
		}
		n, icons, failed, err := importer.ImportEPLItems(path, sel.Items, sel.Options, l, func(pr importer.Progress) {
			runtime.EventsEmit(a.ctx, "import:progress", pr)
		})
		res.Items = n
		res.Icons = append(res.Icons, icons...)
		res.Problems = append(res.Problems, failed...)
		if err != nil {
			res.Problems = append(res.Problems, err.Error())
		}
		if n > 0 {
			if _, err := a.saveAndInstall(l); err != nil {
				res.Problems = append(res.Problems, err.Error())
			}
		}
	}
	return res, nil
}

// SetAccessoryIcons stores shop icons rendered by the page (accessory id → PNG data URL) and reinstalls the library once.
func (a *App) SetAccessoryIcons(icons map[string]string) error {
	l, err := a.accLib()
	if err != nil {
		return err
	}
	for id, png := range icons {
		i := l.Index(id)
		if i < 0 {
			return errors.New("accessory " + id + " not found")
		}
		b, err := decodeDataURL(png)
		if err != nil {
			return err
		}
		rel, err := l.WriteImage(id, "icon", b)
		if err != nil {
			return err
		}
		l.Lib.Accessories[i].Icon = rel
	}
	_, err = a.saveAndInstall(l)
	ids := make([]string, 0, len(icons))
	for id := range icons {
		ids = append(ids, id)
	}
	a.catalogItems(ids)
	return err
}

func (a *App) useEPLCache() {
	importer.EPLCacheDir = filepath.Join(a.home().CacheDir())
}

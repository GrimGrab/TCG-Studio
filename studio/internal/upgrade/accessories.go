package upgrade

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/catalog"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

// Accessory, furniture and decoration files of the older layout: kept in a setup's accessories\images and referred to
// by that path ("images/<id>_texture.png") instead of the shared store ("assets/<hash>.png").

const oldImagesDir = "images"

// oldPath is a setup's own file of the older layout ("" for store files and paths outside the folder).
func oldPath(l *accessories.Library, rel string) string {
	if rel == "" || assets.IsAsset(rel) {
		return ""
	}
	p := filepath.Join(l.Folder, filepath.FromSlash(rel))
	if r, err := filepath.Rel(l.Folder, p); err != nil || strings.HasPrefix(r, "..") {
		return ""
	}
	return p
}

// itemFiles lists the setup files (not store files) a library refers to that exist.
func itemFiles(l *accessories.Library) []string {
	var out []string
	for _, rel := range l.Files() {
		if p := oldPath(l, rel); p != "" && fsutil.Exists(p) {
			out = append(out, rel)
		}
	}
	return out
}

// itemLeftovers lists files under a library's images folder that no entry refers to.
func itemLeftovers(l *accessories.Library) []string {
	used := map[string]bool{}
	for _, rel := range l.Files() {
		if p := oldPath(l, rel); p != "" {
			used[filepath.Clean(p)] = true
		}
	}
	var out []string
	_ = filepath.WalkDir(filepath.Join(l.Folder, oldImagesDir), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !used[filepath.Clean(p)] {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// checkAccessories lists every setup's own item files and leftovers.
func checkAccessories(h setups.Home) ([]File, error) {
	list, err := h.List()
	if err != nil {
		return nil, err
	}
	var out []File
	for _, s := range list {
		l, err := workspace.Dir(h.Root).Items(s.Folder)
		if err != nil {
			continue
		}
		for _, rel := range itemFiles(l) {
			out = append(out, File{Setup: s.Name, Owner: "Accessories & furniture", File: rel, Bytes: size(oldPath(l, rel)), Action: ActionMove})
		}
		for _, p := range itemLeftovers(l) {
			rel, _ := filepath.Rel(l.Folder, p)
			out = append(out, File{Setup: s.Name, Owner: "Accessories & furniture", File: filepath.ToSlash(rel), Bytes: size(p), Action: ActionLeftover})
		}
	}
	return out, nil
}

// moveAccessories stores every setup's own item files in the shared store, points the entries and editor layouts at
// them, saves, then deletes the old files and leftovers.
func moveAccessories(ctx context.Context, h setups.Home, res *Result) error {
	list, err := h.List()
	if err != nil {
		return err
	}
	for _, s := range list {
		if err := ctx.Err(); err != nil {
			return err
		}
		l, err := workspace.Dir(h.Root).Items(s.Folder)
		if err != nil {
			return err
		}
		own, junk := itemFiles(l), itemLeftovers(l)
		if len(own)+len(junk) == 0 {
			removeEmptyDirs(filepath.Join(l.Folder, oldImagesDir), true)
			continue
		}
		moved := map[string]string{} // old path → store path
		for _, rel := range own {
			to, err := l.Assets.PutFile(oldPath(l, rel))
			if err != nil {
				return err
			}
			moved[rel] = to
		}
		repointItems(l, func(rel string) string {
			if to, ok := moved[rel]; ok {
				return to
			}
			return rel
		})
		if err := l.Save(); err != nil {
			return err
		}
		for rel, to := range moved { // the store holds the same bytes now
			if err := res.discard(oldPath(l, rel), l.Assets.Path(to)); err != nil {
				return err
			}
		}
		for _, p := range junk { // no entry refers to them, but they may be someone's work: kept
			if err := res.keep(p, false); err != nil {
				return err
			}
		}
		removeEmptyDirs(filepath.Join(l.Folder, oldImagesDir), true)
	}
	return nil
}

// FillOrigins gives every library's (setups' and the catalog's) converted entries an origin when they were converted
// before origins were recorded. Writes only studio.json, so it is safe while the game runs.
func FillOrigins(h setups.Home) error {
	list, err := h.List()
	if err != nil {
		return err
	}
	for _, root := range append(setups.Folders(list), catalog.For(h.Root).Root) {
		l, err := workspace.Dir(h.Root).Items(root)
		if err != nil {
			continue
		}
		if l.FillConvertedOrigins() {
			if err := l.SaveMeta(); err != nil {
				return err
			}
		}
	}
	return nil
}

// repointItems maps every file path in a library (entries and editor layouts) through to.
func repointItems(l *accessories.Library, to func(string) string) {
	fix := func(p *string) { *p = to(*p) }
	for i := range l.Lib.Accessories {
		a := &l.Lib.Accessories[i]
		fix(&a.Texture)
		fix(&a.Icon)
		fix(&a.Mesh)
	}
	for i := range l.Lib.Furniture {
		for _, p := range l.Lib.Furniture[i].FileRefs() {
			fix(p)
		}
	}
	for i := range l.Lib.Decorations {
		for _, p := range l.Lib.Decorations[i].FileRefs() {
			fix(p)
		}
	}
	for id, raw := range l.Meta.Layouts {
		l.Meta.Layouts[id] = fsutil.MapJSONStrings(raw, to)
	}
}

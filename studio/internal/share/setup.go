package share

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"tcgstudio/internal/content"
	"tcgstudio/internal/origin"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
	"tcgstudio/internal/ziputil"
)

// A setup file (version 3) = manifest.json + setup/setup.json (installed sets) + setup/game/… (the mod's settings, the
// card back, Shop look pictures) + items/<kind>/<id>/… for every item of the setup, in its list order. Saves never go.

const setupDir = "setup/"

// ExportSetup writes setup id to dest. The caller snapshots the active setup's game settings first. progress gets
// (items done, total).
func ExportSetup(h setups.Home, id, dest, studioVersion string, progress func(done, total int)) error {
	ws := workspace.Dir(h.Root)
	dir := h.Dir(id)
	in, err := setups.LoadInfo(dir)
	if err != nil {
		return err
	}
	var list []located
	counts := map[string]int{}
	for _, k := range content.Kinds {
		items, err := k.Store.List(ws, dir)
		if err != nil {
			return err
		}
		for _, it := range items {
			if !it.Converted {
				list = append(list, located{k, dir, it})
				counts[k.ID]++
			}
		}
	}
	z, err := ziputil.Create(dest)
	if err != nil {
		return err
	}
	fail := func(err error) error { z.Abort(); return err }
	items, err := exportItems(ws, list, z, progress)
	if err != nil {
		return fail(err)
	}
	// Installed sets that stay on this PC (converted) are left out, or the receiving game would look for them.
	exported := map[string]bool{}
	for _, it := range items {
		if it.Kind == "set" {
			exported[it.ID] = true
		}
	}
	info := setups.Info{Name: in.Name, Description: in.Description, InstalledSets: []string{}}
	for _, sid := range in.InstalledSets {
		if exported[sid] {
			info.InstalledSets = append(info.InstalledSets, sid)
		}
	}
	if err := z.JSON(setupDir+setups.InfoFile, info); err != nil {
		return fail(err)
	}
	if err := exportGame(filepath.Join(dir, setups.GameStateDir), setupDir+setups.GameStateDir+"/", z); err != nil {
		return fail(err)
	}
	m := Manifest{Format: setups.Format, FormatVersion: SetupVersion, StudioVersion: studioVersion, Exported: time.Now().UTC(),
		Items: items, Name: in.Name, Description: in.Description, Sets: counts["set"], Accessories: counts["accessory"],
		Furniture: counts["furniture"], Decorations: counts["decoration"]}
	if err := z.JSON(manifestFile, m); err != nil {
		return fail(err)
	}
	return z.Close()
}

// exportGame adds a setup's game folder: the mod's settings (absolute paths blanked), card back, Shop look pictures.
// Content older Studio versions parked there (Sets, Accessories, Library) stays on this PC.
func exportGame(dir, zipDir string, z *ziputil.Writer) error {
	return filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return filepath.SkipAll
		}
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if first, _, _ := strings.Cut(rel, "/"); first == "Sets" || first == "Accessories" || first == "Library" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(rel, ".tmp") || strings.HasSuffix(rel, ".installing") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		switch strings.ToLower(path.Ext(rel)) {
		case ".cfg":
			b = ziputil.ScrubCfg(b)
		case ".json":
			b = ziputil.ScrubJSON(b)
		}
		return z.Bytes(zipDir+rel, b)
	})
}

// SetupInfo describes a setup file before importing it.
type SetupInfo struct {
	File     string    `json:"file"`
	Manifest *Manifest `json:"manifest"`
}

// ReadSetup reads a setup file's manifest (any version this Studio can import).
func ReadSetup(src string) (*SetupInfo, error) {
	zr, m, err := readManifest(src)
	if err != nil {
		return nil, err
	}
	zr.Close()
	if err := checkSetupFile(m); err != nil {
		return nil, err
	}
	return &SetupInfo{File: src, Manifest: m}, nil
}

func checkSetupFile(m *Manifest) error {
	switch {
	case m.Format != setups.Format:
		return fmt.Errorf("not a TCG Studio setup file")
	case m.FormatVersion > SetupVersion:
		return fmt.Errorf("this setup was made with a newer TCG Studio (%s) — update TCG Studio first", m.StudioVersion)
	case m.FormatVersion >= 3:
		return checkItems(m)
	}
	return nil
}

// ImportSetup adds the setup in src as a new, inactive setup with no saves and returns its id and a note for the player
// ("" = nothing to say). Items whose id this workspace already uses get new ids (installed sets follow). Files of format
// version 1 and 2 are read by setups.Import.
func ImportSetup(h setups.Home, src, studioVersion string) (string, string, error) {
	zr, m, err := readManifest(src)
	if err != nil {
		return "", "", err
	}
	defer zr.Close()
	if err := checkSetupFile(m); err != nil {
		return "", "", err
	}
	if m.FormatVersion < 3 {
		zr.Close()
		id, kept, err := h.Import(src, studioVersion)
		note := ""
		if len(kept) > 0 {
			note = fmt.Sprintf("%d file(s) of its sets differ from the ones on this PC: this PC's were kept.", len(kept))
		}
		return id, note, err
	}
	ids, err := newIDs(h, m.Items)
	if err != nil {
		return "", "", err
	}
	renamed := 0
	for p, id := range ids {
		if p.ID != id {
			renamed++
		}
	}
	id, err := h.AddImported(m.Name, m.Description, studioVersion, func(dir string) error {
		tmp, done, err := tempDir(dir)
		if err != nil {
			return err
		}
		defer done()
		// setup.json and game\ come straight from the file.
		if err := ziputil.ExtractAll(&zr.Reader, tmp, func(n string) bool { return strings.HasPrefix(n, setupDir) }); err != nil {
			return err
		}
		if err := moveInto(filepath.Join(tmp, strings.TrimSuffix(setupDir, "/")), dir); err != nil {
			return err
		}
		dirs, err := unpack(&zr.Reader, tmp, m.Items)
		if err != nil {
			return err
		}
		keep := func(it Item, _ string) *origin.Origin { return content.OriginOf(it.Origin) }
		if err := importInto(workspace.Dir(h.Root), dir, m.Items, dirs, ids, keep); err != nil {
			return err
		}
		// Installed sets follow their new ids.
		in, err := setups.LoadInfo(dir)
		if err != nil {
			return nil // no setup.json in the file: nothing installed
		}
		for i, sid := range in.InstalledSets {
			if n, ok := ids[Pick{"set", sid}]; ok {
				in.InstalledSets[i] = n
			}
		}
		return setups.SaveInfo(dir, in)
	})
	note := ""
	if renamed > 0 {
		note = fmt.Sprintf("%d of its items had an id this PC already uses and got a new one.", renamed)
	}
	return id, note, err
}

// moveInto moves the contents of src into dst (folders merged).
func moveInto(src, dst string) error {
	des, err := os.ReadDir(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, d := range des {
		if err := os.Rename(filepath.Join(src, d.Name()), filepath.Join(dst, d.Name())); err != nil {
			return err
		}
	}
	return nil
}

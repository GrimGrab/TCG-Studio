package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"tcgstudio/internal/fsutil"
)

// A setup's installed sets are the "installedSets" of its setup.json (setups.Info): the game's mod loads exactly those,
// straight from the setup's projects folder (game.StudioPathsFile names the active setup). Edited here as raw JSON, so
// the rest of setup.json stays as the setups package wrote it.

const setupInfoFile = "setup.json"

// Installed is the set ids the setup's game loads (none for a workspace without setup.json, e.g. the catalog).
func (w Workspace) Installed() map[string]bool {
	out := map[string]bool{}
	var info struct {
		InstalledSets []string `json:"installedSets"`
	}
	if b, err := os.ReadFile(filepath.Join(w.Root, setupInfoFile)); err == nil && json.Unmarshal(b, &info) == nil {
		for _, id := range info.InstalledSets {
			out[id] = true
		}
	}
	return out
}

// setInstalled adds set id to (or removes it from) the setup's installed sets, under setup.json's file lock (shared with
// setups.EditInfo). Nothing to change → no write, so a workspace without setup.json can still uninstall.
func (w Workspace) setInstalled(id string, on bool) error {
	path := filepath.Join(w.Root, setupInfoFile)
	if w.Installed()[id] == on {
		return nil
	}
	return fsutil.WithLock(path, func() error { return w.writeInstalled(path, id, on) })
}

func (w Workspace) writeInstalled(path, id string, on bool) error {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return errors.New("this workspace has no setup, so sets can't be installed (see the Setups page)")
	}
	if err != nil {
		return err
	}
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	set := w.Installed()
	if set[id] == on {
		return nil
	}
	if on {
		set[id] = true
	} else {
		delete(set, id)
	}
	ids := make([]string, 0, len(set))
	for x := range set {
		ids = append(ids, x)
	}
	sort.Strings(ids)
	if raw["installedSets"], err = json.Marshal(ids); err != nil {
		return err
	}
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

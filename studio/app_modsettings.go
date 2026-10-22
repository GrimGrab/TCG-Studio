package main

import (
	"errors"
	"os"
	"path/filepath"

	"tcgstudio/internal/game"
	"tcgstudio/internal/modconfig"
	"tcgstudio/internal/setups"
)

// ---------------------------------------------------------------- mod settings (the in-game F1 menu, edited from the studio)

// ModSettingsView is the mod's config file as sections of self-describing entries.
type ModSettingsView struct {
	Sections    []modconfig.Section           `json:"sections"`
	GameRunning bool                          `json:"gameRunning"`
	ShowWhen    map[string]modconfig.ShowWhen `json:"showWhen"`  // settings shown only for some value of another (settings-meta.json)
	Elsewhere   []string                      `json:"elsewhere"` // settings edited on another studio page (settings-meta.json)
	Setup       string                        `json:"setup"`     // name of the active setup: each setup keeps its own copy of these settings
}

// openModConfig returns the mod's config file, first bringing it up to this mod version: settings a mod update added are
// inserted, and a missing file is written from the defaults.
func (a *App) openModConfig() (string, error) {
	if !game.IsGameDir(a.gameDir()) {
		return "", errNoGame
	}
	p := modconfig.Path(a.gameDir())
	if info, err := os.Stat(p); err == nil {
		// Settings a mod update added show up (at their defaults) without starting the game first.
		if n, err := modconfig.AddMissing(p); err == nil && n > 0 {
			info, _ = os.Stat(p)
		}
		// Remember this mod version's defaults, for creating the file when it is missing (new setup, fresh install).
		if c, cerr := os.Stat(modDefaultsCache()); cerr != nil || info.ModTime().After(c.ModTime()) {
			_ = modconfig.MakeDefaults(p, modDefaultsCache())
		}
		return p, nil
	}
	if !game.GetStatus(a.gameDir()).ModInstalled {
		return "", errors.New("the mod isn't installed yet — install it on the Game & mod page")
	}
	// No file yet (the game hasn't run with the mod, or a setup without saved settings): write the defaults; the mod reads
	// it on start and adds any setting the file doesn't have.
	if err := modconfig.WriteDefaults(p, modDefaultsCache()); err != nil {
		return "", err
	}
	return p, nil
}

// modDefaultsCache is a copy of the installed mod's config with every value at its default.
func modDefaultsCache() string { return filepath.Join(cacheDir(), "mod-defaults.cfg") }

func (a *App) ModSettings() (ModSettingsView, error) {
	p, err := a.openModConfig()
	if err != nil {
		return ModSettingsView{}, err
	}
	secs, err := modconfig.Read(p)
	if err != nil {
		return ModSettingsView{}, err
	}
	meta, err := modconfig.SettingsMeta()
	if err != nil {
		return ModSettingsView{}, err
	}
	setup := ""
	if id := a.activeID(); id != "" {
		if in, err := setups.LoadInfo(a.home().Dir(id)); err == nil {
			setup = in.Name
		}
	}
	return ModSettingsView{Sections: modconfig.WithDefaultsMeta(secs), GameRunning: game.IsRunning(), ShowWhen: meta.ShowWhen, Elsewhere: meta.Elsewhere, Setup: setup}, nil
}

// ModSettingChange is one edited value from the Mod settings page.
type ModSettingChange struct {
	Section string `json:"section"`
	Key     string `json:"key"`
	Value   string `json:"value"`
}

// SaveModSettings stores the page's edited values in one write (each validated against its type/range; nothing is written
// when any is invalid) and returns the file as it now is. A running game reloads it right away.
func (a *App) SaveModSettings(changes []ModSettingChange) (ModSettingsView, error) {
	p, err := a.openModConfig()
	if err != nil {
		return ModSettingsView{}, err
	}
	if len(changes) > 0 {
		kvs := make([][3]string, len(changes))
		for i, c := range changes {
			kvs[i] = [3]string{c.Section, c.Key, c.Value}
		}
		if _, err := modconfig.SetMany(p, kvs); err != nil {
			return ModSettingsView{}, err
		}
	}
	return a.ModSettings()
}

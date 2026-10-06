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
	ShowWhen    map[string]modconfig.ShowWhen `json:"showWhen"` // settings shown only for some value of another (settings-meta.json)
	Setup       string                        `json:"setup"`    // name of the active setup: each setup keeps its own copy of these settings
}

func (a *App) modConfigPath() (string, error) {
	if !game.IsGameDir(a.settings.GameDir) {
		return "", errors.New("game folder not set (Settings → Game)")
	}
	p := modconfig.Path(a.settings.GameDir)
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
	if !game.GetStatus(a.settings.GameDir).ModInstalled {
		return "", errors.New("the mod isn't installed yet — install it on the Setup screen")
	}
	// No file yet (the game hasn't run with the mod, or a setup without saved settings): write the defaults; the mod reads
	// it on start and adds any setting the file doesn't have.
	if err := modconfig.WriteDefaults(p, modDefaultsCache()); err != nil {
		return "", err
	}
	return p, nil
}

// modDefaultsCache is a copy of the installed mod's config with every value at its default.
func modDefaultsCache() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "TCG Studio", "mod-defaults.cfg")
}

func (a *App) ModSettings() (ModSettingsView, error) {
	p, err := a.modConfigPath()
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
	return ModSettingsView{Sections: modconfig.WithDefaultsMeta(secs), GameRunning: game.IsRunning(), ShowWhen: meta.ShowWhen, Setup: setup}, nil
}

// SetModSetting changes one value (validated against the setting's type/range). A running game reloads it right away.
func (a *App) SetModSetting(section, key, value string) (modconfig.Entry, error) {
	p, err := a.modConfigPath()
	if err != nil {
		return modconfig.Entry{}, err
	}
	return modconfig.Set(p, section, key, value)
}

// RestoreModDefaults resets one section ("" = every setting) to the mod's defaults; returns how many values changed.
func (a *App) RestoreModDefaults(section string) (int, error) {
	p, err := a.modConfigPath()
	if err != nil {
		return 0, err
	}
	return modconfig.RestoreDefaults(p, section)
}

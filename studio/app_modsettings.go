package main

import (
	"errors"
	"os"

	"tcgstudio/internal/game"
	"tcgstudio/internal/modconfig"
)

// ---------------------------------------------------------------- mod settings (the in-game F1 menu, edited from the studio)

// ModSettingsView is the mod's config file as sections of self-describing entries.
type ModSettingsView struct {
	Sections    []modconfig.Section `json:"sections"`
	GameRunning bool                `json:"gameRunning"`
}

func (a *App) modConfigPath() (string, error) {
	if !game.IsGameDir(a.settings.GameDir) {
		return "", errors.New("game folder not set (Settings → Game)")
	}
	p := modconfig.Path(a.settings.GameDir)
	if _, err := os.Stat(p); err != nil {
		return "", errors.New("the mod hasn't created its settings file yet — start the game once with the mod installed")
	}
	return p, nil
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
	return ModSettingsView{Sections: secs, GameRunning: game.IsRunning()}, nil
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

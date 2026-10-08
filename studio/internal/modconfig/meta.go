package modconfig

import (
	_ "embed"
	"encoding/json"
)

// settingsMetaJSON is the settings structure shared with the mod (it embeds the same file as a resource): which settings are
// only shown while another setting has certain values. TestSettingsMeta checks it against defaults.cfg.
//
//go:embed settings-meta.json
var settingsMetaJSON []byte

// ShowWhen: the setting is shown only while Setting's value is one of Is (compared case-insensitively).
type ShowWhen struct {
	Setting string   `json:"setting"`
	Is      []string `json:"is"`
}

type Meta struct {
	ShowWhen  map[string]ShowWhen `json:"showWhen"`
	Elsewhere []string            `json:"elsewhere"` // settings edited on another studio page (hidden on Mod settings)
}

// ShopSignFolder is the plugin-dir folder holding the image of [Visuals] ShopSignImage (Studio Settings → Shop sign; a
// setup carries it with its mod config).
const ShopSignFolder = "ShopSign"

// SettingsMeta parses the embedded settings-meta.json.
func SettingsMeta() (Meta, error) {
	var m Meta
	err := json.Unmarshal(settingsMetaJSON, &m)
	return m, err
}

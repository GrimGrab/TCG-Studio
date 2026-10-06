// Package origin records where converted content came from (an EPL mod's set, accessory, figurine or furniture piece).
// Studio-only data: kept in studio.json (project meta, accessory library meta), never in the mod's files.
package origin

import "time"

type Origin struct {
	Kind     string    `json:"kind"`              // "EPL mod"
	Mod      string    `json:"mod"`               // the mod's name: the player's, else its descriptor's Name ("001 - 2019 Base")
	Author   string    `json:"author,omitempty"`  // given by the player on import
	Link     string    `json:"link,omitempty"`    // e.g. the mod's Nexus page, given by the player on import
	Package  string    `json:"package,omitempty"` // the folder or .zip it was imported from (file name)
	Path     string    `json:"path,omitempty"`    // that folder/zip on this PC at import time
	Bundle   string    `json:"bundle,omitempty"`  // the descriptor inside the mod (X.json)
	Item     string    `json:"item,omitempty"`    // the item's name in the mod (accessories, furniture)
	Imported time.Time `json:"imported"`
}

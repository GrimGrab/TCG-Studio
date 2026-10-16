// Package origin records where content came from: converted from an EPL mod, or imported from a shared item file.
// Studio-only data: kept in studio.json (project meta, accessory library meta), never in the mod's files.
package origin

import "time"

// Kinds of origin.
const (
	// KindEPL: converted from an Enhanced Prefab Loader mod.
	KindEPL = "EPL mod"
	// KindShared: imported from a TCG Studio item file; the person importing named the source.
	KindShared = "Shared file"
)

type Origin struct {
	Kind     string    `json:"kind"`              // KindEPL or KindShared
	Mod      string    `json:"mod"`               // the source's name: the mod's (player's, else its descriptor's Name), or the shared file's source
	Author   string    `json:"author,omitempty"`  // given by the player on import
	Link     string    `json:"link,omitempty"`    // e.g. the mod's Nexus page, given by the player on import
	Package  string    `json:"package,omitempty"` // the folder, .zip or item file it was imported from (file name)
	Path     string    `json:"path,omitempty"`    // that folder/zip on this PC at import time
	Bundle   string    `json:"bundle,omitempty"`  // the descriptor inside the mod (X.json)
	Item     string    `json:"item,omitempty"`    // the item's name in the mod or file
	OrigID   string    `json:"origId,omitempty"`  // shared files: the item's id in the file, when this PC gave it another
	Imported time.Time `json:"imported"`
}

// Name is the mod's name for display; Mod is empty for content converted before Studio recorded it.
func (o Origin) Name() string {
	if o.Mod != "" {
		return o.Mod
	}
	return "an unnamed " + o.Kind
}

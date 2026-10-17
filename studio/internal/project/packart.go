package project

import "tcgstudio/internal/uvmap"

// packArtKinds maps studio.json packArt entries to their editor models.
var packArtKinds = map[string]string{"pack": "Pack", "box": "Box"}

// MigratePackArt brings the pack editor's layouts up to the models' current nets (uvmap.MigrateLayout: layers move with
// their faces when a net changed); ok = something changed. Saved with the project's next save.
func MigratePackArt(p *Project) bool {
	changed := false
	if p.Meta == nil {
		return false
	}
	for _, layouts := range p.Meta.PackArt {
		for which, raw := range layouts {
			kind, ok := packArtKinds[which]
			if !ok {
				continue
			}
			if out, ok := uvmap.MigrateLayout(kind, raw); ok {
				layouts[which] = out
				changed = true
			}
		}
	}
	return changed
}

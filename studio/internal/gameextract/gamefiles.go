package gameextract

import (
	"fmt"

	"tcgstudio/internal/unityfs"
)

// The game's files are found by what is in them, not by name: the scenes and shared-asset files are numbered by build
// order, and an update that adds a scene moves everything after it (game update 2026-10-27 added a scene before the shop:
// the shop went from level1 / sharedassets1 to level2 / sharedassets2).

// assetFiles is every shared-assets file (sharedassets0, 1, 2, …) and then resources.assets: where prefabs, scriptable
// objects and fonts live.
func (x *extractor) assetFiles() []string {
	names := x.env.Names("sharedassets", ".assets")
	if len(names) == 0 { // a bundle, or a folder listing failed: the old layout
		names = []string{"sharedassets0.assets", "sharedassets1.assets", "sharedassets2.assets"}
	}
	return append(names, "resources.assets")
}

// sceneWith is the first scene (level0, level1, …) with a MonoBehaviour of class: ShelfManager = the shop, TitleScreen =
// the title screen.
func (x *extractor) sceneWith(class string) (*unityfs.File, error) {
	if x.scenes == nil {
		x.scenes = map[string]*unityfs.File{}
	}
	if f, ok := x.scenes[class]; ok {
		return f, nil
	}
	names := x.env.Names("level", "")
	if len(names) == 0 {
		names = []string{"level0", "level1", "level2"}
	}
	for _, n := range names {
		f, err := x.env.File(n)
		if err != nil || f == nil {
			continue
		}
		if len(x.env.FindScripts(f, class)) > 0 {
			x.scenes[class] = f
			return f, nil
		}
	}
	return nil, fmt.Errorf("no scene with %s found in the game files", class)
}

// shopScene is the scene with the shop (ShelfManager).
func (x *extractor) shopScene() (*unityfs.File, error) { return x.sceneWith("ShelfManager") }

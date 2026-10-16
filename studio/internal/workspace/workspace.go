// Package workspace opens the content of a setup root (a setup folder or the catalog) against the Studio workspace's
// shared storage: card art in <workspace>\library, item files in <workspace>\assets.
package workspace

import (
	"path/filepath"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/project"
)

// Dir is the Studio workspace folder (it holds setups, library, assets and catalog).
type Dir string

// Library is the shared card-art folder.
func (w Dir) Library() string { return project.LibraryDir(string(w)) }

// Kept is where the storage upgrade puts files it would otherwise delete or overwrite (anything that isn't an exact copy
// of a file that stays), one folder per run; the Storage page shows and deletes it.
func (w Dir) Kept() string { return filepath.Join(string(w), "upgrade-kept") }

// Store is the shared item-file store.
func (w Dir) Store() assets.Store { return assets.For(string(w)) }

// Projects is the set projects of a setup root, with card art in the shared library.
func (w Dir) Projects(root string) project.Workspace {
	return project.Workspace{Root: root, Library: w.Library()}
}

// Items opens the accessory library of a setup root, with files in the shared store.
func (w Dir) Items(root string) (*accessories.Library, error) {
	return accessories.Open(root, w.Store())
}

// Package content is the registry of the kinds of content Studio keeps per setup (sets, accessories, furniture,
// decorations) and the two generic ways they are stored. The catalog, item files (.tcgitems) and setup files (.tcgsetup)
// work through it, never through a switch on the kind:
//
//   - Folder (sets): <root>\projects\<id>\ (set.json + studio.json) plus every file the set names in
//     <workspace>\library\<id>\ (project.Project.Files()).
//   - Entry (any list of the accessory library): a JSON object with an "id" in <root>\accessories\accessories.json, its
//     editor layout and origin (by id) in that folder's studio.json, and its files in the shared store as "assets/…"
//     strings anywhere in the entry or layout.
//
// Adding content: a new accessory kind (a board game, say) is just another "kind" value in the accessories list and needs
// nothing here; a new file of a set needs nothing either once set.json or studio.json names it. Only a new top-level list
// of the accessory library (a peer of furniture and decorations) needs a line in Kinds; TestKindsCoverLibrary fails until
// it has one. A root is a setup folder or the catalog.
package content

import (
	"tcgstudio/internal/origin"
	"tcgstudio/internal/workspace"
	"tcgstudio/internal/ziputil"
)

// Kind is one kind of content.
type Kind struct {
	ID    string  `json:"id"`   // "set", "accessory", "furniture", "decoration"
	One   string  `json:"one"`  // "furniture piece"
	Many  string  `json:"many"` // "furniture pieces"
	Title string  `json:"title"`
	Store Storage `json:"-"`
}

// Kinds is every kind, in the order pages list them. The only place a content type is named.
var Kinds = []Kind{
	{ID: "set", One: "set", Many: "sets", Title: "Sets", Store: Folder{}},
	{ID: "accessory", One: "accessory", Many: "accessories", Title: "Accessories", Store: Entry{Field: "accessories", Sub: "kind"}},
	{ID: "furniture", One: "furniture piece", Many: "furniture pieces", Title: "Furniture", Store: Entry{Field: "furniture", Sub: "type"}},
	{ID: "decoration", One: "decoration", Many: "decorations", Title: "Decorations", Store: Entry{Field: "decorations", Sub: "kind"}},
}

// Get finds a kind by id.
func Get(id string) (Kind, bool) {
	for _, k := range Kinds {
		if k.ID == id {
			return k, true
		}
	}
	return Kind{}, false
}

// Item describes one piece of content in a root.
type Item struct {
	ID        string
	Name      string
	Sub       string // accessory kind, furniture type… ("" for sets)
	Detail    string // "983 cards · 1 pack", or the sub label
	Source    string // sets: the import source ("scryfall", "manual", …)
	Cover     string // sets: a card image (relative to the set) for thumbnails
	Icon      string // entries: the shop icon (accessory library relative)
	Origin    *origin.Origin
	Converted bool // converted from another mod: its art belongs to that mod's authors, so it is never exported
}

// In is one item to import: the folder its Export wrote (unpacked), the id it gets and the origin it gets (nil = none,
// even if the file carried one).
type In struct {
	Dir    string
	NewID  string
	Origin *origin.Origin
}

// Storage is how a kind is stored. Every method works on one root against the workspace's shared storage.
type Storage interface {
	// List returns the root's items in list order.
	List(ws workspace.Dir, root string) ([]Item, error)
	// Copy copies items (game info only; their files are shared) from one root to another, replacing ones it has.
	// Returns the ids copied.
	Copy(ws workspace.Dir, from, to string, ids []string) ([]string, error)
	// Remove deletes items from a root and returns how many it had, plus the shared-store files they referred to (for
	// Cleanup).
	Remove(ws workspace.Dir, root string, ids []string) (removed int, refs []string, err error)
	// Cleanup deletes shared files of removed items that no root in roots still uses. Returns the ids whose shared
	// folder went (sets).
	Cleanup(ws workspace.Dir, roots, ids, refs []string) []string
	// Export writes item id of root into z under prefix ("items/<kind>/<id>/"): its game info and every file it uses.
	Export(ws workspace.Dir, root, id, prefix string, z *ziputil.Writer) error
	// Import adds unpacked items to root (appended, in order), with their new ids and origins.
	Import(ws workspace.Dir, root string, in []In) error
	// Taken reports an id that is in use beyond the roots' lists (a set's shared folder without a project).
	Taken(ws workspace.Dir, id string) bool
}

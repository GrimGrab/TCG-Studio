package library

import (
	"tcgstudio/internal/catalog"
	"tcgstudio/internal/content"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

// Meta is where a listed set or item came from and its picture, so the Storage page can group and show its lists the way
// the Catalog page does (lib/catalog.ts grouped).
type Meta struct {
	Origin     string        `json:"origin,omitempty"`     // the mod it was converted from, or a shared file's source
	OriginKind string        `json:"originKind,omitempty"` // origin.KindEPL / origin.KindShared
	Author     string        `json:"author,omitempty"`
	Source     string        `json:"source,omitempty"` // sets: the import source
	Cover      string        `json:"cover,omitempty"`  // sets: a card image (relative to the set)
	Icon       string        `json:"icon,omitempty"`   // items: the shop icon (accessory library relative)
	Look       *content.Look `json:"look,omitempty"`   // items with no picture: what to show instead
}

// metaIndex reads every content kind in every setup and the catalog once: content kind id → item id → Meta.
func metaIndex(h setups.Home, list []setups.Summary) map[string]map[string]Meta {
	ws := workspace.Dir(h.Root)
	roots := append(setups.Folders(list), catalog.For(h.Root).Root)
	out := map[string]map[string]Meta{}
	for _, k := range content.Kinds {
		m := map[string]Meta{}
		for _, root := range roots {
			items, err := k.Store.List(ws, root)
			if err != nil {
				continue
			}
			for _, it := range items {
				if _, ok := m[it.ID]; ok {
					continue
				}
				e := Meta{Source: it.Source, Cover: it.Cover, Icon: it.Icon, Look: it.Look}
				if o := it.Origin; o != nil {
					e.Origin, e.OriginKind, e.Author = o.Name(), o.Kind, o.Author
				}
				m[it.ID] = e
			}
		}
		out[k.ID] = m
	}
	return out
}

// itemMeta finds an item of the library, whatever its kind (ids are unique across them in practice).
func itemMeta(idx map[string]map[string]Meta, id string) Meta {
	for _, k := range content.LibraryKinds() {
		if m, ok := idx[k.ID][id]; ok {
			return m
		}
	}
	return Meta{}
}

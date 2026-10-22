package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/setfmt"
)

// ---------------------------------------------------------------- saving a library page's draft
//
// The Accessories, Furniture and Decorations pages keep every edit in a draft (new entries, changed ones, deletes, the
// accessories' order) until the page's Save, which stores all of it here in one write of the library.

// LibraryDraftItem is a new or changed entry: the accessory, furniture piece or decoration (JSON), its editor layout ("" keeps
// it) and images made in the editor (data URLs or raw base64, "" keeps the current file).
type LibraryDraftItem struct {
	Item    json.RawMessage `json:"item"`
	Layout  string          `json:"layout"`
	Texture string          `json:"texture"` // accessories only
	Icon    string          `json:"icon"`
}

// LibraryDraft is a page's unsaved edits.
type LibraryDraft struct {
	Order   []string           `json:"order"`   // accessories: every accessory id in its new order (empty = unchanged)
	Deleted []string           `json:"deleted"` // ids of stored entries to delete
	Items   []LibraryDraftItem `json:"items"`
}

// Library lists a draft can change (the page it comes from).
const (
	listAccessories = "accessories"
	listFurniture   = "furniture"
	listDecorations = "decorations"
)

// SaveLibraryDraft stores a page's draft (list = accessories | furniture | decorations). Every entry is checked first: a bad
// one writes nothing. Deleted entries are kept in the shared catalog first, as before.
func (a *App) SaveLibraryDraft(list string, d LibraryDraft) (AccessoryView, error) {
	l, err := a.items()
	if err != nil {
		return AccessoryView{}, err
	}
	if err := applyLibraryDraft(l, list, d); err != nil {
		return AccessoryView{}, err
	}
	if len(d.Deleted) > 0 {
		if err := a.keepInCatalog("", d.Deleted, l); err != nil {
			return AccessoryView{}, err
		}
	}
	for _, id := range d.Deleted {
		deleteFromList(l, list, id)
	}
	return a.saveLibrary(l)
}

// applyLibraryDraft puts the draft's entries and order into the library in memory (deletes are left to the caller, after
// the catalog keeps a copy).
func applyLibraryDraft(l *accessories.Library, list string, d LibraryDraft) error {
	index := map[string]func(string) int{listAccessories: l.Index, listFurniture: l.FurnitureIndex, listDecorations: l.DecorationIndex}[list]
	if index == nil {
		return errors.New("unknown library list " + list)
	}
	for _, id := range d.Deleted {
		if index(id) < 0 {
			return fmt.Errorf("can't delete %q: it isn't in the library", id)
		}
	}
	seen := map[string]bool{}
	for _, it := range d.Items {
		var head struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(it.Item, &head); err != nil {
			return err
		}
		id := head.ID
		switch {
		case !setfmt.SafeID(id):
			return fmt.Errorf("bad id %q", id)
		case seen[id]:
			return fmt.Errorf("%q is in the draft twice", id)
		case index(id) < 0 && l.HasItem(id):
			return fmt.Errorf("the id %q is already used by another item", id)
		}
		seen[id] = true
		if err := putLayout(l, id, it.Layout); err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
		switch list {
		case listAccessories:
			var acc setfmt.Accessory
			if err := json.Unmarshal(it.Item, &acc); err != nil {
				return fmt.Errorf("%s: %w", id, err)
			}
			if _, ok := setfmt.KindInfo(acc.Kind); !ok {
				return fmt.Errorf("%s: unknown accessory kind %q", id, acc.Kind)
			}
			if err := errors.Join(putEditorImage(l, id, "texture", it.Texture, &acc.Texture), putEditorImage(l, id, "icon", it.Icon, &acc.Icon)); err != nil {
				return fmt.Errorf("%s: %w", id, err)
			}
			l.Put(acc)
		case listFurniture:
			var f setfmt.Furniture
			if err := json.Unmarshal(it.Item, &f); err != nil {
				return fmt.Errorf("%s: %w", id, err)
			}
			if _, ok := setfmt.FurnitureTypeInfo(f.Type); !ok {
				return fmt.Errorf("%s: unknown furniture type %q", id, f.Type)
			}
			if err := putEditorImage(l, id, "icon", it.Icon, &f.Icon); err != nil {
				return fmt.Errorf("%s: %w", id, err)
			}
			l.PutFurniture(f)
		case listDecorations:
			var dec setfmt.Decoration
			if err := json.Unmarshal(it.Item, &dec); err != nil {
				return fmt.Errorf("%s: %w", id, err)
			}
			if _, ok := setfmt.DecorationKindInfo(dec.Kind); !ok {
				return fmt.Errorf("%s: unknown decoration kind %q", id, dec.Kind)
			}
			if err := putEditorImage(l, id, "icon", it.Icon, &dec.Icon); err != nil {
				return fmt.Errorf("%s: %w", id, err)
			}
			l.PutDecoration(dec)
		}
	}
	if list == listAccessories && len(d.Order) > 0 {
		reorderAccessories(l, d.Order)
	}
	return nil
}

func deleteFromList(l *accessories.Library, list, id string) {
	switch list {
	case listAccessories:
		l.Delete(id)
	case listFurniture:
		l.DeleteFurniture(id)
	case listDecorations:
		l.DeleteDecoration(id)
	}
}

// reorderAccessories sorts the accessories by order; ids the order doesn't name keep their relative place after the others.
func reorderAccessories(l *accessories.Library, order []string) {
	pos := map[string]int{}
	for i, id := range order {
		pos[id] = i
	}
	named, rest := []setfmt.Accessory{}, []setfmt.Accessory{}
	for _, acc := range l.Lib.Accessories {
		if _, ok := pos[acc.ID]; ok {
			named = append(named, acc)
		} else {
			rest = append(rest, acc)
		}
	}
	sorted := make([]setfmt.Accessory, len(named))
	copy(sorted, named)
	for i := 1; i < len(sorted); i++ { // insertion sort: lists are short
		for j := i; j > 0 && pos[sorted[j].ID] < pos[sorted[j-1].ID]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	l.Lib.Accessories = append(sorted, rest...)
}

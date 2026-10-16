package catalog

import (
	"fmt"

	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
)

// Deleting from the catalog deletes everywhere (user decision 2026-10-06): the template, every setup's copy and the shared
// files nothing uses any more, and each setup's installed sets lose them (the game reads those from the setup).

// Deleted reports a DeleteEverywhere.
type Deleted struct {
	Setups  []string `json:"setups"`  // setup ids that had something removed
	SetIDs  []string `json:"setIds"`  // ids whose shared folder went (sets no setup has any more)
	Removed int      `json:"removed"` // copies removed (templates + setups)
}

// DeleteEverywhere removes items of a kind (by id) from the catalog and from every setup, then their shared files that
// nothing uses any more.
func (c Catalog) DeleteEverywhere(h setups.Home, kindID string, keys []string) (*Deleted, error) {
	k, err := kind(kindID)
	if err != nil {
		return nil, err
	}
	for _, id := range keys {
		if !setfmt.SafeID(id) {
			return nil, fmt.Errorf("bad id %q", id)
		}
	}
	list, err := h.List()
	if err != nil {
		return nil, err
	}
	res := &Deleted{Setups: []string{}, SetIDs: []string{}}
	var refs []string
	roots := append(setups.Folders(list), c.Root)
	for _, s := range list {
		n, r, err := k.Store.Remove(c.ws(), s.Folder, keys)
		if err != nil {
			return res, err
		}
		if n > 0 {
			res.Removed += n
			res.Setups = append(res.Setups, s.ID)
			refs = append(refs, r...)
		}
	}
	n, r, err := k.Store.Remove(c.ws(), c.Root, keys)
	if err != nil {
		return res, err
	}
	res.Removed += n
	refs = append(refs, r...)
	if gone := k.Store.Cleanup(c.ws(), roots, keys, refs); gone != nil {
		res.SetIDs = gone
	}
	return res, nil
}

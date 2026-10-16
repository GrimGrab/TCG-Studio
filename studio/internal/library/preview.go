package library

import (
	"encoding/base64"
	"errors"
	"os"

	"tcgstudio/internal/setups"
)

// ShrinkPreview is one card of a set as it is now (PNG) and as Shrink would save it (JPEG), for a before/after look.
type ShrinkPreview struct {
	Card     string `json:"card"`
	PNG      string `json:"png"`  // data: URL
	JPEG     string `json:"jpeg"` // data: URL (encoded in memory, nothing written)
	PNGSize  int64  `json:"pngSize"`
	JPEGSize int64  `json:"jpegSize"`
}

// Preview converts one PNG card of set id to JPEG in memory (the rarest card, often the most detailed).
func Preview(h setups.Home, id string) (*ShrinkPreview, error) {
	bySet, _, err := sets(h)
	if err != nil {
		return nil, err
	}
	locs := bySet[id]
	if len(locs) == 0 {
		return nil, errors.New("set not found")
	}
	png := map[string]bool{}
	for _, rel := range pngCards(locs) {
		png[rel] = true
	}
	p := locs[0].p
	best, rank, name := "", -1, ""
	for _, c := range p.Set.Cards {
		if r := p.Set.RankOf(c.Rarity); png[c.Image] && r > rank {
			best, rank, name = c.Image, r, c.Name
		}
	}
	if best == "" {
		return nil, errors.New("this set has no PNG card art")
	}
	b, err := os.ReadFile(p.ImagePath(best))
	if err != nil {
		return nil, err
	}
	j, err := toJPEG(b)
	if err != nil {
		return nil, err
	}
	return &ShrinkPreview{Card: name,
		PNG:     "data:image/png;base64," + base64.StdEncoding.EncodeToString(b),
		JPEG:    "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(j),
		PNGSize: int64(len(b)), JPEGSize: int64(len(j))}, nil
}

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

// Preview converts one PNG card of set id in setup setupID to JPEG in memory (the rarest card, often the most detailed).
func Preview(h setups.Home, setupID, id string) (*ShrinkPreview, error) {
	bySet, _, err := sets(h)
	if err != nil {
		return nil, err
	}
	for _, l := range bySet[id] {
		if l.setup != setupID {
			continue
		}
		best, rank := "", -1
		name := ""
		for _, c := range l.p.Set.Cards {
			if !isPNG(c.Image) || !fileExists(l.p.ImagePath(c.Image)) {
				continue
			}
			if r := l.p.Set.RankOf(c.Rarity); r > rank { // the rarest card: often the most detailed
				best, rank, name = c.Image, r, c.Name
			}
		}
		if best == "" {
			return nil, errors.New("this set has no PNG card art")
		}
		b, err := os.ReadFile(l.p.ImagePath(best))
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
	return nil, errors.New("set not found")
}

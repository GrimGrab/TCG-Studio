package main

import (
	"fmt"
	"strings"
	"sync"

	"tcgstudio/internal/importer"
)

// previewCache keeps set previews for the session: the card list doesn't change while Studio runs, and clicking
// through sets on the Import page should not ask the source again.
type previewCache struct {
	mu   sync.Mutex
	sets map[string]*importer.SetPreview
}

// PreviewSet sums up what importing a set with opt would make (cards, rarities, prices, pack, thumbnails) from the
// source's card list; nothing is written or downloaded. Runs next to a running import.
func (a *App) PreviewSet(source, code string, opt importer.Options) (*importer.SetPreview, error) {
	src, err := a.sources.Get(source)
	if err != nil {
		return nil, err
	}
	key := strings.Join([]string{source, code, opt.Lang, fmt.Sprint(opt.IncludeVariants), fmt.Sprint(opt.RarityMap)}, "|")
	a.previews.mu.Lock()
	got := a.previews.sets[key]
	a.previews.mu.Unlock()
	if got != nil {
		return got, nil
	}
	if got, err = importer.Preview(a.ctx, src, code, opt); err != nil {
		return nil, err
	}
	a.previews.mu.Lock()
	if a.previews.sets == nil {
		a.previews.sets = map[string]*importer.SetPreview{}
	}
	a.previews.sets[key] = got
	a.previews.mu.Unlock()
	return got, nil
}

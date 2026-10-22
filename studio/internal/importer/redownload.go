package importer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/project"
	"tcgstudio/internal/scryfall"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/tcgdex"
)

// artSource is a source that can download a card's art again (Re-download image on the Cards tab).
type artSource interface {
	// cardArt finds the art of p's cards ids at the source, fetched the way the import did.
	cardArt(ctx context.Context, p *project.Project, ids []string) (*cardArt, error)
}

// cardArt is where a set's card art comes from at its source.
type cardArt struct {
	urls   map[string]artURLs // our card id → its art
	get    func(context.Context, string) ([]byte, error)
	rotate bool    // turn landscape art upright (as the import did)
	aspect float64 // > 0: stretch to this width/height (as the import did)
}

type artURLs struct {
	front, back string
	aspect      float64 // > 0: this card's own stretch target
}

// artFromCards matches a fresh card list to p's cards by SourceID (like refreshFrom).
func artFromCards(p *project.Project, cards []cardIn, get func(context.Context, string) ([]byte, error), rotate bool, aspect float64) *cardArt {
	by := map[string]*cardIn{}
	for i := range cards {
		by[cards[i].SourceID] = &cards[i]
	}
	a := &cardArt{urls: map[string]artURLs{}, get: get, rotate: rotate, aspect: aspect}
	for cid, m := range p.Meta.Cards {
		if c := by[m.SourceID]; m.SourceID != "" && c != nil && c.Image != "" {
			a.urls[cid] = artURLs{front: c.Image, aspect: c.Aspect}
		}
	}
	return a
}

// CardImages are a card's new image paths after Re-download image (BackImage only for double-faced cards).
type CardImages struct {
	Image     string `json:"image"`
	BackImage string `json:"backImage,omitempty"`
}

// RedownloadImages downloads the art of p's cards ids again from the source the set was imported from, resized and
// fitted like the import. Each image is written under name(its current path) — a new file, so the game sees it once the
// set is saved — and returned per card id. Cards the source doesn't have any more are listed in missing.
func (r *Registry) RedownloadImages(ctx context.Context, p *project.Project, ids []string, name func(string) string) (map[string]CardImages, []string, error) {
	if p.Meta == nil {
		return nil, nil, fmt.Errorf("%s was not imported, so there is nothing to download again — use Change image…", p.ID)
	}
	var as artSource
	if s, err := r.Get(p.Meta.Source); err == nil {
		as, _ = s.(artSource)
	}
	if as == nil {
		what := "a card database"
		if p.Meta.Source == "epl" {
			what = "a mod"
		}
		return nil, nil, fmt.Errorf("%s came from %s, so its art can't be downloaded again — use Change image…", p.Set.Name, what)
	}
	src, err := as.cardArt(ctx, p, ids)
	if err != nil {
		return nil, nil, err
	}
	width, format := DefaultOptions().ImageWidth, "png"
	if lm := project.LoadLibMeta(p.LibFolder); lm != nil {
		width, format = lm.Width, lm.Format
	}
	byID := map[string]*setfmt.Card{}
	for i := range p.Set.Cards {
		byID[p.Set.Cards[i].ID] = &p.Set.Cards[i]
	}
	out := map[string]CardImages{}
	var missing []string
	for _, id := range ids {
		c := byID[id]
		u, ok := src.urls[id]
		if c == nil || !ok {
			missing = append(missing, id)
			continue
		}
		front := c.Image
		if front == "" {
			front = "images/" + id + "." + format
		}
		aspect := src.aspect
		if u.aspect > 0 {
			aspect = u.aspect
		}
		var got CardImages
		if got.Image, err = redownload(ctx, p, src, u.front, name(front), aspect, width); err != nil {
			return out, missing, fmt.Errorf("%s: %w", c.Name, err)
		}
		if u.back != "" {
			back := c.BackImage
			if back == "" {
				ext := filepath.Ext(front)
				back = strings.TrimSuffix(front, ext) + "_back" + ext
			}
			if got.BackImage, err = redownload(ctx, p, src, u.back, name(back), aspect, width); err != nil {
				return out, missing, fmt.Errorf("%s (back): %w", c.Name, err)
			}
		}
		out[id] = got
	}
	return out, missing, nil
}

// redownload saves one image at rel in the set's shared folder (format from rel's extension) and returns rel.
func redownload(ctx context.Context, p *project.Project, src *cardArt, url, rel string, aspect float64, width int) (string, error) {
	rel, file := p.WriteTarget(rel)
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return "", err
	}
	format := "png"
	if ext := strings.ToLower(filepath.Ext(rel)); ext == ".jpg" || ext == ".jpeg" {
		format = "jpg"
	}
	job := imageJob{url: url, path: file, format: format, rotate: src.rotate, aspect: aspect}
	if err := saveImage(ctx, src.get, job, width); err != nil {
		return "", err
	}
	return rel, nil
}

func (s *scryfallSource) cardArt(ctx context.Context, p *project.Project, _ []string) (*cardArt, error) {
	if p.Meta.ScryfallCode == "" {
		return nil, fmt.Errorf("%s was not imported from Scryfall", p.ID)
	}
	cards, err := s.sf.SetCards(ctx, p.Meta.ScryfallCode, nil)
	if err != nil {
		return nil, err
	}
	byID := map[string]*scryfall.Card{}
	for i := range cards {
		byID[cards[i].ID] = &cards[i]
	}
	a := &cardArt{urls: map[string]artURLs{}, get: scryfall.NewUnthrottled().Download}
	for cid, m := range p.Meta.Cards {
		c := byID[m.ScryfallID]
		if c == nil {
			continue
		}
		var u artURLs
		if img := c.FrontImage(); img != nil {
			u.front = img.PNG
			if u.front == "" {
				u.front = img.Large
			}
		}
		if back := c.BackImage(); back != nil {
			u.back = back.PNG
			if u.back == "" {
				u.back = back.Large
			}
		}
		if u.front != "" {
			a.urls[cid] = u
		}
	}
	return a, nil
}

func (s *tcgdexSource) cardArt(ctx context.Context, p *project.Project, ids []string) (*cardArt, error) {
	if p.Meta.Source != "tcgdex" || p.Meta.SetCode == "" {
		return nil, fmt.Errorf("%s was not imported from TCGdex", p.ID)
	}
	var brief []tcgdex.CardBrief
	keys := map[string][]string{} // TCGdex id → our card ids
	for _, cid := range ids {
		m, ok := p.Meta.Cards[cid]
		if !ok || m.SourceID == "" {
			continue
		}
		if len(keys[m.SourceID]) == 0 {
			brief = append(brief, tcgdex.CardBrief{ID: m.SourceID})
		}
		keys[m.SourceID] = append(keys[m.SourceID], cid)
	}
	cards, _, err := s.fetchCards(ctx, langOr(p.Meta.Lang), brief, nil)
	if err != nil {
		return nil, err
	}
	a := &cardArt{urls: map[string]artURLs{}, get: tcgdex.NewUnthrottled().Download}
	for _, c := range cards {
		if c == nil {
			continue
		}
		if img := c.ImageURL("high"); img != "" {
			for _, cid := range keys[c.ID] {
				a.urls[cid] = artURLs{front: img}
			}
		}
	}
	return a, nil
}

func (s *ygoSource) cardArt(ctx context.Context, p *project.Project, _ []string) (*cardArt, error) {
	cards, err := s.setCards(ctx, p)
	if err != nil {
		return nil, err
	}
	return artFromCards(p, cards, s.c.Download, false, CardAspect), nil
}

func (s *onePieceSource) cardArt(ctx context.Context, p *project.Project, _ []string) (*cardArt, error) {
	cards, err := s.setCards(ctx, p)
	if err != nil {
		return nil, err
	}
	return artFromCards(p, cards, s.t.c.Download, false, CardAspect), nil
}

func (s *fabSource) cardArt(ctx context.Context, p *project.Project, _ []string) (*cardArt, error) {
	cards, err := s.setCards(ctx, p)
	if err != nil {
		return nil, err
	}
	return artFromCards(p, cards, s.t.c.Download, false, CardAspect), nil
}

func (s *unionArenaSource) cardArt(ctx context.Context, p *project.Project, _ []string) (*cardArt, error) {
	cards, err := s.setCards(ctx, p)
	if err != nil {
		return nil, err
	}
	return artFromCards(p, cards, s.t.c.Download, false, CardAspect), nil
}

func (s *swuSource) cardArt(ctx context.Context, p *project.Project, _ []string) (*cardArt, error) {
	cards, err := s.setCards(ctx, p)
	if err != nil {
		return nil, err
	}
	return artFromCards(p, cards, s.c.Download, true, 0), nil
}

func (s *lorcanaSource) cardArt(ctx context.Context, p *project.Project, _ []string) (*cardArt, error) {
	cards, err := s.setCards(ctx, p)
	if err != nil {
		return nil, err
	}
	return artFromCards(p, cards, s.cdn.Download, true, 0), nil
}

func (folderSource) cardArt(_ context.Context, p *project.Project, _ []string) (*cardArt, error) {
	dir := p.Meta.SourceDir
	if dir == "" {
		return nil, fmt.Errorf("%s was not imported from an image folder", p.ID)
	}
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("the image folder %s is gone — use Change image…", dir)
	}
	sc, err := scanFolder(dir, Options{})
	if err != nil {
		return nil, err
	}
	read := func(_ context.Context, path string) ([]byte, error) { return os.ReadFile(path) }
	return artFromCards(p, sc.cards, read, true, 0), nil
}

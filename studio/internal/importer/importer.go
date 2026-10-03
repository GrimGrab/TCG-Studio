// Package importer turns a set from a card database (Scryfall, TCGdex, … — see source.go) into a studio project
// (set.json + images + studio.json).
package importer

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/draw"

	"tcgstudio/internal/project"
	"tcgstudio/internal/scryfall"
	"tcgstudio/internal/setfmt"
)

type Options struct {
	IncludeVariants bool              `json:"includeVariants"` // showcase/borderless/extended-art printings
	ImageWidth      int               `json:"imageWidth"`      // resize width (0 = keep 745)
	RarityMap       map[string]string `json:"rarityMap"`       // source rarity → game rarity
	Lang            string            `json:"lang,omitempty"`  // card language, for sources with several
}

func DefaultOptions() Options {
	return Options{IncludeVariants: true, ImageWidth: 512, RarityMap: DefaultRarityMap()}
}

func DefaultRarityMap() map[string]string {
	return map[string]string{"common": "Common", "uncommon": "Rare", "rare": "Epic", "mythic": "Legendary", "special": "Legendary", "bonus": "Legendary"}
}

// Progress is reported as the import runs (stage: "cards", "images", "done").
type Progress struct {
	Stage   string `json:"stage"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

func SetID(code string) string { return "mtg-" + strings.ToLower(code) }

var unsafe = regexp.MustCompile(`[\s:/|\\]+`)

func CardID(collector string) string {
	id := strings.ToLower(unsafe.ReplaceAllString(collector, "-"))
	if id == "" {
		id = "card"
	}
	return id
}

// Import creates a new project from a Scryfall set. Fails if the project already exists.
func Import(ctx context.Context, sf *scryfall.Client, ws project.Workspace, code string, opt Options, report func(Progress)) (*project.Project, error) {
	if opt.RarityMap == nil {
		opt.RarityMap = DefaultRarityMap()
	}
	id := SetID(code)
	if _, err := os.Stat(ws.Folder(id)); err == nil {
		return nil, fmt.Errorf("%s is already imported — open it and use Refresh prices, or delete it first", id)
	}
	sfSet, err := sf.GetSet(ctx, code)
	if err != nil {
		return nil, err
	}
	report(Progress{Stage: "cards", Message: "Fetching card list…"})
	cards, err := sf.SetCards(ctx, code, func(got, total int) {
		report(Progress{Stage: "cards", Done: got, Total: total, Message: fmt.Sprintf("Fetched %d / %d cards", got, total)})
	})
	if err != nil {
		return nil, err
	}

	folder := ws.Folder(id)
	if err := os.MkdirAll(filepath.Join(folder, "images"), 0o755); err != nil {
		return nil, err
	}
	set := setfmt.NewSet(id, sfSet.Name)
	set.RenderMode = "FullImage"
	meta := &project.Meta{Source: "scryfall", ScryfallCode: sfSet.Code, ReleasedAt: sfSet.ReleasedAt,
		ImportedAt: time.Now(), PricesUpdated: time.Now(), Cards: map[string]project.CardMeta{}}

	var jobs []imageJob
	used := map[string]int{}
	for _, c := range cards {
		if c.Digital && !sfSet.Digital {
			continue
		}
		if !opt.IncludeVariants && c.Variation {
			continue
		}
		img := c.FrontImage()
		if img == nil {
			continue
		}
		cid := CardID(c.CollectorNumber)
		if n := used[cid]; n > 0 {
			cid = fmt.Sprintf("%s-%d", cid, n+1)
		}
		used[CardID(c.CollectorNumber)]++

		rarity := opt.RarityMap[c.Rarity]
		if rarity == "" {
			rarity = "Common"
		}
		tags := VariantTags(&c)
		rel := "images/" + cid + ".png"
		name := c.Name
		if len(tags) > 0 {
			name += " (" + strings.Join(tags, ", ") + ")"
		}
		card := setfmt.Card{
			ID: cid, Name: name, Description: c.Text(), Artist: c.ArtistName(),
			Rarity: rarity, Number: c.CollectorNumber, Image: rel, Play: setfmt.DefaultPlay(),
		}
		cm := project.CardMeta{ScryfallID: c.ID, Name: c.Name, Layout: c.Layout, Colors: c.ColorList(), TypeLine: c.TypeLine, ManaCost: c.ManaCost,
			CMC: c.CMC, Power: c.Power, Toughness: c.Toughness, SrcRarity: c.Rarity, Variant: tags,
			USD: scryfall.ParsePrice(c.Prices.USD), USDFoil: scryfall.ParsePrice(c.Prices.USDFoil), EUR: scryfall.ParsePrice(c.Prices.EUR)}
		if cm.USDFoil == nil {
			cm.USDFoil = scryfall.ParsePrice(c.Prices.USDEtch)
		}
		if cm.ManaCost == "" && len(c.CardFaces) > 0 {
			cm.ManaCost = c.CardFaces[0].ManaCost // double-faced cards keep the cost on the faces
		}
		card.Price = RealPrice(cm)
		set.Cards = append(set.Cards, card)
		meta.Cards[cid] = cm

		u := img.PNG
		if u == "" {
			u = img.Large
		}
		jobs = append(jobs, imageJob{u, filepath.Join(folder, filepath.FromSlash(rel))})
	}
	if len(set.Cards) == 0 {
		return nil, fmt.Errorf("no printable cards found in set %s", code)
	}

	// Default booster: MTG-like 5 commons, 1 uncommon, 1 rare/mythic.
	pack := setfmt.NewPack("booster", sfSet.Name+" Booster")
	pack.Slots = []setfmt.Slot{
		{Count: 5, Weights: map[string]float64{"Common": 1}},
		{Count: 1, Weights: map[string]float64{"Rare": 1}},
		{Count: 1, Weights: map[string]float64{"Epic": 7, "Legendary": 1}},
	}
	pack.FoilChance = 10
	set.Packs = append(set.Packs, pack)

	// Set icon (used by the pack art generator).
	if sfSet.IconSVGURI != "" {
		if b, err := sf.Download(ctx, sfSet.IconSVGURI); err == nil {
			_ = os.WriteFile(filepath.Join(folder, "images", "set_icon.svg"), b, 0o644)
		}
	}

	// Images: the card CDN has no API rate limit, but stay modest.
	failed := downloadImages(ctx, scryfall.NewUnthrottled().Download, jobs, opt.ImageWidth, report)
	if ctx.Err() != nil {
		_ = os.RemoveAll(folder)
		return nil, ctx.Err()
	}

	p := &project.Project{ID: id, Folder: folder, Set: set, Meta: meta}
	project.FillMtg(p)
	if err := ws.Save(p); err != nil {
		return nil, err
	}
	msg := fmt.Sprintf("Imported %d cards", len(set.Cards))
	if len(failed) > 0 {
		msg += fmt.Sprintf(" (%d images failed: %s)", len(failed), strings.Join(failed, ", "))
	}
	report(Progress{Stage: "done", Done: len(jobs), Total: len(jobs), Message: msg})
	return p, nil
}

// RefreshMeta re-downloads the set's card list and updates the real prices kept in studio.json (matched by Scryfall id).
// Returns how many cards were updated. Card definitions are not touched.
func RefreshMeta(ctx context.Context, sf *scryfall.Client, p *project.Project) (int, error) {
	if p.Meta.ScryfallCode == "" {
		return 0, fmt.Errorf("%s was not imported from Scryfall", p.ID)
	}
	cards, err := sf.SetCards(ctx, p.Meta.ScryfallCode, nil)
	if err != nil {
		return 0, err
	}
	byID := map[string]*scryfall.Card{}
	for i := range cards {
		byID[cards[i].ID] = &cards[i]
	}
	n := 0
	for cid, m := range p.Meta.Cards {
		c, ok := byID[m.ScryfallID]
		if !ok {
			continue
		}
		m.USD, m.USDFoil, m.EUR = scryfall.ParsePrice(c.Prices.USD), scryfall.ParsePrice(c.Prices.USDFoil), scryfall.ParsePrice(c.Prices.EUR)
		if m.USDFoil == nil {
			m.USDFoil = scryfall.ParsePrice(c.Prices.USDEtch)
		}
		m.Name, m.Layout = c.Name, c.Layout // older imports lacked these (MTG export names)
		if c.ManaCost == "" && len(c.CardFaces) > 0 {
			m.ManaCost = c.CardFaces[0].ManaCost
		}
		p.Meta.Cards[cid] = m
		n++
	}
	p.Meta.PricesUpdated = time.Now()
	project.FillMtg(p)
	return n, nil
}

// NeedsNames reports whether a Scryfall project was imported before studio.json kept each card's Scryfall name and layout
// (needed for the exact Forge card names of double-faced, adventure and split cards).
func NeedsNames(p *project.Project) bool {
	if p == nil || p.Meta == nil || p.Meta.Source != "scryfall" || p.Meta.ScryfallCode == "" {
		return false
	}
	for _, m := range p.Meta.Cards {
		if m.ScryfallID != "" && m.Name == "" {
			return true
		}
	}
	return false
}

// FillNames downloads the set's card list and fills the missing Scryfall names/layouts (and a front-face mana cost) in
// studio.json, then the MTG export data. Unlike RefreshMeta it leaves prices alone. Returns how many cards were filled.
func FillNames(ctx context.Context, sf *scryfall.Client, p *project.Project) (int, error) {
	cards, err := sf.SetCards(ctx, p.Meta.ScryfallCode, nil)
	if err != nil {
		return 0, err
	}
	byID := map[string]*scryfall.Card{}
	for i := range cards {
		byID[cards[i].ID] = &cards[i]
	}
	n := 0
	for cid, m := range p.Meta.Cards {
		c, ok := byID[m.ScryfallID]
		if !ok || m.Name != "" {
			continue
		}
		m.Name, m.Layout = c.Name, c.Layout
		if m.ManaCost == "" {
			m.ManaCost = c.ManaCost
			if m.ManaCost == "" && len(c.CardFaces) > 0 {
				m.ManaCost = c.CardFaces[0].ManaCost
			}
		}
		p.Meta.Cards[cid] = m
		n++
	}
	project.FillMtg(p)
	return n, nil
}

// RealPrice maps Scryfall prices to a card price: base = USD (EUR×1.08 fallback, min 0.10),
// foil multiplier from the real foil price when both exist.
func RealPrice(m project.CardMeta) setfmt.CardPrice {
	var base float64
	switch {
	case m.USD != nil && *m.USD > 0:
		base = *m.USD
	case m.EUR != nil && *m.EUR > 0:
		base = *m.EUR * 1.08
	case m.USDFoil != nil && *m.USDFoil > 0:
		base = *m.USDFoil / 2.5 // foil-only printing
	default:
		base = 0.10
	}
	base = math.Max(0.10, math.Round(base*100)/100)
	p := setfmt.CardPrice{Base: base}
	if m.USD != nil && *m.USD > 0 && m.USDFoil != nil && *m.USDFoil > 0 {
		f := math.Round(math.Min(50, math.Max(1, *m.USDFoil / *m.USD))*100) / 100
		p.FoilMultiplier = &f
	}
	return p
}

type imageJob struct{ url, path string }

// downloadImages saves the jobs' images with a few workers (resized to width), reporting "images" progress.
// Returns the file names that failed; the caller checks ctx for a cancel.
func downloadImages(ctx context.Context, get func(context.Context, string) ([]byte, error), jobs []imageJob, width int, report func(Progress)) []string {
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		done   int
		failed []string
	)
	ch := make(chan imageJob)
	for w := 0; w < 6; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				err := saveImage(ctx, get, j.url, j.path, width)
				mu.Lock()
				done++
				if err != nil {
					failed = append(failed, filepath.Base(j.path))
				}
				report(Progress{Stage: "images", Done: done, Total: len(jobs), Message: fmt.Sprintf("Downloaded %d / %d images", done, len(jobs))})
				mu.Unlock()
			}
		}()
	}
	for _, j := range jobs {
		if ctx.Err() != nil {
			break
		}
		ch <- j
	}
	close(ch)
	wg.Wait()
	return failed
}

func saveImage(ctx context.Context, get func(context.Context, string) ([]byte, error), url, path string, width int) error {
	b, err := get(ctx, url)
	if err != nil {
		return err
	}
	if width > 0 {
		if img, _, err := image.Decode(bytes.NewReader(b)); err == nil && img.Bounds().Dx() > width {
			h := img.Bounds().Dy() * width / img.Bounds().Dx()
			dst := image.NewNRGBA(image.Rect(0, 0, width, h))
			draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
			var buf bytes.Buffer
			if err := png.Encode(&buf, dst); err == nil {
				b = buf.Bytes()
			}
		}
	}
	return os.WriteFile(path, b, 0o644)
}

// VariantTags names what makes a printing special (empty for a regular printing).
func VariantTags(c *scryfall.Card) []string {
	var tags []string
	if c.BorderColor == "borderless" {
		tags = append(tags, "Borderless")
	}
	for _, f := range c.FrameEffects {
		switch f {
		case "showcase":
			tags = append(tags, "Showcase")
		case "extendedart":
			tags = append(tags, "Extended Art")
		case "etched":
			tags = append(tags, "Etched")
		}
	}
	if c.FullArt && c.BorderColor != "borderless" {
		tags = append(tags, "Full Art")
	}
	return tags
}

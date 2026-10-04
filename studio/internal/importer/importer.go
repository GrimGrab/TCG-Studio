// Package importer turns a set from a card database (Scryfall, TCGdex, … — see source.go) into a studio project
// (set.json + images + studio.json).
package importer

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "github.com/gen2brain/avif" // Lorcana art is AVIF only
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"tcgstudio/internal/project"
	"tcgstudio/internal/scryfall"
	"tcgstudio/internal/setfmt"
)

type Options struct {
	IncludeVariants bool              `json:"includeVariants"` // showcase/borderless/extended-art printings
	ImageWidth      int               `json:"imageWidth"`      // resize width (0 = keep 745)
	RarityMap       map[string]string `json:"rarityMap"`       // source rarity → game rarity
	Lang            string            `json:"lang,omitempty"`  // card language, for sources with several
	// ImageFormat of the card art: "png" (default, lossless) or "jpg" (about 6x smaller; transparent corners filled).
	ImageFormat string `json:"imageFormat,omitempty"`
	// UseLibrary: the shared library already has this set made with another format or width, and the player chose to use
	// that art (nothing downloaded) instead of a separate copy for this setup.
	UseLibrary bool `json:"useLibrary,omitempty"`
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
	art, err := cardArtTarget(ws, id, folder, "scryfall", sfSet.Code, "", opt)
	if err != nil {
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
		rel := art.rel(cid)
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
		jobs = append(jobs, art.job(u, cid))
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

	p := &project.Project{ID: id, Folder: folder, LibFolder: ws.LibFolder(id), Set: set, Meta: meta}
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

type imageJob struct {
	url, path string
	format    string // "jpg" = save as JPEG, else PNG
	keep      bool   // shared library: an existing file is already this image (another setup downloaded it)
	rotate    bool    // turn landscape art upright
	aspect    float64 // > 0: stretch to this width/height (cards narrower than the game's 63×88 slot, e.g. Yu-Gi-Oh! 59×86)
}

// CardAspect is the card shape the game's FullImage slot is drawn for (63×88 mm, like Magic); the mod keeps an image's
// own aspect, so narrower art leaves the frame showing on both sides.
const CardAspect = 63.0 / 88.0

// downloadImages saves the jobs' images with a few workers (resized to width), reporting "images" progress. Images the
// shared library already has (job.keep) aren't downloaded again. Returns the file names that failed; the caller checks ctx
// for a cancel.
func downloadImages(ctx context.Context, get func(context.Context, string) ([]byte, error), jobs []imageJob, width int, report func(Progress)) []string {
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		done   int
		have   int
		failed []string
	)
	ch := make(chan imageJob)
	for w := 0; w < 6; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				var err error
				_, statErr := os.Stat(j.path)
				exists := j.keep && statErr == nil
				if !exists {
					err = saveImage(ctx, get, j, width)
				}
				mu.Lock()
				done++
				if exists {
					have++
				}
				if err != nil {
					failed = append(failed, filepath.Base(j.path))
				}
				msg := fmt.Sprintf("Downloaded %d / %d images", done, len(jobs))
				if have > 0 {
					msg += fmt.Sprintf(" (%d already on this PC)", have)
				}
				report(Progress{Stage: "images", Done: done, Total: len(jobs), Message: msg})
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

// saveImage downloads a card image and writes it as PNG, or JPEG when job.format is "jpg" (the game and the pack-art tools
// read PNG/JPG; AVIF/WebP sources are converted). Landscape cards are turned upright when job.rotate is set; width > 0
// shrinks wider images. Written via a temp file, so an interrupted import never leaves a half image (the shared library
// keeps files for later imports).
func saveImage(ctx context.Context, get func(context.Context, string) ([]byte, error), job imageJob, width int) error {
	b, err := get(ctx, job.url)
	if err != nil {
		return err
	}
	img, format, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		if bytes.HasPrefix(b, []byte{0x89, 'P', 'N', 'G'}) {
			return writeFileAtomic(job.path, b) // undecodable but already PNG: keep as is
		}
		return fmt.Errorf("%s: %w", job.url, err)
	}
	changed := format != "png"
	if job.rotate && img.Bounds().Dx() > img.Bounds().Dy() {
		img = rotateCW(img)
		changed = true
	}
	if fitted, ok := fitImage(img, job.aspect, width); ok {
		img, changed = fitted, true
	}
	switch {
	case job.format == "jpg":
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, Opaque(img), &jpeg.Options{Quality: JPEGQuality}); err != nil {
			return err
		}
		b = buf.Bytes()
	case changed:
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return err
		}
		b = buf.Bytes()
	}
	return writeFileAtomic(job.path, b)
}

// writeFileAtomic writes path via a temp file + rename.
func writeFileAtomic(path string, b []byte) error {
	tmp := path + ".part"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// fitImage stretches img to aspect (when set and more than half a percent off) and shrinks it to width (when > 0).
// Returns ok=false when nothing needs changing.
func fitImage(img image.Image, aspect float64, width int) (image.Image, bool) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if aspect > 0 && math.Abs(float64(w)/float64(h)-aspect) > 0.005 {
		w = int(math.Round(float64(h) * aspect))
	}
	if width > 0 && w > width {
		h = int(math.Round(float64(h) * float64(width) / float64(w)))
		w = width
	}
	if w == b.Dx() && h == b.Dy() {
		return img, false
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst, true
}

// FitCardImages stretches a project's card images to aspect in place (fixes sets imported before their source fitted
// the art). Returns how many images changed.
func FitCardImages(p *project.Project, aspect float64) (int, error) {
	n := 0
	for _, c := range p.Set.Cards {
		path := p.ImagePath(c.Image) // shared library art is fixed once for every setup with the set
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		img, _, err := image.Decode(bytes.NewReader(b))
		if err != nil {
			continue
		}
		fitted, ok := fitImage(img, aspect, 0)
		if !ok {
			continue
		}
		var buf bytes.Buffer
		if strings.EqualFold(filepath.Ext(path), ".jpg") {
			err = jpeg.Encode(&buf, Opaque(fitted), &jpeg.Options{Quality: JPEGQuality})
		} else {
			err = png.Encode(&buf, fitted)
		}
		if err != nil {
			return n, err
		}
		if err := writeFileAtomic(path, buf.Bytes()); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// rotateCW turns an image a quarter turn clockwise (landscape cards → portrait card slots).
func rotateCW(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dst.Set(b.Max.Y-1-y, x-b.Min.X, src.At(x, y))
		}
	}
	return dst
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

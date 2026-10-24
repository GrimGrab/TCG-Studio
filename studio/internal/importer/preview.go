package importer

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"tcgstudio/internal/scryfall"
	"tcgstudio/internal/setfmt"
)

// ---------------------------------------------------------------- set preview (Import page side panel)
// What an import would make, read from the source's card list only: no project is written and no card art is
// downloaded (thumbnails are the source's own small images, shown straight from its CDN).

// preparer is a source whose import is "find the set, fetch its card list" followed by buildProject; the preview stops
// before buildProject. Scryfall and TCGdex build their projects themselves and implement it for the preview only.
type preparer interface {
	prepare(ctx context.Context, code string, opt *Options, report func(Progress)) (setIn, error)
}

// SetPreview sums up a set before it is imported.
type SetPreview struct {
	Source     string          `json:"source"`
	Code       string          `json:"code"`
	Name       string          `json:"name"`
	ReleasedAt string          `json:"releasedAt"`
	Logo       string          `json:"logo"`
	Cards      int             `json:"cards"`
	Variants   int             `json:"variants"` // of Cards: alternate printings (Include variant printings)
	Priced     int             `json:"priced"`
	Value      float64         `json:"value"` // sum of every card's real price (USD, else EUR)
	Rarities   []PreviewRarity `json:"rarities"`
	Top        []PreviewCard   `json:"top"` // most valuable first
	All        []PreviewCard   `json:"all"` // set order
	Pack       PreviewPack     `json:"pack"`
	Notes      []string        `json:"notes"`
}

type PreviewRarity struct {
	Name  string `json:"name"` // the source's rarity
	Game  string `json:"game"` // the game rarity it becomes
	Count int    `json:"count"`
}

type PreviewCard struct {
	Name    string   `json:"name"`
	Number  string   `json:"number"`
	Rarity  string   `json:"rarity"`
	Game    string   `json:"game"`
	Variant string   `json:"variant"`
	Price   *float64 `json:"price"`
	Thumb   string   `json:"thumb"` // small image for the grid
	Image   string   `json:"image"` // full-size image (click to enlarge)
}

type PreviewPack struct {
	Cards      int           `json:"cards"`
	Slots      []setfmt.Slot `json:"slots"`
	FoilChance float64       `json:"foilChance"`
}

// Preview reads a set's card list and sums up what importing it with opt would make.
func Preview(ctx context.Context, src Source, code string, opt Options) (*SetPreview, error) {
	p, ok := src.(preparer)
	if !ok {
		return nil, fmt.Errorf("%s has no set preview", src.Info().Name)
	}
	if opt.RarityMap == nil {
		opt.RarityMap = src.DefaultOptions().RarityMap
	}
	in, err := p.prepare(ctx, code, &opt, func(Progress) {})
	if err != nil {
		return nil, err
	}
	out := &SetPreview{Source: in.Source, Code: code, Name: in.Name, ReleasedAt: in.ReleasedAt, Logo: in.Logo, Notes: in.Notes}
	count := map[string]int{}
	game := map[string]string{}
	var order []string
	set := &setfmt.Set{}
	for _, c := range in.Cards {
		g := in.Rarity(c.SrcRarity)
		if count[c.SrcRarity] == 0 {
			order = append(order, c.SrcRarity)
		}
		count[c.SrcRarity]++
		game[c.SrcRarity] = g
		set.Cards = append(set.Cards, setfmt.Card{Rarity: g})
		pc := PreviewCard{Name: c.Name, Number: c.Number, Rarity: c.SrcRarity, Game: g, Variant: strings.Join(c.Variant, ", "),
			Price: c.USD, Thumb: thumbURL(c.Image), Image: c.Image}
		if pc.Price == nil {
			pc.Price = c.EUR
		}
		if len(c.Variant) > 0 {
			out.Variants++
		}
		if pc.Price != nil {
			out.Priced++
			out.Value += *pc.Price
		}
		out.All = append(out.All, pc)
	}
	out.Cards = len(out.All)
	// Rarities in the source's own order where it has one (else the set's), lowest first.
	rank := map[string]int{}
	for i, r := range append(append([]string{}, src.Info().RarityOrder...), in.RarityOrder...) {
		if _, ok := rank[r]; !ok {
			rank[r] = i
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		ri, iok := rank[order[i]]
		rj, jok := rank[order[j]]
		if iok != jok {
			return iok
		}
		return ri < rj
	})
	for _, r := range order {
		out.Rarities = append(out.Rarities, PreviewRarity{Name: r, Game: game[r], Count: count[r]})
	}
	top := append([]PreviewCard{}, out.All...)
	sort.SliceStable(top, func(i, j int) bool { return price0(top[i].Price) > price0(top[j].Price) })
	for _, c := range top[:min(8, len(top))] {
		if c.Price != nil {
			out.Top = append(out.Top, c)
		}
	}
	slots := fitSlots(in.Slots, set)
	out.Pack = PreviewPack{Cards: setfmt.SlotTotal(slots), Slots: slots, FoilChance: in.FoilChance}
	return out, nil
}

func price0(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// thumbURL is a source's small version of a card image (the import downloads the large one).
func thumbURL(u string) string {
	switch {
	case strings.Contains(u, "tcgplayer-cdn.tcgplayer.com"):
		return strings.Replace(u, "_in_1000x1000.", "_200w.", 1)
	case strings.Contains(u, "images.ygoprodeck.com/images/cards/"):
		return strings.Replace(u, "/images/cards/", "/images/cards_small/", 1)
	case strings.Contains(u, "cards.lorcast.io"):
		return strings.Replace(u, "/digital/large/", "/digital/small/", 1)
	case strings.Contains(u, "assets.tcgdex.net"):
		return strings.Replace(u, "/high.png", "/low.png", 1)
	case strings.Contains(u, "cards.scryfall.io/"):
		return strings.Replace(strings.Replace(u, "/normal/", "/small/", 1), "/large/", "/small/", 1)
	}
	return u
}

// prepare (preview only) reads a Magic set's card list like Import does.
func (s *scryfallSource) prepare(ctx context.Context, code string, opt *Options, _ func(Progress)) (setIn, error) {
	sfSet, err := s.sf.GetSet(ctx, code)
	if err != nil {
		return setIn{}, err
	}
	cards, err := s.sf.SetCards(ctx, code, func(int, int) {})
	if err != nil {
		return setIn{}, err
	}
	in := setIn{Source: "scryfall", Code: code, Name: sfSet.Name, ReleasedAt: sfSet.ReleasedAt, Logo: sfSet.IconSVGURI,
		Rarity: rarityMapper(opt.RarityMap, func(string) string { return "Common" })}
	for _, c := range cards {
		img := scryfallKeep(&c, sfSet, *opt)
		if img == nil {
			continue
		}
		in.Cards = append(in.Cards, cardIn{SourceID: c.ID, Name: c.Name, Number: c.CollectorNumber, SrcRarity: c.Rarity,
			Variant: VariantTags(&c), Image: img.Normal, USD: scryfall.ParsePrice(c.Prices.USD), EUR: scryfall.ParsePrice(c.Prices.EUR)})
	}
	b := MtgBooster(sfSet.Code, sfSet.ReleasedAt)
	in.Slots, in.FoilChance = b.Slots, b.FoilChance
	return in, nil
}

// prepare (preview only) reads a Pokémon set's card list like Import does (one request per card, a few at a time).
func (s *tcgdexSource) prepare(ctx context.Context, code string, opt *Options, _ func(Progress)) (setIn, error) {
	lang := langOr(opt.Lang)
	tset, err := s.tc.GetSet(ctx, lang, code)
	if err != nil {
		return setIn{}, err
	}
	cards, failed, err := s.fetchCards(ctx, lang, tset.Cards, func(int, int) {})
	if err != nil {
		return setIn{}, err
	}
	in := setIn{Source: "tcgdex", Code: code, Name: tset.Name, ReleasedAt: tset.ReleaseDate,
		Rarity: func(r string) string { return PokemonRarity(opt.RarityMap, r) }}
	if tset.Logo != "" {
		in.Logo = tset.Logo + ".png"
	}
	for _, c := range cards {
		img := pokemonImage(c)
		if img == "" {
			continue
		}
		usd, _, eur := c.Prices()
		in.Cards = append(in.Cards, cardIn{SourceID: c.ID, Name: c.Name, Number: c.LocalID, SrcRarity: c.Rarity,
			Image: img, USD: usd, EUR: eur})
	}
	if len(failed) > 0 {
		in.Notes = append(in.Notes, fmt.Sprintf("%d cards could not be read: %s", len(failed), strings.Join(failed, ", ")))
	}
	b := PokemonBooster(tset.ReleaseDate)
	in.Slots, in.FoilChance = b.Slots, b.FoilChance
	return in, nil
}

package importer

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"regexp"
	"sort"
	"strings"
)

// ---------------------------------------------------------------- sealed product photos (pack / box art)
// TCGplayer's catalog (via tcgcsv) lists each set's sealed products — booster packs, displays, bundles — with a front-on
// photo on a white background (1000 px for recent sets, ~200 px for old ones). The pack art editor offers them as image
// layers so imported sets get real-looking packs and boxes without anyone hunting for art.

// SealedGame is a TCGplayer category the photo search can look in.
type SealedGame struct {
	Category int    `json:"category"`
	Name     string `json:"name"`
}

// SealedGames are the games Studio imports, in the import page's order.
var SealedGames = []SealedGame{{1, "Magic: The Gathering"}, {3, "Pokémon TCG"}, {2, "Yu-Gi-Oh!"}, {68, "One Piece Card Game"},
	{79, "Star Wars: Unlimited"}, {71, "Disney Lorcana"}, {62, "Flesh and Blood"}}

// SealedCategory is the TCGplayer category of an import source (0 = unknown, e.g. hand-made sets).
func SealedCategory(source string) int {
	switch source {
	case "scryfall":
		return 1
	case "tcgdex":
		return 3
	case "ygoprodeck":
		return 2
	case "optcg":
		return opCategory
	case "swudb":
		return 79
	case "lorcast":
		return 71
	case "fab":
		return fabCategory
	}
	return 0
}

// SealedGroup is one TCGplayer set.
type SealedGroup struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	Released string `json:"released"`
}

// SealedProduct is one sealed product with a photo. Kind: "pack", "box" or "other" (bundles, tins, decks…).
type SealedProduct struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Thumb string `json:"thumb"`
	Image string `json:"image"`
}

// Sealed looks up sealed product photos on TCGplayer.
type Sealed struct{ t *tcgcsv }

func NewSealed() *Sealed { return &Sealed{newTCGCSV()} }

// Groups returns a game's sets, newest first, and the id of the one that best matches a project's set code and name
// (0 = no good match).
func (s *Sealed) Groups(ctx context.Context, category int, code, name string) ([]SealedGroup, int, error) {
	gs, err := s.t.groups(ctx, category)
	if err != nil {
		return nil, 0, err
	}
	tcgSortGroups(gs)
	out := make([]SealedGroup, len(gs))
	for i, g := range gs {
		out[i] = SealedGroup{ID: g.ID, Name: g.Name, Code: g.Abbreviation, Released: strings.SplitN(g.PublishedOn, "T", 2)[0]}
	}
	return out, matchGroup(gs, code, name), nil
}

var sealedNorm = regexp.MustCompile(`[^a-z0-9]+`)

func normName(s string) string {
	return strings.TrimSpace(sealedNorm.ReplaceAllString(strings.ToLower(s), " "))
}

// sideGroup: TCGplayer groups next to a main set that hold its extras, never its booster packs.
var sideGroup = regexp.MustCompile(`(?i)^(art series|promo pack|commander|archenemy|alchemy|jumpstart|secret lair|.* (tokens?|promos?|prerelease cards))\b`)

// matchGroup picks the group for a project: the same set code (Scryfall and TCGplayer share Magic's codes), else the same
// name, else a group whose name ends with the project's ("SV: Scarlet & Violet 151" for TCGdex's "151"). Side groups (art
// series, promos, Commander decks…) only match on the exact code.
func matchGroup(gs []tcgGroup, code, name string) int {
	if code != "" {
		for _, g := range gs {
			if strings.EqualFold(g.Abbreviation, code) {
				return g.ID
			}
		}
	}
	n := normName(name)
	if n == "" {
		return 0
	}
	for _, g := range gs {
		if normName(g.Name) == n {
			return g.ID
		}
	}
	best, bestLen := 0, 0
	for _, g := range gs {
		gn := normName(g.Name)
		if sideGroup.MatchString(g.Name) || !(strings.HasSuffix(gn, " "+n) || strings.HasPrefix(gn, n+" ")) {
			continue
		}
		if best == 0 || len(gn) < bestLen { // the shortest name is the main set ("151" before "151 Mini Tins")
			best, bestLen = g.ID, len(gn)
		}
	}
	return best
}

var (
	packName = regexp.MustCompile(`(?i)\b(booster pack|draft booster|set booster|collector booster|play booster|jumpstart booster|booster)( pack)?$|\bpack$`)
	boxName  = regexp.MustCompile(`(?i)\b(booster box|booster display|display box|display)$`)
	tcgTags2 = regexp.MustCompile(`\s*\[[^\[\]]*\]\s*$`)
	caseName = regexp.MustCompile(`(?i)\b(case|bulk case|master case)$`)
)

// sealedKind sorts products: "pack" for single boosters, "box" for booster boxes/displays, "other" for the rest.
func sealedKind(name string) string {
	n := strings.TrimSpace(tcgTags2.ReplaceAllString(tcgParens.ReplaceAllString(name, ""), "")) // "(…)" and "[1st Edition]"
	switch {
	case caseName.MatchString(n):
		return "other"
	case strings.Contains(strings.ToLower(n), "bundle") || strings.Contains(strings.ToLower(n), "sleeved"):
		return "other"
	case boxName.MatchString(n):
		return "box"
	case packName.MatchString(n) && !strings.Contains(strings.ToLower(n), "prerelease"):
		return "pack"
	}
	return "other"
}

// Products returns a set's sealed products that have a photo: packs first, then boxes, then the rest (each in catalog order).
func (s *Sealed) Products(ctx context.Context, category, group int) ([]SealedProduct, error) {
	ps, err := s.t.products(ctx, category, group)
	if err != nil {
		return nil, err
	}
	out := []SealedProduct{}
	for _, p := range ps {
		if p.ext("Rarity") != "" || p.ext("Number") != "" || p.image() == "" {
			continue // a card, or no photo yet
		}
		out = append(out, SealedProduct{ID: p.ID, Name: p.Name, Kind: sealedKind(p.Name), Thumb: p.ImageURL, Image: p.image()})
	}
	rank := map[string]int{"pack": 0, "box": 1, "other": 2}
	sort.SliceStable(out, func(i, j int) bool { return rank[out[i].Kind] < rank[out[j].Kind] })
	return out, nil
}

// Photo downloads a product photo (the large one, else the thumbnail) and crops away its white background.
func (s *Sealed) Photo(ctx context.Context, p SealedProduct) (image.Image, error) {
	var last error
	for _, u := range []string{p.Image, p.Thumb} {
		if !strings.HasPrefix(u, "https://tcgplayer-cdn.tcgplayer.com/") {
			continue
		}
		b, err := s.t.c.Download(ctx, u)
		if err != nil {
			last = err
			continue
		}
		img, _, err := image.Decode(bytes.NewReader(b))
		if err != nil {
			last = fmt.Errorf("TCGplayer photo: %w", err)
			continue
		}
		return TrimBackground(img), nil
	}
	if last == nil {
		last = fmt.Errorf("no photo for %s", p.Name)
	}
	return nil, last
}

// TrimBackground crops an image to the box around everything that isn't (near) white or transparent.
func TrimBackground(img image.Image) image.Image {
	b := img.Bounds()
	bg := func(c color.Color) bool {
		r, g, bl, a := c.RGBA()
		return a < 0x1000 || (r > 0xF000 && g > 0xF000 && bl > 0xF000)
	}
	x0, y0, x1, y1 := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if !bg(img.At(x, y)) {
				x0, y0 = min(x0, x), min(y0, y)
				x1, y1 = max(x1, x+1), max(y1, y+1)
			}
		}
	}
	r := image.Rect(x0, y0, x1, y1)
	if r.Empty() || r.Dx() < b.Dx()/4 || r.Dy() < b.Dy()/4 {
		return img // all background, or only a speck: keep the photo as it is
	}
	if si, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok {
		return si.SubImage(r)
	}
	return img
}

var (
	boosterWord = regexp.MustCompile(`(?i)\b(\w+) booster\b`)
	oddPack     = regexp.MustCompile(`(?i)\b(sample|omega|tournament|starter|theme|promo)\b`)
	notBooster  = regexp.MustCompile(`(?i)\b(deck|starter|commander|tin|collection|kit)\b`) // displays of other products
)

// PickPackAndBox chooses the photos for a set's pack number index (0 = first pack in set.json): the index-th booster pack
// (regular ones before samples, tournament and starter packs; the last one when there are fewer), and the box of the same
// booster kind ("Play Booster Display" for "Play Booster Pack"), else the first box. Either can be nil.
func PickPackAndBox(ps []SealedProduct, index int) (pack, box *SealedProduct) {
	var packs, boxes []*SealedProduct
	for i := range ps {
		switch ps[i].Kind {
		case "pack":
			packs = append(packs, &ps[i])
		case "box":
			boxes = append(boxes, &ps[i])
		}
	}
	sort.SliceStable(packs, func(i, j int) bool { return !oddPack.MatchString(packs[i].Name) && oddPack.MatchString(packs[j].Name) })
	if len(packs) > 0 {
		pack = packs[min(max(index, 0), len(packs)-1)]
	}
	word := ""
	if pack != nil {
		if m := boosterWord.FindStringSubmatch(pack.Name); m != nil {
			word = strings.ToLower(m[1])
		}
	}
	for _, b := range boxes {
		if m := boosterWord.FindStringSubmatch(b.Name); word != "" && m != nil && strings.ToLower(m[1]) == word && !oddPack.MatchString(b.Name) {
			return pack, b
		}
	}
	for _, b := range boxes {
		if !oddPack.MatchString(b.Name) && !notBooster.MatchString(b.Name) {
			return pack, b
		}
	}
	return pack, nil // only starter/commander deck displays: not a booster box
}

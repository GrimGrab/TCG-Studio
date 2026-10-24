package importer

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

// tcgGameSource is the import source of one tcgGame.
type tcgGameSource struct {
	g                   tcgGame
	t                   *tcgcsv
	variant, drop, skip *regexp.Regexp // skip nil = none
}

func newTCGGameSources() []Source {
	t := newTCGCSV()
	re := func(v, def string) *regexp.Regexp {
		if v == "" {
			v = def
		}
		if v == "" {
			return nil
		}
		return regexp.MustCompile("(?i)" + v)
	}
	var out []Source
	for _, g := range tcgGames {
		out = append(out, &tcgGameSource{g: g, t: t, variant: re(g.VariantTags, tcgVariantTags),
			drop: re(g.DropTags, tcgCodeTag), skip: re(g.SkipTags, "")})
	}
	return out
}

// TCGGame returns the tcgGame of a source id (false for the other sources).
func tcgGameOf(source string) (tcgGame, bool) {
	for _, g := range tcgGames {
		if g.ID == source {
			return g, true
		}
	}
	return tcgGame{}, false
}

var (
	// tcgDefaultFields: extendedData names games use for the same thing, tried in order.
	tcgDefaultFields = map[string][]string{
		"color": {"Color", "Colour", "Element", "Attribute", "Aspect", "Domain", "Ink", "Class", "Faction", "Affiliation"},
		"type":  {"CardType", "Card Type", "Type"},
		"text":  {"Description", "Text", "Card Text", "Effect"},
	}
	tcgSkipRarities = []string{"token", "code card", "marker", "playmat", "oversized", "jumbo"}
	// tcgVariantTags: name tags that mark another printing of a card (the rest — "(Destroy Mode)", "(Red)" — belong to the name).
	tcgVariantTags = `\+$|★|\*|parallel|alternate|alt\.? art|foil|full art|extended|showcase|serial|box topper|promo|` +
		`pre-?release|stamp|textured|signature|gold|silver|holo|secret|reprint|1st|edition|borderless|overnumbered|` +
		`\bart\b|rare$|^(sp|spr|sr|ssr|sur|sec|ur|xr|exr|gxr|osr|tsr|ssp|tsp|sss|rrr|svr|xvr|ofr|lrp|srp|scr|cpr|csr|ir|ar|br|alt \d+)$`
	// tcgCodeTag: a tag that is a card code ("GD01-001", "hBP04-068", "PR-020EN"): only tells same-named cards apart.
	tcgCodeTag        = `^[^\s]*\d[^\s]*$`
	tcgDefaultBooster = Booster{Name: "booster, 10 cards", Slots: []setfmt.Slot{slot(6, cC), slot(3, cU), slot(1, w("Epic", 4, "Legendary", 1))},
		FoilChance: 5, Estimate: true}
)

func (s *tcgGameSource) field(p *tcgProduct, name, kind string) string {
	if name != "" {
		return p.ext(name)
	}
	for _, n := range tcgDefaultFields[kind] {
		if v := p.ext(n); v != "" {
			return v
		}
	}
	return ""
}

func (s *tcgGameSource) Info() SourceInfo {
	g := s.g
	label := g.ColorLabel
	if label == "" && len(g.Colors) > 0 {
		label = "Color"
	}
	var colors []Facet
	for _, c := range g.Colors {
		colors = append(colors, Facet{c, c})
	}
	if len(colors) > 0 && g.Color != "" { // a card-type filter (no Color field) has no multicolour
		colors = append(colors, Facet{"M", "Multicolor"})
	}
	var sorts []Facet
	if g.CostLabel != "" {
		sorts = append(sorts, Facet{"cost", g.CostLabel})
	}
	if g.PowerLabel != "" {
		sorts = append(sorts, Facet{"power", g.PowerLabel})
	}
	return SourceInfo{ID: g.ID, Name: "TCGplayer", Game: g.Game, ColorLabel: label, Colors: colors,
		RarityOrder: s.rarityOrder(), Sorts: sorts, Variants: true}
}

// rarityOrder is the game's Ladder lowest first (nil = ranked per set at import).
func (s *tcgGameSource) rarityOrder() []string {
	var out []string
	for _, tier := range s.g.Ladder {
		out = append(out, tier...)
	}
	return out
}

func (s *tcgGameSource) rarityMap() map[string]string {
	m := map[string]string{}
	for i, tier := range s.g.Ladder {
		for _, r := range tier {
			m[r] = []string{"Common", "Rare", "Epic", "Legendary"}[i]
		}
	}
	return m
}

func (s *tcgGameSource) DefaultOptions() Options {
	return Options{IncludeVariants: true, ImageWidth: 512, RarityMap: s.rarityMap()}
}

// tcgGroupKind sorts a group by its name: promos/events, decks, else a booster set (main).
func tcgGroupKind(g tcgGroup) (string, bool) {
	n := strings.ToLower(g.Name + " " + g.Abbreviation)
	for _, w := range []string{"promo", "pre-release", "prerelease", "release event", "judge", "championship", "tournament",
		"event", "store", "participation", "prize", "token", "kickstarter", "convention", "winner", "regional", "national"} {
		if strings.Contains(n, w) {
			return "Promos & events", false
		}
	}
	for _, w := range []string{"starter", "deck", "structure", "trial", "precon", "start deck", "battle box", "gift", "collection",
		"tin", "bundle", "special set", "premium"} {
		if strings.Contains(n, w) {
			return "Decks & other products", false
		}
	}
	return "Booster sets", true
}

func (s *tcgGameSource) Sets(ctx context.Context, _ string) ([]SetInfo, error) {
	gs, err := s.t.groups(ctx, s.g.Category)
	if err != nil {
		return nil, err
	}
	tcgSortGroups(gs)
	var out []SetInfo
	for _, g := range gs {
		group, main := tcgGroupKind(g)
		out = append(out, SetInfo{Code: tcgCode(g, gs), Name: g.Name, Group: group, ReleasedAt: date10(g.PublishedOn), Main: main})
	}
	return out, nil
}

func (s *tcgGameSource) ProjectID(code, _ string) string { return s.g.Prefix + "-" + slug(code) }

// tcgElement picks a play element from a colour: the game's own map, else colour words.
func (s *tcgGameSource) element(color string) string {
	if e := s.g.Elements[color]; e != "" {
		return e
	}
	c := strings.ToLower(color)
	switch {
	case strings.Contains(c, "red"), strings.Contains(c, "fire"), strings.Contains(c, "orange"):
		return "Fire"
	case strings.Contains(c, "blue"), strings.Contains(c, "water"), strings.Contains(c, "aqua"):
		return "Water"
	case strings.Contains(c, "green"), strings.Contains(c, "earth"), strings.Contains(c, "black"), strings.Contains(c, "brown"):
		return "Earth"
	}
	return "Wind"
}

// splitColors turns "Red;Blue", "Red/Blue", "Red, Blue" or "Red Blue" (when every word is a known colour) into values.
func (s *tcgGameSource) splitColors(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.FieldsFunc(v, func(r rune) bool { return r == ';' || r == '/' || r == ',' || r == '|' })
	if len(parts) == 1 && len(s.g.Colors) > 0 {
		words := strings.Fields(v)
		all := len(words) > 1
		for _, w := range words {
			known := false
			for _, c := range s.g.Colors {
				known = known || strings.EqualFold(c, w)
			}
			all = all && known
		}
		if all {
			parts = words
		}
	}
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func hasPrefixFold(v string, list []string) bool {
	v = strings.ToLower(v)
	for _, p := range list {
		if strings.HasPrefix(v, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

var tcgFirstNumber = regexp.MustCompile(`-?[0-9]+(\.[0-9]+)?`)

func tcgNum(v string) float64 {
	f, _ := strconv.ParseFloat(tcgFirstNumber.FindString(v), 64)
	return f
}

// cards reads a group's cards. skipped counts the cards left out because TCGplayer has no picture of them yet.
func (s *tcgGameSource) cards(ctx context.Context, grp *tcgGroup, variants bool) ([]cardIn, int, error) {
	products, err := s.t.products(ctx, s.g.Category, grp.ID)
	if err != nil {
		return nil, 0, err
	}
	prices, err := s.t.prices(ctx, s.g.Category, grp.ID)
	if err != nil {
		return nil, 0, err
	}
	out, noImage := s.parse(products, prices, variants)
	return out, noImage, nil
}

// parse turns a group's products into cards (sealed products, tokens and skipped printings left out).
func (s *tcgGameSource) parse(products []tcgProduct, prices map[int]map[string]float64, variants bool) (out []cardIn, noImage int) {
	g := s.g
	skipR := g.SkipRarities
	if skipR == nil {
		skipR = tcgSkipRarities
	}
	normal, foil := g.Normal, g.Foil
	if normal == nil {
		normal = []string{"Normal", "Unlimited", "1st Edition"}
	}
	if foil == nil {
		foil = []string{"Foil", "Holofoil"}
	}
	for i := range products {
		p := &products[i]
		number, rarity := p.ext("Number"), p.ext("Rarity")
		typ := s.field(p, g.Type, "type")
		// Sealed products: no number, rarity and card type (some carry Rarity "None" and a product description), or a
		// retail code. Vintage Japanese Pokémon cards have neither number nor rarity, only their card type.
		if number == "" && (rarity == "" || rarity == "None") && typ == "" || p.ext("GTIN") != "" || p.ext("SKU") != "" || p.ext("MSRP") != "" {
			continue
		}
		if hasPrefixFold(rarity, skipR) || (number != "" && hasPrefixFold(number, g.SkipNumbers)) ||
			strings.Contains(strings.ToLower(typ), "token") && hasPrefixFold("token", skipR) {
			continue
		}
		name, tags := tcgName(p.Name)
		skipTag := false
		for _, t := range tags {
			skipTag = skipTag || s.skip != nil && s.skip.MatchString(t)
		}
		if skipTag {
			continue
		}
		var variant []string
		for _, t := range tags {
			switch {
			case s.drop.MatchString(t): // "(GD01-001)": a card code, only tells same-named cards apart
			case s.variant.MatchString(t):
				variant = append(variant, strings.ReplaceAll(t, "*", "★"))
			default:
				name += " (" + t + ")" // part of the name: "(Destroy Mode)", "(Red)"
			}
		}
		if len(variant) > 0 && !variants {
			continue
		}
		if p.image() == "" {
			noImage++
			continue
		}
		color := s.field(p, g.Color, "color")
		colors := s.splitColors(color)
		tl := typ
		for _, f := range g.TypeParts {
			if v := strings.ReplaceAll(p.ext(f), ";", ", "); v != "" {
				if tl == "" {
					tl = v
				} else {
					tl += " — " + v
				}
			}
		}
		text := tcgText(s.field(p, g.Text, "text"))
		for _, f := range g.TextFields {
			if v := tcgText(p.ext(f)); v != "" {
				text += "\n" + f + ": " + strings.ReplaceAll(v, ";", ", ")
			}
		}
		ci := cardIn{SourceID: strconv.Itoa(p.ID), Name: name, Number: number, Text: strings.TrimSpace(text), SrcRarity: rarity,
			TypeLine: tl, Colors: colors, Image: p.image(), Variant: variant}
		if len(colors) > 0 {
			ci.Element = s.element(colors[0])
		}
		if g.Cost != "" {
			ci.Cost = tcgNum(p.ext(g.Cost))
		}
		if g.Power != "" {
			if v := tcgFirstNumber.FindString(p.ext(g.Power)); v != "" {
				ci.Power = v
			}
		}
		if ci.Number == "" {
			ci.Number = ci.SourceID
		}
		ci.USD, ci.USDFoil = tcgPrices(prices[p.ID], normal, foil)
		out = append(out, ci)
	}
	// Set order: by number, the regular printing before its variants.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Number != out[j].Number {
			return naturalLess(out[i].Number, out[j].Number)
		}
		if (len(out[i].Variant) == 0) != (len(out[j].Variant) == 0) {
			return len(out[i].Variant) == 0
		}
		a, _ := strconv.Atoi(out[i].SourceID)
		b, _ := strconv.Atoi(out[j].SourceID)
		return a < b
	})
	return out, noImage
}

// tcgRankRarities orders a set's own rarities lowest first and maps each to a game rarity when the game has no fixed
// list: tier by name (keywordRarity), unnamed ones by how common they are, then by count (more cards = lower).
func tcgRankRarities(cards []cardIn) ([]string, map[string]string) {
	count := map[string]int{}
	for _, c := range cards {
		count[c.SrcRarity]++
	}
	tierOf := map[string]string{}
	rank := map[string]int{"Common": 0, "Rare": 1, "Epic": 2, "Legendary": 3}
	for r, n := range count {
		t := tcgTier(r)
		if t == "" {
			if t = "Epic"; n*4 >= len(cards) { // a quarter of the set or more: the commons
				t = "Common"
			}
		}
		tierOf[r] = t
	}
	var order []string
	for r := range count {
		order = append(order, r)
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := order[i], order[j]
		if rank[tierOf[a]] != rank[tierOf[b]] {
			return rank[tierOf[a]] < rank[tierOf[b]]
		}
		if count[a] != count[b] {
			return count[a] > count[b]
		}
		return a < b
	})
	return order, tierOf
}

func (s *tcgGameSource) Import(ctx context.Context, ws project.Workspace, code string, opt Options, report func(Progress)) (*project.Project, error) {
	in, err := s.prepare(ctx, code, &opt, report)
	if err != nil {
		return nil, err
	}
	return buildProject(ctx, ws, in, opt, report)
}

// prepare finds the set and reads its card list (Import and the set preview).
func (s *tcgGameSource) prepare(ctx context.Context, code string, opt *Options, report func(Progress)) (setIn, error) {
	grp, _, err := s.t.tcgFind(ctx, s.g.Category, code, s.g.Game)
	if err != nil {
		return setIn{}, err
	}
	report(Progress{Stage: "cards", Message: "Fetching card list…"})
	cards, noImage, err := s.cards(ctx, grp, opt.IncludeVariants)
	if err != nil {
		return setIn{}, err
	}
	if len(cards) == 0 {
		if noImage > 0 {
			return setIn{}, fmt.Errorf("TCGplayer has no card pictures for %s yet", grp.Name)
		}
		return setIn{}, fmt.Errorf("%s has no cards on TCGplayer", grp.Name)
	}
	var notes []string
	if noImage > 0 {
		notes = append(notes, fmt.Sprintf("%d cards left out: TCGplayer has no picture of them yet", noImage))
		report(Progress{Stage: "cards", Message: notes[0]})
	}
	// Rarities the game's Ladder doesn't list (a new set's new rarity) get a tier from their name, and the set keeps its own
	// order (meta.rarityOrder) so the Cards tab's rarity filter shows them too.
	order, tiers := tcgRankRarities(cards)
	fallback := func(r string) string { return tiers[r] }
	if ladder := s.rarityOrder(); len(ladder) > 0 {
		known := map[string]bool{}
		for _, r := range ladder {
			known[r] = true
		}
		extra := false
		for _, r := range order {
			extra = extra || !known[r]
		}
		if extra {
			order = s.mergeOrder(order, tiers)
		} else {
			order = nil // the source's own list covers the set
		}
	}
	if opt.RarityMap == nil {
		opt.RarityMap = s.rarityMap()
	}
	b := tcgDefaultBooster
	if s.g.Booster != nil {
		b = *s.g.Booster
	}
	return setIn{ID: s.ProjectID(code, ""), Source: s.g.ID, Code: code, Name: grp.Name,
		ReleasedAt: date10(grp.PublishedOn), Cards: cards, Get: s.t.c.Download, Aspect: CardAspect, Rotate: true,
		Rarity: rarityMapper(opt.RarityMap, fallback), RarityOrder: order, Slots: b.Slots, FoilChance: b.FoilChance, Notes: notes}, nil
}

// setCards fetches the card list of the set p was imported from (prices, Re-download image).
func (s *tcgGameSource) setCards(ctx context.Context, p *project.Project) ([]cardIn, error) {
	grp, _, err := s.t.tcgFind(ctx, s.g.Category, p.Meta.SetCode, s.g.Game)
	if err != nil {
		return nil, err
	}
	cards, _, err := s.cards(ctx, grp, true)
	return cards, err
}

// RefreshMeta updates prices from TCGplayer.
func (s *tcgGameSource) RefreshMeta(ctx context.Context, p *project.Project) (int, error) {
	cards, err := s.setCards(ctx, p)
	if err != nil {
		return 0, err
	}
	return refreshFrom(p, cards), nil
}

// tcgTier is the game rarity a TCGplayer rarity name suggests ("" = can't tell): deck-only and unlabelled printings are
// commons, promos epic, art versions (full art, serials, parallels) legendary, then the usual rarity words.
func tcgTier(r string) string {
	s := strings.ToLower(strings.TrimSpace(r))
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(s, w) {
				return true
			}
		}
		return false
	}
	switch {
	case s == "" || s == "none" || has("starter", "fixed", "trial", "intro deck", "repack", "character card", "theme deck"):
		return "Common"
	case strings.HasSuffix(s, "+"), has("full art", "extended", "alternate", "alt art", "serial", "signature", "showcase",
		"overnumbered", "box topper", "parallel", "iconic"):
		return "Legendary"
	case has("promo", "holo"):
		return "Epic"
	}
	return keywordRarity(r)
}

// mergeOrder is the game's Ladder with a set's unlisted rarities (setOrder, tiers from tcgRankRarities) put after the last
// listed rarity of the same game rarity.
func (s *tcgGameSource) mergeOrder(setOrder []string, tiers map[string]string) []string {
	rank := map[string]int{"Common": 0, "Rare": 1, "Epic": 2, "Legendary": 3}
	listed := s.rarityMap()
	out := s.rarityOrder()
	for _, r := range setOrder {
		if listed[r] != "" {
			continue
		}
		at := 0
		for i, o := range out {
			t := listed[o]
			if t == "" {
				t = tiers[o]
			}
			if rank[t] <= rank[tiers[r]] {
				at = i + 1
			}
		}
		out = append(out[:at], append([]string{r}, out[at:]...)...)
	}
	return out
}

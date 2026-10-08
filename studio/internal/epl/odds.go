package epl

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"tcgstudio/internal/setfmt"
)

// cardsPerPack is fixed in the game and in EPL's pack generators.
const cardsPerPack = 7

// SetPlan is a card expansion converted to our model: one card per artwork, its tiers with how often a pack gives them,
// and its packs. Game rarities are chosen afterwards (DefaultRarities, or the player's choice in the preview).
type SetPlan struct {
	Exp      *CardExpansion
	Cards    []PlannedCard
	Tiers    []Tier
	Packs    []PackPlan
	CardBack string // the expansion's card back image ("" = none)
	// CardBackAlt is the card back most of its cards use (mods sometimes name a set card back that isn't in the bundle).
	CardBackAlt string
	Warnings    []string
}

// PlannedCard is one artwork. EPL entries sharing a Sprite (a tier and its "… Foil" tier) become one card; the foil
// entry is our foil roll of it.
type PlannedCard struct {
	Sprite  string
	Name    string
	Tier    string
	Variant string // tier tag added to the name when the same name appears in several tiers
	Element string // Fire/Earth/Water/Wind
	Order   int    // 1… in EPL card order
	// EPL rarities of its plain and foil entries (prices, see price.go); "" when it has none of that kind.
	PlainRarity, FoilRarity string
}

// Tier is one of the expansion's rarities (foil tiers folded into their base tier). With the mod's rarities kept, each tier is a
// rarity of the set (Rarity then only orders the list); otherwise Rarity is the game rarity its cards get.
type Tier struct {
	Name    string  `json:"name"`
	Cards   int     `json:"cards"`
	PerPack float64 `json:"perPack"` // expected cards of this tier per pack
	PerCard float64 `json:"perCard"` // expected copies of one specific card per pack
	Rarity  string  `json:"rarity"`  // default game rarity
}

// PackPlan is one EPL pack item (and its box) with the source odds in our terms.
type PackPlan struct {
	Item       *Item
	Box        *Item
	slots      []map[string]float64 // per pack slot: tier → probability (sums to 1)
	FoilChance float64              // percent per card
	Strategy   string
}

// PlanSet converts one card expansion of a descriptor.
func PlanSet(d *Descriptor, exp *CardExpansion) *SetPlan {
	sp := &SetPlan{Exp: exp, CardBack: exp.CardbackSprite}
	base := map[string]string{} // EPL rarity → our tier (foil tiers → their base tier when it exists)
	tierSet := map[string]bool{}
	for _, c := range exp.Cards {
		if !IsFoilTier(c.Rarity) {
			tierSet[c.Rarity] = true
		}
	}
	tierOf := func(r string) string {
		if b, ok := base[r]; ok {
			return b
		}
		b := r
		if IsFoilTier(r) {
			if t := strings.TrimSpace(r[:len(r)-len(" foil")]); tierSet[t] {
				b = t
			}
		}
		base[r] = b
		return b
	}

	// Cards: one per sprite, in EPL order.
	entries := map[string]int{} // EPL rarity → entries (for AllRandom odds and empty-pool checks)
	bySprite := map[string]int{}
	for _, c := range exp.Cards {
		entries[c.Rarity]++
		if c.Sprite == "" {
			continue
		}
		i, seen := bySprite[c.Sprite]
		if !seen {
			i = len(sp.Cards)
			bySprite[c.Sprite] = i
			sp.Cards = append(sp.Cards, PlannedCard{Sprite: c.Sprite, Name: strings.TrimSpace(c.Name), Tier: tierOf(c.Rarity),
				Element: element(c.ElementType), Order: len(sp.Cards) + 1})
		}
		if pc := &sp.Cards[i]; c.IsFoil || IsFoilTier(c.Rarity) {
			if pc.FoilRarity == "" {
				pc.FoilRarity = c.Rarity
			}
		} else if pc.PlainRarity == "" {
			pc.PlainRarity = c.Rarity
		}
		if !seen {
			continue
		}
		if IsFoilTier(sp.Cards[i].Tier) && !IsFoilTier(c.Rarity) { // a foil entry came first
			sp.Cards[i].Tier = tierOf(c.Rarity)
		}
	}
	sp.CardBackAlt = mostCommon(exp.Cards, func(c Card) string { return c.Cardback })

	// Tiers in the expansion's order (foil tiers folded), then any tier a card uses that isn't listed.
	counts := map[string]int{}
	for _, c := range sp.Cards {
		counts[c.Tier]++
	}
	var order []string
	seen := map[string]bool{}
	for _, r := range append(append([]string{}, exp.Rarities...), cardTiers(sp.Cards)...) {
		t := tierOf(r)
		if !seen[t] && counts[t] > 0 {
			seen[t] = true
			order = append(order, t)
		}
	}

	// Packs of this expansion (EPL pack items name it in CardExpansion) and their boxes.
	for i := range d.Items {
		it := &d.Items[i]
		if !it.IsCardPack || !strings.EqualFold(it.CardExpansion, exp.CardExpansion) {
			continue
		}
		pp := PackPlan{Item: it, Strategy: it.PackGenerationStrategy}
		for j := range d.Items {
			if b := &d.Items[j]; b.IsCardBox && b.SpawnsPackType != "" && strings.EqualFold(b.SpawnsPackType, it.ItemType) {
				pp.Box = b
				break
			}
		}
		var foil float64
		pp.slots, foil, pp.Strategy = packSlots(it, entries, tierOf, sp)
		pp.FoilChance = 100 * foil / cardsPerPack
		if exp.HasRandomFoils {
			pp.FoilChance = percent(exp.FoilChance)
		}
		sp.Packs = append(sp.Packs, pp)
	}
	if len(sp.Packs) == 0 {
		sp.Warnings = append(sp.Warnings, "no pack items for this set in the mod: a standard booster is used")
	}

	// Pull rates per tier (average over the set's packs; none = by tier order).
	rate := map[string]float64{}
	for _, pp := range sp.Packs {
		for _, s := range pp.slots {
			for t, p := range s {
				rate[t] += p / float64(len(sp.Packs))
			}
		}
	}
	for _, t := range order {
		tr := Tier{Name: t, Cards: counts[t], PerPack: rate[t]}
		if tr.Cards > 0 {
			tr.PerCard = tr.PerPack / float64(tr.Cards)
		}
		sp.Tiers = append(sp.Tiers, tr)
	}
	sp.setDefaultRarities()
	sp.setVariants()
	return sp
}

// packSlots returns per-slot tier probabilities, expected foil cards per pack and the strategy used.
func packSlots(it *Item, entries map[string]int, tierOf func(string) string, sp *SetPlan) ([]map[string]float64, float64, string) {
	// A weight only counts when its rarity has cards (EPL drops empty pools the same way).
	norm := func(w map[string]float64) (map[string]float64, float64) {
		var tot float64
		for r, v := range w {
			if v > 0 && entries[r] > 0 {
				tot += v
			}
		}
		out := map[string]float64{}
		var foil float64
		if tot == 0 {
			return out, 0
		}
		for r, v := range w {
			if v > 0 && entries[r] > 0 {
				out[tierOf(r)] += v / tot
				if IsFoilTier(r) {
					foil += v / tot
				}
			}
		}
		return out, foil
	}
	byCount := func() (map[string]float64, float64) {
		w := map[string]float64{}
		for r, n := range entries {
			w[r] = float64(n)
		}
		return norm(w)
	}
	repeat := func(s map[string]float64, foil float64, n int) ([]map[string]float64, float64) {
		out := make([]map[string]float64, n)
		for i := range out {
			out[i] = s
		}
		return out, foil * float64(n)
	}

	normal := it.NormalWeights
	if len(normal) == 0 && len(it.GenerationWeights) > 0 { // descriptors written before 2026-03-28
		normal = map[string]float64{}
		for _, g := range it.GenerationWeights {
			normal[g.Rarity] += g.NormalWeight
		}
	}
	switch strings.ToLower(it.PackGenerationStrategy) {
	case "guaranteed":
		if len(it.SlotWeights) > 0 {
			var slots []map[string]float64
			var foil float64
			for _, k := range it.SlotNames() {
				if len(slots) == cardsPerPack {
					break
				}
				s, f := norm(it.SlotWeights[k])
				if len(s) == 0 {
					s, f = byCount()
				}
				slots = append(slots, s)
				foil += f
			}
			if len(slots) < cardsPerPack { // EPL fills the rest at random
				s, f := byCount()
				rest, rf := repeat(s, f, cardsPerPack-len(slots))
				slots, foil = append(slots, rest...), foil+rf
			}
			return slots, foil, "Guaranteed"
		}
	case "allrandomweighted", "weightedallrandom":
		if s, f := norm(normal); len(s) > 0 {
			slots, foil := repeat(s, f, cardsPerPack)
			return slots, foil, "AllRandomWeighted"
		}
	case "godpack":
		sp.Warnings = append(sp.Warnings, fmt.Sprintf("%s always gives a god pack in EPL; converted with its god pack odds", it.Name))
		if s, f := norm(it.GodWeightsOr(normal)); len(s) > 0 {
			slots, foil := repeat(s, f, cardsPerPack)
			return slots, foil, "GodPack"
		}
	case "allrandom":
		s, f := byCount()
		slots, foil := repeat(s, f, cardsPerPack)
		return slots, foil, "AllRandom"
	}
	if s, f := norm(normal); len(s) > 0 { // unknown strategy name but weights given
		slots, foil := repeat(s, f, cardsPerPack)
		sp.Warnings = append(sp.Warnings, fmt.Sprintf("%s: pack rule %q unknown, used its weights", it.Name, it.PackGenerationStrategy))
		return slots, foil, "AllRandomWeighted"
	}
	if it.PackGenerationStrategy != "" && !strings.EqualFold(it.PackGenerationStrategy, "AllRandom") {
		sp.Warnings = append(sp.Warnings, fmt.Sprintf("%s: no usable pack odds, cards are drawn by how many each tier has", it.Name))
	}
	s, f := byCount()
	slots, foil := repeat(s, f, cardsPerPack)
	return slots, foil, "AllRandom"
}

// GodWeightsOr is the item's god pack weights, or fallback when it has none.
func (it *Item) GodWeightsOr(fallback map[string]float64) map[string]float64 {
	if len(it.GodWeights) > 0 {
		return it.GodWeights
	}
	return fallback
}

// setDefaultRarities picks a game rarity per tier from the mod's own odds: tiers are ranked by how often one specific
// card comes out of a pack (most pulled first) and cut into up to four runs — Common, Rare, Epic, Legendary — whose
// cards have the closest odds (oddsGroups), since the game gives every card of a rarity the same chance. Tiers stay
// whole; tiers no pack gives are Legendary. Without pack odds the tiers are spread over the four rarities in their
// listed order.
func (sp *SetPlan) setDefaultRarities() {
	ladder := []string{"Common", "Rare", "Epic", "Legendary"}
	var any bool
	total := 0
	for _, t := range sp.Tiers {
		any = any || t.PerCard > 0
		total += t.Cards
	}
	if !any || total == 0 {
		for i := range sp.Tiers {
			k := 0
			if n := len(sp.Tiers); n > 1 {
				k = int(math.Round(float64(i) * 3 / float64(n-1)))
			}
			sp.Tiers[i].Rarity = ladder[k]
		}
		return
	}
	idx := make([]int, len(sp.Tiers))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return sp.Tiers[idx[a]].PerCard > sp.Tiers[idx[b]].PerCard })
	// Tiers no pack gives: Legendary (only reachable otherwise).
	n := len(idx)
	for n > 0 && sp.Tiers[idx[n-1]].PerCard <= 0 {
		sp.Tiers[idx[n-1]].Rarity = "Legendary"
		n--
	}
	for g, grp := range oddsGroups(sp.Tiers, idx[:n], len(ladder)) {
		for _, i := range grp {
			sp.Tiers[i].Rarity = ladder[g]
		}
	}
}

// oddsGroups splits tiers (sorted by per-card odds, most pulled first) into at most k runs so that the cards of a run
// have odds as close as possible: the game picks a rarity and then any card of it, so every card of a game rarity comes
// out equally often. Minimises Σ cards × (log odds − log run odds)², the run's odds being what each of its cards gets in
// game (its tiers' pulls ÷ its cards). Weighting by cards lets a few odd cards (single-card award tiers) join a big tier
// instead of taking a rarity of their own. Exact (dynamic programming; tiers are few).
func oddsGroups(tiers []Tier, idx []int, k int) [][]int {
	n := len(idx)
	if n == 0 {
		return nil
	}
	if k > n {
		k = n
	}
	cost := func(a, b int) float64 { // run idx[a:b]
		var pulls, cards float64
		for _, i := range idx[a:b] {
			pulls += tiers[i].PerPack
			cards += float64(tiers[i].Cards)
		}
		if cards == 0 || pulls <= 0 {
			return 0
		}
		lg := math.Log(pulls / cards)
		c := 0.0
		for _, i := range idx[a:b] {
			d := math.Log(tiers[i].PerCard) - lg
			c += float64(tiers[i].Cards) * d * d
		}
		return c
	}
	// best[g][j]: cheapest split of the first j tiers into g runs; cut[g][j]: where its last run starts.
	inf := math.Inf(1)
	best := make([][]float64, k+1)
	cut := make([][]int, k+1)
	for g := range best {
		best[g] = make([]float64, n+1)
		cut[g] = make([]int, n+1)
		for j := range best[g] {
			best[g][j] = inf
		}
	}
	best[0][0] = 0
	for g := 1; g <= k; g++ {
		for j := g; j <= n; j++ {
			for s := g - 1; s < j; s++ {
				if c := best[g-1][s] + cost(s, j); c < best[g][j] {
					best[g][j], cut[g][j] = c, s
				}
			}
		}
	}
	// Fewer runs only when more can't help (identical odds).
	g := k
	for g > 1 && best[g-1][n] <= best[g][n]+1e-12 {
		g--
	}
	out := make([][]int, g)
	for j := n; g > 0; g-- {
		s := cut[g][j]
		out[g-1] = idx[s:j]
		j = s
	}
	return out
}

// DefaultRarities is the tier → game rarity map shown (and editable) in the preview.
func (sp *SetPlan) DefaultRarities() map[string]string {
	m := map[string]string{}
	for _, t := range sp.Tiers {
		m[t.Name] = t.Rarity
	}
	return m
}

// TierOrder lists the tiers, lowest first in the expansion's own order.
func (sp *SetPlan) TierOrder() []string {
	out := make([]string, len(sp.Tiers))
	for i, t := range sp.Tiers {
		out[i] = t.Name
	}
	return out
}

// setVariants tags cards whose name appears in several tiers with their tier, minus the words all tiers share
// ("Base Tier 2 Green" → "2 Green").
func (sp *SetPlan) setVariants() {
	tiersByName := map[string]map[string]bool{}
	for _, c := range sp.Cards {
		if tiersByName[c.Name] == nil {
			tiersByName[c.Name] = map[string]bool{}
		}
		tiersByName[c.Name][c.Tier] = true
	}
	prefix := commonWordPrefix(sp.TierOrder())
	for i := range sp.Cards {
		c := &sp.Cards[i]
		if len(tiersByName[c.Name]) > 1 {
			c.Variant = strings.TrimSpace(strings.TrimPrefix(c.Tier, prefix))
			if c.Variant == "" {
				c.Variant = c.Tier
			}
		}
	}
}

// Slots turns a pack's odds into our slots with the chosen tier → game rarity map: one slot per EPL slot, weights
// grouped by game rarity, identical slots merged (counts add up to 7).
func (pp *PackPlan) Slots(rarity map[string]string) []setfmt.Slot {
	var out []setfmt.Slot
	for _, s := range pp.slots {
		w := map[string]float64{}
		for t, p := range s {
			if g := rarity[t]; g != "" {
				w[g] += p
			}
		}
		for g, v := range w {
			w[g] = roundSig(v*100, 4) // percent; significant digits, so rare tiers (0.0004 %) keep their odds
			if w[g] == 0 {
				delete(w, g)
			}
		}
		if len(w) == 0 {
			continue
		}
		if n := len(out); n > 0 && sameWeights(out[n-1].Weights, w) {
			out[n-1].Count++
			continue
		}
		out = append(out, setfmt.Slot{Count: 1, Weights: w})
	}
	return out
}

// roundSig rounds v to n significant digits.
func roundSig(v float64, n int) float64 {
	if v == 0 {
		return 0
	}
	p := math.Pow(10, float64(n)-math.Ceil(math.Log10(math.Abs(v))))
	return math.Round(v*p) / p
}

func sameWeights(a, b map[string]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// element maps EPL's element names to the game's four play elements (others, e.g. "Destiny", → Fire).
func element(e string) string {
	for _, v := range setfmt.Elements {
		if strings.EqualFold(e, v) {
			return v
		}
	}
	return "Fire"
}

// percent reads a chance written as a fraction (0.05) or a percent (5).
func percent(v float64) float64 {
	if v > 0 && v <= 1 {
		return v * 100
	}
	return v
}

func cardTiers(cards []PlannedCard) []string {
	var out []string
	for _, c := range cards {
		out = append(out, c.Tier)
	}
	return out
}

func mostCommon(cards []Card, f func(Card) string) string {
	n := map[string]int{}
	best := ""
	for _, c := range cards {
		v := f(c)
		if v == "" {
			continue
		}
		n[v]++
		if n[v] > n[best] || n[v] == n[best] && v < best {
			best = v
		}
	}
	return best
}

func commonWordPrefix(names []string) string {
	if len(names) < 2 {
		return ""
	}
	words := strings.Fields(names[0])
	n := len(words)
	for _, s := range names[1:] {
		w := strings.Fields(s)
		k := 0
		for k < n && k < len(w) && strings.EqualFold(w[k], words[k]) {
			k++
		}
		n = k
	}
	if n == 0 {
		return ""
	}
	return strings.Join(words[:n], " ")
}

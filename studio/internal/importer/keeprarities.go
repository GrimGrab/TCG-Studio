package importer

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

// KeepSourceRarities gives a set the source's own rarities (meta.cards[].srcRarity: "Secret Rare", "SR", "mythic"…) as its
// rarity list, instead of the 4 game rarities the import mapped them to. The order is the set's (meta.rarityOrder) or the
// source's (order), and pack slot weights are shared out over the new rarities so the odds stay as they were (each game
// rarity's weight goes to the source rarities its cards had, by card count).
// Cards without a source rarity (added by hand) keep their game rarity. Used by imports ("Keep the set's own rarities")
// and the Set tab ("Use the source's rarities").
func KeepSourceRarities(p *project.Project, order []string) error {
	set := p.Set
	if set.OwnRarities() {
		return errors.New("the set already has its own rarities")
	}
	if len(p.Meta.RarityOrder) > 0 {
		order = p.Meta.RarityOrder
	}
	type group struct {
		src   string
		own   bool           // a source rarity (false: cards without one, grouped by their game rarity)
		votes map[string]int // game rarity → cards
		first int
	}
	groups := map[string]*group{}
	var keys []string
	for i, c := range set.Cards {
		src, own := p.Meta.Cards[c.ID].SrcRarity, true
		if strings.TrimSpace(src) == "" {
			src, own = c.Rarity, false
		}
		key := src
		if !own {
			key = gameKey + src // a game rarity, never merged with a source rarity of the same name
		}
		g := groups[key]
		if g == nil {
			g = &group{src: src, own: own, votes: map[string]int{}, first: i}
			groups[key] = g
			keys = append(keys, key)
		}
		g.votes[c.Rarity]++
	}
	if len(keys) == 0 {
		return errors.New("the set has no cards")
	}
	base := func(g *group) string { // the game rarity most of its cards had
		best, n := "Common", -1
		for _, r := range setfmt.GameRarities {
			if g.votes[r] > n {
				best, n = r, g.votes[r]
			}
		}
		return best
	}
	rankIn := func(src string) int {
		for i, o := range order {
			if strings.EqualFold(o, src) {
				return i
			}
		}
		return -1
	}
	baseRank := func(r string) int {
		for i, g := range setfmt.GameRarities {
			if g == r {
				return i
			}
		}
		return 0
	}
	// Known rarities in the source's order. Each other one (not in the order, or cards without a source rarity — e.g. images
	// at the top of an image folder) goes after the last known one whose cards had the same or a lower game rarity, so an
	// unsorted "Common" lands at the bottom, not the top.
	var known, other []string
	for _, k := range keys {
		if rankIn(groups[k].src) >= 0 {
			known = append(known, k)
		} else {
			other = append(other, k)
		}
	}
	sort.SliceStable(known, func(i, j int) bool { return rankIn(groups[known[i]].src) < rankIn(groups[known[j]].src) })
	sort.SliceStable(other, func(i, j int) bool {
		a, b := groups[other[i]], groups[other[j]]
		if ba, bb := baseRank(base(a)), baseRank(base(b)); ba != bb {
			return ba < bb
		}
		return a.first < b.first
	})
	keys = known
	for _, k := range other {
		at := 0
		for i, q := range keys {
			if baseRank(base(groups[q])) <= baseRank(base(groups[k])) {
				at = i + 1
			}
		}
		keys = append(keys[:at], append([]string{k}, keys[at:]...)...)
	}

	idOf := map[string]string{}
	groupOf := map[string]string{} // new id → the game rarity its cards had (shares out that rarity's pack weight)
	used := map[string]bool{}
	var list []setfmt.Rarity
	var srcOrder []string
	for _, key := range keys {
		g := groups[key]
		id := setfmt.RarityID(g.src)
		for n := 2; used[id]; n++ {
			id = fmt.Sprintf("%s-%d", setfmt.RarityID(g.src), n)
		}
		used[id] = true
		idOf[key] = id
		groupOf[id] = base(g)
		list = append(list, setfmt.Rarity{ID: id, Name: displayRarity(g.src)})
		if g.own {
			srcOrder = append(srcOrder, g.src)
		}
	}

	for i := range set.Cards {
		c := &set.Cards[i]
		key := p.Meta.Cards[c.ID].SrcRarity
		if strings.TrimSpace(key) == "" {
			key = gameKey + c.Rarity
		}
		c.Rarity = idOf[key]
	}
	set.Rarities = list
	for i := range set.Packs {
		pk := &set.Packs[i]
		for j := range pk.Slots {
			pk.Slots[j].Weights = set.SplitWeights(pk.Slots[j].Weights, func(id string) string { return groupOf[id] })
		}
		pk.Slots = fitSlots(pk.Slots, set)
	}
	p.Meta.RarityOrder = srcOrder
	return nil
}

// gameKey prefixes group keys of cards without a source rarity.
const gameKey = "<game rarity> "

// displayRarity capitalises all-lowercase source rarities ("mythic" → "Mythic", "super_rare" → "Super Rare").
func displayRarity(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "_", " "))
	if s == "" {
		return "Common"
	}
	if strings.ToLower(s) != s {
		return s
	}
	words := strings.Fields(s)
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

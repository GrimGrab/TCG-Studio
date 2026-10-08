package setfmt

import (
	"math"
	"strings"
	"unicode"
)

// Rarity mirrors RarityDef: one of a set's own rarities (EPL-style: a real rarity of its own in game, with its own pack
// weights). Where the game only knows its 4 rarities (the rarity icon, fame), the mod uses the one for the rarity's place in
// the list (TierOf).
type Rarity struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color,omitempty"` // "#RRGGBB" (Studio)
}

// GameRarities are the vanilla rarities a card can use when its set has no list of its own (SuperLegend has no icon in game).
var GameRarities = []string{"Common", "Rare", "Epic", "Legendary"}

// VanillaRarity returns the vanilla rarity named id (any case), or "".
func VanillaRarity(id string) string {
	for _, r := range Rarities {
		if strings.EqualFold(r, id) {
			return r
		}
	}
	return ""
}

// RarityID turns a rarity name into an id ("Secret Rare Alt-Art" → "secret-rare-alt-art").
func RarityID(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	if id := strings.TrimSuffix(b.String(), "-"); id != "" {
		return id
	}
	return "rarity"
}

// OwnRarities reports whether the set has its own rarity list.
func (s *Set) OwnRarities() bool { return len(s.Rarities) > 0 }

// RarityList is the set's rarities, lowest first: its own list, else the vanilla ones (SuperLegend only when a card uses it).
func (s *Set) RarityList() []Rarity {
	if s.OwnRarities() {
		return s.Rarities
	}
	out := make([]Rarity, 0, 5)
	for _, r := range GameRarities {
		out = append(out, Rarity{ID: r, Name: r})
	}
	for _, c := range s.Cards {
		if c.Rarity == "SuperLegend" {
			out = append(out, Rarity{ID: "SuperLegend", Name: "SuperLegend"})
			break
		}
	}
	return out
}

// RankOf is the rarity's position in the set's list, lowest first (-1 = unknown).
func (s *Set) RankOf(id string) int {
	for i, r := range s.RarityList() {
		if r.ID == id {
			return i
		}
	}
	return -1
}

// TierOf is the vanilla rarity the mod uses for a rarity where the game only knows its 4 (rarity icon, fame,
// odds of packs without slots): by its place in the set's list — the n-th of up to four is the n-th vanilla rarity, longer
// lists are spread evenly over Common…Legendary (mirrors SetLoader). Sets without a list: the rarity itself.
func (s *Set) TierOf(id string) string {
	if !s.OwnRarities() {
		if v := VanillaRarity(id); v != "" {
			return v
		}
		return "Common"
	}
	i, n := s.RankOf(id), len(s.Rarities)
	switch {
	case i < 0:
		return "Common"
	case n <= 4:
		return GameRarities[i]
	}
	return GameRarities[int(math.Round(float64(i)*3/float64(n-1)))]
}

// SplitWeights turns slot weights keyed by vanilla rarity (a source's booster, presets) into weights over the set's own
// rarities: each vanilla rarity's weight is shared by the rarities group puts in it, in proportion to their card counts.
// Keys that already are the set's own ids are kept. Sets without a list: unchanged.
func (s *Set) SplitWeights(w map[string]float64, group func(id string) string) map[string]float64 {
	if !s.OwnRarities() {
		return w
	}
	counts := map[string]int{}
	for _, c := range s.Cards {
		counts[c.Rarity]++
	}
	out := map[string]float64{}
	for key, weight := range w {
		if s.RankOf(key) >= 0 {
			out[key] += weight
			continue
		}
		var members []string
		total := 0
		for _, r := range s.Rarities {
			if group(r.ID) == key && counts[r.ID] > 0 {
				members = append(members, r.ID)
				total += counts[r.ID]
			}
		}
		for _, id := range members { // none: no card of that vanilla rarity; the mod rolls another rarity for the slot
			out[id] += weight * float64(counts[id]) / float64(total)
		}
	}
	return out
}

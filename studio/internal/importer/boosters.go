package importer

import "tcgstudio/internal/setfmt"

// Real booster layouts per game and era, in game rarities: every import maps its source rarities onto Common < Rare <
// Epic < Legendary (see each source's Default…RarityMap), so "uncommon" slots are Rare, "rare" Epic, the top tier
// Legendary. Card counts are the playable cards: ads, tokens-only cards, code cards and Pokémon's basic Energy are left
// out. The game rolls foil per card, so the pack-wide FoilChance stands in for foil/reverse-holo slots. The Packs tab
// lists the same table (App.PackPresets). Sources and estimates: docs/import-sources.md "Real boosters".
//
// Slot order is reveal order (PackRoller fills cards slot by slot; the opening shows them first to last), so every
// booster goes from least to most exciting like a real pack opened front to back: commons (and land), uncommons, the
// any-rarity / foil / reverse-holo slots, and the guaranteed rare-or-better slot last.

// Booster is one real booster layout.
type Booster struct {
	Game       string        `json:"game"`
	Name       string        `json:"name"`
	Slots      []setfmt.Slot `json:"slots"`
	FoilChance float64       `json:"foilChance"` // percent per card
	Estimate   bool          `json:"estimate"`   // the publisher doesn't publish the slot odds
}

func slot(n int, w map[string]float64) setfmt.Slot { return setfmt.Slot{Count: n, Weights: w} }

func w(kv ...interface{}) map[string]float64 {
	m := map[string]float64{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i].(string)] = float64(kv[i+1].(int))
	}
	return m
}

var (
	cC  = w("Common", 1)
	cU  = w("Rare", 1)
	cR  = w("Epic", 1)
	cRM = w("Epic", 7, "Legendary", 1) // MTG rare/mythic: mythic 1 in 8
	// "any rarity" slots (wildcards, foil and reverse-holo slots): mostly commons and uncommons
	cAny = w("Common", 8, "Rare", 8, "Epic", 3, "Legendary", 1)
)

// ---------------------------------------------------------------- Magic: The Gathering (Scryfall)

var (
	MtgPlay    = Booster{"Magic", "MTG Play Booster (2024 on), 14 cards", []setfmt.Slot{slot(7, cC), slot(1, cC), slot(3, cU), slot(2, cAny), slot(1, cRM)}, 7, false}
	MtgDraft   = Booster{"Magic", "MTG Draft Booster (2008–2023), 15 cards", []setfmt.Slot{slot(10, cC), slot(1, cC), slot(3, cU), slot(1, cRM)}, 2.2, false}
	MtgClassic = Booster{"Magic", "MTG Classic booster (1993–2008), 15 cards", []setfmt.Slot{slot(11, cC), slot(3, cU), slot(1, cR)}, 0, false}
	MtgTwelve  = Booster{"Magic", "MTG Alliances / Chronicles, 12 cards", []setfmt.Slot{slot(8, cC), slot(3, cU), slot(1, cR)}, 0, false}
	MtgEight   = Booster{"Magic", "MTG Early expansion (1993–1995), 8 cards", []setfmt.Slot{slot(6, cC), slot(2, cU)}, 0, false}
)

// MtgBooster picks a Scryfall set's real booster by its code and release date (YYYY-MM-DD).
func MtgBooster(code, releasedAt string) Booster {
	switch code {
	case "arn", "atq", "drk", "fem", "hml":
		return MtgEight
	case "all", "chr":
		return MtgTwelve
	}
	switch {
	case releasedAt != "" && releasedAt < "2008-10-03": // Shards of Alara: mythics + land slot
		b := MtgClassic
		if releasedAt >= "1999-02-15" { // Urza's Legacy: first foils, about 1 per 3 packs
			b.FoilChance = 2.2
		}
		return b
	case releasedAt >= "2024-02-09": // Murders at Karlov Manor: Play Boosters
		return MtgPlay
	default:
		return MtgDraft
	}
}

// ---------------------------------------------------------------- Pokémon (TCGdex)
// Bulbapedia "Booster pack (TCG)": 11 cards Base Set–Neo Destiny, 9 in e-Card/EX, 10 from Diamond & Pearl (+ a basic
// Energy from Sun & Moon, left out), Scarlet & Violet: 4 C, 3 U, 2 reverse holos, 1 rare or better.

var (
	PkmSV      = Booster{"Pokémon", "Pokémon Scarlet & Violet booster (2023 on), 10 cards", []setfmt.Slot{slot(4, cC), slot(3, cU), slot(2, w("Common", 6, "Rare", 3, "Epic", 1)), slot(1, w("Epic", 3, "Legendary", 1))}, 20, false}
	PkmModern  = Booster{"Pokémon", "Pokémon booster (2007–2023), 10 cards", []setfmt.Slot{slot(5, cC), slot(3, cU), slot(1, w("Common", 6, "Rare", 3, "Epic", 1)), slot(1, w("Epic", 4, "Legendary", 1))}, 10, false}
	PkmEX      = Booster{"Pokémon", "Pokémon e-Card / EX booster (2002–2007), 9 cards", []setfmt.Slot{slot(5, cC), slot(2, cU), slot(1, w("Common", 6, "Rare", 3, "Epic", 1)), slot(1, w("Epic", 9, "Legendary", 1))}, 11, false}
	PkmClassic = Booster{"Pokémon", "Pokémon Base Set–Neo booster (1999–2002), 11 cards", []setfmt.Slot{slot(7, cC), slot(3, cU), slot(1, cR)}, 3, false}
)

// PokemonBooster picks a TCGdex set's booster by release date.
func PokemonBooster(releasedAt string) Booster {
	switch {
	case releasedAt == "":
		return PkmSV
	case releasedAt < "2002-09-15": // e-Card Expedition
		return PkmClassic
	case releasedAt < "2007-05-23": // Diamond & Pearl
		return PkmEX
	case releasedAt < "2023-03-31": // Scarlet & Violet
		return PkmModern
	default:
		return PkmSV
	}
}

// ---------------------------------------------------------------- Yu-Gi-Oh! (YGOPRODeck)
// 9-card core boosters. Legend of Blue Eyes–Light of Destruction: 8 C + 1 rare or better. From Breakers of Shadow
// (2016-01-15): 7 C, 1 Rare, 1 foil (Super, Ultra about 1 in 6, Secret about 1 in 12). Game rarities: rares Rare,
// supers Epic, ultra and up Legendary.

var (
	YgoModern  = Booster{"Yu-Gi-Oh!", "Yu-Gi-Oh! core booster (2016 on), 9 cards", []setfmt.Slot{slot(7, cC), slot(1, cU), slot(1, w("Epic", 9, "Legendary", 3))}, 11, false}
	YgoClassic = Booster{"Yu-Gi-Oh!", "Yu-Gi-Oh! booster (2002–2015), 9 cards", []setfmt.Slot{slot(8, cC), slot(1, w("Rare", 18, "Epic", 4, "Legendary", 2))}, 0, true}
)

// YgoBooster picks a set's core booster by its TCG release date.
func YgoBooster(releasedAt string) Booster {
	if releasedAt != "" && releasedAt < "2016-01-15" {
		return YgoClassic
	}
	return YgoModern
}

// ---------------------------------------------------------------- the other card games

var (
	// One Piece: 12 cards; Bandai publishes no slot odds. Guides: mostly commons, ~3 uncommons, at least one rare,
	// a Super Rare about every 3 packs. Game rarities: UC Rare, R/Leader Epic, SR and up Legendary.
	OnePiece = Booster{"One Piece", "One Piece booster, 12 cards", []setfmt.Slot{slot(7, cC), slot(3, cU), slot(1, cR), slot(1, w("Epic", 2, "Legendary", 1))}, 3, true}
	// Star Wars: Unlimited (starwarsunlimited.com): 16 cards = leader, base, 9 C, 3 U, 1 rare/legendary, 1 foil of any
	// rarity. The game can't guarantee a leader/base slot (no card-type slots): they're counted as commons.
	SwuBooster = Booster{"Star Wars: Unlimited", "Star Wars: Unlimited booster, 16 cards", []setfmt.Slot{slot(11, cC), slot(3, cU), slot(1, cAny), slot(1, w("Epic", 6, "Legendary", 1))}, 6, false}
	// Lorcana: 12 cards = 6 C, 3 U, 2 rare or better (Rare/Super Rare/Legendary), 1 foil of any rarity.
	Lorcana = Booster{"Lorcana", "Lorcana booster, 12 cards", []setfmt.Slot{slot(6, cC), slot(3, cU), slot(1, cAny), slot(2, w("Epic", 11, "Legendary", 1))}, 8, false}
	// Flesh and Blood: layouts change per set; the common shape is 11 C, 1 Rare, 1 rare or better (Majestic ~1 in 4),
	// 1 rainbow foil of any rarity, 1 equipment/token (common). Game rarities: Rare Rare, SR/Majestic Epic, Legendary+.
	Fab = Booster{"Flesh and Blood", "Flesh and Blood booster, 15 cards", []setfmt.Slot{slot(12, cC), slot(1, cU), slot(1, cAny), slot(1, w("Rare", 3, "Epic", 1))}, 7, true}
	// Union Arena: 12 cards (official pages from 2025), 8 in earlier English sets; Bandai publishes no slot odds.
	// Game rarities: U Rare, R/Union Rare Epic, SR and star versions Legendary.
	UnionArena12 = Booster{"Union Arena", "Union Arena booster (2025 on), 12 cards", []setfmt.Slot{slot(8, cC), slot(3, cU), slot(1, w("Epic", 4, "Legendary", 1))}, 0, true}
	UnionArena8  = Booster{"Union Arena", "Union Arena booster (before 2025), 8 cards", []setfmt.Slot{slot(5, cC), slot(2, cU), slot(1, w("Epic", 4, "Legendary", 1))}, 0, true}
)

// UnionArenaBooster picks by release date.
func UnionArenaBooster(releasedAt string) Booster {
	if releasedAt != "" && releasedAt < "2025-01-01" {
		return UnionArena8
	}
	return UnionArena12
}

// PackPresets is every real booster, game by game (Studio's Packs tab "Apply preset…").
func PackPresets() []Booster {
	return []Booster{MtgPlay, MtgDraft, MtgClassic, MtgTwelve, MtgEight,
		PkmSV, PkmModern, PkmEX, PkmClassic,
		YgoModern, YgoClassic,
		OnePiece, SwuBooster, Lorcana, Fab, UnionArena12, UnionArena8}
}

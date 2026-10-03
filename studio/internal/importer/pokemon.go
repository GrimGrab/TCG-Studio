package importer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/tcgdex"
)

// ---------------------------------------------------------------- TCGdex (Pokémon)

type tcgdexSource struct{ tc *tcgdex.Client }

func (s *tcgdexSource) Info() SourceInfo {
	colors := []Facet{}
	for _, t := range []string{"Grass", "Fire", "Water", "Lightning", "Psychic", "Fighting", "Darkness", "Metal", "Fairy", "Dragon",
		"Colorless", "Trainer", "Energy"} {
		colors = append(colors, Facet{t, t})
	}
	return SourceInfo{ID: "tcgdex", Name: "TCGdex", Game: "Pokémon TCG", Languages: tcgdex.Languages,
		ColorLabel: "Type", Colors: colors, RarityOrder: PokemonRarityOrder, Sorts: []Facet{{"power", "HP"}}}
}

// PokemonRarityOrder ranks TCGdex rarities (English names), lowest first. Keep in step with DefaultPokemonRarityMap.
var PokemonRarityOrder = []string{"None", "Common", "Uncommon", "Rare", "Rare Holo", "Holo Rare", "Promo", "Classic Collection",
	"Rare Holo LV.X", "Rare PRIME", "LEGEND", "Holo Rare V", "Holo Rare VMAX", "Holo Rare VSTAR", "Amazing Rare", "Radiant Rare",
	"Double rare", "ACE SPEC Rare", "Pikachu Rare", "Futuristic Rare", "Ultra Rare", "Full Art Trainer", "Shiny rare",
	"Shiny rare V", "Shiny rare VMAX", "Illustration rare", "Black White Rare", "Shiny Ultra Rare", "Secret Rare",
	"Special illustration rare", "Hyper rare", "Mega Hyper Rare", "Crown"}

func (s *tcgdexSource) DefaultOptions() Options {
	return Options{IncludeVariants: true, ImageWidth: 512, RarityMap: DefaultPokemonRarityMap(), Lang: "en"}
}

// PokemonSetID turns a TCGdex set id into a project id: "sv03.5" → "ptcg-sv03-5" (+ "-<lang>" for other languages).
func PokemonSetID(code, lang string) string {
	id := "ptcg-" + strings.ReplaceAll(CardID(code), ".", "-")
	if lang != "" && lang != "en" {
		id += "-" + lang
	}
	return id
}

func (s *tcgdexSource) ProjectID(code, lang string) string { return PokemonSetID(code, lang) }

// Sets lists the language's sets, newest series first (TCGdex lists release dates per series; the set list only per set).
// TCG Pocket sets are left out (digital only, no prices).
func (s *tcgdexSource) Sets(ctx context.Context, lang string) ([]SetInfo, error) {
	lang = langOr(lang)
	sets, err := s.tc.Sets(ctx, lang)
	if err != nil {
		return nil, err
	}
	series, err := s.tc.Series(ctx, lang)
	if err != nil {
		return nil, err
	}
	type place struct {
		serieID, serie, date string
		order                int
	}
	where := map[string]place{}
	for _, sb := range series {
		if sb.ID == "tcgp" {
			continue
		}
		se, err := s.tc.GetSerie(ctx, lang, sb.ID)
		if err != nil {
			return nil, err
		}
		for i, x := range se.Sets {
			where[x.ID] = place{sb.ID, se.Name, se.ReleaseDate, i}
		}
	}
	out := make([]SetInfo, 0, len(sets))
	order := map[string]place{}
	for _, x := range sets {
		pl, ok := where[x.ID]
		if !ok || x.CardCount.Total == 0 {
			continue
		}
		icon := x.Logo
		if icon != "" {
			icon += ".png"
		}
		promo := strings.Contains(strings.ToLower(x.Name), "promo")
		out = append(out, SetInfo{Code: x.ID, Name: x.Name, Group: pl.serie, Icon: icon, Cards: x.CardCount.Total,
			Main: !promo && pl.serieID != "misc"})
		order[x.ID] = pl
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := order[out[i].Code], order[out[j].Code]
		if a.date != b.date {
			return a.date > b.date
		}
		return a.order > b.order
	})
	return out, nil
}

func langOr(l string) string {
	if l == "" {
		return "en"
	}
	return l
}

// DefaultPokemonRarityMap maps TCGdex rarities to game rarities. SuperLegend is avoided (no rarity icon in the game).
// Rarities not listed go through PokemonRarity's fallback.
func DefaultPokemonRarityMap() map[string]string {
	m := map[string]string{"Common": "Common", "None": "Common", "Uncommon": "Rare",
		"Rare": "Epic", "Rare Holo": "Epic", "Holo Rare": "Epic", "Promo": "Epic", "Classic Collection": "Epic"}
	for _, r := range []string{"ACE SPEC Rare", "Amazing Rare", "Black White Rare", "Crown", "Double rare", "Full Art Trainer",
		"Futuristic Rare", "Holo Rare V", "Holo Rare VMAX", "Holo Rare VSTAR", "Hyper rare", "Illustration rare", "LEGEND",
		"Mega Hyper Rare", "Pikachu Rare", "Radiant Rare", "Rare Holo LV.X", "Rare PRIME", "Secret Rare", "Shiny Ultra Rare",
		"Shiny rare", "Shiny rare V", "Shiny rare VMAX", "Special illustration rare", "Ultra Rare"} {
		m[r] = "Legendary"
	}
	return m
}

// PokemonRarity maps a TCGdex rarity with the options' map; unknown ones become Common/Rare by name, else Legendary.
func PokemonRarity(m map[string]string, r string) string {
	if g := m[r]; g != "" {
		return g
	}
	l := strings.ToLower(r)
	switch {
	case l == "":
		return "Common"
	case strings.Contains(l, "uncommon"):
		return "Rare"
	case strings.Contains(l, "common"):
		return "Common"
	}
	return "Legendary"
}

// pokemonElement picks the game element closest to a Pokémon type (the vanilla play table only knows four).
func pokemonElement(types []string) string {
	if len(types) == 0 {
		return "Fire"
	}
	switch types[0] {
	case "Water":
		return "Water"
	case "Grass", "Fighting", "Darkness", "Metal":
		return "Earth"
	case "Lightning", "Psychic", "Colorless", "Fairy":
		return "Wind"
	}
	return "Fire" // Fire, Dragon
}

func pokemonMeta(c *tcgdex.Card) project.CardMeta {
	usd, foil, eur := c.Prices()
	m := project.CardMeta{SourceID: c.ID, Name: c.Name, TypeLine: c.TypeLine(), Colors: c.Types, SrcRarity: c.Rarity,
		USD: usd, USDFoil: foil, EUR: eur}
	if c.HP > 0 {
		m.Power = strconv.Itoa(c.HP) // the "HP" sort
	}
	return m
}

// fetchCards loads the full card records with a few workers. Cards that fail to load are returned by id in failed.
func (s *tcgdexSource) fetchCards(ctx context.Context, lang string, brief []tcgdex.CardBrief, report func(done, total int)) ([]*tcgdex.Card, []string, error) {
	out := make([]*tcgdex.Card, len(brief))
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		done   int
		failed []string
	)
	ch := make(chan int)
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range ch {
				c, err := s.tc.GetCard(ctx, lang, brief[i].ID)
				mu.Lock()
				if err != nil {
					failed = append(failed, brief[i].ID)
				} else {
					out[i] = c
				}
				done++
				if report != nil {
					report(done, len(brief))
				}
				mu.Unlock()
			}
		}()
	}
	for i := range brief {
		if ctx.Err() != nil {
			break
		}
		ch <- i
	}
	close(ch)
	wg.Wait()
	return out, failed, ctx.Err()
}

func (s *tcgdexSource) Import(ctx context.Context, ws project.Workspace, code string, opt Options, report func(Progress)) (*project.Project, error) {
	if opt.RarityMap == nil {
		opt.RarityMap = DefaultPokemonRarityMap()
	}
	lang := langOr(opt.Lang)
	id := PokemonSetID(code, lang)
	if _, err := os.Stat(ws.Folder(id)); err == nil {
		return nil, fmt.Errorf("%s is already imported — open it and use Refresh prices, or delete it first", id)
	}
	tset, err := s.tc.GetSet(ctx, lang, code)
	if err != nil {
		return nil, err
	}
	report(Progress{Stage: "cards", Total: len(tset.Cards), Message: "Fetching card list…"})
	cards, failed, err := s.fetchCards(ctx, lang, tset.Cards, func(done, total int) {
		report(Progress{Stage: "cards", Done: done, Total: total, Message: fmt.Sprintf("Fetched %d / %d cards", done, total)})
	})
	if err != nil {
		return nil, err
	}

	folder := ws.Folder(id)
	if err := os.MkdirAll(filepath.Join(folder, "images"), 0o755); err != nil {
		return nil, err
	}
	set := setfmt.NewSet(id, tset.Name)
	set.RenderMode = "FullImage"
	meta := &project.Meta{Source: "tcgdex", SetCode: tset.ID, Lang: lang, ReleasedAt: tset.ReleaseDate,
		ImportedAt: time.Now(), PricesUpdated: time.Now(), Cards: map[string]project.CardMeta{}}

	var jobs []imageJob
	used := map[string]int{}
	for _, c := range cards {
		if c == nil {
			continue
		}
		img := c.ImageURL("high")
		if img == "" {
			continue
		}
		cid := CardID(c.LocalID)
		if n := used[cid]; n > 0 {
			cid = fmt.Sprintf("%s-%d", cid, n+1)
		}
		used[CardID(c.LocalID)]++

		rel := "images/" + cid + ".png"
		play := setfmt.DefaultPlay()
		play.Element = pokemonElement(c.Types)
		card := setfmt.Card{ID: cid, Name: c.Name, Description: c.Text(), Artist: c.Illustrator,
			Rarity: PokemonRarity(opt.RarityMap, c.Rarity), Number: c.LocalID, Image: rel, Play: play}
		cm := pokemonMeta(c)
		card.Price = RealPrice(cm)
		set.Cards = append(set.Cards, card)
		meta.Cards[cid] = cm
		jobs = append(jobs, imageJob{url: img, path: filepath.Join(folder, filepath.FromSlash(rel))})
	}
	if len(set.Cards) == 0 {
		_ = os.RemoveAll(folder)
		return nil, fmt.Errorf("no cards with images found in set %s (%s)", code, lang)
	}

	// Default booster: 7 cards like the other sources — 4 commons, 2 uncommons, 1 rare-or-better.
	pack := setfmt.NewPack("booster", tset.Name+" Booster")
	pack.Slots = fitSlots([]setfmt.Slot{
		{Count: 4, Weights: map[string]float64{"Common": 1}},
		{Count: 2, Weights: map[string]float64{"Rare": 1}},
		{Count: 1, Weights: map[string]float64{"Epic": 6, "Legendary": 1}},
	}, set.Cards)
	pack.FoilChance = 15 // reverse holos are common in Pokémon packs
	set.Packs = append(set.Packs, pack)

	// Set logo (a layer for the pack art editor).
	if tset.Logo != "" {
		if b, err := s.tc.Download(ctx, tset.Logo+".png"); err == nil {
			_ = os.WriteFile(filepath.Join(folder, "images", "set_logo.png"), b, 0o644)
		}
	}

	imgFailed := downloadImages(ctx, tcgdex.NewUnthrottled().Download, jobs, opt.ImageWidth, report)
	if err := ctx.Err(); err != nil {
		_ = os.RemoveAll(folder)
		return nil, err
	}

	p := &project.Project{ID: id, Folder: folder, Set: set, Meta: meta}
	if err := ws.Save(p); err != nil {
		return nil, err
	}
	msg := fmt.Sprintf("Imported %d cards", len(set.Cards))
	if len(failed) > 0 {
		msg += fmt.Sprintf(" (%d cards failed to load: %s)", len(failed), strings.Join(failed, ", "))
	}
	if len(imgFailed) > 0 {
		msg += fmt.Sprintf(" (%d images failed: %s)", len(imgFailed), strings.Join(imgFailed, ", "))
	}
	report(Progress{Stage: "done", Done: len(jobs), Total: len(jobs), Message: msg})
	return p, nil
}

// RefreshMeta re-downloads each card and updates the real prices kept in studio.json (matched by TCGdex card id).
func (s *tcgdexSource) RefreshMeta(ctx context.Context, p *project.Project) (int, error) {
	if p.Meta.Source != "tcgdex" || p.Meta.SetCode == "" {
		return 0, fmt.Errorf("%s was not imported from TCGdex", p.ID)
	}
	var brief []tcgdex.CardBrief
	keys := map[string][]string{} // TCGdex id → our card ids
	for cid, m := range p.Meta.Cards {
		if m.SourceID == "" {
			continue
		}
		if len(keys[m.SourceID]) == 0 {
			brief = append(brief, tcgdex.CardBrief{ID: m.SourceID})
		}
		keys[m.SourceID] = append(keys[m.SourceID], cid)
	}
	cards, _, err := s.fetchCards(ctx, langOr(p.Meta.Lang), brief, nil)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, c := range cards {
		if c == nil {
			continue
		}
		usd, foil, eur := c.Prices()
		for _, cid := range keys[c.ID] {
			m := p.Meta.Cards[cid]
			m.USD, m.USDFoil, m.EUR = usd, foil, eur
			if m.Power == "" && c.HP > 0 {
				m.Power = strconv.Itoa(c.HP) // imported before the HP sort
			}
			p.Meta.Cards[cid] = m
			n++
		}
	}
	p.Meta.PricesUpdated = time.Now()
	return n, nil
}

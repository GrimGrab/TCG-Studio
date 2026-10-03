package importer

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"tcgstudio/internal/project"
	"tcgstudio/internal/scryfall"
	"tcgstudio/internal/tcgdex"
)

// SourceInfo describes an import source for the "Import sets" page.
type SourceInfo struct {
	ID        string   `json:"id"`        // stored as project Meta.Source
	Name      string   `json:"name"`      // "Scryfall"
	Game      string   `json:"game"`      // "Magic: The Gathering"
	Languages []string `json:"languages"` // empty = one language only
	// ColorLabel/Colors drive the editor's colour filter ("Color" W/U/B/R/G/M/C for Magic, "Type" for Pokémon).
	// M = multicolour and C = colourless are special values; any other value matches meta.colors or the start of meta.typeLine.
	ColorLabel string  `json:"colorLabel"`
	Colors     []Facet `json:"colors"`
	// RarityOrder lists the source's own rarities (meta.srcRarity), lowest first, for the editor's rarity filter and sort.
	RarityOrder []string `json:"rarityOrder"`
}

type Facet struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// SetInfo is one importable set, the same shape for every source. Sources return them newest first.
type SetInfo struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Group      string `json:"group"` // set type or series, used as the list filter
	ReleasedAt string `json:"releasedAt"`
	Icon       string `json:"icon"`
	IconMono   bool   `json:"iconMono"` // single-colour black icon (shown inverted on the dark UI)
	Cards      int    `json:"cards"`
	Main       bool   `json:"main"` // shown under "Main sets"
}

// Source is a card database Studio can import whole sets from. Add a new one by implementing this and listing it in NewRegistry.
type Source interface {
	Info() SourceInfo
	DefaultOptions() Options
	Sets(ctx context.Context, lang string) ([]SetInfo, error)
	ProjectID(code, lang string) string
	Import(ctx context.Context, ws project.Workspace, code string, opt Options, report func(Progress)) (*project.Project, error)
	// RefreshMeta updates the real prices in studio.json; card definitions are not touched. Returns how many cards changed.
	RefreshMeta(ctx context.Context, p *project.Project) (int, error)
}

type Registry struct{ list []Source }

func NewRegistry(sf *scryfall.Client, tc *tcgdex.Client) *Registry {
	return &Registry{list: []Source{&scryfallSource{sf}, &tcgdexSource{tc}}}
}

func (r *Registry) All() []Source { return r.list }

func (r *Registry) Get(id string) (Source, error) {
	for _, s := range r.list {
		if s.Info().ID == id {
			return s, nil
		}
	}
	return nil, fmt.Errorf("unknown import source %q", id)
}

// ---------------------------------------------------------------- Scryfall

type scryfallSource struct{ sf *scryfall.Client }

var scryfallMainTypes = map[string]bool{"core": true, "expansion": true, "masters": true, "draft_innovation": true,
	"commander": true, "starter": true, "funny": true}

func (s *scryfallSource) Info() SourceInfo {
	return SourceInfo{ID: "scryfall", Name: "Scryfall", Game: "Magic: The Gathering", ColorLabel: "Color",
		Colors:      []Facet{{"W", "White"}, {"U", "Blue"}, {"B", "Black"}, {"R", "Red"}, {"G", "Green"}, {"M", "Multicolor"}, {"C", "Colorless"}},
		RarityOrder: []string{"common", "uncommon", "rare", "mythic", "special", "bonus"}}
}

func (s *scryfallSource) DefaultOptions() Options { return DefaultOptions() }

func (s *scryfallSource) Sets(ctx context.Context, _ string) ([]SetInfo, error) {
	sets, err := s.sf.Sets(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SetInfo, 0, len(sets))
	for _, x := range sets {
		if x.Digital || x.CardCount == 0 {
			continue
		}
		out = append(out, SetInfo{Code: x.Code, Name: x.Name, Group: strings.ReplaceAll(x.SetType, "_", " "),
			ReleasedAt: x.ReleasedAt, Icon: x.IconSVGURI, IconMono: true, Cards: x.CardCount, Main: scryfallMainTypes[x.SetType]})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ReleasedAt > out[j].ReleasedAt })
	return out, nil
}

func (s *scryfallSource) ProjectID(code, _ string) string { return SetID(code) }

func (s *scryfallSource) Import(ctx context.Context, ws project.Workspace, code string, opt Options, report func(Progress)) (*project.Project, error) {
	return Import(ctx, s.sf, ws, code, opt, report)
}

func (s *scryfallSource) RefreshMeta(ctx context.Context, p *project.Project) (int, error) {
	return RefreshMeta(ctx, s.sf, p)
}

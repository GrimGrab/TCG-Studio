package main

import (
	"sort"
	"strings"

	"tcgstudio/internal/project"
)

// fillCollection gives a set its binder grouping when it has none yet: the import source's game ("Magic: The Gathering",
// "Pokémon"…) and the set's release date from studio.json. Local sources (image folder, EPL mod) don't know the game:
// the player types it on the Set tab. Returns true when something was filled in.
func (a *App) fillCollection(p *project.Project) bool {
	if p == nil || p.Set == nil || p.Meta == nil {
		return false
	}
	changed := false
	if p.Set.Collection == "" {
		if src, err := a.sources.Get(p.Meta.Source); err == nil && !src.Info().Local && src.Info().Game != "" {
			p.Set.Collection = src.Info().Game
			changed = true
		}
	}
	if p.Set.Released == "" && len(p.Meta.ReleasedAt) >= 10 {
		p.Set.Released = p.Meta.ReleasedAt[:10]
		changed = true
	}
	return changed
}

// Collections lists the collection names to suggest on the Set tab: every import source's game plus the ones the
// active setup's sets already use, sorted.
func (a *App) Collections() []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[strings.ToLower(s)] {
			seen[strings.ToLower(s)] = true
			out = append(out, s)
		}
	}
	for _, s := range a.sources.All() {
		if !s.Info().Local {
			add(s.Info().Game)
		}
	}
	if list, err := a.ws().List(a.settings.GameDir); err == nil {
		for _, s := range list {
			add(s.Collection)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}

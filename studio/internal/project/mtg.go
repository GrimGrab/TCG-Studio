package project

import (
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/game"
	"tcgstudio/internal/setfmt"
)

// FillMtg adds the MTG export data (set.json "mtg" fields) to a Scryfall-imported project from its meta, so the mod can
// export decks to Forge. Projects imported before this existed get it on load. Returns true when anything changed.
func FillMtg(p *Project) bool {
	if p == nil || p.Set == nil || p.Meta == nil || p.Meta.Source != "scryfall" || p.Meta.ScryfallCode == "" {
		return false
	}
	changed := false
	code := strings.ToUpper(p.Meta.ScryfallCode)
	if p.Set.Mtg == nil || p.Set.Mtg.SetCode != code {
		p.Set.Mtg = &setfmt.SetMtg{SetCode: code}
		changed = true
	}
	for i := range p.Set.Cards {
		c := &p.Set.Cards[i]
		m, ok := p.Meta.Cards[c.ID]
		if !ok {
			continue
		}
		want := setfmt.CardMtg{Name: ForgeName(c.Name, m), TypeLine: m.TypeLine, ManaCost: m.ManaCost, Colors: m.Colors,
			Rarity: m.SrcRarity, CMC: m.CMC, Power: m.Power, Toughness: m.Toughness, BackName: m.BackName}
		if c.Mtg == nil || !sameMtg(*c.Mtg, want) {
			c.Mtg = &want
			changed = true
		}
	}
	return changed
}

// PatchInstalledMtg copies the project's MTG export data (set + per card, matched by card id) into the copy of the set
// installed in the game, and nothing else — prices, licenses or unsaved-to-game edits stay as the player installed them.
// Returns true when the installed set.json changed.
func PatchInstalledMtg(p *Project, gameDir string) (bool, error) {
	if p == nil || p.Set == nil || !setfmt.SafeID(p.ID) || !game.IsGameDir(gameDir) {
		return false, nil
	}
	path := filepath.Join(game.SetsDir(gameDir), p.ID, SetFile)
	if _, err := os.Stat(path); err != nil {
		return false, nil // not installed
	}
	inst, err := setfmt.Load(path)
	if err != nil {
		return false, err
	}
	changed := false
	if p.Set.Mtg != nil && (inst.Mtg == nil || *inst.Mtg != *p.Set.Mtg) {
		m := *p.Set.Mtg
		inst.Mtg = &m
		changed = true
	}
	byID := map[string]*setfmt.CardMtg{}
	for i := range p.Set.Cards {
		if p.Set.Cards[i].Mtg != nil {
			byID[p.Set.Cards[i].ID] = p.Set.Cards[i].Mtg
		}
	}
	for i := range inst.Cards {
		want, ok := byID[inst.Cards[i].ID]
		if !ok || (inst.Cards[i].Mtg != nil && sameMtg(*inst.Cards[i].Mtg, *want)) {
			continue
		}
		m := *want
		inst.Cards[i].Mtg = &m
		changed = true
	}
	if !changed {
		return false, nil
	}
	return true, inst.Save(path)
}

// onlyVariantTags reports whether s is a list of importer.VariantTags names, e.g. "Borderless, Showcase".
func onlyVariantTags(s string) bool {
	for _, t := range strings.Split(s, ", ") {
		switch t {
		case "Borderless", "Showcase", "Extended Art", "Etched", "Full Art":
		default:
			return false
		}
	}
	return true
}

func sameMtg(a, b setfmt.CardMtg) bool {
	return a.Name == b.Name && a.TypeLine == b.TypeLine && a.ManaCost == b.ManaCost && strings.Join(a.Colors, "") == strings.Join(b.Colors, "") &&
		a.Rarity == b.Rarity && a.CMC == b.CMC && a.Power == b.Power && a.Toughness == b.Toughness && a.BackName == b.BackName
}

// ForgeName is the card name Forge accepts in a .dck: split cards keep "A // B", every other multi-face layout
// (transform, modal DFC, adventure, flip, meld) uses the front face. Measured with Forge 2.0.14's deck loader.
func ForgeName(displayName string, m CardMeta) string {
	name := m.Name
	if name == "" {
		// Imported before the meta kept the Scryfall name: the display name may carry " (Borderless, …)" variant tags
		// (older imports didn't record them in meta.Variant either, so recognise the tags themselves).
		name = displayName
		if i := strings.LastIndex(name, " ("); i > 0 && strings.HasSuffix(name, ")") && onlyVariantTags(name[i+2:len(name)-1]) {
			name = name[:i]
		}
	}
	// Scryfall "Human—Time Lord Meta-Crisis" (em dash); Forge 2.0.14 has no card name with an em/en dash, only hyphens.
	name = strings.NewReplacer("—", "-", "–", "-").Replace(name)
	faces := strings.Split(name, " // ")
	if len(faces) < 2 {
		return name
	}
	if m.Layout != "" {
		if m.Layout == "split" {
			return name
		}
		return faces[0]
	}
	// No layout recorded: split cards are two instants/sorceries without the Adventure subtype.
	for _, t := range strings.Split(m.TypeLine, " // ") {
		t = strings.TrimSpace(t)
		if strings.Contains(t, "Adventure") || !(strings.HasPrefix(t, "Instant") || strings.HasPrefix(t, "Sorcery")) {
			return faces[0]
		}
	}
	return name
}

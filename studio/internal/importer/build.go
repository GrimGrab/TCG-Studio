package importer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

// cardIn is one card as a source hands it to buildProject (already mapped from the source's API).
type cardIn struct {
	SourceID  string // unique per printing at the source; matches prices on refresh
	Name      string // shown name (variant tags are added by buildProject)
	Number    string // collector number / printing code; also the card id
	Text      string
	Artist    string
	SrcRarity string
	TypeLine  string
	Colors    []string // what the source's colour/type filter matches
	Variant   []string // printing tags ("Hyperspace", "Parallel") added to the shown name
	Cost      float64  // "cost" sort (stored as meta.cmc)
	Power     string   // "power" sort
	Element   string   // vanilla play element (Fire/Earth/Water/Wind)
	Image     string
	USD       *float64
	USDFoil   *float64
	EUR       *float64
}

// setIn describes a whole set for buildProject.
type setIn struct {
	ID         string // project id
	Source     string // meta.source
	Code       string // meta.setCode
	Name       string
	ReleasedAt string
	Lang       string
	Logo       string // optional image saved as images/set_logo.png (a pack art layer)
	Cards      []cardIn
	Slots      []setfmt.Slot
	FoilChance float64
	Rotate     bool    // turn landscape card art upright
	Aspect     float64 // > 0: stretch card art to this width/height (CardAspect for cards narrower than the game's slot)
	Get        func(context.Context, string) ([]byte, error)
	Rarity     func(src string) string // source rarity → game rarity
}

// buildProject writes a new project from a source's set: set.json with a default booster, images, studio.json.
func buildProject(ctx context.Context, ws project.Workspace, in setIn, opt Options, report func(Progress)) (*project.Project, error) {
	if _, err := os.Stat(ws.Folder(in.ID)); err == nil {
		return nil, fmt.Errorf("%s is already imported — open it and use Refresh prices, or delete it first", in.ID)
	}
	folder := ws.Folder(in.ID)
	set := setfmt.NewSet(in.ID, in.Name)
	set.RenderMode = "FullImage"
	meta := &project.Meta{Source: in.Source, SetCode: in.Code, Lang: in.Lang, ReleasedAt: in.ReleasedAt,
		ImportedAt: time.Now(), PricesUpdated: time.Now(), Cards: map[string]project.CardMeta{}}

	var jobs []imageJob
	used := map[string]int{}
	for _, c := range in.Cards {
		if c.Image == "" {
			continue
		}
		base := CardID(c.Number)
		cid := base
		if n := used[base]; n > 0 {
			cid = fmt.Sprintf("%s-%d", base, n+1)
		}
		used[base]++
		name := c.Name
		if len(c.Variant) > 0 {
			name += " (" + strings.Join(c.Variant, ", ") + ")"
		}
		play := setfmt.DefaultPlay()
		if c.Element != "" {
			play.Element = c.Element
		}
		rel := "images/" + cid + ".png"
		cm := project.CardMeta{SourceID: c.SourceID, Name: c.Name, TypeLine: c.TypeLine, Colors: c.Colors, SrcRarity: c.SrcRarity,
			Variant: c.Variant, CMC: c.Cost, Power: c.Power, USD: c.USD, USDFoil: c.USDFoil, EUR: c.EUR}
		card := setfmt.Card{ID: cid, Name: name, Description: c.Text, Artist: c.Artist, Rarity: in.Rarity(c.SrcRarity),
			Number: c.Number, Image: rel, Play: play, Price: RealPrice(cm)}
		set.Cards = append(set.Cards, card)
		meta.Cards[cid] = cm
		jobs = append(jobs, imageJob{url: c.Image, path: filepath.Join(folder, filepath.FromSlash(rel)), rotate: in.Rotate, aspect: in.Aspect})
	}
	if len(set.Cards) == 0 {
		if in.ReleasedAt > time.Now().Format("2006-01-02") {
			return nil, fmt.Errorf("%s isn't released yet (%s) — there are no card images for it yet", in.Name, in.ReleasedAt)
		}
		return nil, fmt.Errorf("no cards with images found in %s", in.Name)
	}
	if err := os.MkdirAll(filepath.Join(folder, "images"), 0o755); err != nil {
		return nil, err
	}

	pack := setfmt.NewPack("booster", in.Name+" Booster")
	pack.Slots = fitSlots(in.Slots, set.Cards)
	pack.FoilChance = in.FoilChance
	set.Packs = append(set.Packs, pack)

	if in.Logo != "" {
		_ = saveImage(ctx, in.Get, imageJob{url: in.Logo, path: filepath.Join(folder, "images", "set_logo.png")}, 0)
	}
	failed := downloadImages(ctx, in.Get, jobs, opt.ImageWidth, report)
	if err := ctx.Err(); err != nil {
		_ = os.RemoveAll(folder)
		return nil, err
	}
	// Cards whose art didn't download are left out rather than kept with a missing image.
	if len(failed) > 0 {
		bad := map[string]bool{}
		for _, f := range failed {
			bad["images/"+f] = true
		}
		kept := set.Cards[:0]
		for _, c := range set.Cards {
			if bad[c.Image] {
				delete(meta.Cards, c.ID)
				continue
			}
			kept = append(kept, c)
		}
		set.Cards = kept
		if len(set.Cards) == 0 {
			_ = os.RemoveAll(folder)
			return nil, fmt.Errorf("no card images could be downloaded for %s", in.Name)
		}
		pack.Slots = fitSlots(in.Slots, set.Cards)
		set.Packs[0] = pack
	}
	p := &project.Project{ID: in.ID, Folder: folder, Set: set, Meta: meta}
	if err := ws.Save(p); err != nil {
		return nil, err
	}
	msg := fmt.Sprintf("Imported %d cards", len(set.Cards))
	if len(failed) > 0 {
		msg += fmt.Sprintf(" (%d left out, their images failed: %s)", len(failed), strings.Join(failed, ", "))
	}
	report(Progress{Stage: "done", Done: len(jobs), Total: len(jobs), Message: msg})
	return p, nil
}

// fitSlots drops slot rarities the set has no cards of; a slot left empty takes the closest lower rarity the set has
// (e.g. modern Yu-Gi-Oh! sets have no Rare printings, 30th Celebration no Uncommons), so packs never draw from an empty bucket.
func fitSlots(slots []setfmt.Slot, cards []setfmt.Card) []setfmt.Slot {
	have := map[string]bool{}
	for _, c := range cards {
		have[c.Rarity] = true
	}
	ladder := []string{"Common", "Rare", "Epic", "Legendary"}
	var out []setfmt.Slot
	for _, s := range slots {
		w := map[string]float64{}
		top := -1
		for r, v := range s.Weights {
			if have[r] {
				w[r] = v
			}
			for i, l := range ladder {
				if l == r && i > top {
					top = i
				}
			}
		}
		for i := top; len(w) == 0 && i >= 0; i-- {
			if have[ladder[i]] {
				w[ladder[i]] = 1
			}
		}
		for i := top + 1; len(w) == 0 && i < len(ladder); i++ { // nothing lower: the next higher one
			if have[ladder[i]] {
				w[ladder[i]] = 1
			}
		}
		if len(w) > 0 {
			out = append(out, setfmt.Slot{Count: s.Count, Weights: w})
		}
	}
	return out
}

// refreshFrom updates the real prices (and missing sort numbers) in studio.json from a fresh card list, matched by SourceID.
func refreshFrom(p *project.Project, cards []cardIn) int {
	by := map[string]*cardIn{}
	for i := range cards {
		by[cards[i].SourceID] = &cards[i]
	}
	n := 0
	for cid, m := range p.Meta.Cards {
		c := by[m.SourceID]
		if m.SourceID == "" || c == nil {
			continue
		}
		m.USD, m.USDFoil, m.EUR = c.USD, c.USDFoil, c.EUR
		if m.CMC == 0 {
			m.CMC = c.Cost
		}
		if m.Power == "" {
			m.Power = c.Power
		}
		p.Meta.Cards[cid] = m
		n++
	}
	p.Meta.PricesUpdated = time.Now()
	return n
}

// rarityMapper maps a source rarity with the options' map, falling back to fallback for rarities not in it.
func rarityMapper(m map[string]string, fallback func(string) string) func(string) string {
	return func(r string) string {
		if g := m[r]; g != "" {
			return g
		}
		return fallback(r)
	}
}

// ladderMap builds a rarity map from groups: every rarity in groups[i] maps to game rarity names[i].
func ladderMap(groups map[string][]string) map[string]string {
	m := map[string]string{}
	for game, list := range groups {
		for _, r := range list {
			m[r] = game
		}
	}
	return m
}

func price(s string) *float64 {
	var f float64
	if _, err := fmt.Sscan(strings.TrimSpace(s), &f); err != nil || f <= 0 {
		return nil
	}
	return &f
}

func pricef(f float64) *float64 {
	if f <= 0 {
		return nil
	}
	return &f
}

// slug makes a project-id-safe lower-case code ("OP-01" → "op-01", "Hyperia City" → "hyperia-city").
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

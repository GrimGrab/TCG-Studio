package importer

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"tcgstudio/internal/art"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

// cardIn is one card as a source hands it to buildProject (already mapped from the source's API).
type cardIn struct {
	SourceID  string // unique per printing at the source; matches prices on refresh
	Name      string // shown name (variant tags are added by buildProject)
	Number    string // collector number / printing code; also the card id unless ID is set
	ID        string // card id when it shouldn't come from Number (image folder: the file name)
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
	Aspect    float64 // > 0: this card's own stretch target (overrides setIn.Aspect)
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
	Rarity     func(src string) string // source rarity → the card's rarity (a game rarity, or an id of Rarities)
	// Rarities: the set's own rarity list (EPL mods with their tiers kept); Slots keyed by game rarity are shared out over it
	// by Group (rarity id → the game rarity whose weight it shares).
	Rarities []setfmt.Rarity
	Group    func(id string) string
	// Local: images come from the player's own folder, so they are always copied again (a library file from an
	// earlier import of that folder may be an older picture).
	Local       bool
	SourceDir   string   // meta.sourceDir: the folder a local import read
	RarityOrder []string // meta.rarityOrder: this set's own rarities, lowest first (sources without a fixed list)
	// Packs replaces the default booster (EPL mods bring their own packs and art); slots are fitted to the cards.
	Packs []setfmt.Pack
	// Extra images written into the project folder through Get (pack/box art, the card back).
	Extra      []extraImage
	RenderMode string // "" = FullImage
}

// extraImage is a non-card image of an import: saved as PNG at Rel, or composed into the set's card back.
type extraImage struct {
	URL, Rel string
	CardBack bool
}

// buildProject writes a new project from a source's set: set.json with a default booster, images, studio.json.
func buildProject(ctx context.Context, ws project.Workspace, in setIn, opt Options, report func(Progress)) (*project.Project, error) {
	if _, err := os.Stat(ws.Folder(in.ID)); err == nil {
		return nil, fmt.Errorf("%s is already imported — open it and use Refresh prices, or delete it first", in.ID)
	}
	folder := ws.Folder(in.ID)
	art, err := cardArtTarget(ws, in.ID, folder, in.Source, in.Code, in.Lang, opt)
	if err != nil {
		return nil, err
	}
	set := setfmt.NewSet(in.ID, in.Name)
	set.Rarities = in.Rarities
	set.RenderMode = "FullImage"
	if in.RenderMode != "" {
		set.RenderMode = in.RenderMode
	}
	meta := &project.Meta{Source: in.Source, SetCode: in.Code, Lang: in.Lang, ReleasedAt: in.ReleasedAt,
		ImportedAt: time.Now(), PricesUpdated: time.Now(), Cards: map[string]project.CardMeta{},
		SourceDir: in.SourceDir, RarityOrder: in.RarityOrder}

	var jobs []imageJob
	used := map[string]int{}
	for _, c := range in.Cards {
		if c.Image == "" {
			continue
		}
		base := CardID(c.Number)
		if c.ID != "" {
			base = c.ID
		}
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
		rel := art.rel(cid)
		cm := project.CardMeta{SourceID: c.SourceID, Name: c.Name, TypeLine: c.TypeLine, Colors: c.Colors, SrcRarity: c.SrcRarity,
			Variant: c.Variant, CMC: c.Cost, Power: c.Power, USD: c.USD, USDFoil: c.USDFoil, EUR: c.EUR}
		card := setfmt.Card{ID: cid, Name: name, Description: c.Text, Artist: c.Artist, Rarity: in.Rarity(c.SrcRarity),
			Number: c.Number, Image: rel, Play: play, Price: RealPrice(cm)}
		set.Cards = append(set.Cards, card)
		meta.Cards[cid] = cm
		j := art.job(c.Image, cid)
		j.rotate, j.aspect = in.Rotate, in.Aspect
		if c.Aspect > 0 {
			j.aspect = c.Aspect
		}
		if in.Local {
			j.keep = false
		}
		jobs = append(jobs, j)
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

	packs := in.Packs
	if len(packs) == 0 {
		pack := setfmt.NewPack("booster", in.Name+" Booster")
		pack.FoilChance = in.FoilChance
		for _, sl := range in.Slots {
			pack.Slots = append(pack.Slots, setfmt.Slot{Count: sl.Count, Weights: set.SplitWeights(sl.Weights, in.Group)})
		}
		packs = []setfmt.Pack{pack}
	}
	for _, pk := range packs {
		fitted := pk
		fitted.Slots = fitSlots(pk.Slots, set)
		if n := setfmt.SlotTotal(fitted.Slots); n > 0 {
			fitted.CardsPerPack = n // the slots decide the pack's size
		}
		set.Packs = append(set.Packs, fitted)
	}

	if in.Logo != "" {
		_ = saveImage(ctx, in.Get, imageJob{url: in.Logo, path: filepath.Join(art.dir, "set_logo.png")}, 0) // next to the card art (the set's shared folder)
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
		for i := range set.Packs {
			set.Packs[i].Slots = fitSlots(packs[i].Slots, set)
		}
	}
	// Pack/box art and the card back go next to the card art too (the set's shared folder when it has one), like every set file
	// Studio writes (Project.WriteTarget) — else Storage's "Move everything to shared" would offer them right after the import.
	failed = append(failed, writeExtras(ctx, in, filepath.Dir(art.dir), set)...)
	p := &project.Project{ID: in.ID, Folder: folder, LibFolder: ws.LibFolder(in.ID), Set: set, Meta: meta}
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
// "Lower" follows the set's rarity list.
func fitSlots(slots []setfmt.Slot, set *setfmt.Set) []setfmt.Slot {
	have := map[string]bool{}
	for _, c := range set.Cards {
		have[c.Rarity] = true
	}
	var ladder []string
	for _, r := range set.RarityList() {
		ladder = append(ladder, r.ID)
	}
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

// writeExtras saves an import's pack/box art and card back into the project folder. An image that fails leaves its
// field empty (generated art is used instead) and is reported with the failed images.
// writeExtras saves the import's extra images under base (the folder their set.json paths are relative to).
func writeExtras(ctx context.Context, in setIn, base string, set *setfmt.Set) []string {
	var failed []string
	for _, e := range in.Extra {
		out := filepath.Join(base, filepath.FromSlash(e.Rel))
		err := func() error {
			b, err := in.Get(ctx, e.URL)
			if err != nil {
				return err
			}
			if e.CardBack {
				img, _, err := image.Decode(bytes.NewReader(b))
				if err != nil {
					return err
				}
				return art.ComposeCardBack(img, out)
			}
			return writeFileAtomic(out, b)
		}()
		if err == nil {
			if e.CardBack {
				set.CardBack = e.Rel
			}
			continue
		}
		failed = append(failed, path.Base(e.Rel))
		for i := range set.Packs {
			pk := &set.Packs[i]
			for _, f := range []*string{&pk.PackTexture, &pk.PackIcon, &pk.BoxTexture, &pk.BoxIcon} {
				if *f == e.Rel {
					*f = ""
				}
			}
		}
	}
	return failed
}

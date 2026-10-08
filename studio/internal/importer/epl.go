package importer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/image/draw"

	"tcgstudio/internal/epl"
	"tcgstudio/internal/origin"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/uvmap"
)

// ---------------------------------------------------------------- Enhanced Prefab Loader mods (experimental)
// A mod made for EPL (a folder or .zip with X.json + asset bundle X) is converted once into Studio sets with the mod's
// own card art, pack/box art and card back, and pack odds translated to ours (internal/epl). Nothing of EPL is needed in
// game afterwards. Code = "<mod path>|<descriptor path in the mod>|<expansion enum name>".

type eplSource struct{}

// EPLCacheDir is where zipped mods are unpacked (set by the app to its cache folder; a temp folder otherwise).
var EPLCacheDir = filepath.Join(os.TempDir(), "TCG Studio")

func (eplSource) Info() SourceInfo {
	return SourceInfo{ID: "epl", Name: "Enhanced Prefab Loader (experimental)", Game: "Import EPL mod", Local: true, OwnArt: true}
}

// DefaultOptions: a mod's tiers are kept as the set's rarities (with their pack odds) unless the player turns it off.
func (eplSource) DefaultOptions() Options { return Options{ImageWidth: 512, KeepRarities: true} }

func (eplSource) Sets(context.Context, string) ([]SetInfo, error) { return nil, nil }

func (eplSource) ProjectID(code, _ string) string {
	_, _, exp := SplitEPLCode(code)
	return EPLProjectID(exp)
}

// EPLProjectID is the project id of a converted expansion: "epl-" + its enum name (EPL needs those unique in a game).
func EPLProjectID(expansion string) string {
	s := slug(expansion)
	if len(s) > 40 {
		s = strings.TrimSuffix(s[:40], "-")
	}
	if s == "" {
		s = "set"
	}
	return "epl-" + s
}

// leadingNumber is a number a mod puts before its names ("001 - 2019 Base", "12: Playmat"). It only counts with a
// separator after it, so names that start with a number ("2019 Base") stay whole.
var leadingNumber = regexp.MustCompile(`^#?\d{1,5}\s*[-–—:.)|]\s*`)

// modName is a mod's display name, without its leading number when strip is set (the Import page's "Strip leading
// numbers"). Card names are never stripped ("3-D Man").
func modName(s string, strip bool) string {
	s = strings.TrimSpace(s)
	if strip {
		if t := strings.TrimSpace(leadingNumber.ReplaceAllString(s, "")); t != "" {
			return t
		}
	}
	return s
}

// eplOrigin records where converted content came from (item = the item's name in the mod, "" for a set).
func eplOrigin(mod *epl.Mod, b *epl.Bundle, item string, opt Options) *origin.Origin {
	name := strings.TrimSpace(opt.OriginMod)
	if name == "" {
		name = strings.TrimSpace(b.Desc.Name)
	}
	if name == "" {
		name = mod.Name
	}
	return &origin.Origin{Kind: "EPL mod", Mod: name, Author: strings.TrimSpace(opt.OriginAuthor),
		Link: strings.TrimSpace(opt.OriginLink), Package: mod.Name, Path: mod.Source, Bundle: b.Rel,
		Item: strings.TrimSpace(item), Imported: time.Now()}
}

// defaultModName is what the Import page pre-fills as the mod's name: its first descriptor's Name, else the folder/zip name.
func defaultModName(mod *epl.Mod) string {
	for _, b := range mod.Bundles {
		if n := strings.TrimSpace(b.Desc.Name); n != "" {
			return n
		}
	}
	return mod.Name
}

func EPLCode(mod, descriptor, expansion string) string {
	return mod + "|" + descriptor + "|" + expansion
}

func SplitEPLCode(code string) (mod, descriptor, expansion string) {
	parts := strings.SplitN(code, "|", 3)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	if len(parts) == 3 && parts[2] == "" && parts[1] == "" { // a bare expansion name (registry checks)
		return "", "", parts[0]
	}
	return parts[0], parts[1], parts[2]
}

// EPLPreview is what the Import page shows for a mod before importing.
type EPLPreview struct {
	Mod      string       `json:"mod"`
	Name     string       `json:"name"`
	ModName  string       `json:"modName"` // pre-filled mod name (descriptor Name), editable before importing
	Sets     []EPLSet     `json:"sets"`
	Items    []EPLItem    `json:"items"`   // accessories it converts
	Skipped  []EPLSkipped `json:"skipped"` // mod content that isn't converted
	Warnings []string     `json:"warnings"`
}

type EPLSet struct {
	Code       string     `json:"code"`
	ProjectID  string     `json:"projectId"`
	Imported   bool       `json:"imported"` // set by the app
	Name       string     `json:"name"`
	Cards      int        `json:"cards"`
	Tiers      []epl.Tier `json:"tiers"`
	Packs      []EPLPack  `json:"packs"`
	RenderMode string     `json:"renderMode"`
	Problems   []string   `json:"problems"` // cards that can't be imported (first few) …
	ProblemN   int        `json:"problemCount"`
	Warnings   []string   `json:"warnings"`
}

type EPLPack struct {
	Name       string  `json:"name"`
	Box        string  `json:"box"`
	Strategy   string  `json:"strategy"`
	FoilChance float64 `json:"foilChance"`
}

type EPLSkipped struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

// ScanEPL reads a mod for the preview: every card expansion with its tiers, default rarities and packs, which cards
// can't be read, and what else the mod contains.
func ScanEPL(path string, strip bool, progress func(done, total int64)) (EPLPreview, error) {
	mod, err := epl.Open(path, EPLCacheDir, progress)
	if err != nil {
		return EPLPreview{}, err
	}
	pv := EPLPreview{Mod: path, Name: mod.Name, ModName: defaultModName(mod)}
	for _, b := range mod.Bundles {
		assets, err := epl.OpenAssets(b.Path)
		if err != nil {
			pv.Warnings = append(pv.Warnings, fmt.Sprintf("%s: %v", b.Rel, err))
			continue
		}
		for i := range b.Desc.CardExpansions {
			exp := &b.Desc.CardExpansions[i]
			sp := epl.PlanSet(b.Desc, exp)
			set := EPLSet{Code: EPLCode(path, b.Rel, exp.CardExpansion), ProjectID: EPLProjectID(exp.CardExpansion),
				Name: modName(exp.Name, strip), Cards: len(sp.Cards), Tiers: sp.Tiers, Warnings: sp.Warnings}
			set.RenderMode, set.Problems, set.ProblemN = checkCards(assets, sp)
			for _, pp := range sp.Packs {
				ep := EPLPack{Name: modName(pp.Item.Name, strip), Strategy: pp.Strategy, FoilChance: math.Round(pp.FoilChance*10) / 10}
				if pp.Box != nil {
					ep.Box = modName(pp.Box.Name, strip)
				}
				set.Packs = append(set.Packs, ep)
			}
			pv.Sets = append(pv.Sets, set)
		}
		items, skipped := eplItems(&b, assets, strip)
		pv.Items = append(pv.Items, items...)
		pv.Skipped = append(pv.Skipped, skipped...)
		assets.Close()
	}
	return pv, nil
}

// checkCards reads every card image's header: the render mode from their shape and the cards that can't be imported.
func checkCards(a *epl.Assets, sp *epl.SetPlan) (mode string, problems []string, n int) {
	cardShaped := 0
	for _, c := range sp.Cards {
		w, h, err := a.SpriteSize(c.Sprite)
		if err != nil {
			n++
			if len(problems) < 8 {
				problems = append(problems, fmt.Sprintf("%s: %v", c.Name, err))
			}
			continue
		}
		if cardShape(w, h) > 0 {
			cardShaped++
		}
	}
	mode = "Framed"
	if cardShaped*2 >= len(sp.Cards)-n {
		mode = "FullImage"
	}
	return mode, problems, n
}

// cardShape is CardAspect when an image (turned upright) is within 12 % of a card's shape, else 0.
func cardShape(w, h int) float64 {
	if w <= 0 || h <= 0 {
		return 0
	}
	fw, fh := float64(w), float64(h)
	if fw > fh {
		fw, fh = fh, fw
	}
	if math.Abs(fw/fh-CardAspect)/CardAspect <= 0.12 {
		return CardAspect
	}
	return 0
}

func (s eplSource) Import(ctx context.Context, ws project.Workspace, code string, opt Options, report func(Progress)) (*project.Project, error) {
	path, rel, expName := SplitEPLCode(code)
	report(Progress{Stage: "cards", Message: "Reading the mod…"})
	mod, err := epl.Open(path, EPLCacheDir, func(done, total int64) {
		report(Progress{Stage: "cards", Done: int(done >> 20), Total: int(total >> 20), Message: "Unpacking the mod…"})
	})
	if err != nil {
		return nil, err
	}
	var b *epl.Bundle
	var exp *epl.CardExpansion
	for i := range mod.Bundles {
		if mod.Bundles[i].Rel != rel {
			continue
		}
		b = &mod.Bundles[i]
		for j := range b.Desc.CardExpansions {
			if b.Desc.CardExpansions[j].CardExpansion == expName {
				exp = &b.Desc.CardExpansions[j]
			}
		}
	}
	if exp == nil {
		return nil, fmt.Errorf("card set %q not found in %s", expName, filepath.Base(path))
	}
	assets, err := epl.OpenAssets(b.Path)
	if err != nil {
		return nil, err
	}
	defer assets.Close()
	sp := epl.PlanSet(b.Desc, exp)
	rarity := sp.DefaultRarities()    // tier → game rarity (with the mod's rarities kept: the game rarity it counts as)
	for t, g := range opt.RarityMap { // the player's choices in the preview
		if _, ok := rarity[t]; ok && contains(setfmt.GameRarities, g) {
			rarity[t] = g
		}
	}
	mode, _, _ := checkCards(assets, sp)

	// Rarity of each card: with KeepRarities every tier is a rarity of the set (its exact pack odds kept), else its game rarity.
	cardRarity := rarity
	var own []setfmt.Rarity
	groupOf := map[string]string{} // own rarity id → its tier's odds group (only orders the list and shares a fallback booster)
	if opt.KeepRarities {
		cardRarity = map[string]string{}
		used := map[string]bool{}
		// Lowest first: by the game rarity each counts as, then most pulled first (mods list tiers in any order, DBS
		// alphabetically). This order is the binder's rarity sort and Gamify's price ranges; prices stay the mod's.
		tiers := append([]epl.Tier(nil), sp.Tiers...)
		gameRank := func(t epl.Tier) int {
			for i, g := range setfmt.GameRarities {
				if g == rarity[t.Name] {
					return i
				}
			}
			return 0
		}
		sort.SliceStable(tiers, func(i, j int) bool {
			if a, b := gameRank(tiers[i]), gameRank(tiers[j]); a != b {
				return a < b
			}
			return tiers[i].PerCard > tiers[j].PerCard
		})
		for _, t := range tiers {
			id := setfmt.RarityID(t.Name)
			for n := 2; used[id]; n++ {
				id = fmt.Sprintf("%s-%d", setfmt.RarityID(t.Name), n)
			}
			used[id] = true
			cardRarity[t.Name] = id
			groupOf[id] = rarity[t.Name]
			own = append(own, setfmt.Rarity{ID: id, Name: t.Name})
		}
	}

	// Prices: EPL's own price generator for the expansion (expected value), Base border; foil = its foil entry's price.
	pm := epl.PriceModelOf(exp)

	var cards []cardIn
	for _, c := range sp.Cards {
		ci := cardIn{SourceID: c.Sprite, ID: slug(c.Sprite), Name: c.Name, Number: fmt.Sprint(c.Order), SrcRarity: c.Tier,
			Element: c.Element, Image: "sprite:" + c.Sprite}
		if c.PlainRarity != "" {
			ci.USD = pricef(pm.Price(c.PlainRarity, 0, false))
		}
		if c.FoilRarity != "" {
			ci.USDFoil = pricef(pm.Price(c.FoilRarity, 0, true))
		} else if c.PlainRarity != "" && (exp.HasRandomFoils || len(sp.Packs) == 0 || anyFoil(sp)) {
			ci.USDFoil = pricef(pm.Price(c.PlainRarity, 0, true))
		}
		if c.Variant != "" {
			ci.Variant = []string{c.Variant}
		}
		if w, h, err := assets.SpriteSize(c.Sprite); err == nil && mode == "FullImage" {
			ci.Aspect = cardShape(w, h)
		}
		cards = append(cards, ci)
	}

	in := setIn{ID: EPLProjectID(exp.CardExpansion), Source: "epl", Code: exp.CardExpansion, Name: modName(exp.Name, opt.StripNumbers),
		Cards: cards, Rotate: mode == "FullImage", Local: true, SourceDir: path, RarityOrder: sp.TierOrder(), RenderMode: mode,
		Rarities: own, Group: func(id string) string { return groupOf[id] },
		Rarity: func(t string) string {
			if g := cardRarity[t]; g != "" {
				return g
			}
			if len(own) > 0 {
				return own[0].ID
			}
			return "Common"
		},
		Slots: []setfmt.Slot{ // used when the mod has no pack items for this set
			{Count: 4, Weights: map[string]float64{"Common": 1}},
			{Count: 2, Weights: map[string]float64{"Rare": 1}},
			{Count: 1, Weights: map[string]float64{"Epic": 4, "Legendary": 1}},
		}, FoilChance: 5,
		Get: func(_ context.Context, url string) ([]byte, error) { return eplImage(assets, url) }}
	used := map[string]bool{}
	for _, pp := range sp.Packs {
		id := slug(pp.Item.Name)
		if id == "" || used[id] {
			id = fmt.Sprintf("pack%d", len(in.Packs)+1)
		}
		used[id] = true
		pk := setfmt.NewPack(id, modName(pp.Item.Name, opt.StripNumbers))
		pk.Slots = pp.Slots(cardRarity)
		pk.FoilChance = math.Round(pp.FoilChance*100) / 100
		pk.AllowDuplicates = pp.Item.CanHaveDuplicates
		if pp.Item.BaseCost > 0 {
			pk.PackCost, pk.MarketMin, pk.MarketMax = pp.Item.BaseCost, pp.Item.BaseCost, math.Round(pp.Item.BaseCost*120)/100
		}
		if pp.Item.LicenseLevelRequirement > 0 {
			pk.License.PackLevel = pp.Item.LicenseLevelRequirement
		}
		if pp.Item.LicensePrice > 0 {
			pk.License.PackPrice = pp.Item.LicensePrice
		}
		art := func(url, rel string, field *string) {
			in.Extra = append(in.Extra, extraImage{URL: url, Rel: rel})
			*field = rel
		}
		if pp.Item.Material != "" {
			art("material:"+pp.Item.Material, "images/pack_"+id+".png", &pk.PackTexture)
		}
		if pp.Item.SpriteName != "" {
			art("sprite:"+pp.Item.SpriteName, "images/pack_"+id+"_icon.png", &pk.PackIcon)
		}
		// Every pack gets a booster box like any imported set (the mod's own when it has one; otherwise Studio makes its
		// art from the pack's front after the import, see App.ImportSet).
		pk.HasBox = true
		if bx := pp.Box; bx != nil {
			pk.BoxName = modName(bx.Name, opt.StripNumbers)
			if bx.LicenseLevelRequirement > 0 {
				pk.License.BoxLevel = bx.LicenseLevelRequirement
			}
			if bx.LicensePrice > 0 {
				pk.License.BoxPrice = bx.LicensePrice
			}
			if bx.Material != "" {
				art("box:"+bx.MeshToUse+"|"+bx.Material, "images/box_"+id+".png", &pk.BoxTexture)
			}
			if bx.SpriteName != "" {
				art("sprite:"+bx.SpriteName, "images/box_"+id+"_icon.png", &pk.BoxIcon)
			}
		}
		in.Packs = append(in.Packs, pk)
	}
	for _, back := range []string{sp.CardBack, sp.CardBackAlt} {
		if _, _, err := assets.SpriteSize(back); back != "" && err == nil {
			in.Extra = append(in.Extra, extraImage{URL: "cardback:" + back, Rel: "images/card_back.png", CardBack: true})
			break
		}
	}
	p, err := buildProject(ctx, ws, in, opt, report)
	if err != nil {
		return nil, err
	}
	p.Meta.Origin = eplOrigin(mod, b, "", opt)
	// EPL's border/foil multipliers; Real pricing (prices from EPL's generator, above) keeps them.
	pd := setfmt.PriceDefaults{BorderMultipliers: pm.BorderMultipliers(), FoilMultiplier: math.Round(pm.FoilMult*100) / 100, Minimum: 0.01}
	p.Set.PriceDefaults = pd
	p.Meta.SrcPriceDefaults = &pd
	p.Meta.Pricing = &project.Pricing{Mode: "real", BorderCurve: "gentle", TierStep: 0}
	// The pack/box 3D editor starts from the mod's own art ("Earlier art (snapshot)"), not the vanilla pack and box.
	if p.Meta.PackArt == nil {
		p.Meta.PackArt = map[string]map[string]json.RawMessage{}
	}
	for _, pk := range p.Set.Packs {
		art := map[string]json.RawMessage{}
		for which, file := range map[string]string{"pack": pk.PackTexture, "box": pk.BoxTexture} {
			if file != "" {
				art[which], _ = json.Marshal(map[string]any{"version": 2, "layers": []any{}, "base": "vanilla",
					"baseColor": "#3a3a3a", "baseItem": "file", "baseFile": file})
			}
		}
		if len(art) > 0 {
			p.Meta.PackArt[pk.ID] = art
		}
	}
	if err := ws.Save(p); err != nil {
		return nil, err
	}
	return p, nil
}

// eplImage renders a bundle image as PNG: "sprite:<name>", "material:<name>" (its main texture) or
// "box:<vanilla box mesh>|<material>" (a box texture moved to the Basic box's layout, which our boxes use).
func eplImage(a *epl.Assets, url string) ([]byte, error) {
	kind, name, _ := strings.Cut(url, ":")
	var img *image.NRGBA
	var err error
	switch kind {
	case "sprite":
		img, err = a.Image(name)
	case "cardback":
		if img, err = a.Image(name); err == nil {
			img = cardBackArt(img)
		}
	case "material":
		img, err = a.MaterialTexture(name)
	case "box":
		base, mat, _ := strings.Cut(name, "|")
		if img, err = a.MaterialTexture(mat); err == nil {
			img = basicBoxLayout(img, base)
		}
	default:
		err = fmt.Errorf("unknown image %q", url)
	}
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// cardBackArt cuts the art window out of a card-back texture in the game's card-back mesh layout (T_CardBackMesh, square:
// back face plus edge strip; window x78–692, y66–927 of 1024). A card-shaped image is already just the art.
func cardBackArt(src *image.NRGBA) *image.NRGBA {
	b := src.Bounds()
	if b.Dx() != b.Dy() || b.Dx() == 0 {
		return src
	}
	k := float64(b.Dx()) / 1024
	win := image.Rect(int(78*k), int(66*k), int(693*k), int(928*k)).Add(b.Min)
	return src.SubImage(win).(*image.NRGBA)
}

// basicBoxLayout copies a vanilla box texture's faces for box mesh base (RareCardBox, DestinyEpicCardBox, …: other areas
// of the shared box atlas) into the Basic box's areas. Basic layouts and unknown bases are returned unchanged.
func basicBoxLayout(src *image.NRGBA, base string) *image.NRGBA {
	model, _ := uvmap.ModelFor("Box")
	var b *uvmap.Base
	for i := range model.Bases {
		if strings.EqualFold(model.Bases[i].ID, base) {
			b = &model.Bases[i]
		}
	}
	if b == nil || b.Targets == nil || model.TextureSize == 0 {
		return src
	}
	scale := float64(src.Bounds().Dx()) / float64(model.TextureSize)
	r := func(v [4]float64) image.Rectangle {
		return image.Rect(int(v[0]*scale), int(v[1]*scale), int(v[2]*scale), int(v[3]*scale))
	}
	dst := image.NewNRGBA(src.Bounds())
	draw.Copy(dst, image.Point{}, src, src.Bounds(), draw.Src, nil)
	for _, f := range model.Faces {
		ts := b.Targets[f.ID]
		if len(ts) == 0 {
			continue
		}
		face := faceImage(src, r(ts[0].Rect), ts[0])
		targets := append([]uvmap.Target{}, f.Targets...)
		sort.SliceStable(targets, func(i, j int) bool { return targets[i].Bleed && !targets[j].Bleed }) // bleed first
		for _, t := range targets {
			draw.ApproxBiLinear.Scale(dst, r(t.Rect), face, face.Bounds(), draw.Src, nil)
		}
	}
	return dst
}

// faceImage cuts a face out of a texture area, undoing the area's transpose/flips so the face is upright.
func faceImage(src *image.NRGBA, area image.Rectangle, t uvmap.Target) *image.NRGBA {
	aw, ah := area.Dx(), area.Dy()
	fw, fh := aw, ah
	if t.Transpose {
		fw, fh = ah, aw
	}
	out := image.NewNRGBA(image.Rect(0, 0, fw, fh))
	for y := 0; y < fh; y++ {
		for x := 0; x < fw; x++ {
			tx, ty := x, y
			if t.Transpose {
				tx, ty = y, x
			}
			if t.FlipX {
				tx = aw - 1 - tx
			}
			if t.FlipY {
				ty = ah - 1 - ty
			}
			out.SetNRGBA(x, y, src.NRGBAAt(area.Min.X+tx, area.Min.Y+ty))
		}
	}
	return out
}

func (eplSource) RefreshMeta(_ context.Context, p *project.Project) (int, error) {
	return 0, fmt.Errorf("%s came from a mod: its prices come from the mod's price settings and don't change (use Gamify to re-price it)", p.ID)
}

// anyFoil reports whether the set's packs give foils (then cards without a foil entry still get a foil price).
func anyFoil(sp *epl.SetPlan) bool {
	for _, pp := range sp.Packs {
		if pp.FoilChance > 0 {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

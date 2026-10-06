package importer

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"image"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
)

// ---------------------------------------------------------------- Image folder (the player's own card images)
// For games no public database covers (e.g. Topps' Marvel Collect, checked 2026-10-05): the player points Studio at a
// folder of card images they own. Layout:
//
//	<folder>\            set name = folder name
//	  logo.png|set.png   optional set logo (pack art icon)
//	  cards.csv          optional: file,name,number,rarity,price,foilPrice,artist,text,type (only "file" required)
//	  <Rarity>\…         optional rarity subfolders, lowest first by name ("1 Common", "2 Rare"…); top-level images = Common
//
// The file name is the card's name and id; cards are numbered in file-name order. With Options.StripNumbers a leading
// number is the card's number instead ("001 Captain Marvel.png" → #001 "Captain Marvel"). The import "code" is the
// folder's absolute path.

type folderSource struct{}

// FolderRarityOrder lists the rarity names the image folder knows by name, lowest first; other folder names are ranked
// by their order in the folder.
var FolderRarityOrder = []string{"Common", "Base", "Basic", "Uncommon", "Rare", "Super Rare", "Epic", "Legendary",
	"Mythic", "Ultra Rare", "Secret Rare", "Special"}

func DefaultFolderRarityMap() map[string]string {
	m := map[string]string{}
	for _, r := range FolderRarityOrder {
		m[r] = keywordRarity(r)
	}
	return m
}

// keywordRarity maps a rarity name to a game rarity by its words ("" when no word is known).
func keywordRarity(r string) string {
	s := strings.ToLower(r)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(s, w) {
				return true
			}
		}
		return false
	}
	switch {
	case has("legend", "mythic", "secret", "ultra", "special", "exclusive", "limited", "hyper", "chase"):
		return "Legendary"
	case has("super rare", "epic", "very rare"):
		return "Epic"
	case has("uncommon"):
		return "Rare"
	case has("rare"):
		return "Epic"
	case has("common", "base", "basic"):
		return "Common"
	}
	return ""
}

func (folderSource) Info() SourceInfo {
	return SourceInfo{ID: "folder", Name: "an image folder", Game: "Your own card images", RarityOrder: FolderRarityOrder, Local: true}
}

func (folderSource) DefaultOptions() Options {
	return Options{ImageWidth: 512, RarityMap: DefaultFolderRarityMap()}
}

func (folderSource) Sets(context.Context, string) ([]SetInfo, error) { return nil, nil }

// ProjectID is "img-" + the folder's name ("Heroines of Marvel" → img-heroines-of-marvel).
func (folderSource) ProjectID(dir, _ string) string {
	s := slug(filepath.Base(filepath.Clean(dir)))
	if s == "" {
		s = "set"
	}
	if len(s) > 40 {
		s = strings.TrimSuffix(s[:40], "-")
	}
	return "img-" + s
}

// FolderPreview is what the Import page shows before an image-folder import.
type FolderPreview struct {
	Dir        string         `json:"dir"`
	Name       string         `json:"name"`
	ProjectID  string         `json:"projectId"`
	Imported   bool           `json:"imported"` // set by the app: a project with this id exists
	Cards      int            `json:"cards"`
	Rarities   []RarityCount  `json:"rarities"`
	CSV        bool           `json:"csv"`
	CSVRows    int            `json:"csvRows"`
	CSVMatched int            `json:"csvMatched"`
	Unmatched  []string       `json:"unmatched"` // cards.csv rows whose file isn't in the folder
	Skipped    []string       `json:"skipped"`   // files that aren't images
	Logo       bool           `json:"logo"`
	Rotated    int            `json:"rotated"`  // landscape images turned upright
	OffShape   int            `json:"offShape"` // images far from a card's shape (kept as they are)
	Warnings   []string       `json:"warnings"`
	Samples    []FolderSample `json:"samples"` // the first cards, to show how file names are read
}

type FolderSample struct {
	File   string `json:"file"`
	Number string `json:"number"`
	Name   string `json:"name"`
}

type RarityCount struct {
	Name  string `json:"name"`
	Game  string `json:"game"`
	Cards int    `json:"cards"`
}

var (
	folderImageExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".avif": true}
	folderIgnored  = map[string]bool{"thumbs.db": true, "desktop.ini": true, ".ds_store": true}
	numberedName   = regexp.MustCompile(`^#?(\d{1,5}[a-zA-Z]?)(?:\s*[-_.)]\s*|\s+)(.+)$`)
	onlyNumber     = regexp.MustCompile(`^#?(\d{1,5}[a-zA-Z]?)$`)
	rankPrefix     = regexp.MustCompile(`^\d+\s*[-_.)]?\s*`)
)

// folderScan is one read of an image folder.
type folderScan struct {
	preview FolderPreview
	cards   []cardIn
	order   []string // rarities, lowest first
	logo    string
	rarity  func(string) string
}

// ScanFolder reads an image folder for the Import page's preview.
func ScanFolder(dir string, opt Options) (FolderPreview, error) {
	s, err := scanFolder(dir, opt)
	if err != nil {
		return FolderPreview{}, err
	}
	return s.preview, nil
}

func scanFolder(dir string, opt Options) (*folderScan, error) {
	dir = filepath.Clean(dir)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("%s is not a folder", dir)
	}
	s := &folderScan{preview: FolderPreview{Dir: dir, Name: filepath.Base(dir), ProjectID: folderSource{}.ProjectID(dir, "")}}
	p := &s.preview
	if opt.SetName != "" {
		p.Name = opt.SetName
	}
	rows, err := readCardsCSV(dir)
	if err != nil {
		return nil, err
	}
	p.CSV, p.CSVRows = rows != nil, len(rows)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return naturalLess(strings.ToLower(entries[i].Name()), strings.ToLower(entries[j].Name()))
	})
	type file struct{ path, rel, folderRarity string }
	var files []file
	var folders []string
	for _, e := range entries {
		name := e.Name()
		low := strings.ToLower(name)
		switch {
		case e.IsDir():
			rar := strings.TrimSpace(rankPrefix.ReplaceAllString(name, ""))
			if rar == "" {
				rar = name
			}
			n := 0
			_ = filepath.WalkDir(filepath.Join(dir, name), func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				if folderImageExt[strings.ToLower(filepath.Ext(path))] {
					rel, _ := filepath.Rel(dir, path)
					files = append(files, file{path, rel, rar})
					n++
				} else if !folderIgnored[strings.ToLower(d.Name())] {
					p.Skipped = append(p.Skipped, filepath.Join(name, d.Name()))
				}
				return nil
			})
			if n > 0 && !containsFold(folders, rar) {
				folders = append(folders, rar)
			}
		case strings.TrimSuffix(low, filepath.Ext(low)) == "logo" || strings.TrimSuffix(low, filepath.Ext(low)) == "set":
			if folderImageExt[filepath.Ext(low)] && s.logo == "" {
				s.logo, p.Logo = filepath.Join(dir, name), true
			}
		case low == "cards.csv" || folderIgnored[low]:
		case folderImageExt[filepath.Ext(low)]:
			files = append(files, file{filepath.Join(dir, name), name, ""})
		default:
			p.Skipped = append(p.Skipped, name)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no card images (PNG, JPG, WebP or AVIF) found in %s", dir)
	}

	// Card order = file names in natural order across all subfolders ("2 X" before "10 X").
	stemOf := func(rel string) string { return strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel)) }
	sort.SliceStable(files, func(i, j int) bool {
		return naturalLess(strings.ToLower(stemOf(files[i].rel)), strings.ToLower(stemOf(files[j].rel)))
	})

	used := map[int]bool{} // csv rows matched
	maxNum, width := 0, 1  // highest number and its digits ("001" → 3), so added numbers sort and look the same
	var unnumbered []int
	for _, f := range files {
		stem := stemOf(f.rel)
		// The id comes from the file name, so adding, removing or renaming other images never moves a card's id
		// (player saves keep the right cards); repeats get -2, -3 in buildProject.
		id := slug(stem)
		if id == "" {
			id = "card"
		}
		c := cardIn{SourceID: strings.ToLower(filepath.ToSlash(f.rel)), ID: id, Image: f.path, SrcRarity: f.folderRarity,
			Name: cleanName(stem)}
		// Strip leading numbers (opt-in): "001 Captain Marvel" = card number 001 named "Captain Marvel". Off, the whole
		// file name is the name ("2099 Spider-Man", "3-D Man").
		if opt.StripNumbers {
			if m := numberedName.FindStringSubmatch(stem); m != nil {
				c.Number, c.Name = m[1], cleanName(m[2])
			} else if m := onlyNumber.FindStringSubmatch(stem); m != nil {
				c.Number = m[1]
			}
		}
		if i, ok := matchRow(rows, f.rel); ok {
			used[i] = true
			r := rows[i]
			set := func(dst *string, key string) {
				if v := strings.TrimSpace(r[key]); v != "" {
					*dst = v
				}
			}
			set(&c.Name, "name")
			set(&c.Number, "number")
			set(&c.SrcRarity, "rarity")
			set(&c.Artist, "artist")
			set(&c.Text, "text")
			set(&c.TypeLine, "type")
			c.USD, c.USDFoil = money(r["price"]), money(r["foilprice"])
		}
		if n, err := strconv.Atoi(c.Number); err == nil {
			maxNum, width = max(maxNum, n), max(width, len(c.Number))
		}
		if c.Number == "" {
			unnumbered = append(unnumbered, len(s.cards))
		}
		if fit, rotated, ok := folderShape(f.path); ok {
			c.Aspect = fit
			if rotated {
				p.Rotated++
			}
			if fit == 0 {
				p.OffShape++
			}
		}
		s.cards = append(s.cards, c)
	}
	// Cards without a number (from the file name or cards.csv) are numbered after the numbered ones, in file-name order.
	for k, i := range unnumbered {
		s.cards[i].Number = fmt.Sprintf("%0*d", width, maxNum+1+k)
	}
	sort.SliceStable(s.cards, func(i, j int) bool { return naturalLess(s.cards[i].Number, s.cards[j].Number) })
	for _, c := range s.cards[:min(5, len(s.cards))] {
		p.Samples = append(p.Samples, FolderSample{File: filepath.Base(c.Image), Number: c.Number, Name: c.Name})
	}

	for i, r := range rows {
		if !used[i] {
			p.Unmatched = append(p.Unmatched, r["file"])
		}
	}
	p.CSVMatched = len(used)

	// Rarity order: the subfolders as they sort in the folder, then rarities only cards.csv names (by known rank).
	s.order = folders
	var extra []string
	for _, c := range s.cards {
		if c.SrcRarity != "" && !containsFold(s.order, c.SrcRarity) && !containsFold(extra, c.SrcRarity) {
			extra = append(extra, c.SrcRarity)
		}
	}
	sort.SliceStable(extra, func(i, j int) bool { return knownRank(extra[i]) < knownRank(extra[j]) })
	s.order = append(s.order, extra...)
	s.rarity = folderRarityFunc(s.order, opt.RarityMap)

	counts := map[string]int{}
	for _, c := range s.cards {
		counts[c.SrcRarity]++
	}
	if n := counts[""]; n > 0 {
		p.Rarities = append(p.Rarities, RarityCount{Name: "(no rarity)", Game: s.rarity(""), Cards: n})
	}
	for _, r := range s.order {
		if n := counts[r]; n > 0 {
			p.Rarities = append(p.Rarities, RarityCount{Name: r, Game: s.rarity(r), Cards: n})
		}
	}
	p.Cards = len(s.cards)
	if len(p.Skipped) > 0 {
		p.Warnings = append(p.Warnings, fmt.Sprintf("%d file(s) skipped (not PNG/JPG/WebP/AVIF)", len(p.Skipped)))
	}
	if p.OffShape > 0 {
		p.Warnings = append(p.Warnings, fmt.Sprintf("%d image(s) are not card-shaped and are kept as they are — crop them to 63×88 (5:7) for a full card", p.OffShape))
	}
	if len(p.Unmatched) > 0 {
		p.Warnings = append(p.Warnings, fmt.Sprintf("%d cards.csv row(s) name a file that isn't in the folder", len(p.Unmatched)))
	}
	return s, nil
}

// folderRarityFunc maps a rarity to a game rarity: the options' map, then known words, then the rarity's place in the
// set's own order spread over Common…Legendary. No rarity = Common.
func folderRarityFunc(order []string, m map[string]string) func(string) string {
	ladder := []string{"Common", "Rare", "Epic", "Legendary"}
	return func(r string) string {
		if r == "" {
			return "Common"
		}
		for k, g := range m {
			if strings.EqualFold(k, r) {
				return g
			}
		}
		if g := keywordRarity(r); g != "" {
			return g
		}
		for i, o := range order {
			if strings.EqualFold(o, r) {
				if len(order) == 1 {
					return "Common"
				}
				return ladder[int(math.Round(float64(i)*3/float64(len(order)-1)))]
			}
		}
		return "Common"
	}
}

func knownRank(r string) int {
	for i, k := range FolderRarityOrder {
		if strings.EqualFold(k, r) {
			return i
		}
	}
	switch keywordRarity(r) {
	case "Common":
		return 1
	case "Rare":
		return 3
	case "Epic":
		return 6
	case "Legendary":
		return 9
	}
	return 5
}

func containsFold(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}

// cleanName turns a file-name stem into a card name: underscores → spaces (hyphens stay: Spider-Man).
func cleanName(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "_", " ")), " ")
}

// folderShape reads an image's size: fit = CardAspect when the (upright) image is within 12 % of a card's shape (it is
// stretched to fit exactly), 0 when it is kept as is; rotated = landscape (turned upright on import).
func folderShape(path string) (fit float64, rotated, ok bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false, false
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width == 0 || cfg.Height == 0 {
		return 0, false, false
	}
	w, h := float64(cfg.Width), float64(cfg.Height)
	if w > h {
		w, h, rotated = h, w, true
	}
	if math.Abs(w/h-CardAspect)/CardAspect <= 0.12 {
		fit = CardAspect
	}
	return fit, rotated, true
}

// money reads a price like "1.50", "$1.50", "1,50 €"; nil when empty or not positive.
func money(s string) *float64 {
	s = strings.TrimSpace(strings.NewReplacer("$", "", "€", "", "£", "", " ", "").Replace(s))
	if strings.Contains(s, ",") && !strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ",", ".")
	}
	return price(s)
}

// readCardsCSV reads <dir>\cards.csv (nil when there is none): one map per row, keys = lower-case header names without
// spaces ("foilPrice" → "foilprice"), with a few aliases. Comma or semicolon separated (Excel in many locales).
func readCardsCSV(dir string) ([]map[string]string, error) {
	b, err := os.ReadFile(filepath.Join(dir, "cards.csv"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	r := csv.NewReader(bytes.NewReader(b))
	r.FieldsPerRecord, r.LazyQuotes = -1, true
	if first, _, _ := bytes.Cut(b, []byte("\n")); bytes.Count(first, []byte(";")) > bytes.Count(first, []byte(",")) {
		r.Comma = ';'
	}
	recs, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("cards.csv: %w", err)
	}
	if len(recs) == 0 {
		return []map[string]string{}, nil
	}
	alias := map[string]string{"image": "file", "filename": "file", "foil": "foilprice", "description": "text",
		"no": "number", "#": "number", "cardnumber": "number", "usd": "price"}
	var keys []string
	for _, h := range recs[0] {
		k := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(h), " ", ""))
		if a, ok := alias[k]; ok {
			k = a
		}
		keys = append(keys, k)
	}
	if !containsFold(keys, "file") {
		return nil, fmt.Errorf("cards.csv needs a \"file\" column (the image's file name)")
	}
	rows := []map[string]string{}
	for _, rec := range recs[1:] {
		row := map[string]string{}
		for i, v := range rec {
			if i < len(keys) {
				row[keys[i]] = v
			}
		}
		if strings.TrimSpace(row["file"]) != "" {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// matchRow finds the cards.csv row for an image by its path in the folder, or its file name with or without extension.
func matchRow(rows []map[string]string, rel string) (int, bool) {
	norm := func(s string) string { return strings.ToLower(filepath.ToSlash(strings.TrimSpace(s))) }
	relN := norm(rel)
	base := norm(filepath.Base(rel))
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	best := -1
	for i, r := range rows {
		f := norm(r["file"])
		switch {
		case f == relN:
			return i, true
		case best < 0 && (f == base || f == stem):
			best = i
		}
	}
	return best, best >= 0
}

func (s folderSource) Import(ctx context.Context, ws project.Workspace, dir string, opt Options, report func(Progress)) (*project.Project, error) {
	report(Progress{Stage: "cards", Message: "Reading the folder…"})
	if opt.RarityMap == nil {
		opt.RarityMap = DefaultFolderRarityMap()
	}
	sc, err := scanFolder(dir, opt)
	if err != nil {
		return nil, err
	}
	return buildProject(ctx, ws, setIn{ID: sc.preview.ProjectID, Source: "folder", Code: slug(filepath.Base(sc.preview.Dir)),
		Name: sc.preview.Name, Logo: sc.logo, Cards: sc.cards, Rotate: true, Rarity: sc.rarity,
		Get:   func(_ context.Context, path string) ([]byte, error) { return os.ReadFile(path) },
		Local: true, SourceDir: sc.preview.Dir, RarityOrder: sc.order,
		Slots: []setfmt.Slot{
			{Count: 4, Weights: map[string]float64{"Common": 1}},
			{Count: 2, Weights: map[string]float64{"Rare": 1}},
			{Count: 1, Weights: map[string]float64{"Epic": 4, "Legendary": 1}},
		}, FoilChance: 5}, opt, report)
}

// RefreshMeta re-reads the prices in the folder's cards.csv (cards are matched by their path in the folder).
func (folderSource) RefreshMeta(_ context.Context, p *project.Project) (int, error) {
	dir := p.Meta.SourceDir
	if dir == "" {
		return 0, fmt.Errorf("%s was not imported from an image folder", p.ID)
	}
	if _, err := os.Stat(dir); err != nil {
		return 0, fmt.Errorf("the image folder %s is gone — prices come from its cards.csv", dir)
	}
	sc, err := scanFolder(dir, Options{})
	if err != nil {
		return 0, err
	}
	if !sc.preview.CSV {
		return 0, fmt.Errorf("%s has no cards.csv — add a price column there to set real prices", dir)
	}
	return refreshFrom(p, sc.cards), nil
}

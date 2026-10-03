package importer

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"tcgstudio/internal/webapi"
)

// ---------------------------------------------------------------- TCGCSV (TCGplayer catalog mirror)
// https://tcgcsv.com — free daily mirror of TCGplayer's catalog for every game TCGplayer sells: groups (sets, with
// release dates), products (cards + sealed, with "extendedData" fields like Number/Rarity/Color) and prices per
// printing (Normal/Foil…). Card images are TCGplayer's own scans (no "SAMPLE" watermark, unlike some publishers' lists).

const tcgcsvAPI = "https://tcgcsv.com/tcgplayer/"

type tcgcsv struct{ c *webapi.Client }

func newTCGCSV() *tcgcsv { return &tcgcsv{webapi.New("tcgcsv", 100*time.Millisecond)} }

type tcgGroup struct {
	ID           int    `json:"groupId"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	Supplemental bool   `json:"isSupplemental"`
	PublishedOn  string `json:"publishedOn"`
}

type tcgProduct struct {
	ID       int    `json:"productId"`
	Name     string `json:"name"`
	ImageURL string `json:"imageUrl"`
	Images   int    `json:"imageCount"` // 0 = no scan yet (unreleased sets): imageUrl then points at nothing
	Extended []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"extendedData"`
}

type tcgPrice struct {
	ProductID int      `json:"productId"`
	Market    *float64 `json:"marketPrice"`
	Mid       *float64 `json:"midPrice"`
	Low       *float64 `json:"lowPrice"`
	SubType   string   `json:"subTypeName"` // Normal, Foil, Holofoil, …
}

func (t *tcgcsv) groups(ctx context.Context, category int) ([]tcgGroup, error) {
	var res struct {
		Results []tcgGroup `json:"results"`
	}
	return res.Results, t.c.GetJSON(ctx, fmt.Sprintf("%s%d/groups", tcgcsvAPI, category), &res)
}

func (t *tcgcsv) products(ctx context.Context, category, group int) ([]tcgProduct, error) {
	var res struct {
		Results []tcgProduct `json:"results"`
	}
	return res.Results, t.c.GetJSON(ctx, fmt.Sprintf("%s%d/%d/products", tcgcsvAPI, category, group), &res)
}

// prices returns each product's market price per printing ("Normal", "Foil", …), mid then low price as fallback.
func (t *tcgcsv) prices(ctx context.Context, category, group int) (map[int]map[string]float64, error) {
	var res struct {
		Results []tcgPrice `json:"results"`
	}
	if err := t.c.GetJSON(ctx, fmt.Sprintf("%s%d/%d/prices", tcgcsvAPI, category, group), &res); err != nil {
		return nil, err
	}
	out := map[int]map[string]float64{}
	for _, p := range res.Results {
		for _, v := range []*float64{p.Market, p.Mid, p.Low} {
			if v != nil && *v > 0 {
				if out[p.ProductID] == nil {
					out[p.ProductID] = map[string]float64{}
				}
				out[p.ProductID][p.SubType] = *v
				break
			}
		}
	}
	return out, nil
}

func (p *tcgProduct) ext(name string) string {
	for _, e := range p.Extended {
		if e.Name == name {
			return e.Value
		}
	}
	return ""
}

// image returns the large scan (the listed one is a 200 px thumbnail).
func (p *tcgProduct) image() string {
	if p.ImageURL == "" || p.Images == 0 {
		return ""
	}
	return strings.Replace(p.ImageURL, "_200w.", "_in_1000x1000.", 1)
}

var (
	tcgTags   = regexp.MustCompile(`<[^>]+>`)
	tcgParens = regexp.MustCompile(`\s*\(([^()]*)\)\s*$`)
)

// tcgText turns TCGplayer's HTML rules text into plain lines.
func tcgText(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = regexp.MustCompile(`(?i)<br\s*/?>`).ReplaceAllString(s, "\n")
	s = regexp.MustCompile(`(?is)<a [^>]*>.*?</a>`).ReplaceAllString(s, "") // errata links
	s = html.UnescapeString(tcgTags.ReplaceAllString(s, ""))
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, "\n")
}

// tcgName splits "Roronoa Zoro (001) (Parallel)" into "Roronoa Zoro" and the printing tags ["Parallel"]; a pure-number
// tag only tells same-named cards apart (the card number is shown anyway) and is dropped.
func tcgName(name string) (string, []string) {
	var tags []string
	for {
		m := tcgParens.FindStringSubmatchIndex(name)
		if m == nil {
			break
		}
		tag := name[m[2]:m[3]]
		name = name[:m[0]]
		if _, err := strconv.Atoi(tag); err != nil && tag != "" {
			tags = append([]string{tag}, tags...)
		}
	}
	return strings.TrimSpace(name), tags
}

// tcgCode is a group's import code: its abbreviation when that is set and unique in the category, else the TCGplayer
// group id (Flesh and Blood repeats "GEM" for six packs and leaves 45 groups without one).
func tcgCode(g tcgGroup, all []tcgGroup) string {
	if g.Abbreviation != "" {
		n := 0
		for _, o := range all {
			if o.Abbreviation == g.Abbreviation {
				n++
			}
		}
		if n == 1 {
			return g.Abbreviation
		}
	}
	return strconv.Itoa(g.ID)
}

// tcgFind resolves an import code to its group.
func (t *tcgcsv) tcgFind(ctx context.Context, category int, code, game string) (*tcgGroup, []tcgGroup, error) {
	gs, err := t.groups(ctx, category)
	if err != nil {
		return nil, nil, err
	}
	for i := range gs {
		if tcgCode(gs[i], gs) == code {
			return &gs[i], gs, nil
		}
	}
	return nil, gs, fmt.Errorf("%s set %s not found on TCGplayer", game, code)
}

// tcgPrices picks a card's base and foil price from TCGplayer's printings: base = the first of normal in order,
// else any non-foil printing, else a foil one (foil-only cards); foil = the first of foil in order, else any foil.
func tcgPrices(pr map[string]float64, normal, foil []string) (usd, usdFoil *float64) {
	isFoil := func(k string) bool { return strings.Contains(strings.ToLower(k), "foil") }
	pick := func(keys []string, want bool) *float64 {
		for _, k := range keys {
			if v, ok := pr[k]; ok {
				return pricef(v)
			}
		}
		var ks []string
		for k := range pr {
			if isFoil(k) == want {
				ks = append(ks, k)
			}
		}
		sort.Strings(ks)
		if len(ks) > 0 {
			return pricef(pr[ks[0]])
		}
		return nil
	}
	usd, usdFoil = pick(normal, false), pick(foil, true)
	if usd == nil {
		usd, usdFoil = usdFoil, nil
	}
	return usd, usdFoil
}

// tcgSortGroups orders groups newest first (undated last).
func tcgSortGroups(gs []tcgGroup) {
	sort.SliceStable(gs, func(i, j int) bool { return gs[i].PublishedOn > gs[j].PublishedOn })
}

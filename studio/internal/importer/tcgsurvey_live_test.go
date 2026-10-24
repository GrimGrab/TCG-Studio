package importer

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestTCGSurvey writes a Markdown report of TCGplayer categories (fields, rarities, name tags, images, prices) used to
// write the game table in tcgplayer.go. TCG_SURVEY=<report.md> [TCG_SURVEY_CATS=86,63] go test -run TCGSurvey -v
func TestTCGSurvey(t *testing.T) {
	out := os.Getenv("TCG_SURVEY")
	if out == "" {
		t.Skip("TCG_SURVEY not set")
	}
	var cats []int
	for _, s := range strings.Split(os.Getenv("TCG_SURVEY_CATS"), ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			cats = append(cats, n)
		}
	}
	if len(cats) == 0 {
		for _, g := range tcgGames {
			cats = append(cats, g.Category)
		}
	}
	ctx := context.Background()
	tc := newTCGCSV()
	var b strings.Builder
	for _, cat := range cats {
		surveyCategory(ctx, t, tc, cat, &b)
		_ = os.WriteFile(out, []byte(b.String()), 0o644)
	}
}

var surveyNumberShape = regexp.MustCompile(`[0-9]+`)

func surveyCategory(ctx context.Context, t *testing.T, tc *tcgcsv, cat int, b *strings.Builder) {
	gs, err := tc.groups(ctx, cat)
	if err != nil {
		fmt.Fprintf(b, "## Category %d\nERROR groups: %v\n\n", cat, err)
		return
	}
	tcgSortGroups(gs)
	fmt.Fprintf(b, "## Category %d — %d groups\n\n", cat, len(gs))
	for _, g := range gs {
		fmt.Fprintf(b, "- %d `%s` %s (%s)%s\n", g.ID, g.Abbreviation, g.Name, date10(g.PublishedOn),
			map[bool]string{true: " supplemental", false: ""}[g.Supplemental])
	}
	// Sample: newest 3, oldest 2, spread over the list.
	pick := map[int]bool{}
	for _, i := range []int{0, 1, 2, len(gs) / 2, len(gs) - 2, len(gs) - 1} {
		if i >= 0 && i < len(gs) {
			pick[i] = true
		}
	}
	fields := map[string]int{}
	fieldEx := map[string]string{}
	rarities := map[string]int{}
	tags := map[string]int{}
	shapes := map[string]string{}
	var cards, sealed, noImage, priced int
	subTypes := map[string]int{}
	var idx []int
	for i := range pick {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	for _, i := range idx {
		g := gs[i]
		ps, err := tc.products(ctx, cat, g.ID)
		if err != nil {
			fmt.Fprintf(b, "\nERROR products %d: %v\n", g.ID, err)
			continue
		}
		pr, _ := tc.prices(ctx, cat, g.ID)
		for _, p := range pr {
			for k := range p {
				subTypes[k]++
			}
		}
		for k := range ps {
			p := &ps[k]
			if len(p.Extended) == 0 || (p.ext("Rarity") == "" && p.ext("Number") == "") {
				sealed++
				continue
			}
			cards++
			if p.Images == 0 {
				noImage++
			}
			if len(pr[p.ID]) > 0 {
				priced++
			}
			for _, e := range p.Extended {
				fields[e.Name]++
				if _, ok := fieldEx[e.Name]; !ok || len(fieldEx[e.Name]) < 3 {
					v := tcgText(e.Value)
					if len(v) > 70 {
						v = v[:70] + "…"
					}
					fieldEx[e.Name] = strings.ReplaceAll(v, "\n", " / ")
				}
			}
			rarities[p.ext("Rarity")]++
			_, tg := tcgName(p.Name)
			for _, x := range tg {
				tags[x]++
			}
			if n := p.ext("Number"); n != "" {
				shapes[surveyNumberShape.ReplaceAllString(n, "9")] = n
			}
		}
	}
	fmt.Fprintf(b, "\nSampled %d groups: %d cards, %d sealed, %d without image, %d priced\n\n", len(idx), cards, sealed, noImage, priced)
	b.WriteString("Fields:\n")
	for _, k := range sortedKeys(fields) {
		fmt.Fprintf(b, "- %s (%d): %s\n", k, fields[k], fieldEx[k])
	}
	b.WriteString("\nRarities: ")
	for _, k := range sortedKeys(rarities) {
		fmt.Fprintf(b, "%q %d; ", k, rarities[k])
	}
	b.WriteString("\n\nName tags: ")
	n := 0
	for _, k := range sortedKeys(tags) {
		if n++; n > 40 {
			b.WriteString("…")
			break
		}
		fmt.Fprintf(b, "%q %d; ", k, tags[k])
	}
	b.WriteString("\n\nNumber shapes: ")
	n = 0
	for k, v := range shapes {
		if n++; n > 15 {
			break
		}
		fmt.Fprintf(b, "`%s` (%s); ", k, v)
	}
	b.WriteString("\n\nPrice sub-types: ")
	for _, k := range sortedKeys(subTypes) {
		fmt.Fprintf(b, "%q %d; ", k, subTypes[k])
	}
	b.WriteString("\n\n")
	t.Logf("category %d: %d groups, %d cards sampled", cat, len(gs), cards)
}

func sortedKeys(m map[string]int) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool {
		if m[ks[i]] != m[ks[j]] {
			return m[ks[i]] > m[ks[j]]
		}
		return ks[i] < ks[j]
	})
	return ks
}

// TestTCGValues lists every value of the given fields (and every rarity) over the 12 newest booster sets of each game:
// TCG_VALUES=<report.md> TCG_VALUES_SPEC="86:Color,63:Color;DigimonForm" go test -run TCGValues -v
func TestTCGValues(t *testing.T) {
	out := os.Getenv("TCG_VALUES")
	if out == "" {
		t.Skip("TCG_VALUES not set")
	}
	ctx := context.Background()
	tc := newTCGCSV()
	var b strings.Builder
	for _, item := range strings.Split(os.Getenv("TCG_VALUES_SPEC"), ",") {
		c, f, _ := strings.Cut(item, ":")
		cat, _ := strconv.Atoi(c)
		fields := strings.Split(f, ";")
		gs, err := tc.groups(ctx, cat)
		if err != nil {
			fmt.Fprintf(&b, "## %d ERROR %v\n", cat, err)
			continue
		}
		tcgSortGroups(gs)
		vals := map[string]map[string]int{"Rarity": {}}
		for _, f := range fields {
			vals[f] = map[string]int{}
		}
		n := 0
		for _, g := range gs {
			if _, main := tcgGroupKind(g); !main || n >= 12 {
				continue
			}
			n++
			ps, err := tc.products(ctx, cat, g.ID)
			if err != nil {
				continue
			}
			for i := range ps {
				p := &ps[i]
				if p.ext("Number") == "" {
					continue
				}
				for k, m := range vals {
					if v := p.ext(k); v != "" {
						for _, x := range strings.Split(v, ";") {
							m[strings.TrimSpace(x)]++
						}
					}
				}
			}
		}
		fmt.Fprintf(&b, "## %d (%d booster groups)\n", cat, n)
		for _, k := range append([]string{"Rarity"}, fields...) {
			fmt.Fprintf(&b, "- %s: ", k)
			for _, v := range sortedKeys(vals[k]) {
				fmt.Fprintf(&b, "%q %d; ", v, vals[k][v])
			}
			b.WriteString("\n")
		}
		_ = os.WriteFile(out, []byte(b.String()), 0o644)
	}
}

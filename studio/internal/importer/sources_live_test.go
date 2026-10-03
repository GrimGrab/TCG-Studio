package importer

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"tcgstudio/internal/project"
	"tcgstudio/internal/scryfall"
	"tcgstudio/internal/tcgdex"
)

// TestSourcesLive imports real sets over the network:
// IMPORT_LIVE="ygoprodeck:LOB,optcg:OP-01,swudb:SOR,lorcast:1" go test ./internal/importer -run SourcesLive -v -timeout 30m
func TestSourcesLive(t *testing.T) {
	spec := os.Getenv("IMPORT_LIVE")
	if spec == "" {
		t.Skip("set IMPORT_LIVE=<source>:<set>[,…] to run")
	}
	reg := NewRegistry(scryfall.New(), tcgdex.New())
	ctx := context.Background()
	for _, item := range strings.Split(spec, ",") {
		id, code, _ := strings.Cut(item, ":")
		t.Run(item, func(t *testing.T) {
			src, err := reg.Get(id)
			if err != nil {
				t.Fatal(err)
			}
			sets, err := src.Sets(ctx, "")
			if err != nil {
				t.Fatal(err)
			}
			main := 0
			for _, s := range sets {
				if s.Main {
					main++
				}
			}
			t.Logf("%d sets (%d main); newest: %+v", len(sets), main, sets[:min(2, len(sets))])
			ws := project.Workspace{Root: t.TempDir()}
			p, err := src.Import(ctx, ws, code, src.DefaultOptions(), func(pr Progress) {
				if pr.Stage == "done" {
					t.Log(pr.Message)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			info := src.Info()
			counts, srcCounts, unknown := map[string]int{}, map[string]int{}, map[string]bool{}
			for _, c := range p.Set.Cards {
				m := p.Meta.Cards[c.ID]
				counts[c.Rarity]++
				srcCounts[m.SrcRarity]++
				if !slices.Contains(info.RarityOrder, m.SrcRarity) {
					unknown[m.SrcRarity] = true
				}
			}
			t.Logf("%s: %d cards, game %v, source %v", p.ID, len(p.Set.Cards), counts, srcCounts)
			if len(unknown) > 0 {
				t.Errorf("source rarities missing from RarityOrder: %v", unknown)
			}
			priced := 0
			for _, c := range p.Set.Cards {
				if m := p.Meta.Cards[c.ID]; m.USD != nil || m.EUR != nil {
					priced++
				}
			}
			t.Logf("%d/%d cards have a real price", priced, len(p.Set.Cards))
			for _, c := range p.Set.Cards[:min(2, len(p.Set.Cards))] {
				m := p.Meta.Cards[c.ID]
				t.Logf("  %s %q %s/%s type=%q colors=%v cost=%v power=%q price=%v", c.ID, c.Name, c.Rarity, m.SrcRarity, m.TypeLine, m.Colors, m.CMC, m.Power, c.Price.Base)
			}
			// Every image is a portrait PNG.
			for _, c := range p.Set.Cards {
				f, err := os.Open(filepath.Join(p.Folder, c.Image))
				if err != nil {
					t.Errorf("%s: %v", c.ID, err)
					continue
				}
				cfg, format, err := image.DecodeConfig(f)
				f.Close()
				ratio := float64(cfg.Width) / float64(cfg.Height)
				if err != nil || format != "png" || ratio < 0.70 || ratio > 0.74 { // the game's slot is 63×88 ≈ 0.716
					t.Errorf("%s: format %s %dx%d %v", c.ID, format, cfg.Width, cfg.Height, err)
				}
			}
			for _, i := range p.Set.Validate(p.Folder) {
				if i.Level == "error" {
					t.Errorf("validation: %+v", i)
				}
			}
			if n, err := src.RefreshMeta(ctx, p); err != nil || n == 0 {
				t.Errorf("refresh: %d %v", n, err)
			}
		})
	}
}

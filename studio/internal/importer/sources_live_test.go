package importer

import (
	"context"
	"image"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"tcgstudio/internal/project"
	"tcgstudio/internal/scryfall"
	"tcgstudio/internal/tcgdex"
)

// TestSourcesLive imports real sets over the network:
// IMPORT_LIVE="ygoprodeck:LOB,optcg:OP-01,swudb:SOR,lorcast:1" go test ./internal/importer -run SourcesLive -v -timeout 30m
// A source without ":<set>" imports its newest released main set; "tcg-all" = every TCGplayer game of tcgplayer.go.
func TestSourcesLive(t *testing.T) {
	spec := os.Getenv("IMPORT_LIVE")
	if spec == "" {
		t.Skip("set IMPORT_LIVE=<source>[:<set>][,…] to run")
	}
	reg := NewRegistry(scryfall.New(), tcgdex.New())
	ctx := context.Background()
	var items []string
	for _, item := range strings.Split(spec, ",") {
		if item != "tcg-all" {
			items = append(items, item)
			continue
		}
		for _, g := range tcgGames {
			items = append(items, g.ID)
		}
	}
	today := time.Now().Format("2006-01-02")
	for _, item := range items {
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
			// No set given: the newest released sets, newest first (a set TCGplayer lists without card data yet is skipped).
			codes := []string{code}
			if code == "" {
				codes = nil
				for _, s := range sets {
					if s.ReleasedAt != "" && s.ReleasedAt <= today && (s.Main || main == 0) && len(codes) < 6 {
						codes = append(codes, s.Code)
					}
				}
			}
			var p *project.Project
			for _, c := range codes {
				t.Logf("importing %s", c)
				ws := project.Workspace{Root: t.TempDir(), Library: project.LibraryDir(t.TempDir())} // art never lands in the package folder
				p, err = src.Import(ctx, ws, c, src.DefaultOptions(), func(pr Progress) {
					if pr.Stage == "done" || pr.Stage == "cards" && strings.Contains(pr.Message, "left out") {
						t.Log(pr.Message)
					}
				})
				if err == nil || code != "" || !strings.Contains(err.Error(), "no card") {
					break
				}
				t.Log(err)
			}
			if err != nil || p == nil {
				t.Fatal("no importable set: ", err)
			}
			info := src.Info()
			counts, srcCounts, unknown := map[string]int{}, map[string]int{}, map[string]bool{}
			for _, c := range p.Set.Cards {
				m := p.Meta.Cards[c.ID]
				counts[c.Rarity]++
				srcCounts[m.SrcRarity]++
				if !slices.Contains(info.RarityOrder, m.SrcRarity) && !slices.Contains(p.Meta.RarityOrder, m.SrcRarity) {
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
				f, err := os.Open(p.ImagePath(c.Image))
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
			for _, i := range p.Set.Validate(p.LibFolder) {
				if i.Level == "error" {
					t.Errorf("validation: %+v", i)
				}
			}
			if n, err := src.RefreshMeta(ctx, p); err != nil || n == 0 && priced > 0 {
				t.Errorf("refresh: %d %v", n, err)
			}
		})
	}
}

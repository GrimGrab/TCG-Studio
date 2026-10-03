package importer

import (
	"context"
	"os"
	"testing"

	"tcgstudio/internal/project"
	"tcgstudio/internal/tcgdex"
)

// TestTCGdexLive imports a real set over the network: TCGDEX_LIVE=<set id> go test ./internal/importer -run TCGdexLive -v
func TestTCGdexLive(t *testing.T) {
	code := os.Getenv("TCGDEX_LIVE")
	if code == "" {
		t.Skip("set TCGDEX_LIVE=<set id> to run")
	}
	src := &tcgdexSource{tcgdex.New()}
	ctx := context.Background()
	sets, err := src.Sets(ctx, "en")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d sets; newest: %+v", len(sets), sets[:3])
	ws := project.Workspace{Root: t.TempDir()}
	last := -1
	p, err := src.Import(ctx, ws, code, src.DefaultOptions(), func(pr Progress) {
		if pr.Stage == "done" || pr.Done*10/max(pr.Total, 1) != last {
			last = pr.Done * 10 / max(pr.Total, 1)
			t.Logf("%s %d/%d %s", pr.Stage, pr.Done, pr.Total, pr.Message)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, c := range p.Set.Cards {
		counts[c.Rarity]++
	}
	t.Logf("%s: %d cards, rarities %v", p.ID, len(p.Set.Cards), counts)
	for _, c := range p.Set.Cards[:min(3, len(p.Set.Cards))] {
		t.Logf("%s %s %s price %+v meta %+v", c.ID, c.Name, c.Rarity, c.Price, p.Meta.Cards[c.ID])
	}
	for _, i := range p.Set.Validate(p.Folder) {
		if i.Level == "error" {
			t.Errorf("validation: %+v", i)
		}
	}
	n, err := src.RefreshMeta(ctx, p)
	if err != nil || n == 0 {
		t.Fatalf("refresh: %d %v", n, err)
	}
}

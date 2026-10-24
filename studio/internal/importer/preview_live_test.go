package importer

import (
	"context"
	"os"
	"strings"
	"testing"

	"tcgstudio/internal/scryfall"
	"tcgstudio/internal/tcgdex"
)

// TestPreviewLive previews real sets: PREVIEW_LIVE="scryfall:dmu,tcgdex:sv03,tcg-gundam:GD01" go test -run PreviewLive -v
func TestPreviewLive(t *testing.T) {
	spec := os.Getenv("PREVIEW_LIVE")
	if spec == "" {
		t.Skip("set PREVIEW_LIVE=<source>:<set>[,…] to run")
	}
	reg := NewRegistry(scryfall.New(), tcgdex.New())
	for _, item := range strings.Split(spec, ",") {
		id, code, _ := strings.Cut(item, ":")
		t.Run(item, func(t *testing.T) {
			src, err := reg.Get(id)
			if err != nil {
				t.Fatal(err)
			}
			opt := src.DefaultOptions()
			if len(src.Info().Languages) > 0 {
				opt.Lang = "en"
			}
			p, err := Preview(context.Background(), src, code, opt)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s: %d cards (%d variants), %d priced, $%.0f, %d rarities, pack %d, top %d, notes %v",
				p.Name, p.Cards, p.Variants, p.Priced, p.Value, len(p.Rarities), p.Pack.Cards, len(p.Top), p.Notes)
			if p.Cards == 0 || p.Pack.Cards == 0 || len(p.Rarities) == 0 || p.All[0].Thumb == "" {
				t.Errorf("empty preview: %+v", p.All[:min(1, len(p.All))])
			}
			t.Logf("thumb %s", p.All[0].Thumb)
		})
	}
}

package importer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/project"
)

// EPL_LIVE="<mod folder or .zip>" [EPL_SET=<name part>] [EPL_KEEP=<dir>] go test ./internal/importer -run EPLLive -v -timeout 30m
// Previews a real Enhanced Prefab Loader mod and imports its card sets into a temporary workspace (or EPL_KEEP).
func TestEPLLive(t *testing.T) {
	path := os.Getenv("EPL_LIVE")
	if path == "" {
		t.Skip("EPL_LIVE not set")
	}
	start := time.Now()
	pv, err := ScanEPL(path, os.Getenv("EPL_STRIP") != "", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %d sets, %d skipped, warnings %v (preview %v)", pv.Name, len(pv.Sets), len(pv.Skipped), pv.Warnings, time.Since(start))
	for _, s := range pv.Skipped {
		t.Logf("  skipped %s %q: %s", s.Kind, s.Name, s.Reason)
	}
	root := os.Getenv("EPL_KEEP")
	if root == "" {
		root = t.TempDir()
	}
	ws := project.Workspace{Root: filepath.Join(root, "ws"), Library: filepath.Join(root, "lib")}
	for _, s := range pv.Sets {
		t.Logf("set %q (%s): %d cards, mode %s, problems %d %v, packs %+v, warnings %v", s.Name, s.ProjectID, s.Cards,
			s.RenderMode, s.ProblemN, s.Problems, s.Packs, s.Warnings)
		if f := os.Getenv("EPL_SET"); f != "" && !strings.Contains(strings.ToLower(s.Name), strings.ToLower(f)) {
			continue
		}
		t0 := time.Now()
		last := ""
		p, err := eplSource{}.Import(context.Background(), ws, s.Code, eplSource{}.DefaultOptions(), func(pr Progress) {
			if pr.Stage == "done" {
				last = pr.Message
			}
		})
		if err != nil {
			t.Fatalf("%s: %v", s.Name, err)
		}
		t.Logf("  imported in %v: %s", time.Since(t0), last)
		for _, pk := range p.Set.Packs {
			n := 0
			for _, sl := range pk.Slots {
				n += sl.Count
			}
			t.Logf("  pack %s %q foil %.2f%% slots %v (sum %d) art %q %q box %v %q %q", pk.ID, pk.Name, pk.FoilChance, pk.Slots, n,
				pk.PackTexture, pk.PackIcon, pk.HasBox, pk.BoxTexture, pk.BoxIcon)
		}
		t.Logf("  card back %q, render %s, first card %+v", p.Set.CardBack, p.Set.RenderMode, p.Set.Cards[0])
		for id, art := range p.Meta.PackArt {
			t.Logf("  packArt %s: pack %s box %s", id, art["pack"], art["box"])
		}
		for _, i := range p.Set.Validate(p.Folder, p.LibFolder) {
			if i.Level == "error" {
				t.Errorf("  validation: %+v", i)
			}
		}
	}
}

// EPL_LIVE=<mod> go test ./internal/importer -run EPLItemsLive -v: converts the mod's accessories into a temporary library.
func TestEPLItemsLive(t *testing.T) {
	path := os.Getenv("EPL_LIVE")
	if path == "" {
		t.Skip("EPL_LIVE not set")
	}
	if b, err := os.ReadFile("../../frontend/src/lib/figurineSizes.json"); err == nil {
		var d struct {
			Items []ToySize `json:"items"`
		}
		_ = json.Unmarshal(b, &d)
		FigurineToys = d.Items
	}
	pv, err := ScanEPL(path, os.Getenv("EPL_STRIP") != "", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range pv.Skipped {
		t.Logf("skipped %s %q: %s", s.Kind, s.Name, s.Reason)
	}
	var keys []string
	for _, it := range pv.Items {
		t.Logf("item %-50q %-10s base %-18s id %s", it.Name, it.Kind, it.Base, it.ID)
		keys = append(keys, it.Key)
	}
	root := t.TempDir()
	if k := os.Getenv("EPL_KEEP"); k != "" {
		root = k
	}
	lib, err := accessories.Open(root, assets.For(root))
	if err != nil {
		t.Fatal(err)
	}
	n, icons, failed, err := ImportEPLItems(path, keys, Options{StripNumbers: os.Getenv("EPL_STRIP") != "", OriginMod: os.Getenv("EPL_MODNAME")}, lib, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("converted %d, %d icons to render, failed %v", n, len(icons), failed)
	for id, o := range lib.Meta.Origins {
		t.Logf("origin %s: %+v", id, o)
		break
	}
	if len(lib.Meta.Origins) != n {
		t.Errorf("%d origins for %d items", len(lib.Meta.Origins), n)
	}
	for _, a := range lib.Lib.Accessories {
		var l struct {
			TextureFile string `json:"textureFile"`
			Version     string `json:"version"`
		}
		_ = json.Unmarshal(lib.Meta.Layouts[a.ID], &l)
		if a.Kind != "Figurine" && l.TextureFile == "" {
			t.Errorf("%s: no editor layout with the texture", a.ID)
		}
	}
	if err := lib.Save(); err != nil {
		t.Fatal(err)
	}
	errs, warns := lib.Lib.Validate(lib.Resolve)
	t.Logf("validation errors %v warnings %v", errs, warns)
	if len(errs) > 0 || n != len(keys) {
		t.Fail()
	}
}

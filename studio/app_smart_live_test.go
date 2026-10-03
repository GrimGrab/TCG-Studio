package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSmartArtSourcesLive runs Smart generate's source lookup on copies of real projects (set.json, studio.json and the
// set icon only; the originals are not touched): SMART_LIVE="<project folder>[,…]" go test . -run SmartArtSourcesLive -v
func TestSmartArtSourcesLive(t *testing.T) {
	spec := os.Getenv("SMART_LIVE")
	if spec == "" {
		t.Skip("set SMART_LIVE=<project folders> to run")
	}
	for _, src := range strings.Split(spec, ",") {
		id := filepath.Base(src)
		t.Run(id, func(t *testing.T) {
			root := t.TempDir()
			if r := os.Getenv("SMART_ROOT"); r != "" { // keep the result (photos + sources.json) for a look
				root = r
			}
			dst := filepath.Join(root, "projects", id)
			for _, f := range []string{"set.json", "studio.json", "images/set_icon.svg", "images/set_logo.png"} {
				b, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(f)))
				if err != nil {
					continue
				}
				_ = os.MkdirAll(filepath.Dir(filepath.Join(dst, filepath.FromSlash(f))), 0o755)
				if err := os.WriteFile(filepath.Join(dst, filepath.FromSlash(f)), b, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			a := &App{ctx: context.Background()}
			a.settings.Workspace = root
			p, err := a.ws().Load(id)
			if err != nil {
				t.Fatal(err)
			}
			for _, pk := range p.Set.Packs {
				s, err := a.SmartArtSources(id, pk.ID, SmartChoice{})
				if err != nil {
					t.Fatal(err)
				}
				b, _ := json.MarshalIndent(s, "", " ")
				t.Logf("pack %s:\n%s", pk.ID, b)
				_ = os.WriteFile(filepath.Join(dst, "sources_"+pk.ID+".json"), b, 0o644)
			}
		})
	}
}

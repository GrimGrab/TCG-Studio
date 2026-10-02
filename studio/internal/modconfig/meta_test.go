package modconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every rule in settings-meta.json names settings that exist and values the controlling setting can take.
func TestSettingsMeta(t *testing.T) {
	m, err := SettingsMeta()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(p, defaultsCfg, 0o644); err != nil {
		t.Fatal(err)
	}
	secs, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]Entry{}
	for _, s := range secs {
		for _, e := range s.Entries {
			byKey[e.Key] = e
		}
	}
	if len(m.ShowWhen) == 0 {
		t.Fatal("no showWhen rules parsed")
	}
	for key, rule := range m.ShowWhen {
		if _, ok := byKey[key]; !ok {
			t.Errorf("showWhen %s: no such setting", key)
		}
		ctl, ok := byKey[rule.Setting]
		if !ok {
			t.Errorf("showWhen %s: controlling setting %s doesn't exist", key, rule.Setting)
			continue
		}
		allowed := ctl.Options
		if ctl.Type == "Boolean" {
			allowed = []string{"true", "false"}
		}
		if len(rule.Is) == 0 {
			t.Errorf("showWhen %s: no values", key)
		}
		for _, v := range rule.Is {
			found := len(allowed) == 0 // free-form setting: any value
			for _, a := range allowed {
				if strings.EqualFold(a, v) {
					found = true
				}
			}
			if !found {
				t.Errorf("showWhen %s: %s can't be %q (allowed: %v)", key, rule.Setting, v, allowed)
			}
		}
	}
}

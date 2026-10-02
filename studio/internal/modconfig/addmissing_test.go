package modconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddMissing(t *testing.T) {
	file := "[MTG]\r\n\r\n## Old.\r\n# Setting type: Boolean\r\n# Default value: true\r\nOld = false\r\n"
	defaults := file +
		"\r\n## New one.\r\n# Setting type: AiDeckStyle\r\n# Default value: Random\r\n# Acceptable values: Random, Sealed\r\nAiDeckStyle = Random\r\n" +
		"\r\n[Extra]\r\n\r\n## Brand new section.\r\n# Setting type: Single\r\n# Default value: 1\r\nScale = 1\r\n"
	p := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(p, []byte(file), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := addMissing(p, []byte(defaults))
	if err != nil || n != 2 {
		t.Fatalf("added %d, err %v", n, err)
	}
	secs, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Entry{}
	for _, s := range secs {
		for _, e := range s.Entries {
			got[s.Name+"/"+e.Key] = e
		}
	}
	if got["MTG/Old"].Value != "false" {
		t.Errorf("existing value changed: %+v", got["MTG/Old"])
	}
	if e := got["MTG/AiDeckStyle"]; e.Value != "Random" || len(e.Options) != 2 || e.Description != "New one." {
		t.Errorf("AiDeckStyle not added right: %+v", e)
	}
	if got["Extra/Scale"].Type != "Single" {
		t.Errorf("new section not added: %+v", secs)
	}
	if n, _ := addMissing(p, []byte(defaults)); n != 0 {
		t.Errorf("second run added %d", n)
	}
	b, _ := os.ReadFile(p)
	if strings.Contains(strings.ReplaceAll(string(b), "\r\n", ""), "\n") {
		t.Errorf("mixed line endings")
	}
}

// A setting moved to a sub-section keeps its value; the old entry goes, and restoring the parent section covers it.
func TestAddMissingMoved(t *testing.T) {
	rim := "## Rim.\r\n# Setting type: Boolean\r\n# Default value: true\r\nHoloFoilRim = true\r\n"
	file := "[Foil]\r\n\r\n" + rim + "\r\n## Base colour.\r\n# Setting type: Color\r\n# Default value: FFFFFFFF\r\nBaseColor = 112233FF\r\n"
	defaults := "[Foil]\r\n\r\n" + rim + "\r\n[Foil - Base]\r\n\r\n## Base colour.\r\n# Setting type: Color\r\n# Default value: FFFFFFFF\r\nBaseColor = FFFFFFFF\r\n"
	p := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(p, []byte(file), 0o644); err != nil {
		t.Fatal(err)
	}
	if n, err := addMissing(p, []byte(defaults)); err != nil || n != 1 {
		t.Fatalf("added %d, err %v", n, err)
	}
	secs, _ := Read(p)
	var where []string
	for _, s := range secs {
		for _, e := range s.Entries {
			if e.Key == "BaseColor" {
				where = append(where, s.Name+"="+e.Value)
			}
		}
	}
	if len(where) != 1 || where[0] != "Foil - Base=112233FF" {
		t.Errorf("BaseColor = %v, want only [Foil - Base] 112233FF", where)
	}
	if b, _ := os.ReadFile(p); strings.Count(string(b), "## Base colour.") != 1 {
		t.Errorf("old block left behind:\n%s", b)
	}
	if n, _ := RestoreDefaults(p, "Foil"); n != 1 {
		t.Errorf("RestoreDefaults(Foil) reset %d, want 1 (sub-section included)", n)
	}
}

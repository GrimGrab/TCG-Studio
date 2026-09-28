package modconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = "## Settings file was created by plugin TCG Custom Cards v0.3.1\r\n## Plugin GUID: tcgcustomcards\r\n\r\n" +
	"[Content]\r\n\r\n## Show the vanilla card sets.\r\n# Setting type: Boolean\r\n# Default value: true\r\nShowVanillaCards = false\r\n\r\n" +
	"[Display]\r\n\r\nFullImageScale = 0.900939\r\n\r\n" +
	"[Foil]\r\n\r\n## Speed.\r\n## Second line.\r\n# Setting type: Single\r\n# Default value: 1.49\r\n# Acceptable value range: From 0 to 3\r\nHoloFoilMotion = 1.492958\r\n\r\n" +
	"## Base foil colour.\r\n# Setting type: Color\r\n# Default value: 376761FF\r\nBaseColor = 376761FF\r\n\r\n" +
	"## Pattern.\r\n# Setting type: HoloPattern\r\n# Default value: None\r\n# Acceptable values: None, Sparkle, Etched, Cosmos\r\nBasePattern = None\r\n"

func write(t *testing.T) string {
	p := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(p, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRead(t *testing.T) {
	secs, err := Read(write(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(secs) != 2 || secs[0].Name != "Content" || secs[1].Name != "Foil" {
		t.Fatalf("sections = %+v (orphan-only [Display] must be dropped)", secs)
	}
	m := secs[1].Entries[0]
	if m.Key != "HoloFoilMotion" || m.Type != "Single" || *m.Min != 0 || *m.Max != 3 || m.Description != "Speed. Second line." || m.Default != "1.49" {
		t.Fatalf("motion = %+v", m)
	}
	if p := secs[1].Entries[2]; len(p.Options) != 4 || p.Options[3] != "Cosmos" {
		t.Fatalf("pattern = %+v", p)
	}
}

func TestSetKeepsFile(t *testing.T) {
	p := write(t)
	if _, err := Set(p, "Foil", "HoloFoilMotion", "2"); err != nil {
		t.Fatal(err)
	}
	if _, err := Set(p, "Foil", "HoloFoilMotion", "9"); err == nil {
		t.Fatal("out of range accepted")
	}
	if _, err := Set(p, "Foil", "BasePattern", "Glitter"); err == nil {
		t.Fatal("unknown enum value accepted")
	}
	if _, err := Set(p, "Content", "ShowVanillaCards", "true"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	want := strings.Replace(strings.Replace(sample, "HoloFoilMotion = 1.492958", "HoloFoilMotion = 2", 1), "ShowVanillaCards = false", "ShowVanillaCards = true", 1)
	if string(b) != want {
		t.Fatalf("file changed beyond the value lines:\n%s", b)
	}
}

func TestRestoreDefaults(t *testing.T) {
	p := write(t)
	n, err := RestoreDefaults(p, "Foil")
	if err != nil || n != 1 { // only HoloFoilMotion differs from its default in [Foil]
		t.Fatalf("n=%d err=%v", n, err)
	}
	secs, _ := Read(p)
	if secs[0].Entries[0].Value != "false" || secs[1].Entries[0].Value != "1.49" {
		t.Fatalf("after section restore: %+v", secs)
	}
	if n, _ := RestoreDefaults(p, ""); n != 1 {
		t.Fatalf("restore all changed %d, want 1 (ShowVanillaCards)", n)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "FullImageScale = 0.900939") {
		t.Fatal("orphaned values must be left alone")
	}
}

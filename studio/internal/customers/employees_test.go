package customers

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// Perks point at real perks, every perk says what it does, and each situation has built-in lines to say.
func TestEmployeeDefaults(t *testing.T) {
	d, err := LoadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	if len(d.EmployeePerks) == 0 {
		t.Fatal("no perks")
	}
	ids := map[string]bool{}
	for _, p := range d.EmployeePerks {
		if ids[p.ID] {
			t.Errorf("perk %q is listed twice", p.ID)
		}
		ids[p.ID] = true
		if p.Name == "" || p.Description == "" {
			t.Errorf("perk %q needs a name and a description", p.ID)
		}
		if len(p.Effects) == 0 {
			t.Errorf("perk %q does nothing", p.ID)
		}
	}
	for _, p := range d.EmployeePerks {
		if p.Opposite != "" && !ids[p.Opposite] {
			t.Errorf("perk %q: unknown opposite %q", p.ID, p.Opposite)
		}
	}
	for _, s := range d.EmployeeSituations {
		if len(d.EmployeeLines[s.ID]) == 0 {
			t.Errorf("employee situation %q has no lines", s.ID)
		}
	}
}

// Every situation the mod's employees say something in is listed.
func TestEmployeeSituationsListed(t *testing.T) {
	d, _ := LoadDefaults()
	have := map[string]bool{}
	for _, s := range d.EmployeeSituations {
		have[s.ID] = true
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "src", "TCGCustomCards", "Runtime", "Employees", "EmployeeLife.cs"))
	if err != nil {
		t.Skip("mod source not found")
	}
	for _, m := range regexp.MustCompile(`"(emp[A-Z]\w+)"`).FindAllSubmatch(b, -1) {
		if !have[string(m[1])] {
			t.Errorf("EmployeeLife.cs says %q, which defaults.json doesn't list", m[1])
		}
	}
}

// Saving the customers' lines elsewhere must not lose the employees'.
func TestEmployeeLinesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := SetupLines{EmployeeLines: map[string][]string{"empGreet": {"Hi."}}, EmployeeRemoved: map[string][]string{"empScan": {"Beep."}}}
	if err := SaveLines(dir, in); err != nil {
		t.Fatal(err)
	}
	out, err := LoadLines(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.EmployeeLines["empGreet"]) != 1 || len(out.EmployeeRemoved["empScan"]) != 1 {
		t.Errorf("lost: %+v", out)
	}
}

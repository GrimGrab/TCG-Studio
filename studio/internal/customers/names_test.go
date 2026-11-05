package customers

import (
	"reflect"
	"strings"
	"testing"
)

func TestDefaultNames(t *testing.T) {
	d, err := LoadDefaults()
	if err != nil {
		t.Fatal(err)
	}
	for _, list := range NameLists {
		names := d.Names[list]
		if len(names) < 100 {
			t.Errorf("%s: only %d names", list, len(names))
		}
		seen := map[string]bool{}
		for _, n := range names {
			if strings.TrimSpace(n) != n || n == "" || seen[n] {
				t.Errorf("%s: empty, untrimmed or repeated name %q", list, n)
			}
			seen[n] = true
		}
	}
}

func TestSetupNamesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := SetupLines{Names: map[string][]string{"last": {" Rossi ", ""}}, NamesOff: map[string][]string{"male": {"Sam"}}}
	if err := SaveLines(dir, in); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLines(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Names["last"], []string{"Rossi"}) || !reflect.DeepEqual(got.NamesOff["male"], []string{"Sam"}) {
		t.Errorf("got %+v", got)
	}
}

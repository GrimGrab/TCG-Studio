package ids

import (
	"regexp"
	"testing"

	"tcgstudio/internal/setfmt"
)

func TestNew(t *testing.T) {
	re := regexp.MustCompile(`^[a-z][a-z2-7]{11}$`)
	seen := map[string]bool{}
	for i := 0; i < 100_000; i++ {
		id := New()
		if !re.MatchString(id) || !setfmt.SafeID(id) {
			t.Fatalf("bad id %q", id)
		}
		if seen[id] {
			t.Fatalf("repeated id %q", id)
		}
		seen[id] = true
	}
}

func TestFree(t *testing.T) {
	n := 0
	id := Free(func(string) bool { n++; return n < 3 })
	if n != 3 || id == "" {
		t.Fatalf("Free tried %d ids", n)
	}
}

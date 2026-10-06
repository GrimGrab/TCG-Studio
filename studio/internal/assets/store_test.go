package assets

import (
	"os"
	"slices"
	"testing"
)

func TestPutDedupeAndRefs(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	a, err := s.Put([]byte("hello"), ".PNG")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Put([]byte("hello"), "png")
	if err != nil || a != b || !IsAsset(a) {
		t.Fatalf("dedupe: %q %q %v", a, b, err)
	}
	c, _ := s.Put([]byte("other"), ".obj")
	if c == a {
		t.Fatal("different content, same name")
	}
	if got, _ := os.ReadFile(s.Path(a)); string(got) != "hello" {
		t.Fatalf("content %q", got)
	}
	list, _ := s.List()
	if len(list) != 2 {
		t.Fatalf("list %v", list)
	}
	doc := []byte(`{"texture":"` + a + `","layers":[{"src":"` + c + `"},{"src":"images/src/x.png"}],"n":2,"dup":"` + a + `"}`)
	if refs := Refs(doc); !slices.Equal(refs, []string{a, c}) && !slices.Equal(refs, []string{c, a}) {
		t.Fatalf("refs %v", refs)
	}
	for _, bad := range []string{"assets/../x.png", "assets/sub/x.png", "images/x.png", "assets"} {
		if IsAsset(bad) {
			t.Errorf("%q counted as an asset", bad)
		}
	}
	if _, err := s.Put([]byte("x"), "../evil"); err == nil {
		t.Error("bad extension accepted")
	}
}

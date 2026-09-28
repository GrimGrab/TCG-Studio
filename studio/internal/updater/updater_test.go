package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.3.1", "0.3.0", true}, {"v0.4.0", "0.3.9", true}, {"1.0.0", "0.99.99", true},
		{"0.3.0", "0.3.0", false}, {"0.2.9", "0.3.0", false}, {"dev", "0.3.0", false}, {"0.3.10", "0.3.9", true},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q,%q)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestCheckDownloadSwap(t *testing.T) {
	payload := []byte("new exe bytes")
	sum := sha256.Sum256(payload)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v0.3.1","html_url":"x","body":"Fixed the binder.\n\nsha256: %s\n",
				"assets":[{"name":"TCG Studio.exe","browser_download_url":"%s/dl","size":%d}]}`, hex.EncodeToString(sum[:]), srv.URL, len(payload))
		case "/dl":
			w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	APIBase = srv.URL

	rel, err := Latest(context.Background(), "o/r")
	if err != nil || rel == nil {
		t.Fatalf("latest: %v %v", rel, err)
	}
	if rel.Version != "0.3.1" || rel.Notes != "Fixed the binder." || rel.SHA256 == "" {
		t.Fatalf("bad parse: %+v", rel)
	}
	file, err := Download(context.Background(), rel, nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "TCG Studio.exe")
	os.WriteFile(exe, []byte("old exe"), 0o755)
	if err := Swap(exe, file); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b) != string(payload) {
		t.Fatal("exe not replaced")
	}
	if b, _ := os.ReadFile(exe + ".old"); string(b) != "old exe" {
		t.Fatal("old exe not kept")
	}
	Cleanup(exe)
	if _, err := os.Stat(exe + ".old"); err == nil {
		t.Fatal("old exe not cleaned up")
	}
}

func TestBadChecksumRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("tampered")) }))
	defer srv.Close()
	_, err := Download(context.Background(), &Release{Version: "9", URL: srv.URL, SHA256: "00000000000000000000000000000000000000000000000000000000000000ff"}, nil)
	if err == nil {
		t.Fatal("expected checksum error")
	}
}

func TestNoReleasesYet(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	APIBase = srv.URL
	rel, err := Latest(context.Background(), "o/r")
	if err != nil || rel != nil {
		t.Fatalf("want nil,nil got %v %v", rel, err)
	}
}

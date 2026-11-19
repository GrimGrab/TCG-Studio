package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// A file the WebView has cached comes again as a whole file, never as 304: Wails turns a 304 into a 500, which made fonts
// loaded before fail their preview ("can't preview this font").
func TestFileHandlerNeverAnswers304(t *testing.T) {
	f := filepath.Join(t.TempDir(), "font.ttf")
	if err := os.WriteFile(f, []byte("\x00\x01\x00\x00 font bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	pickedFonts.mu.Lock()
	pickedFonts.paths = append(pickedFonts.paths, f)
	id := len(pickedFonts.paths) - 1
	pickedFonts.mu.Unlock()

	h := (&App{}).fileHandler()
	req := httptest.NewRequest("GET", "/pickedfont/"+strconv.Itoa(id), nil)
	req.Header.Set("If-Modified-Since", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Fatalf("got %d with %d bytes, want the whole file", rec.Code, rec.Body.Len())
	}
}

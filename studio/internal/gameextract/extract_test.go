package gameextract

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Extracts from the installed game (TCG_GAME, default Steam path) into a temp folder; EXTRACT_OUT keeps it somewhere.
func TestExtract(t *testing.T) {
	game := os.Getenv("TCG_GAME")
	if game == "" {
		game = `D:\SteamLibrary\steamapps\common\TCG Card Shop Simulator`
	}
	if _, err := DataDir(game); err != nil {
		t.Skip(err)
	}
	out := os.Getenv("EXTRACT_OUT")
	if out == "" {
		out = filepath.Join(t.TempDir(), "templates")
	}
	start := time.Now()
	info, err := Extract(game, out, func(s string) { t.Log(s) })
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("done in %v, %d warnings", time.Since(start).Round(time.Millisecond), len(info.Warnings))
	for _, w := range info.Warnings {
		t.Log("warning:", w)
	}
}

package game

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRenameShop(t *testing.T) {
	dir := t.TempDir()
	save := func(name, body string, age time.Duration) string {
		f := filepath.Join(dir, name)
		if err := os.WriteFile(f, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		at := time.Now().Add(-age)
		_ = os.Chtimes(f, at, at)
		return f
	}
	a := save("savedGames_Release1.json", `{"m_CoinAmount":12.5,"m_PlayerName":"Old \"Shop\"","m_Other":"x"}`, time.Hour)
	b := save("savedGames_ReleaseBackupFile1.json", `{"m_PlayerName":"Old \"Shop\""}`, 2*time.Hour)
	c := save("savedGames_Release2.json", `{"m_PlayerName":"Another Shop"}`, 3*time.Hour)
	if got := shopNameIn(dir); got != `Old "Shop"` {
		t.Fatalf("name %q", got)
	}
	before, _ := os.Stat(a)
	n, err := renameShopIn(dir, `Old "Shop"`, "  Robin's <Cards> & Co  ")
	if err != nil || n != 2 {
		t.Fatalf("renamed %d %v", n, err)
	}
	got, _ := os.ReadFile(a)
	if string(got) != `{"m_CoinAmount":12.5,"m_PlayerName":"Robin's <Cards> & Co","m_Other":"x"}` {
		t.Fatalf("save %s", got)
	}
	if after, _ := os.Stat(a); !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("modification time changed")
	}
	if g, _ := os.ReadFile(b); !strings.Contains(string(g), "Robin") {
		t.Fatalf("backup %s", g)
	}
	if g, _ := os.ReadFile(c); !strings.Contains(string(g), "Another Shop") {
		t.Fatal("another shop's save renamed")
	}
	if _, err := renameShopIn(dir, "x", "  "); err == nil {
		t.Fatal("empty name accepted")
	}
}

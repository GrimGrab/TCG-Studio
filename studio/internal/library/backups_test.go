package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBackups(t *testing.T) {
	game := t.TempDir()
	write := func(rel, data string) {
		p := filepath.Join(game, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`TCGStudio_backup_20261015-154542\install.log`, "log")
	write(`TCGStudio_backup_20261016-155941\BepInEx\plugins\TCGCustomCards\Sets\a\set.json`, "{}")
	write(`TCGStudio_backup_20261016-155941-2\x.txt`, "x")
	write(`TCGStudio_backup_old\x.txt`, "x") // not a Studio name: left alone
	write(`BepInEx\plugins\TCGCustomCards\TCGCustomCards.dll`, "dll")

	list := Backups(game)
	if len(list) != 3 {
		t.Fatalf("got %d backups, want 3: %+v", len(list), list)
	}
	if list[0].Name != "TCGStudio_backup_20261016-155941-2" || list[2].Kind != "install" || list[1].Kind != "content" {
		t.Fatalf("order/kind wrong: %+v", list)
	}
	if list[1].Files != 1 || list[1].Bytes != 2 || list[1].Time.IsZero() {
		t.Fatalf("size/time wrong: %+v", list[1])
	}

	if _, err := DeleteBackups(game, []string{"BepInEx"}); err == nil {
		t.Fatal("deleting a non-backup folder must fail")
	}
	if _, err := DeleteBackups(game, []string{`TCGStudio_backup_20261016-155941\..\BepInEx`}); err == nil {
		t.Fatal("a path through a backup name must fail")
	}
	freed, err := DeleteBackups(game, []string{"TCGStudio_backup_20261016-155941", "TCGStudio_backup_20261015-154542"})
	if err != nil || freed != 5 {
		t.Fatalf("freed %d, err %v", freed, err)
	}
	if got := Backups(game); len(got) != 1 {
		t.Fatalf("left %d backups, want 1", len(got))
	}
	if _, err := os.Stat(filepath.Join(game, `BepInEx\plugins\TCGCustomCards\TCGCustomCards.dll`)); err != nil {
		t.Fatal("the mod was touched")
	}
}

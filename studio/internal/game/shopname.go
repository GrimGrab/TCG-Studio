package game

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The shop's name is m_PlayerName in the game's saves (CGameData; the game shows it on the sign and in its menus).

// saveFiles are the game's JSON saves: every slot and its backup (the .gd files are only read when a JSON is missing).
func saveFiles(dir string) []string {
	var out []string
	seen := map[string]bool{}
	for _, pattern := range []string{"savedGames_Release*.json", "savedGames_ReleaseBackupFile*.json"} {
		m, _ := filepath.Glob(filepath.Join(dir, pattern))
		for _, f := range m {
			if !seen[f] { // the first pattern matches the backups too
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	return out
}

// playerName matches the m_PlayerName field of a save (JsonUtility writes it without spaces).
var playerName = regexp.MustCompile(`"m_PlayerName"\s*:\s*"((?:[^"\\]|\\.)*)"`)

func nameIn(b []byte) (string, []int, bool) {
	loc := playerName.FindSubmatchIndex(b)
	if loc == nil {
		return "", nil, false
	}
	var s string
	if json.Unmarshal(b[loc[2]-1:loc[3]+1], &s) != nil {
		return "", nil, false
	}
	return s, loc, true
}

// ShopName is the shop's name in the most recently written save ("" without saves).
func ShopName() string { return shopNameIn(SaveDir()) }

func shopNameIn(dir string) string {
	var newest string
	var at time.Time
	files, _ := filepath.Glob(filepath.Join(dir, "savedGames_Release*.json"))
	for _, f := range files {
		if strings.Contains(filepath.Base(f), "BackupFile") {
			continue
		}
		if info, err := os.Stat(f); err == nil && info.ModTime().After(at) {
			newest, at = f, info.ModTime()
		}
	}
	b, err := os.ReadFile(newest)
	if err != nil {
		return ""
	}
	name, _, _ := nameIn(b)
	return name
}

// RenameShop sets the shop's name to name in every save (and backup) that has the name old. Only that field changes and
// each file keeps its modification time (which save is newest stays the same). The game must be closed: it keeps the
// name in memory and writes it back on its next save. Returns the number of files changed.
func RenameShop(old, name string) (int, error) {
	if IsRunning() {
		return 0, errors.New("close the game first to rename the shop — it would write the old name back")
	}
	return renameShopIn(SaveDir(), old, name)
}

func renameShopIn(dir, old, name string) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, errors.New("the shop name can't be empty")
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(name); err != nil {
		return 0, err
	}
	quoted := bytes.TrimSpace(buf.Bytes())
	n := 0
	for _, f := range saveFiles(dir) {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			return n, err
		}
		cur, loc, ok := nameIn(b)
		if !ok || cur != old || cur == name {
			continue
		}
		out := append(append(append([]byte{}, b[:loc[2]-1]...), quoted...), b[loc[3]+1:]...)
		if !json.Valid(out) {
			return n, errors.New("couldn't rename the shop in " + filepath.Base(f))
		}
		tmp := f + ".tcgstudio"
		if err := os.WriteFile(tmp, out, info.Mode().Perm()); err != nil {
			return n, err
		}
		if err := os.Rename(tmp, f); err != nil {
			_ = os.Remove(tmp)
			return n, err
		}
		_ = os.Chtimes(f, info.ModTime(), info.ModTime())
		n++
	}
	return n, nil
}

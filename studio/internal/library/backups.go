package library

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

// Backup folders in the game folder: <game>\TCGStudio_backup_<yyyymmdd-hhmmss>[-n]\, made by Game & mod's Install /
// Repair (files it replaced, plus install.log) and by RemoveGameCopies (the content older Studio versions copied into
// the game). Nothing reads them; the Storage page lists them so the player can open or delete them.

var backupName = regexp.MustCompile(`^TCGStudio_backup_(\d{8}-\d{6})(-\d+)?$`)

// BackupInfo is one backup folder.
type BackupInfo struct {
	Name  string    `json:"name"`
	Path  string    `json:"path"`
	Kind  string    `json:"kind"` // "install" (Install / Repair) or "content" (the game's old copies of your content)
	Time  time.Time `json:"time"`
	Bytes int64     `json:"bytes"`
	Files int       `json:"files"`
}

// Backups lists the game folder's backup folders, newest first.
func Backups(gameDir string) []BackupInfo {
	out := []BackupInfo{}
	entries, _ := os.ReadDir(gameDir)
	for _, e := range entries {
		m := backupName.FindStringSubmatch(e.Name())
		if !e.IsDir() || m == nil {
			continue
		}
		dir := filepath.Join(gameDir, e.Name())
		b := BackupInfo{Name: e.Name(), Path: dir, Kind: "content"}
		b.Time, _ = time.ParseInLocation("20060102-150405", m[1], time.Local)
		b.Bytes, b.Files = dirSize(dir)
		if _, err := os.Stat(filepath.Join(dir, "install.log")); err == nil {
			b.Kind = "install"
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out
}

// DeleteBackups deletes the named backup folders of the game folder (names only; anything that isn't a backup folder
// there is refused). Returns the bytes freed.
func DeleteBackups(gameDir string, names []string) (int64, error) {
	var freed int64
	for _, name := range names {
		if !backupName.MatchString(name) {
			return freed, fmt.Errorf("%q is not a TCG Studio backup folder", name)
		}
		dir := filepath.Join(gameDir, name)
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		n, _ := dirSize(dir)
		if err := os.RemoveAll(dir); err != nil {
			return freed, err
		}
		freed += n
	}
	return freed, nil
}

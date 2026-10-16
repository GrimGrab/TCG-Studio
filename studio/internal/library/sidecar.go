package library

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/game"
	"tcgstudio/internal/setups"
)

// The mod's side-car saves (<save folder>\TCGCustomCards\slot{n}.json + .bak) are gzip-compressed JSON since 2026-10-15;
// older ones are plain JSON until the game saves that slot again. The mod reads both, so Studio can compress the rest:
// the same JSON, nothing else changed.

const sideCarDir = "TCGCustomCards"

// sideCarRatio is the measured size of a compressed side-car against its plain JSON (indented, 27 MB → 2.6 MB).
const sideCarRatio = 0.1

// PlainSaves is what CompressSaves would do.
type PlainSaves struct {
	Files     int   `json:"files"`
	Bytes     int64 `json:"bytes"`     // their size now
	After     int64 `json:"after"`     // estimate once compressed
	GameFiles int   `json:"gameFiles"` // of which in the game's own save folder (only while the game is closed)
}

// sideCarFolders are the side-car folders: every setup's parked saves, then (gameDir set) the game's save folder.
func sideCarFolders(h setups.Home, withGame bool) []string {
	var out []string
	if list, err := h.List(); err == nil {
		for _, s := range list {
			out = append(out, filepath.Join(s.Folder, setups.SavesDir, sideCarDir))
		}
	}
	if withGame {
		out = append(out, filepath.Join(game.SaveDir(), sideCarDir))
	}
	return out
}

// plainSideCars lists the side-car files (slot*.json, slot*.json.bak) that aren't compressed yet.
func plainSideCars(dir string) []string {
	var out []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasPrefix(n, "slot") || !(strings.HasSuffix(n, ".json") || strings.HasSuffix(n, ".json.bak")) {
			continue
		}
		f := filepath.Join(dir, n)
		if head, err := readHead(f); err == nil && !(len(head) == 2 && head[0] == 0x1f && head[1] == 0x8b) && len(head) > 0 {
			out = append(out, f)
		}
	}
	return out
}

func readHead(f string) ([]byte, error) {
	r, err := os.Open(f)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	b := make([]byte, 2)
	n, err := io.ReadFull(r, b)
	if err == io.ErrUnexpectedEOF {
		err = nil
	}
	return b[:n], err
}

// checkPlainSaves measures the plain side-car files (the game's folder only when withGame).
func checkPlainSaves(h setups.Home, withGame bool) PlainSaves {
	var r PlainSaves
	gameFolder := filepath.Join(game.SaveDir(), sideCarDir)
	for _, dir := range sideCarFolders(h, withGame) {
		for _, f := range plainSideCars(dir) {
			r.Files++
			r.Bytes += size(f)
			if dir == gameFolder {
				r.GameFiles++
			}
		}
	}
	r.After = int64(float64(r.Bytes) * sideCarRatio)
	return r
}

// CompressSaves gzips every plain side-car file (setups' parked saves, and the game's save folder when gameClosed). Each
// file is written next to the original, read back and compared, then put in its place with its modification time kept.
func CompressSaves(ctx context.Context, h setups.Home, gameClosed bool, progress func(Progress)) (*Result, error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	var files []string
	for _, dir := range sideCarFolders(h, gameClosed) {
		files = append(files, plainSideCars(dir)...)
	}
	res := &Result{}
	for i, f := range files {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if gameClosed && game.IsRunning() { // the game started meanwhile: it may be writing its save folder
			return res, errors.New("the game was started — the rest is left as it is")
		}
		progress(Progress{Message: "Compressing " + filepath.Base(f) + "…", Done: i, Total: len(files)})
		before := size(f)
		after, err := compressFile(f)
		if err != nil {
			return res, err
		}
		res.Files++
		res.Freed += before - after
	}
	progress(Progress{Message: "Done", Done: len(files), Total: len(files)})
	return res, nil
}

// compressFile replaces a plain file by its gzip-compressed copy (checked before the swap). Returns the new size.
func compressFile(f string) (int64, error) {
	info, err := os.Stat(f)
	if err != nil {
		return 0, err
	}
	plain, err := os.ReadFile(f)
	if err != nil {
		return 0, err
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if _, err := zw.Write(plain); err != nil {
		return 0, err
	}
	if err := zw.Close(); err != nil {
		return 0, err
	}
	zr, err := gzip.NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return 0, err
	}
	back, err := io.ReadAll(zr)
	if err != nil || !bytes.Equal(back, plain) {
		return 0, errors.New("compressing " + filepath.Base(f) + " didn't give the same data back — left as it is")
	}
	tmp := f + ".compressing"
	if err := os.WriteFile(tmp, buf.Bytes(), info.Mode().Perm()); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	if now, err := os.Stat(f); err != nil || !now.ModTime().Equal(info.ModTime()) || now.Size() != info.Size() {
		_ = os.Remove(tmp) // written meanwhile (a save): keep that one
		return size(f), nil
	}
	if err := os.Rename(tmp, f); err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	_ = os.Chtimes(f, info.ModTime(), info.ModTime())
	return int64(buf.Len()), nil
}

package epl

import (
	"archive/zip"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Bundle is one descriptor + asset bundle pair of a mod (EPL loads X.json with the bundle file X beside it).
type Bundle struct {
	JSON string // path of X.json
	Path string // path of the bundle X
	Rel  string // X.json relative to the mod folder / zip, for messages
	Desc *Descriptor
}

// Mod is a mod folder or zip with its descriptor/bundle pairs.
type Mod struct {
	Source  string // the folder or .zip the player picked
	Name    string // its name without extension
	Bundles []Bundle
	Skipped []string // JSON files that aren't EPL descriptors or have no bundle beside them
	temp    string   // extraction folder for a zip (removed by Close)
}

// Open finds every EPL descriptor in a mod folder or .zip. A zip's descriptors and bundles are extracted to a
// subfolder of cacheDir (named by the zip's path and size), reused while it exists; Close removes it.
func Open(path, cacheDir string, progress func(done, total int64)) (*Mod, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	m := &Mod{Source: path, Name: name}
	dir := path
	if !st.IsDir() {
		extract := extractDescriptors
		switch {
		case strings.EqualFold(filepath.Ext(path), ".zip"):
		case isTarArchive(path):
			extract = extractArchive
		default:
			return nil, fmt.Errorf("pick an EPL mod's folder or its .zip, .rar or .7z")
		}
		m.temp = extractDir(path, st, cacheDir)
		if err := extract(path, m.temp, progress); err != nil {
			os.RemoveAll(m.temp)
			return nil, err
		}
		dir = m.temp
	}
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".json") {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		bundle := strings.TrimSuffix(p, filepath.Ext(p))
		if bst, err := os.Stat(bundle); err != nil || bst.IsDir() {
			return nil // ordinary JSON (configs, manifests): not an EPL descriptor
		}
		desc, err := ReadDescriptor(p)
		if err != nil || !desc.HasContent() {
			m.Skipped = append(m.Skipped, rel)
			return nil
		}
		m.Bundles = append(m.Bundles, Bundle{JSON: p, Path: bundle, Rel: rel, Desc: desc})
		return nil
	})
	if err != nil {
		m.Close()
		return nil, err
	}
	sort.Slice(m.Bundles, func(i, j int) bool { return m.Bundles[i].Rel < m.Bundles[j].Rel })
	if len(m.Bundles) == 0 {
		m.Close()
		return nil, fmt.Errorf("no Enhanced Prefab Loader content found in %s (looked for X.json files with an asset bundle X beside them)", path)
	}
	return m, nil
}

// Close removes a zip's extracted files.
func (m *Mod) Close() {
	if m.temp != "" {
		os.RemoveAll(m.temp)
	}
}

// extractDir is where Open unpacks an archive: named by its path, size and time, so a changed download unpacks again.
func extractDir(path string, st fs.FileInfo, cacheDir string) string {
	h := sha1.Sum([]byte(fmt.Sprintf("%s|%d|%d", path, st.Size(), st.ModTime().Unix())))
	return filepath.Join(cacheDir, "epl", hex.EncodeToString(h[:8]))
}

// Forget removes the unpacked files of an archive opened with Open (the preview and imports reuse them; once the
// import is done they are only a copy of the download). Nothing to do for a mod folder.
func Forget(path, cacheDir string) {
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		os.RemoveAll(extractDir(path, st, cacheDir))
	}
}

// ClearCache removes every unpacked archive, e.g. ones a preview left behind without an import. The folder is renamed
// first, so the next Open starts clean at once while the old files are deleted in the background.
func ClearCache(cacheDir string) {
	old, _ := filepath.Glob(filepath.Join(cacheDir, "epl-old-*"))
	dir := filepath.Join(cacheDir, "epl")
	if _, err := os.Stat(dir); err == nil {
		gone := fmt.Sprintf("%s-old-%d", dir, time.Now().UnixNano())
		if os.Rename(dir, gone) == nil {
			old = append(old, gone)
		}
	}
	if len(old) == 0 {
		return
	}
	go func() {
		for _, d := range old {
			os.RemoveAll(d)
		}
	}()
}

// extractDescriptors extracts every *.json that has a bundle entry beside it, plus that bundle (other files are skipped,
// e.g. Holographic Overhaul's .chpack). A complete earlier extraction is reused.
func extractDescriptors(zipPath, dest string, progress func(done, total int64)) error {
	done := filepath.Join(dest, ".complete")
	if _, err := os.Stat(done); err == nil {
		return nil
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("can't open %s: %w", filepath.Base(zipPath), err)
	}
	defer zr.Close()
	byName := map[string]*zip.File{}
	for _, f := range zr.File {
		byName[f.Name] = f
	}
	var want []*zip.File
	var total int64
	for _, f := range zr.File {
		if !strings.EqualFold(filepath.Ext(f.Name), ".json") {
			continue
		}
		b := byName[strings.TrimSuffix(f.Name, filepath.Ext(f.Name))]
		if b == nil || b.FileInfo().IsDir() {
			continue
		}
		want = append(want, f, b)
		total += int64(f.UncompressedSize64 + b.UncompressedSize64)
	}
	var n int64
	for _, f := range want {
		name := filepath.FromSlash(f.Name)
		if strings.Contains(name, "..") {
			return fmt.Errorf("bad path in zip: %s", f.Name)
		}
		out := filepath.Join(dest, name)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := extractOne(f, out, func(k int64) {
			if progress != nil {
				progress(n+k, total)
			}
		}); err != nil {
			return err
		}
		n += int64(f.UncompressedSize64)
	}
	return os.WriteFile(done, nil, 0o644)
}

func extractOne(f *zip.File, out string, progress func(int64)) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	w, err := os.Create(out)
	if err != nil {
		return err
	}
	buf := make([]byte, 4<<20)
	var n int64
	for {
		k, rerr := rc.Read(buf)
		if k > 0 {
			if _, err := w.Write(buf[:k]); err != nil {
				w.Close()
				return err
			}
			n += int64(k)
			progress(n)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			w.Close()
			return fmt.Errorf("%s: %w", f.Name, rerr)
		}
	}
	return w.Close()
}

// ModPath turns what the player picked or dropped into the path Open takes: a folder or archive as is; a file inside a mod
// folder (a descriptor .json, a bundle) → the mod's folder: the child of the nearest "plugins" folder above it (where
// BepInEx mods live, e.g. plugins\Corpo Capsula TCG with several X_prefabLoader folders), else the parent of an
// "…_prefabLoader" folder (EPL's naming), else the file's own folder.
func ModPath(p string) (string, error) {
	st, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if st.IsDir() || strings.EqualFold(filepath.Ext(p), ".zip") || isTarArchive(p) {
		return p, nil
	}
	dir := filepath.Dir(p)
	for d := dir; ; {
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		if strings.EqualFold(filepath.Base(parent), "plugins") {
			return d, nil
		}
		d = parent
	}
	if strings.HasSuffix(strings.ToLower(filepath.Base(dir)), "_prefabloader") {
		return filepath.Dir(dir), nil
	}
	return dir, nil
}

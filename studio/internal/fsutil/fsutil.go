// Package fsutil holds the file helpers the workspace packages share (copy, move, atomic write, image names).
package fsutil

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Exists reports whether p exists (file or folder).
func Exists(p string) bool { _, err := os.Stat(p); return err == nil }

// IsImage reports whether name has an image extension a set or accessory can refer to.
func IsImage(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".svg", ".gif", ".bmp":
		return true
	}
	return false
}

// WriteAtomic writes b to p via a temporary file and a rename, so readers never see half a file.
func WriteAtomic(p string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".part"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// CopyFile copies src to dst atomically (temporary file + rename), creating dst's folder. dst keeps src's modification
// time, so size + time comparisons (project.InstallState) see the copy as current.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		if info, serr := in.Stat(); serr == nil {
			_ = os.Chtimes(tmp, info.ModTime(), info.ModTime())
		}
		err = os.Rename(tmp, dst)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}

// MoveFile renames src to dst (creating dst's folder), falling back to copy + delete across volumes.
func MoveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := CopyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// CopyTree copies src into dst. skip (optional) gets slash-separated paths relative to src; a skipped folder is not
// entered.
func CopyTree(src, dst string, skip func(rel string, dir bool) bool) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		if skip != nil && skip(filepath.ToSlash(rel), d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		return CopyFile(p, filepath.Join(dst, rel))
	})
}

// SameContent reports whether two files exist and have the same bytes.
func SameContent(a, b string) bool {
	ia, err := os.Stat(a)
	if err != nil {
		return false
	}
	ib, err := os.Stat(b)
	if err != nil || ia.Size() != ib.Size() {
		return false
	}
	fa, err := os.Open(a)
	if err != nil {
		return false
	}
	defer fa.Close()
	fb, err := os.Open(b)
	if err != nil {
		return false
	}
	defer fb.Close()
	ba, bb := make([]byte, 64<<10), make([]byte, 64<<10)
	for {
		na, ea := io.ReadFull(fa, ba)
		nb, eb := io.ReadFull(fb, bb)
		if na != nb || !bytes.Equal(ba[:na], bb[:nb]) {
			return false
		}
		if ea != nil || eb != nil {
			return ea != nil && eb != nil
		}
	}
}

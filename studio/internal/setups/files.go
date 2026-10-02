package setups

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if info, err := os.Stat(src); err == nil {
		_ = os.Chtimes(dst, info.ModTime(), info.ModTime()) // keeps project.InstallState's mtime check meaningful
	}
	return nil
}

// copyTree copies src into dst. skip gets slash-separated paths relative to src; a skipped folder is not entered.
func copyTree(src, dst string, skip func(rel string, dir bool) bool) error {
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
		return copyFile(p, filepath.Join(dst, rel))
	})
}

// moveFile renames, falling back to copy + delete across volumes (e.g. a workspace on another drive).
func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// moveTree moves every file under src to the same place under dst (replacing files there), then removes the empty folders.
func moveTree(src, dst string) error {
	var dirs []string
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			dirs = append(dirs, p)
			return nil
		}
		return moveFile(p, filepath.Join(dst, rel))
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		_ = os.Remove(dirs[i]) // only succeeds when empty
	}
	return nil
}

// dirEmpty reports whether dir is missing or has no entries.
func dirEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err != nil || len(entries) == 0
}

// isAbsPath reports whether s looks like an absolute Windows path (C:\…, C:/…, \\server\…).
func isAbsPath(s string) bool {
	if len(s) >= 3 && s[1] == ':' && (s[2] == '\\' || s[2] == '/') {
		c := s[0] | 0x20
		return c >= 'a' && c <= 'z'
	}
	return strings.HasPrefix(s, `\\`)
}

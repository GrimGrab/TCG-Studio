// Package ziputil writes and safely unpacks the zip files Studio shares (setup files, item files): entry names checked
// against zip-slip, size and count limits, already-compressed images stored, and machine-specific absolute paths blanked
// from JSON and BepInEx config files.
package ziputil

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"tcgstudio/internal/fsutil"
)

const (
	MaxSize  = 8 << 30 // uncompressed bytes an import may unpack
	MaxFiles = 200_000
)

// Writer builds a zip at dest (via a temp file; Close renames it into place, Abort removes it).
type Writer struct {
	dest, tmp string
	f         *os.File
	zw        *zip.Writer
	names     map[string]bool
}

func Create(dest string) (*Writer, error) {
	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return nil, err
	}
	return &Writer{dest: dest, tmp: tmp, f: f, zw: zip.NewWriter(f), names: map[string]bool{}}, nil
}

// Has reports whether an entry was written already.
func (w *Writer) Has(name string) bool { return w.names[name] }

// Bytes writes one entry. Images and models are stored as they are (already compressed), everything else deflated.
func (w *Writer) Bytes(name string, b []byte) error {
	if w.names[name] {
		return nil
	}
	method := zip.Deflate
	switch strings.ToLower(path.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".glb":
		method = zip.Store
	}
	out, err := w.zw.CreateHeader(&zip.FileHeader{Name: name, Method: method, Modified: time.Now()})
	if err != nil {
		return err
	}
	if _, err := out.Write(b); err != nil {
		return err
	}
	w.names[name] = true
	return nil
}

// File writes a file from disk as entry name.
func (w *Writer) File(name, file string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	return w.Bytes(name, b)
}

// JSON writes v (indented) as entry name.
func (w *Writer) JSON(name string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return w.Bytes(name, b)
}

// Close finishes the zip and moves it to dest.
func (w *Writer) Close() error {
	if err := w.zw.Close(); err != nil {
		w.Abort()
		return err
	}
	if err := w.f.Close(); err != nil {
		_ = os.Remove(w.tmp)
		return err
	}
	_ = os.Remove(w.dest)
	return os.Rename(w.tmp, w.dest)
}

// Abort drops an unfinished zip.
func (w *Writer) Abort() {
	_ = w.f.Close()
	_ = os.Remove(w.tmp)
}

// Open opens a zip for reading and checks its overall size and file count.
func Open(src string) (*zip.ReadCloser, error) {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return nil, err
	}
	if len(zr.File) > MaxFiles {
		zr.Close()
		return nil, errors.New("the file has too many entries")
	}
	var total uint64
	for _, f := range zr.File {
		total += f.UncompressedSize64
	}
	if total > MaxSize {
		zr.Close()
		return nil, errors.New("the file is too large")
	}
	return zr, nil
}

// ReadJSON decodes entry name of zr into v (at most 1 MB); false when the zip has no such entry.
func ReadJSON(zr *zip.Reader, name string, v any) (bool, error) {
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return true, err
		}
		defer rc.Close()
		return true, json.NewDecoder(io.LimitReader(rc, 1<<20)).Decode(v)
	}
	return false, nil
}

// Clean validates a zip entry name: a safe slash path, or an error for absolute paths, drive letters and "..".
func Clean(name string) (string, error) {
	n := strings.ReplaceAll(name, `\`, "/")
	if strings.HasPrefix(n, "/") || strings.Contains(n, ":") {
		return "", fmt.Errorf("unsafe path in file: %q", name)
	}
	for _, part := range strings.Split(n, "/") {
		if part == ".." {
			return "", fmt.Errorf("unsafe path in file: %q", name)
		}
	}
	return path.Clean(n), nil
}

// ExtractAll unpacks every entry keep accepts (nil = all, given the cleaned name) under dir.
func ExtractAll(zr *zip.Reader, dir string, keep func(name string) bool) error {
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		n, err := Clean(f.Name)
		if err != nil {
			return err
		}
		if n == "." || (keep != nil && !keep(n)) {
			continue
		}
		if err := Extract(f, filepath.Join(dir, filepath.FromSlash(n))); err != nil {
			return err
		}
	}
	return nil
}

// Extract writes one entry to dst (folders created), refusing an entry larger than it claims.
func Extract(f *zip.File, dst string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, io.LimitReader(rc, int64(f.UncompressedSize64)+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && uint64(n) > f.UncompressedSize64 {
		err = fmt.Errorf("corrupt entry %q", f.Name)
	}
	return err
}

// IsAbsPath reports a Windows absolute path ("C:\…", "C:/…", "\\server\…").
func IsAbsPath(s string) bool {
	if len(s) >= 3 && s[1] == ':' && (s[2] == '\\' || s[2] == '/') {
		c := s[0] | 0x20
		return c >= 'a' && c <= 'z'
	}
	return strings.HasPrefix(s, `\\`)
}

// ScrubCfg blanks BepInEx config values that are absolute paths (e.g. [MTG] ForgeFolder): they only fit this PC.
func ScrubCfg(b []byte) []byte {
	lines := strings.Split(string(b), "\n")
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "[") {
			continue
		}
		if k, v, ok := strings.Cut(l, "="); ok && IsAbsPath(strings.TrimSpace(v)) {
			lines[i] = strings.TrimRight(k, " ") + " = "
			if strings.HasSuffix(l, "\r") {
				lines[i] += "\r"
			}
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// ScrubJSON blanks absolute-path strings anywhere in a JSON document (studio metadata may remember picked source files).
// Documents that don't parse are returned unchanged.
func ScrubJSON(b []byte) []byte {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return b
	}
	changed := false
	v = fsutil.MapStrings(v, func(s string) string {
		if IsAbsPath(s) {
			changed = true
			return ""
		}
		return s
	})
	if !changed {
		return b
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return b
	}
	return out.Bytes()
}

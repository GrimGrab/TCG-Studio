package setups

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

	"tcgstudio/internal/project"
)

// Ext is the file extension of an exported setup (a zip).
const Ext = ".tcgsetup"

const (
	manifestFile  = "manifest.json"
	archiveFormat = "tcgsetup"
	maxImportSize = 8 << 30 // uncompressed bytes
	maxImportN    = 200_000 // files
)

// Manifest describes an exported setup (manifest.json at the root of the zip).
type Manifest struct {
	Format        string    `json:"format"`
	FormatVersion int       `json:"formatVersion"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	StudioVersion string    `json:"studioVersion,omitempty"`
	Exported      time.Time `json:"exported"`
	Sets          int       `json:"sets"`
	Accessories   int       `json:"accessories"`
	Furniture     int       `json:"furniture"`
}

// exportable reports whether a path inside a setup goes into an export: setup.json, projects\, accessories\ and game\ —
// never saves\ (or a saves-conflict-* folder) or leftovers of an interrupted write.
func exportable(rel string) bool {
	top, _, _ := strings.Cut(rel, "/")
	switch top {
	case InfoFile, "projects", "accessories", GameStateDir:
	default:
		return false
	}
	return !strings.HasSuffix(rel, ".tmp") && !strings.HasSuffix(rel, ".installing")
}

// Export writes setup id to dest as a zip. The caller snapshots the active setup first. Machine-specific values (absolute
// paths in the mod config and studio metadata) are blanked; progress gets (files done, total).
func (h Home) Export(id, dest, studioVersion string, progress func(done, total int)) error {
	dir := h.Dir(id)
	in, err := LoadInfo(dir)
	if err != nil {
		return err
	}
	var files []string
	err = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if !exportable(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Card art in the shared library goes into the file at the project path it is used under, so the format is unchanged
	// and the receiver needs no library.
	src := map[string]string{} // zip name → file outside the setup folder
	have := map[string]bool{}
	for _, f := range files {
		have[f] = true
	}
	ws := project.Workspace{Root: dir, Library: project.LibraryDir(h.Root)}
	projects, _ := os.ReadDir(filepath.Join(dir, "projects"))
	for _, e := range projects {
		p, err := ws.Load(e.Name())
		if err != nil {
			continue
		}
		for _, c := range p.Set.Cards {
			name := "projects/" + e.Name() + "/" + c.Image
			if c.Image == "" || have[name] || !p.InLibrary(c.Image) {
				continue
			}
			have[name] = true
			files = append(files, name)
			src[name] = p.ImagePath(c.Image)
		}
	}
	m := Manifest{Format: archiveFormat, FormatVersion: FormatVersion, Name: in.Name, Description: in.Description,
		StudioVersion: studioVersion, Exported: time.Now().UTC()}
	if list, err := h.List(); err == nil {
		for _, s := range list {
			if s.ID == id {
				m.Sets, m.Accessories, m.Furniture = s.Sets, s.Accessories, s.Furniture
			}
		}
	}

	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			f.Close()
			_ = os.Remove(tmp)
		}
	}()
	zw := zip.NewWriter(f) // buffers internally; Close flushes
	mb, _ := json.MarshalIndent(m, "", "  ")
	if err := writeEntry(zw, manifestFile, mb); err != nil {
		return err
	}
	for i, rel := range files {
		file := filepath.Join(dir, filepath.FromSlash(rel))
		if f, ok := src[rel]; ok {
			file = f
		}
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		switch {
		case rel == GameStateDir+"/"+cfgFile:
			b = scrubCfg(b)
		case path.Base(rel) == "studio.json":
			b = scrubJSON(b)
		}
		if err := writeEntry(zw, rel, b); err != nil {
			return err
		}
		if progress != nil && (i%25 == 0 || i == len(files)-1) {
			progress(i+1, len(files))
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	ok = true
	_ = os.Remove(dest)
	return os.Rename(tmp, dest)
}

func writeEntry(zw *zip.Writer, name string, b []byte) error {
	method := zip.Deflate
	switch strings.ToLower(path.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".glb":
		method = zip.Store // already compressed
	}
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: method, Modified: time.Now()})
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// scrubCfg blanks BepInEx config values that are absolute paths (e.g. [MTG] ForgeFolder) — they only fit this PC.
func scrubCfg(b []byte) []byte {
	lines := strings.Split(string(b), "\n")
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "[") {
			continue
		}
		if k, v, ok := strings.Cut(l, "="); ok && isAbsPath(strings.TrimSpace(v)) {
			lines[i] = strings.TrimRight(k, " ") + " = "
			if strings.HasSuffix(l, "\r") {
				lines[i] += "\r"
			}
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// scrubJSON blanks absolute-path strings anywhere in a JSON document (studio metadata may remember picked source files).
// Documents that don't parse are exported unchanged.
func scrubJSON(b []byte) []byte {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return b
	}
	changed := false
	var walk func(any) any
	walk = func(x any) any {
		switch t := x.(type) {
		case map[string]any:
			for k, e := range t {
				t[k] = walk(e)
			}
		case []any:
			for i, e := range t {
				t[i] = walk(e)
			}
		case string:
			if isAbsPath(t) {
				changed = true
				return ""
			}
		}
		return x
	}
	v = walk(v)
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

// ReadManifest returns the manifest of an exported setup without importing it.
func ReadManifest(src string) (*Manifest, error) {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return nil, fmt.Errorf("not a setup file: %w", err)
	}
	defer zr.Close()
	return manifestOf(&zr.Reader)
}

func manifestOf(zr *zip.Reader) (*Manifest, error) {
	for _, f := range zr.File {
		if f.Name != manifestFile {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		m := &Manifest{}
		if err := json.NewDecoder(io.LimitReader(rc, 1<<20)).Decode(m); err != nil {
			return nil, fmt.Errorf("bad manifest: %w", err)
		}
		if m.Format != archiveFormat {
			return nil, errors.New("not a TCG Studio setup file")
		}
		if m.FormatVersion > FormatVersion {
			return nil, fmt.Errorf("this setup was made with a newer TCG Studio (%s) — update TCG Studio first", m.StudioVersion)
		}
		return m, nil
	}
	return nil, errors.New("not a TCG Studio setup file (no manifest)")
}

// cleanEntry validates a zip entry name and returns it as a safe slash path, or "" to skip it.
func cleanEntry(name string) (string, error) {
	n := strings.ReplaceAll(name, `\`, "/")
	if strings.HasPrefix(n, "/") || strings.Contains(n, ":") {
		return "", fmt.Errorf("unsafe path in setup file: %q", name)
	}
	for _, part := range strings.Split(n, "/") {
		if part == ".." {
			return "", fmt.Errorf("unsafe path in setup file: %q", name)
		}
	}
	n = path.Clean(n)
	if n == "." || n == manifestFile || !exportable(n) {
		return "", nil // saves, unknown folders: ignored
	}
	return n, nil
}

// Import unpacks an exported setup as a new, inactive setup (with no saves) and returns its id.
func (h Home) Import(src, studioVersion string) (string, error) {
	r, err := h.Registry()
	if err != nil {
		return "", err
	}
	zr, err := zip.OpenReader(src)
	if err != nil {
		return "", fmt.Errorf("not a setup file: %w", err)
	}
	defer zr.Close()
	m, err := manifestOf(&zr.Reader)
	if err != nil {
		return "", err
	}
	if len(zr.File) > maxImportN {
		return "", errors.New("setup file has too many files")
	}
	var total uint64
	for _, f := range zr.File {
		total += f.UncompressedSize64
	}
	if total > maxImportSize {
		return "", errors.New("setup file is too large")
	}

	name := h.uniqueName(m.Name)
	id := h.newID(name)
	tmp := h.Dir(id) + ".importing"
	_ = os.RemoveAll(tmp)
	fail := func(err error) (string, error) { _ = os.RemoveAll(tmp); return "", err }
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rel, err := cleanEntry(f.Name)
		if err != nil {
			return fail(err)
		}
		if rel == "" {
			continue
		}
		if err := extract(f, filepath.Join(tmp, filepath.FromSlash(rel))); err != nil {
			return fail(err)
		}
	}
	in, err := LoadInfo(tmp)
	if err != nil {
		in = &Info{}
	}
	in.Name, in.Description, in.Created, in.StudioVersion, in.FormatVersion = name, m.Description, time.Now(), studioVersion, FormatVersion
	if in.InstalledSets == nil {
		in.InstalledSets = []string{}
	}
	if err := SaveInfo(tmp, in); err != nil {
		return fail(err)
	}
	if err := os.Rename(tmp, h.Dir(id)); err != nil {
		return fail(err)
	}
	r.Order = h.knownIDs(append(r.Order, id))
	return id, h.saveRegistry(r)
}

func extract(f *zip.File, dst string) error {
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

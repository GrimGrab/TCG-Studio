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

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/workspace"
)

// Ext is the file extension of an exported setup (a zip).
const Ext = ".tcgsetup"

const (
	manifestFile  = "manifest.json"
	archiveFormat = "tcgsetup"
	// ArchiveVersion is the export format: 2 = set files under library/<setId>/ (1 = inside projects/<setId>/, still read).
	ArchiveVersion = 2
	// libraryEntry is the zip folder of the exported sets' files.
	libraryEntry  = "library"
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
	Decorations   int       `json:"decorations,omitempty"`
}

// exportable reports whether a path inside a setup goes into an export: setup.json, projects\, accessories\ and game\ (minus
// parked game-folder content) —
// never saves\ (or a saves-conflict-* folder) or leftovers of an interrupted write. An export also carries library/ (the
// sets' shared files).
func exportable(rel string) bool {
	top, _, _ := strings.Cut(rel, "/")
	switch top {
	case InfoFile, "projects", "accessories", GameStateDir, libraryEntry:
	default:
		return false
	}
	if top == GameStateDir {
		// Older Studio versions parked hand-installed sets and accessories (often converted mods) there; they stay on this PC.
		_, sub, _ := strings.Cut(rel, "/")
		first, _, _ := strings.Cut(sub, "/")
		if first == "Sets" || first == "Accessories" || first == "Library" {
			return false
		}
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
	// Every file a set uses comes from its shared folder: it goes into library/<setId>/ and the receiver's import puts it
	// into its own shared folder.
	src := map[string]string{} // zip name → file outside the setup folder
	have := map[string]bool{}
	ws := workspace.Dir(h.Root).Projects(dir)
	projects, _ := os.ReadDir(filepath.Join(dir, "projects"))
	converted := map[string]bool{} // sets converted from another mod (EPL) carry its authors' art: they stay on this PC
	for _, e := range projects {
		p, err := ws.Load(e.Name())
		if err != nil {
			continue
		}
		if p.Meta != nil && p.Meta.Source == "epl" {
			converted[e.Name()] = true
			continue
		}
		for _, rel := range p.Files() {
			name := libraryEntry + "/" + e.Name() + "/" + rel
			if !have[name] && fsutil.Exists(p.ImagePath(rel)) {
				have[name] = true
				files = append(files, name)
				src[name] = p.ImagePath(rel)
			}
		}
	}
	{ // converted sets stay on this PC
		kept := files[:0]
		for _, f := range files {
			if top, rest, ok := strings.Cut(f, "/"); ok && top == "projects" {
				if id, _, _ := strings.Cut(rest, "/"); converted[id] {
					continue
				}
			}
			kept = append(kept, f)
		}
		files = kept
	}
	// Accessory and furniture files in the shared store go into the zip under accessories/assets/, where the receiver's
	// import puts them back into its own store (converted mod content stays out, like its entries).
	if lib, err := workspace.Dir(h.Root).Items(dir); err == nil {
		for _, rel := range lib.FilesWhere(func(id string) bool { return !accessories.IsConverted(id) }) {
			name := "accessories/" + rel
			if !assets.IsAsset(rel) || have[name] {
				continue
			}
			have[name] = true
			files = append(files, name)
			src[name] = lib.Assets.Path(rel)
		}
	}
	m := Manifest{Format: archiveFormat, FormatVersion: ArchiveVersion, Name: in.Name, Description: in.Description,
		StudioVersion: studioVersion, Exported: time.Now().UTC()}
	if list, err := h.List(); err == nil {
		for _, s := range list {
			if s.ID == id {
				m.Sets, m.Accessories, m.Furniture, m.Decorations = s.Sets-len(converted), s.Accessories, s.Furniture, s.Decorations
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
		case rel == "accessories/"+accessories.LibraryFile:
			b = withoutConverted(b)
		case rel == InfoFile && len(converted) > 0:
			b = withoutInstalled(b, converted)
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
	v = fsutil.MapStrings(v, func(s string) string {
		if isAbsPath(s) {
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
		if m.FormatVersion > ArchiveVersion {
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

// Import unpacks an exported setup as a new, inactive setup (with no saves) and returns its id. The sets' files go into
// the workspace's shared folders; where this PC already has a different file for a set, its own file is kept (returned
// in kept as "<setId>/<path>").
func (h Home) Import(src, studioVersion string) (id string, kept []string, err error) {
	r, err := h.Registry()
	if err != nil {
		return "", nil, err
	}
	zr, err := zip.OpenReader(src)
	if err != nil {
		return "", nil, fmt.Errorf("not a setup file: %w", err)
	}
	defer zr.Close()
	m, err := manifestOf(&zr.Reader)
	if err != nil {
		return "", nil, err
	}
	if len(zr.File) > maxImportN {
		return "", nil, errors.New("setup file has too many files")
	}
	var total uint64
	for _, f := range zr.File {
		total += f.UncompressedSize64
	}
	if total > maxImportSize {
		return "", nil, errors.New("setup file is too large")
	}

	name := h.uniqueName(m.Name)
	id = h.newID(name)
	tmp := h.Dir(id) + ".importing"
	_ = os.RemoveAll(tmp)
	fail := func(err error) (string, []string, error) { _ = os.RemoveAll(tmp); return "", nil, err }
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
	// Shared accessory files travel in the zip under accessories/assets/: they go into this workspace's store (once).
	if err := absorbAssets(filepath.Join(tmp, "accessories", assets.DirName), assets.For(h.Root)); err != nil {
		return fail(err)
	}
	if kept, err = absorbSetFiles(tmp, workspace.Dir(h.Root).Library()); err != nil {
		return fail(err)
	}
	if err := os.Rename(tmp, h.Dir(id)); err != nil {
		return fail(err)
	}
	r.Order = h.knownIDs(append(r.Order, id))
	return id, kept, h.saveRegistry(r)
}

// absorbSetFiles moves the set files of an imported setup (tmp = its folder) into the workspace's shared folders
// (library = <workspace>\library): library/<setId>/…, and files an older export kept in projects/<setId>/ next to
// set.json. A file the shared folder already has with other bytes stays as it is (returned). Removes tmp's library/.
func absorbSetFiles(tmp, library string) (kept []string, err error) {
	type file struct{ src, set, rel string }
	var files []file
	walk := func(root string, skip func(rel string) bool) error {
		sets, _ := os.ReadDir(root)
		for _, s := range sets {
			if !s.IsDir() || !setfmt.SafeID(s.Name()) {
				continue
			}
			base := filepath.Join(root, s.Name())
			err := filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(base, p)
				if rel = filepath.ToSlash(rel); !skip(rel) {
					files = append(files, file{p, s.Name(), rel})
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(filepath.Join(tmp, libraryEntry), func(string) bool { return false }); err != nil {
		return nil, err
	}
	if err := walk(filepath.Join(tmp, "projects"), func(rel string) bool { return rel == "set.json" || rel == "studio.json" }); err != nil {
		return nil, err
	}
	for _, f := range files {
		dst := filepath.Join(library, f.set, filepath.FromSlash(f.rel))
		switch {
		case !fsutil.Exists(dst):
			if err := fsutil.MoveFile(f.src, dst); err != nil {
				return kept, err
			}
		case fsutil.SameContent(f.src, dst):
			_ = os.Remove(f.src)
		default:
			kept = append(kept, f.set+"/"+f.rel)
			_ = os.Remove(f.src)
		}
	}
	projects, _ := os.ReadDir(filepath.Join(tmp, "projects"))
	for _, d := range projects {
		if d.IsDir() {
			removeEmptySubdirs(filepath.Join(tmp, "projects", d.Name()))
		}
	}
	return kept, os.RemoveAll(filepath.Join(tmp, libraryEntry))
}

// removeEmptySubdirs deletes the empty folders under dir (not dir itself).
func removeEmptySubdirs(dir string) {
	des, _ := os.ReadDir(dir)
	for _, d := range des {
		if d.IsDir() {
			sub := filepath.Join(dir, d.Name())
			removeEmptySubdirs(sub)
			_ = os.Remove(sub) // only succeeds when empty
		}
	}
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

// withoutConverted drops accessories and furniture converted from another mod (their art belongs to its authors) from an
// accessories.json; anything unreadable is passed through unchanged.
func withoutConverted(b []byte) []byte {
	var lib setfmt.AccessoryLibrary
	if json.Unmarshal(b, &lib) != nil {
		return b
	}
	n := len(lib.Accessories) + len(lib.Furniture) + len(lib.Decorations)
	acc := lib.Accessories[:0]
	for _, a := range lib.Accessories {
		if !accessories.IsConverted(a.ID) {
			acc = append(acc, a)
		}
	}
	fur := lib.Furniture[:0]
	for _, f := range lib.Furniture {
		if !accessories.IsConverted(f.ID) {
			fur = append(fur, f)
		}
	}
	deco := lib.Decorations[:0]
	for _, d := range lib.Decorations {
		if !accessories.IsConverted(d.ID) {
			deco = append(deco, d)
		}
	}
	if len(acc)+len(fur)+len(deco) == n {
		return b
	}
	lib.Accessories, lib.Furniture, lib.Decorations = acc, fur, deco
	out, err := json.MarshalIndent(lib, "", "  ")
	if err != nil {
		return b
	}
	return out
}

// absorbAssets moves the store files of an imported setup (dir = its accessories/assets folder) into the workspace's
// store and removes the folder. A file whose content doesn't match its name is stored under its real name; references to
// it then simply resolve to nothing, as a missing file would.
func absorbAssets(dir string, store assets.Store) error {
	des, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, d := range des {
		if d.IsDir() {
			continue
		}
		if _, err := store.PutFile(filepath.Join(dir, d.Name())); err != nil {
			return err
		}
	}
	return os.RemoveAll(dir)
}

// withoutInstalled drops sets that stay on this PC from an exported setup.json's installed sets (else the importing game
// looks for projects that aren't there).
func withoutInstalled(b []byte, drop map[string]bool) []byte {
	in := &Info{}
	if json.Unmarshal(b, in) != nil {
		return b
	}
	kept := []string{}
	for _, id := range in.InstalledSets {
		if !drop[id] {
			kept = append(kept, id)
		}
	}
	in.InstalledSets = kept
	out, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return b
	}
	return out
}

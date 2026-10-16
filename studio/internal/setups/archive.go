package setups

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tcgstudio/internal/assets"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/workspace"
	"tcgstudio/internal/ziputil"
)

// Ext is the file extension of an exported setup (a zip).
const Ext = ".tcgsetup"

// Setup files are written by internal/share (format version 3: every set and item through the content registry). This
// file reads the older versions: 2 = set files under library/<setId>/, 1 = inside projects/<setId>/.
const (
	manifestFile = "manifest.json"
	// Format is the manifest's "format" of a setup file.
	Format = "tcgsetup"
	// LegacyVersion is the newest format version Import reads (newer files go through internal/share).
	LegacyVersion = 2
	// libraryEntry is the zip folder of the exported sets' files.
	libraryEntry = "library"
)

// Manifest is manifest.json of an older setup file.
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

// exportable reports whether an entry of an older setup file is imported: setup.json, projects\, accessories\, game\
// (minus parked game-folder content) and library/ (the sets' shared files) — never saves\ or leftovers of an interrupted
// write.
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
		if m.Format != Format {
			return nil, errors.New("not a TCG Studio setup file")
		}
		if m.FormatVersion > LegacyVersion {
			return nil, fmt.Errorf("this setup was made with a newer TCG Studio (%s) — update TCG Studio first", m.StudioVersion)
		}
		return m, nil
	}
	return nil, errors.New("not a TCG Studio setup file (no manifest)")
}

// cleanEntry validates a zip entry name and returns it as a safe slash path, or "" to skip it.
func cleanEntry(name string) (string, error) {
	n, err := ziputil.Clean(name)
	if err != nil {
		return "", err
	}
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
	zr, err := ziputil.Open(src)
	if err != nil {
		return "", nil, fmt.Errorf("not a setup file: %w", err)
	}
	defer zr.Close()
	m, err := manifestOf(&zr.Reader)
	if err != nil {
		return "", nil, err
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
		if err := ziputil.Extract(f, filepath.Join(tmp, filepath.FromSlash(rel))); err != nil {
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

// AddImported creates a new, inactive setup (no saves) named after name (made unique, "X (2)") from a setup file:
// fill writes the setup's content into a temporary folder (setup.json may come from the file; its name, description,
// dates and format are set here), which then becomes the setup. Returns the new setup's id.
func (h Home) AddImported(name, description, studioVersion string, fill func(dir string) error) (string, error) {
	r, err := h.Registry()
	if err != nil {
		return "", err
	}
	name = h.uniqueName(name)
	id := h.newID(name)
	tmp := h.Dir(id) + ".importing"
	_ = os.RemoveAll(tmp)
	fail := func(err error) (string, error) { _ = os.RemoveAll(tmp); return "", err }
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return fail(err)
	}
	if err := fill(tmp); err != nil {
		return fail(err)
	}
	in, err := LoadInfo(tmp)
	if err != nil {
		in = &Info{}
	}
	in.Name, in.Description, in.Created, in.StudioVersion, in.FormatVersion = name, description, time.Now(), studioVersion, FormatVersion
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

// CfgFile is the mod's config file name inside a setup's game folder.
const CfgFile = cfgFile

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

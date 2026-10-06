// Package assets is the shared, content-addressed store for accessory and furniture files (textures, icons, models and
// the editors' source images): <workspace>\assets\<hash>.<ext>. Files are immutable and named by their content, so a file
// is stored once however many setups use it. Setups keep only the game info (accessories.json, studio.json layouts),
// which refers to a file as "assets/<hash>.<ext>"; paths not starting with "assets/" are per-setup files.
package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DirName is the store folder in the workspace (outside library\, whose unknown folders the Storage page treats as
// unused sets).
const DirName = "assets"

// Prefix starts every store reference in game info.
const Prefix = DirName + "/"

type Store struct{ Dir string }

// For is the store of a workspace.
func For(workspace string) Store { return Store{Dir: filepath.Join(workspace, DirName)} }

// IsAsset reports a store reference ("assets/<hash>.<ext>").
func IsAsset(rel string) bool {
	rel = filepath.ToSlash(rel)
	return strings.HasPrefix(rel, Prefix) && !strings.Contains(rel[len(Prefix):], "/") && !strings.Contains(rel, "..")
}

// Path is the file of a store reference ("" when rel isn't one).
func (s Store) Path(rel string) string {
	if !IsAsset(rel) {
		return ""
	}
	return filepath.Join(s.Dir, filepath.Base(filepath.FromSlash(rel)))
}

// Put stores b (once: identical content gives the same name) and returns its reference. ext is like ".png".
func (s Store) Put(b []byte, ext string) (string, error) {
	ext = strings.ToLower(ext)
	if ext == "" || strings.ContainsAny(ext, `/\`) {
		return "", fmt.Errorf("bad asset extension %q", ext)
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	sum := sha256.Sum256(b)
	name := hex.EncodeToString(sum[:8]) + ext
	path := filepath.Join(s.Dir, name)
	if st, err := os.Stat(path); err == nil && st.Size() == int64(len(b)) {
		return Prefix + name, nil
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(s.Dir, name+".*.part")
	if err != nil {
		return "", err
	}
	_, werr := tmp.Write(b)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(tmp.Name())
		if werr != nil {
			return "", werr
		}
		return "", cerr
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		if _, serr := os.Stat(path); serr == nil { // written by someone else meanwhile: same content
			return Prefix + name, nil
		}
		return "", err
	}
	return Prefix + name, nil
}

// PutFile stores a file's content (its extension kept).
func (s Store) PutFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return s.Put(b, filepath.Ext(path))
}

// Entry is one stored file.
type Entry struct {
	Rel  string
	Size int64
}

// List returns every stored file (leftover .part files of an interrupted write are skipped).
func (s Store) List() ([]Entry, error) {
	des, err := os.ReadDir(s.Dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, d := range des {
		if d.IsDir() || strings.HasSuffix(d.Name(), ".part") {
			continue
		}
		info, err := d.Info()
		if err != nil {
			continue
		}
		out = append(out, Entry{Rel: Prefix + d.Name(), Size: info.Size()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out, nil
}

// Refs returns every store reference found in a JSON document (any string value, at any depth), deduplicated.
func Refs(doc []byte) []string {
	var v any
	if json.Unmarshal(doc, &v) != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			if IsAsset(t) && !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		case []any:
			for _, x := range t {
				walk(x)
			}
		case map[string]any:
			for _, x := range t {
				walk(x)
			}
		}
	}
	walk(v)
	sort.Strings(out)
	return out
}

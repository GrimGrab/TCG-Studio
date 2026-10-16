package upgrade

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

// Preview is one file of a differing copy or a named folder next to the shared file it would replace or give way to.
type Preview struct {
	File   string `json:"file"`
	Own    string `json:"own"`    // data: URL
	Shared string `json:"shared"` // data: URL ("" when the shared art has no such file)
}

// PreviewDiffer shows the sample file of a Differ (by key).
func PreviewDiffer(h setups.Home, key string) (*Preview, error) {
	bySet, _, err := sets(h)
	if err != nil {
		return nil, err
	}
	setup, set, _ := strings.Cut(key, "/")
	claimed := map[string]string{} // rel → the file Run shares first (as in Check)
	for _, l := range bySet[set] {
		for _, rel := range ownFiles(l.p) {
			f := own(l.p, rel)
			shared := ""
			if cp := counterpart(l.p, rel); cp != "" {
				shared = lib(l.p, cp)
			} else if claimed[rel] != "" {
				shared = claimed[rel]
			} else {
				claimed[rel] = f
				continue
			}
			if l.setup != setup || fsutil.SameContent(f, shared) {
				continue
			}
			ownURL, err := dataURL(f)
			if err != nil {
				return nil, err
			}
			sharedURL, _ := dataURL(shared)
			return &Preview{File: rel, Own: ownURL, Shared: sharedURL}, nil
		}
	}
	if _, ok := bySet[set]; ok {
		return nil, errors.New("nothing differs")
	}
	return nil, errors.New("set not found in that setup")
}

// PreviewNamed shows a named folder's sample file (Named.Sample) next to the set's shared file.
func PreviewNamed(h setups.Home, set, name, sample string) (*Preview, error) {
	base := filepath.Join(workspace.Dir(h.Root).Library(), set)
	if !setfmt.SafeID(set) || strings.Contains(name+"/"+sample, "..") {
		return nil, errors.New("bad path")
	}
	own, err := dataURL(filepath.Join(base, name, filepath.FromSlash(sample)))
	if err != nil {
		return nil, err
	}
	shared, _ := dataURL(filepath.Join(base, filepath.FromSlash(sample)))
	return &Preview{File: sample, Own: own, Shared: shared}, nil
}

func dataURL(f string) (string, error) {
	b, err := os.ReadFile(f)
	if err != nil {
		return "", err
	}
	mime := map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp",
		".svg": "image/svg+xml", ".gif": "image/gif"}[strings.ToLower(filepath.Ext(f))]
	if mime == "" {
		mime = "application/octet-stream"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b), nil
}

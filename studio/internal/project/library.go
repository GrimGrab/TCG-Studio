package project

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// LibMetaFile describes a set's folder in the shared card-art library: which import made the art, so another setup's
// import of the same set only shares it when it would make the same files.
const LibMetaFile = "library.json"

type LibMeta struct {
	Source string `json:"source"`
	Code   string `json:"code"`
	Lang   string `json:"lang,omitempty"`
	Width  int    `json:"width"`  // the import's image width (0 = original size)
	Format string `json:"format"` // "png" or "jpg"
}

// LoadLibMeta reads dir's library.json (nil when there is none).
func LoadLibMeta(dir string) *LibMeta {
	b, err := os.ReadFile(filepath.Join(dir, LibMetaFile))
	if err != nil {
		return nil
	}
	m := &LibMeta{}
	if json.Unmarshal(b, m) != nil {
		return nil
	}
	if m.Format == "" {
		m.Format = "png"
	}
	return m
}

func SaveLibMeta(dir string, m *LibMeta) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, LibMetaFile+".tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, LibMetaFile))
}

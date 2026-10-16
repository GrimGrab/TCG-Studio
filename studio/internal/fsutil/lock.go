package fsutil

import (
	"path/filepath"
	"strings"
	"sync"
)

var fileLocks sync.Map // cleaned lower-case path → *sync.Mutex

// WithLock runs f while holding the process-wide lock for path, so read-modify-write edits of one file (a setup's
// setup.json is edited by installs, renames, the catalog and the upgrade) never lose each other's changes.
func WithLock(path string, f func() error) error {
	key := strings.ToLower(filepath.Clean(path))
	m, _ := fileLocks.LoadOrStore(key, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	return f()
}

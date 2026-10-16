package upgrade

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/workspace"
)

// Every file the upgrade would otherwise delete or overwrite goes to workspace.Dir.Kept()\<time>\, under its path inside
// the workspace. The upgrade only ever deletes a file outright when an identical copy stays (a set file already in the
// shared library, an item file now in the store), so nobody's work is lost to it; the Storage page shows what was kept
// and deletes it on request.

// discard removes f when copy holds the same bytes, else keeps it (see workspace.Dir.Kept).
func (r *Result) discard(f, copy string) error {
	if copy != "" && fsutil.SameContent(f, copy) {
		n := size(f)
		if err := os.Remove(f); err != nil {
			return err
		}
		r.Files++
		r.Freed += n
		return nil
	}
	return r.keep(f, false)
}

// keepCopy keeps a copy of f before it is overwritten (nothing to do when it doesn't exist or already has new's bytes).
func (r *Result) keepCopy(f, replacement string) error {
	if !fsutil.Exists(f) || fsutil.SameContent(f, replacement) {
		return nil
	}
	return r.keep(f, true)
}

func (r *Result) keep(f string, copy bool) error {
	if r.keptRun == "" {
		r.keptRun = filepath.Join(workspace.Dir(r.root).Kept(), time.Now().Format("20060102-150405"))
	}
	rel, err := filepath.Rel(r.root, f)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		rel = filepath.Join("outside", filepath.Base(filepath.Dir(f)), filepath.Base(f)) // e.g. the game folder
	}
	dst := filepath.Join(r.keptRun, rel)
	n := size(f)
	if copy {
		err = fsutil.CopyFile(f, dst)
	} else {
		err = fsutil.MoveFile(f, dst)
	}
	if err != nil {
		return err
	}
	r.Kept++
	r.KeptBytes += n
	r.KeptDir = r.keptRun
	return nil
}

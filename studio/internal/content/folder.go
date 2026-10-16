package content

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/origin"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/workspace"
	"tcgstudio/internal/ziputil"
)

// Folder stores sets: a project folder per set in the root, and the set's files in the workspace's shared library.
type Folder struct{}

const (
	folderProject = "project/" // inside an item's zip folder: set.json + studio.json
	folderLibrary = "library/" // the set's files, by their path in the set
)

func (Folder) ids(ws workspace.Dir, root string) []string {
	pw := ws.Projects(root)
	des, err := os.ReadDir(pw.ProjectsDir())
	if err != nil {
		return nil
	}
	var out []string
	for _, d := range des {
		if d.IsDir() && !strings.Contains(d.Name(), ".") && fsutil.Exists(filepath.Join(pw.Folder(d.Name()), project.SetFile)) {
			out = append(out, d.Name())
		}
	}
	return out
}

func (f Folder) List(ws workspace.Dir, root string) ([]Item, error) {
	pw := ws.Projects(root)
	var out []Item
	for _, id := range f.ids(ws, root) {
		p, err := pw.Load(id)
		if err != nil {
			continue
		}
		it := Item{ID: id, Name: p.Set.Name, Source: p.Meta.Source, Origin: p.Meta.Origin, Converted: p.Meta.Source == "epl",
			Detail: fmt.Sprintf("%d cards · %d pack%s", len(p.Set.Cards), len(p.Set.Packs), plural(len(p.Set.Packs)))}
		if it.Origin == nil {
			if n := p.Meta.OriginName(); n != "" { // converted before origins were recorded
				it.Origin = &origin.Origin{Kind: origin.KindEPL, Mod: n}
			}
		}
		for _, c := range p.Set.Cards {
			if c.Image != "" {
				it.Cover = c.Image
				break
			}
		}
		out = append(out, it)
	}
	return out, nil
}

func (f Folder) Copy(ws workspace.Dir, from, to string, ids []string) ([]string, error) {
	src, dst := ws.Projects(from), ws.Projects(to)
	var done []string
	for _, id := range ids {
		if !setfmt.SafeID(id) || !fsutil.Exists(filepath.Join(src.Folder(id), project.SetFile)) {
			continue
		}
		target := dst.Folder(id)
		tmp := target + ".adding"
		_ = os.RemoveAll(tmp)
		if err := fsutil.CopyTree(src.Folder(id), tmp, nil); err != nil {
			_ = os.RemoveAll(tmp)
			return done, err
		}
		_ = os.RemoveAll(target)
		if err := os.Rename(tmp, target); err != nil {
			_ = os.RemoveAll(tmp)
			return done, err
		}
		done = append(done, id)
	}
	return done, nil
}

func (f Folder) Remove(ws workspace.Dir, root string, ids []string) (int, []string, error) {
	pw := ws.Projects(root)
	n := 0
	for _, id := range ids {
		if !setfmt.SafeID(id) || !fsutil.Exists(pw.Folder(id)) {
			continue
		}
		if err := pw.Uninstall(id); err != nil {
			return n, nil, err
		}
		if err := pw.Delete(id); err != nil {
			return n, nil, err
		}
		n++
	}
	return n, nil, nil
}

func (f Folder) Cleanup(ws workspace.Dir, roots, ids, _ []string) []string {
	var gone []string
	for _, id := range ids {
		if !setfmt.SafeID(id) {
			continue
		}
		used := false
		for _, root := range roots {
			if fsutil.Exists(filepath.Join(ws.Projects(root).Folder(id), project.SetFile)) {
				used = true // there (readable or not: keep its files)
				break
			}
		}
		if !used {
			_ = os.RemoveAll(filepath.Join(ws.Library(), id))
			gone = append(gone, id)
		}
	}
	return gone
}

func (f Folder) Export(ws workspace.Dir, root, id, prefix string, z *ziputil.Writer) error {
	pw := ws.Projects(root)
	p, err := pw.Load(id)
	if err != nil {
		return err
	}
	for _, name := range []string{project.SetFile, project.MetaFile} {
		b, err := os.ReadFile(filepath.Join(p.Folder, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if err := z.Bytes(prefix+folderProject+name, ziputil.ScrubJSON(b)); err != nil {
			return err
		}
	}
	for _, rel := range p.Files() {
		if file := p.ImagePath(rel); fsutil.Exists(file) {
			if err := z.File(prefix+folderLibrary+rel, file); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f Folder) Import(ws workspace.Dir, root string, in []In) error {
	pw := ws.Projects(root)
	for _, it := range in {
		if !setfmt.SafeID(it.NewID) {
			return fmt.Errorf("bad set id %q", it.NewID)
		}
		target := pw.Folder(it.NewID)
		if fsutil.Exists(target) {
			return fmt.Errorf("set %s already exists", it.NewID)
		}
		src := filepath.Join(it.Dir, filepath.FromSlash(strings.TrimSuffix(folderProject, "/")))
		if !fsutil.Exists(filepath.Join(src, project.SetFile)) {
			return fmt.Errorf("set %s: no set.json in the file", it.NewID)
		}
		if err := fsutil.CopyTree(src, target, nil); err != nil {
			return err
		}
		p, err := pw.Load(it.NewID)
		if err != nil {
			_ = os.RemoveAll(target)
			return err
		}
		p.Set.ID, p.Meta.Origin = it.NewID, it.Origin
		if err := pw.Save(p); err != nil {
			return err
		}
		// The set's files go into its shared folder (new for a new id; files already there are kept).
		lib := filepath.Join(it.Dir, filepath.FromSlash(strings.TrimSuffix(folderLibrary, "/")))
		err = filepath.WalkDir(lib, func(file string, d os.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return filepath.SkipAll
			}
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(lib, file)
			rel = path.Clean(filepath.ToSlash(rel))
			if !project.IsRelFile(rel) {
				return nil
			}
			if dst := p.ImagePath(rel); !fsutil.Exists(dst) {
				return fsutil.MoveFile(file, dst)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (Folder) Taken(ws workspace.Dir, id string) bool {
	return fsutil.Exists(filepath.Join(ws.Library(), id))
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

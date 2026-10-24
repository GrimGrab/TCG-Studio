package content

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/origin"
	"tcgstudio/internal/workspace"
	"tcgstudio/internal/ziputil"
)

// Entry stores one list of the accessory library (Field = its JSON name). Entries are handled as JSON objects, so any
// field of any entry kind is carried; their files are the "assets/…" strings in the entry and its editor layout.
type Entry struct {
	Field string // "accessories", "furniture", "decorations"
	Sub   string // the entry field shown as sub label ("kind", "type")
}

const (
	entryFile  = "entry.json"  // inside an item's zip folder: the entry
	layoutFile = "layout.json" // its editor layout (studio.json layouts[id])
	filesDir   = "files/"      // its store files, by store name
)

type entryFields struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
	Base string `json:"base"` // the vanilla item it is built on (accessories, furniture)
}

func (e Entry) open(ws workspace.Dir, root string) (*accessories.Library, []json.RawMessage, error) {
	l, err := ws.Items(root)
	if err != nil {
		return nil, nil, err
	}
	list, err := l.RawList(e.Field)
	return l, list, err
}

func find(list []json.RawMessage, id string) int {
	for i, raw := range list {
		if accessories.EntryID(raw) == id {
			return i
		}
	}
	return -1
}

func (e Entry) List(ws workspace.Dir, root string) ([]Item, error) {
	if !fsutil.Exists(filepath.Join(accessories.Folder(root), accessories.LibraryFile)) {
		return nil, nil
	}
	l, list, err := e.open(ws, root)
	if err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(list))
	for _, raw := range list {
		var f entryFields
		if json.Unmarshal(raw, &f) != nil || f.ID == "" {
			continue
		}
		it := Item{ID: f.ID, Name: f.Name, Icon: f.Icon, Base: f.Base, Converted: accessories.IsConverted(f.ID)}
		if e.Sub != "" {
			var m map[string]any
			_ = json.Unmarshal(raw, &m)
			it.Sub, _ = m[e.Sub].(string)
			it.Detail = it.Sub
		}
		if o, ok := l.Meta.Origins[f.ID]; ok {
			o := o
			it.Origin = &o
		}
		out = append(out, it)
	}
	return out, nil
}

func (e Entry) Copy(ws workspace.Dir, from, to string, ids []string) ([]string, error) {
	src, srcList, err := e.open(ws, from)
	if err != nil {
		return nil, err
	}
	dst, dstList, err := e.open(ws, to)
	if err != nil {
		return nil, err
	}
	var done []string
	for _, id := range ids {
		i := find(srcList, id)
		if i < 0 {
			continue
		}
		if j := find(dstList, id); j >= 0 {
			dstList[j] = srcList[i]
		} else {
			dstList = append(dstList, srcList[i])
		}
		if raw, ok := src.Meta.Layouts[id]; ok {
			dst.Meta.Layouts[id] = raw
		} else {
			delete(dst.Meta.Layouts, id)
		}
		if o, ok := src.Meta.Origins[id]; ok {
			dst.Meta.Origins[id] = o
		} else {
			delete(dst.Meta.Origins, id)
		}
		done = append(done, id)
	}
	if len(done) == 0 {
		return nil, nil
	}
	if err := os.MkdirAll(dst.Folder, 0o755); err != nil {
		return nil, err
	}
	if err := dst.SetRawList(e.Field, dstList); err != nil {
		return nil, err
	}
	return done, dst.Save()
}

func (e Entry) Remove(ws workspace.Dir, root string, ids []string) (int, []string, error) {
	if !fsutil.Exists(filepath.Join(accessories.Folder(root), accessories.LibraryFile)) {
		return 0, nil, nil
	}
	l, list, err := e.open(ws, root)
	if err != nil {
		return 0, nil, err
	}
	drop := map[string]bool{}
	for _, id := range ids {
		drop[id] = true
	}
	var refs []string
	kept := list[:0]
	for _, raw := range list {
		id := accessories.EntryID(raw)
		if !drop[id] {
			kept = append(kept, raw)
			continue
		}
		refs = append(refs, assets.Refs(raw)...)
		if lay, ok := l.Meta.Layouts[id]; ok {
			refs = append(refs, assets.Refs(lay)...)
		}
		delete(l.Meta.Layouts, id)
		delete(l.Meta.Origins, id)
	}
	n := len(list) - len(kept)
	if n == 0 {
		return 0, nil, nil
	}
	if err := l.SetRawList(e.Field, kept); err != nil {
		return 0, nil, err
	}
	return n, refs, l.Save()
}

func (e Entry) Cleanup(ws workspace.Dir, roots, _, refs []string) []string {
	still := accessories.StoreRefs(roots...)
	store := ws.Store()
	for _, r := range refs {
		if !still[r] {
			_ = os.Remove(store.Path(r))
		}
	}
	return nil
}

func (e Entry) Export(ws workspace.Dir, root, id, prefix string, z *ziputil.Writer) error {
	l, list, err := e.open(ws, root)
	if err != nil {
		return err
	}
	i := find(list, id)
	if i < 0 {
		return fmt.Errorf("%s %s not found", e.Field, id)
	}
	if err := z.Bytes(prefix+entryFile, indent(list[i])); err != nil {
		return err
	}
	refs := assets.Refs(list[i])
	if lay, ok := l.Meta.Layouts[id]; ok {
		if err := z.Bytes(prefix+layoutFile, ziputil.ScrubJSON(indent(lay))); err != nil {
			return err
		}
		refs = append(refs, assets.Refs(lay)...)
	}
	for _, r := range refs {
		if file := ws.Store().Path(r); fsutil.Exists(file) {
			if err := z.File(prefix+filesDir+path.Base(r), file); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e Entry) Import(ws workspace.Dir, root string, in []In) error {
	if len(in) == 0 {
		return nil
	}
	l, list, err := e.open(ws, root)
	if err != nil {
		return err
	}
	store := ws.Store()
	for _, it := range in {
		b, err := os.ReadFile(filepath.Join(it.Dir, entryFile))
		if err != nil {
			return fmt.Errorf("%s %s: %w", e.Field, it.NewID, err)
		}
		if find(list, it.NewID) >= 0 {
			return fmt.Errorf("%s %s already exists", e.Field, it.NewID)
		}
		// Store files go into this workspace's store; a file is named by its content, so a reference only changes if the
		// file in the zip doesn't match its name.
		names := map[string]string{}
		files, _ := os.ReadDir(filepath.Join(it.Dir, filepath.FromSlash(filesDir)))
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			rel, err := store.PutFile(filepath.Join(it.Dir, filepath.FromSlash(filesDir), f.Name()))
			if err != nil {
				return err
			}
			names[assets.Prefix+f.Name()] = rel
		}
		repoint := func(s string) string {
			if n, ok := names[s]; ok {
				return n
			}
			return s
		}
		entry, err := withID(fsutil.MapJSONStrings(b, repoint), it.NewID)
		if err != nil {
			return fmt.Errorf("%s %s: %w", e.Field, it.NewID, err)
		}
		list = append(list, entry)
		if lay, err := os.ReadFile(filepath.Join(it.Dir, layoutFile)); err == nil {
			l.Meta.Layouts[it.NewID] = json.RawMessage(fsutil.MapJSONStrings(lay, repoint))
		}
		if it.Origin != nil {
			l.Meta.Origins[it.NewID] = *it.Origin
		} else {
			delete(l.Meta.Origins, it.NewID)
		}
	}
	if err := os.MkdirAll(l.Folder, 0o755); err != nil {
		return err
	}
	if err := l.SetRawList(e.Field, list); err != nil {
		return err
	}
	return l.Save()
}

func (Entry) Taken(workspace.Dir, string) bool { return false }

// withID sets a raw entry's "id" (numbers kept as they are).
func withID(raw []byte, id string) (json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	m := map[string]any{}
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	m["id"] = id
	return json.Marshal(m)
}

func indent(raw []byte) []byte {
	var out bytes.Buffer
	if json.Indent(&out, raw, "", "  ") != nil {
		return raw
	}
	return out.Bytes()
}

// OriginOf is a copy of o (nil stays nil), for callers that keep one origin for several items.
func OriginOf(o *origin.Origin) *origin.Origin {
	if o == nil {
		return nil
	}
	c := *o
	return &c
}

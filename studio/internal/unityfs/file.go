package unityfs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Well-known class IDs.
const (
	ClassGameObject    = 1
	ClassTransform     = 4
	ClassMaterial      = 21
	ClassMeshRenderer  = 23
	ClassTexture2D     = 28
	ClassMeshFilter    = 33
	ClassMesh          = 43
	ClassMonoBehaviour = 114
	ClassMonoScript    = 115
	ClassSprite        = 213
	ClassRectTransform = 224
)

// File is one serialized file (sharedassets0.assets, level1, ...). Object data is read on demand.
type File struct {
	Name       string // base name, lower case
	path       string
	f          io.ReaderAt
	closer     io.Closer // nil for files inside a bundle
	be         bool
	version    uint32
	dataOffset int64
	Unity      string
	types      []fileType
	Objects    []*Object
	byID       map[int64]*Object
	scripts    []PPtr // script type table: MonoScript refs per MonoBehaviour type
	externals  []string
}

type fileType struct {
	classID     int32
	scriptIndex int16
}

// Object is one entry of a file's object table.
type Object struct {
	File    *File
	PathID  int64
	ClassID int32
	start   int64
	size    uint32
	typeIdx int32
}

func openFile(path string) (sf *File, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	sf, err = openReader(path, f)
	if err != nil {
		f.Close()
		return nil, err
	}
	sf.closer = f
	return sf, nil
}

// openReader parses a serialized file from any random-access source (a file on disk or a node inside a bundle).
func openReader(path string, f io.ReaderAt) (sf *File, err error) {
	defer catch(&err, filepath.Base(path))

	head := make([]byte, 48)
	if _, err := f.ReadAt(head, 0); err != nil {
		return nil, err
	}
	hr := &reader{b: head, be: true}
	hr.u32()
	hr.u32()
	version := hr.u32()
	hr.u32()
	if version < 22 {
		return nil, fmt.Errorf("unsupported serialized file version %d", version)
	}
	endian := hr.u8()
	hr.skip(3)
	metaSize := int(hr.u32())
	hr.i64() // file size
	dataOffset := hr.i64()
	hr.i64()
	meta := make([]byte, metaSize)
	if _, err := f.ReadAt(meta, int64(hr.p)); err != nil && err != io.EOF {
		return nil, err
	}
	sf = &File{Name: strings.ToLower(filepath.Base(path)), path: path, f: f, be: endian != 0, version: version, dataOffset: dataOffset, byID: map[int64]*Object{}}
	r := &reader{b: meta, be: sf.be}
	sf.Unity = r.cstr()
	r.i32() // target platform
	typeTree := r.bool()
	sf.types = readTypes(r, typeTree, false)
	n := r.count(20)
	sf.Objects = make([]*Object, n)
	for i := range sf.Objects {
		r.align()
		o := &Object{File: sf, PathID: r.i64(), start: r.i64(), size: r.u32(), typeIdx: r.i32()}
		if o.typeIdx >= 0 && int(o.typeIdx) < len(sf.types) {
			o.ClassID = sf.types[o.typeIdx].classID
		}
		sf.Objects[i] = o
		sf.byID[o.PathID] = o
	}
	sc := r.count(12)
	sf.scripts = make([]PPtr, sc)
	for i := range sf.scripts {
		fi := r.i32()
		r.align()
		sf.scripts[i] = PPtr{FileID: fi, PathID: r.i64()}
	}
	ec := r.count(21)
	sf.externals = make([]string, ec)
	for i := range sf.externals {
		r.cstr()
		r.skip(16 + 4) // guid, type
		sf.externals[i] = strings.ToLower(filepath.Base(strings.ReplaceAll(r.cstr(), "\\", "/")))
	}
	return sf, nil
}

func readTypes(r *reader, typeTree, ref bool) []fileType {
	out := make([]fileType, r.count(23))
	for i := range out {
		t := fileType{classID: r.i32()}
		r.u8() // stripped
		t.scriptIndex = r.i16()
		if ref && t.scriptIndex >= 0 || t.classID < 0 || t.classID == ClassMonoBehaviour {
			r.skip(16) // script id
		}
		r.skip(16) // type hash
		if typeTree {
			nodes := r.count(32)
			strs := int(r.i32())
			r.skip(nodes*32 + strs)
			if !ref {
				r.skip(r.count(4) * 4) // type dependencies
			} else {
				r.cstr()
				r.cstr()
				r.cstr()
			}
		}
		out[i] = t
	}
	return out
}

// Data reads the object's bytes.
func (o *Object) Data() ([]byte, error) {
	b := make([]byte, o.size)
	if _, err := o.File.f.ReadAt(b, o.File.dataOffset+o.start); err != nil && err != io.EOF {
		return nil, fmt.Errorf("%s object %d: %w", o.File.Name, o.PathID, err)
	}
	return b, nil
}

func (o *Object) reader() (*reader, error) {
	b, err := o.Data()
	if err != nil {
		return nil, err
	}
	return &reader{b: b, be: o.File.be}, nil
}

// Name reads the leading m_Name of named objects (Mesh, Texture2D, Material, Sprite, MonoScript, ...).
func (o *Object) Name() (name string, err error) {
	defer catch(&err, "name")
	b := make([]byte, 260)
	if o.size < uint32(len(b)) {
		b = b[:o.size]
	}
	if _, err := o.File.f.ReadAt(b, o.File.dataOffset+o.start); err != nil && err != io.EOF {
		return "", err
	}
	r := &reader{b: b, be: o.File.be}
	n := int(r.i32())
	if n < 0 || n > len(b)-4 {
		return "", fmt.Errorf("no name")
	}
	return string(r.take(n)), nil
}

func (sf *File) String() string { return sf.Name }

// Env is the game's data folder (<game>\Card Shop Simulator_Data) or an asset bundle: files opened on demand and
// references resolved across them.
type Env struct {
	Dir     string
	bundle  *Bundle // set: files and .resS data come from this bundle instead of Dir
	mu      sync.Mutex
	files   map[string]*File
	scripts map[*Object]string // MonoScript → class name
}

// Open prepares an environment for a Unity data folder.
func Open(dataDir string) (*Env, error) {
	if _, err := os.Stat(filepath.Join(dataDir, "globalgamemanagers")); err != nil {
		return nil, fmt.Errorf("not a Unity data folder: %s", dataDir)
	}
	return &Env{Dir: dataDir, files: map[string]*File{}, scripts: map[*Object]string{}}, nil
}

// File opens a serialized file by base name (cached). Returns nil, nil for references outside the data folder
// (e.g. "unity default resources" when it isn't there).
func (e *Env) File(name string) (*File, error) {
	name = strings.ToLower(name)
	e.mu.Lock()
	defer e.mu.Unlock()
	if f, ok := e.files[name]; ok {
		return f, nil
	}
	if e.bundle != nil {
		n := e.bundle.Node(name)
		if n == nil || !n.Serialized {
			e.files[name] = nil
			return nil, nil
		}
		f, err := openReader(n.Name, n)
		if err != nil {
			return nil, err
		}
		e.files[name] = f
		return f, nil
	}
	path := filepath.Join(e.Dir, name)
	if _, err := os.Stat(path); err != nil {
		path = filepath.Join(e.Dir, "Resources", name)
		if _, err := os.Stat(path); err != nil {
			e.files[name] = nil
			return nil, nil
		}
	}
	f, err := openFile(path)
	if err != nil {
		return nil, err
	}
	e.files[name] = f
	return f, nil
}

// Close releases every open file.
func (e *Env) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, f := range e.files {
		if f != nil && f.closer != nil {
			f.closer.Close()
		}
	}
	e.files = map[string]*File{}
}

// Resolve follows a reference made from inside file from. Null references and missing files give nil, nil.
func (e *Env) Resolve(from *File, p PPtr) (*Object, error) {
	if p.Null() {
		return nil, nil
	}
	target := from
	if p.FileID != 0 {
		i := int(p.FileID) - 1
		if i < 0 || i >= len(from.externals) {
			return nil, fmt.Errorf("%s: bad file id %d", from.Name, p.FileID)
		}
		f, err := e.File(from.externals[i])
		if err != nil || f == nil {
			return nil, err
		}
		target = f
	}
	return target.byID[p.PathID], nil
}

// ScriptClass is the class name of a MonoBehaviour object ("" if its script can't be resolved).
func (e *Env) ScriptClass(o *Object) string {
	f := o.File
	if o.ClassID != ClassMonoBehaviour || o.typeIdx < 0 || int(o.typeIdx) >= len(f.types) {
		return ""
	}
	si := int(f.types[o.typeIdx].scriptIndex)
	if si < 0 || si >= len(f.scripts) {
		return ""
	}
	ref := f.scripts[si]
	var ms *Object
	if ref.FileID == 0 {
		ms = f.byID[ref.PathID]
	} else {
		ms, _ = e.Resolve(f, PPtr{FileID: ref.FileID, PathID: ref.PathID})
	}
	if ms == nil {
		return ""
	}
	e.mu.Lock()
	name, ok := e.scripts[ms]
	e.mu.Unlock()
	if ok {
		return name
	}
	s, err := ReadMonoScript(ms)
	if err == nil {
		name = s.ClassName
	}
	e.mu.Lock()
	e.scripts[ms] = name
	e.mu.Unlock()
	return name
}

// FindScripts returns every MonoBehaviour in the file whose script class is one of classes.
func (e *Env) FindScripts(f *File, classes ...string) []*Object {
	var out []*Object
	for _, o := range f.Objects {
		if o.ClassID != ClassMonoBehaviour {
			continue
		}
		c := e.ScriptClass(o)
		for _, want := range classes {
			if c == want {
				out = append(out, o)
				break
			}
		}
	}
	return out
}

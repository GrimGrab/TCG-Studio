package unityfs

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/pierrec/lz4/v4"
)

// Bundle is a Unity AssetBundle ("UnityFS" container) as Unity's ChunkBasedCompression builds it: LZ4 (or uncompressed)
// blocks holding the serialized file(s) and their .resS data. Blocks are decompressed on demand, so a large bundle is
// never unpacked to disk. Only what Enhanced Prefab Loader's exporter writes is supported (LZ4 chunks, Unity 2021.3);
// anything else is refused with a clear error rather than half-read.
type Bundle struct {
	Path   string
	Unity  string // engine revision, e.g. "2021.3.38f1"
	f      *os.File
	blocks []bundleBlock
	Nodes  []BundleNode

	mu    sync.Mutex
	cache map[int][]byte // decompressed blocks, small LRU
	order []int
}

type bundleBlock struct {
	uOff, cOff   int64 // start in the uncompressed stream / in the file
	uSize, cSize uint32
	comp         uint16
}

// BundleNode is one file inside the bundle (a serialized file "CAB-…" or its "CAB-….resS" data).
type BundleNode struct {
	Name       string
	offset     int64
	Size       int64
	Serialized bool
	b          *Bundle
}

const (
	compNone  = 0
	compLZMA  = 1
	compLZ4   = 2
	compLZ4HC = 3
)

const bundleCacheBlocks = 24 // ≈ 3 MB of 128 KB blocks; enough for the worker pool reading textures in parallel

// OpenBundle opens a UnityFS bundle.
func OpenBundle(path string) (b *Bundle, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			f.Close()
		}
	}()
	defer catch(&err, "bundle")
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	head := make([]byte, 256)
	n, _ := f.ReadAt(head, 0)
	r := &reader{b: head[:n], be: true}
	if sig := r.cstr(); sig != "UnityFS" {
		return nil, fmt.Errorf("%s is not a Unity asset bundle", path)
	}
	version := r.u32()
	if version < 6 || version > 8 {
		return nil, fmt.Errorf("unsupported bundle version %d", version)
	}
	r.cstr() // "5.x.x"
	b = &Bundle{Path: path, Unity: r.cstr(), f: f, cache: map[int][]byte{}}
	if !strings.HasPrefix(b.Unity, "2021.3.") {
		return nil, fmt.Errorf("bundle was built with Unity %s; only Unity 2021.3 bundles (the game's version) can be read", b.Unity)
	}
	r.i64() // total size
	cInfo := int64(r.u32())
	uInfo := int(r.u32())
	flags := r.u32()
	pos := int64(r.p)
	if version >= 7 {
		pos = (pos + 15) &^ 15
	}
	infoAt := pos
	if flags&0x80 != 0 { // blocks info at the end of the file
		infoAt = st.Size() - cInfo
	}
	raw := make([]byte, cInfo)
	if _, err := f.ReadAt(raw, infoAt); err != nil {
		return nil, fmt.Errorf("blocks info: %w", err)
	}
	info, err := decompress(raw, uInfo, uint16(flags&0x3F))
	if err != nil {
		return nil, fmt.Errorf("blocks info: %w", err)
	}
	data := pos
	if flags&0x80 == 0 {
		data += cInfo
	}
	if flags&0x200 != 0 { // padding before the first block
		data = (data + 15) &^ 15
	}

	ir := &reader{b: info, be: true}
	ir.skip(16) // hash
	var uOff int64
	cOff := data
	for i, nb := 0, ir.count(10); i < nb; i++ {
		blk := bundleBlock{uOff: uOff, cOff: cOff, uSize: ir.u32(), cSize: ir.u32(), comp: ir.u16() & 0x3F}
		if blk.comp != compNone && blk.comp != compLZ4 && blk.comp != compLZ4HC {
			return nil, fmt.Errorf("bundle uses compression %d; only LZ4 bundles (Unity's ChunkBasedCompression, what Enhanced Prefab Loader builds) are supported", blk.comp)
		}
		b.blocks = append(b.blocks, blk)
		uOff += int64(blk.uSize)
		cOff += int64(blk.cSize)
	}
	for i, nn := 0, ir.count(20); i < nn; i++ {
		off, size, nflags := ir.i64(), ir.i64(), ir.u32()
		b.Nodes = append(b.Nodes, BundleNode{offset: off, Size: size, Serialized: nflags&4 != 0, Name: ir.cstr(), b: b})
	}
	return b, nil
}

// Close releases the bundle file.
func (b *Bundle) Close() error { return b.f.Close() }

// Node finds a file in the bundle by name (case-insensitive), nil when absent.
func (b *Bundle) Node(name string) *BundleNode {
	for i := range b.Nodes {
		if strings.EqualFold(b.Nodes[i].Name, name) {
			return &b.Nodes[i]
		}
	}
	return nil
}

// ReadAt reads from the node's bytes.
func (n *BundleNode) ReadAt(p []byte, off int64) (int, error) {
	if off >= n.Size {
		return 0, io.EOF
	}
	want := len(p)
	if rest := n.Size - off; int64(want) > rest {
		want = int(rest)
	}
	got, err := n.b.readAt(p[:want], n.offset+off)
	if err == nil && got < len(p) {
		err = io.EOF
	}
	return got, err
}

// readAt reads from the uncompressed stream of all blocks.
func (b *Bundle) readAt(p []byte, off int64) (int, error) {
	done := 0
	for done < len(p) {
		i := sort.Search(len(b.blocks), func(i int) bool { return b.blocks[i].uOff+int64(b.blocks[i].uSize) > off })
		if i >= len(b.blocks) {
			return done, io.EOF
		}
		blk, err := b.block(i)
		if err != nil {
			return done, err
		}
		n := copy(p[done:], blk[off-b.blocks[i].uOff:])
		done += n
		off += int64(n)
	}
	return done, nil
}

// block returns block i decompressed (cached).
func (b *Bundle) block(i int) ([]byte, error) {
	b.mu.Lock()
	if d, ok := b.cache[i]; ok {
		b.mu.Unlock()
		return d, nil
	}
	b.mu.Unlock()
	blk := b.blocks[i]
	raw := make([]byte, blk.cSize)
	if _, err := b.f.ReadAt(raw, blk.cOff); err != nil {
		return nil, fmt.Errorf("bundle block %d: %w", i, err)
	}
	d, err := decompress(raw, int(blk.uSize), blk.comp)
	if err != nil {
		return nil, fmt.Errorf("bundle block %d: %w", i, err)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.cache[i]; !ok {
		b.cache[i] = d
		b.order = append(b.order, i)
		if len(b.order) > bundleCacheBlocks {
			delete(b.cache, b.order[0])
			b.order = b.order[1:]
		}
	}
	return d, nil
}

func decompress(raw []byte, size int, comp uint16) ([]byte, error) {
	switch comp {
	case compNone:
		return raw, nil
	case compLZ4, compLZ4HC:
		out := make([]byte, size)
		n, err := lz4.UncompressBlock(raw, out)
		if err != nil {
			return nil, err
		}
		if n != size {
			return nil, fmt.Errorf("lz4: got %d bytes, want %d", n, size)
		}
		return out, nil
	}
	return nil, fmt.Errorf("compression %d not supported", comp)
}

// NewBundleEnv is an environment over a bundle's serialized files: references and .resS data resolve inside the bundle.
func NewBundleEnv(b *Bundle) *Env {
	return &Env{bundle: b, files: map[string]*File{}, scripts: map[*Object]string{}}
}

// BundleFiles opens every serialized file in the environment's bundle.
func (e *Env) BundleFiles() ([]*File, error) {
	if e.bundle == nil {
		return nil, fmt.Errorf("not a bundle environment")
	}
	var out []*File
	for _, n := range e.bundle.Nodes {
		if !n.Serialized {
			continue
		}
		f, err := e.File(n.Name)
		if err != nil {
			return nil, err
		}
		if f != nil {
			out = append(out, f)
		}
	}
	return out, nil
}

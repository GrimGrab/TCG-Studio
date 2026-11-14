package cardshader

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/pierrec/lz4/v4"
)

// A shader bundle held in memory: the UnityFS container, its serialized file and the Shader object in it. Only what
// the Unity editor's BuildAssetBundles writes for one shader is understood (type trees included, little-endian data);
// anything else is refused rather than half-read. Shader bundles are a few tens of KB, so nothing is streamed.

type bundle struct {
	version         uint32
	generation      string // "5.x.x"
	unity           string // "2021.3.38f1"
	padBeforeBlocks bool
	nodes           []bundleNode
}

type bundleNode struct {
	name  string
	flags uint32
	data  []byte
}

const (
	bundleInfoCompression = 0x3F
	bundleInfoCombined    = 0x40
	bundleInfoAtEnd       = 0x80
	bundlePadBeforeBlocks = 0x200
	nodeSerializedFile    = 4
)

type cursor struct {
	b   []byte
	p   int
	ord binary.ByteOrder
}

func (c *cursor) take(n int) []byte {
	if n < 0 || c.p+n > len(c.b) {
		panic(fmt.Errorf("read past the end (at %d, %d bytes of %d)", c.p, n, len(c.b)))
	}
	s := c.b[c.p : c.p+n]
	c.p += n
	return s
}
func (c *cursor) u8() byte    { return c.take(1)[0] }
func (c *cursor) u16() uint16 { return c.ord.Uint16(c.take(2)) }
func (c *cursor) u32() uint32 { return c.ord.Uint32(c.take(4)) }
func (c *cursor) i32() int    { return int(int32(c.u32())) }
func (c *cursor) u64() uint64 { return c.ord.Uint64(c.take(8)) }
func (c *cursor) align(n int) { c.p = (c.p + n - 1) &^ (n - 1) }
func (c *cursor) cstr() string {
	end := bytes.IndexByte(c.b[c.p:], 0)
	if end < 0 {
		panic(fmt.Errorf("unterminated string at %d", c.p))
	}
	s := string(c.b[c.p : c.p+end])
	c.p += end + 1
	return s
}

func recoverTo(err *error, what string) {
	if r := recover(); r != nil {
		if e, ok := r.(error); ok {
			*err = fmt.Errorf("%s: %w", what, e)
			return
		}
		panic(r)
	}
}

func unlz4(raw []byte, size int, comp uint32) ([]byte, error) {
	switch comp {
	case 0:
		return raw, nil
	case 2, 3:
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

// readBundle unpacks a UnityFS bundle.
func readBundle(raw []byte) (b *bundle, err error) {
	defer recoverTo(&err, "shader bundle")
	c := &cursor{b: raw, ord: binary.BigEndian}
	if c.cstr() != "UnityFS" {
		return nil, fmt.Errorf("not a Unity asset bundle")
	}
	b = &bundle{version: c.u32(), generation: c.cstr(), unity: c.cstr()}
	if b.version < 6 || b.version > 8 {
		return nil, fmt.Errorf("unsupported bundle version %d", b.version)
	}
	c.u64() // total size
	cInfo, uInfo, flags := int(c.u32()), int(c.u32()), c.u32()
	if b.version >= 7 {
		c.align(16)
	}
	b.padBeforeBlocks = flags&bundlePadBeforeBlocks != 0
	var infoRaw []byte
	if flags&bundleInfoAtEnd != 0 {
		infoRaw = raw[len(raw)-cInfo:]
	} else {
		infoRaw = c.take(cInfo)
	}
	if b.padBeforeBlocks {
		c.align(16)
	}
	info, err := unlz4(infoRaw, uInfo, flags&bundleInfoCompression)
	if err != nil {
		return nil, fmt.Errorf("blocks info: %w", err)
	}
	ic := &cursor{b: info, ord: binary.BigEndian}
	ic.take(16) // hash
	var stream []byte
	for i, n := 0, ic.i32(); i < n; i++ {
		uSize, cSize, bf := int(ic.u32()), int(ic.u32()), ic.u16()
		blk, err := unlz4(c.take(cSize), uSize, uint32(bf)&bundleInfoCompression)
		if err != nil {
			return nil, fmt.Errorf("block %d: %w", i, err)
		}
		stream = append(stream, blk...)
	}
	for i, n := 0, ic.i32(); i < n; i++ {
		off, size, nf := int(ic.u64()), int(ic.u64()), ic.u32()
		name := ic.cstr()
		if off < 0 || off+size > len(stream) {
			return nil, fmt.Errorf("file %s lies outside the bundle's data", name)
		}
		b.nodes = append(b.nodes, bundleNode{name: name, flags: nf, data: stream[off : off+size]})
	}
	return b, nil
}

// bytes packs the bundle again: one uncompressed block, the directory right after the header.
func (b *bundle) bytes() []byte {
	var data bytes.Buffer
	var info bytes.Buffer
	be := binary.BigEndian
	put := func(w *bytes.Buffer, v any) { binary.Write(w, be, v) }
	total := 0
	for _, n := range b.nodes {
		total += len(n.data)
	}
	info.Write(make([]byte, 16))
	put(&info, uint32(1))
	put(&info, uint32(total))
	put(&info, uint32(total))
	put(&info, uint16(0))
	put(&info, uint32(len(b.nodes)))
	for _, n := range b.nodes {
		put(&info, uint64(data.Len()))
		put(&info, uint64(len(n.data)))
		put(&info, n.flags)
		info.WriteString(n.name)
		info.WriteByte(0)
		data.Write(n.data)
	}

	var out bytes.Buffer
	pad := func() {
		for out.Len()%16 != 0 {
			out.WriteByte(0)
		}
	}
	out.WriteString("UnityFS\x00")
	put(&out, b.version)
	out.WriteString(b.generation + "\x00" + b.unity + "\x00")
	sizeAt := out.Len()
	put(&out, uint64(0))
	put(&out, uint32(info.Len()))
	put(&out, uint32(info.Len()))
	flags := uint32(bundleInfoCombined)
	if b.padBeforeBlocks {
		flags |= bundlePadBeforeBlocks
	}
	put(&out, flags)
	if b.version >= 7 {
		pad()
	}
	out.Write(info.Bytes())
	if b.padBeforeBlocks {
		pad()
	}
	out.Write(data.Bytes())
	res := out.Bytes()
	be.PutUint64(res[sizeAt:], uint64(len(res)))
	return res
}

// serialized file ------------------------------------------------------------------------------------------------

const classShader = 48

type assetFile struct {
	raw        []byte
	dataOffset int
	objects    []assetObject
	trees      []*typeTree // per type; nil without a type tree
}

type assetObject struct {
	pathID  int64
	classID int
	start   int // relative to dataOffset
	size    int
	typeIdx int
	entryAt int // where the object's table entry (start i64, size u32) is in raw
}

type typeTree struct{ nodes []treeNode }

type treeNode struct {
	level   int
	isArray bool
	size    int
	align   bool
	name    string // "" for names from Unity's built-in string table
	kids    []int
}

func readAssetFile(raw []byte) (f *assetFile, err error) {
	defer recoverTo(&err, "serialized file")
	h := &cursor{b: raw, ord: binary.BigEndian}
	h.take(8)
	version := h.u32()
	h.take(4)
	if version < 22 {
		return nil, fmt.Errorf("unsupported serialized file version %d", version)
	}
	if h.u8() != 0 {
		return nil, fmt.Errorf("big-endian files are not supported")
	}
	h.take(3)
	h.u32() // metadata size
	h.u64() // file size
	f = &assetFile{raw: raw, dataOffset: int(h.u64())}
	h.u64()

	c := &cursor{b: raw, p: h.p, ord: binary.LittleEndian}
	c.cstr() // engine revision
	c.u32()  // platform
	hasTrees := c.u8() != 0
	types := make([]int, c.i32())
	f.trees = make([]*typeTree, len(types))
	for i := range types {
		types[i] = c.i32()
		c.u8()
		script := int16(c.u16())
		if types[i] < 0 || types[i] == 114 || script >= 0 {
			c.take(16)
		}
		c.take(16)
		if !hasTrees {
			continue
		}
		nodes, strs := c.i32(), c.i32()
		nc := &cursor{b: c.take(nodes * 32), ord: binary.LittleEndian}
		pool := c.take(strs)
		t := &typeTree{nodes: make([]treeNode, nodes)}
		var stack []int
		for n := range t.nodes {
			nc.u16()
			level := int(nc.u8())
			flags := nc.u8()
			nc.u32()
			nameOff := nc.u32()
			size := nc.i32()
			nc.u32()
			meta := nc.u32()
			nc.u64()
			name := ""
			if nameOff&0x80000000 == 0 && int(nameOff) < len(pool) {
				end := bytes.IndexByte(pool[nameOff:], 0)
				name = string(pool[nameOff : int(nameOff)+end])
			}
			t.nodes[n] = treeNode{level: level, isArray: flags&1 != 0, size: size, align: meta&0x4000 != 0, name: name}
			stack = stack[:min(level, len(stack))]
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				t.nodes[p].kids = append(t.nodes[p].kids, n)
			}
			stack = append(stack, n)
		}
		f.trees[i] = t
		c.take(c.i32() * 4) // type dependencies
	}
	f.objects = make([]assetObject, c.i32())
	for i := range f.objects {
		c.align(4)
		o := assetObject{pathID: int64(c.u64()), entryAt: c.p}
		o.start, o.size, o.typeIdx = int(c.u64()), int(c.u32()), c.i32()
		if o.typeIdx < 0 || o.typeIdx >= len(types) {
			return nil, fmt.Errorf("object %d has no type", o.pathID)
		}
		o.classID = types[o.typeIdx]
		if f.dataOffset+o.start+o.size > len(raw) {
			return nil, fmt.Errorf("object %d lies outside the file", o.pathID)
		}
		f.objects[i] = o
	}
	return f, nil
}

func (f *assetFile) data(o assetObject) []byte {
	return f.raw[f.dataOffset+o.start : f.dataOffset+o.start+o.size]
}

// replace gives object i new data: appended at the end of the file (the old bytes stay, unused), so nothing else moves.
func (f *assetFile) replace(i int, data []byte) {
	raw := append([]byte(nil), f.raw...)
	for (len(raw)-f.dataOffset)%16 != 0 {
		raw = append(raw, 0)
	}
	o := &f.objects[i]
	o.start, o.size = len(raw)-f.dataOffset, len(data)
	raw = append(raw, data...)
	binary.LittleEndian.PutUint64(raw[o.entryAt:], uint64(o.start))
	binary.LittleEndian.PutUint32(raw[o.entryAt+8:], uint32(o.size))
	binary.BigEndian.PutUint64(raw[24:], uint64(len(raw)))
	f.raw = raw
}

// skip returns the position after node n's value starting at p.
func (t *typeTree) skip(n int, b []byte, p int) int {
	nd := &t.nodes[n]
	switch {
	case nd.isArray:
		if len(nd.kids) != 2 || p+4 > len(b) {
			panic(fmt.Errorf("unexpected array layout at %d", p))
		}
		count := int(binary.LittleEndian.Uint32(b[p:]))
		p += 4
		elem := &t.nodes[nd.kids[1]]
		if len(elem.kids) == 0 {
			p += count * elem.size
		} else {
			for i := 0; i < count; i++ {
				p = t.skip(nd.kids[1], b, p)
			}
		}
	case len(nd.kids) == 0:
		p += nd.size
	default:
		for _, k := range nd.kids {
			p = t.skip(k, b, p)
		}
	}
	if nd.align {
		p = (p + 3) &^ 3
	}
	if p > len(b) {
		panic(fmt.Errorf("value runs past the object (%d of %d)", p, len(b)))
	}
	return p
}

// fields returns the byte range [start, end) of every top-level field of an object, by name.
func (t *typeTree) fields(b []byte) (m map[string][2]int, order []string, err error) {
	defer recoverTo(&err, "object layout")
	m = map[string][2]int{}
	p := 0
	for i, k := range t.nodes[0].kids {
		end := t.skip(k, b, p)
		name := t.nodes[k].name
		if name == "" {
			name = fmt.Sprintf("#%d", i)
		}
		m[name] = [2]int{p, end}
		order = append(order, name)
		p = end
	}
	if p != len(b) {
		return nil, nil, fmt.Errorf("layout covers %d of %d bytes", p, len(b))
	}
	return m, order, nil
}

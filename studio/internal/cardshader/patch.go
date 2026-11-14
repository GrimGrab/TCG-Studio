package cardshader

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/pierrec/lz4/v4"
)

// Swapping the compiled pixel programs of a template shader bundle.
//
// A Shader object keeps its compiled programs in one LZ4 blob: a table of (offset, length, segment) entries followed
// by the entries. A program entry is
//
//	version, program type, 4 statistics, keyword names, code length, code, padding to 4, trailing data
//
// and for Direct3D 11 the code is a 38-byte Unity header followed by the DXBC container. Parameter entries (constant
// buffer and texture bindings) have no DXBC and are left alone, as are the vertex programs: the new pixel programs are
// compiled against the very same inputs and binding slots, so everything Unity stored about them still holds.

const (
	blobVersion    = 202012090 // Unity 2021.2+
	programPixel40 = 17        // Direct3D 11 pixel program, shader model 4.0
	dx11CodeHeader = 38
)

// program is one compiled pixel program of the template.
type program struct {
	Keywords []string
	DXBC     []byte
}

type blobEntry struct {
	raw      []byte
	segment  uint32
	pixel    bool
	keywords []string
	codeAt   int // start of the code bytes (the length is the 4 bytes before)
	codeLen  int
}

func parseBlob(blob []byte) (entries []blobEntry, err error) {
	defer recoverTo(&err, "shader programs")
	c := &cursor{b: blob, ord: binary.LittleEndian}
	n := c.i32()
	for i := range n {
		off, ln, seg := c.i32(), c.i32(), c.u32()
		if off < 0 || ln < 8 || off+ln > len(blob) {
			return nil, fmt.Errorf("program %d lies outside the data", i)
		}
		e := blobEntry{raw: blob[off : off+ln], segment: seg}
		ec := &cursor{b: e.raw, ord: binary.LittleEndian}
		if v := ec.i32(); v != blobVersion {
			return nil, fmt.Errorf("program data version %d is not supported", v)
		}
		if ec.i32() == programPixel40 {
			ec.take(16)
			for range ec.i32() {
				e.keywords = append(e.keywords, string(ec.take(ec.i32())))
				ec.align(4)
			}
			e.codeLen = ec.i32()
			e.codeAt = ec.p
			code := ec.take(e.codeLen)
			if e.codeLen < dx11CodeHeader+4 || string(code[dx11CodeHeader:dx11CodeHeader+4]) != "DXBC" {
				return nil, fmt.Errorf("program %d has no Direct3D code where expected", i)
			}
			e.pixel = true
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func buildBlob(entries []blobEntry) []byte {
	le := binary.LittleEndian
	head := make([]byte, 4+12*len(entries))
	le.PutUint32(head, uint32(len(entries)))
	var body []byte
	for i, e := range entries {
		le.PutUint32(head[4+i*12:], uint32(len(head)+len(body)))
		le.PutUint32(head[8+i*12:], uint32(len(e.raw)))
		le.PutUint32(head[12+i*12:], e.segment)
		body = append(body, e.raw...)
	}
	return append(head, body...)
}

// withCode returns the entry with other DXBC behind the same Unity header.
func (e blobEntry) withCode(dxbc []byte) blobEntry {
	var out bytes.Buffer
	out.Write(e.raw[:e.codeAt-4])
	binary.Write(&out, binary.LittleEndian, uint32(dx11CodeHeader+len(dxbc)))
	out.Write(e.raw[e.codeAt : e.codeAt+dx11CodeHeader])
	out.Write(dxbc)
	for out.Len()%4 != 0 {
		out.WriteByte(0)
	}
	out.Write(e.raw[(e.codeAt+e.codeLen+3)&^3:])
	e.raw = out.Bytes()
	return e
}

// shaderPrograms returns the pixel programs of the named shader in a bundle.
func shaderPrograms(bundle []byte, shaderName string) ([]program, error) {
	var out []program
	_, err := rewrite(bundle, shaderName, "", func(e blobEntry) ([]byte, error) {
		out = append(out, program{Keywords: e.keywords, DXBC: e.raw[e.codeAt+dx11CodeHeader : e.codeAt+e.codeLen]})
		return nil, nil
	})
	return out, err
}

// swapPrograms returns a copy of the template bundle in which every pixel program of the named shader is the one
// compile returns for its keywords. cab is the new name of the file inside the bundle ("CAB-" + 32 hex digits): Unity
// refuses to load two bundles holding a file of the same name.
func swapPrograms(template []byte, shaderName, cab string, compile func(keywords []string) ([]byte, error)) ([]byte, error) {
	return rewrite(template, shaderName, cab, func(e blobEntry) ([]byte, error) { return compile(e.keywords) })
}

func rewrite(template []byte, shaderName, cab string, each func(e blobEntry) ([]byte, error)) (out []byte, err error) {
	defer recoverTo(&err, "shader bundle")
	b, err := readBundle(template)
	if err != nil {
		return nil, err
	}
	le := binary.LittleEndian
	for ni := range b.nodes {
		node := &b.nodes[ni]
		if node.flags&nodeSerializedFile == 0 {
			continue
		}
		f, err := readAssetFile(node.data)
		if err != nil {
			return nil, err
		}
		for oi, o := range f.objects {
			if o.classID != classShader || f.trees[o.typeIdx] == nil {
				continue
			}
			d := f.data(o)
			m, _, err := f.trees[o.typeIdx].fields(d)
			if err != nil {
				return nil, err
			}
			parsed, platforms, lens, sizes, blobAt := m["m_ParsedForm"], m["platforms"], m["compressedLengths"], m["decompressedLengths"], m["compressedBlob"]
			if parsed[1] == 0 || blobAt[1] == 0 || !bytes.Contains(d[parsed[0]:parsed[1]], lenPrefixed(shaderName)) {
				continue
			}
			// One platform (Direct3D 11), one segment: [1][1]{value} arrays.
			if platforms[1]-platforms[0] != 8 || le.Uint32(d[platforms[0]:]) != 1 || lens[1]-lens[0] != 12 || sizes[1]-sizes[0] != 12 ||
				le.Uint32(d[lens[0]+4:]) != 1 || le.Uint32(d[m["offsets"][0]+8:]) != 0 {
				return nil, fmt.Errorf("shader %s is not built for exactly one platform", shaderName)
			}
			packed := d[blobAt[0]+4 : blobAt[0]+4+int(le.Uint32(d[blobAt[0]:]))]
			blob, err := unlz4(packed, int(le.Uint32(d[sizes[0]+8:])), 2)
			if err != nil {
				return nil, err
			}
			entries, err := parseBlob(blob)
			if err != nil {
				return nil, err
			}
			pixels := 0
			for i, e := range entries {
				if !e.pixel {
					continue
				}
				pixels++
				dxbc, err := each(e)
				if err != nil {
					return nil, err
				}
				if dxbc != nil {
					entries[i] = e.withCode(dxbc)
				}
			}
			if pixels == 0 {
				return nil, fmt.Errorf("shader %s has no Direct3D 11 pixel programs", shaderName)
			}
			if cab == "" {
				return nil, nil
			}
			blob = buildBlob(entries)
			packed = make([]byte, lz4.CompressBlockBound(len(blob)))
			n, err := (&lz4.CompressorHC{Level: lz4.Level9}).CompressBlock(blob, packed)
			if err != nil || n == 0 {
				return nil, fmt.Errorf("packing the shader programs failed: %v", err)
			}
			var nd bytes.Buffer
			nd.Write(d[:blobAt[0]])
			binary.Write(&nd, le, uint32(n))
			nd.Write(packed[:n])
			for nd.Len()%4 != 0 {
				nd.WriteByte(0)
			}
			nd.Write(d[blobAt[1]:])
			res := nd.Bytes()
			le.PutUint32(res[lens[0]+8:], uint32(n))
			le.PutUint32(res[sizes[0]+8:], uint32(len(blob)))
			f.replace(oi, res)
			node.data = f.raw
			node.name = cab
			return b.bytes(), nil
		}
	}
	return nil, fmt.Errorf("shader %s is not in the bundle", shaderName)
}

// lenPrefixed is how a string is stored in an object: length, then the bytes.
func lenPrefixed(s string) []byte {
	out := make([]byte, 4, 4+len(s))
	binary.LittleEndian.PutUint32(out, uint32(len(s)))
	return append(out, s...)
}

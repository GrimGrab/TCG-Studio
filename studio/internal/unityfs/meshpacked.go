package unityfs

import (
	"fmt"
	"math"
)

// Compressed meshes (Mesh Compression on in the import settings) keep their data in bit-packed vectors instead of the
// vertex buffer. Since its Unity 6 build the game ships a few that way (chairs).

type packedFloats struct {
	n          uint32
	rng, start float32
	data       []byte
	bits       byte
}

type packedInts struct {
	n    uint32
	data []byte
	bits byte
}

func (r *reader) packedFloats() packedFloats {
	p := packedFloats{n: r.u32(), rng: r.f32(), start: r.f32()}
	p.data = r.bytes()
	r.align()
	p.bits = r.u8()
	r.align()
	return p
}

func (r *reader) packedInts() packedInts {
	p := packedInts{n: r.u32()}
	p.data = r.bytes()
	r.align()
	p.bits = r.u8()
	r.align()
	return p
}

// bitReader reads values of a fixed bit size, least significant bits first.
type bitReader struct {
	data       []byte
	index, bit int
}

func (b *bitReader) next(size int) uint32 {
	var x uint32
	for got := 0; got < size; {
		if b.index >= len(b.data) {
			panic(fmt.Errorf("packed vector too short"))
		}
		x |= uint32(b.data[b.index]>>b.bit) << got
		n := min(size-got, 8-b.bit)
		b.bit += n
		got += n
		if b.bit == 8 {
			b.index++
			b.bit = 0
		}
	}
	if size < 32 {
		x &= 1<<size - 1
	}
	return x
}

// floats unpacks count values starting at item `from`.
func (p packedFloats) floats(from, count int) []float32 {
	if p.bits == 0 || count <= 0 {
		return nil
	}
	pos := int(p.bits) * from
	b := bitReader{data: p.data, index: pos / 8, bit: pos % 8}
	maxv := float64(uint64(1)<<p.bits - 1)
	out := make([]float32, count)
	for i := range out {
		out[i] = float32(float64(b.next(int(p.bits)))/maxv*float64(p.rng)) + p.start
	}
	return out
}

func (p packedInts) ints() []uint32 {
	if p.bits == 0 {
		return make([]uint32, p.n)
	}
	b := bitReader{data: p.data}
	out := make([]uint32, p.n)
	for i := range out {
		out[i] = b.next(int(p.bits))
	}
	return out
}

// decodeCompressed fills positions, normals, UVs and triangles of a compressed mesh (bones and colours aren't needed).
func (m *Mesh) decodeCompressed(subs []SubMesh, verts, uv, normals packedFloats, normalSigns, triangles packedInts, uvInfo uint32) error {
	n := int(verts.n / 3)
	if n == 0 {
		return fmt.Errorf("compressed mesh %s has no vertices", m.Name)
	}
	pos := verts.floats(0, n*3)
	for i := 0; i < n; i++ {
		m.Pos = append(m.Pos, [3]float32{pos[i*3], pos[i*3+1], pos[i*3+2]})
	}
	// UVs: 4 info bits per channel (bit 2 = present, bits 0–1 = dimension − 1); channels follow each other.
	if uv.n > 0 {
		off := 0
		for ch := 0; ch < 8; ch++ {
			info := uvInfo >> (ch * 4) & 15
			if uvInfo == 0 && ch == 0 {
				info = 4 | 1 // old meshes: one 2D channel
			}
			if info&4 == 0 {
				continue
			}
			dim := int(info&3) + 1
			vals := uv.floats(off, n*dim)
			off += n * dim
			list := make([][2]float32, n)
			for i := range list {
				list[i] = [2]float32{vals[i*dim], at(vals[i*dim:i*dim+dim], 1)}
			}
			if ch == 0 {
				m.UV = list
			}
		}
	}
	// Normals: x and y packed, z rebuilt with its sign.
	if normals.n > 0 {
		xy := normals.floats(0, int(normals.n))
		signs := normalSigns.ints()
		for i := 0; i < n && i*2+1 < len(xy); i++ {
			x, y := float64(xy[i*2]), float64(xy[i*2+1])
			zz := 1 - x*x - y*y
			z := 0.0
			if zz >= 0 {
				z = math.Sqrt(zz)
			} else {
				l := math.Hypot(x, y)
				x, y = x/l, y/l
			}
			if i < len(signs) && signs[i] == 0 {
				z = -z
			}
			m.Normal = append(m.Normal, [3]float32{float32(x), float32(y), float32(z)})
		}
	}
	idx := triangles.ints()
	for _, s := range subs {
		var tris []uint32
		first := int(s.FirstByte) / 2 // 16-bit index positions
		for k := 0; k < int(s.IndexCount) && first+k < len(idx); k++ {
			tris = append(tris, idx[first+k]+s.BaseVertex)
		}
		m.Subs = append(m.Subs, tris)
	}
	return nil
}

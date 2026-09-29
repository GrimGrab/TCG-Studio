package unityfs

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type SubMesh struct {
	FirstByte, IndexCount uint32
	Topology              int32
	BaseVertex            uint32
}

// Mesh is decoded geometry in Unity mesh space (left-handed, Y up; UV origin bottom-left).
type Mesh struct {
	Name       string
	Pos        [][3]float32
	Normal     [][3]float32
	UV         [][2]float32
	Subs       [][]uint32 // triangle vertex indices per submesh (base vertex applied)
	Center     [3]float32 // local AABB
	Extent     [3]float32
	Compressed bool
}

func ReadMesh(e *Env, o *Object) (m *Mesh, err error) {
	defer catch(&err, "Mesh")
	r, err := o.reader()
	if err != nil {
		return nil, err
	}
	m = &Mesh{}
	m.Name = r.str()
	subs := make([]SubMesh, r.count(48))
	for i := range subs {
		subs[i] = SubMesh{FirstByte: r.u32(), IndexCount: r.u32(), Topology: r.i32(), BaseVertex: r.u32()}
		r.u32() // first vertex
		r.u32() // vertex count
		r.skip(24)
	}
	// Blend shapes.
	r.skip(r.count(40) * 40)
	r.skip(r.count(12) * 12)
	for i, n := 0, r.count(16); i < n; i++ {
		r.str()
		r.skip(12)
	}
	r.skip(r.count(4) * 4)
	r.skip(r.count(64) * 64) // bind poses
	r.skip(r.count(4) * 4)   // bone name hashes
	r.u32()                  // root bone hash
	r.skip(r.count(24) * 24) // bones AABB
	r.skip(r.count(4) * 4)   // variable bone count weights
	compression := r.u8()
	r.skip(3) // readable, keep vertices, keep indices
	r.align()
	indexFormat := r.i32()
	indexBuf := r.bytes()
	r.align()
	vertexCount := int(r.u32())
	type channel struct{ stream, offset, format, dim byte }
	chans := make([]channel, r.count(4))
	for i := range chans {
		chans[i] = channel{r.u8(), r.u8(), r.u8(), r.u8() & 0xF}
	}
	vdata := r.bytes()
	r.align()
	// Compressed mesh (packed bit vectors): skipped; the game's meshes aren't compressed.
	packedFloat := func() {
		r.u32()
		r.f32()
		r.f32()
		r.bytes()
		r.align()
		r.u8()
		r.align()
	}
	packedInt := func() { r.u32(); r.bytes(); r.align(); r.u8(); r.align() }
	packedFloat() // vertices
	packedFloat() // uv
	packedFloat() // normals
	packedFloat() // tangents
	packedInt()   // weights
	packedInt()   // normal signs
	packedInt()   // tangent signs
	packedFloat() // float colors
	packedInt()   // bone indices
	packedInt()   // triangles
	r.u32()       // uv info
	m.Center = r.vec3()
	m.Extent = r.vec3()
	r.i32() // usage flags
	r.bytes()
	r.align()
	r.bytes()
	r.align()
	r.f32()
	r.f32() // metrics
	st := r.stream()
	if compression != 0 {
		m.Compressed = true
		return m, fmt.Errorf("mesh %s is compressed (not supported)", m.Name)
	}
	if st.Size > 0 {
		if vdata, err = e.streamData(o.File, vdata, st); err != nil {
			return nil, err
		}
	}

	// Streams: channels grouped by stream; each stream's block starts 16-byte aligned.
	nStreams := 0
	for _, c := range chans {
		if c.dim > 0 && int(c.stream)+1 > nStreams {
			nStreams = int(c.stream) + 1
		}
	}
	strides := make([]int, nStreams)
	offsets := make([]int, nStreams)
	for _, c := range chans {
		if c.dim > 0 {
			strides[c.stream] += int(c.dim) * formatSize(c.format)
		}
	}
	off := 0
	for s := 0; s < nStreams; s++ {
		offsets[s] = off
		off += vertexCount * strides[s]
		off = (off + 15) &^ 15
	}
	read := func(ci int) ([][]float32, error) {
		if ci >= len(chans) || chans[ci].dim == 0 {
			return nil, nil
		}
		c := chans[ci]
		fs := formatSize(c.format)
		out := make([][]float32, vertexCount)
		for v := 0; v < vertexCount; v++ {
			p := offsets[c.stream] + v*strides[c.stream] + int(c.offset)
			if p+int(c.dim)*fs > len(vdata) {
				return nil, fmt.Errorf("vertex data too short")
			}
			vals := make([]float32, c.dim)
			for k := range vals {
				vals[k] = decodeComponent(vdata[p+k*fs:], c.format)
			}
			out[v] = vals
		}
		return out, nil
	}
	pos, err := read(0)
	if err != nil {
		return nil, err
	}
	nrm, err := read(1)
	if err != nil {
		return nil, err
	}
	uv, err := read(4)
	if err != nil {
		return nil, err
	}
	for i := 0; i < vertexCount; i++ {
		if pos != nil {
			m.Pos = append(m.Pos, [3]float32{pos[i][0], pos[i][1], at(pos[i], 2)})
		}
		if nrm != nil {
			m.Normal = append(m.Normal, [3]float32{nrm[i][0], nrm[i][1], at(nrm[i], 2)})
		}
		if uv != nil {
			m.UV = append(m.UV, [2]float32{uv[i][0], at(uv[i], 1)})
		}
	}

	isz := 2
	if indexFormat == 1 {
		isz = 4
	}
	for _, s := range subs {
		var tris []uint32
		if s.Topology == 0 {
			for k := 0; k < int(s.IndexCount); k++ {
				p := int(s.FirstByte) + k*isz
				if p+isz > len(indexBuf) {
					return nil, fmt.Errorf("index buffer too short")
				}
				var ix uint32
				if isz == 2 {
					ix = uint32(binary.LittleEndian.Uint16(indexBuf[p:]))
				} else {
					ix = binary.LittleEndian.Uint32(indexBuf[p:])
				}
				tris = append(tris, ix+s.BaseVertex)
			}
		}
		m.Subs = append(m.Subs, tris)
	}
	return m, nil
}

func at(v []float32, i int) float32 {
	if i < len(v) {
		return v[i]
	}
	return 0
}

// Vertex formats (UnityEngine.Rendering.VertexAttributeFormat).
func formatSize(f byte) int {
	switch f {
	case 0, 10, 11:
		return 4
	case 1, 4, 5, 8, 9:
		return 2
	default:
		return 1
	}
}

func decodeComponent(b []byte, f byte) float32 {
	switch f {
	case 0:
		return math.Float32frombits(binary.LittleEndian.Uint32(b))
	case 1:
		return half(binary.LittleEndian.Uint16(b))
	case 2:
		return float32(b[0]) / 255
	case 3:
		return float32(math.Max(float64(int8(b[0]))/127, -1))
	case 4:
		return float32(binary.LittleEndian.Uint16(b)) / 65535
	case 5:
		return float32(math.Max(float64(int16(binary.LittleEndian.Uint16(b)))/32767, -1))
	case 6:
		return float32(b[0])
	case 7:
		return float32(int8(b[0]))
	case 8:
		return float32(binary.LittleEndian.Uint16(b))
	case 9:
		return float32(int16(binary.LittleEndian.Uint16(b)))
	case 10:
		return float32(binary.LittleEndian.Uint32(b))
	case 11:
		return float32(int32(binary.LittleEndian.Uint32(b)))
	}
	return 0
}

func half(h uint16) float32 {
	sign := uint32(h>>15) << 31
	exp := int(h >> 10 & 0x1F)
	frac := uint32(h & 0x3FF)
	switch {
	case exp == 0:
		v := float32(frac) / 1024 / 16384 // subnormal: frac × 2^-24
		if sign != 0 {
			v = -v
		}
		return v
	case exp == 31:
		return math.Float32frombits(sign | 0x7F800000 | frac<<13)
	}
	return math.Float32frombits(sign | uint32(exp-15+127)<<23 | frac<<13)
}

// Bounds as the mod writes them (center, size).
func (m *Mesh) Bounds() map[string][3]float32 {
	return map[string][3]float32{"center": m.Center, "size": {m.Extent[0] * 2, m.Extent[1] * 2, m.Extent[2] * 2}}
}

// OBJ writes the mesh like the mod's MeshReader.ToObj (v/vt/vn with equal indices, one group per submesh); the mod writes
// a triangle soup (GPU capture), this keeps the shared vertices.
func (m *Mesh) OBJ(header string) string {
	var sb strings.Builder
	f := func(v float32) string { return strconv.FormatFloat(float64(v), 'g', -1, 32) }
	sb.WriteString("# " + m.Name + " — " + header + " (Unity space: left-handed, Y up; UV origin bottom-left)\n")
	sb.WriteString(fmt.Sprintf("# vertices %d, submeshes %d\n", len(m.Pos), len(m.Subs)))
	for _, v := range m.Pos {
		sb.WriteString("v " + f(v[0]) + " " + f(v[1]) + " " + f(v[2]) + "\n")
	}
	hasUV, hasN := len(m.UV) == len(m.Pos), len(m.Normal) == len(m.Pos)
	if hasUV {
		for _, t := range m.UV {
			sb.WriteString("vt " + f(t[0]) + " " + f(t[1]) + "\n")
		}
	}
	if hasN {
		for _, n := range m.Normal {
			sb.WriteString("vn " + f(n[0]) + " " + f(n[1]) + " " + f(n[2]) + "\n")
		}
	}
	for s, tri := range m.Subs {
		sb.WriteString(fmt.Sprintf("g submesh%d\n", s))
		for i := 0; i+2 < len(tri); i += 3 {
			sb.WriteByte('f')
			for k := 0; k < 3; k++ {
				x := strconv.Itoa(int(tri[i+k]) + 1)
				sb.WriteString(" " + x)
				if hasUV || hasN {
					sb.WriteByte('/')
					if hasUV {
						sb.WriteString(x)
					}
				}
				if hasN {
					sb.WriteString("/" + x)
				}
			}
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

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
	Name   string
	Pos    [][3]float32
	Normal [][3]float32
	UV     [][2]float32
	UV2    [][2]float32 // second UV channel (empty when the mesh has none)
	Subs   [][]uint32   // triangle vertex indices per submesh (base vertex applied)
	Shapes []BlendShape
	// Skinning (skinned meshes only): per bone of the renderer's m_Bones its bind pose (mesh space → bone space, column-major
	// like Unity's Matrix4x4 storage), and per vertex up to 4 bone indices with weights.
	BindPoses  [][16]float32
	Joints     [][4]uint16
	Weights    [][4]float32
	Center     [3]float32 // local AABB
	Extent     [3]float32
	Compressed bool
}

// BlendShape is a named morph (a face slider): its frames, each the full shape at Weight (0–100; most have one at 100).
type BlendShape struct {
	Name   string
	Frames []BlendFrame
}

type BlendFrame struct {
	Weight float32
	Deltas []BlendDelta
}

// BlendDelta moves one vertex (mesh space) at the frame's full weight.
type BlendDelta struct {
	Index       uint32
	Pos, Normal [3]float32
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
	// Blend shapes: the deltas of every frame in one list, frames as ranges of it, channels (named) as ranges of frames.
	type shapeVertex struct {
		pos, normal [3]float32
		index       uint32
	}
	verts := make([]shapeVertex, r.count(40))
	for i := range verts {
		verts[i].pos = r.vec3()
		verts[i].normal = r.vec3()
		r.vec3() // tangent
		verts[i].index = r.u32()
	}
	type frame struct{ first, count uint32 }
	frames := make([]frame, r.count(12))
	for i := range frames {
		frames[i] = frame{r.u32(), r.u32()}
		r.skip(4) // has normals, has tangents (+ align)
	}
	type shapeChannel struct {
		name         string
		first, count int32
	}
	channels := make([]shapeChannel, r.count(16))
	for i := range channels {
		channels[i].name = r.str()
		r.u32() // name hash
		channels[i].first = r.i32()
		channels[i].count = r.i32()
	}
	weights := make([]float32, r.count(4))
	for i := range weights {
		weights[i] = r.f32()
	}
	fi := 0
	for _, ch := range channels {
		bs := BlendShape{Name: ch.name}
		for k := int32(0); k < ch.count; k++ {
			f := int(ch.first + k)
			if f < 0 || f >= len(frames) {
				break
			}
			fr := BlendFrame{Weight: 100}
			if fi < len(weights) {
				fr.Weight = weights[fi]
			}
			fi++
			end := int(frames[f].first + frames[f].count)
			for v := int(frames[f].first); v < end && v < len(verts); v++ {
				fr.Deltas = append(fr.Deltas, BlendDelta{Index: verts[v].index, Pos: verts[v].pos, Normal: verts[v].normal})
			}
			bs.Frames = append(bs.Frames, fr)
		}
		m.Shapes = append(m.Shapes, bs)
	}
	m.BindPoses = make([][16]float32, r.count(64))
	for i := range m.BindPoses {
		for k := range 16 {
			m.BindPoses[i][k] = r.f32()
		}
	}
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
	// Compressed mesh (packed bit vectors): decoded below when the mesh uses it.
	pVerts := r.packedFloats()
	pUV := r.packedFloats()
	pNormals := r.packedFloats()
	r.packedFloats() // tangents
	r.packedInts()   // weights
	pNormalSigns := r.packedInts()
	r.packedInts()   // tangent signs
	r.packedFloats() // float colors
	r.packedInts()   // bone indices
	pTriangles := r.packedInts()
	uvInfo := r.u32()
	m.Center = r.vec3()
	m.Extent = r.vec3()
	r.i32() // usage flags
	if o.File.AtLeast(2022, 1) {
		r.i32() // cooking options
	}
	r.bytes()
	r.align()
	r.bytes()
	r.align()
	r.f32()
	r.f32() // metrics
	st := r.stream()
	if compression != 0 || (vertexCount == 0 && pVerts.n > 0) {
		m.Compressed = true
		if err := m.decodeCompressed(subs, pVerts, pUV, pNormals, pNormalSigns, pTriangles, uvInfo); err != nil {
			return nil, err
		}
		return m, nil
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
	uv2, err := read(5)
	if err != nil {
		return nil, err
	}
	bw, err := read(12) // blend weights
	if err != nil {
		return nil, err
	}
	bi, err := read(13) // blend indices
	if err != nil {
		return nil, err
	}
	if bi != nil {
		m.Joints = make([][4]uint16, vertexCount)
		m.Weights = make([][4]float32, vertexCount)
		for i := 0; i < vertexCount; i++ {
			n := len(bi[i])
			for k := 0; k < n && k < 4; k++ {
				m.Joints[i][k] = uint16(bi[i][k])
				switch {
				case bw != nil && k < len(bw[i]):
					m.Weights[i][k] = bw[i][k]
				case bw == nil && k == 0:
					m.Weights[i][k] = 1 // one bone per vertex: no weights stored
				}
			}
		}
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
		if uv2 != nil {
			m.UV2 = append(m.UV2, [2]float32{uv2[i][0], at(uv2[i], 1)})
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

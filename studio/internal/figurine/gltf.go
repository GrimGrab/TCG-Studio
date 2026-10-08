package figurine

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // glTF images
	_ "image/png"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp" // EXT_texture_webp
)

// glTF 2.0 subset: meshes (triangles, strips, fans) under the default scene's node tree, base colour texture/factor
// (+ KHR_texture_transform, KHR_materials_pbrSpecularGlossiness diffuse), vertex colours, quantized attributes. Skins and
// morph targets are ignored (the bind pose is used); Draco / meshopt compression and KTX2 textures are refused with a message.

type gltfDoc struct {
	Scene  *int `json:"scene"`
	Scenes []struct {
		Nodes []int `json:"nodes"`
	} `json:"scenes"`
	Nodes []struct {
		Children    []int     `json:"children"`
		Mesh        *int      `json:"mesh"`
		Matrix      []float64 `json:"matrix"`
		Translation []float64 `json:"translation"`
		Rotation    []float64 `json:"rotation"`
		Scale       []float64 `json:"scale"`
	} `json:"nodes"`
	Meshes []struct {
		Primitives []struct {
			Attributes map[string]int             `json:"attributes"`
			Indices    *int                       `json:"indices"`
			Material   *int                       `json:"material"`
			Mode       *int                       `json:"mode"`
			Extensions map[string]json.RawMessage `json:"extensions"`
		} `json:"primitives"`
	} `json:"meshes"`
	Accessors []struct {
		BufferView    *int            `json:"bufferView"`
		ByteOffset    int             `json:"byteOffset"`
		ComponentType int             `json:"componentType"`
		Normalized    bool            `json:"normalized"`
		Count         int             `json:"count"`
		Type          string          `json:"type"`
		Sparse        json.RawMessage `json:"sparse"`
	} `json:"accessors"`
	BufferViews []struct {
		Buffer     int `json:"buffer"`
		ByteOffset int `json:"byteOffset"`
		ByteLength int `json:"byteLength"`
		ByteStride int `json:"byteStride"`
	} `json:"bufferViews"`
	Buffers []struct {
		URI        string `json:"uri"`
		ByteLength int    `json:"byteLength"`
	} `json:"buffers"`
	Materials []struct {
		Name string `json:"name"`
		PBR  *struct {
			BaseColorFactor  []float64   `json:"baseColorFactor"`
			BaseColorTexture *gltfTexRef `json:"baseColorTexture"`
		} `json:"pbrMetallicRoughness"`
		NormalTexture *struct {
			gltfTexRef
			Scale *float64 `json:"scale"`
		} `json:"normalTexture"`
		Extensions map[string]json.RawMessage `json:"extensions"`
	} `json:"materials"`
	Textures []struct {
		Source     *int                       `json:"source"`
		Extensions map[string]json.RawMessage `json:"extensions"`
	} `json:"textures"`
	Images []struct {
		URI        string `json:"uri"`
		BufferView *int   `json:"bufferView"`
		MimeType   string `json:"mimeType"`
	} `json:"images"`
	ExtensionsRequired []string `json:"extensionsRequired"`
}

type gltfTexRef struct {
	Index      int                        `json:"index"`
	TexCoord   int                        `json:"texCoord"`
	Extensions map[string]json.RawMessage `json:"extensions"`
}

type gltfFile struct {
	doc     gltfDoc
	dir     string
	buffers [][]byte
	images  map[int]image.Image
	scene   *Scene
}

// LoadGLTF reads a .glb or .gltf file.
func LoadGLTF(path string) (*Scene, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f := &gltfFile{dir: filepath.Dir(path), images: map[int]image.Image{}, scene: &Scene{}}
	var jsonPart, binPart []byte
	if len(raw) >= 12 && string(raw[:4]) == "glTF" {
		if v := binary.LittleEndian.Uint32(raw[4:8]); v != 2 {
			return nil, fmt.Errorf("glTF version %d is not supported (only 2.0)", v)
		}
		for off := 12; off+8 <= len(raw); {
			n := int(binary.LittleEndian.Uint32(raw[off:]))
			typ := binary.LittleEndian.Uint32(raw[off+4:])
			if off+8+n > len(raw) {
				return nil, errors.New("truncated GLB file")
			}
			chunk := raw[off+8 : off+8+n]
			switch typ {
			case 0x4E4F534A:
				jsonPart = chunk
			case 0x004E4942:
				binPart = chunk
			}
			off += 8 + n
		}
	} else {
		jsonPart = raw
	}
	if jsonPart == nil {
		return nil, errors.New("no glTF JSON found")
	}
	if err := json.Unmarshal(jsonPart, &f.doc); err != nil {
		return nil, fmt.Errorf("bad glTF JSON: %w", err)
	}
	for _, ext := range f.doc.ExtensionsRequired {
		switch ext {
		case "KHR_draco_mesh_compression", "EXT_meshopt_compression":
			return nil, fmt.Errorf("the model uses %s (compressed geometry), which isn't supported — export it again without mesh compression (Blender: File → Export → glTF, uncheck Compression)", ext)
		case "KHR_texture_basisu":
			return nil, errors.New("the model uses KTX2/Basis textures, which aren't supported — export it again with PNG or JPEG textures")
		}
	}
	for i, b := range f.doc.Buffers {
		switch {
		case b.URI == "" && i == 0 && binPart != nil:
			f.buffers = append(f.buffers, binPart)
		case strings.HasPrefix(b.URI, "data:"):
			data, err := decodeDataURI(b.URI)
			if err != nil {
				return nil, fmt.Errorf("buffer %d: %w", i, err)
			}
			f.buffers = append(f.buffers, data)
		case b.URI != "":
			data, err := os.ReadFile(f.resolve(b.URI))
			if err != nil {
				return nil, fmt.Errorf("buffer file %q is missing next to the model (keep the .gltf, .bin and textures together): %w", b.URI, err)
			}
			f.buffers = append(f.buffers, data)
		default:
			f.buffers = append(f.buffers, nil)
		}
	}
	f.loadMaterials()

	roots := []int{}
	if len(f.doc.Scenes) > 0 {
		si := 0
		if f.doc.Scene != nil && *f.doc.Scene < len(f.doc.Scenes) {
			si = *f.doc.Scene
		}
		roots = f.doc.Scenes[si].Nodes
	} else {
		// No scene: every node that isn't a child is a root.
		child := map[int]bool{}
		for _, n := range f.doc.Nodes {
			for _, c := range n.Children {
				child[c] = true
			}
		}
		for i := range f.doc.Nodes {
			if !child[i] {
				roots = append(roots, i)
			}
		}
	}
	for _, r := range roots {
		if err := f.walk(r, identity(), 0); err != nil {
			return nil, err
		}
	}
	if len(f.scene.Parts) == 0 {
		return nil, errors.New("the model has no triangle meshes")
	}
	return f.scene, nil
}

func (f *gltfFile) resolve(uri string) string {
	if u, err := url.PathUnescape(uri); err == nil {
		uri = u
	}
	return filepath.Join(f.dir, filepath.FromSlash(uri))
}

func decodeDataURI(uri string) ([]byte, error) {
	i := strings.Index(uri, ",")
	if i < 0 {
		return nil, errors.New("bad data URI")
	}
	if strings.Contains(uri[:i], ";base64") {
		return base64.StdEncoding.DecodeString(uri[i+1:])
	}
	s, err := url.PathUnescape(uri[i+1:])
	return []byte(s), err
}

// ---- materials

func (f *gltfFile) loadMaterials() {
	for _, m := range f.doc.Materials {
		mat := Material{Name: m.Name, Factor: [4]float64{1, 1, 1, 1}}
		var ref *gltfTexRef
		if m.PBR != nil {
			if len(m.PBR.BaseColorFactor) == 4 {
				copy(mat.Factor[:], m.PBR.BaseColorFactor)
			}
			ref = m.PBR.BaseColorTexture
		}
		if raw, ok := m.Extensions["KHR_materials_pbrSpecularGlossiness"]; ok && ref == nil {
			var sg struct {
				DiffuseFactor  []float64   `json:"diffuseFactor"`
				DiffuseTexture *gltfTexRef `json:"diffuseTexture"`
			}
			if json.Unmarshal(raw, &sg) == nil {
				if len(sg.DiffuseFactor) == 4 {
					copy(mat.Factor[:], sg.DiffuseFactor)
				}
				ref = sg.DiffuseTexture
			}
		}
		if ref != nil {
			if ref.TexCoord != 0 {
				f.scene.warn(fmt.Sprintf("material %q uses a second UV set for its colour texture; the first one is used", m.Name))
			}
			mat.Image = f.textureImage(ref.Index)
		}
		// glTF colours are linear; the combined texture holds sRGB pixels.
		mat.Factor = srgbColor(mat.Factor)
		if nt := m.NormalTexture; nt != nil {
			if nimg := f.textureImage(nt.Index); nimg != nil {
				strength := 1.0
				if nt.Scale != nil {
					strength = *nt.Scale
				}
				mat.Image = withDetail(mat.Image, shadeFromNormal(nimg, strength))
			}
		}
		f.scene.Materials = append(f.scene.Materials, mat)
	}
}

func (f *gltfFile) textureImage(ti int) image.Image {
	if ti < 0 || ti >= len(f.doc.Textures) {
		return nil
	}
	t := f.doc.Textures[ti]
	src := t.Source
	for _, ext := range []string{"EXT_texture_webp", "KHR_texture_basisu", "MSFT_texture_dds"} {
		if raw, ok := t.Extensions[ext]; ok {
			var e struct {
				Source *int `json:"source"`
			}
			if json.Unmarshal(raw, &e) == nil && e.Source != nil && ext == "EXT_texture_webp" {
				src = e.Source
			}
		}
	}
	if src == nil || *src < 0 || *src >= len(f.doc.Images) {
		return nil
	}
	if img, ok := f.images[*src]; ok {
		return img
	}
	im := f.doc.Images[*src]
	var data []byte
	var err error
	switch {
	case im.BufferView != nil:
		data, err = f.view(*im.BufferView)
	case strings.HasPrefix(im.URI, "data:"):
		data, err = decodeDataURI(im.URI)
	case im.URI != "":
		data, err = os.ReadFile(f.resolve(im.URI))
	}
	var img image.Image
	if err == nil && data != nil {
		img, _, err = image.Decode(bytes.NewReader(data))
	}
	if err != nil || img == nil {
		name := im.URI
		if name == "" || strings.HasPrefix(name, "data:") {
			name = fmt.Sprintf("image %d", *src)
		}
		f.scene.warn(fmt.Sprintf("texture %s couldn't be read (%v); its material is drawn in its plain colour", name, err))
	}
	f.images[*src] = img
	return img
}

// ---- nodes and meshes

func (f *gltfFile) walk(ni int, parent mat4, depth int) error {
	if ni < 0 || ni >= len(f.doc.Nodes) || depth > 64 {
		return nil
	}
	n := f.doc.Nodes[ni]
	local := identity()
	if len(n.Matrix) == 16 {
		copy(local[:], n.Matrix) // column-major like ours
	} else {
		t, r, s := [3]float64{}, [4]float64{0, 0, 0, 1}, [3]float64{1, 1, 1}
		if len(n.Translation) == 3 {
			copy(t[:], n.Translation)
		}
		if len(n.Rotation) == 4 {
			copy(r[:], n.Rotation)
		}
		if len(n.Scale) == 3 {
			copy(s[:], n.Scale)
		}
		local = trs(t, r, s)
	}
	world := parent.mul(local)
	if n.Mesh != nil {
		if err := f.addMesh(*n.Mesh, world); err != nil {
			return err
		}
	}
	for _, c := range n.Children {
		if err := f.walk(c, world, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (f *gltfFile) addMesh(mi int, world mat4) error {
	if mi < 0 || mi >= len(f.doc.Meshes) {
		return nil
	}
	nrmM := world.normalMatrix()
	flip := world.det3() < 0
	for _, p := range f.doc.Meshes[mi].Primitives {
		if _, ok := p.Extensions["KHR_draco_mesh_compression"]; ok {
			return errors.New("the model uses Draco mesh compression, which isn't supported — export it again without compression")
		}
		mode := 4
		if p.Mode != nil {
			mode = *p.Mode
		}
		if mode != 4 && mode != 5 && mode != 6 {
			continue // points / lines
		}
		pa, ok := p.Attributes["POSITION"]
		if !ok {
			continue
		}
		pos, err := f.floats(pa, 3)
		if err != nil {
			return err
		}
		part := Part{Mat: -1}
		if p.Material != nil && *p.Material < len(f.scene.Materials) {
			part.Mat = *p.Material
		}
		for _, v := range pos {
			part.Pos = append(part.Pos, world.point([3]float64{v[0], v[1], v[2]}))
		}
		if na, ok := p.Attributes["NORMAL"]; ok {
			nrm, err := f.floats(na, 3)
			if err == nil && len(nrm) == len(pos) {
				for _, v := range nrm {
					part.Nrm = append(part.Nrm, normalize(nrmM.vec([3]float64{v[0], v[1], v[2]})))
				}
			}
		}
		if ua, ok := p.Attributes["TEXCOORD_0"]; ok {
			uv, err := f.floats(ua, 2)
			if err == nil && len(uv) == len(pos) {
				tt := f.texTransform(part.Mat)
				for _, v := range uv {
					part.UV = append(part.UV, tt.apply([2]float64{v[0], v[1]}))
				}
			}
		}
		if ca, ok := p.Attributes["COLOR_0"]; ok {
			col, err := f.floats(ca, 0)
			if err == nil && len(col) == len(pos) {
				for _, v := range col {
					c := [4]float64{1, 1, 1, 1}
					copy(c[:], v)
					part.Color = append(part.Color, srgbColor(c))
				}
			}
		}
		var idx []uint32
		if p.Indices != nil {
			ix, err := f.floats(*p.Indices, 1)
			if err != nil {
				return err
			}
			idx = make([]uint32, len(ix))
			for i, v := range ix {
				idx[i] = uint32(v[0])
			}
		} else {
			idx = make([]uint32, len(pos))
			for i := range idx {
				idx[i] = uint32(i)
			}
		}
		idx = toTriangles(idx, mode)
		for _, v := range idx {
			if int(v) >= len(pos) {
				return errors.New("the model has an index out of range (broken file)")
			}
		}
		if flip {
			for i := 0; i+2 < len(idx); i += 3 {
				idx[i+1], idx[i+2] = idx[i+2], idx[i+1]
			}
		}
		part.Idx = idx
		if part.Nrm == nil {
			part.Nrm = smoothNormals(part.Pos, part.Idx)
		}
		f.scene.Parts = append(f.scene.Parts, part)
	}
	return nil
}

func toTriangles(idx []uint32, mode int) []uint32 {
	switch mode {
	case 5: // strip
		var out []uint32
		for i := 0; i+2 < len(idx); i++ {
			if i%2 == 0 {
				out = append(out, idx[i], idx[i+1], idx[i+2])
			} else {
				out = append(out, idx[i+1], idx[i], idx[i+2])
			}
		}
		return out
	case 6: // fan
		var out []uint32
		for i := 1; i+1 < len(idx); i++ {
			out = append(out, idx[0], idx[i], idx[i+1])
		}
		return out
	}
	return idx[:len(idx)/3*3]
}

// texTransform is KHR_texture_transform on a material's base colour texture.
type texTransform struct {
	off, scl [2]float64
	rot      float64
	on       bool
}

func (t texTransform) apply(uv [2]float64) [2]float64 {
	if !t.on {
		return uv
	}
	c, s := math.Cos(t.rot), math.Sin(t.rot)
	x, y := uv[0]*t.scl[0], uv[1]*t.scl[1]
	return [2]float64{c*x + s*y + t.off[0], -s*x + c*y + t.off[1]}
}

func (f *gltfFile) texTransform(mi int) texTransform {
	if mi < 0 {
		return texTransform{}
	}
	m := f.doc.Materials[mi]
	if m.PBR == nil || m.PBR.BaseColorTexture == nil {
		return texTransform{}
	}
	raw, ok := m.PBR.BaseColorTexture.Extensions["KHR_texture_transform"]
	if !ok {
		return texTransform{}
	}
	var e struct {
		Offset   []float64 `json:"offset"`
		Rotation float64   `json:"rotation"`
		Scale    []float64 `json:"scale"`
	}
	if json.Unmarshal(raw, &e) != nil {
		return texTransform{}
	}
	t := texTransform{scl: [2]float64{1, 1}, rot: e.Rotation, on: true}
	if len(e.Offset) == 2 {
		copy(t.off[:], e.Offset)
	}
	if len(e.Scale) == 2 {
		copy(t.scl[:], e.Scale)
	}
	return t
}

// ---- accessors

func (f *gltfFile) view(vi int) ([]byte, error) {
	if vi < 0 || vi >= len(f.doc.BufferViews) {
		return nil, errors.New("bad bufferView")
	}
	v := f.doc.BufferViews[vi]
	if v.Buffer < 0 || v.Buffer >= len(f.buffers) || f.buffers[v.Buffer] == nil {
		return nil, errors.New("missing buffer")
	}
	b := f.buffers[v.Buffer]
	if v.ByteOffset+v.ByteLength > len(b) {
		return nil, errors.New("bufferView outside its buffer (broken file)")
	}
	return b[v.ByteOffset : v.ByteOffset+v.ByteLength], nil
}

var typeComps = map[string]int{"SCALAR": 1, "VEC2": 2, "VEC3": 3, "VEC4": 4, "MAT4": 16}

// floats reads an accessor as float rows (normalized integers mapped to 0..1 / -1..1). want = 0 keeps the accessor's width.
func (f *gltfFile) floats(ai, want int) ([][]float64, error) {
	if ai < 0 || ai >= len(f.doc.Accessors) {
		return nil, errors.New("bad accessor")
	}
	a := f.doc.Accessors[ai]
	if len(a.Sparse) > 0 && string(a.Sparse) != "null" {
		f.scene.warn("the model uses sparse accessors (morph data); they were ignored")
	}
	comps := typeComps[a.Type]
	if comps == 0 {
		return nil, fmt.Errorf("unsupported accessor type %s", a.Type)
	}
	if want == 0 {
		want = comps
	}
	size := map[int]int{5120: 1, 5121: 1, 5122: 2, 5123: 2, 5125: 4, 5126: 4}[a.ComponentType]
	if size == 0 {
		return nil, fmt.Errorf("unsupported component type %d", a.ComponentType)
	}
	out := make([][]float64, a.Count)
	if a.BufferView == nil {
		for i := range out {
			out[i] = make([]float64, want)
		}
		return out, nil
	}
	data, err := f.view(*a.BufferView)
	if err != nil {
		return nil, err
	}
	stride := f.doc.BufferViews[*a.BufferView].ByteStride
	if stride == 0 {
		stride = size * comps
	}
	for i := 0; i < a.Count; i++ {
		row := make([]float64, want)
		base := a.ByteOffset + i*stride
		for c := 0; c < comps && c < want; c++ {
			o := base + c*size
			if o+size > len(data) {
				return nil, errors.New("accessor outside its bufferView (broken file)")
			}
			var v float64
			switch a.ComponentType {
			case 5126:
				v = float64(math.Float32frombits(binary.LittleEndian.Uint32(data[o:])))
			case 5125:
				v = float64(binary.LittleEndian.Uint32(data[o:]))
			case 5123:
				v = float64(binary.LittleEndian.Uint16(data[o:]))
				if a.Normalized {
					v /= 65535
				}
			case 5122:
				v = float64(int16(binary.LittleEndian.Uint16(data[o:])))
				if a.Normalized {
					v = math.Max(v/32767, -1)
				}
			case 5121:
				v = float64(data[o])
				if a.Normalized {
					v /= 255
				}
			case 5120:
				v = float64(int8(data[o]))
				if a.Normalized {
					v = math.Max(v/127, -1)
				}
			}
			row[c] = v
		}
		out[i] = row
	}
	return out, nil
}

// ---- matrices (column-major 4×4, like glTF)

type mat4 [16]float64

func identity() mat4 { return mat4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1} }

func (a mat4) mul(b mat4) mat4 {
	var o mat4
	for c := 0; c < 4; c++ {
		for r := 0; r < 4; r++ {
			s := 0.0
			for k := 0; k < 4; k++ {
				s += a[k*4+r] * b[c*4+k]
			}
			o[c*4+r] = s
		}
	}
	return o
}

func trs(t [3]float64, q [4]float64, s [3]float64) mat4 {
	x, y, z, w := q[0], q[1], q[2], q[3]
	r := [9]float64{
		1 - 2*(y*y+z*z), 2 * (x*y + z*w), 2 * (x*z - y*w),
		2 * (x*y - z*w), 1 - 2*(x*x+z*z), 2 * (y*z + x*w),
		2 * (x*z + y*w), 2 * (y*z - x*w), 1 - 2*(x*x+y*y),
	} // columns
	return mat4{
		r[0] * s[0], r[1] * s[0], r[2] * s[0], 0,
		r[3] * s[1], r[4] * s[1], r[5] * s[1], 0,
		r[6] * s[2], r[7] * s[2], r[8] * s[2], 0,
		t[0], t[1], t[2], 1,
	}
}

func (m mat4) point(p [3]float64) [3]float64 {
	return [3]float64{
		m[0]*p[0] + m[4]*p[1] + m[8]*p[2] + m[12],
		m[1]*p[0] + m[5]*p[1] + m[9]*p[2] + m[13],
		m[2]*p[0] + m[6]*p[1] + m[10]*p[2] + m[14],
	}
}

func (m mat4) vec(p [3]float64) [3]float64 {
	return [3]float64{
		m[0]*p[0] + m[4]*p[1] + m[8]*p[2],
		m[1]*p[0] + m[5]*p[1] + m[9]*p[2],
		m[2]*p[0] + m[6]*p[1] + m[10]*p[2],
	}
}

func (m mat4) det3() float64 {
	return m[0]*(m[5]*m[10]-m[9]*m[6]) - m[4]*(m[1]*m[10]-m[9]*m[2]) + m[8]*(m[1]*m[6]-m[5]*m[2])
}

// normalMatrix is the inverse transpose of the upper 3×3 (as a mat4 without translation).
func (m mat4) normalMatrix() mat4 {
	d := m.det3()
	if math.Abs(d) < 1e-30 {
		return identity()
	}
	a, b, c := m[0], m[4], m[8]
	e, f, g := m[1], m[5], m[9]
	h, i, j := m[2], m[6], m[10]
	// inverse (row r, col c) then transpose → cofactor matrix / det
	co := [9]float64{
		(f*j - g*i), -(e*j - g*h), (e*i - f*h),
		-(b*j - c*i), (a*j - c*h), -(a*i - b*h),
		(b*g - c*f), -(a*g - c*e), (a*f - b*e),
	} // co[row*3+col] = cofactor of element (row,col)
	var o mat4
	o[15] = 1
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			o[c*4+r] = co[r*3+c] / d
		}
	}
	return o
}

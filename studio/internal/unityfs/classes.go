package unityfs

import "fmt"

// Layouts below are Unity 2021.3 (format 22, no type trees). Only fields Studio uses are kept; the rest is skipped in order.

type MonoScript struct {
	Name, ClassName, Namespace, Assembly string
}

func ReadMonoScript(o *Object) (s MonoScript, err error) {
	defer catch(&err, "MonoScript")
	r, err := o.reader()
	if err != nil {
		return s, err
	}
	s.Name = r.str()
	r.i32()    // execution order
	r.skip(16) // properties hash
	s.ClassName = r.str()
	s.Namespace = r.str()
	s.Assembly = r.str()
	return s, nil
}

type GameObject struct {
	Components []PPtr
	Layer      uint32
	Name       string
	Active     bool
}

func ReadGameObject(o *Object) (g GameObject, err error) {
	defer catch(&err, "GameObject")
	r, err := o.reader()
	if err != nil {
		return g, err
	}
	g.Components = r.pptrs()
	g.Layer = r.u32()
	g.Name = r.str()
	r.u16() // tag
	g.Active = r.bool()
	return g, nil
}

// Transform (and RectTransform, which only adds fields after these).
type Transform struct {
	GameObject PPtr
	Rotation   [4]float32 // x y z w
	Position   [3]float32
	Scale      [3]float32
	Children   []PPtr
	Father     PPtr
}

func ReadTransform(o *Object) (t Transform, err error) {
	defer catch(&err, "Transform")
	r, err := o.reader()
	if err != nil {
		return t, err
	}
	t.GameObject = r.pptr()
	t.Rotation = r.vec4()
	t.Position = r.vec3()
	t.Scale = r.vec3()
	t.Children = r.pptrs()
	t.Father = r.pptr()
	return t, nil
}

type MeshFilter struct {
	GameObject, Mesh PPtr
}

func ReadMeshFilter(o *Object) (m MeshFilter, err error) {
	defer catch(&err, "MeshFilter")
	r, err := o.reader()
	if err != nil {
		return m, err
	}
	return MeshFilter{r.pptr(), r.pptr()}, nil
}

// Renderer (MeshRenderer / SkinnedMeshRenderer prefix).
type Renderer struct {
	GameObject     PPtr
	Enabled        bool
	Materials      []PPtr
	StaticFirstSub uint16
	StaticSubCount uint16
}

func ReadRenderer(o *Object) (m Renderer, err error) {
	defer catch(&err, "Renderer")
	r, err := o.reader()
	if err != nil {
		return m, err
	}
	m.GameObject = r.pptr()
	m.Enabled = r.bool()
	r.skip(9) // cast/receive shadows, dynamic occludee, static shadow caster, motion vectors, light/reflection probes, ray tracing ×2
	r.align()
	r.u32()    // rendering layer mask
	r.i32()    // renderer priority
	r.skip(4)  // lightmap indices
	r.skip(32) // lightmap tiling offsets
	m.Materials = r.pptrs()
	m.StaticFirstSub = r.u16()
	m.StaticSubCount = r.u16()
	return m, nil
}

// MonoBehaviour header; Fields continues at the script's own serialized fields.
type MonoBehaviour struct {
	GameObject PPtr
	Enabled    bool
	Script     PPtr
	Name       string
	fields     *reader
}

func readMonoBehaviour(o *Object) (m MonoBehaviour, err error) {
	defer catch(&err, "MonoBehaviour")
	r, err := o.reader()
	if err != nil {
		return m, err
	}
	m.GameObject = r.pptr()
	m.Enabled = r.bool()
	r.align()
	m.Script = r.pptr()
	m.Name = r.str()
	m.fields = r
	return m, nil
}

// Material: only the texture slots (m_SavedProperties.m_TexEnvs).
type Material struct {
	Name     string
	Shader   PPtr
	Textures map[string]TexEnv
	Order    []string
}

type TexEnv struct {
	Texture       PPtr
	Scale, Offset [2]float32
}

func ReadMaterial(o *Object) (m Material, err error) {
	defer catch(&err, "Material")
	r, err := o.reader()
	if err != nil {
		return m, err
	}
	m.Name = r.str()
	m.Shader = r.pptr()
	for i, n := 0, r.count(4); i < n; i++ { // valid keywords
		r.str()
	}
	for i, n := 0, r.count(4); i < n; i++ { // invalid keywords
		r.str()
	}
	r.u32()  // lightmap flags
	r.bool() // instancing
	r.bool() // double sided GI
	r.align()
	r.i32()                                 // render queue
	for i, n := 0, r.count(8); i < n; i++ { // string tag map
		r.str()
		r.str()
	}
	for i, n := 0, r.count(4); i < n; i++ { // disabled passes
		r.str()
	}
	m.Textures = map[string]TexEnv{}
	for i, n := 0, r.count(32); i < n; i++ {
		name := r.str()
		m.Textures[name] = TexEnv{Texture: r.pptr(), Scale: r.vec2(), Offset: r.vec2()}
		m.Order = append(m.Order, name)
	}
	return m, nil
}

// MainTexture is what Material.mainTexture returns for the game's shaders: _MainTex, else _BaseMap / _BaseColorMap.
func (m Material) MainTexture() (TexEnv, bool) {
	for _, k := range []string{"_MainTex", "_BaseMap", "_BaseColorMap"} {
		if t, ok := m.Textures[k]; ok && !t.Texture.Null() {
			return t, true
		}
	}
	return TexEnv{}, false
}

// StreamRef points at data kept in a .resS file next to the serialized file.
type StreamRef struct {
	Offset uint64
	Size   uint32
	Path   string
}

func (r *reader) stream() StreamRef { return StreamRef{r.u64(), r.u32(), r.str()} }

// streamData reads bytes kept inline or in the .resS file.
func (e *Env) streamData(f *File, inline []byte, s StreamRef) ([]byte, error) {
	if s.Size == 0 || s.Path == "" {
		return inline, nil
	}
	path := s.Path
	if i := lastSlash(path); i >= 0 {
		path = path[i+1:]
	}
	b := make([]byte, s.Size)
	rf, err := openRaw(e.Dir, path)
	if err != nil {
		return nil, err
	}
	defer rf.Close()
	if _, err := rf.ReadAt(b, int64(s.Offset)); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return b, nil
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' || s[i] == '\\' {
			return i
		}
	}
	return -1
}

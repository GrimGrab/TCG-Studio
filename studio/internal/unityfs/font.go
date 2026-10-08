package unityfs

import (
	"encoding/binary"
	"fmt"
)

const ClassFont = 128

// FontFile finds the TrueType/OpenType data a Font asset embeds (m_FontData, present when "Include Font Data" was on).
// It scans for a length-prefixed block that starts with a font signature instead of walking Font's version-dependent fields.
func FontFile(o *Object) ([]byte, error) {
	d, err := o.Data()
	if err != nil {
		return nil, err
	}
	for i := 0; i+12 <= len(d); i += 4 {
		n := int(binary.LittleEndian.Uint32(d[i:]))
		if n < 1024 || i+4+n > len(d) {
			continue
		}
		switch string(d[i+4 : i+8]) {
		case "\x00\x01\x00\x00", "OTTO", "true":
			if tables := binary.BigEndian.Uint16(d[i+8:]); tables > 0 && tables < 100 {
				return d[i+4 : i+4+n], nil
			}
		}
	}
	return nil, fmt.Errorf("no font data")
}

// TMPFont is the part of a TextMeshPro font asset (TMP_FontAsset, TMP 3.0.x, version "1.1.0") the shop sign preview draws with.
type TMPFont struct {
	Name       string     `json:"name"`
	Family     string     `json:"family"`
	PointSize  float32    `json:"pointSize"`
	LineHeight float32    `json:"lineHeight"`
	Ascent     float32    `json:"ascent"`
	Descent    float32    `json:"descent"`
	Glyphs     []TMPGlyph `json:"glyphs"`
	Atlas      PPtr       `json:"-"`
	AtlasW     int        `json:"atlasWidth"`
	AtlasH     int        `json:"atlasHeight"`
	Gradient   float32    `json:"gradientScale"` // material _GradientScale: the SDF spread in atlas pixels (padding + 1)
}

// TMPGlyph: metrics in point-size units (y up, from the baseline); Rect = x, y, w, h in atlas pixels from the bottom-left.
type TMPGlyph struct {
	Unicode uint32   `json:"u"`
	W       float32  `json:"w"`
	H       float32  `json:"h"`
	BX      float32  `json:"bx"`
	BY      float32  `json:"by"`
	Adv     float32  `json:"adv"`
	Rect    [4]int32 `json:"rect"`
}

func (e *Env) ReadTMPFont(o *Object) (f TMPFont, err error) {
	defer catch(&err, "TMP_FontAsset")
	m, err := readMonoBehaviour(o)
	if err != nil {
		return f, err
	}
	r := m.fields
	f.Name = m.Name
	r.i32()  // hashCode
	r.pptr() // material
	r.i32()  // materialHashCode
	if v := r.str(); v != "1.1.0" {
		return f, fmt.Errorf("font asset version %q", v)
	}
	r.str()  // source font file GUID
	r.pptr() // source font file
	r.i32()  // atlas population mode
	// FaceInfo
	r.i32() // face index
	f.Family = r.str()
	r.str() // style
	f.PointSize = float32(r.i32())
	r.f32() // scale
	r.i32() // units per EM
	f.LineHeight = r.f32()
	f.Ascent = r.f32()
	r.f32() // cap line
	r.f32() // mean line
	r.f32() // baseline
	f.Descent = r.f32()
	r.skip(4 * 9) // super/subscript, underline, strikethrough, tab width
	n := r.count(52)
	glyphs := make(map[uint32]TMPGlyph, n)
	for range n {
		idx := r.u32()
		g := TMPGlyph{W: r.f32(), H: r.f32(), BX: r.f32(), BY: r.f32(), Adv: r.f32()}
		g.Rect = [4]int32{r.i32(), r.i32(), r.i32(), r.i32()}
		r.f32() // scale
		r.i32() // atlas index
		r.i32() // class definition type
		glyphs[idx] = g
	}
	n = r.count(16)
	for range n {
		r.i32() // element type
		u := r.u32()
		gi := r.u32()
		r.f32() // scale
		if g, ok := glyphs[gi]; ok {
			g.Unicode = u
			f.Glyphs = append(f.Glyphs, g)
		}
	}
	atlases := r.pptrs()
	if len(atlases) == 0 {
		return f, fmt.Errorf("no atlas texture")
	}
	f.Atlas = atlases[0]
	return f, nil
}

// TMPText is the layout part of a TextMeshProUGUI component (TMP 3.0.x field order).
type TMPText struct {
	Text           string
	Font, Material PPtr
	Color          [4]float32 // m_fontColor (gamma)
	Gradient       bool       // m_enableVertexGradient
	GradientColors [4][4]float32
	FontSize       float32
	AutoSize       bool
	SizeMin        float32
	SizeMax        float32
	HAlign, VAlign int32
	WordWrap       bool
}

func ReadTMPText(o *Object) (t TMPText, err error) {
	defer catch(&err, "TextMeshProUGUI")
	m, err := readMonoBehaviour(o)
	if err != nil {
		return t, err
	}
	r := m.fields
	r.pptr()                     // m_Material
	r.vec4()                     // m_Color
	r.bool()                     // m_RaycastTarget
	r.align()                    //
	r.vec4()                     // m_RaycastPadding
	r.bool()                     // m_Maskable
	r.align()                    //
	if n := r.count(4); n != 0 { // m_OnCullStateChanged.m_PersistentCalls.m_Calls
		return t, fmt.Errorf("unexpected cull-state listeners")
	}
	t.Text = r.str()
	r.bool() // right to left
	r.align()
	t.Font = r.pptr()
	t.Material = r.pptr()
	r.pptrs() // shared materials
	r.pptr()  // font material
	r.pptrs() // font materials
	r.u32()   // colour32
	t.Color = r.vec4()
	t.Gradient = r.bool()
	r.align()
	r.i32() // colour mode
	for i := range t.GradientColors {
		t.GradientColors[i] = r.vec4()
	}
	r.pptr() // gradient preset
	r.pptr() // sprite asset
	r.bool() // tint sprites
	r.align()
	r.pptr() // style sheet
	r.i32()  // style hash
	r.bool() // override html colours
	r.align()
	r.u32() // face colour
	t.FontSize = r.f32()
	r.f32() // base size
	r.i32() // weight
	t.AutoSize = r.bool()
	r.align()
	t.SizeMin = r.f32()
	t.SizeMax = r.f32()
	r.i32() // style
	t.HAlign = r.i32()
	t.VAlign = r.i32()
	r.i32()       // legacy alignment
	r.skip(4 * 6) // character/word/line spacing, line spacing max, paragraph spacing, width adjustment
	t.WordWrap = r.bool()
	return t, nil
}

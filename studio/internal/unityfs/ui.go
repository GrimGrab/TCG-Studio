package unityfs

import "fmt"

// readGraphic reads the fields every uGUI Graphic (Image, TextMeshProUGUI, …) starts with and returns m_Color.
func readGraphic(r *reader) (color [4]float32, err error) {
	r.pptr() // m_Material
	color = r.vec4()
	r.bool() // m_RaycastTarget
	r.align()
	r.vec4() // m_RaycastPadding
	r.bool() // m_Maskable
	r.align()
	if n := r.count(4); n != 0 { // m_OnCullStateChanged.m_PersistentCalls.m_Calls
		return color, fmt.Errorf("unexpected cull-state listeners")
	}
	return color, nil
}

// UIImage is the drawing part of a uGUI Image component.
type UIImage struct {
	Color          [4]float32 // m_Color (gamma)
	Sprite         PPtr
	Type           int32 // 0 Simple, 1 Sliced, 2 Tiled, 3 Filled
	PreserveAspect bool
	Enabled        bool // the component (a disabled Image draws nothing)
}

func ReadUIImage(o *Object) (im UIImage, err error) {
	defer catch(&err, "Image")
	m, err := readMonoBehaviour(o)
	if err != nil {
		return im, err
	}
	im.Enabled = m.Enabled
	r := m.fields
	if im.Color, err = readGraphic(r); err != nil {
		return im, err
	}
	im.Sprite = r.pptr()
	im.Type = r.i32()
	im.PreserveAspect = r.bool()
	return im, nil
}

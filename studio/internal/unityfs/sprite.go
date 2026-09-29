package unityfs

import (
	"fmt"
	"image"
	"math"
)

type Sprite struct {
	Name              string
	Rect              [4]float32 // x y w h (full rect, including transparent padding)
	Atlas             PPtr
	Texture           PPtr
	TextureRect       [4]float32 // pixels used, in the texture (bottom-left origin)
	TextureRectOffset [2]float32 // where those pixels sit inside Rect
	file              *File
}

func ReadSprite(o *Object) (s Sprite, err error) {
	defer catch(&err, "Sprite")
	r, err := o.reader()
	if err != nil {
		return s, err
	}
	s.file = o.File
	s.Name = r.str()
	s.Rect = r.vec4()
	r.vec2() // offset
	r.vec4() // border
	r.f32()  // pixels per unit
	r.vec2() // pivot
	r.u32()  // extrude
	r.bool() // is polygon
	r.align()
	r.skip(16 + 8) // render data key
	for i, n := 0, r.count(4); i < n; i++ {
		r.str() // atlas tags
	}
	s.Atlas = r.pptr()
	// m_RD
	s.Texture = r.pptr()
	r.pptr() // alpha texture
	for i, n := 0, r.count(16); i < n; i++ {
		r.pptr()
		r.str()
	}
	r.skip(r.count(48) * 48) // submeshes
	r.bytes()                // index buffer
	r.align()
	skipVertexData(r)
	r.skip(r.count(64) * 64) // bind poses
	s.TextureRect = r.vec4()
	s.TextureRectOffset = r.vec2()
	return s, nil
}

func skipVertexData(r *reader) {
	r.u32()
	r.skip(r.count(4) * 4)
	r.bytes()
	r.align()
}

// SpriteImage renders the sprite at its full rect size: its pixels placed at TextureRectOffset, transparent elsewhere
// (as the mod's former in-game export saved icons).
func (e *Env) SpriteImage(s Sprite) (*image.NRGBA, error) {
	if !s.Atlas.Null() {
		return nil, fmt.Errorf("sprite %s is packed in a sprite atlas (not supported)", s.Name)
	}
	to, err := e.Resolve(s.file, s.Texture)
	if err != nil || to == nil {
		return nil, fmt.Errorf("sprite %s: texture not found", s.Name)
	}
	t, err := ReadTexture2D(to)
	if err != nil {
		return nil, err
	}
	src, err := e.TextureImage(t)
	if err != nil {
		return nil, err
	}
	w, h := int(math.Round(float64(s.Rect[2]))), int(math.Round(float64(s.Rect[3])))
	px, py := int(s.TextureRect[0]), int(s.TextureRect[1])
	pw, ph := int(s.TextureRect[2]), int(s.TextureRect[3])
	ox, oy := int(math.Round(float64(s.TextureRectOffset[0]))), int(math.Round(float64(s.TextureRectOffset[1])))
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	H := t.Height
	for j := 0; j < ph; j++ {
		sy, dy := H-1-(py+j), h-1-(oy+j)
		if sy < 0 || sy >= H || dy < 0 || dy >= h {
			continue
		}
		for i := 0; i < pw; i++ {
			sx, dx := px+i, ox+i
			if sx < 0 || sx >= t.Width || dx < 0 || dx >= w {
				continue
			}
			si, di := src.PixOffset(sx, sy), out.PixOffset(dx, dy)
			copy(out.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return out, nil
}

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
	RenderKey         SpriteKey  // its entry in Atlas
	file              *File
}

// SpriteKey is a sprite's render-data key (GUID + local id): what a SpriteAtlas files its pixels under.
type SpriteKey struct {
	GUID [16]byte
	ID   int64
}

// ClassSpriteAtlas is Unity's SpriteAtlas. Since its Unity 6 build (2026-10-27) the game packs most item, furniture and
// pack icons into atlases.
const ClassSpriteAtlas = 687078895

// atlasEntry is a SpriteAtlasData: where one sprite's pixels are in the atlas texture.
type atlasEntry struct {
	Texture           PPtr
	TextureRect       [4]float32
	TextureRectOffset [2]float32
	Settings          uint32 // bit 0 packed, bit 1 tight packing, bits 2–5 rotation
}

// readSpriteAtlas reads a SpriteAtlas' render data map.
func readSpriteAtlas(o *Object) (m map[SpriteKey]atlasEntry, err error) {
	defer catch(&err, "SpriteAtlas")
	r, err := o.reader()
	if err != nil {
		return nil, err
	}
	r.str()                  // name
	r.skip(r.count(12) * 12) // packed sprites
	for i, n := 0, r.count(4); i < n; i++ {
		r.str() // packed sprite names
	}
	n := r.count(24)
	m = make(map[SpriteKey]atlasEntry, n)
	for i := 0; i < n; i++ {
		var k SpriteKey
		copy(k.GUID[:], r.take(16))
		k.ID = r.i64()
		var e atlasEntry
		e.Texture = r.pptr()
		r.pptr() // alpha texture
		e.TextureRect = r.vec4()
		e.TextureRectOffset = r.vec2()
		r.vec2() // atlas rect offset
		r.vec4() // uv transform
		r.f32()  // downscale multiplier
		e.Settings = r.u32()
		for j, sn := 0, r.count(16); j < sn; j++ { // secondary textures
			r.pptr()
			r.str()
		}
		m[k] = e
	}
	return m, nil
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
	copy(s.RenderKey.GUID[:], r.take(16))
	s.RenderKey.ID = r.i64()
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
	texFile := s.file
	if !s.Atlas.Null() {
		// Packed: the atlas says where its pixels are (rect packing; rotated or tight-packed entries aren't drawn).
		ao, err := e.Resolve(s.file, s.Atlas)
		if err != nil || ao == nil || ao.ClassID != ClassSpriteAtlas {
			return nil, fmt.Errorf("sprite %s: atlas not found", s.Name)
		}
		entries, err := readSpriteAtlas(ao)
		if err != nil {
			return nil, err
		}
		ent, ok := entries[s.RenderKey]
		if !ok {
			return nil, fmt.Errorf("sprite %s: not in its atlas", s.Name)
		}
		if ent.Settings>>2&15 != 0 {
			return nil, fmt.Errorf("sprite %s is rotated in its atlas (not supported)", s.Name)
		}
		s.Texture, s.TextureRect, s.TextureRectOffset, texFile = ent.Texture, ent.TextureRect, ent.TextureRectOffset, ao.File
	}
	to, err := e.Resolve(texFile, s.Texture)
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

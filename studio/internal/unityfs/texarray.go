package unityfs

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// ClassTexture2DArray is Unity's Texture2DArray. Since the game update of 2026-10-27 the packs and boxes share one material
// over such an array (CardPackArrayMaterial / CardBoxMaterial, property _MainTexArray); each item picks its slice with
// ItemMeshData.textureIndex.
const ClassTexture2DArray = 187

// Texture2DArray is the part of a Texture2DArray Studio uses: its slices' size and format and where the pixels are.
type Texture2DArray struct {
	Name           string
	Width, Height  int
	Depth          int // slices
	MipCount       int
	GraphicsFormat int // UnityEngine.Experimental.Rendering.GraphicsFormat
	DataSize       int // every slice with its mips
	data           []byte
	stream         StreamRef
	file           *File
}

// ReadTexture2DArray reads the header (Unity 2021.3, measured on the game's arrays: name, fallback format, two flags,
// GraphicsFormat, width, height, depth, mip count, one more int, data size, texture settings, …, inline image data, stream
// data). The pixels are found from the end: the stream reference (offset, size, path) is the last field.
func ReadTexture2DArray(o *Object) (t Texture2DArray, err error) {
	defer catch(&err, "Texture2DArray")
	if o.ClassID != ClassTexture2DArray {
		return t, fmt.Errorf("not a Texture2DArray (class %d)", o.ClassID)
	}
	r, err := o.reader()
	if err != nil {
		return t, err
	}
	t.file = o.File
	t.Name = r.str()
	r.i32()  // forced fallback format
	r.bool() // downscale fallback
	r.bool() // alpha channel optional
	r.align()
	t.GraphicsFormat = int(r.i32())
	t.Width = int(r.i32())
	t.Height = int(r.i32())
	t.Depth = int(r.i32())
	t.MipCount = int(r.i32())
	r.i32() // colour space
	t.DataSize = int(r.i32())
	if t.Width <= 0 || t.Height <= 0 || t.Depth <= 0 || t.DataSize <= 0 {
		return t, fmt.Errorf("%s: unexpected header (%dx%d, %d slices, %d bytes)", t.Name, t.Width, t.Height, t.Depth, t.DataSize)
	}
	// Stream reference at the end: u64 offset, u32 size, aligned string path.
	b := r.b
	if i := strings.LastIndex(string(b), ".resS"); i > 12 {
		end := i + len(".resS")
		start := end
		for start > 0 && b[start-1] != 0 && b[start-1] >= 0x20 {
			start--
		}
		n := int(binary.LittleEndian.Uint32(b[start-4:]))
		if n == end-start && start-16 >= 0 {
			t.stream = StreamRef{binary.LittleEndian.Uint64(b[start-16:]), binary.LittleEndian.Uint32(b[start-8:]), string(b[start:end])}
		}
	}
	if t.stream.Size == 0 {
		return t, fmt.Errorf("%s: pixel data not found (inline arrays aren't supported)", t.Name)
	}
	return t, nil
}

// arrayFormat maps the GraphicsFormats the game's arrays use to the TextureFormat decoder and colour space (1 = sRGB).
func arrayFormat(gf int) (format, colorSpace int, ok bool) {
	switch gf {
	case 96, 97: // RGBA_DXT1 sRGB / UNorm
		return FmtDXT1, boolInt(gf == 96), true
	case 100, 101: // RGBA_DXT5
		return FmtDXT5, boolInt(gf == 100), true
	case 108, 109: // RGBA_BC7
		return FmtBC7, boolInt(gf == 108), true
	case 8, 4: // R8G8B8A8 sRGB / UNorm
		return FmtRGBA32, boolInt(gf == 8), true
	}
	return 0, 0, false
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// mip0Size is the byte size of a slice's full-size level.
func mip0Size(format, w, h int) int {
	blocks := ((w + 3) / 4) * ((h + 3) / 4)
	switch format {
	case FmtDXT1:
		return blocks * 8
	case FmtDXT5, FmtBC7:
		return blocks * 16
	case FmtRGBA32:
		return w * h * 4
	}
	return 0
}

// SliceTexture is one slice's full-size level as a Texture2D (slices are stored one after another, each with its mips);
// TextureImage decodes it, reading only that slice from the .resS file.
func (t Texture2DArray) SliceTexture(index int) (Texture2D, error) {
	if index < 0 || index >= t.Depth {
		return Texture2D{}, fmt.Errorf("%s: slice %d of %d", t.Name, index, t.Depth)
	}
	format, cs, ok := arrayFormat(t.GraphicsFormat)
	if !ok {
		return Texture2D{}, fmt.Errorf("%s: graphics format %d isn't supported", t.Name, t.GraphicsFormat)
	}
	per := t.DataSize / t.Depth
	size := mip0Size(format, t.Width, t.Height)
	if size == 0 || size > per || index*per+size > int(t.stream.Size) {
		return Texture2D{}, fmt.Errorf("%s: slice %d doesn't fit the data", t.Name, index)
	}
	part := StreamRef{Offset: t.stream.Offset + uint64(index*per), Size: uint32(size), Path: t.stream.Path}
	return Texture2D{Name: fmt.Sprintf("%s[%d]", t.Name, index), Width: t.Width, Height: t.Height, Format: format, MipCount: 1,
		ColorSpace: cs, stream: part, file: t.file}, nil
}

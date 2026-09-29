// Package unityfs reads the game's own Unity data files (Unity 2021.3 serialized files, format 22, type trees stripped) so
// TCG Studio can take meshes, textures, sprites and item data straight from the player's install instead of waiting for
// the mod's in-game template export. Only the classes and fields Studio needs are decoded.
package unityfs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

var errShort = errors.New("unexpected end of data")

// reader walks one object's bytes. Reads past the end panic with errShort; exported entry points recover it (catch).
type reader struct {
	b  []byte
	p  int
	be bool
}

func (r *reader) take(n int) []byte {
	if n < 0 || r.p+n > len(r.b) {
		panic(errShort)
	}
	s := r.b[r.p : r.p+n]
	r.p += n
	return s
}

func (r *reader) ord() binary.ByteOrder {
	if r.be {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

func (r *reader) u8() byte         { return r.take(1)[0] }
func (r *reader) bool() bool       { return r.u8() != 0 }
func (r *reader) u16() uint16      { return r.ord().Uint16(r.take(2)) }
func (r *reader) i16() int16       { return int16(r.u16()) }
func (r *reader) u32() uint32      { return r.ord().Uint32(r.take(4)) }
func (r *reader) i32() int32       { return int32(r.u32()) }
func (r *reader) u64() uint64      { return r.ord().Uint64(r.take(8)) }
func (r *reader) i64() int64       { return int64(r.u64()) }
func (r *reader) f32() float32     { return math.Float32frombits(r.u32()) }
func (r *reader) skip(n int)       { r.take(n) }
func (r *reader) align()           { r.p = (r.p + 3) &^ 3 }
func (r *reader) vec2() [2]float32 { return [2]float32{r.f32(), r.f32()} }
func (r *reader) vec3() [3]float32 { return [3]float32{r.f32(), r.f32(), r.f32()} }
func (r *reader) vec4() [4]float32 { return [4]float32{r.f32(), r.f32(), r.f32(), r.f32()} }

// count reads an array length and sanity-checks it against the bytes left (each element takes at least minSize bytes).
func (r *reader) count(minSize int) int {
	n := int(r.i32())
	if n < 0 || (minSize > 0 && n > (len(r.b)-r.p)/minSize) {
		panic(fmt.Errorf("bad array length %d", n))
	}
	return n
}

// str reads an aligned string (int32 length, bytes, align).
func (r *reader) str() string {
	s := string(r.take(r.count(1)))
	r.align()
	return s
}

// bytes reads a byte array (int32 length, bytes) without aligning.
func (r *reader) bytes() []byte { return r.take(r.count(1)) }

// cstr reads a zero-terminated string (file metadata).
func (r *reader) cstr() string {
	for i := r.p; i < len(r.b); i++ {
		if r.b[i] == 0 {
			s := string(r.b[r.p:i])
			r.p = i + 1
			return s
		}
	}
	panic(errShort)
}

// PPtr is a reference to an object: FileID 0 = the same file, n = the file's n-th external; PathID identifies the object.
type PPtr struct {
	FileID int32
	PathID int64
}

func (p PPtr) Null() bool { return p.PathID == 0 }

func (r *reader) pptr() PPtr { return PPtr{r.i32(), r.i64()} }

func (r *reader) pptrs() []PPtr {
	out := make([]PPtr, r.count(12))
	for i := range out {
		out[i] = r.pptr()
	}
	return out
}

func (r *reader) i32s() []int32 {
	out := make([]int32, r.count(4))
	for i := range out {
		out[i] = r.i32()
	}
	return out
}

// catch turns a parse panic into an error.
func catch(err *error, what string) {
	if v := recover(); v != nil {
		e, ok := v.(error)
		if !ok {
			e = fmt.Errorf("%v", v)
		}
		*err = fmt.Errorf("%s: %w", what, e)
	}
}

package unityfs

import (
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
)

// Texture formats (UnityEngine.TextureFormat) this package decodes.
const (
	FmtAlpha8   = 1
	FmtARGB4444 = 2
	FmtRGB24    = 3
	FmtRGBA32   = 4
	FmtARGB32   = 5
	FmtRGB565   = 7
	FmtDXT1     = 10
	FmtDXT5     = 12
	FmtRGBA4444 = 13
	FmtBGRA32   = 14
	// Decoded by decoders.wasm (wasmdec.go): what mod asset bundles use besides DXT.
	FmtBC7          = 25
	FmtDXT1Crunched = 28
	FmtDXT5Crunched = 29
)

type Texture2D struct {
	Name          string
	Width, Height int
	Format        int
	MipCount      int
	ColorSpace    int // 0 linear, 1 sRGB
	data          []byte
	stream        StreamRef
	file          *File
}

func ReadTexture2D(o *Object) (t Texture2D, err error) {
	defer catch(&err, "Texture2D")
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
	t.Width = int(r.i32())
	t.Height = int(r.i32())
	r.i32() // complete image size
	r.i32() // mips stripped
	t.Format = int(r.i32())
	t.MipCount = int(r.i32())
	r.skip(4) // readable, preprocessed, ignore master texture limit, streaming mipmaps
	r.align()
	r.i32()       // streaming priority
	r.i32()       // image count
	r.i32()       // dimension
	r.skip(4 * 6) // texture settings: filter, aniso, mip bias, wrap u/v/w
	r.i32()       // lightmap format
	t.ColorSpace = int(r.i32())
	r.bytes() // platform blob
	r.align()
	t.data = r.bytes()
	r.align()
	t.stream = r.stream()
	return t, nil
}

// Image decodes the top mip level into a top-down image, the way the mod's GPU read-back sees it (linear textures are
// blitted into an sRGB target, so their values come out gamma-encoded).
func (e *Env) TextureImage(t Texture2D) (*image.NRGBA, error) {
	raw, err := e.streamData(t.file, t.data, t.stream)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.Name, err)
	}
	img, err := decode(raw, t.Width, t.Height, t.Format)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", t.Name, err)
	}
	if t.ColorSpace == 0 {
		encodeSRGB(img)
	}
	return img, nil
}

// decode returns a top-down image; Unity stores rows bottom-up.
func decode(b []byte, w, h, format int) (*image.NRGBA, error) {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	set := func(x, y int, c [4]byte) {
		if x < w && y < h {
			i := img.PixOffset(x, h-1-y)
			copy(img.Pix[i:i+4], c[:])
		}
	}
	px := func(bpp int, f func(p []byte) [4]byte) error {
		if len(b) < w*h*bpp {
			return fmt.Errorf("texture data too short")
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				set(x, y, f(b[(y*w+x)*bpp:]))
			}
		}
		return nil
	}
	var err error
	switch format {
	case FmtAlpha8:
		err = px(1, func(p []byte) [4]byte { return [4]byte{255, 255, 255, p[0]} })
	case FmtRGB24:
		err = px(3, func(p []byte) [4]byte { return [4]byte{p[0], p[1], p[2], 255} })
	case FmtRGBA32:
		err = px(4, func(p []byte) [4]byte { return [4]byte{p[0], p[1], p[2], p[3]} })
	case FmtARGB32:
		err = px(4, func(p []byte) [4]byte { return [4]byte{p[1], p[2], p[3], p[0]} })
	case FmtBGRA32:
		err = px(4, func(p []byte) [4]byte { return [4]byte{p[2], p[1], p[0], p[3]} })
	case FmtRGB565:
		err = px(2, func(p []byte) [4]byte {
			v := uint16(p[0]) | uint16(p[1])<<8
			return [4]byte{ext5(v >> 11), ext6(v >> 5 & 63), ext5(v & 31), 255}
		})
	case FmtRGBA4444:
		err = px(2, func(p []byte) [4]byte {
			v := uint16(p[0]) | uint16(p[1])<<8
			return [4]byte{byte(v>>12) * 17, byte(v>>8&15) * 17, byte(v>>4&15) * 17, byte(v&15) * 17}
		})
	case FmtARGB4444:
		err = px(2, func(p []byte) [4]byte {
			v := uint16(p[0]) | uint16(p[1])<<8
			return [4]byte{byte(v>>8&15) * 17, byte(v>>4&15) * 17, byte(v&15) * 17, byte(v>>12) * 17}
		})
	case FmtDXT1Crunched, FmtDXT5Crunched:
		dxt, _, _, bpb, cerr := unpackCrunch(b)
		if cerr != nil {
			return nil, cerr
		}
		f := FmtDXT5
		if bpb == 8 {
			f = FmtDXT1
		}
		return decode(dxt, w, h, f)
	case FmtBC7:
		rgba, berr := decodeBC7(b, w, h)
		if berr != nil {
			return nil, berr
		}
		pw := (w + 3) / 4 * 4
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				i := (y*pw + x) * 4
				set(x, y, [4]byte{rgba[i], rgba[i+1], rgba[i+2], rgba[i+3]})
			}
		}
	case FmtDXT1, FmtDXT5:
		bs := 16
		if format == FmtDXT1 {
			bs = 8
		}
		bw, bh := (w+3)/4, (h+3)/4
		if len(b) < bw*bh*bs {
			return nil, fmt.Errorf("texture data too short")
		}
		var block [16][4]byte
		for by := 0; by < bh; by++ {
			for bx := 0; bx < bw; bx++ {
				blk := b[(by*bw+bx)*bs:]
				switch format {
				case FmtDXT1:
					dxtColor(blk, &block, true)
				case FmtDXT5:
					dxtColor(blk[8:], &block, false)
					dxtAlpha(blk, &block)
				}
				for i := 0; i < 16; i++ {
					set(bx*4+i%4, by*4+i/4, block[i])
				}
			}
		}
	default:
		return nil, fmt.Errorf("unsupported texture format %d", format)
	}
	return img, err
}

func ext5(v uint16) byte { return byte(v<<3 | v>>2) }
func ext6(v uint16) byte { return byte(v<<2 | v>>4) }

func dxtColor(b []byte, out *[16][4]byte, dxt1 bool) {
	c0 := uint16(b[0]) | uint16(b[1])<<8
	c1 := uint16(b[2]) | uint16(b[3])<<8
	var pal [4][4]byte
	pal[0] = [4]byte{ext5(c0 >> 11), ext6(c0 >> 5 & 63), ext5(c0 & 31), 255}
	pal[1] = [4]byte{ext5(c1 >> 11), ext6(c1 >> 5 & 63), ext5(c1 & 31), 255}
	mix := func(a, b, wa, wb, d int) byte { return byte((a*wa + b*wb + d/2) / d) }
	if c0 > c1 || !dxt1 {
		for k := 0; k < 3; k++ {
			pal[2][k] = mix(int(pal[0][k]), int(pal[1][k]), 2, 1, 3)
			pal[3][k] = mix(int(pal[0][k]), int(pal[1][k]), 1, 2, 3)
		}
		pal[2][3], pal[3][3] = 255, 255
	} else {
		for k := 0; k < 3; k++ {
			pal[2][k] = mix(int(pal[0][k]), int(pal[1][k]), 1, 1, 2)
		}
		pal[2][3] = 255
		pal[3] = [4]byte{0, 0, 0, 0}
	}
	bits := uint32(b[4]) | uint32(b[5])<<8 | uint32(b[6])<<16 | uint32(b[7])<<24
	for i := 0; i < 16; i++ {
		c := pal[bits>>(2*i)&3]
		out[i][0], out[i][1], out[i][2] = c[0], c[1], c[2]
		out[i][3] = c[3]
	}
}

func dxtAlpha(b []byte, out *[16][4]byte) {
	a0, a1 := int(b[0]), int(b[1])
	var pal [8]int
	pal[0], pal[1] = a0, a1
	if a0 > a1 {
		for i := 1; i < 7; i++ {
			pal[i+1] = ((7-i)*a0 + i*a1 + 3) / 7
		}
	} else {
		for i := 1; i < 5; i++ {
			pal[i+1] = ((5-i)*a0 + i*a1 + 2) / 5
		}
		pal[6], pal[7] = 0, 255
	}
	var bits uint64
	for i := 0; i < 6; i++ {
		bits |= uint64(b[2+i]) << (8 * i)
	}
	for i := 0; i < 16; i++ {
		out[i][3] = byte(pal[bits>>(3*i)&7])
	}
}

// encodeSRGB converts linear 8-bit colour to sRGB (what a Blit into an sRGB render target does).
func encodeSRGB(img *image.NRGBA) {
	var lut [256]byte
	for i := range lut {
		v := float64(i) / 255
		if v <= 0.0031308 {
			v *= 12.92
		} else {
			v = 1.055*math.Pow(v, 1/2.4) - 0.055
		}
		lut[i] = byte(math.Round(v * 255))
	}
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2] = lut[img.Pix[i]], lut[img.Pix[i+1]], lut[img.Pix[i+2]]
	}
}

func openRaw(dir, name string) (*os.File, error) {
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		f, err = os.Open(filepath.Join(dir, "Resources", name))
	}
	return f, err
}

// Decodable reports whether TextureImage can decode a texture format.
func Decodable(format int) bool {
	switch format {
	case FmtAlpha8, FmtARGB4444, FmtRGB24, FmtRGBA32, FmtARGB32, FmtRGB565, FmtDXT1, FmtDXT5, FmtRGBA4444, FmtBGRA32,
		FmtBC7, FmtDXT1Crunched, FmtDXT5Crunched:
		return true
	}
	return false
}

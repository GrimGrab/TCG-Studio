package art

import (
	"bytes"
	"encoding/binary"
	"image"
)

// RotateQuarter turns an image by turns quarter turns clockwise (negative = anticlockwise). The result starts at (0,0).
func RotateQuarter(src image.Image, turns int) image.Image {
	t := ((turns % 4) + 4) % 4
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if t == 0 {
		dst := image.NewNRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.Set(x, y, src.At(b.Min.X+x, b.Min.Y+y))
			}
		}
		return dst
	}
	dw, dh := w, h
	if t != 2 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := src.At(b.Min.X+x, b.Min.Y+y)
			switch t {
			case 1:
				dst.Set(h-1-y, x, c)
			case 2:
				dst.Set(w-1-x, h-1-y, c)
			case 3:
				dst.Set(y, w-1-x, c)
			}
		}
	}
	return dst
}

// mirror flips an image left-right.
func mirror(src image.Image) image.Image {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.Set(b.Dx()-1-x, y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// ApplyOrientation draws img the way an EXIF orientation value (1–8) says it should be shown. Windows Photos' Rotate
// button only sets this tag on JPEGs; browsers (Studio's preview) honour it, the game does not.
func ApplyOrientation(img image.Image, o int) image.Image {
	switch o {
	case 2:
		return mirror(img)
	case 3:
		return RotateQuarter(img, 2)
	case 4:
		return mirror(RotateQuarter(img, 2))
	case 5:
		return mirror(RotateQuarter(img, 1))
	case 6:
		return RotateQuarter(img, 1)
	case 7:
		return mirror(RotateQuarter(img, -1))
	case 8:
		return RotateQuarter(img, -1)
	}
	return img
}

// ExifOrientation reads the EXIF orientation tag of a JPEG (1 = as stored; also for non-JPEGs and unreadable data).
func ExifOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(b); {
		if b[i] != 0xFF {
			return 1
		}
		marker := b[i+1]
		if marker == 0xFF { // fill byte
			i++
			continue
		}
		if marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) { // markers without a length
			i += 2
			continue
		}
		if marker == 0xDA || marker == 0xD9 { // image data starts: no EXIF before it
			return 1
		}
		n := int(binary.BigEndian.Uint16(b[i+2:]))
		if n < 2 || i+2+n > len(b) {
			return 1
		}
		seg := b[i+4 : i+2+n]
		if marker == 0xE1 && bytes.HasPrefix(seg, []byte("Exif\x00\x00")) {
			return tiffOrientation(seg[6:])
		}
		i += 2 + n
	}
	return 1
}

// tiffOrientation finds tag 0x0112 in IFD0 of a TIFF block.
func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:]))
	if off < 8 || off+2 > len(t) {
		return 1
	}
	count := int(bo.Uint16(t[off:]))
	for k := 0; k < count; k++ {
		e := off + 2 + 12*k
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			if o := int(bo.Uint16(t[e+8:])); o >= 1 && o <= 8 {
				return o
			}
			return 1
		}
	}
	return 1
}

package art

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// grid is a w×h image whose pixel (x,y) has R=x, G=y, so every pixel can be traced after a transform.
func grid(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x), uint8(y), 0, 255})
		}
	}
	return img
}

func at(img image.Image, x, y int) (int, int) {
	c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	return int(c.R), int(c.G)
}

func TestRotateQuarter(t *testing.T) {
	src := grid(2, 3) // 2 wide, 3 tall
	cases := []struct {
		turns  int
		w, h   int
		x, y   int // where the source's top-left (0,0) ends up
		x2, y2 int // where the source's (1,0) ends up
	}{
		{0, 2, 3, 0, 0, 1, 0},
		{1, 3, 2, 2, 0, 2, 1},
		{2, 2, 3, 1, 2, 0, 2},
		{3, 3, 2, 0, 1, 0, 0},
		{-1, 3, 2, 0, 1, 0, 0},
		{5, 3, 2, 2, 0, 2, 1},
	}
	for _, c := range cases {
		r := RotateQuarter(src, c.turns)
		if r.Bounds().Dx() != c.w || r.Bounds().Dy() != c.h {
			t.Fatalf("turns %d: size %v, want %dx%d", c.turns, r.Bounds(), c.w, c.h)
		}
		if x, y := at(r, c.x, c.y); x != 0 || y != 0 {
			t.Errorf("turns %d: (%d,%d) holds source (%d,%d), want (0,0)", c.turns, c.x, c.y, x, y)
		}
		if x, y := at(r, c.x2, c.y2); x != 1 || y != 0 {
			t.Errorf("turns %d: (%d,%d) holds source (%d,%d), want (1,0)", c.turns, c.x2, c.y2, x, y)
		}
	}
}

func TestApplyOrientation(t *testing.T) {
	src := grid(2, 3)
	// Where the stored image's (0,0) and (1,0) are shown for each EXIF orientation (w=2, h=3).
	want := map[int][4]int{
		1: {0, 0, 1, 0},
		2: {1, 0, 0, 0},
		3: {1, 2, 0, 2},
		4: {0, 2, 1, 2},
		5: {0, 0, 0, 1},
		6: {2, 0, 2, 1},
		7: {2, 1, 2, 0},
		8: {0, 1, 0, 0},
	}
	for o, p := range want {
		r := ApplyOrientation(src, o)
		if x, y := at(r, p[0], p[1]); x != 0 || y != 0 {
			t.Errorf("orientation %d: (%d,%d) holds (%d,%d), want (0,0)", o, p[0], p[1], x, y)
		}
		if x, y := at(r, p[2], p[3]); x != 1 || y != 0 {
			t.Errorf("orientation %d: (%d,%d) holds (%d,%d), want (1,0)", o, p[2], p[3], x, y)
		}
	}
}

// withExif inserts an APP1 EXIF segment carrying orientation o right after the JPEG's SOI marker.
func withExif(t *testing.T, jpg []byte, o uint16, bo binary.ByteOrder) []byte {
	t.Helper()
	tiff := make([]byte, 8+2+12+4)
	if bo == binary.LittleEndian {
		copy(tiff, "II")
	} else {
		copy(tiff, "MM")
	}
	bo.PutUint16(tiff[2:], 42)
	bo.PutUint32(tiff[4:], 8)
	bo.PutUint16(tiff[8:], 1)        // one entry
	bo.PutUint16(tiff[10:], 0x0112)  // orientation
	bo.PutUint16(tiff[12:], 3)       // SHORT
	bo.PutUint32(tiff[14:], 1)       // count
	bo.PutUint16(tiff[18:], o)       // value
	seg := append([]byte("Exif\x00\x00"), tiff...)
	app1 := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(app1[2:], uint16(len(seg)+2))
	out := append([]byte{}, jpg[:2]...)
	out = append(out, app1...)
	out = append(out, seg...)
	return append(out, jpg[2:]...)
}

func TestExifOrientation(t *testing.T) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, grid(4, 4), nil); err != nil {
		t.Fatal(err)
	}
	plain := buf.Bytes()
	if o := ExifOrientation(plain); o != 1 {
		t.Errorf("JPEG without EXIF: %d, want 1", o)
	}
	for _, bo := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for _, o := range []uint16{3, 6, 8} {
			b := withExif(t, plain, o, bo)
			if got := ExifOrientation(b); got != int(o) {
				t.Errorf("%v orientation %d: got %d", bo, o, got)
			}
			if _, err := jpeg.Decode(bytes.NewReader(b)); err != nil {
				t.Fatalf("test JPEG with EXIF does not decode: %v", err)
			}
		}
	}
	if o := ExifOrientation([]byte{0x89, 'P', 'N', 'G'}); o != 1 {
		t.Errorf("PNG: %d, want 1", o)
	}
	if o := ExifOrientation(withExif(t, plain, 6, binary.LittleEndian)[:30]); o != 1 {
		t.Errorf("truncated: %d, want 1", o)
	}
}

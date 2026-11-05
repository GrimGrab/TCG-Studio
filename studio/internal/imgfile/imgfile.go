// Package imgfile reads and writes image files: one place for the PNG settings, and every write is atomic and makes its folder.
package imgfile

import (
	"bytes"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"

	"tcgstudio/internal/fsutil"
)

// Load decodes an image file (the formats registered in the program; PNG and JPEG here).
func Load(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// Save writes a PNG at the default compression: for pictures that are kept.
func Save(path string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return fsutil.WriteAtomic(path, buf.Bytes())
}

// EncodeFast is a PNG at the fastest compression: for work that writes hundreds (game templates, conversions).
func EncodeFast(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img)
	return buf.Bytes(), err
}

// SaveFast writes a PNG at the fastest compression.
func SaveFast(path string, img image.Image) error {
	b, err := EncodeFast(img)
	if err != nil {
		return err
	}
	return fsutil.WriteAtomic(path, b)
}

package art

import (
	"fmt"
	"image"

	xdraw "golang.org/x/image/draw"
)

// cropped is the part of src a crop box picked: crop = [x, y, w, h] fractions of src from its top-left (the crop box's
// format); the height follows from the width and aspect (width ÷ height), as the crop box keeps it. Empty = all of src.
func cropped(src image.Image, crop []float64, aspect float64) image.Image {
	b := src.Bounds()
	r := b
	if len(crop) >= 3 && crop[2] > 0 {
		iw, ih := float64(b.Dx()), float64(b.Dy())
		w := min(crop[2], 1) * iw
		h := w / aspect
		x := min(max(crop[0]*iw, 0), iw-w)
		y := min(max(crop[1]*ih, 0), max(ih-h, 0))
		r = image.Rect(b.Min.X+int(x+0.5), b.Min.Y+int(y+0.5), b.Min.X+int(x+w+0.5), b.Min.Y+int(min(y+h, ih)+0.5))
	}
	sub := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	xdraw.Copy(sub, image.Point{}, src, r, xdraw.Src, nil)
	return sub
}

// CropToFile writes the part of the picture at srcPath a crop box picked (see cropped) as a PNG. size: the result's pixels;
// a zero size keeps the cropped part's own size, at most maxSide on its longer side.
func CropToFile(srcPath string, crop []float64, aspect float64, size image.Point, maxSide int, out string) error {
	src, err := loadImage(srcPath)
	if err != nil {
		return fmt.Errorf("picture: %w", err)
	}
	part := cropped(src, crop, aspect)
	if size == (image.Point{}) {
		size = part.Bounds().Size()
		if m := max(size.X, size.Y); maxSide > 0 && m > maxSide {
			size = image.Pt(max(1, size.X*maxSide/m), max(1, size.Y*maxSide/m))
		}
	}
	img := image.NewNRGBA(image.Rectangle{Max: size})
	drawCover(img, img.Bounds(), part)
	return savePNG(out, img)
}

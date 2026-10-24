// Command appicon draws the TCG Studio logo (the two fanned cards of the sidebar's .logo in App.svelte: same sizes,
// angles and colours) into build/appicon.png and build/windows/icon.ico, every icon size drawn on its own.
// Run from studio\: go run ./cmd/appicon   (then wails build embeds the new icon)
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

type card struct {
	cx, cy, rot float64 // centre in logo units (sidebar CSS px), rotation in degrees
	solid       [3]float64
	grad        *[2][3]float64
}

const w, h, r = 16.0, 22.0, 3.0

func hex(v uint32) [3]float64 {
	return [3]float64{float64(v>>16&255) / 255, float64(v>>8&255) / 255, float64(v&255) / 255}
}

// sdRoundRect: signed distance to a w×h rounded rect centred on 0.
func sdRoundRect(x, y float64) float64 {
	qx := math.Abs(x) - (w/2 - r)
	qy := math.Abs(y) - (h/2 - r)
	out := math.Hypot(math.Max(qx, 0), math.Max(qy, 0))
	return out + math.Min(math.Max(qx, qy), 0) - r
}

func local(c card, x, y float64) (float64, float64) {
	a := -c.rot * math.Pi / 180
	dx, dy := x-c.cx, y-c.cy
	return dx*math.Cos(a) - dy*math.Sin(a), dx*math.Sin(a) + dy*math.Cos(a)
}

func (c card) colorAt(lx, ly float64) [3]float64 {
	if c.grad == nil {
		return c.solid
	}
	th := 160 * math.Pi / 180
	dx, dy := math.Sin(th), -math.Cos(th)
	l := math.Abs(w*dx) + math.Abs(h*dy)
	t := math.Max(0, math.Min(1, 0.5+(lx*dx+ly*dy)/l))
	g := c.grad
	return [3]float64{g[0][0] + (g[1][0]-g[0][0])*t, g[0][1] + (g[1][1]-g[0][1])*t, g[0][2] + (g[1][2]-g[0][2])*t}
}

func render(size int) *image.NRGBA {
	back := card{cx: 8, cy: 13, rot: -12, solid: hex(0x3f6ee0)}
	front := card{cx: 14, cy: 13, rot: 8, grad: &[2][3]float64{hex(0x5b8cff), hex(0xb66dff)}}
	// Fit the fanned pair (28.6 × 24.8 units, centred 1.8 left of x=11) with a small margin.
	scale := float64(size) * 0.94 / 28.6
	ox, oy := 10.75, 12.9
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const ss = 4
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var cr, cg, cb, ca float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px)+(float64(sx)+0.5)/ss-float64(size)/2)/scale + ox
					y := (float64(py)+(float64(sy)+0.5)/ss-float64(size)/2)/scale + oy
					var col [3]float64
					var a float64
					// Back card.
					if lx, ly := local(back, x, y); sdRoundRect(lx, ly) <= 0 {
						col, a = back.colorAt(lx, ly), 1
					}
					// Front card's soft shadow on the back card (CSS: -2px 1px 4px #0006).
					if lx, ly := local(front, x+2, y-1); a > 0 {
						d := sdRoundRect(lx, ly)
						if s := 0.3 * math.Max(0, math.Min(1, 1-(d+0.3)/2.2)); s > 0 {
							for i := range col {
								col[i] *= 1 - s
							}
						}
					}
					// Front card.
					if lx, ly := local(front, x, y); sdRoundRect(lx, ly) <= 0 {
						col, a = front.colorAt(lx, ly), 1
					}
					cr += col[0] * a
					cg += col[1] * a
					cb += col[2] * a
					ca += a
				}
			}
			if ca == 0 {
				continue
			}
			img.SetNRGBA(px, py, color.NRGBA{uint8(cr / ca * 255), uint8(cg / ca * 255), uint8(cb / ca * 255), uint8(ca / (ss * ss) * 255)})
		}
	}
	return img
}

func pngBytes(img image.Image) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}

func main() {
	out := "build"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := os.WriteFile(filepath.Join(out, "appicon.png"), pngBytes(render(1024)), 0o644); err != nil {
		panic(err)
	}
	// ICO with PNG entries (Windows Vista+), one rendered per size.
	sizes := []int{16, 20, 24, 32, 40, 48, 64, 128, 256}
	var dir, data bytes.Buffer
	binary.Write(&dir, binary.LittleEndian, []uint16{0, 1, uint16(len(sizes))})
	offset := 6 + 16*len(sizes)
	for _, s := range sizes {
		p := pngBytes(render(s))
		d := byte(s)
		if s == 256 {
			d = 0
		}
		dir.Write([]byte{d, d, 0, 0})
		binary.Write(&dir, binary.LittleEndian, []uint16{1, 32})
		binary.Write(&dir, binary.LittleEndian, []uint32{uint32(len(p)), uint32(offset)})
		offset += len(p)
		data.Write(p)
	}
	if err := os.WriteFile(filepath.Join(out, "windows", "icon.ico"), append(dir.Bytes(), data.Bytes()...), 0o644); err != nil {
		panic(err)
	}
}

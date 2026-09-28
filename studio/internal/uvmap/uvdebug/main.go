// Debug: draws a mesh's UV panels over its texture and prints the panel list.
// usage: go run . <mesh.obj> <texture.png> <out.png>
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"

	"tcgstudio/internal/uvmap"
)

func main() {
	m, err := uvmap.LoadOBJ(os.Args[1])
	if err != nil {
		panic(err)
	}
	f, _ := os.Open(os.Args[2])
	src, err := png.Decode(f)
	if err != nil {
		panic(err)
	}
	b := src.Bounds()
	img := image.NewNRGBA(b)
	draw.Draw(img, b, src, b.Min, draw.Src)
	for i := range img.Pix {
		if i%4 != 3 {
			img.Pix[i] /= 2
		}
	}
	w, h := float64(b.Dx()), float64(b.Dy())
	cols := map[string]color.NRGBA{"front": {255, 60, 60, 255}, "back": {60, 255, 60, 255}, "left": {60, 140, 255, 255},
		"right": {255, 255, 60, 255}, "top": {255, 60, 255, 255}, "bottom": {60, 255, 255, 255}}
	line := func(a, c uvmap.Vec2, col color.NRGBA) {
		x0, y0 := a[0]*(w-1), (1-a[1])*(h-1)
		x1, y1 := c[0]*(w-1), (1-c[1])*(h-1)
		n := int(max(abs(x1-x0), abs(y1-y0))) + 1
		for s := 0; s <= n; s++ {
			t := float64(s) / float64(n)
			x, y := int(x0+(x1-x0)*t), int(y0+(y1-y0)*t)
			for dx := -1; dx <= 1; dx++ {
				for dy := -1; dy <= 1; dy++ {
					img.Set(x+dx, y+dy, col)
				}
			}
		}
	}
	ps := m.Panels(0.0005)
	for i, p := range ps {
		for _, e := range p.Outline {
			line(e[0], e[1], cols[p.Face])
		}
		fmt.Printf("%2d %-6s flat=%v tris=%4d uv x %.3f-%.3f y %.3f-%.3f (px x %4.0f-%4.0f y %4.0f-%4.0f top-down) pos %v..%v areaUV=%.4f area3D=%.4f\n",
			i, p.Face, p.Flat, p.Tris, p.UVMin[0], p.UVMax[0], p.UVMin[1], p.UVMax[1],
			p.UVMin[0]*w, p.UVMax[0]*w, (1-p.UVMax[1])*h, (1-p.UVMin[1])*h, fmt3(p.PosMin), fmt3(p.PosMax), p.AreaUV, p.Area3D)
	}
	o, _ := os.Create(os.Args[3])
	png.Encode(o, img)
	o.Close()
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
func fmt3(v uvmap.Vec3) string { return fmt.Sprintf("(%.3f,%.3f,%.3f)", v[0], v[1], v[2]) }

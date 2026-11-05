package uvmap

// Island is one UV island: Rect in texture pixels (top-left origin); Small = a seam or trim (a hidden face).
type Island struct {
	ID, Label string
	Rect      [4]float64
	Small     bool
}

// Clothing is the face editor's model of one material of a customer clothing piece: the print lies on the piece's own UVs
// (the shader's _Print at scale 1), so every UV island is a face at its place in the texture. The net is the texture
// itself (pixels / 100), so the faces keep their layout and size.
func Clothing(mesh, url string, size int, islands []Island) Model {
	m := Model{Kind: "Clothing", Mesh: mesh, TextureSize: size, Icon: "mesh",
		Size:    [3]float64{float64(size) / 100, float64(size) / 100, 1},
		Preview: []PreviewPart{{Mesh: mesh, URL: url, Texture: "main"}}}
	for _, is := range islands {
		r := is.Rect
		f := face(is.ID, is.Label, [4]float64{r[0] / 100, r[1] / 100, (r[2] - r[0]) / 100, (r[3] - r[1]) / 100}, at(r))
		if is.Small {
			f = hidden(f)
		}
		m.Faces = append(m.Faces, f)
	}
	return inclusive(m)
}

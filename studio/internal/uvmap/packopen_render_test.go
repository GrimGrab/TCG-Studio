package uvmap

import (
	"encoding/json"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/packmesh"
)

// TestRenderPackOpen (RENDER_OUT=<dir>) renders the pack-opening pack from the templates (packopen.json, built by
// gameextract) folded shut, next to the shelf pack, both with the Basic pack's texture, from the front, back and sides:
// the two should look the same. Run it after a game update.
func TestRenderPackOpen(t *testing.T) {
	out := os.Getenv("RENDER_OUT")
	if out == "" {
		t.Skip("set RENDER_OUT")
	}
	dir := templatesDir(t)
	b, err := os.ReadFile(filepath.Join(dir, "packopen.json"))
	if err != nil {
		t.Skip("no packopen.json in the templates: ", err)
	}
	var f struct {
		Shelf       string    `json:"shelf"`
		Bones       []string  `json:"bones"`
		BoneParents []int     `json:"boneParents"`
		BindPoses   []float64 `json:"bindPoses"`
		Positions   []float64 `json:"positions"`
		Normals     []float64 `json:"normals"`
		UVs         []float64 `json:"uvs"`
		BoneIndices []int     `json:"boneIndices"`
		BoneWeights []float64 `json:"boneWeights"`
		Triangles   []int     `json:"triangles"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	var m packmesh.Mesh
	for i := 0; i < len(f.Positions)/3; i++ {
		m.Pos = append(m.Pos, packmesh.Vec3{f.Positions[3*i], f.Positions[3*i+1], f.Positions[3*i+2]})
		m.Norm = append(m.Norm, packmesh.Vec3{f.Normals[3*i], f.Normals[3*i+1], f.Normals[3*i+2]})
		m.UV = append(m.UV, packmesh.Vec2{f.UVs[2*i], f.UVs[2*i+1]})
		m.Joints = append(m.Joints, [4]int{f.BoneIndices[4*i], f.BoneIndices[4*i+1], f.BoneIndices[4*i+2], f.BoneIndices[4*i+3]})
		m.Weights = append(m.Weights, [4]float64{f.BoneWeights[4*i], f.BoneWeights[4*i+1], f.BoneWeights[4*i+2], f.BoneWeights[4*i+3]})
	}
	for i := 0; i+2 < len(f.Triangles); i += 3 {
		m.Tris = append(m.Tris, [3]int{f.Triangles[i], f.Triangles[i+1], f.Triangles[i+2]})
	}
	bones := make([]packmesh.Bone, len(f.Bones))
	for i, n := range f.Bones {
		bones[i] = packmesh.Bone{Name: n, Parent: f.BoneParents[i]}
		copy(bones[i].Bind[:], f.BindPoses[16*i:16*i+16])
	}
	pos, _, err := packmesh.Fold(m, bones)
	if err != nil {
		t.Fatal(err)
	}
	folded := &Mesh{Tris: m.Tris}
	for i := range pos {
		folded.Pos = append(folded.Pos, Vec3(pos[i]))
		folded.UV = append(folded.UV, Vec2(m.UV[i]))
	}
	shelf, err := LoadOBJ(filepath.Join(dir, f.Shelf+".obj"))
	if err != nil {
		t.Fatal(err)
	}
	tf, err := os.Open(filepath.Join(filepath.Dir(dir), "BasicCardPack_texture.png"))
	if err != nil {
		t.Fatal(err)
	}
	src, err := png.Decode(tf)
	tf.Close()
	if err != nil {
		t.Fatal(err)
	}
	tex := image.NewNRGBA(src.Bounds())
	draw.Draw(tex, tex.Bounds(), src, image.Point{}, draw.Src)
	// The open sheet (rest pose) from inside and outside: what shows once the pack is torn open.
	rest := &Mesh{Tris: m.Tris}
	for i := range m.Pos {
		rest.Pos = append(rest.Pos, Vec3(m.Pos[i]))
		rest.UV = append(rest.UV, Vec2(m.UV[i]))
	}
	if src := os.Getenv("PACKOPEN_VANILLA"); src != "" { // the game's own opening model (an OBJ), for comparison
		if v, err := LoadOBJ(src); err == nil {
			savePNGTest(filepath.Join(out, "packopen_vanilla_open_inside.png"), renderOrtho(v, tex, Vec3{0, 0, 1}, Vec3{0, 1, 0}, 700))
			savePNGTest(filepath.Join(out, "packopen_vanilla_open_outside.png"), renderOrtho(v, tex, Vec3{0, 0, -1}, Vec3{0, 1, 0}, 700))
		}
	}
	savePNGTest(filepath.Join(out, "packopen_open_inside.png"), renderOrtho(rest, tex, Vec3{0, 0, 1}, Vec3{0, 1, 0}, 700))
	savePNGTest(filepath.Join(out, "packopen_open_outside.png"), renderOrtho(rest, tex, Vec3{0, 0, -1}, Vec3{0, 1, 0}, 700))
	pm, _ := ModelFor("Pack")
	faces := labelTexture(pm)
	// The shelf pack's print faces -z, the opening pack's +z: each seen from its own front, back and sides.
	for _, v := range []struct {
		name        string
		shelf, open Vec3
	}{{"front", Vec3{0, 0, 1}, Vec3{0, 0, -1}}, {"back", Vec3{0, 0, -1}, Vec3{0, 0, 1}},
		{"left", Vec3{1, 0, 0}, Vec3{-1, 0, 0}}, {"right", Vec3{-1, 0, 0}, Vec3{1, 0, 0}}} {
		savePNGTest(filepath.Join(out, "packopen_shelf_"+v.name+".png"), renderOrtho(shelf, tex, v.shelf, Vec3{0, 1, 0}, 500))
		savePNGTest(filepath.Join(out, "packopen_opening_"+v.name+".png"), renderOrtho(folded, tex, v.open, Vec3{0, 1, 0}, 500))
		// Every face its own colour and label: art of one face showing on another (an edge on the front) stands out.
		savePNGTest(filepath.Join(out, "packopen_shelf_"+v.name+"_faces.png"), renderOrtho(shelf, faces, v.shelf, Vec3{0, 1, 0}, 500))
		savePNGTest(filepath.Join(out, "packopen_opening_"+v.name+"_faces.png"), renderOrtho(folded, faces, v.open, Vec3{0, 1, 0}, 500))
	}
}

package gameextract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/packmesh"
	"tcgstudio/internal/unityfs"
	"tcgstudio/internal/uvmap"
)

// Pack models the mod uses, built from the game files into the templates' accessories folder (TCG Studio names that folder in
// the game's studio-paths.json "packModels"):
//   - <shelf mesh>.fixed.obj: the shelf pack with one continuous back (packmesh.FixBack), for custom packs (Runtime/PackModels)
//     and Studio's pack editor. Vanilla packs keep the game's model: their art was painted for it.
//   - PackOpenFile / PackOpenFixedFile: the pack in the pack opening (the game's opening model re-textured from the vanilla /
//     the fixed shelf pack, internal/packmesh), so a pack opens looking as it does on the shelf.
const (
	PackOpenFile      = "packopen.json"
	PackOpenFixedFile = "packopen.fixed.json"
	FixedShelfSuffix  = ".fixed" // <shelf mesh name> + this + ".obj"
)

// PackOpenVersion changes when the opening files' format or the way they are built changes.
const PackOpenVersion = 1

// PackOpen is PackOpenFile. Every list is flat: positions/normals 3 per vertex, uvs 2, boneIndices/boneWeights 4,
// triangles 3; bind poses 16 per bone (row-major); all in Unity mesh space.
type PackOpen struct {
	Version     int       `json:"version"`
	Mesh        string    `json:"mesh"`        // the opening model it was made from
	Shelf       string    `json:"shelf"`       // the shelf pack model whose UVs it carries
	Bones       []string  `json:"bones"`       // the renderer's bones, in its order (the mod checks them by name)
	BoneParents []int     `json:"boneParents"` // each bone's parent in Bones (-1 = none)
	BindPoses   []float32 `json:"bindPoses"`
	Positions   []float32 `json:"positions"`
	Normals     []float32 `json:"normals"`
	UVs         []float32 `json:"uvs"`
	BoneIndices []int     `json:"boneIndices"`
	BoneWeights []float32 `json:"boneWeights"`
	Triangles   []int     `json:"triangles"`
}

// shelfModel is a shelf pack model an opening pack is made from.
type shelfModel struct {
	mesh packmesh.Mesh
	name string
}

// packModels writes the fixed shelf pack (always, so the pack editor has its model: unchanged when the back can't be fixed)
// and then the two opening packs.
func (x *extractor) packModels() error {
	shelfObj, err := x.env.Resolve(x.so.File, x.so.Meshes[0].Mesh)
	if err != nil || shelfObj == nil {
		return fmt.Errorf("the shelf pack's model wasn't found")
	}
	shelf, err := unityfs.ReadMesh(x.env, shelfObj)
	if err != nil {
		return err
	}
	sm := toPackMesh(shelf)
	fixed := sm
	back, err := packFaceArt("back")
	if err == nil {
		fixed, _, err = packmesh.FixBack(sm, back)
	}
	if err != nil {
		x.warn("pack back: %v (custom packs keep the game's back)", err)
		fixed = sm
	}
	fixedMesh := *shelf
	fixedMesh.Name = shelf.Name + FixedShelfSuffix
	fixedMesh.UV = make([][2]float32, len(fixed.UV))
	for i, uv := range fixed.UV {
		fixedMesh.UV[i] = [2]float32{float32(uv[0]), float32(uv[1])}
	}
	obj := filepath.Join(x.acc, fixedMesh.Name+".obj")
	if err := os.WriteFile(obj, []byte(fixedMesh.OBJ("extracted by TCG Studio from the game files, back fixed")), 0o644); err != nil {
		return err
	}
	return x.packOpen(map[string]shelfModel{PackOpenFile: {sm, shelf.Name}, PackOpenFixedFile: {fixed, fixedMesh.Name}})
}

// packOpen builds an opening pack per shelf model (file -> shelf pack) from the shop scene's CardOpeningSequence pack.
func (x *extractor) packOpen(shelves map[string]shelfModel) error {
	scene, err := x.shopScene()
	if err != nil {
		return err
	}
	g := newGraph(x.env, scene)
	var rendObj *unityfs.Object
	var meshPtr unityfs.PPtr
	for _, n := range g.byGO {
		if !n.has("CardOpeningSequence") {
			continue
		}
		n.walk(func(c *node) {
			for _, co := range c.comps {
				if co.ClassID != unityfs.ClassSkinnedMesh || rendObj != nil {
					continue
				}
				if _, mp, err := unityfs.ReadSkinnedMesh(co); err == nil {
					if mo, _ := x.env.Resolve(scene, mp); mo != nil {
						if name, _ := mo.Name(); strings.Contains(name, "CardPack") {
							rendObj, meshPtr = co, mp
						}
					}
				}
			}
		})
	}
	if rendObj == nil {
		return fmt.Errorf("the pack opening's pack model wasn't found")
	}
	mo, err := x.env.Resolve(scene, meshPtr)
	if err != nil || mo == nil {
		return fmt.Errorf("the pack opening's model: %v", err)
	}
	open, err := unityfs.ReadMesh(x.env, mo)
	if err != nil {
		return err
	}
	bonePtrs, _, err := unityfs.ReadSkinnedMeshBones(rendObj)
	if err != nil {
		return err
	}
	if len(bonePtrs) != len(open.BindPoses) {
		return fmt.Errorf("the opening pack has %d bones but %d bind poses", len(bonePtrs), len(open.BindPoses))
	}
	bones := make([]packmesh.Bone, len(bonePtrs))
	index := map[*node]int{}
	nodes := make([]*node, len(bonePtrs))
	for i, b := range bonePtrs {
		n := g.transformNode(b)
		if n == nil {
			return fmt.Errorf("bone %d of the opening pack wasn't found", i)
		}
		nodes[i], index[n] = n, i
	}
	for i, n := range nodes {
		bones[i] = packmesh.Bone{Name: n.name, Parent: -1}
		for p := n.parent; p != nil; p = p.parent {
			if j, ok := index[p]; ok {
				bones[i].Parent = j
				break
			}
		}
		for k := range 16 {
			bones[i].Bind[k] = float64(open.BindPoses[i][k])
		}
	}

	om := toPackMesh(open)
	if _, err := packmesh.Unpinch(&om, bones); err != nil {
		return err
	}
	pos, norm, err := packmesh.Fold(om, bones)
	if err != nil {
		return err
	}
	edge, err := packFaceArt("edges")
	if err != nil {
		return err
	}
	for file, shelf := range shelves {
		out, st := packmesh.Remap(om, pos, norm, bones, shelf.mesh, edge)
		if st.Outside == 0 {
			return fmt.Errorf("nothing of the opening pack faces outwards when folded (the game's model changed)")
		}
		if st.UncoveredArea > st.Area/50 {
			return fmt.Errorf("the opening pack and the shelf pack don't line up (%.1f%% of its outside off the shelf pack)", 100*st.UncoveredArea/st.Area)
		}
		f := PackOpen{Version: PackOpenVersion, Mesh: open.Name, Shelf: shelf.name}
		for i, b := range bones {
			f.Bones, f.BoneParents = append(f.Bones, b.Name), append(f.BoneParents, b.Parent)
			for k := range 16 {
				f.BindPoses = append(f.BindPoses, open.BindPoses[i][k])
			}
		}
		for i := range out.Pos {
			for k := range 3 {
				f.Positions = append(f.Positions, float32(out.Pos[i][k]))
				f.Normals = append(f.Normals, float32(out.Norm[i][k]))
			}
			f.UVs = append(f.UVs, float32(out.UV[i][0]), float32(out.UV[i][1]))
			for k := range 4 {
				f.BoneIndices = append(f.BoneIndices, out.Joints[i][k])
				f.BoneWeights = append(f.BoneWeights, float32(out.Weights[i][k]))
			}
		}
		for _, t := range out.Tris {
			f.Triangles = append(f.Triangles, t[0], t[1], t[2])
		}
		if err := writeJSON(filepath.Join(x.acc, file), f); err != nil {
			return err
		}
	}
	return nil
}

// packFaceArt: whether a shelf pack UV is art of face id: inside a piece of the pack face map's face and not inside a piece
// of the faces it touches (the Edges rows overlap the Front and Back rows by a few pixels): edges not in the front, the back
// not in the edges.
func packFaceArt(id string) (func(packmesh.Vec2) bool, error) {
	m, ok := uvmap.ModelFor("Pack")
	if !ok {
		return nil, fmt.Errorf("no pack layout")
	}
	notIn := map[string]string{"edges": "front", "back": "edges"}[id]
	var want, front []uvmap.Target
	for _, f := range m.Faces {
		if f.ID == id {
			want = f.Targets
		}
		if notIn != "" && f.ID == notIn {
			front = f.Targets
		}
	}
	if len(want) == 0 {
		return nil, fmt.Errorf("the pack layout has no %s face", id)
	}
	size := float64(m.TextureSize)
	in := func(ts []uvmap.Target, x, y float64) bool {
		for _, t := range ts {
			if t.Paints(x, y) {
				return true
			}
		}
		return false
	}
	return func(uv packmesh.Vec2) bool {
		x, y := uv[0]*size, (1-uv[1])*size
		return in(want, x, y) && !in(front, x, y)
	}, nil
}

// toPackMesh: one submesh's triangles (the pack models have one), skin data when there is some.
func toPackMesh(m *unityfs.Mesh) packmesh.Mesh {
	var o packmesh.Mesh
	for i, p := range m.Pos {
		o.Pos = append(o.Pos, packmesh.Vec3{float64(p[0]), float64(p[1]), float64(p[2])})
		var n packmesh.Vec3
		if i < len(m.Normal) {
			n = packmesh.Vec3{float64(m.Normal[i][0]), float64(m.Normal[i][1]), float64(m.Normal[i][2])}
		}
		o.Norm = append(o.Norm, n)
		var uv packmesh.Vec2
		if i < len(m.UV) {
			uv = packmesh.Vec2{float64(m.UV[i][0]), float64(m.UV[i][1])}
		}
		o.UV = append(o.UV, uv)
	}
	if len(m.Weights) == len(m.Pos) && len(m.Joints) == len(m.Pos) {
		for i := range m.Pos {
			var j [4]int
			var w [4]float64
			for k := range 4 {
				j[k], w[k] = int(m.Joints[i][k]), float64(m.Weights[i][k])
			}
			o.Joints, o.Weights = append(o.Joints, j), append(o.Weights, w)
		}
	}
	for _, s := range m.Subs {
		for i := 0; i+2 < len(s); i += 3 {
			o.Tris = append(o.Tris, [3]int{int(s[i]), int(s[i+1]), int(s[i+2])})
		}
	}
	return o
}

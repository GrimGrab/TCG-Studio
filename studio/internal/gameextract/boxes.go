package gameextract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tcgstudio/internal/unityfs"
)

// Game look boxes (BoxesDir): the cardboard boxes (RestockManager's four package box prefabs), the bulk card boxes
// (items BulkBox_*), the paper shopping bags and the street billboards, each as its preview mesh + vanilla texture, listed
// in boxes.json. The editor paints over the vanilla UVs, so the mod only swaps the texture.

// BoxesDir is the templates sub folder of the box looks.
const BoxesDir = "boxes"

// Box is one paintable box look in boxes.json.
type Box struct {
	ID          string `json:"id"`          // restockBig, restockSmall, furnitureBox, gradedBox, or the bulk box's EItemType name
	Kind        string `json:"kind"`        // "cardboard", "bulk", "bag" or "street"
	TextureName string `json:"textureName"` // the vanilla texture the mod replaces
	Texture     string `json:"texture"`     // PNG, relative to BoxesDir
	Size        [2]int `json:"size"`
	Mesh        string `json:"mesh"` // OBJ, relative to BoxesDir (the closed box)
}

// The package box prefabs, in RestockManager's field order.
var packageBoxes = []string{"restockBig", "restockSmall", "furnitureBox", "gradedBox"}

func (x *extractor) boxes() error {
	dir := filepath.Join(x.dir, BoxesDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	meshes := x.newMeshSet(dir)
	var out []Box
	var errs []string
	if err := x.cardboardBoxes(dir, meshes, &out); err != nil {
		errs = append(errs, "cardboard: "+err.Error())
	}
	if err := x.bulkBoxes(dir, meshes, &out); err != nil {
		errs = append(errs, "bulk: "+err.Error())
	}
	if err := x.sceneLooks(dir, meshes, &out); err != nil {
		errs = append(errs, "bags/billboards: "+err.Error())
	}
	if err := writeJSON(filepath.Join(dir, "boxes.json"), map[string]any{"version": 1, "boxes": out}); err != nil {
		return err
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

func (x *extractor) cardboardBoxes(dir string, meshes *meshSet, out *[]Box) error {
	f, _ := x.env.File("level1")
	if f == nil {
		return fmt.Errorf("level1 not found")
	}
	rm := x.env.FindScripts(f, "RestockManager")
	if len(rm) == 0 {
		return fmt.Errorf("RestockManager not found")
	}
	prefabs, err := unityfs.ReadUIPrefabs(rm[0], len(packageBoxes)) // the script's first fields are the four prefabs
	if err != nil {
		return err
	}
	n := 0
	for i, id := range packageBoxes {
		po, _ := x.env.Resolve(rm[0].File, prefabs[i])
		if po == nil {
			x.warn("box %s: prefab not found", id)
			continue
		}
		gp, err := unityfs.ComponentGameObject(po)
		if err != nil {
			continue
		}
		root := newGraph(x.env, po.File).nodeOf(gp)
		if root == nil {
			continue
		}
		if b, ok := x.cardboardBox(dir, meshes, id, root); ok {
			*out = append(*out, b)
			n++
		} else {
			x.warn("box %s: no cardboard mesh found", id)
		}
	}
	if n == 0 {
		return fmt.Errorf("no box could be read")
	}
	return nil
}

// cardboardBox: the box's look is its CardboardBox_* model (other models in the prefab, e.g. the shelf box's unused
// PackagingBox01, are inactive leftovers). The closed static mesh is the preview; boxes without one (the graded-card box)
// use the animated model's mesh.
func (x *extractor) cardboardBox(dir string, meshes *meshSet, id string, root *node) (Box, bool) {
	var closed, rig unityfs.PPtr
	var tex unityfs.Texture2D
	var from *unityfs.File
	found := false
	root.walk(func(n *node) {
		if !strings.HasPrefix(n.name, "CardboardBox") || n.hasInParent("ShelfCompartment") || strings.EqualFold(n.parent.name, "Outline") {
			return
		}
		for _, c := range n.comps {
			var rend unityfs.Renderer
			var mesh unityfs.PPtr
			var err error
			switch c.ClassID {
			case unityfs.ClassMeshRenderer:
				rend, err = unityfs.ReadRenderer(c)
			case unityfs.ClassSkinnedMesh:
				rend, mesh, err = unityfs.ReadSkinnedMesh(c)
			default:
				continue
			}
			if err != nil || len(rend.Materials) == 0 {
				continue
			}
			_, t, ok := x.mainTexture(c.File, rend.Materials[0])
			if !ok {
				continue
			}
			if !found {
				tex, from, found = t, c.File, true
			} else if t.Name != tex.Name {
				x.warn("box %s: second texture %s", id, t.Name)
				continue
			}
			if c.ClassID == unityfs.ClassSkinnedMesh {
				rig = mesh
			} else if strings.HasSuffix(n.name, "StaticClosed") {
				closed = x.meshOf(n)
			}
		}
	})
	if !found {
		return Box{}, false
	}
	pick := closed
	if pick == (unityfs.PPtr{}) {
		pick = rig
	}
	file, m := meshes.export(from, pick)
	if m == nil || !x.saveTexture(tex, filepath.Join(dir, id+".png")) {
		return Box{}, false
	}
	return Box{ID: id, Kind: "cardboard", TextureName: tex.Name, Texture: id + ".png", Size: [2]int{tex.Width, tex.Height}, Mesh: file}, true
}

// meshOf is the mesh of a node's MeshFilter.
func (x *extractor) meshOf(n *node) unityfs.PPtr {
	for _, c := range n.comps {
		if c.ClassID == unityfs.ClassMeshFilter {
			if mf, err := unityfs.ReadMeshFilter(c); err == nil {
				return mf.Mesh
			}
		}
	}
	return unityfs.PPtr{}
}

// bulkBoxes: every BulkBox_* item (one mesh, a texture per variant).
func (x *extractor) bulkBoxes(dir string, meshes *meshSet, out *[]Box) error {
	f := x.so.File
	n := 0
	for i := 0; i < x.count(); i++ {
		name := x.itemNames[int32(i)]
		if !strings.HasPrefix(name, "BulkBox_") {
			continue
		}
		md := x.so.Meshes[i]
		file, m := meshes.export(f, md.Mesh)
		_, t, ok := x.mainTexture(f, md.Material)
		if m == nil || !ok || !x.saveTexture(t, filepath.Join(dir, name+".png")) {
			x.warn("bulk box %s: mesh or texture missing", name)
			continue
		}
		*out = append(*out, Box{ID: name, Kind: "bulk", TextureName: t.Name, Texture: name + ".png", Size: [2]int{t.Width, t.Height}, Mesh: file})
		n++
	}
	if n == 0 {
		return fmt.Errorf("no bulk box items found")
	}
	return nil
}

// The shop scene's looks found by object name: the bag customers carry out (on every customer's hand), the open bag on
// the cashier counter (also under the play tables) and the street billboards of building 32. The billboards' shown LOD is
// static-batched (a combined scene mesh), so the preview takes the LOD 1 copy of the same model; its first submesh holds the
// ads, the second the metal frame (another texture), left out of the preview.
var sceneLooks = []struct {
	id, kind, object string
	adsOnly          bool
}{
	{"shoppingBag", "bag", "PaperBagClosed", false},
	{"shoppingBagOpen", "bag", "PaperBagOpen", false},
	{"streetBillboard", "street", "mcp_building_32_billboards", true},
}

func (x *extractor) sceneLooks(dir string, meshes *meshSet, out *[]Box) error {
	f, _ := x.env.File("level1")
	if f == nil {
		return fmt.Errorf("level1 not found")
	}
	g := newGraph(x.env, f)
	var missing []string
	for _, s := range sceneLooks {
		found := false
		for _, n := range g.byGO {
			if found || n.name != s.object {
				continue
			}
			mp := x.meshOf(n)
			mo, _ := x.env.Resolve(f, mp)
			if mo == nil {
				continue
			}
			if name, _ := mo.Name(); strings.HasPrefix(name, "Combined Mesh") {
				continue
			}
			for _, c := range n.comps {
				if c.ClassID != unityfs.ClassMeshRenderer {
					continue
				}
				rend, err := unityfs.ReadRenderer(c)
				if err != nil || len(rend.Materials) == 0 {
					continue
				}
				_, t, ok := x.mainTexture(c.File, rend.Materials[0])
				if !ok || !x.saveTexture(t, filepath.Join(dir, s.id+".png")) {
					continue
				}
				file, m := meshes.export(f, mp)
				if m == nil {
					continue
				}
				if s.adsOnly && len(m.Subs) > 1 {
					one := *m
					one.Subs = m.Subs[:1]
					file = s.id + ".obj"
					if err := os.WriteFile(filepath.Join(dir, file), []byte(one.OBJ("extracted by TCG Studio from the game files")), 0o644); err != nil {
						continue
					}
				}
				*out = append(*out, Box{ID: s.id, Kind: s.kind, TextureName: t.Name, Texture: s.id + ".png", Size: [2]int{t.Width, t.Height}, Mesh: file})
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, s.id)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("not found: %s", strings.Join(missing, ", "))
	}
	return nil
}

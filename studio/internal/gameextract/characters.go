package gameextract

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"

	"tcgstudio/internal/imgfile"
	"tcgstudio/internal/unityfs"
)

// Customer characters for Studio's face editor (CharactersDir): each body's mesh (LOD1: every face slider, a quarter of
// LOD0's vertices) with its face sliders as sparse morphs, the hair / eyebrow / beard pieces, the textures the head draws
// with, and every character preset (CC_CharacterData: slider values, hair, colours, skin settings). The game applies a
// preset through CharacterCustomization (setBlendshapeByName value × 100 = blend shape weight, setHairByName,
// setColorProperty, setFloatProperty, setTextureProperty), which is what the mod repeats for a regular's own face.
// Meshes are turned like the clothes (yUp), so the clothes' OBJs sit on the body.

// CharactersDir is the templates subfolder of the character data.
const CharactersDir = "characters"

// charTextureMax caps the saved textures' size (the preview needs no more).
const charTextureMax = 1024

type CharactersFile struct {
	Version  int                 `json:"version"`
	Bodies   map[string]CharBody `json:"bodies"`   // "male", "female"
	Textures map[string]string   `json:"textures"` // texture name → PNG (relative to CharactersDir)
}

type CharBody struct {
	Mesh     string       `json:"mesh"`               // .bin (CharMesh format)
	Skeleton []CharBone   `json:"skeleton,omitempty"` // the Animator's bones, parents first; the meshes' joints index it
	Parts    []CharPart   `json:"parts"`              // per submesh
	Hair     []HairSlot   `json:"hair"`               // CharacterCustomization.HairTables order
	Presets  []CharPreset `json:"presets"`
}

// CharBone is one bone of a body's skeleton in Unity's space (local to its parent), at rest = the pose the meshes are bound
// in. Its Path is what the game's Animator (and the mod) finds it by. Both bodies share the bone names and paths (the
// Unreal mannequin layout: root/pelvis/spine_01…, 70 skinned bones), which is why one animation fits both.
type CharBone struct {
	Name   string     `json:"name"`
	Path   string     `json:"path"`   // from the body's GameObject (the Animator's): "root/pelvis/spine_01"
	Parent int        `json:"parent"` // index, -1 = top
	Pos    [3]float32 `json:"pos"`
	Rot    [4]float32 `json:"rot"` // x y z w
	Scale  [3]float32 `json:"scale"`
}

// CharPart is a submesh's material and its texture slots (property → texture name in Textures).
type CharPart struct {
	Material string            `json:"material"`
	Textures map[string]string `json:"textures"`
}

type HairSlot struct {
	Table  string     `json:"table"`  // Hair_M, Eyebrows, Beards…
	Shadow string     `json:"shadow"` // head material property the item's ShadowMap goes in
	Tint   string     `json:"tint"`   // head material property tinted with the hair colour
	Items  []HairItem `json:"items"`
}

type HairItem struct {
	Name    string `json:"name"`
	Mesh    string `json:"mesh,omitempty"`    // .bin, "" = no mesh (None, painted-only eyebrows)
	Texture string `json:"texture,omitempty"` // strand texture (alpha cut-out)
	Shadow  string `json:"shadow,omitempty"`  // texture painted on the head (eyebrows, hairline)
}

type CharPreset struct {
	Name      string             `json:"name"`
	Shapes    map[string]float32 `json:"shapes"`
	Hair      []string           `json:"hair"`
	HairColor string             `json:"hairColor,omitempty"`
	Floats    map[string]float32 `json:"floats"`
	Textures  map[string]string  `json:"textures"`
	Colors    map[string]string  `json:"colors"`
}

func (x *extractor) characters() error {
	f, err := x.env.File("resources.assets")
	if err != nil || f == nil {
		return fmt.Errorf("resources.assets: %v", err)
	}
	dir := filepath.Join(x.dir, CharactersDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	out := CharactersFile{Version: 2, Bodies: map[string]CharBody{}, Textures: map[string]string{}}
	byName := map[string]*unityfs.Object{}
	for _, o := range f.Objects {
		if o.ClassID == unityfs.ClassTexture2D {
			if n, err := o.Name(); err == nil && n != "" {
				byName[n] = o
			}
		}
	}
	saveTex := func(o *unityfs.Object) string {
		if o == nil {
			return ""
		}
		t, err := unityfs.ReadTexture2D(o)
		if err != nil {
			return ""
		}
		if _, ok := out.Textures[t.Name]; ok {
			return t.Name
		}
		if skipCharTexture(t.Name) {
			return ""
		}
		img, err := x.env.TextureImage(t)
		if err != nil {
			x.warn("character texture %s: %v", t.Name, err)
			return ""
		}
		file := "tex_" + safeName(t.Name) + ".png"
		if err := imgfile.Save(filepath.Join(dir, file), shrinkTo(img, charTextureMax)); err != nil {
			return ""
		}
		out.Textures[t.Name] = file
		return t.Name
	}
	texRef := func(from *unityfs.File, p unityfs.PPtr) string {
		if p.Null() {
			return ""
		}
		o, _ := x.env.Resolve(from, p)
		return saveTex(o)
	}
	for body, cb := range x.customerBodies() {
		o, cc := cb.obj, cb.cc
		var b CharBody
		gob, _ := x.env.Resolve(o.File, cc.GameObject)
		r, m, so := x.lodMesh(gob, "_LOD1")
		if m == nil {
			return fmt.Errorf("%s: no body mesh", body)
		}
		yUp(m)
		b.Skeleton = x.skeleton(gob)
		bones := boneIndex(b.Skeleton)
		b.Mesh = body + "_body.bin"
		if err := writeCharMesh(filepath.Join(dir, b.Mesh), m, isPreviewShape, x.skin(so, m, bones)); err != nil {
			return err
		}
		for _, mp := range r.Materials {
			part := CharPart{Textures: map[string]string{}}
			if mo, mat := x.material(gob.File, mp); mat != nil {
				part.Material = mat.Name
				for _, k := range mat.Order {
					if n := texRef(mo.File, mat.Textures[k].Texture); n != "" {
						part.Textures[k] = n
					}
				}
			}
			b.Parts = append(b.Parts, part)
		}
		for _, hp := range cc.HairTables {
			ho, err := x.env.Resolve(o.File, hp)
			if err != nil || ho == nil {
				continue
			}
			tab, err := unityfs.ReadHairTable(ho)
			if err != nil {
				x.warn("%s hair: %v", body, err)
				continue
			}
			slot := HairSlot{Table: tab.Name, Shadow: tab.ShadowProperty, Tint: tab.TintProperty}
			for _, it := range tab.Items {
				hi := HairItem{Name: it.Name, Shadow: texRef(ho.File, it.ShadowMap)}
				if !it.Mesh.Null() {
					if pg, _ := x.env.Resolve(ho.File, it.Mesh); pg != nil {
						if hr, hm, hso := x.lodMesh(pg, "_LOD1"); hm != nil {
							yUp(hm)
							hi.Mesh = body + "_hair_" + safeName(it.Name) + ".bin"
							if err := writeCharMesh(filepath.Join(dir, hi.Mesh), hm, isPreviewShape, x.skin(hso, hm, bones)); err != nil {
								return err
							}
							if len(hr.Materials) > 0 {
								if mo, mat := x.material(pg.File, hr.Materials[0]); mat != nil {
									if te, ok := mat.MainTexture(); ok {
										hi.Texture = texRef(mo.File, te.Texture)
									}
								}
							}
						}
					}
				}
				slot.Items = append(slot.Items, hi)
			}
			b.Hair = append(b.Hair, slot)
		}
		if po, err := x.env.Resolve(o.File, cc.Presets); err == nil && po != nil {
			_, ps, err := unityfs.ReadCharacterPresets(po)
			if err != nil {
				return err
			}
			for _, p := range ps {
				cp := CharPreset{Name: p.Name, Shapes: map[string]float32{}, Hair: p.HairNames, Floats: map[string]float32{},
					Textures: map[string]string{}, Colors: map[string]string{}}
				for _, s := range p.Blendshapes {
					cp.Shapes[s.Name] = s.Float
				}
				for _, h := range p.HairColor {
					if h.String != "" {
						cp.HairColor = "#" + strings.ToLower(h.String)
					}
				}
				for _, fl := range p.FloatProperties {
					cp.Floats[fl.Name] = fl.Float
				}
				for _, t := range p.TextureProperties {
					cp.Textures[t.Name] = t.String
					saveTex(byName[t.String]) // the skin textures and make-up the presets choose from (Resources.Load by name)
				}
				for _, c := range p.ColorProperties {
					cp.Colors[c.Name] = "#" + strings.ToLower(c.String)
				}
				b.Presets = append(b.Presets, cp)
			}
		}
		out.Bodies[body] = b
	}
	if len(out.Bodies) == 0 {
		return fmt.Errorf("no customer bodies found")
	}
	return writeJSON(filepath.Join(dir, "characters.json"), out)
}

// isPreviewShape: the shapes the face editor offers — what the character presets set (mod_*, shp_*, body_fat), the body
// shapes (body_curvy, body_skinny, body_muscular, body_breastsize) and the facial expressions (the face-tracking set:
// mouthSmileLeft, browInnerUp, …; the game never animates them, so one set stays as a resting expression). The lip-sync
// shapes are left out.
func isPreviewShape(name string) bool {
	return !lipSync[name]
}

var lipSync = map[string]bool{"sil": true, "aa": true, "E": true, "ih": true, "oh": true, "ou": true, "PP": true, "FF": true,
	"TH": true, "DD": true, "kk": true, "CH": true, "SS": true, "nn": true, "RR": true, "laughter": true}

// skipCharTexture: maps the preview doesn't draw (normal, curvature, scattering).
func skipCharTexture(name string) bool {
	return strings.HasSuffix(name, "_N") || strings.Contains(name, "Curvature") || strings.Contains(name, "SSS") ||
		strings.Contains(name, "Normal")
}

// lodMesh finds the SkinnedMeshRenderer under gob whose GameObject name ends in lod (else the first one, shadow meshes
// left out) and reads its mesh.
func (x *extractor) lodMesh(gob *unityfs.Object, lod string) (unityfs.Renderer, *unityfs.Mesh, *unityfs.Object) {
	type found struct {
		name string
		r    unityfs.Renderer
		mesh unityfs.PPtr
		file *unityfs.File
		obj  *unityfs.Object // the SkinnedMeshRenderer
	}
	var all []found
	var walk func(g *unityfs.Object, depth int)
	walk = func(g *unityfs.Object, depth int) {
		if g == nil || depth > 12 {
			return
		}
		gg, err := unityfs.ReadGameObject(g)
		if err != nil {
			return
		}
		for _, cp := range gg.Components {
			co, _ := x.env.Resolve(g.File, cp)
			if co == nil {
				continue
			}
			switch co.ClassID {
			case unityfs.ClassSkinnedMesh:
				if r, mp, err := unityfs.ReadSkinnedMesh(co); err == nil && !strings.Contains(gg.Name, "Shadow") {
					all = append(all, found{gg.Name, r, mp, co.File, co})
				}
			case unityfs.ClassTransform:
				if tr, err := unityfs.ReadTransform(co); err == nil {
					for _, ch := range tr.Children {
						cto, _ := x.env.Resolve(co.File, ch)
						if cto == nil {
							continue
						}
						if ctr, err := unityfs.ReadTransform(cto); err == nil {
							cgo, _ := x.env.Resolve(cto.File, ctr.GameObject)
							walk(cgo, depth+1)
						}
					}
				}
			}
		}
	}
	walk(gob, 0)
	if len(all) == 0 {
		return unityfs.Renderer{}, nil, nil
	}
	pick := all[0]
	for _, f := range all {
		if strings.HasSuffix(f.name, lod) {
			pick = f
			break
		}
	}
	mo, _ := x.env.Resolve(pick.file, pick.mesh)
	if mo == nil {
		return pick.r, nil, nil
	}
	m, err := unityfs.ReadMesh(x.env, mo)
	if err != nil || len(m.UV) != len(m.Pos) {
		return pick.r, nil, nil
	}
	return pick.r, m, pick.obj
}

// skeleton reads the bones under the body's GameObject (its "root" child and everything below), parents first.
func (x *extractor) skeleton(gob *unityfs.Object) []CharBone {
	_, tr, _ := x.goTransform(gob)
	var top *unityfs.Object
	for _, ch := range tr.Children {
		if co, _ := x.env.Resolve(gob.File, ch); co != nil {
			if _, name := x.transformName(co); name == "root" {
				top = co
			}
		}
	}
	var out []CharBone
	var walk func(o *unityfs.Object, parent int, path string)
	walk = func(o *unityfs.Object, parent int, path string) {
		t, err := unityfs.ReadTransform(o)
		if err != nil {
			return
		}
		_, name := x.transformName(o)
		if path != "" {
			path += "/"
		}
		path += name
		out = append(out, CharBone{Name: name, Path: path, Parent: parent, Pos: t.Position, Rot: t.Rotation, Scale: t.Scale})
		me := len(out) - 1
		for _, ch := range t.Children {
			if co, _ := x.env.Resolve(o.File, ch); co != nil {
				walk(co, me, path)
			}
		}
	}
	if top != nil {
		walk(top, -1, "")
	}
	return out
}

// goTransform returns a GameObject's Transform (object, data) and name.
func (x *extractor) goTransform(g *unityfs.Object) (*unityfs.Object, unityfs.Transform, string) {
	gg, err := unityfs.ReadGameObject(g)
	if err != nil {
		return nil, unityfs.Transform{}, ""
	}
	for _, cp := range gg.Components {
		if co, _ := x.env.Resolve(g.File, cp); co != nil && co.ClassID == unityfs.ClassTransform {
			t, _ := unityfs.ReadTransform(co)
			return co, t, gg.Name
		}
	}
	return nil, unityfs.Transform{}, gg.Name
}

// transformName returns a Transform's data and its GameObject's name.
func (x *extractor) transformName(o *unityfs.Object) (unityfs.Transform, string) {
	t, err := unityfs.ReadTransform(o)
	if err != nil {
		return t, ""
	}
	g, _ := x.env.Resolve(o.File, t.GameObject)
	if g == nil {
		return t, ""
	}
	gg, _ := unityfs.ReadGameObject(g)
	return t, gg.Name
}

func boneIndex(sk []CharBone) map[string]int {
	m := make(map[string]int, len(sk))
	for i, b := range sk {
		m[b.Name] = i
	}
	return m
}

// skin maps a skinned mesh's per-vertex bones (indices into its renderer's bones) to the body's skeleton by bone name; a
// bone the skeleton lacks follows its nearest parent that it has. Nil when the mesh isn't skinned.
func (x *extractor) skin(so *unityfs.Object, m *unityfs.Mesh, bones map[string]int) [][4]uint16 {
	if so == nil || len(m.Joints) != len(m.Pos) || len(bones) == 0 {
		return nil
	}
	list, _, err := unityfs.ReadSkinnedMeshBones(so)
	if err != nil {
		return nil
	}
	to := make([]uint16, len(list))
	for i, bp := range list {
		o, _ := x.env.Resolve(so.File, bp)
		for depth := 0; o != nil && depth < 64; depth++ {
			t, name := x.transformName(o)
			if k, ok := bones[name]; ok {
				to[i] = uint16(k)
				break
			}
			if t.Father.Null() {
				break
			}
			o, _ = x.env.Resolve(o.File, t.Father)
		}
	}
	out := make([][4]uint16, len(m.Joints))
	for v, j := range m.Joints {
		for k := range 4 {
			if int(j[k]) < len(to) {
				out[v][k] = to[j[k]]
			}
		}
	}
	return out
}

// writeCharMesh writes the preview mesh format Studio's lib/characterView.ts reads (little endian):
//
//	"TCGM" u32 version=3, u32 vertices, u32 submeshes, u32 index count per submesh, u32 shapes, u32 flags (1 = uv2,
//	2 = skin), f32 pos ×3n, f32 normal ×3n, f32 uv ×2n (V up; the body's parts lie in UV tiles 0–4: sample frac(uv)),
//	[f32 uv2 ×2n], [u16 bone ×4n (CharBody.Skeleton indices), f32 weight ×4n],
//	u32 indices per submesh, per shape: u32 name length, name (padded to 4), u32 deltas, u32 vertex ×deltas,
//	i16 dx dy dz ×deltas (units of 0.1 mm, full weight 100; padded to 4).
func writeCharMesh(path string, m *unityfs.Mesh, keep func(string) bool, joints [][4]uint16) error {
	var b bytes.Buffer
	w := func(v any) { binary.Write(&b, binary.LittleEndian, v) }
	pad := func() { b.Write(make([]byte, (4-b.Len()%4)%4)) }
	var shapes []unityfs.BlendShape
	for _, s := range m.Shapes {
		if keep(s.Name) && len(s.Frames) > 0 {
			shapes = append(shapes, s)
		}
	}
	hasUV2 := len(m.UV2) == len(m.Pos)
	b.WriteString("TCGM")
	w(uint32(3))
	w(uint32(len(m.Pos)))
	w(uint32(len(m.Subs)))
	for _, s := range m.Subs {
		w(uint32(len(s)))
	}
	w(uint32(len(shapes)))
	flags := uint32(0)
	if hasUV2 {
		flags |= 1
	}
	skinned := len(joints) == len(m.Pos) && len(m.Weights) == len(m.Pos)
	if skinned {
		flags |= 2
	}
	w(flags)
	for _, p := range m.Pos {
		w(p)
	}
	for i := range m.Pos {
		n := [3]float32{0, 1, 0}
		if i < len(m.Normal) {
			n = m.Normal[i]
		}
		w(n)
	}
	for _, uv := range m.UV {
		w(uv)
	}
	if hasUV2 {
		for _, uv := range m.UV2 {
			w(uv)
		}
	}
	if skinned {
		for _, j := range joints {
			w(j)
		}
		for _, wt := range m.Weights {
			w(wt)
		}
	}
	for _, s := range m.Subs {
		w(s)
	}
	q := func(v float32) int16 { return int16(math.Max(-32767, math.Min(32767, math.Round(float64(v)*10000)))) }
	for _, s := range shapes {
		fr := s.Frames[len(s.Frames)-1] // the full shape (weight 100)
		w(uint32(len(s.Name)))
		b.WriteString(s.Name)
		pad()
		w(uint32(len(fr.Deltas)))
		for _, d := range fr.Deltas {
			w(d.Index)
		}
		for _, d := range fr.Deltas {
			w([3]int16{q(d.Pos[0]), q(d.Pos[2]), q(-d.Pos[1])}) // yUp, like the vertices
		}
		pad()
	}
	return os.WriteFile(path, b.Bytes(), 0o644)
}

// shrinkTo scales an image down so its longer side is at most max.
func shrinkTo(img *image.NRGBA, max int) image.Image {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w <= max && h <= max {
		return img
	}
	k := float64(max) / math.Max(float64(w), float64(h))
	dst := image.NewNRGBA(image.Rect(0, 0, int(float64(w)*k), int(float64(h)*k)))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Src, nil)
	return dst
}

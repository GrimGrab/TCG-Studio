package gameextract

import (
	"image"
	"image/color"
	"os"
	"path/filepath"

	"tcgstudio/internal/imgfile"
	"tcgstudio/internal/unityfs"
)

// Since the 2026-10-27 update the street billboards are part of building 32's own mesh (mcpBuilding32), drawn from the
// shared street atlas Buildings4_AlbedoOpacity (4096²) next to shop fronts, cars and signs; the middle top board (ADN
// Systems) is gone. Billboard designs stay in the earlier 1024² layout (mcp_building_32_billboards_d): the template is
// that layout rebuilt from the atlas, the preview mesh is the building's ad faces with their UVs moved into it, and the
// mod paints each ad of a design into a copy of the atlas (Runtime/BoxLook StreetAds — the same table).

const (
	streetAtlas       = "Buildings4_AlbedoOpacity"
	streetAtlasObject = "mcpBuilding32"
	streetOldSize     = 1024.0
)

// streetAd is one ad: its rectangle in the earlier layout (pixels of 1024², top-left origin) and in the atlas (pixels of
// 4096²). turned = the atlas holds it turned a quarter clockwise: the ad's left edge at the atlas rect's top, its top edge
// at the right. Measured on the game files 2026-10-29 by fitting the atlas pixels to the earlier texture.
type streetAd struct {
	old, atlas [4]float64
	turned     bool
}

var streetAds = []streetAd{
	{old: [4]float64{39, 404, 980, 781}, atlas: [4]float64{0, 0, 614, 1295}, turned: true}, // big board (Spotlight Pizzeria)
	{old: [4]float64{31, 37, 489, 353}, atlas: [4]float64{0, 1295, 614, 1718}},             // top side boards (Euphoria)
}

// fractions of a point within a rectangle
func within(r [4]float64, x, y float64) (float64, float64) {
	return (x - r[0]) / (r[2] - r[0]), (y - r[1]) / (r[3] - r[1])
}

func inRect(r [4]float64, x, y, tol float64) bool {
	return x >= r[0]-tol && x <= r[2]+tol && y >= r[1]-tol && y <= r[3]+tol
}

// toAtlas maps a point of the earlier layout into the atlas.
func (a streetAd) toAtlas(x, y float64) (float64, float64) {
	fx, fy := within(a.old, x, y)
	if a.turned {
		return a.atlas[2] - fy*(a.atlas[2]-a.atlas[0]), a.atlas[1] + fx*(a.atlas[3]-a.atlas[1])
	}
	return a.atlas[0] + fx*(a.atlas[2]-a.atlas[0]), a.atlas[1] + fy*(a.atlas[3]-a.atlas[1])
}

// toOld maps a point of the atlas into the earlier layout.
func (a streetAd) toOld(x, y float64) (float64, float64) {
	fx, fy := within(a.atlas, x, y)
	if a.turned {
		fx, fy = fy, 1-fx
	}
	return a.old[0] + fx*(a.old[2]-a.old[0]), a.old[1] + fy*(a.old[3]-a.old[1])
}

// streetBillboardFromAtlas writes the billboard template (streetBillboard.png in the earlier layout + streetBillboard.obj
// with the building's ad faces) from the street atlas; false when this game version has no such building.
func (x *extractor) streetBillboardFromAtlas(f *unityfs.File, g *graph, meshes *meshSet, dir string) (Box, bool) {
	for _, n := range g.byGO {
		if n.name != streetAtlasObject {
			continue
		}
		mo, _ := x.env.Resolve(f, x.meshOf(n))
		if mo == nil {
			continue
		}
		for _, c := range n.comps {
			if c.ClassID != unityfs.ClassMeshRenderer {
				continue
			}
			rend, err := unityfs.ReadRenderer(c)
			if err != nil {
				continue
			}
			for slot, mp := range rend.Materials {
				_, t, ok := x.mainTexture(c.File, mp)
				if !ok || t.Name != streetAtlas {
					continue
				}
				m := meshes.mesh(mo)
				if m == nil || slot >= len(m.Subs) || len(m.UV) != len(m.Pos) {
					continue
				}
				atlas, err := x.env.TextureImage(t)
				if err != nil {
					x.warn("texture %s: %v", t.Name, err)
					return Box{}, false
				}
				if imgfile.SaveFast(filepath.Join(dir, "streetBillboard.png"), streetOldLayout(atlas, float64(t.Width))) != nil {
					return Box{}, false
				}
				ads := streetAdFaces(m, m.Subs[slot], n.tr.Rotation)
				if len(ads.Subs[0]) == 0 {
					x.warn("%s: no billboard faces on %s", streetAtlasObject, t.Name)
					return Box{}, false
				}
				if os.WriteFile(filepath.Join(dir, "streetBillboard.obj"), []byte(ads.OBJ("extracted by TCG Studio from the game files")), 0o644) != nil {
					return Box{}, false
				}
				return Box{ID: "streetBillboard", Kind: "street", TextureName: t.Name, Texture: "streetBillboard.png",
					Size: [2]int{int(streetOldSize), int(streetOldSize)}, Mesh: "streetBillboard.obj"}, true
			}
		}
	}
	return Box{}, false
}

// streetOldLayout: the ads cut from the atlas (side atlasSize px) into the earlier 1024² layout, black around them.
func streetOldLayout(atlas image.Image, atlasSize float64) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, int(streetOldSize), int(streetOldSize)))
	for i := 3; i < len(out.Pix); i += 4 {
		out.Pix[i] = 255
	}
	b := atlas.Bounds()
	scale := atlasSize / 4096
	for _, a := range streetAds {
		for y := int(a.old[1]); y < int(a.old[3]); y++ {
			for x := int(a.old[0]); x < int(a.old[2]); x++ {
				ax, ay := a.toAtlas(float64(x)+0.5, float64(y)+0.5)
				r, g, bl, _ := atlas.At(b.Min.X+int(ax*scale), b.Min.Y+int(ay*scale)).RGBA()
				out.SetNRGBA(x, y, color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), 255})
			}
		}
	}
	return out
}

// streetAdFaces: the triangles of tris that show an ad (at least half of it each way: not the boards' thin edges or a board
// showing a small patch of one), turned by rot (the building's rotation in the scene: it stands upside down in its own
// mesh space), as a mesh of their own with UVs in the earlier layout.
func streetAdFaces(m *unityfs.Mesh, tris []uint32, rot [4]float32) *unityfs.Mesh {
	out := &unityfs.Mesh{Name: "streetBillboard", Subs: [][]uint32{nil}}
	px := func(i uint32) (float64, float64) { // the table's atlas pixels (top-left origin)
		uv := m.UV[i]
		return float64(uv[0]) * 4096, (1 - float64(uv[1])) * 4096
	}
	for t := 0; t+2 < len(tris); t += 3 {
		for _, a := range streetAds {
			all := true
			lo, hi := [2]float64{1e9, 1e9}, [2]float64{-1e9, -1e9}
			for k := 0; k < 3; k++ {
				ax, ay := px(tris[t+k])
				if !inRect(a.atlas, ax, ay, 1) {
					all = false
					break
				}
				lo, hi = [2]float64{min(lo[0], ax), min(lo[1], ay)}, [2]float64{max(hi[0], ax), max(hi[1], ay)}
			}
			if !all || hi[0]-lo[0] < (a.atlas[2]-a.atlas[0])/2 || hi[1]-lo[1] < (a.atlas[3]-a.atlas[1])/2 {
				continue
			}
			for k := 0; k < 3; k++ {
				i := tris[t+k]
				ox, oy := a.toOld(px(i))
				out.Subs[0] = append(out.Subs[0], uint32(len(out.Pos)))
				out.Pos = append(out.Pos, rotate(rot, m.Pos[i]))
				if len(m.Normal) == len(m.Pos) {
					out.Normal = append(out.Normal, rotate(rot, m.Normal[i]))
				}
				out.UV = append(out.UV, [2]float32{float32(ox / streetOldSize), float32(1 - oy/streetOldSize)})
			}
			break
		}
	}
	if len(out.Normal) != len(out.Pos) {
		out.Normal = nil
	}
	return out
}

// rotate turns v by the unit quaternion q (x, y, z, w).
func rotate(q [4]float32, v [3]float32) [3]float32 {
	x, y, z, w := q[0], q[1], q[2], q[3]
	// t = 2 * cross(q.xyz, v); v' = v + w*t + cross(q.xyz, t)
	tx, ty, tz := 2*(y*v[2]-z*v[1]), 2*(z*v[0]-x*v[2]), 2*(x*v[1]-y*v[0])
	return [3]float32{v[0] + w*tx + (y*tz - z*ty), v[1] + w*ty + (z*tx - x*tz), v[2] + w*tz + (x*ty - y*tx)}
}

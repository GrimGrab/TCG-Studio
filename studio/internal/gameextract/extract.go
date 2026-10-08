// Package gameextract builds the templates folder Studio's editors read (the layout the mod's former in-game export wrote,
// mod ≤ 0.7.x: TemplateExport, AccessoryTemplateExport, PackTemplateExport, ShelfExport) straight from the player's
// installed game files, so nothing has to be run in game. Nothing of the game is shipped: everything
// is read from the local install at runtime.
package gameextract

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"tcgstudio/internal/unityfs"
)

// Version changes whenever the output changes (forces a new extraction).
const Version = 9

// Info is written as studio.json next to the extracted templates.
type Info struct {
	Extractor int       `json:"extractor"`
	Stamp     string    `json:"stamp"` // game files identity (see Stamp)
	Unity     string    `json:"unity"`
	Created   time.Time `json:"created"`
	Warnings  []string  `json:"warnings,omitempty"`
}

// DataDir finds the Unity data folder of a game install (<game>\<exe name>_Data).
func DataDir(gameDir string) (string, error) {
	ms, _ := filepath.Glob(filepath.Join(gameDir, "*_Data"))
	for _, m := range ms {
		if _, err := os.Stat(filepath.Join(m, "globalgamemanagers")); err == nil {
			return m, nil
		}
	}
	return "", fmt.Errorf("no Unity data folder in %s", gameDir)
}

// Stamp identifies the installed game files (size + modification time of the files read), so an update re-extracts.
func Stamp(gameDir string) (string, error) {
	data, err := DataDir(gameDir)
	if err != nil {
		return "", err
	}
	var parts []string
	for _, n := range []string{"globalgamemanagers", "globalgamemanagers.assets", "sharedassets0.assets", "sharedassets1.assets", "resources.assets", `Managed\Assembly-CSharp.dll`} {
		st, err := os.Stat(filepath.Join(data, n))
		if err != nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:%d:%d", filepath.Base(n), st.Size(), st.ModTime().Unix()))
	}
	return strings.Join(parts, ";"), nil
}

type extractor struct {
	env       *unityfs.Env
	dir, acc  string
	itemNames map[int32]string
	objNames  map[int32]string
	catNames  map[int32]string
	so        unityfs.StockItemData
	warnings  []string
	progress  func(string)
	texNames  map[*unityfs.Object]unityfs.Texture2D
	accMeshes *meshSet
}

// Extract writes the templates into outDir (replaced atomically: built in outDir+".new", then swapped in).
func Extract(gameDir, outDir string, progress func(string)) (info Info, err error) {
	if progress == nil {
		progress = func(string) {}
	}
	data, err := DataDir(gameDir)
	if err != nil {
		return info, err
	}
	stamp, _ := Stamp(gameDir)
	enums, err := unityfs.ReadEnums(filepath.Join(data, "Managed", "Assembly-CSharp.dll"), "EItemType", "EItemCategory", "EObjectType")
	if err != nil {
		return info, err
	}
	env, err := unityfs.Open(data)
	if err != nil {
		return info, err
	}
	defer env.Close()

	tmp := outDir + ".new"
	os.RemoveAll(tmp)
	x := &extractor{env: env, dir: tmp, acc: filepath.Join(tmp, "accessories"), progress: progress,
		itemNames: invert(enums["EItemType"]), catNames: invert(enums["EItemCategory"]), objNames: invert(enums["EObjectType"]), texNames: map[*unityfs.Object]unityfs.Texture2D{}}
	if err := os.MkdirAll(x.acc, 0o755); err != nil {
		return info, err
	}
	if err := x.run(); err != nil {
		os.RemoveAll(tmp)
		return info, err
	}
	gg, _ := env.File("globalgamemanagers.assets")
	info = Info{Extractor: Version, Stamp: stamp, Created: time.Now(), Warnings: x.warnings}
	if gg != nil {
		info.Unity = gg.Unity
	}
	b, _ := json.MarshalIndent(info, "", "  ")
	if err := os.WriteFile(filepath.Join(tmp, "studio.json"), b, 0o644); err != nil {
		return info, err
	}
	env.Close()
	old := outDir + ".old"
	os.RemoveAll(old)
	if _, err := os.Stat(outDir); err == nil {
		if err := os.Rename(outDir, old); err != nil {
			return info, err
		}
	}
	if err := os.Rename(tmp, outDir); err != nil {
		return info, err
	}
	os.RemoveAll(old)
	return info, nil
}

func invert(m map[string]int32) map[int32]string {
	out := map[int32]string{}
	for k, v := range m {
		out[v] = k
	}
	return out
}

func (x *extractor) warn(format string, a ...any) {
	x.warnings = append(x.warnings, fmt.Sprintf(format, a...))
}

func (x *extractor) run() error {
	x.progress("Reading the game's item list…")
	var soObj *unityfs.Object
	for _, name := range []string{"sharedassets1.assets", "sharedassets0.assets", "resources.assets"} {
		f, err := x.env.File(name)
		if err != nil || f == nil {
			continue
		}
		if found := x.env.FindScripts(f, "StockItemData_ScriptableObject"); len(found) > 0 {
			soObj = found[0]
			break
		}
	}
	if soObj == nil {
		return fmt.Errorf("the game's item list (StockItemData) wasn't found")
	}
	so, err := x.env.ReadStockItemData(soObj)
	if err != nil {
		return err
	}
	if len(so.Items) != len(so.Meshes) || len(so.Items) == 0 {
		return fmt.Errorf("item list looks wrong (%d items, %d meshes) — the game's format may have changed", len(so.Items), len(so.Meshes))
	}
	x.so = so
	// Sanity check against the enum: item i must be named like EItemType i (spot check on known entries).
	if n := x.itemNames[0]; n != "BasicCardPack" {
		return fmt.Errorf("unexpected item type 0 %q", n)
	}

	x.progress("Extracting pack and box art…")
	if err := x.packTemplates(); err != nil {
		return err
	}
	x.progress("Extracting accessories and figurines…")
	items, err := x.accessories()
	if err != nil {
		return err
	}
	x.progress("Extracting pack and box models…")
	packs, err := x.packMeshes()
	if err != nil {
		return err
	}
	x.progress("Extracting shelves…")
	prefab, shelves, tables, err := x.shelves()
	if err != nil {
		return err
	}
	x.progress("Extracting furniture…")
	if _, err := x.furniture(); err != nil {
		// Furniture is optional for the other editors: keep the rest of the templates.
		x.warn("furniture: %v", err)
	}
	acc := map[string]any{"version": 2, "items": items, "tables": tables, "itemPrefab": prefab, "shelves": shelves}
	if err := writeJSON(filepath.Join(x.acc, "accessories.json"), acc); err != nil {
		return err
	}
	return writeJSON(filepath.Join(x.acc, "packs.json"), map[string]any{"version": 1, "items": packs, "renderers": []any{}})
}

func (x *extractor) count() int {
	n := len(x.so.Items)
	if max, ok := invertBack(x.itemNames, "Max"); ok && int(max) < n {
		n = int(max)
	}
	return n
}

func invertBack(m map[int32]string, name string) (int32, bool) {
	for k, v := range m {
		if v == name {
			return k, true
		}
	}
	return 0, false
}

// ---- files

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// texture resolves a texture reference and returns its header (cached).
func (x *extractor) texture(from *unityfs.File, p unityfs.PPtr) (*unityfs.Object, unityfs.Texture2D, bool) {
	o, err := x.env.Resolve(from, p)
	if err != nil || o == nil || o.ClassID != unityfs.ClassTexture2D {
		return nil, unityfs.Texture2D{}, false
	}
	if t, ok := x.texNames[o]; ok {
		return o, t, true
	}
	t, err := unityfs.ReadTexture2D(o)
	if err != nil {
		x.warn("%v", err)
		return nil, t, false
	}
	x.texNames[o] = t
	return o, t, true
}

func (x *extractor) saveTexture(t unityfs.Texture2D, path string) bool {
	img, err := x.env.TextureImage(t)
	if err == nil {
		err = savePNG(path, img)
	}
	if err != nil {
		x.warn("texture %s: %v", t.Name, err)
		return false
	}
	return true
}

func (x *extractor) saveSprite(from *unityfs.File, p unityfs.PPtr, path string) (rect [2]float32, ok bool) {
	o, err := x.env.Resolve(from, p)
	if err != nil || o == nil || o.ClassID != unityfs.ClassSprite {
		return rect, false
	}
	s, err := unityfs.ReadSprite(o)
	if err != nil {
		x.warn("%v", err)
		return rect, false
	}
	img, err := x.env.SpriteImage(s)
	if err == nil {
		err = savePNG(path, img)
	}
	if err != nil {
		x.warn("icon %s: %v", s.Name, err)
		return rect, false
	}
	return [2]float32{s.Rect[2], s.Rect[3]}, true
}

// material reads a material reference (nil when missing).
func (x *extractor) material(from *unityfs.File, p unityfs.PPtr) (*unityfs.Object, *unityfs.Material) {
	o, err := x.env.Resolve(from, p)
	if err != nil || o == nil || o.ClassID != unityfs.ClassMaterial {
		return nil, nil
	}
	m, err := unityfs.ReadMaterial(o)
	if err != nil {
		x.warn("%v", err)
		return nil, nil
	}
	return o, &m
}

// mainTexture of a material reference.
func (x *extractor) mainTexture(from *unityfs.File, p unityfs.PPtr) (*unityfs.Object, unityfs.Texture2D, bool) {
	mo, m := x.material(from, p)
	if m == nil {
		return nil, unityfs.Texture2D{}, false
	}
	te, ok := m.MainTexture()
	if !ok {
		return nil, unityfs.Texture2D{}, false
	}
	return x.texture(mo.File, te.Texture)
}

// matInfo mirrors the former in-game export's material info (shader name not read: it isn't used by Studio).
func (x *extractor) matInfo(from *unityfs.File, p unityfs.PPtr) any {
	mo, m := x.material(from, p)
	if m == nil {
		return nil
	}
	tex := map[string]string{}
	for _, k := range m.Order {
		if _, t, ok := x.texture(mo.File, m.Textures[k].Texture); ok {
			tex[k] = fmt.Sprintf("%s %dx%d", t.Name, t.Width, t.Height)
		}
	}
	main := m.Textures["_MainTex"]
	if main.Scale == [2]float32{} {
		main.Scale = [2]float32{1, 1}
	}
	return map[string]any{"name": m.Name, "shader": nil, "textures": tex, "mainTextureScale": main.Scale, "mainTextureOffset": main.Offset}
}

// ---- meshes

// meshSet names exported meshes like the mod's ExportMesh: Safe(name).obj, "_<count>" when the name is taken.
type meshSet struct {
	x     *extractor
	dir   string
	done  map[*unityfs.Object]string
	files map[string]bool
	order int
	cache map[*unityfs.Object]*unityfs.Mesh
}

func (x *extractor) newMeshSet(dir string) *meshSet {
	return &meshSet{x: x, dir: dir, done: map[*unityfs.Object]string{}, files: map[string]bool{}, cache: map[*unityfs.Object]*unityfs.Mesh{}}
}

func safe(s string) string {
	if s == "" {
		s = "mesh"
	}
	var b strings.Builder
	for _, c := range s {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c > 127 && (unicode.IsLetter(c) || unicode.IsDigit(c)) {
			b.WriteRune(c)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func (ms *meshSet) mesh(o *unityfs.Object) *unityfs.Mesh {
	if m, ok := ms.cache[o]; ok {
		return m
	}
	m, err := unityfs.ReadMesh(ms.x.env, o)
	if err != nil {
		ms.x.warn("%v", err)
		m = nil
	}
	ms.cache[o] = m
	return m
}

// export writes the mesh once and returns its file name ("" when it can't be read), plus the decoded mesh.
func (ms *meshSet) export(from *unityfs.File, p unityfs.PPtr) (string, *unityfs.Mesh) {
	o, err := ms.x.env.Resolve(from, p)
	if err != nil || o == nil || o.ClassID != unityfs.ClassMesh {
		return "", nil
	}
	m := ms.mesh(o)
	if f, ok := ms.done[o]; ok {
		return f, m
	}
	name, _ := o.Name()
	file := safe(name) + ".obj"
	for ms.files[file] {
		file = safe(name) + "_" + fmt.Sprint(len(ms.done)) + ".obj"
	}
	if m == nil {
		ms.done[o] = ""
		return "", nil
	}
	ms.done[o] = file
	ms.files[file] = true
	if err := os.WriteFile(filepath.Join(ms.dir, file), []byte(m.OBJ("extracted by TCG Studio from the game files")), 0o644); err != nil {
		ms.x.warn("mesh %s: %v", name, err)
		ms.done[o] = ""
		return "", nil
	}
	return file, m
}

func bounds(m *unityfs.Mesh) any {
	if m == nil {
		return nil
	}
	return m.Bounds()
}

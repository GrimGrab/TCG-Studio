package epl

import (
	"fmt"
	"image"
	"strings"

	"tcgstudio/internal/unityfs"
)

// Assets indexes one mod bundle's sprites, textures, materials and meshes by name (EPL refers to them by name; it asks
// sprite names to be unique per bundle). Safe for concurrent reads.
type Assets struct {
	bundle    *unityfs.Bundle
	env       *unityfs.Env
	sprites   map[string]*unityfs.Object
	textures  map[string]*unityfs.Object
	materials map[string]*unityfs.Object
	meshes    map[string]*unityfs.Object
	objects   map[string]*unityfs.Object // GameObjects (prefab roots)
}

// OpenAssets opens a bundle and indexes it.
func OpenAssets(path string) (*Assets, error) {
	b, err := unityfs.OpenBundle(path)
	if err != nil {
		return nil, err
	}
	a := &Assets{bundle: b, env: unityfs.NewBundleEnv(b), sprites: map[string]*unityfs.Object{},
		textures: map[string]*unityfs.Object{}, materials: map[string]*unityfs.Object{}, meshes: map[string]*unityfs.Object{},
		objects: map[string]*unityfs.Object{}}
	files, err := a.env.BundleFiles()
	if err != nil {
		b.Close()
		return nil, err
	}
	for _, f := range files {
		for _, o := range f.Objects {
			var m map[string]*unityfs.Object
			switch o.ClassID {
			case unityfs.ClassSprite:
				m = a.sprites
			case unityfs.ClassTexture2D:
				m = a.textures
			case unityfs.ClassMaterial:
				m = a.materials
			case unityfs.ClassMesh:
				m = a.meshes
			case unityfs.ClassGameObject:
				m = a.objects
			default:
				continue
			}
			var name string
			var err error
			if o.ClassID == unityfs.ClassGameObject {
				var g unityfs.GameObject
				g, err = unityfs.ReadGameObject(o)
				name = g.Name
			} else {
				name, err = o.Name()
			}
			if err != nil {
				continue
			}
			if _, dup := m[strings.ToLower(name)]; !dup {
				m[strings.ToLower(name)] = o
			}
		}
	}
	return a, nil
}

// Close releases the bundle.
func (a *Assets) Close() { a.bundle.Close() }

// Env is the bundle's object environment (for meshes and prefab hierarchies).
func (a *Assets) Env() *unityfs.Env { return a.env }

func (a *Assets) Mesh(name string) *unityfs.Object       { return a.meshes[strings.ToLower(name)] }
func (a *Assets) GameObject(name string) *unityfs.Object { return a.objects[strings.ToLower(name)] }

// SpriteSize returns a sprite's size without decoding it, or an error saying why it can't be imported (missing, packed
// in an atlas, texture format not supported).
func (a *Assets) SpriteSize(name string) (w, h int, err error) {
	o := a.sprites[strings.ToLower(name)]
	if o == nil {
		if t := a.textures[strings.ToLower(name)]; t != nil { // some mods name the texture, not the sprite
			tex, err := unityfs.ReadTexture2D(t)
			if err != nil {
				return 0, 0, err
			}
			if !unityfs.Decodable(tex.Format) {
				return 0, 0, fmt.Errorf("texture format %d not supported", tex.Format)
			}
			return tex.Width, tex.Height, nil
		}
		return 0, 0, fmt.Errorf("image %q not in the bundle", name)
	}
	s, err := unityfs.ReadSprite(o)
	if err != nil {
		return 0, 0, err
	}
	if !s.Atlas.Null() {
		return 0, 0, fmt.Errorf("image %q is packed in a sprite atlas (not supported)", name)
	}
	to, err := a.env.Resolve(o.File, s.Texture)
	if err != nil || to == nil {
		return 0, 0, fmt.Errorf("image %q has no texture", name)
	}
	tex, err := unityfs.ReadTexture2D(to)
	if err != nil {
		return 0, 0, err
	}
	if !unityfs.Decodable(tex.Format) {
		return 0, 0, fmt.Errorf("image %q: texture format %d not supported", name, tex.Format)
	}
	return int(s.Rect[2] + 0.5), int(s.Rect[3] + 0.5), nil
}

// Image decodes a sprite by name (or, failing that, a texture of that name).
func (a *Assets) Image(name string) (*image.NRGBA, error) {
	if o := a.sprites[strings.ToLower(name)]; o != nil {
		s, err := unityfs.ReadSprite(o)
		if err != nil {
			return nil, err
		}
		return a.env.SpriteImage(s)
	}
	if t := a.textures[strings.ToLower(name)]; t != nil {
		return a.texture(t)
	}
	return nil, fmt.Errorf("image %q not in the bundle", name)
}

// MaterialTexture decodes a material's main texture (EPL items name a material). A texture with the material's name is
// accepted too.
func (a *Assets) MaterialTexture(name string) (*image.NRGBA, error) {
	if o := a.materials[strings.ToLower(name)]; o != nil {
		m, err := unityfs.ReadMaterial(o)
		if err != nil {
			return nil, err
		}
		te, ok := m.MainTexture()
		if !ok {
			return nil, fmt.Errorf("material %q has no main texture", name)
		}
		to, err := a.env.Resolve(o.File, te.Texture)
		if err != nil || to == nil {
			return nil, fmt.Errorf("material %q: texture missing", name)
		}
		return a.texture(to)
	}
	if t := a.textures[strings.ToLower(name)]; t != nil {
		return a.texture(t)
	}
	return nil, fmt.Errorf("material %q not in the bundle", name)
}

func (a *Assets) texture(o *unityfs.Object) (*image.NRGBA, error) {
	t, err := unityfs.ReadTexture2D(o)
	if err != nil {
		return nil, err
	}
	return a.env.TextureImage(t)
}

package project

import (
	"encoding/json"
	"path"
	"sort"
	"strings"
)

// fileExts are the extensions of files a set refers to (images and icons).
var fileExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".svg": true, ".gif": true, ".bmp": true}

// implicitFiles are used by name without being referenced (the art generator's default set icon).
var implicitFiles = []string{"images/set_icon.svg", "images/set_icon.png"}

// IsRelFile reports whether s looks like a relative path to a set file ("images/1.jpg", "MTG Alpha (Test)/images/1.jpg").
func IsRelFile(s string) bool {
	if s == "" || strings.ContainsAny(s, ":\\") || strings.HasPrefix(s, "/") || !strings.Contains(s, "/") {
		return false
	}
	for _, part := range strings.Split(s, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return fileExts[strings.ToLower(path.Ext(s))]
}

// Files lists every file the set refers to, as relative paths (each once, sorted): set.json's card images, card back and
// pack/box art; every file path in studio.json (pack art editor layers and bases, product photos, Smart-art parts); and
// the set icon the art generator picks up by name when it exists. Storage moves, leftover detection and setup export use
// it, so a file the set needs is never left behind or deleted.
func (p *Project) Files() []string {
	seen := map[string]bool{}
	add := func(rel string) {
		if IsRelFile(rel) {
			seen[rel] = true
		}
	}
	add(p.Set.CardBack)
	for _, c := range p.Set.Cards {
		add(c.Image)
	}
	for _, pk := range p.Set.Packs {
		add(pk.PackTexture)
		add(pk.PackIcon)
		add(pk.BoxTexture)
		add(pk.BoxIcon)
	}
	if p.Meta != nil {
		if b, err := json.Marshal(p.Meta); err == nil {
			var v any
			if json.Unmarshal(b, &v) == nil {
				walkStrings(v, add)
			}
		}
	}
	for _, rel := range implicitFiles {
		if fileExists(p.ImagePath(rel)) {
			seen[rel] = true
		}
	}
	out := make([]string, 0, len(seen))
	for rel := range seen {
		out = append(out, rel)
	}
	sort.Strings(out)
	return out
}

func walkStrings(v any, f func(string)) {
	switch t := v.(type) {
	case string:
		f(t)
	case []any:
		for _, x := range t {
			walkStrings(x, f)
		}
	case map[string]any:
		for _, x := range t {
			walkStrings(x, f)
		}
	}
}

// RepointFiles rewrites every reference to a file in set.json and studio.json through to (rel → new rel; unchanged when
// to returns rel). The caller saves the project.
func (p *Project) RepointFiles(to func(rel string) string) {
	re := func(s *string) {
		if IsRelFile(*s) {
			*s = to(*s)
		}
	}
	re(&p.Set.CardBack)
	for i := range p.Set.Cards {
		re(&p.Set.Cards[i].Image)
	}
	for i := range p.Set.Packs {
		pk := &p.Set.Packs[i]
		re(&pk.PackTexture)
		re(&pk.PackIcon)
		re(&pk.BoxTexture)
		re(&pk.BoxIcon)
	}
	if p.Meta == nil {
		return
	}
	b, err := json.Marshal(p.Meta)
	if err != nil {
		return
	}
	var v any
	if json.Unmarshal(b, &v) != nil {
		return
	}
	v = mapStrings(v, func(s string) string {
		if IsRelFile(s) {
			return to(s)
		}
		return s
	})
	if nb, err := json.Marshal(v); err == nil {
		var m Meta
		if json.Unmarshal(nb, &m) == nil {
			*p.Meta = m
		}
	}
}

func mapStrings(v any, f func(string) string) any {
	switch t := v.(type) {
	case string:
		return f(t)
	case []any:
		for i := range t {
			t[i] = mapStrings(t[i], f)
		}
	case map[string]any:
		for k := range t {
			t[k] = mapStrings(t[k], f)
		}
	}
	return v
}

// ArtFolder is the named shared folder the set's card art is kept in as its own (card images "<Name>/images/…", made by
// the Storage page's "Different art — keep as its own"), or "" for the set's ordinary shared art.
func (p *Project) ArtFolder() string {
	for _, c := range p.Set.Cards {
		if first, rest, ok := strings.Cut(c.Image, "/"); ok && first != "images" && strings.Contains(rest, "/") {
			return first
		}
	}
	return ""
}

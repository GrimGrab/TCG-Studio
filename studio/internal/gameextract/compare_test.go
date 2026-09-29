package gameextract

import (
	"bufio"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestCompareWithMod compares an extraction (EXTRACTED) with the mod's in-game export (MOD_TEMPLATES): same files, images
// within a small colour tolerance, meshes with the same triangles/bounds/UV range, JSON fields equal. Skips without both.
func TestCompareWithMod(t *testing.T) {
	mod, ext := os.Getenv("MOD_TEMPLATES"), os.Getenv("EXTRACTED")
	if mod == "" || ext == "" {
		t.Skip("set MOD_TEMPLATES and EXTRACTED")
	}
	for _, sub := range []string{"", "accessories"} {
		mf, _ := os.ReadDir(filepath.Join(mod, sub))
		for _, e := range mf {
			if e.IsDir() {
				continue
			}
			name := filepath.Join(sub, e.Name())
			a, b := filepath.Join(mod, name), filepath.Join(ext, name)
			if _, err := os.Stat(b); err != nil {
				t.Logf("MISSING %s", name)
				continue
			}
			switch strings.ToLower(filepath.Ext(name)) {
			case ".png":
				if msg := comparePNG(a, b); msg != "" {
					t.Logf("IMAGE %s: %s", name, msg)
				}
			case ".obj":
				if msg := compareOBJ(a, b); msg != "" {
					t.Logf("MESH %s: %s", name, msg)
				}
			}
		}
		ef, _ := os.ReadDir(filepath.Join(ext, sub))
		for _, e := range ef {
			if _, err := os.Stat(filepath.Join(mod, sub, e.Name())); err != nil && !e.IsDir() {
				t.Logf("EXTRA %s", filepath.Join(sub, e.Name()))
			}
		}
	}
	for _, j := range []string{"accessories.json", "packs.json"} {
		var a, b any
		readJSON(t, filepath.Join(mod, "accessories", j), &a)
		readJSON(t, filepath.Join(ext, "accessories", j), &b)
		var diffs []string
		diffJSON(j, normalizeShelves(a), normalizeShelves(b), &diffs)
		sort.Strings(diffs)
		for i, d := range diffs {
			if i == 5000 {
				t.Logf("... %d more", len(diffs)-5000)
				break
			}
			t.Log(d)
		}
	}
}

func readJSON(t *testing.T, path string, v any) {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

// normalizeShelves keeps prefab shelves only (the mod's scene shelves come from one player's save), keyed by name.
func normalizeShelves(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	if list, ok := m["shelves"].([]any); ok {
		byName := map[string]any{}
		for _, s := range list {
			sm := s.(map[string]any)
			if sm["isPrefab"] == true {
				byName[sm["name"].(string)] = sm
			}
		}
		m["shelves"] = byName
	}
	if list, ok := m["items"].([]any); ok {
		byType := map[string]any{}
		for _, s := range list {
			sm := s.(map[string]any)
			byType[sm["type"].(string)] = sm
		}
		m["items"] = byType
	}
	delete(m, "tables")
	delete(m, "renderers")
	return m
}

func diffJSON(path string, a, b any, out *[]string) {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			*out = append(*out, fmt.Sprintf("%s: %T vs %T", path, a, b))
			return
		}
		for k := range av {
			if _, ok := bv[k]; !ok {
				*out = append(*out, fmt.Sprintf("%s.%s: missing", path, k))
				continue
			}
			diffJSON(path+"."+k, av[k], bv[k], out)
		}
		for k := range bv {
			if _, ok := av[k]; !ok {
				*out = append(*out, fmt.Sprintf("%s.%s: extra", path, k))
			}
		}
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			*out = append(*out, fmt.Sprintf("%s: %v vs %v", path, short(a), short(b)))
			return
		}
		for i := range av {
			diffJSON(fmt.Sprintf("%s[%d]", path, i), av[i], bv[i], out)
		}
	case float64:
		bv, ok := b.(float64)
		if !ok || math.Abs(av-bv) > 1e-4*math.Max(1, math.Abs(av)) {
			*out = append(*out, fmt.Sprintf("%s: %v vs %v", path, a, b))
		}
	default:
		if !reflect.DeepEqual(a, b) {
			*out = append(*out, fmt.Sprintf("%s: %v vs %v", path, short(a), short(b)))
		}
	}
}

func short(v any) string {
	s := fmt.Sprint(v)
	if len(s) > 80 {
		s = s[:80] + "…"
	}
	return s
}

func loadPNG(p string) (image.Image, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func comparePNG(a, b string) string {
	ia, err := loadPNG(a)
	if err != nil {
		return err.Error()
	}
	ib, err := loadPNG(b)
	if err != nil {
		return err.Error()
	}
	if ia.Bounds() != ib.Bounds() {
		return fmt.Sprintf("size %v vs %v", ia.Bounds().Size(), ib.Bounds().Size())
	}
	var sum, max float64
	n := 0
	r := ia.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c1 := toNRGBA(ia.At(x, y))
			c2 := toNRGBA(ib.At(x, y))
			for k := 0; k < 4; k++ {
				d := math.Abs(float64(c1[k]) - float64(c2[k]))
				if k < 3 && c1[3] == 0 && c2[3] == 0 {
					d = 0 // colour under full transparency doesn't matter
				}
				sum += d
				if d > max {
					max = d
				}
				n++
			}
		}
	}
	mean := sum / float64(n)
	if mean > 1.5 || max > 40 {
		return fmt.Sprintf("mean diff %.2f, max %.0f", mean, max)
	}
	return ""
}

func toNRGBA(c interface{ RGBA() (r, g, b, a uint32) }) [4]uint8 {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return [4]uint8{0, 0, 0, 0}
	}
	return [4]uint8{uint8(r * 0xffff / a >> 8), uint8(g * 0xffff / a >> 8), uint8(b * 0xffff / a >> 8), uint8(a >> 8)}
}

type objStats struct {
	tris     int
	min, max [3]float64
	uvMin    [2]float64
	uvMax    [2]float64
	area     float64
}

func objFile(p string) (s objStats, err error) {
	f, err := os.Open(p)
	if err != nil {
		return s, err
	}
	defer f.Close()
	var pos [][3]float64
	for i := 0; i < 3; i++ {
		s.min[i], s.max[i] = math.Inf(1), math.Inf(-1)
	}
	s.uvMin, s.uvMax = [2]float64{math.Inf(1), math.Inf(1)}, [2]float64{math.Inf(-1), math.Inf(-1)}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		fs := strings.Fields(sc.Text())
		if len(fs) == 0 {
			continue
		}
		switch fs[0] {
		case "v":
			var v [3]float64
			for i := 0; i < 3; i++ {
				v[i], _ = strconv.ParseFloat(fs[1+i], 64)
				s.min[i], s.max[i] = math.Min(s.min[i], v[i]), math.Max(s.max[i], v[i])
			}
			pos = append(pos, v)
		case "vt":
			for i := 0; i < 2; i++ {
				v, _ := strconv.ParseFloat(fs[1+i], 64)
				s.uvMin[i], s.uvMax[i] = math.Min(s.uvMin[i], v), math.Max(s.uvMax[i], v)
			}
		case "f":
			s.tris++
			var ix [3]int
			for k := 0; k < 3; k++ {
				ix[k], _ = strconv.Atoi(strings.SplitN(fs[1+k], "/", 2)[0])
			}
			a, b, c := pos[ix[0]-1], pos[ix[1]-1], pos[ix[2]-1]
			u, v := [3]float64{b[0] - a[0], b[1] - a[1], b[2] - a[2]}, [3]float64{c[0] - a[0], c[1] - a[1], c[2] - a[2]}
			cr := [3]float64{u[1]*v[2] - u[2]*v[1], u[2]*v[0] - u[0]*v[2], u[0]*v[1] - u[1]*v[0]}
			s.area += math.Sqrt(cr[0]*cr[0]+cr[1]*cr[1]+cr[2]*cr[2]) / 2
		}
	}
	return s, sc.Err()
}

func compareOBJ(a, b string) string {
	sa, err := objFile(a)
	if err != nil {
		return err.Error()
	}
	sb, err := objFile(b)
	if err != nil {
		return err.Error()
	}
	var msgs []string
	if sa.tris != sb.tris {
		msgs = append(msgs, fmt.Sprintf("triangles %d vs %d", sa.tris, sb.tris))
	}
	for i := 0; i < 3; i++ {
		if math.Abs(sa.min[i]-sb.min[i]) > 1e-3 || math.Abs(sa.max[i]-sb.max[i]) > 1e-3 {
			msgs = append(msgs, fmt.Sprintf("bounds %v..%v vs %v..%v", sa.min, sa.max, sb.min, sb.max))
			break
		}
	}
	for i := 0; i < 2; i++ {
		if math.Abs(sa.uvMin[i]-sb.uvMin[i]) > 1e-3 || math.Abs(sa.uvMax[i]-sb.uvMax[i]) > 1e-3 {
			msgs = append(msgs, fmt.Sprintf("uv %v..%v vs %v..%v", sa.uvMin, sa.uvMax, sb.uvMin, sb.uvMax))
			break
		}
	}
	if math.Abs(sa.area-sb.area) > 1e-3*math.Max(1, sa.area) {
		msgs = append(msgs, fmt.Sprintf("area %.4f vs %.4f", sa.area, sb.area))
	}
	return strings.Join(msgs, "; ")
}

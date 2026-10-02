package modconfig

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// loopBinds lists the settings the mod binds in loops (section/key built at run time), keyed by a piece of the Bind call's
// source text. A Bind whose section or key the test can't read must be listed here, or TestDefaultsCoverMod fails — so a new
// setting can't slip past it unchecked.
var loopBinds = map[string][]string{
	`Config.Bind("Content", key,`: {"Content/ShowVanillaDeckBoxes", "Content/ShowVanillaPlaymats", "Content/ShowVanillaSleeves",
		"Content/ShowVanillaDice", "Content/ShowVanillaComics", "Content/ShowVanillaCollectionBooks", "Content/ShowVanillaBattleDecks",
		"Content/ShowVanillaFigurines"},
	`Config.Bind(foilSection, $"{grades[g]}`: foilGrades(),
	`Config.Bind(aiCol, i == 0 ? "AiDeckWeight1Color"`: {"MTG - AI deck colours/AiDeckWeight1Color",
		"MTG - AI deck colours/AiDeckWeight2Colors", "MTG - AI deck colours/AiDeckWeight3Colors",
		"MTG - AI deck colours/AiDeckWeight4Colors", "MTG - AI deck colours/AiDeckWeight5Colors"},
}

func foilGrades() []string {
	var out []string
	names := [][2]string{{"Base", "Base"}, {"FirstEdition", "First edition"}, {"Silver", "Silver"}, {"Gold", "Gold"}, {"EX", "EX"}, {"FullArt", "Full art"}}
	for _, g := range names {
		for _, k := range []string{"Color", "Strength", "Pattern"} {
			out = append(out, "Foil - "+g[1]+"/"+g[0]+k)
		}
	}
	return out
}

// TestDefaultsCoverMod fails when the mod binds a setting the built-in defaults.cfg doesn't have (refresh it, see defaults.go).
// Sections/keys may be string literals or `const string` names from the same file; anything else must be in loopBinds.
func TestDefaultsCoverMod(t *testing.T) {
	src := filepath.Join("..", "..", "..", "src", "TCGCustomCards")
	if _, err := os.Stat(src); err != nil {
		t.Skip("mod source not found")
	}
	tmp := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(tmp, defaultsCfg, 0o644); err != nil {
		t.Fatal(err)
	}
	secs, err := Read(tmp)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, s := range secs {
		for _, e := range s.Entries {
			have[s.Name+"/"+e.Key] = true
		}
	}
	check := func(sk string) {
		if !have[sk] {
			t.Errorf("defaults.cfg lacks %s — run: go run ./cmd/modcfgdefaults <game cfg>, or add the entry by hand", sk)
		}
	}
	bind := regexp.MustCompile(`(?:Config|config)\.Bind(?:<[^>]*>)?\(\s*([^,]+?)\s*,\s*([^,]+?)\s*,`)
	constDecl := regexp.MustCompile(`(\w+)\s*=\s*"([^"]*)"`)
	n := 0
	usedLoops := map[string]bool{}
	err = filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".cs") {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		text := string(b)
		consts := map[string]string{}
		for _, line := range strings.Split(text, "\n") {
			if i := strings.Index(line, "const string "); i >= 0 {
				for _, m := range constDecl.FindAllStringSubmatch(line[i:], -1) {
					consts[m[1]] = m[2]
				}
			}
		}
		resolve := func(arg string) (string, bool) {
			if strings.HasPrefix(arg, `"`) && strings.HasSuffix(arg, `"`) && len(arg) >= 2 {
				return arg[1 : len(arg)-1], true
			}
			v, ok := consts[arg]
			return v, ok
		}
		for _, loc := range bind.FindAllStringSubmatchIndex(text, -1) {
			n++
			call := text[loc[0]:loc[1]]
			sec, okS := resolve(text[loc[2]:loc[3]])
			key, okK := resolve(text[loc[4]:loc[5]])
			if okS && okK {
				check(sec + "/" + key)
				continue
			}
			known := false
			for prefix, keys := range loopBinds {
				if strings.HasPrefix(call, prefix) || strings.Contains(text[loc[0]:min(len(text), loc[0]+len(prefix)+5)], prefix) {
					known = true
					usedLoops[prefix] = true
					for _, sk := range keys {
						check(sk)
					}
				}
			}
			if !known {
				t.Errorf("%s: can't read the section/key of %q — add its keys to loopBinds in this test", filepath.Base(p), call)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("no Config.Bind calls found — did the mod source move?")
	}
	for prefix := range loopBinds {
		if !usedLoops[prefix] {
			t.Errorf("loopBinds entry %q no longer matches any Bind — update or remove it", prefix)
		}
	}
}

func TestWriteDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "BepInEx", "config", FileName)
	if err := WriteDefaults(path, filepath.Join(dir, "missing.cfg")); err != nil {
		t.Fatal(err)
	}
	secs, err := Read(path)
	if err != nil || len(secs) == 0 {
		t.Fatalf("Read = %v, %v", secs, err)
	}
	for _, s := range secs {
		for _, e := range s.Entries {
			if e.Value != e.Default {
				t.Errorf("[%s] %s = %q, default %q", s.Name, e.Key, e.Value, e.Default)
			}
		}
	}

	// A cached copy with fewer settings than the built-in one is ignored.
	small := filepath.Join(dir, "small.cfg")
	if err := os.WriteFile(small, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteDefaults(path, small); err != nil {
		t.Fatal(err)
	}
	if after, _ := Read(path); len(after) != len(secs) {
		t.Fatal("used the smaller cached copy")
	}
}

package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"

	"tcgstudio/internal/fontname"
)

type installedFont struct{ name, path string }

// installedFonts lists the .ttf/.otf fonts registered with Windows, for the whole machine and for this user (fonts installed
// "for me only" live in %LOCALAPPDATA%\Microsoft\Windows\Fonts). Names come from the registry ("Arial Bold (TrueType)" → "Arial Bold").
func installedFonts() []installedFont {
	const key = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`
	winFonts := filepath.Join(os.Getenv("WINDIR"), "Fonts")
	seen := map[string]bool{}
	var out []installedFont
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		k, err := registry.OpenKey(root, key, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		names, _ := k.ReadValueNames(-1)
		for _, n := range names {
			file, _, err := k.GetStringValue(n)
			if err != nil || !fontname.IsFont(file) {
				continue
			}
			if !filepath.IsAbs(file) {
				file = filepath.Join(winFonts, file)
			}
			if seen[strings.ToLower(file)] {
				continue
			}
			if _, err := os.Stat(file); err != nil {
				continue
			}
			seen[strings.ToLower(file)] = true
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(n, " (TrueType)"), " (OpenType)"))
			out = append(out, installedFont{name: name, path: file})
		}
		k.Close()
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].name) < strings.ToLower(out[j].name) })
	return out
}

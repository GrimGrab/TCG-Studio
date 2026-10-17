package main

import (
	"net/http"
	"path/filepath"
	"strings"

	astore "tcgstudio/internal/assets"
	"tcgstudio/internal/gameextract"
	"tcgstudio/internal/modconfig"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
)

// fileRoute serves files below one URL prefix. depth = how many path segments may follow the prefix (0 = any);
// file maps the checked relative path to a file ("" = not found). frontend/src/lib/urls.ts builds these URLs.
type fileRoute struct {
	prefix string
	depth  int
	file   func(a *App, rel string) string
}

var fileRoutes = []fileRoute{
	// set files (card art may live in the shared library)
	{"proj/", 0, func(a *App, rel string) string {
		id, r, ok := strings.Cut(rel, "/")
		if !ok || !setfmt.SafeID(id) || safeJoin(a.ws().Folder(id), r) == "" {
			return ""
		}
		return a.ws().ImagePath(id, r)
	}},
	{"templates/", 1, func(a *App, rel string) string { return safeJoin(a.templatesDir(), rel) }},
	{"acctemplates/", 1, func(a *App, rel string) string { return safeJoin(a.templatesDir(), "accessories", rel) }},
	{"furntemplates/", 1, func(a *App, rel string) string {
		return safeJoin(a.templatesDir(), gameextract.FurnitureDir, rel)
	}},
	{"decotemplates/", 1, func(a *App, rel string) string {
		return safeJoin(a.templatesDir(), gameextract.DecorationsDir, rel)
	}},
	// accessory, furniture and decoration files: the shared store
	{"acc/", 0, func(a *App, rel string) string {
		if !astore.IsAsset(rel) {
			return ""
		}
		return safeJoin(a.store().Dir, strings.TrimPrefix(rel, astore.Prefix))
	}},
	{"globalback/", 1, func(a *App, rel string) string {
		if rel != globalBackFile {
			return ""
		}
		return safeJoin(a.root(), setups.GameStateDir, rel)
	}},
	// Game look previews (sign layout, fonts, phone, open/closed sign) and the chosen pictures and fonts
	{"signtemplates/", 2, func(a *App, rel string) string {
		return safeJoin(a.templatesDir(), gameextract.ShopSignDir, rel)
	}},
	{"shopsign/", 1, func(a *App, rel string) string {
		return safeJoin(a.root(), setups.GameStateDir, modconfig.ShopSignFolder, rel)
	}},
}

// fileHandler serves the files the frontend shows (see fileRoutes).
func (a *App) fileHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if file := a.routeFile(strings.TrimPrefix(r.URL.Path, "/")); file != "" {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, file)
			return
		}
		http.NotFound(w, r)
	})
}

// routeFile is the file a URL path (without the leading "/") serves, or "".
func (a *App) routeFile(path string) string {
	for _, rt := range fileRoutes {
		rel, ok := strings.CutPrefix(path, rt.prefix)
		if !ok {
			continue
		}
		if !safeRel(rel, rt.depth) {
			return ""
		}
		return rt.file(a, rel)
	}
	return ""
}

// safeRel reports whether rel is a plain relative URL path: forward slashes, no "." / ".." / empty segments, at most
// depth segments (0 = any).
func safeRel(rel string, depth int) bool {
	if rel == "" || strings.ContainsAny(rel, `\:`) {
		return false
	}
	segs := strings.Split(rel, "/")
	if depth > 0 && len(segs) > depth {
		return false
	}
	for _, s := range segs {
		if s == "" || s == "." || s == ".." {
			return false
		}
	}
	return true
}

// safeJoin joins a checked relative path (slashes) onto root and keeps the result inside root. An empty or relative
// root (no game folder or templates yet) gives "", so nothing is served relative to Studio's working directory.
func safeJoin(root string, rel ...string) string {
	if root == "" || !filepath.IsAbs(root) {
		return ""
	}
	p := filepath.Join(append([]string{root}, rel...)...)
	if r, err := filepath.Rel(root, p); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return ""
	}
	return p
}

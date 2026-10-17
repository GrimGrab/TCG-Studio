package main

import (
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/game"
)

func TestRouteFile(t *testing.T) {
	ws := t.TempDir()
	a := &App{settings: Settings{Workspace: ws}} // no game folder: templates and plugin routes have no root
	setup := filepath.Join(ws, "setups", "default")
	a.setRoot(setup)
	lib := filepath.Join(ws, "library", "abc", "images")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lib, "1.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		path, want string
	}{
		{"proj/abc/images/1.png", filepath.Join(lib, "1.png")},
		{"proj/abc/../../x.png", ""},
		{"proj/../abc/images/1.png", ""},
		{"proj/a:b/images/1.png", ""},
		{`proj/abc/images\1.png`, ""},
		{"proj/abc", ""},
		{"acc/assets/0123456789abcdef.png", filepath.Join(ws, "assets", "0123456789abcdef.png")},
		{"acc/images/a.png", ""}, // only shared store files
		{"acc/../setup.json", ""},
		{"acc/./x.png", ""},
		{"templates/BasicCardPack_texture.png", ""}, // no game folder → no templates root
		{"templates/../x.png", ""},
		{"furntemplates/sub/x.json", ""},
		{"signtemplates/fonts/x.ttf", ""},
		{"shopsign/sign.png", filepath.Join(setup, "game", "ShopSign", "sign.png")}, // the setup's Game look files
		{"shopsign/../x.png", ""},
		{"globalback/card_back.png", filepath.Join(setup, "game", "card_back.png")},
		{"globalback/other.png", ""},
		{"unknown/x.png", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := a.routeFile(c.path); got != c.want {
			t.Errorf("routeFile(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestSafeJoin(t *testing.T) {
	root := t.TempDir()
	if got := safeJoin(root, "a", "b.png"); got != filepath.Join(root, "a", "b.png") {
		t.Errorf("got %q", got)
	}
	for _, bad := range [][]string{{".."}, {"a", "..", ".."}} {
		if got := safeJoin(root, bad...); got != "" {
			t.Errorf("safeJoin(%v) = %q, want \"\"", bad, got)
		}
	}
	if safeJoin("", "x") != "" || safeJoin(filepath.Join("rel", "dir"), "x") != "" {
		t.Error("empty or relative root must give \"\"")
	}
}

// Studio points the game at the active setup and the workspace's shared folders.
func TestPointGameAtWorkspace(t *testing.T) {
	ws, gameDir := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(gameDir, "Card Shop Simulator_Data"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := &App{settings: Settings{Workspace: ws, GameDir: gameDir}}
	setup := filepath.Join(ws, "setups", "default")
	a.setRoot(setup)
	a.pointGameAtWorkspace()
	sp := game.ReadStudioPaths(gameDir)
	if !game.SameDir(sp.Setup, setup) || !game.SameDir(sp.Library, filepath.Join(ws, "library")) || !game.SameDir(sp.Assets, filepath.Join(ws, "assets")) {
		t.Fatalf("studio paths %+v", sp)
	}
}

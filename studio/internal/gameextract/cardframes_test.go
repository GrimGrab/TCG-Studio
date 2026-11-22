package gameextract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"tcgstudio/internal/unityfs"
)

// The game's card rims read from the installed game (skipped without it): every frame template has six borders, Tetramon's
// Gold has a rim picture, and its Full Art style draws none.
func TestCardFrames(t *testing.T) {
	game := os.Getenv("TCG_GAME")
	if game == "" {
		game = `D:\SteamLibrary\steamapps\common\TCG Card Shop Simulator`
	}
	data, err := DataDir(game)
	if err != nil {
		t.Skip(err)
	}
	enums, err := unityfs.ReadEnums(filepath.Join(data, "Managed", "Assembly-CSharp.dll"), "ECardExpansionType")
	if err != nil {
		t.Fatal(err)
	}
	env, err := unityfs.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	defer env.Close()
	if env.Scripts, err = unityfs.ScriptLayoutOf(filepath.Join(data, "Managed", "Assembly-CSharp.dll")); err != nil {
		t.Fatal(err)
	}
	dir := os.Getenv("FRAMES_OUT")
	if dir == "" {
		dir = t.TempDir()
	}
	x := &extractor{env: env, dir: dir, progress: func(string) {}}
	if err := x.cardFrames(invert(enums["ECardExpansionType"])); err != nil {
		t.Fatal(err)
	}
	for _, w := range x.warnings {
		t.Log("warning:", w)
	}
	b, err := os.ReadFile(filepath.Join(dir, CardFramesDir, CardFramesFile))
	if err != nil {
		t.Fatal(err)
	}
	var list []CardFrame
	if err := json.Unmarshal(b, &list); err != nil {
		t.Fatal(err)
	}
	got := map[string]CardFrame{}
	for _, f := range list {
		got[f.Template+"/"+f.Border] = f
		t.Logf("%s %s → %q", f.Template, f.Border, f.File)
	}
	gold := got["Tetramon/Gold"]
	if gold.File == "" {
		t.Fatalf("Tetramon Gold has no rim: %+v", list)
	}
	if _, err := os.Stat(filepath.Join(dir, CardFramesDir, gold.File)); err != nil {
		t.Fatal(err)
	}
	if f, ok := got["Tetramon/FullArt"]; !ok || f.File != "" {
		t.Errorf("Tetramon Full Art: %+v", f)
	}
}

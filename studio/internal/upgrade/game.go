package upgrade

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/assets"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/game"
	"tcgstudio/internal/modconfig"
	"tcgstudio/internal/project"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/workspace"
)

// Older Studio versions kept the active setup's content in the game folder: its installed sets (<plugin>\Sets\<id>, files in
// <plugin>\Library\<id>), the accessory library (<plugin>\Accessories), the global card back (<plugin>\card_back.png) and the
// Game look pictures and fonts (<plugin>\ShopSign). A switch parked an inactive setup's hand-installed sets and accessories
// in <setup>\game\Sets and game\Accessories. The mod now reads everything from the setup (game.StudioPathsFile), so each
// setup takes that content over once (setups.Info.AdoptedGame); afterwards the game folder's copies are only leftovers the
// Storage page may remove. Files are copied, never moved.

// GameState is what a setup has not taken over yet from the game folder (the active setup) or its parked game\ folder.
type GameState struct {
	Setup       string   `json:"setup"`       // the active setup's name
	Sets        []string `json:"sets"`        // installed sets that are projects of the setup
	Imported    []string `json:"imported"`    // installed sets without a project: become projects of the setup
	Accessories bool     `json:"accessories"` // an accessory library the setup doesn't have
	CardBack    bool     `json:"cardBack"`    // a global card back the setup doesn't have, or an older one
	ShopLook    []string `json:"shopLook"`    // Game look files the setup doesn't have, or older ones
	Others      []string `json:"others"`      // inactive setups with parked content
}

func (g GameState) empty() bool {
	return len(g.Sets) == 0 && len(g.Imported) == 0 && !g.Accessories && !g.CardBack && len(g.ShopLook) == 0 && len(g.Others) == 0
}

// source is where one setup's old content is: the game folder for the active setup, the parked game\ folder otherwise.
type source struct {
	sets, library, accessories string // set folders (set.json, older versions also files), set files, accessory library
	cardBack, shopLook         string // "" for parked setups (their game\ folder is already where the mod reads them)
}

func sourceFor(h setups.Home, id string, active bool, gameDir string) (source, bool) {
	if active {
		if !game.IsGameDir(gameDir) {
			return source{}, false
		}
		plugin := game.PluginDir(gameDir)
		return source{sets: game.SetsDir(gameDir), library: project.GameLibraryDir(gameDir), accessories: accessories.InstalledDir(gameDir),
			cardBack: filepath.Join(plugin, globalBack), shopLook: filepath.Join(plugin, modconfig.ShopSignFolder)}, true
	}
	parked := filepath.Join(h.Dir(id), setups.GameStateDir)
	return source{sets: filepath.Join(parked, "Sets"), library: filepath.Join(parked, project.GameLibraryDirName),
		accessories: filepath.Join(parked, accessories.InstallDir)}, true
}

const globalBack = "card_back.png"

// setupState is one setup's part of GameState (nothing once it has adopted).
func setupState(h setups.Home, id string, src source) GameState {
	g := GameState{Sets: []string{}, Imported: []string{}, ShopLook: []string{}, Others: []string{}}
	dir := h.Dir(id)
	in, err := setups.LoadInfo(dir)
	if err != nil || in.AdoptedGame {
		return g
	}
	installed := map[string]bool{}
	for _, s := range in.InstalledSets {
		installed[s] = true
	}
	entries, _ := os.ReadDir(src.sets)
	for _, e := range entries {
		sid := e.Name()
		if !e.IsDir() || !setfmt.SafeID(sid) || !fsutil.Exists(filepath.Join(src.sets, sid, project.SetFile)) {
			continue
		}
		switch {
		case !fsutil.Exists(filepath.Join(dir, "projects", sid, project.SetFile)):
			g.Imported = append(g.Imported, sid)
		case !installed[sid]:
			g.Sets = append(g.Sets, sid)
		}
	}
	sort.Strings(g.Sets)
	sort.Strings(g.Imported)
	g.Accessories = fsutil.Exists(filepath.Join(src.accessories, accessories.LibraryFile)) && !fsutil.Exists(filepath.Join(dir, "accessories", accessories.LibraryFile))
	if src.cardBack != "" {
		g.CardBack = newer(src.cardBack, filepath.Join(dir, setups.GameStateDir, globalBack))
	}
	if src.shopLook != "" {
		_ = filepath.WalkDir(src.shopLook, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(src.shopLook, p)
			if newer(p, filepath.Join(dir, setups.GameStateDir, modconfig.ShopSignFolder, rel)) {
				g.ShopLook = append(g.ShopLook, filepath.ToSlash(rel))
			}
			return nil
		})
	}
	return g
}

// newer: src exists and dst is missing or older (the game folder had the newest card back / Game look: older Studio versions
// wrote it there and only copied it into the setup on a switch).
func newer(src, dst string) bool {
	si, err := os.Stat(src)
	if err != nil || si.IsDir() {
		return false
	}
	di, err := os.Stat(dst)
	return err != nil || (si.ModTime().After(di.ModTime()) && !fsutil.SameContent(src, dst))
}

// gameState lists what is still to take over: the active setup's in detail, inactive setups by name.
func gameState(h setups.Home, gameDir string) GameState {
	g := GameState{Sets: []string{}, Imported: []string{}, ShopLook: []string{}, Others: []string{}}
	active, err := h.Active()
	if err != nil {
		return g
	}
	list, _ := h.List()
	for _, s := range list {
		src, ok := sourceFor(h, s.ID, s.ID == active, gameDir)
		if !ok {
			continue
		}
		st := setupState(h, s.ID, src)
		if s.ID == active {
			st.Setup, st.Others = s.Name, g.Others
			g = st
		} else if !st.empty() {
			g.Others = append(g.Others, s.Name)
		}
	}
	return g
}

// GamePending reports whether a setup still has content only in the game folder or its parked game\ folder (the Storage
// page then keeps the game's copies until it has been taken over).
func GamePending(h setups.Home, gameDir string) bool { return !gameState(h, gameDir).empty() }

// AdoptGame takes every setup's old content over (see GameState) and marks the setup as done, so content removed in Studio
// later never comes back from an old copy. The active setup waits until the game folder is known.
func AdoptGame(h setups.Home, gameDir string, res *Result) error {
	active, err := h.Active()
	if err != nil {
		return err
	}
	list, err := h.List()
	if err != nil {
		return err
	}
	for _, s := range list {
		src, ok := sourceFor(h, s.ID, s.ID == active, gameDir)
		if !ok {
			continue
		}
		if err := adoptSetup(h, s.ID, s.Name, src, res); err != nil {
			return err
		}
	}
	return nil
}

func adoptSetup(h setups.Home, id, name string, src source, res *Result) error {
	dir := h.Dir(id)
	in, err := setups.LoadInfo(dir)
	if err != nil || in.AdoptedGame {
		return err
	}
	g := setupState(h, id, src)
	ws := workspace.Dir(h.Root)
	for _, sid := range g.Imported {
		if err := importSet(ws, dir, src, sid); err != nil {
			return err
		}
		res.Notes = append(res.Notes, name+": set "+sid+" from the game folder is now a project of the setup")
	}
	if g.Accessories {
		n, err := adoptAccessories(ws, src.accessories, filepath.Join(dir, "accessories"))
		if err != nil {
			return err
		}
		res.Files += n
		res.Notes = append(res.Notes, name+": accessory library taken over from the game folder")
	}
	if g.CardBack {
		from, to := src.cardBack, filepath.Join(dir, setups.GameStateDir, globalBack)
		if err := res.keepCopy(to, from); err != nil { // the setup's older card back
			return err
		}
		if err := fsutil.CopyFile(from, to); err != nil {
			return err
		}
		res.Files++
	}
	for _, f := range g.ShopLook {
		from, to := filepath.Join(src.shopLook, filepath.FromSlash(f)), filepath.Join(dir, setups.GameStateDir, modconfig.ShopSignFolder, filepath.FromSlash(f))
		if err := res.keepCopy(to, from); err != nil {
			return err
		}
		if err := fsutil.CopyFile(from, to); err != nil {
			return err
		}
		res.Files++
	}
	return setups.EditInfo(dir, func(in *setups.Info) {
		seen := map[string]bool{}
		for _, s := range in.InstalledSets {
			seen[s] = true
		}
		for _, s := range append(append([]string{}, g.Sets...), g.Imported...) {
			if !seen[s] {
				seen[s] = true
				in.InstalledSets = append(in.InstalledSets, s)
			}
		}
		sort.Strings(in.InstalledSets)
		in.AdoptedGame = true
	})
}

// importSet makes a set that was only in the game (hand-installed, or its project deleted) a project of the setup: its
// set.json into projects\<id>, its files into the shared library where the library doesn't have them yet.
func importSet(ws workspace.Dir, setupDir string, src source, id string) error {
	if err := fsutil.CopyFile(filepath.Join(src.sets, id, project.SetFile), filepath.Join(setupDir, "projects", id, project.SetFile)); err != nil {
		return err
	}
	lib := filepath.Join(ws.Library(), id)
	keep := func(rel string, dir bool) bool {
		return !dir && (rel == project.SetFile || rel == project.MetaFile || fsutil.Exists(filepath.Join(lib, filepath.FromSlash(rel))))
	}
	for _, from := range []string{filepath.Join(src.sets, id), filepath.Join(src.library, id)} {
		if fsutil.Exists(from) {
			if err := fsutil.CopyTree(from, lib, keep); err != nil {
				return err
			}
		}
	}
	return nil
}

// adoptAccessories copies an accessory library into a setup that has none: store files ("assets/…") into the workspace
// store, everything else next to accessories.json (the upgrade's accessory step then moves images into the store).
func adoptAccessories(ws workspace.Dir, from, to string) (int, error) {
	store := ws.Store()
	n := 0
	err := filepath.WalkDir(from, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		rel = filepath.ToSlash(rel)
		dst := filepath.Join(to, filepath.FromSlash(rel))
		if strings.HasPrefix(rel, "assets/") && assets.IsAsset(rel) {
			dst = store.Path(rel)
		}
		if fsutil.Exists(dst) { // never over a file the setup or store already has
			return nil
		}
		n++
		return fsutil.CopyFile(p, dst)
	})
	return n, err
}

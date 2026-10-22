package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/art"
	"tcgstudio/internal/catalog"
	"tcgstudio/internal/epl"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/game"
	"tcgstudio/internal/gamify"
	"tcgstudio/internal/ids"
	"tcgstudio/internal/importer"
	"tcgstudio/internal/installer"
	"tcgstudio/internal/project"
	"tcgstudio/internal/scryfall"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/setups"
	"tcgstudio/internal/tcgdex"
	"tcgstudio/internal/updater"
	"tcgstudio/internal/uvmap"
	"tcgstudio/internal/workspace"
)

// App is the Wails-bound backend. Every exported method is callable from the frontend.
type App struct {
	ctx        context.Context
	sf         *scryfall.Client
	sources    *importer.Registry
	started    chan struct{} // closed when startup has picked the workspace/setup (see WaitReady)
	task       taskSlot      // the running import or Forge install
	syncMu     sync.Mutex    // SyncInstalledSets
	catalogMu  sync.Mutex    // the shared catalog (app_catalog.go)
	paintMu    sync.Mutex    // one furniture paint template made at a time (app_furniture.go)
	settingsMu sync.RWMutex  // settings (app_settings.go)
	settings   Settings
	updatedTo  string           // set when started by an update (--updated=<version>)
	release    *updater.Release // newest release found by CheckForUpdate
	tpl        templatesState   // game templates (app_templates.go)
	setups     setupsState      // active setup (app_setups.go)
	storage    storageState     // the running storage task (app_storage.go)
}

// taskSlot is the one import or download that may run at a time; stop cancels it.
type taskSlot struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

// start claims the slot (busy = the error when a task already runs); the returned func frees it.
func (t *taskSlot) start(parent context.Context, busy string) (context.Context, func(), error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cancel != nil {
		return nil, nil, errors.New(busy)
	}
	ctx, cancel := context.WithCancel(parent)
	t.cancel = cancel
	return ctx, func() {
		t.mu.Lock()
		t.cancel = nil
		t.mu.Unlock()
		cancel()
	}, nil
}

func (t *taskSlot) stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cancel != nil {
		t.cancel()
	}
}

func NewApp() *App {
	sf := scryfall.New()
	return &App{sf: sf, sources: importer.NewRegistry(sf, tcgdex.New()), started: make(chan struct{})}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	_, _ = a.updateSettings(func(s *Settings) {
		*s = loadSettings()
		if s.Workspace == "" {
			s.Workspace = project.DefaultRoot()
		}
		if s.GameDir == "" || !game.IsGameDir(s.GameDir) {
			s.GameDir = game.Locate()
		}
	})
	a.initSetups()
	epl.ClearCache(a.home().CacheDir()) // unpacked EPL downloads from earlier sessions
	a.home().RemoveSaveBackups()        // copies of the saves older versions made before the first setup switch
	close(a.started)
	a.RefreshTemplates(false)
}

// WaitReady returns once startup has loaded the settings and picked the active setup. Wails runs startup in a goroutine
// while the page loads, so the frontend awaits this before its first calls (an early ListProjects saw no workspace).
func (a *App) WaitReady() bool {
	<-a.started
	return true
}

// wsDir is the Studio workspace (shared library, store, catalog, setups).
func (a *App) wsDir() workspace.Dir { return workspace.Dir(a.workspaceDir()) }

// ws is the active setup's set projects; see app_setups.go.
func (a *App) ws() project.Workspace { return a.wsDir().Projects(a.root()) }

// items is the active setup's item library (accessories, figurines, furniture, decorations).
func (a *App) items() (*accessories.Library, error) { return a.wsDir().Items(a.root()) }

// ---------------------------------------------------------------- updates (GitHub Releases)

// VersionInfo describes this build and whether it was just updated.
type VersionInfo struct {
	Version    string `json:"version"`
	CanUpdate  bool   `json:"canUpdate"`  // release build with an update repo
	UpdatedTo  string `json:"updatedTo"`  // non-empty right after an update restart
	ReleaseURL string `json:"releaseUrl"` // releases page
}

func (a *App) AppVersion() VersionInfo {
	v := VersionInfo{Version: Version, CanUpdate: Version != "dev" && UpdateRepo != "", UpdatedTo: a.updatedTo}
	if UpdateRepo != "" {
		v.ReleaseURL = "https://github.com/" + UpdateRepo + "/releases"
	}
	return v
}

// CheckForUpdate returns the newest release if it is newer than this build (nil if up to date / no releases / dev build).
func (a *App) CheckForUpdate() (*updater.Release, error) {
	if !a.AppVersion().CanUpdate {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()
	rel, err := updater.Latest(ctx, UpdateRepo)
	if err != nil || rel == nil || !updater.Newer(rel.Version, Version) {
		return nil, err
	}
	a.release = rel
	return rel, nil
}

// InstallUpdate downloads the release found by CheckForUpdate ("update:progress" events: fraction 0..1), swaps it in and
// restarts the app. The mod in the game is updated afterwards from the Game & mod page (it shows as out of date).
func (a *App) InstallUpdate() error {
	if a.release == nil {
		return errors.New("no update to install — check for updates first")
	}
	if game.IsRunning() {
		return errors.New("please close the game first: TCG Studio updates your content and the mod for the game right after updating")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	file, err := updater.Download(a.ctx, a.release, func(done, total int64) {
		if total > 0 {
			runtime.EventsEmit(a.ctx, evUpdateProgress, float64(done)/float64(total))
		}
	})
	if err != nil {
		return err
	}
	if err := updater.Swap(exe, file); err != nil {
		return err
	}
	if err := updater.Restart(exe, a.release.Version); err != nil {
		return fmt.Errorf("updated, but couldn't restart — please start TCG Studio again: %w", err)
	}
	runtime.Quit(a.ctx)
	return nil
}

// ---------------------------------------------------------------- setup (BepInEx + mod installer)

// SetupState inspects the game folder: loader, F1 menu, mod version, conflicts, templates, sets.
func (a *App) SetupState() installer.State {
	p, err := installer.Embedded()
	return installer.Inspect(a.gameDir(), p, err == nil, game.IsRunning())
}

// SetupRepair installs/repairs everything that isn't OK (progress via "setup:progress" events). Returns the backup folder, if any.
func (a *App) SetupRepair() (string, error) {
	p, err := installer.Embedded()
	if err != nil {
		return "", err
	}
	backup, err := a.runSetup("repair", func(log func(string)) (string, error) {
		return installer.Repair(a.gameDir(), p, game.IsRunning(), log)
	})
	a.pointGameAtWorkspace() // a reinstalled plugin folder has no studio-paths.json yet
	return backup, err
}

// SetupUninstall removes the mod (all=false) or the mod and BepInEx (all=true), moving files to a backup folder.
func (a *App) SetupUninstall(all bool) (string, error) {
	return a.runSetup("uninstall", func(log func(string)) (string, error) {
		return installer.Uninstall(a.gameDir(), all, game.IsRunning(), log)
	})
}

func (a *App) runSetup(what string, run func(log func(string)) (string, error)) (string, error) {
	var lines []string
	log := func(s string) {
		lines = append(lines, s)
		runtime.EventsEmit(a.ctx, evSetupProgress, s)
	}
	backup, err := run(log)
	if err != nil {
		lines = append(lines, "ERROR: "+err.Error())
	}
	entry := fmt.Sprintf("== %s %s (game: %s)\r\n%s\r\n", time.Now().Format(time.RFC3339), what, a.gameDir(), strings.Join(lines, "\r\n"))
	_ = os.MkdirAll(a.workspaceDir(), 0o755)
	if f, ferr := os.OpenFile(filepath.Join(a.workspaceDir(), "install.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); ferr == nil {
		_, _ = f.WriteString(entry)
		f.Close()
	}
	return backup, err
}

// LaunchGame starts the game through Steam.
func (a *App) LaunchGame() {
	runtime.BrowserOpenURL(a.ctx, "steam://rungameid/"+game.SteamAppID(a.gameDir()))
}

// OpenFolder opens a folder in Explorer (e.g. the setup backup folder).
func (a *App) OpenFolder(path string) {
	reveal(path)
}

// ModCheck is what the BepInEx log says about the last game launch.
type ModCheck struct {
	LogFound bool     `json:"logFound"`
	When     string   `json:"when"`
	Loaded   bool     `json:"loaded"`
	Summary  string   `json:"summary"`
	Errors   []string `json:"errors"`
}

// CheckModLoaded reads BepInEx/LogOutput.log from the last launch.
func (a *App) CheckModLoaded() ModCheck {
	var c ModCheck
	path := filepath.Join(a.gameDir(), "BepInEx", "LogOutput.log")
	info, err := os.Stat(path)
	if err != nil {
		return c
	}
	c.LogFound = true
	c.When = info.ModTime().Format("2006-01-02 15:04")
	b, _ := os.ReadFile(path)
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "TCG Custom Cards") && strings.Contains(line, "loaded with") {
			c.Loaded = true
			if i := strings.Index(line, "] "); i >= 0 {
				c.Summary = line[i+2:]
			}
		}
		if (strings.HasPrefix(line, "[Error") || strings.HasPrefix(line, "[Fatal")) && len(c.Errors) < 8 {
			c.Errors = append(c.Errors, line)
		}
	}
	return c
}

// ---------------------------------------------------------------- import sets

// ImportableSet is a set offered by an import source, marked when it is already a project.
type ImportableSet struct {
	importer.SetInfo
	Imported  bool   `json:"imported"`
	ProjectID string `json:"projectId"`
}

// ImportSources lists the card databases sets can be imported from.
func (a *App) ImportSources() []importer.SourceInfo {
	var out []importer.SourceInfo
	for _, s := range a.sources.All() {
		out = append(out, s.Info())
	}
	return out
}

// SourceSets lists a source's sets (cached for a day in the workspace) in the source's order (newest first).
func (a *App) SourceSets(source, lang string, refresh bool) ([]ImportableSet, error) {
	src, err := a.sources.Get(source)
	if err != nil {
		return nil, err
	}
	name := "import_" + source
	if lang != "" {
		name += "_" + lang
	}
	cache := filepath.Join(a.home().CacheDir(), name+".json")
	var sets []importer.SetInfo
	if info, err := os.Stat(cache); err == nil && !refresh && time.Since(info.ModTime()) < 24*time.Hour {
		if b, err := os.ReadFile(cache); err == nil {
			_ = json.Unmarshal(b, &sets)
		}
	}
	if len(sets) == 0 {
		if sets, err = src.Sets(a.ctx, lang); err != nil {
			return nil, err
		}
		_ = os.MkdirAll(filepath.Dir(cache), 0o755)
		if b, err := json.Marshal(sets); err == nil {
			_ = os.WriteFile(cache, b, 0o644)
		}
	}
	ws := a.ws()
	out := make([]ImportableSet, 0, len(sets))
	for _, s := range sets {
		id := src.ProjectID(s.Code, lang)
		_, err := os.Stat(ws.Folder(id))
		out = append(out, ImportableSet{SetInfo: s, Imported: err == nil, ProjectID: id})
	}
	return out, nil
}

// PickImportFolder asks for a folder of card images (the "Image folder" source); "" when cancelled.
func (a *App) PickImportFolder() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Select a folder of card images"})
}

// PreviewImportFolder reads a folder of card images: what an image-folder import would make of it.
func (a *App) PreviewImportFolder(dir string, opt importer.Options) (importer.FolderPreview, error) {
	if opt.RarityMap == nil {
		opt.RarityMap = importer.DefaultFolderRarityMap()
	}
	p, err := importer.ScanFolder(dir, opt)
	if err != nil {
		return p, err
	}
	_, err = os.Stat(a.ws().Folder(p.ProjectID))
	p.Imported = err == nil
	return p, nil
}

// PackPresets lists the real boosters of every game we import (Packs tab "Apply preset…"; importer/boosters.go).
func (a *App) PackPresets() []importer.Booster { return importer.PackPresets() }

func (a *App) DefaultImportOptions(source string) (importer.Options, error) {
	src, err := a.sources.Get(source)
	if err != nil {
		return importer.Options{}, err
	}
	opt := src.DefaultOptions()
	opt.ImageFormat = "png"
	if a.cfg().ImageFormat == "jpg" {
		opt.ImageFormat = "jpg"
	}
	return opt, nil
}

// ImportSet runs an import from a source, emitting "import:progress" events. Returns the new project id. A set the
// catalog (or another setup) already has is added from there instead (Import page), never imported again: an import
// would generate pack art over the shared art other setups use.
func (a *App) ImportSet(source, code string, opt importer.Options) (string, error) {
	src, err := a.sources.Get(source)
	if err != nil {
		return "", err
	}
	if id := src.ProjectID(code, opt.Lang); a.inCatalog(id) {
		return "", fmt.Errorf("%s is already in your catalog (imported in another setup or earlier) — use Add from catalog", id)
	}
	ctx, done, err := a.task.start(a.ctx, "an import is already running")
	if err != nil {
		return "", err
	}
	defer done()
	ws := a.ws()
	p, err := src.Import(ctx, ws, code, opt, func(pr importer.Progress) {
		runtime.EventsEmit(a.ctx, evImportProgress, pr)
	})
	if err != nil {
		return "", err
	}
	changed := a.fillCollection(p)
	if opt.KeepRarities && !p.Set.OwnRarities() && importer.KeepSourceRarities(p, src.Info().RarityOrder) == nil {
		changed = true
	}
	// Brand the default booster with generated art when the game's templates are available (not when the import brought
	// its own pack art, e.g. an EPL mod).
	if dir, err := a.requireTemplates(); err == nil && len(p.Set.Packs) > 0 && p.Set.Packs[0].PackTexture == "" {
		if res, err := art.Generate(dir, p.LibFolder, p.ID, p.Set.Name, art.Options{NoPackText: true, Resolve: p.ImagePath, Target: p.WriteTarget}); err == nil {
			applyArt(&p.Set.Packs[0], res)
			changed = true
		}
	}
	if changed {
		_ = ws.Save(p)
	}
	a.catalogSet(p.ID) // available to every setup from now on
	return p.ID, nil
}

// ---------------------------------------------------------------- pack art

func (a *App) DefaultArtColor(id string) string { return art.DefaultColor(id) }

// GeneratePackArt renders pack/box textures + icons for one pack from the game's templates and returns the file paths.
// The caller assigns them to the pack (new file names each time, freshName: the game sees them once the set is saved).
func (a *App) GeneratePackArt(id string, packID string, o art.Options) (art.Result, error) {
	dir, err := a.requireTemplates()
	if err != nil {
		return art.Result{}, err
	}
	p, err := a.loadProject(id)
	if err != nil {
		return art.Result{}, err
	}
	if o.FilePrefix, err = packFilePrefix(packID); err != nil {
		return art.Result{}, err
	}
	o.Resolve = p.ImagePath // the set icon / front image may live in the shared library
	o.Target = func(rel string) (string, string) { return p.WriteTarget(freshName(rel)) }
	return art.Generate(dir, p.LibFolder, p.ID, p.Set.Name, o)
}

// ---------------------------------------------------------------- card backs

const globalBackFile = "card_back.png" // the setup's global card back (<setup>\game\card_back.png, read by the mod)

// GenerateSetCardBack makes a card back from a colour, the set icon and a title.
func (a *App) GenerateSetCardBack(id, color, title string) (string, error) {
	p, err := a.loadProject(id)
	if err != nil {
		return "", err
	}
	rel, out := p.WriteTarget(freshName("images/card_back.png"))
	return rel, art.GenerateCardBackFrom(p.ImagePath, "", color, title, out)
}

func (a *App) globalBackPath() string {
	return filepath.Join(a.root(), setups.GameStateDir, globalBackFile)
}

// GlobalCardBack returns a URL for the current global card back, or "" when none is set.
func (a *App) GlobalCardBack() string {
	info, err := os.Stat(a.globalBackPath())
	if err != nil {
		return ""
	}
	return "/globalback/" + globalBackFile + "?v=" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
}

// BoxArt is a generated box for a pack: its texture and icon, and the 3D editor layout starting from that texture.
type BoxArt struct {
	BoxTexture string `json:"boxTexture"`
	BoxIcon    string `json:"boxIcon"`
	Layout     string `json:"layout"` // JSON for project.meta.packArt[pack].box
}

// boxArtFromPack renders a box from the game template with the pack texture's front panel on it (files written into the
// set's shared folder; the pack's own files are untouched).
func (a *App) boxArtFromPack(p *project.Project, pk *setfmt.Pack) (BoxArt, error) {
	dir, err := a.requireTemplates()
	if err != nil {
		return BoxArt{}, err
	}
	if pk.PackTexture == "" {
		return BoxArt{}, errors.New("this pack has no pack texture to take the front from")
	}
	front, frontFile := p.WriteTarget("images/" + pk.ID + "_box_front.png")
	if err := art.CropPackFront(p.ImagePath(pk.PackTexture), frontFile); err != nil {
		return BoxArt{}, err
	}
	r, err := art.Generate(dir, p.LibFolder, p.ID, p.Set.Name,
		art.Options{FilePrefix: pk.ID + "_box_", FrontImage: front, Resolve: p.ImagePath, Target: p.WriteTarget})
	if err != nil {
		return BoxArt{}, err
	}
	for _, f := range []string{r.PackTexture, r.PackIcon} { // only the box is wanted
		if f != "" && f != pk.PackTexture && f != pk.PackIcon {
			_ = os.Remove(p.ImagePath(f))
		}
	}
	layout, _ := json.Marshal(map[string]any{"version": 2, "net": uvmap.NetVersion, "layers": []any{}, "base": "vanilla",
		"baseColor": "#3a3a3a", "baseItem": "file", "baseFile": r.BoxTexture})
	return BoxArt{BoxTexture: r.BoxTexture, BoxIcon: r.BoxIcon, Layout: string(layout)}, nil
}

// BoxFromPack makes box art for a pack from its own pack art (packs that came without a box). The caller puts the result
// into the pack and project.meta.packArt and saves.
func (a *App) BoxFromPack(id, packID string) (BoxArt, error) {
	p, err := a.loadProject(id)
	if err != nil {
		return BoxArt{}, err
	}
	for i := range p.Set.Packs {
		if p.Set.Packs[i].ID == packID {
			return a.boxArtFromPack(p, &p.Set.Packs[i])
		}
	}
	return BoxArt{}, errors.New("pack " + packID + " not found (save the set first)")
}

func applyArt(pk *setfmt.Pack, r art.Result) {
	pk.PackTexture, pk.PackIcon, pk.BoxTexture, pk.BoxIcon = r.PackTexture, r.PackIcon, r.BoxTexture, r.BoxIcon
}

func (a *App) CancelImport() { a.task.stop() }

// ---------------------------------------------------------------- projects

func (a *App) ListProjects() ([]project.Summary, error) {
	<-a.started
	return a.ws().List(a.gameDir())
}

func (a *App) LoadProject(id string) (*project.Project, error) { return a.loadProject(id) }

// UseSourceRarities returns the editor's project with the source's own rarities as its rarity list (Set tab button; the
// editor saves it). See importer.KeepSourceRarities.
func (a *App) UseSourceRarities(p project.Project) (*project.Project, error) {
	var order []string
	if src, err := a.sources.Get(p.Meta.Source); err == nil {
		order = src.Info().RarityOrder
	}
	if err := importer.KeepSourceRarities(&p, order); err != nil {
		return nil, err
	}
	return &p, nil
}

// CreateProject makes an empty set with a random id (internal/ids), free in this setup and the shared library.
func (a *App) CreateProject(name string) (*project.Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "My Set"
	}
	ws := a.ws()
	id := ids.Free(func(id string) bool { return fsutil.Exists(ws.Folder(id)) || fsutil.Exists(ws.LibFolder(id)) })
	return ws.Create(id, name)
}

// DeleteProject removes the project (taken out of the setup's installed sets first, so the game doesn't look for it).
// Player saves keep their data for the set; it comes back if a set with the same id is installed again.
func (a *App) DeleteProject(id string) error {
	if err := a.keepInCatalog(catalog.KindSet, []string{id}, nil); err != nil {
		return err
	}
	if err := a.ws().Uninstall(id); err != nil {
		return err
	}
	return a.ws().Delete(id)
}

// SaveResult reports a save or install: validation issues and whether the game loads the set.
type SaveResult struct {
	Issues       []setfmt.Issue `json:"issues"`
	Synced       bool           `json:"synced"`       // installed: the game loads the saved set at its next start
	InstallState string         `json:"installState"` // project.StateNone / StateCurrent / StateStale after the call
	GameRunning  bool           `json:"gameRunning"`  // the game must be restarted to load the change
	SyncError    string         `json:"syncError"`    // why an installed set couldn't be updated (e.g. validation errors)
	// Pictures: the file each picture slot was cut into (bakeSetPictures; "" = slot dropped), which set.json now names; the
	// page's copy follows.
	Pictures map[string]string `json:"pictures"`
}

// SaveProject writes the project (even with issues); an installed set is what the game loads at its next start
// (unless validation found errors) — so "Save" is all it takes for edits to reach the game.
func (a *App) SaveProject(p project.Project) (SaveResult, error) {
	if !setfmt.SafeID(p.ID) {
		return SaveResult{}, errBadID
	}
	ws, gameDir := a.ws(), a.gameDir()
	p.Folder = ws.Folder(p.ID)
	p.Set.ID = p.ID
	baked, err := bakeSetPictures(&p)
	if err != nil {
		return SaveResult{}, err
	}
	if err := ws.Save(&p); err != nil {
		return SaveResult{}, err
	}
	r := SaveResult{Issues: issues(p.Set.Validate(p.LibFolder)), Pictures: baked}
	if !game.IsGameDir(gameDir) {
		r.InstallState = project.StateNone
		return r, nil
	}
	r.InstallState = ws.InstallState(&p, gameDir)
	// The game reads the saved set.json itself: an installed set with errors is skipped by the mod until they're fixed.
	switch {
	case r.InstallState == project.StateNone:
	case hasErrors(r.Issues):
		r.SyncError = "the game skips this set until the errors are fixed"
	case r.InstallState == project.StateStale:
		if err := ws.Install(&p, gameDir); err != nil {
			r.SyncError = err.Error()
		} else {
			r.Synced = true
			r.InstallState = project.StateCurrent
		}
	default:
		r.Synced = true // saved where the game reads it
	}
	r.GameRunning = r.Synced && game.IsRunning()
	return r, nil
}

func hasErrors(iss []setfmt.Issue) bool {
	for _, i := range iss {
		if i.Level == "error" {
			return true
		}
	}
	return false
}

// ProjectInstallState tells whether the game has the current version of a set (project.StateNone / StateCurrent / StateStale).
func (a *App) ProjectInstallState(id string) (string, error) {
	p, err := a.loadProject(id)
	if err != nil {
		return "", err
	}
	return a.ws().InstallState(p, a.gameDir()), nil
}

func (a *App) ValidateProject(id string) ([]setfmt.Issue, error) {
	p, err := a.loadProject(id)
	if err != nil {
		return nil, err
	}
	return issues(p.Set.Validate(p.LibFolder)), nil
}

// InstallProject validates the set and adds it to the active setup's installed sets (the game reads it from the project).
// Refuses when there are errors.
func (a *App) InstallProject(id string) (SaveResult, error) {
	p, err := a.loadProject(id)
	if err != nil {
		return SaveResult{}, err
	}
	if a.fillCollection(p) { // sets made before converted content got its mod as binder group
		_ = a.ws().Save(p)
	}
	r := SaveResult{Issues: issues(p.Set.Validate(p.LibFolder))}
	if hasErrors(r.Issues) {
		r.InstallState = a.ws().InstallState(p, a.gameDir())
		return r, errors.New("fix the errors before installing")
	}
	if err := a.ws().Install(p, a.gameDir()); err != nil {
		return r, err
	}
	r.Synced, r.InstallState, r.GameRunning = true, project.StateCurrent, game.IsRunning()
	return r, nil
}

func (a *App) UninstallProject(id string) error { return a.ws().Uninstall(id) }

func (a *App) OpenProjectFolder(id string) {
	if setfmt.SafeID(id) {
		reveal(a.ws().Folder(id))
	}
}

// ---------------------------------------------------------------- gamify & prices

func (a *App) GamifyDefaults() gamify.Settings { return gamify.DefaultSettings() }

// GamifyPreview shows what Gamify would do to the given projects (in tier order) without saving.
func (a *App) GamifyPreview(order []string, s gamify.Settings) ([]gamify.Preview, error) {
	return a.gamify(order, s, false)
}

// GamifyApply re-prices and saves the given projects (in tier order); installed sets change in the game at its next start.
func (a *App) GamifyApply(order []string, s gamify.Settings) ([]gamify.Preview, error) {
	return a.gamify(order, s, true)
}

func (a *App) gamify(order []string, s gamify.Settings, save bool) ([]gamify.Preview, error) {
	ws := a.ws()
	installed := ws.Installed()
	out := []gamify.Preview{}
	for i, id := range order {
		p, err := a.loadProject(id)
		if err != nil {
			return nil, err
		}
		set := s
		if len(order) <= 9 {
			set.MaxLevel = 0 // one set per vanilla product: keep vanilla unlock levels
		}
		pv := gamify.Apply(p, i+1, gamify.Position(i, len(order)), set)
		pv.Installed = installed[id]
		if save {
			if err := ws.Save(p); err != nil {
				return nil, err
			}
		}
		out = append(out, pv)
	}
	return out, nil
}

func (a *App) isInstalled(id string) bool { return setfmt.SafeID(id) && a.ws().Installed()[id] }

// RefreshPrices pulls current prices from the set's import source and re-applies the project's last pricing settings (licenses untouched).
func (a *App) RefreshPrices(id string) (string, error) {
	p, err := a.loadProject(id)
	if err != nil {
		return "", err
	}
	src, err := a.sources.Get(p.Meta.Source)
	if err != nil {
		return "", fmt.Errorf("%s was not imported from a card database", id)
	}
	n, err := src.RefreshMeta(a.ctx, p)
	if err != nil {
		return "", err
	}
	msg := "Updated real prices for " + strconv.Itoa(n) + " cards"
	if p.Meta.Pricing != nil {
		s := gamify.DefaultSettings()
		s.Mode, s.BorderCurve, s.TierStep, s.Progression = p.Meta.Pricing.Mode, p.Meta.Pricing.BorderCurve, p.Meta.Pricing.TierStep, false
		tier := p.Meta.Tier
		if tier < 1 {
			tier = 1
		}
		pos := p.Meta.Pricing.Position
		if pos == 0 && tier > 1 {
			pos = float64(tier - 1) // sets gamified before positions were stored
		}
		gamify.Apply(p, tier, pos, s)
		msg += " and re-priced them (" + s.Mode + ")"
	} else {
		for i := range p.Set.Cards {
			c := &p.Set.Cards[i]
			if m := p.Meta.Cards[c.ID]; !m.Locked && (m.ScryfallID != "" || m.SourceID != "") {
				c.Price = importer.RealPrice(m)
			}
		}
		msg += " and set real prices"
	}
	inGame, err := a.saveSet(p)
	if inGame && err == nil {
		msg += "; the game shows them after a restart"
	}
	return msg, err
}

// FetchBackFaces downloads the missing back-face pictures of a Scryfall set's double-faced cards (sets imported before
// back faces existed), so the mod can show transformed cards; then saves and reinstalls the set like RefreshPrices.
func (a *App) FetchBackFaces(id string) (string, error) {
	p, err := a.loadProject(id)
	if err != nil {
		return "", err
	}
	width := 512 // the importer's default card width; the shared library records the width it was made with
	if lm := project.LoadLibMeta(p.LibFolder); lm != nil && lm.Width > 0 {
		width = lm.Width
	}
	n, failed, err := importer.FetchBackFaces(a.ctx, a.sf, p, width, func(pr importer.Progress) {
		runtime.EventsEmit(a.ctx, evImportProgress, pr)
	})
	if err != nil {
		return "", err
	}
	if n == 0 && len(failed) == 0 {
		return "No double-faced cards without a back face in this set", nil
	}
	msg := fmt.Sprintf("Added %d back face(s)", n)
	if len(failed) > 0 {
		msg += fmt.Sprintf(" (%d failed: %s)", len(failed), strings.Join(failed, ", "))
	}
	inGame, err := a.saveSet(p)
	if inGame && err == nil {
		msg += "; the game shows them after a restart"
	}
	return msg, err
}

// PickImage lets the user choose one image and copies it into the project's images folder. Returns the relative path ("" if cancelled).
func (a *App) PickImage(id, title string) (string, error) {
	file, err := a.pickImageFile(title)
	if err != nil || file == "" {
		return "", err
	}
	return a.copyIntoProject(id, file)
}

// PickImages lets the user choose several images (e.g. one per new card) and copies them into the project.
func (a *App) PickImages(id string) ([]string, error) {
	files, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose card images", Filters: imageFilters})
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, f := range files {
		rel, err := a.copyIntoProject(id, f)
		if err != nil {
			return out, err
		}
		out = append(out, rel)
	}
	return out, nil
}

func (a *App) copyIntoProject(id, file string) (string, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	if b, err = uprightJPEG(b); err != nil {
		return "", fmt.Errorf("%s: %w", filepath.Base(file), err)
	}
	return a.writeIntoProject(id, filepath.Base(file), b)
}

// uprightJPEG turns a JPEG whose EXIF orientation tag says "show rotated" (Windows Photos' Rotate button, phone photos)
// into one whose pixels are upright: the game ignores the tag while Studio's preview honours it. Other files are unchanged.
func uprightJPEG(b []byte) ([]byte, error) {
	o := art.ExifOrientation(b)
	if o == 1 {
		return b, nil
	}
	img, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, art.ApplyOrientation(img, o), &jpeg.Options{Quality: art.JPEGQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RotateCardImages turns card images of project id by turns quarter turns clockwise (negative = anticlockwise); with
// onlyLandscape, only images wider than tall are turned. Each turned image is written under a new name (freshName) and
// returned as old path → new path: the caller points the cards at it, and the game shows it once the set is saved.
func (a *App) RotateCardImages(id string, rels []string, turns int, onlyLandscape bool) (map[string]string, error) {
	if !setfmt.SafeID(id) {
		return nil, errBadID
	}
	p, err := a.loadProject(id)
	if err != nil {
		return nil, err
	}
	done := map[string]bool{}
	n := map[string]string{}
	for _, rel := range rels {
		if rel == "" || done[rel] {
			continue
		}
		done[rel] = true
		if !project.IsRelFile(rel) {
			return n, fmt.Errorf("%s: not a set file", rel)
		}
		dst := a.ws().ImagePath(id, rel)
		b, err := os.ReadFile(dst)
		if err != nil {
			return n, err
		}
		img, format, err := image.Decode(bytes.NewReader(b))
		if err != nil {
			return n, fmt.Errorf("%s: %w", rel, err)
		}
		img = art.ApplyOrientation(img, art.ExifOrientation(b))
		if onlyLandscape && img.Bounds().Dx() <= img.Bounds().Dy() {
			continue
		}
		img = art.RotateQuarter(img, turns)
		var out []byte
		if format == "jpeg" {
			out, err = art.EncodeCardJPEG(img)
		} else {
			var buf bytes.Buffer
			err = png.Encode(&buf, img)
			out = buf.Bytes()
		}
		if err != nil {
			return n, err
		}
		to, err := p.WriteFile(freshName(rel), out)
		if err != nil {
			return n, err
		}
		n[rel] = to
	}
	return n, nil
}

// RedownloadCardImages downloads the art of project id's cards again from the source the set was imported from (Re-download
// image), each under a new name (freshName), and returns card id → new paths: the caller points the cards at them, and the
// game shows them once the set is saved.
func (a *App) RedownloadCardImages(id string, cardIDs []string) (map[string]importer.CardImages, error) {
	if !setfmt.SafeID(id) {
		return nil, errBadID
	}
	p, err := a.loadProject(id)
	if err != nil {
		return nil, err
	}
	got, missing, err := a.sources.RedownloadImages(a.ctx, p, cardIDs, freshName)
	if err != nil {
		return got, err
	}
	if len(got) == 0 && len(missing) > 0 {
		return nil, fmt.Errorf("the source doesn't have this card any more (or it hasn't been saved yet) — use Change image…")
	}
	return got, nil
}

// writeIntoProject stores an image under images/ (in the set's shared folder, Project.WriteTarget) with a free name based
// on name and returns its relative path.
func (a *App) writeIntoProject(id, name string, b []byte) (string, error) {
	p, err := a.loadProject(id)
	if err != nil {
		return "", err
	}
	base := strings.ReplaceAll(filepath.Base(name), " ", "_")
	ext := filepath.Ext(base)
	name = base
	for n := 2; fsutil.Exists(p.ImagePath("images/" + name)); n++ { // other setups may use the shared file of that name
		name = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(base, ext), n, ext)
	}
	return p.WriteFile("images/"+name, b)
}

// SaveProjectImage stores an image made in the studio (data URL, e.g. a straightened photo) in the project; returns its path.
func (a *App) SaveProjectImage(id, name, dataURL string) (string, error) {
	b, err := decodeDataURL(dataURL)
	if err != nil {
		return "", err
	}
	return a.writeIntoProject(id, name, b)
}

func issues(i []setfmt.Issue) []setfmt.Issue {
	if i == nil {
		return []setfmt.Issue{}
	}
	return i
}

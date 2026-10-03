package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/accessories"
	"tcgstudio/internal/art"
	"tcgstudio/internal/game"
	"tcgstudio/internal/gameextract"
	"tcgstudio/internal/gamify"
	"tcgstudio/internal/importer"
	"tcgstudio/internal/installer"
	"tcgstudio/internal/project"
	"tcgstudio/internal/scryfall"
	"tcgstudio/internal/setfmt"
	"tcgstudio/internal/tcgdex"
	"tcgstudio/internal/updater"
)

// App is the Wails-bound backend. Every exported method is callable from the frontend.
type App struct {
	ctx       context.Context
	sf        *scryfall.Client
	sources   *importer.Registry
	started   chan struct{} // closed when startup has picked the workspace/setup (see WaitReady)
	mu        sync.Mutex
	syncMu    sync.Mutex // SyncInstalledSets
	settings  Settings
	cancel    context.CancelFunc
	updatedTo string           // set when started by an update (--updated=<version>)
	release   *updater.Release // newest release found by CheckForUpdate
	tpl       templatesState   // game templates (app_templates.go)
	setups    setupsState      // active setup (app_setups.go)
}

type Settings struct {
	GameDir   string `json:"gameDir"`
	Workspace string `json:"workspace"`
	// SyncedVersion = the Studio version that last brought the installed sets up to date (SyncInstalledSets).
	SyncedVersion string `json:"syncedVersion,omitempty"`
}

func NewApp() *App {
	sf := scryfall.New()
	return &App{sf: sf, sources: importer.NewRegistry(sf, tcgdex.New()), started: make(chan struct{})}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.settings = loadSettings()
	if a.settings.Workspace == "" {
		a.settings.Workspace = project.DefaultRoot()
	}
	if a.settings.GameDir == "" || !game.IsGameDir(a.settings.GameDir) {
		a.settings.GameDir = game.Locate()
	}
	_ = saveSettings(a.settings)
	a.initSetups()
	close(a.started)
	a.RefreshTemplates(false)
}

// WaitReady returns once startup has loaded the settings and picked the active setup. Wails runs startup in a goroutine
// while the page loads, so the frontend awaits this before its first calls (an early ListProjects saw no workspace).
func (a *App) WaitReady() bool {
	<-a.started
	return true
}

// ws is the active setup's studio root (projects + accessory library); see app_setups.go.
func (a *App) ws() project.Workspace { return project.Workspace{Root: a.root()} }

// ---------------------------------------------------------------- settings & game

func settingsPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "TCG Studio", "settings.json")
}

func loadSettings() Settings {
	var s Settings
	if b, err := os.ReadFile(settingsPath()); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

func saveSettings(s Settings) error {
	_ = os.MkdirAll(filepath.Dir(settingsPath()), 0o755)
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(settingsPath(), b, 0o644)
}

func (a *App) GetSettings() Settings { return a.settings }

func (a *App) SaveSettings(s Settings) (Settings, error) {
	if s.GameDir != "" && !game.IsGameDir(s.GameDir) {
		return a.settings, errors.New("that folder is not a TCG Card Shop Simulator install")
	}
	if s.Workspace == "" {
		s.Workspace = project.DefaultRoot()
	}
	s.SyncedVersion = a.settings.SyncedVersion // backend-only
	changed := s.GameDir != a.settings.GameDir
	wsChanged := s.Workspace != a.settings.Workspace
	a.settings = s
	err := saveSettings(s)
	if changed {
		a.invalidateTemplates()
	}
	if wsChanged {
		a.initSetups()
	}
	return s, err
}

func (a *App) GameStatus() game.Status {
	s := game.GetStatus(a.settings.GameDir)
	s.TemplatesFound = a.templatesReady()
	return s
}

func (a *App) LocateGame() game.Status {
	if dir := game.Locate(); dir != "" {
		changed := dir != a.settings.GameDir
		a.settings.GameDir = dir
		_ = saveSettings(a.settings)
		if changed {
			a.invalidateTemplates()
		}
	}
	return a.GameStatus()
}

func (a *App) BrowseGameFolder() (game.Status, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Select the TCG Card Shop Simulator folder"})
	if err != nil || dir == "" {
		return a.GameStatus(), err
	}
	_, err = a.SaveSettings(Settings{GameDir: dir, Workspace: a.settings.Workspace})
	return a.GameStatus(), err
}

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
// restarts the app. The mod in the game is updated afterwards from the Setup screen (it shows as out of date).
func (a *App) InstallUpdate() error {
	if a.release == nil {
		return errors.New("no update to install — check for updates first")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	file, err := updater.Download(a.ctx, a.release, func(done, total int64) {
		if total > 0 {
			runtime.EventsEmit(a.ctx, "update:progress", float64(done)/float64(total))
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
	return installer.Inspect(a.settings.GameDir, p, err == nil, game.IsRunning())
}

// SetupRepair installs/repairs everything that isn't OK (progress via "setup:progress" events). Returns the backup folder, if any.
func (a *App) SetupRepair() (string, error) {
	p, err := installer.Embedded()
	if err != nil {
		return "", err
	}
	return a.runSetup("repair", func(log func(string)) (string, error) {
		return installer.Repair(a.settings.GameDir, p, game.IsRunning(), log)
	})
}

// SetupUninstall removes the mod (all=false) or the mod and BepInEx (all=true), moving files to a backup folder.
func (a *App) SetupUninstall(all bool) (string, error) {
	return a.runSetup("uninstall", func(log func(string)) (string, error) {
		return installer.Uninstall(a.settings.GameDir, all, game.IsRunning(), log)
	})
}

func (a *App) runSetup(what string, run func(log func(string)) (string, error)) (string, error) {
	var lines []string
	log := func(s string) {
		lines = append(lines, s)
		runtime.EventsEmit(a.ctx, "setup:progress", s)
	}
	backup, err := run(log)
	if err != nil {
		lines = append(lines, "ERROR: "+err.Error())
	}
	entry := fmt.Sprintf("== %s %s (game: %s)\r\n%s\r\n", time.Now().Format(time.RFC3339), what, a.settings.GameDir, strings.Join(lines, "\r\n"))
	_ = os.MkdirAll(a.settings.Workspace, 0o755)
	if f, ferr := os.OpenFile(filepath.Join(a.settings.Workspace, "install.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); ferr == nil {
		_, _ = f.WriteString(entry)
		f.Close()
	}
	return backup, err
}

// LaunchGame starts the game through Steam.
func (a *App) LaunchGame() {
	runtime.BrowserOpenURL(a.ctx, "steam://rungameid/"+game.SteamAppID(a.settings.GameDir))
}

// OpenFolder opens a folder in Explorer (e.g. the setup backup folder).
func (a *App) OpenFolder(path string) {
	if path != "" {
		_ = exec.Command("explorer", path).Start()
	}
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
	path := filepath.Join(a.settings.GameDir, "BepInEx", "LogOutput.log")
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
	out := make([]ImportableSet, 0, len(sets))
	for _, s := range sets {
		id := src.ProjectID(s.Code, lang)
		_, err := os.Stat(a.ws().Folder(id))
		out = append(out, ImportableSet{SetInfo: s, Imported: err == nil, ProjectID: id})
	}
	return out, nil
}

func (a *App) DefaultImportOptions(source string) (importer.Options, error) {
	src, err := a.sources.Get(source)
	if err != nil {
		return importer.Options{}, err
	}
	return src.DefaultOptions(), nil
}

// ImportSet runs an import from a source, emitting "import:progress" events. Returns the new project id.
func (a *App) ImportSet(source, code string, opt importer.Options) (string, error) {
	src, err := a.sources.Get(source)
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	if a.cancel != nil {
		a.mu.Unlock()
		return "", errors.New("an import is already running")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.cancel = nil
		a.mu.Unlock()
		cancel()
	}()
	p, err := src.Import(ctx, a.ws(), code, opt, func(pr importer.Progress) {
		runtime.EventsEmit(a.ctx, "import:progress", pr)
	})
	if err != nil {
		return "", err
	}
	// Brand the default booster with generated art when the game's templates are available.
	if len(p.Set.Packs) > 0 && a.templatesReady() {
		if res, err := art.Generate(a.templatesDir(), p.Folder, p.ID, p.Set.Name, art.Options{}); err == nil {
			applyArt(&p.Set.Packs[0], res)
			_ = a.ws().Save(p)
		}
	}
	return p.ID, nil
}

// ---------------------------------------------------------------- pack art

func (a *App) DefaultArtColor(id string) string { return art.DefaultColor(id) }

// GeneratePackArt renders pack/box textures + icons for one pack from the game's templates and returns the file paths.
// The caller assigns them to the pack (the files are written immediately; per-pack files avoid clobbering other packs).
func (a *App) GeneratePackArt(id string, packID string, o art.Options) (art.Result, error) {
	if !a.templatesReady() {
		return art.Result{}, errors.New("the game templates aren't available yet — check Settings → Game (TCG Studio reads them from the game folder)")
	}
	p, err := a.ws().Load(id)
	if err != nil {
		return art.Result{}, err
	}
	// Each pack gets its own files so several packs in one set can have different art.
	o.FilePrefix = ""
	if packID != "" && packID != "booster" && setfmt.SafeID(packID) {
		o.FilePrefix = packID + "_"
	}
	return art.Generate(a.templatesDir(), p.Folder, p.ID, p.Set.Name, o)
}

// ---------------------------------------------------------------- card backs

const globalBackFile = "card_back.png" // read by the mod from <plugin>\card_back.png

// ChooseSetCardBack lets the user pick an image, composes it into the vanilla card-back shape and returns its path.
func (a *App) ChooseSetCardBack(id string) (string, error) {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose a card back image", Filters: imageFilters})
	if err != nil || file == "" {
		return "", err
	}
	rel := "images/card_back.png"
	return rel, art.CardBackFromFile(file, filepath.Join(a.ws().Folder(id), filepath.FromSlash(rel)))
}

// GenerateSetCardBack makes a card back from a colour, the set icon and a title.
func (a *App) GenerateSetCardBack(id, color, title string) (string, error) {
	if !setfmt.SafeID(id) {
		return "", errors.New("bad project id")
	}
	rel := "images/card_back.png"
	folder := a.ws().Folder(id)
	return rel, art.GenerateCardBack(folder, "", color, title, filepath.Join(folder, filepath.FromSlash(rel)))
}

func (a *App) globalBackPath() (string, error) {
	if !game.IsGameDir(a.settings.GameDir) {
		return "", errors.New("game folder not set (Settings → Game)")
	}
	return filepath.Join(game.PluginDir(a.settings.GameDir), globalBackFile), nil
}

// GlobalCardBack returns a URL for the current global card back, or "" when none is set.
func (a *App) GlobalCardBack() string {
	p, err := a.globalBackPath()
	if err != nil {
		return ""
	}
	info, err := os.Stat(p)
	if err != nil {
		return ""
	}
	return "/globalback/" + globalBackFile + "?v=" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
}

func (a *App) ChooseGlobalCardBack() (string, error) {
	p, err := a.globalBackPath()
	if err != nil {
		return "", err
	}
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose the global card back image", Filters: imageFilters})
	if err != nil || file == "" {
		return a.GlobalCardBack(), err
	}
	if err := art.CardBackFromFile(file, p); err != nil {
		return "", err
	}
	return a.GlobalCardBack(), nil
}

func (a *App) GenerateGlobalCardBack(color, title string) (string, error) {
	p, err := a.globalBackPath()
	if err != nil {
		return "", err
	}
	if err := art.GenerateCardBack("", "", color, title, p); err != nil {
		return "", err
	}
	return a.GlobalCardBack(), nil
}

func (a *App) RemoveGlobalCardBack() error {
	p, err := a.globalBackPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func applyArt(pk *setfmt.Pack, r art.Result) {
	pk.PackTexture, pk.PackIcon, pk.BoxTexture, pk.BoxIcon = r.PackTexture, r.PackIcon, r.BoxTexture, r.BoxIcon
}

func (a *App) CancelImport() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}

// ---------------------------------------------------------------- projects

func (a *App) ListProjects() ([]project.Summary, error) {
	<-a.started
	return a.ws().List(a.settings.GameDir)
}

func (a *App) LoadProject(id string) (*project.Project, error) { return a.ws().Load(id) }

func (a *App) CreateProject(id, name string) (*project.Project, error) {
	return a.ws().Create(id, name)
}

// DeleteProject removes the project and, if installed, its copy in the game (so no orphaned set is left behind).
// Player saves keep their data for the set; it comes back if a set with the same id is installed again.
func (a *App) DeleteProject(id string) error {
	if game.IsGameDir(a.settings.GameDir) {
		if err := project.Uninstall(id, a.settings.GameDir); err != nil {
			return err
		}
	}
	return a.ws().Delete(id)
}

// SaveResult reports a save or install: validation issues and the state of the game's copy.
type SaveResult struct {
	Issues       []setfmt.Issue `json:"issues"`
	Synced       bool           `json:"synced"`       // the game's copy was updated
	InstallState string         `json:"installState"` // project.StateNone / StateCurrent / StateStale after the call
	GameRunning  bool           `json:"gameRunning"`  // the game must be restarted to load the change
	SyncError    string         `json:"syncError"`    // why an installed set couldn't be updated (e.g. validation errors)
}

// SaveProject writes the project (even with issues) and, when the set is installed in the game, updates the game's copy too
// (unless validation found errors) — so "Save" is all it takes for edits to reach the game.
func (a *App) SaveProject(p project.Project) (SaveResult, error) {
	if !setfmt.SafeID(p.ID) {
		return SaveResult{}, errors.New("bad project id")
	}
	p.Folder = a.ws().Folder(p.ID)
	p.Set.ID = p.ID
	if err := a.ws().Save(&p); err != nil {
		return SaveResult{}, err
	}
	r := SaveResult{Issues: issues(p.Set.Validate(p.Folder))}
	if !game.IsGameDir(a.settings.GameDir) {
		r.InstallState = project.StateNone
		return r, nil
	}
	r.InstallState = project.InstallState(&p, a.settings.GameDir)
	if r.InstallState == project.StateStale {
		if hasErrors(r.Issues) {
			r.SyncError = "fix the errors so the game can be updated"
		} else if err := a.ws().Install(&p, a.settings.GameDir); err != nil {
			r.SyncError = err.Error()
		} else {
			r.Synced = true
			r.InstallState = project.StateCurrent
		}
		r.GameRunning = r.Synced && game.IsRunning()
	}
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
	p, err := a.ws().Load(id)
	if err != nil {
		return "", err
	}
	return project.InstallState(p, a.settings.GameDir), nil
}

func (a *App) ValidateProject(id string) ([]setfmt.Issue, error) {
	p, err := a.ws().Load(id)
	if err != nil {
		return nil, err
	}
	return issues(p.Set.Validate(p.Folder)), nil
}

// InstallProject validates and copies the set into the game. Refuses when there are errors.
func (a *App) InstallProject(id string) (SaveResult, error) {
	p, err := a.ws().Load(id)
	if err != nil {
		return SaveResult{}, err
	}
	r := SaveResult{Issues: issues(p.Set.Validate(p.Folder))}
	if hasErrors(r.Issues) {
		r.InstallState = project.InstallState(p, a.settings.GameDir)
		return r, errors.New("fix the errors before installing")
	}
	if err := a.ws().Install(p, a.settings.GameDir); err != nil {
		return r, err
	}
	r.Synced, r.InstallState, r.GameRunning = true, project.StateCurrent, game.IsRunning()
	return r, nil
}

func (a *App) UninstallProject(id string) error { return project.Uninstall(id, a.settings.GameDir) }

func (a *App) OpenProjectFolder(id string) {
	if setfmt.SafeID(id) {
		_ = exec.Command("explorer", a.ws().Folder(id)).Start()
	}
}

// ---------------------------------------------------------------- gamify & prices

func (a *App) GamifyDefaults() gamify.Settings { return gamify.DefaultSettings() }

// GamifyPreview shows what Gamify would do to the given projects (in tier order) without saving.
func (a *App) GamifyPreview(order []string, s gamify.Settings) ([]gamify.Preview, error) {
	return a.gamify(order, s, false, false)
}

// GamifyApply re-prices and saves the given projects (in tier order); reinstall also updates sets already installed in the game.
func (a *App) GamifyApply(order []string, s gamify.Settings, reinstall bool) ([]gamify.Preview, error) {
	return a.gamify(order, s, true, reinstall)
}

func (a *App) gamify(order []string, s gamify.Settings, save, reinstall bool) ([]gamify.Preview, error) {
	out := []gamify.Preview{}
	for i, id := range order {
		p, err := a.ws().Load(id)
		if err != nil {
			return nil, err
		}
		set := s
		if len(order) <= 9 {
			set.MaxLevel = 0 // one set per vanilla product: keep vanilla unlock levels
		}
		pv := gamify.Apply(p, i+1, gamify.Position(i, len(order)), set)
		pv.Installed = a.isInstalled(id)
		if save {
			if err := a.ws().Save(p); err != nil {
				return nil, err
			}
			// Keep the game in sync: sets that are already installed get the new prices/licenses right away.
			if reinstall && pv.Installed {
				if err := a.ws().Install(p, a.settings.GameDir); err != nil {
					return nil, fmt.Errorf("saved %s but reinstalling it failed: %w", p.Set.Name, err)
				}
			}
		}
		out = append(out, pv)
	}
	return out, nil
}

func (a *App) isInstalled(id string) bool {
	if !game.IsGameDir(a.settings.GameDir) || !setfmt.SafeID(id) {
		return false
	}
	_, err := os.Stat(filepath.Join(game.SetsDir(a.settings.GameDir), id, project.SetFile))
	return err == nil
}

// RefreshPrices pulls current prices from the set's import source and re-applies the project's last pricing settings (licenses untouched).
func (a *App) RefreshPrices(id string) (string, error) {
	p, err := a.ws().Load(id)
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
	if err := a.ws().Save(p); err != nil {
		return "", err
	}
	if a.isInstalled(id) {
		if err := a.ws().Install(p, a.settings.GameDir); err != nil {
			return msg, fmt.Errorf("prices updated but reinstalling failed: %w", err)
		}
		msg += "; reinstalled in the game"
	}
	return msg, nil
}

var imageFilters = []runtime.FileFilter{{DisplayName: "Images (*.png;*.jpg;*.jpeg)", Pattern: "*.png;*.jpg;*.jpeg"}}

// PickImage lets the user choose one image and copies it into the project's images folder. Returns the relative path ("" if cancelled).
func (a *App) PickImage(id, title string) (string, error) {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: title, Filters: imageFilters})
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
	return a.writeIntoProject(id, filepath.Base(file), b)
}

// writeIntoProject stores an image under images/ with a free name based on name and returns its relative path.
func (a *App) writeIntoProject(id, name string, b []byte) (string, error) {
	if !setfmt.SafeID(id) {
		return "", errors.New("bad project id")
	}
	dir := filepath.Join(a.ws().Folder(id), "images")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := strings.ReplaceAll(filepath.Base(name), " ", "_")
	ext := filepath.Ext(base)
	name = base
	for n := 2; ; n++ {
		if _, err := os.Stat(filepath.Join(dir, name)); os.IsNotExist(err) {
			break
		}
		name = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(base, ext), n, ext)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		return "", err
	}
	return "images/" + name, nil
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

// ---------------------------------------------------------------- files for the frontend

// fileHandler serves project images at /proj/<id>/<relative path>, game templates at /templates/<file>, accessory templates at
// /acctemplates/<file>, furniture templates at /furntemplates/<file> and accessory library files at /acc/<relative path>.
func (a *App) fileHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		var file string
		switch {
		case strings.HasPrefix(path, "proj/"):
			parts := strings.SplitN(strings.TrimPrefix(path, "proj/"), "/", 2)
			if len(parts) != 2 || !setfmt.SafeID(parts[0]) || strings.Contains(parts[1], "..") {
				http.NotFound(w, r)
				return
			}
			file = filepath.Join(a.ws().Folder(parts[0]), filepath.FromSlash(parts[1]))
		case strings.HasPrefix(path, "templates/"):
			name := strings.TrimPrefix(path, "templates/")
			if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
				http.NotFound(w, r)
				return
			}
			file = filepath.Join(a.templatesDir(), name)
		case strings.HasPrefix(path, "acctemplates/"):
			name := strings.TrimPrefix(path, "acctemplates/")
			if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
				http.NotFound(w, r)
				return
			}
			file = filepath.Join(a.templatesDir(), "accessories", name)
		case strings.HasPrefix(path, "furntemplates/"):
			name := strings.TrimPrefix(path, "furntemplates/")
			if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
				http.NotFound(w, r)
				return
			}
			file = filepath.Join(a.templatesDir(), gameextract.FurnitureDir, name)
		case strings.HasPrefix(path, "acc/"):
			rel := strings.TrimPrefix(path, "acc/")
			if strings.Contains(rel, "..") {
				http.NotFound(w, r)
				return
			}
			file = filepath.Join(accessories.Folder(a.root()), filepath.FromSlash(rel))
		case path == "globalback/"+globalBackFile:
			file = filepath.Join(game.PluginDir(a.settings.GameDir), globalBackFile)
		default:
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, file)
	})
}

// Confirm shows a native Yes/No dialog titled "TCG Studio" (the browser confirm() is titled "wails.localhost says").
func (a *App) Confirm(message string) bool {
	res, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type: runtime.QuestionDialog, Title: "TCG Studio", Message: message, DefaultButton: "No",
	})
	return err == nil && res == "Yes"
}

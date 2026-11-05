package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"tcgstudio/internal/catalog"
	"tcgstudio/internal/fsutil"
	"tcgstudio/internal/library"
	"tcgstudio/internal/migrate/relayout"
	"tcgstudio/internal/project"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// migration:art-layouts-1.03
// The game's 2026-10-27 update gave packs, boxes and accessories new models with new texture layouts. Art made with an
// earlier TCG Studio is converted when the player chooses to (ConvertArtLayouts, after the warnings on the notice): it
// then no longer suits older game versions — going back = installing the earlier TCG Studio (legacyRelease). What is in the
// new layouts is marked on the art itself (project Meta.ArtKinds, accessories Meta.ArtCurrent): new content is marked when
// made, copies and shared files keep their source's mark, so art without a mark is the earlier version's.

// setKinds: the set models whose art moved to new layouts; the accessory art keys (relayout.AccessoryKeys) follow them.
var setKinds = project.CurrentArtKinds

// relayoutKinds: every kind and accessory art key, in conversion order.
var relayoutKinds = append(slices.Clone(setKinds), relayout.AccessoryKeys()...)

// kindNames: how the notice names each kind.
var kindNames = map[string]string{"Box": "card boxes", "Pack": "card packs", "Deckbox": "deck boxes", "Sleeve": "sleeves",
	"BattleDeck": "battle decks", "Binder": "collection books", "Dice": "dice boxes", "UP_Playmat1": "Ultra PRO playmat tubes",
	"UP_Album": "Ultra PRO albums"}

// accessoriesItem is the work item of a setup's (or the catalog's) accessory library, next to its sets.
const accessoriesItem = "@accessories"

// legacyRelease is the last TCG Studio for the game's previous models.
const legacyRelease = "v1.14.0"

func legacyReleaseURL() string {
	repo := UpdateRepo
	if repo == "" {
		repo = "GrimGrab/TCG-Studio"
	}
	return "https://github.com/" + repo + "/releases/tag/" + legacyRelease
}

// ArtLayoutStatus: art still made for the game's previous models (the start-up notice).
type ArtLayoutStatus struct {
	Needed      bool     `json:"needed"`
	Kinds       []string `json:"kinds"`       // e.g. "card packs", "card boxes"
	Sets        int      `json:"sets"`        // sets in the setups / catalog still to convert
	Accessories bool     `json:"accessories"` // accessory art too
	Setups      []string `json:"setups"`      // their names ("Catalog" for the catalog)
	RollbackURL string   `json:"rollbackUrl"` // the earlier TCG Studio's release page
	Version     string   `json:"version"`     // its version
}

// artJob is one setup (or the catalog) with sets still to convert: per set, its kinds still to do.
type artJob struct {
	name, dir string
	sets      map[string][]string // set id → kinds still to do
	order     []string            // set ids in folder order
}

// artPlan lists what is still to convert; nil when the current templates aren't there (Studio still reading the game).
func (a *App) artPlan() []artJob {
	tpl := a.templatesDir()
	if !templatesReadyIn(tpl) || a.TemplatesStatus().Busy {
		return nil
	}
	var ready []string // kinds with a new layout here
	for _, k := range relayoutKinds {
		if _, _, changed := relayout.Models(k); changed {
			ready = append(ready, k)
		}
	}
	if len(ready) == 0 {
		return nil
	}
	type root struct{ name, dir string }
	var roots []root
	if list, err := a.home().List(); err == nil {
		for _, s := range list {
			roots = append(roots, root{s.Name, s.Folder})
		}
	}
	roots = append(roots, root{"Catalog", catalog.For(a.workspaceDir()).Root})

	var jobs []artJob
	for _, r := range roots {
		ws := a.wsDir().Projects(r.dir)
		j := artJob{name: r.name, dir: r.dir, sets: map[string][]string{}}
		if es, err := os.ReadDir(ws.ProjectsDir()); err == nil {
			for _, e := range es {
				id := e.Name()
				if !e.IsDir() || !fsutil.Exists(filepath.Join(ws.Folder(id), project.SetFile)) {
					continue
				}
				have := ws.ReadArtKinds(id)
				var kinds []string
				for _, k := range ready {
					if slices.Contains(setKinds, k) && !slices.Contains(have, k) {
						kinds = append(kinds, k)
					}
				}
				if len(kinds) > 0 {
					j.sets[id] = kinds
					j.order = append(j.order, id)
				}
			}
		}
		if lib, err := a.wsDir().Items(r.dir); err == nil {
			if kinds := relayout.PendingAccessoryKeys(lib, ready); len(kinds) > 0 {
				j.sets[accessoriesItem] = kinds
				j.order = append(j.order, accessoriesItem)
			}
		}
		if len(j.order) > 0 {
			jobs = append(jobs, j)
		}
	}
	return jobs
}

// markArtKinds adds kinds to the set's mark.
func markArtKinds(p *project.Project, kinds []string) {
	for _, k := range kinds {
		if !slices.Contains(p.Meta.ArtKinds, k) {
			p.Meta.ArtKinds = append(p.Meta.ArtKinds, k)
		}
	}
}

// ArtLayoutStatus reports art still made for the game's previous models (the notice and its warnings).
func (a *App) ArtLayoutStatus() ArtLayoutStatus {
	st := ArtLayoutStatus{Kinds: []string{}, Setups: []string{}, Version: legacyRelease, RollbackURL: legacyReleaseURL()}
	seen := map[string]bool{}
	for _, j := range a.artPlan() {
		st.Needed = true
		st.Setups = append(st.Setups, j.name)
		for _, id := range j.order {
			if id == accessoriesItem {
				st.Accessories = true
			} else {
				st.Sets++
			}
			for _, k := range j.sets[id] {
				if !seen[k] {
					seen[k] = true
					st.Kinds = append(st.Kinds, kindNames[k])
				}
			}
		}
	}
	return st
}

// ArtConversionResult is what a conversion ended with (the art:converted event).
type ArtConversionResult struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

// errArtConverting: a save while the conversion runs would put the file names it has just replaced back.
var errArtConverting = errors.New("your art is being converted: save again when that has finished")

// BeginArtConversion starts ConvertArtLayouts as a storage task and returns once the task is registered, so the Storage page
// opened next shows its progress (it adopts a running task) instead of starting its own; the result comes as art:converted.
func (a *App) BeginArtConversion() error {
	jobs := a.artPlan()
	if len(jobs) == 0 {
		return errors.New("nothing to convert")
	}
	ctx, done, err := a.startStorage(autoTaskLabel)
	if err != nil {
		return err
	}
	a.artConverting.Store(true)
	go func() {
		defer done()
		defer a.artConverting.Store(false)
		msg, err := a.convertArtLayouts(ctx, jobs)
		res := ArtConversionResult{Message: msg}
		if err != nil {
			res.Error = err.Error()
		}
		if a.ctx != nil {
			wruntime.EventsEmit(a.ctx, evArtConverted, res)
		}
	}()
	return nil
}

// convertArtLayouts moves art made for the game's previous models to the current layouts (relayout.Project) in every
// setup and the catalog: art and designs are converted into new files (old files kept) and saved with their mark.
func (a *App) convertArtLayouts(ctx context.Context, jobs []artJob) (string, error) {
	tpl := a.templatesDir()
	convertSet := func(dir, id string, kinds []string) (bool, error) {
		if id == accessoriesItem {
			lib, err := a.wsDir().Items(dir)
			if err != nil {
				return false, err
			}
			changed, err := relayout.Accessories(lib, tpl, kinds)
			if changed { // also after a failure: what did convert is kept
				err = errors.Join(err, lib.Save())
			}
			return changed, err
		}
		ws := a.wsDir().Projects(dir)
		p, err := ws.Load(id)
		if err != nil {
			return false, err
		}
		changed, err := relayout.Project(p, tpl, kinds)
		if err != nil {
			return false, err
		}
		markArtKinds(p, kinds)
		return changed, ws.Save(p)
	}
	return runArtJobs(ctx, jobs, convertSet, a.storageProgress)
}

// runArtJobs converts every job's sets with convertSet (dir = the setup or catalog folder) on parallel workers. convertSet
// marks what it converted, so a stopped or failed run goes on where it was, never converting a set twice.
func runArtJobs(ctx context.Context, jobs []artJob, convertSet func(dir, id string, kinds []string) (bool, error),
	progress func(library.Progress)) (string, error) {
	// Work items grouped by set id: the copies of one set in different setups and the catalog share its library folder,
	// so one worker converts them one after another (never two writes into the same folder at once).
	type item struct {
		job int
		id  string
	}
	groups := map[string][]item{}
	var ids []string
	total := 0
	for ji, j := range jobs {
		for _, id := range j.order {
			if groups[id] == nil {
				ids = append(ids, id)
			}
			groups[id] = append(groups[id], item{ji, id})
			total++
		}
	}
	var mu sync.Mutex // guards everything below: counts, failures
	n, converted := 0, 0
	var failed []string
	convert := func(it item) {
		j := jobs[it.job]
		changed, err := convertSet(j.dir, it.id, j.sets[it.id])
		mu.Lock()
		defer mu.Unlock()
		n++
		progress(library.Progress{Message: "Converting your art to the game's new models", Done: n, Total: total})
		if err != nil {
			log.Printf("relayout %s/%s: %v", j.dir, it.id, err)
			what := it.id
			if it.id == accessoriesItem {
				what = "accessories " + err.Error()
			}
			failed = append(failed, j.name+": "+what)
			return
		}
		if changed {
			converted++
		}
	}

	workers := min(max(1, runtime.NumCPU()-1), 8, len(ids)) // one core left for Studio itself
	queue := make(chan string)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range queue {
				for _, it := range groups[id] {
					if ctx.Err() != nil {
						return
					}
					convert(it)
				}
			}
		}()
	}
	progress(library.Progress{Message: "Converting your art to the game's new models", Done: 0, Total: total})
feed:
	for _, id := range ids {
		select {
		case queue <- id:
		case <-ctx.Done():
			break feed
		}
	}
	close(queue)
	wg.Wait()
	if ctx.Err() != nil {
		return fmt.Sprintf("Stopped: %d set(s) converted; the rest is still to do.", converted), nil
	}
	if len(failed) > 0 {
		return "", errors.New("these sets couldn't be converted (the others were): " + strings.Join(failed, ", "))
	}
	return fmt.Sprintf("Converted the art of %d set(s) to the game's new models.", converted), nil
}

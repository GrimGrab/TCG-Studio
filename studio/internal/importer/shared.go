package importer

import (
	"context"
	"errors"

	"tcgstudio/internal/project"
)

// sharedSource is "TCG Studio file" on the Import page: a .tcgitems file another player exported from their Catalog
// page (any sets, accessories, furniture, decorations). Like the EPL source it only appears in the source picker; the
// page reads the file through App.PreviewShare / App.ImportShare (internal/share). Imported sets keep their own import
// source (Scryfall, …) in their meta, so refreshes keep working; this source never owns a set.
type sharedSource struct{}

// SharedSourceID is the Import page's id of item files.
const SharedSourceID = "tcgitems"

func (sharedSource) Info() SourceInfo {
	return SourceInfo{ID: SharedSourceID, Name: "TCG Studio file", Game: "Import shared sets & items", Local: true, OwnArt: true}
}

func (sharedSource) DefaultOptions() Options                         { return Options{} }
func (sharedSource) Sets(context.Context, string) ([]SetInfo, error) { return nil, nil }
func (sharedSource) ProjectID(code, _ string) string                 { return code }

func (sharedSource) Import(context.Context, project.Workspace, string, Options, func(Progress)) (*project.Project, error) {
	return nil, errors.New("item files are imported from the Import page's file view")
}

func (sharedSource) RefreshMeta(context.Context, *project.Project) (int, error) { return 0, nil }

// Package migrate is the one list of everything that brings older data up to date: what each step does, what it reads and when
// it can be deleted. Each entry's code is marked with a "migration:<id>" comment, in Go, C# and the frontend alike; searching for
// that is how to find (and remove) all of it, and TestRegistry keeps the marks and this list in step.
//
// The code moved here where it can be: OldLayout, relayout (art for new models), designs (editor
// designs on load). What runs inside another package's own flow stays there, marked: settings in the mod and modconfig (either
// can open the file first), FillMtg and FillNames (imports use them too), editor conversions (frontend).
package migrate

// Migration is one thing older data needs.
type Migration struct {
	ID         string
	What       string
	Since      string // when it arrived
	Reads      string // the old data it looks for
	RemoveWhen string // what has to be true before deleting it
}

// Registry lists every migration, oldest first.
var Registry = []Migration{
	{"setups-layout", "A workspace from before setups moves into setups\\default", "2026-10-02",
		"projects\\ directly in the workspace", "No player can update from a build before setups"},
	{"save-backups", "Deletes the save copies older versions made before the first setup switch", "2026-10-02",
		"saves-backup-<time> folders", "Same as setups-layout"},
	{"setup-archive-v2", "Reads .tcgsetup files of format 1 and 2", "2026-10-16",
		"Setup files with an older FormatVersion", "Old setup files no longer matter: that breaks files players kept, so announce it"},
	{"set-sync", "Installed sets get what a new Studio adds (MTG export data, Scryfall names and layouts)", "2026-09-26",
		"Sets without the data", "Keep: runs once per Studio version; remove only the fills no set can still lack"},
	{"old-layout-check", "A workspace from before shared storage is sent to TCG Studio v1.14.0, which upgrades it", "2026-10-30",
		"Files in project and setup folders, named art folders, game folder content with no setup marked", "Returning players with such a workspace no longer matter (keep the v1.14.0 release up until then)"},
	{"net-v2-pack", "Pack designs: layers follow the faces that grew", "2026-10-17",
		"Pack layouts with net below 2", "No design can be older than net 2: Pack.NetMoves goes too"},
	{"net-v3-box", "Box designs: layers on front, right and top also go on back, left and bottom", "2026-10-30",
		"Box layouts with net below 3", "No design can be older than net 3: Box.NetCopies goes too"},
	{"art-layouts-1.03", "Art for game 1.02's models is redrawn for the current ones; marks say which art is done; EPL bundles older than Unity 6 count as old", "2026-10-29",
		"Sets and accessories without artKinds / artCurrent", "Art from before the update no longer matters: relayout, legacy.go and the 1.02 tables in models.go and tabletop.go all go; the marks can then be dropped too"},
	{"editor-face-fills", "Whole-face colours become colour rectangles when an editor opens", "2026-10-18",
		"Fill layers with no size on face-net models", "No saved design has such a layer: Smart generate and the editor no longer make them"},
	{"editor-layout-v1", "A design of the first layout format is replaced by an empty one", "2026-09-26",
		"Layouts with no version 2", "No saved design has an older version"},
	{"installer-templates", "Install / Repair deletes the art templates older mods exported in game", "2026-09-29",
		"<plugin>\templates", "No player can update from a mod before v0.8"},
	{"settings-moved", "A setting moved to another section keeps its value", "2026-09-28",
		"An unbound entry with the same key", "Keep: the general mechanism for section moves"},
	{"settings-binder-layout", "[Binder] Layout removed; its value is the default for older saves", "2026-10-12",
		"[Binder] Layout", "Saves that old no longer matter"},
	{"settings-credit-card", "Single credit card picture settings removed", "2026-10-17",
		"[Visuals - Game look] CreditCardImage, CreditCardCrop", "No config can still have them"},
	{"settings-bulk-boxes", "Tetramon bulk box designs carry over to BulkBoxNG…UG; Destiny ones are dropped", "2026-10-29",
		"BulkBox_Tetramon* settings", "No config can still have them"},
	{"import-page-paths", "Forgets paths older versions remembered on the Import page", "2026-10-16",
		"localStorage import.epl, import.share", "Harmless to delete after a release or two"},
}

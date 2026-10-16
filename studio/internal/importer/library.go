package importer

import (
	"os"
	"path/filepath"

	"tcgstudio/internal/project"
)

// artTarget is where an import saves a set's card art.
type artTarget struct {
	dir    string // folder for the files (…\images)
	ext    string // ".png" or ".jpg"
	format string // "png" or "jpg"
	shared bool   // files already there are kept (another setup downloaded them)
}

// cardArtTarget is the set's shared folder (<workspace>\library\<id>\images). Files already there are kept (another
// setup downloaded them) when they were made the same way (same source, set, language, width and format), or when the
// player chose the art already on this PC (UseLibrary: its format and size are taken). Otherwise the import downloads
// again over them and records its own settings.
func cardArtTarget(ws project.Workspace, id, source, code, lang string, opt Options) (artTarget, error) {
	t := artTarget{ext: ".png", format: "png"}
	if opt.ImageFormat == "jpg" {
		t.ext, t.format = ".jpg", "jpg"
	}
	lib := ws.LibFolder(id)
	t.dir = filepath.Join(lib, "images")
	want := project.LibMeta{Source: source, Code: code, Lang: lang, Width: opt.ImageWidth, Format: t.format}
	switch have := project.LoadLibMeta(lib); {
	case have != nil && *have == want:
		t.shared = true
	case have != nil && opt.UseLibrary && have.Source == want.Source && have.Code == want.Code && have.Lang == want.Lang:
		t.shared, t.format, t.ext = true, have.Format, "."+have.Format
	default:
		if err := project.SaveLibMeta(lib, &want); err != nil {
			return t, err
		}
	}
	return t, os.MkdirAll(t.dir, 0o755)
}

func (t artTarget) job(url, cid string) imageJob {
	return imageJob{url: url, path: filepath.Join(t.dir, cid+t.ext), format: t.format, keep: t.shared}
}

// rel is the card's image path in set.json (in the set's shared folder).
func (t artTarget) rel(cid string) string { return "images/" + cid + t.ext }

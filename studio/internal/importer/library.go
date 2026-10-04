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
	shared bool   // the shared library: files already there are kept (another setup downloaded them)
}

// cardArtTarget picks the shared card-art library (<workspace>\library\<id>\images) when it has no art for this set yet or
// art made the same way (same source, set, language, width and format); otherwise the project's own images folder, so
// two different versions of a set never mix.
func cardArtTarget(ws project.Workspace, id, folder, source, code, lang string, opt Options) (artTarget, error) {
	t := artTarget{dir: filepath.Join(folder, "images"), ext: ".png", format: "png"}
	if opt.ImageFormat == "jpg" {
		t.ext, t.format = ".jpg", "jpg"
	}
	if ws.Library != "" {
		lib := filepath.Join(ws.Library, id)
		want := project.LibMeta{Source: source, Code: code, Lang: lang, Width: opt.ImageWidth, Format: t.format}
		have := project.LoadLibMeta(lib)
		switch {
		case have == nil:
			if err := project.SaveLibMeta(lib, &want); err != nil {
				return t, err
			}
			t.dir, t.shared = filepath.Join(lib, "images"), true
		case *have == want:
			t.dir, t.shared = filepath.Join(lib, "images"), true
		case opt.UseLibrary && have.Source == want.Source && have.Code == want.Code && have.Lang == want.Lang:
			// The player chose the art already on this PC: take its format (and size) instead of downloading another copy.
			t.dir, t.shared, t.format = filepath.Join(lib, "images"), true, have.Format
			t.ext = "." + have.Format
		}
	}
	return t, os.MkdirAll(t.dir, 0o755)
}

func (t artTarget) job(url, cid string) imageJob {
	return imageJob{url: url, path: filepath.Join(t.dir, cid+t.ext), format: t.format, keep: t.shared}
}

// rel is the card's image path in set.json (resolved from the project folder, then the library).
func (t artTarget) rel(cid string) string { return "images/" + cid + t.ext }

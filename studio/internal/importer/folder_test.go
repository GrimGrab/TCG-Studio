package importer

import (
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"tcgstudio/internal/project"
)

func writeImage(t *testing.T, path string, w, h int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if filepath.Ext(path) == ".jpg" {
		err = jpeg.Encode(f, img, nil)
	} else {
		err = png.Encode(f, img)
	}
	if err != nil {
		t.Fatal(err)
	}
}

// A folder with rarity subfolders, top-level images, a logo, a landscape image and a cards.csv imports as one set.
func TestFolderImport(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Heroines of Marvel")
	writeImage(t, filepath.Join(dir, "1 Common", "001 Captain Marvel.png"), 500, 700)
	writeImage(t, filepath.Join(dir, "1 Common", "002_Spider-Woman.jpg"), 500, 700)
	writeImage(t, filepath.Join(dir, "2 Shiny", "Black Widow.png"), 700, 500) // landscape, no number, unknown rarity word
	writeImage(t, filepath.Join(dir, "3 Legendary", "010 Storm.png"), 500, 700)
	writeImage(t, filepath.Join(dir, "Wasp.png"), 300, 700) // off-shape, top level
	writeImage(t, filepath.Join(dir, "logo.png"), 200, 100)
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	csv := "\xef\xbb\xbfFile;Name;Rarity;Price\n002_Spider-Woman.jpg;Jessica Drew;Rare;$1,50\nWasp;;Epic;\nmissing.png;Nobody;;\n"
	if err := os.WriteFile(filepath.Join(dir, "cards.csv"), []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}

	pv, err := ScanFolder(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if pv.ProjectID != "img-heroines-of-marvel" || pv.Cards != 5 || !pv.Logo || !pv.CSV || pv.CSVRows != 3 || pv.CSVMatched != 2 ||
		!slices.Equal(pv.Unmatched, []string{"missing.png"}) || pv.Rotated != 1 || pv.OffShape != 1 || len(pv.Skipped) != 1 {
		t.Fatalf("preview: %+v", pv)
	}

	ws := project.Workspace{Root: t.TempDir(), Library: project.LibraryDir(t.TempDir())}
	p, err := folderSource{}.Import(context.Background(), ws, dir, folderSource{}.DefaultOptions(), func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	type want struct{ id, number, name, rarity, src string }
	var got []want
	for _, c := range p.Set.Cards {
		got = append(got, want{c.ID, c.Number, c.Name, c.Rarity, p.Meta.Cards[c.ID].SrcRarity})
	}
	// File name = name + id (numbers stay in the name by default); cards numbered in file-name order across subfolders.
	exp := []want{
		{"001-captain-marvel", "1", "001 Captain Marvel", "Common", "Common"},
		{"002-spider-woman", "2", "Jessica Drew", "Epic", "Rare"}, // CSV beats folder + file name; id stays the file's
		{"010-storm", "3", "010 Storm", "Legendary", "Legendary"},
		{"black-widow", "4", "Black Widow", "Rare", "Shiny"}, // unknown "Shiny" = 2nd of 5 rarities
		{"wasp", "5", "Wasp", "Epic", "Epic"},
	}
	if !slices.Equal(got, exp) {
		t.Fatalf("cards:\n got %v\nwant %v", got, exp)
	}
	if m := p.Meta.Cards["002-spider-woman"]; m.USD == nil || *m.USD != 1.5 {
		t.Errorf("price: %+v", m.USD)
	}
	if p.Meta.SourceDir != dir || !slices.Equal(p.Meta.RarityOrder, []string{"Common", "Shiny", "Legendary", "Rare", "Epic"}) {
		t.Errorf("meta: %q %v", p.Meta.SourceDir, p.Meta.RarityOrder)
	}
	for _, i := range p.Set.Validate(p.Folder, p.LibFolder) {
		if i.Level == "error" {
			t.Errorf("validation: %+v", i)
		}
	}
	// The landscape card was turned upright; the off-shape one kept its shape.
	shape := func(rel string) image.Config {
		path := filepath.Join(p.LibFolder, rel)
		if _, err := os.Stat(path); err != nil {
			path = filepath.Join(p.Folder, rel)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		c, _, err := image.DecodeConfig(f)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	if c := shape("images/black-widow.png"); c.Width >= c.Height {
		t.Errorf("landscape not rotated: %dx%d", c.Width, c.Height)
	}
	if c := shape("images/wasp.png"); c.Width != 300 || c.Height != 700 {
		t.Errorf("off-shape image changed: %dx%d", c.Width, c.Height)
	}

	// Refresh prices re-reads cards.csv.
	if err := os.WriteFile(filepath.Join(dir, "cards.csv"), []byte("file,price\n001 Captain Marvel.png,3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (folderSource{}).RefreshMeta(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if m := p.Meta.Cards["001-captain-marvel"]; m.USD == nil || *m.USD != 3 {
		t.Errorf("refreshed price: %+v", m.USD)
	}
}

// Names starting with a number stay whole by default; "Strip leading numbers" makes that number the card number.
func TestFolderStripNumbers(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Names")
	for _, f := range []string{"2099 Spider-Man.png", "3-D Man.png", "Abomination.png", "007.png"} {
		writeImage(t, filepath.Join(dir, f), 500, 700)
	}
	samples := func(opt Options) []FolderSample {
		pv, err := ScanFolder(dir, opt)
		if err != nil {
			t.Fatal(err)
		}
		return pv.Samples
	}
	if got, exp := samples(Options{}), []FolderSample{{"3-D Man.png", "1", "3-D Man"}, {"007.png", "2", "007"},
		{"2099 Spider-Man.png", "3", "2099 Spider-Man"}, {"Abomination.png", "4", "Abomination"}}; !slices.Equal(got, exp) {
		t.Errorf("whole names:\n got %v\nwant %v", got, exp)
	}
	if got, exp := samples(Options{StripNumbers: true}), []FolderSample{{"3-D Man.png", "3", "D Man"}, {"007.png", "007", "007"},
		{"2099 Spider-Man.png", "2099", "Spider-Man"}, {"Abomination.png", "2100", "Abomination"}}; !slices.Equal(got, exp) {
		t.Errorf("stripped:\n got %v\nwant %v", got, exp)
	}
}

func TestFolderRarity(t *testing.T) {
	f := folderRarityFunc([]string{"Bronze", "Silver", "Gold", "Diamond"}, DefaultFolderRarityMap())
	for in, want := range map[string]string{"": "Common", "Bronze": "Common", "Silver": "Rare", "Gold": "Epic", "Diamond": "Legendary",
		"uncommon": "Rare", "Super Rare": "Epic", "Secret Rare": "Legendary", "Base": "Common"} {
		if got := f(in); got != want {
			t.Errorf("%q → %s, want %s", in, got, want)
		}
	}
}

func TestFolderEmpty(t *testing.T) {
	if _, err := ScanFolder(t.TempDir(), Options{}); err == nil {
		t.Error("an empty folder should fail")
	}
}

// Package forge installs Card-Forge (MTG rules engine + AI, GPL-3.0, https://github.com/Card-Forge/forge) and a private
// Java runtime next to the game for the mod's MTG mode. Layout (all under <game>\TCGForge\):
//
//	forge\          Forge program (release tarball minus the Adventure/mobile tools) + forge.profile.properties
//	jre\            Temurin 21 JRE (Forge 2.x needs Java 17+)
//	userdata\       Forge's user dir (decks, preferences) — kept apart from any personal Forge install
//	cache\          Forge's cache dir (downloaded card pictures)
//	tcgforge.json   Manifest the mod reads (Runtime/Mtg/ForgeInstall.cs): java, jar, JVM args. Written last.
package forge

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Pinned Forge release (the mod's deck export is tested against it). Digest from the GitHub release asset.
const (
	Version   = "2.0.14"
	forgeURL  = "https://github.com/Card-Forge/forge/releases/download/forge-2.0.14/forge-installer-2.0.14.tar.bz2"
	forgeSHA  = "e71749376945177d603d52a21a089bc1574330cf34599544b6bae1a04c83da41"
	forgeJar  = "forge-gui-desktop-2.0.14-jar-with-dependencies.jar"
	jreAPI    = "https://api.adoptium.net/v3/assets/latest/21/hotspot?architecture=x64&image_type=jre&os=windows&vendor=eclipse"
	DirName   = "TCGForge"
	Manifest  = "tcgforge.json"
	userAgent = "TCGStudio (MTG mode installer)"
)

// Release files the desktop game doesn't need (Adventure mode + editors, ~106 MB).
var skipFiles = map[string]bool{
	"adventure-editor-jar-with-dependencies.jar":            true,
	"forge-gui-mobile-dev-2.0.14-jar-with-dependencies.jar": true,
	"gdx-particle-editor.jar":                               true,
	"forge-adventure.exe":                                   true,
	"forge-adventure.cmd":                                   true,
	"forge-adventure.sh":                                    true,
	"forge-adventure.command":                               true,
	"adventure-editor.cmd":                                  true,
	"adventure-editor.sh":                                   true,
	"adventure-editor.command":                              true,
}

// ManifestData is tcgforge.json. Paths are relative to the TCGForge folder.
type ManifestData struct {
	ForgeVersion string    `json:"forgeVersion"`
	JavaVersion  string    `json:"javaVersion"`
	Java         string    `json:"java"`    // jre\bin\javaw.exe
	Forge        string    `json:"forge"`   // forge (working directory)
	Jar          string    `json:"jar"`     // forge\forge-gui-desktop-….jar
	UserDir      string    `json:"userDir"` // userdata
	JvmArgs      []string  `json:"jvmArgs"` // from the release's forge.cmd
	Installed    time.Time `json:"installed"`
}

type Status struct {
	Dir          string `json:"dir"`
	Installed    bool   `json:"installed"`
	ForgeVersion string `json:"forgeVersion"`
	JavaVersion  string `json:"javaVersion"`
	Outdated     bool   `json:"outdated"` // installed Forge differs from the pinned one
	Pinned       string `json:"pinned"`
	SizeMB       int    `json:"sizeMB"` // download size of an install
}

func Dir(game string) string { return filepath.Join(game, DirName) }

// Inspect reports whether MTG mode is installed in the game folder.
func Inspect(game string) Status {
	st := Status{Dir: Dir(game), Pinned: Version, SizeMB: 350}
	if game == "" {
		return st
	}
	m, err := readManifest(game)
	if err != nil {
		return st
	}
	st.ForgeVersion, st.JavaVersion = m.ForgeVersion, m.JavaVersion
	st.Installed = fileExists(filepath.Join(st.Dir, m.Java)) && fileExists(filepath.Join(st.Dir, m.Jar))
	st.Outdated = st.Installed && m.ForgeVersion != Version
	return st
}

func readManifest(game string) (*ManifestData, error) {
	b, err := os.ReadFile(filepath.Join(Dir(game), Manifest))
	if err != nil {
		return nil, err
	}
	var m ManifestData
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Progress is reported during Install.
type Progress struct {
	Stage   string `json:"stage"` // java | forge | extract | done
	Done    int64  `json:"done"`
	Total   int64  `json:"total"`
	Message string `json:"message"`
}

// Install downloads and unpacks the JRE and Forge (replacing an older copy; userdata and cache are kept).
func Install(ctx context.Context, game string, report func(Progress)) error {
	if game == "" {
		return fmt.Errorf("game folder not set")
	}
	root := Dir(game)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	// The manifest goes first so a half-finished install never looks installed.
	_ = os.Remove(filepath.Join(root, Manifest))
	tmp := filepath.Join(root, ".download")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	client := &http.Client{Timeout: 0}

	// ---- Java
	report(Progress{Stage: "java", Message: "Looking up Java 21…"})
	jre, err := latestJRE(ctx, client)
	if err != nil {
		return fmt.Errorf("finding Java: %w", err)
	}
	jreZip := filepath.Join(tmp, "jre.zip")
	if err := download(ctx, client, jre.Link, jreZip, jre.SHA256, func(d, t int64) {
		report(Progress{Stage: "java", Done: d, Total: t, Message: fmt.Sprintf("Downloading Java %s: %s", jre.Version, mb(d, t))})
	}); err != nil {
		return fmt.Errorf("downloading Java: %w", err)
	}
	report(Progress{Stage: "extract", Message: "Unpacking Java…"})
	if err := replaceDir(filepath.Join(root, "jre"), func(dst string) error { return unzipStrip(jreZip, dst) }); err != nil {
		return fmt.Errorf("unpacking Java: %w", err)
	}

	// ---- Forge
	forgeTar := filepath.Join(tmp, "forge.tar.bz2")
	if err := download(ctx, client, forgeURL, forgeTar, forgeSHA, func(d, t int64) {
		report(Progress{Stage: "forge", Done: d, Total: t, Message: fmt.Sprintf("Downloading Forge %s: %s", Version, mb(d, t))})
	}); err != nil {
		return fmt.Errorf("downloading Forge: %w", err)
	}
	var args []string
	if err := replaceDir(filepath.Join(root, "forge"), func(dst string) error {
		n := 0
		err := untarBz2(ctx, forgeTar, dst, func(rel string) bool { return !skipFiles[rel] }, func() {
			n++
			if n%500 == 0 {
				report(Progress{Stage: "extract", Message: fmt.Sprintf("Unpacking Forge… %d files", n)})
			}
		})
		if err != nil {
			return err
		}
		args = jvmArgs(filepath.Join(dst, "forge.cmd"))
		// Relative paths here are relative to the Forge program folder.
		profile := "# Written by TCG Studio: keep MTG-mode data out of any personal Forge install.\r\n" +
			"userDir=../userdata/\r\ncacheDir=../cache/\r\n"
		return os.WriteFile(filepath.Join(dst, "forge.profile.properties"), []byte(profile), 0o644)
	}); err != nil {
		return fmt.Errorf("unpacking Forge: %w", err)
	}
	for _, d := range []string{"userdata/decks/constructed", "userdata/preferences", "cache"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			return err
		}
	}

	m := ManifestData{ForgeVersion: Version, JavaVersion: jre.Version, Java: filepath.Join("jre", "bin", "javaw.exe"),
		Forge: "forge", Jar: filepath.Join("forge", forgeJar), UserDir: "userdata", JvmArgs: args, Installed: time.Now()}
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(root, Manifest), b, 0o644); err != nil {
		return err
	}
	report(Progress{Stage: "done", Message: "MTG mode installed"})
	return nil
}

// Remove deletes the whole TCGForge folder (Forge, Java, exported decks, picture cache).
func Remove(game string) error {
	if game == "" {
		return fmt.Errorf("game folder not set")
	}
	return os.RemoveAll(Dir(game))
}

type jreInfo struct{ Link, SHA256, Version string }

func latestJRE(ctx context.Context, c *http.Client) (*jreInfo, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", jreAPI, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("adoptium api: %s", resp.Status)
	}
	var assets []struct {
		ReleaseName string `json:"release_name"`
		Binary      struct {
			Package struct {
				Link     string `json:"link"`
				Checksum string `json:"checksum"`
				Name     string `json:"name"`
			} `json:"package"`
		} `json:"binary"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&assets); err != nil {
		return nil, err
	}
	for _, a := range assets {
		p := a.Binary.Package
		if strings.HasSuffix(p.Name, ".zip") && p.Link != "" && p.Checksum != "" {
			return &jreInfo{Link: p.Link, SHA256: p.Checksum, Version: strings.TrimPrefix(a.ReleaseName, "jdk-")}, nil
		}
	}
	return nil, fmt.Errorf("no Windows x64 JRE zip listed")
}

// download streams url to path, checking the SHA-256.
func download(ctx context.Context, c *http.Client, url, path, sha string, progress func(done, total int64)) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 1<<20)
	var done int64
	last := time.Time{}
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return err
			}
			h.Write(buf[:n])
			done += int64(n)
			if time.Since(last) > 250*time.Millisecond {
				progress(done, resp.ContentLength)
				last = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	progress(done, resp.ContentLength)
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, sha) {
		return fmt.Errorf("download corrupted (sha256 %s, expected %s)", got, sha)
	}
	return nil
}

// replaceDir fills dir via fill(tmpDir) and swaps it in only when that succeeded.
func replaceDir(dir string, fill func(tmp string) error) error {
	tmp := dir + ".installing"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	if err := fill(tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("can't replace %s (is Forge running?): %w", dir, err)
	}
	return os.Rename(tmp, dir)
}

// unzipStrip extracts a zip, dropping its single top-level folder (jdk-21…-jre/).
func unzipStrip(zipPath, dst string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		rel := f.Name
		if i := strings.Index(rel, "/"); i >= 0 {
			rel = rel[i+1:]
		}
		if rel == "" || f.FileInfo().IsDir() {
			continue
		}
		out, err := safeJoin(dst, rel)
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = writeFile(out, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func untarBz2(ctx context.Context, path, dst string, include func(rel string) bool, tick func()) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	tr := tar.NewReader(bzip2.NewReader(f))
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		rel := strings.TrimPrefix(h.Name, "./")
		if !include(rel) {
			continue
		}
		out, err := safeJoin(dst, rel)
		if err != nil {
			return err
		}
		if err := writeFile(out, tr); err != nil {
			return err
		}
		tick()
	}
}

func safeJoin(dst, rel string) (string, error) {
	out := filepath.Join(dst, filepath.FromSlash(rel))
	if !strings.HasPrefix(out, filepath.Clean(dst)+string(filepath.Separator)) {
		return "", fmt.Errorf("bad archive path %q", rel)
	}
	return out, nil
}

func writeFile(path string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// jvmArgs reads the JVM options from the release's forge.cmd (the "java … -jar" line), with a fallback.
func jvmArgs(forgeCmd string) []string {
	def := []string{"-Xmx4096m", "-Dio.netty.tryReflectionSetAccessible=true", "-Dfile.encoding=UTF-8"}
	b, err := os.ReadFile(forgeCmd)
	if err != nil {
		return def
	}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || f[0] != "java" {
			continue
		}
		var args []string
		for _, a := range f[1:] {
			if a == "-jar" {
				if len(args) > 0 {
					return args
				}
				break
			}
			args = append(args, a)
		}
	}
	return def
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func mb(d, t int64) string {
	if t <= 0 {
		return fmt.Sprintf("%.0f MB", float64(d)/1e6)
	}
	return fmt.Sprintf("%.0f / %.0f MB", float64(d)/1e6, float64(t)/1e6)
}

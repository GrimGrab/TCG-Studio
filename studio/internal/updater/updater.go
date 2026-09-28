// Package updater checks GitHub Releases for a newer TCG Studio, downloads it (verifying the SHA-256 from the release notes)
// and swaps it in for the running exe. Releases are published by tools\publish.ps1: tag vX.Y.Z, asset "TCG Studio.exe",
// notes ending with a line "sha256: <hex>".
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AssetName is the release asset clients download.
const AssetName = "TCG Studio.exe"

// APIBase can be overridden in tests.
var APIBase = "https://api.github.com"

var client = &http.Client{Timeout: 5 * time.Minute}

type Release struct {
	Version string `json:"version"` // without the leading "v"
	Notes   string `json:"notes"`   // release notes without the sha256 line
	URL     string `json:"url"`     // asset download URL
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	Page    string `json:"page"` // release page on GitHub
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	Draft   bool   `json:"draft"`
	Pre     bool   `json:"prerelease"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

var shaLine = regexp.MustCompile(`(?mi)^\s*sha256:\s*([0-9a-f]{64})\s*$`)

// Latest returns the newest published release of repo ("owner/name").
func Latest(ctx context.Context, repo string) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, APIBase+"/repos/"+repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "TCGStudio-updater")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // no releases yet
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var gr ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, err
	}
	return parse(gr)
}

func parse(gr ghRelease) (*Release, error) {
	r := &Release{Version: strings.TrimPrefix(strings.TrimSpace(gr.TagName), "v"), Page: gr.HTMLURL}
	if m := shaLine.FindStringSubmatch(gr.Body); m != nil {
		r.SHA256 = strings.ToLower(m[1])
	}
	r.Notes = strings.TrimSpace(shaLine.ReplaceAllString(gr.Body, ""))
	for _, a := range gr.Assets {
		if a.Name == AssetName || strings.EqualFold(strings.ReplaceAll(a.Name, ".", " "), strings.ReplaceAll(AssetName, ".", " ")) {
			r.URL, r.Size = a.URL, a.Size
		}
	}
	if r.URL == "" {
		return nil, fmt.Errorf("release %s has no %q asset", gr.TagName, AssetName)
	}
	return r, nil
}

// Newer reports whether version a (e.g. "0.3.1") is newer than b. Non-numeric parts compare as 0; "dev" is never newer.
func Newer(a, b string) bool {
	pa, pb := parts(a), parts(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parts(v string) [3]int {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	for i, s := range strings.SplitN(v, ".", 3) {
		n, _ := strconv.Atoi(s)
		out[i] = n
	}
	return out
}

// Download fetches the release asset to a temp file and verifies its size and SHA-256.
func Download(ctx context.Context, r *Release, progress func(done, total int64)) (string, error) {
	if r.SHA256 == "" {
		return "", errors.New("the release has no checksum — refusing to install it")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "TCGStudio-updater")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}
	dst := filepath.Join(os.TempDir(), "TCGStudio-update-"+r.Version+".exe")
	f, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	var done int64
	buf := make([]byte, 256*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				f.Close()
				return "", err
			}
			h.Write(buf[:n])
			done += int64(n)
			if progress != nil {
				progress(done, r.Size)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			return "", rerr
		}
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if r.Size > 0 && done != r.Size {
		os.Remove(dst)
		return "", fmt.Errorf("download incomplete (%d of %d bytes)", done, r.Size)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != r.SHA256 {
		os.Remove(dst)
		return "", fmt.Errorf("checksum mismatch — the download is corrupt or was tampered with")
	}
	return dst, nil
}

// Swap replaces exe with newExe: the running exe is renamed to <exe>.old (Windows allows renaming a running exe), the new one
// is moved into place; on failure the original is restored.
func Swap(exe, newExe string) error {
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("can't replace the running app (is it in a protected folder?): %w", err)
	}
	if err := moveFile(newExe, exe); err != nil {
		_ = os.Rename(old, exe)
		return fmt.Errorf("installing the update failed: %w", err)
	}
	return nil
}

// moveFile renames, falling back to copy+delete across drives (temp dir may be on another volume).
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		in.Close()
		return err
	}
	_, err = io.Copy(out, in)
	in.Close()
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(dst)
		return err
	}
	os.Remove(src)
	return nil
}

// Restart launches exe with --updated=<version>; the caller then quits.
func Restart(exe, version string) error {
	cmd := exec.Command(exe, "--updated="+version)
	cmd.Dir = filepath.Dir(exe)
	return cmd.Start()
}

// Cleanup removes the previous exe left behind by Swap (call at startup).
func Cleanup(exe string) { _ = os.Remove(exe + ".old") }

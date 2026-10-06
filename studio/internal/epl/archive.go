package epl

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// .rar and .7z mods are read with Windows' own tar.exe (libarchive, part of Windows 10/11): no extra library ships for it.
// Only the descriptor/bundle pairs are extracted, like a zip's.

// archiveExts are the archive types handed to tar.exe (zips use archive/zip).
var archiveExts = map[string]bool{".rar": true, ".7z": true}

func isTarArchive(p string) bool { return archiveExts[strings.ToLower(filepath.Ext(p))] }

func tarExe() (string, error) {
	exe := filepath.Join(os.Getenv("SystemRoot"), "System32", "tar.exe")
	if _, err := os.Stat(exe); err != nil {
		return "", fmt.Errorf("this Windows has no built-in tar.exe to open .rar/.7z files — extract the archive and choose the folder instead")
	}
	return exe, nil
}

func runTar(args ...string) ([]byte, error) {
	exe, err := tarExe()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return out, fmt.Errorf("Windows couldn't read the archive (%s) — extract it and choose the folder instead", msg)
	}
	return out, nil
}

// tarLine is a bsdtar verbose listing line: mode links owner group size month day time-or-year name.
var tarLine = regexp.MustCompile(`^\S+\s+\d+\s+\S+\s+\S+\s+(\d+)\s+\S+\s+\d+\s+\S+\s(.*)$`)

// extractArchive extracts every *.json with a bundle beside it, plus that bundle, from a .rar/.7z into dest (progress by
// the bytes written so far). A complete earlier extraction is reused.
func extractArchive(archive, dest string, progress func(done, total int64)) error {
	done := filepath.Join(dest, ".complete")
	if _, err := os.Stat(done); err == nil {
		return nil
	}
	out, err := runTar("-tf", archive)
	if err != nil {
		return err
	}
	names := map[string]string{} // clean name → name as listed (what tar wants back)
	var order []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		raw := strings.TrimRight(sc.Text(), "\r")
		clean := strings.TrimSuffix(strings.ReplaceAll(raw, `\`, "/"), "/")
		if clean == "" {
			continue
		}
		names[clean] = raw
		order = append(order, clean)
	}
	var want []string
	for _, n := range order {
		if !strings.EqualFold(path.Ext(n), ".json") {
			continue
		}
		b := strings.TrimSuffix(n, path.Ext(n))
		raw, ok := names[b]
		if !ok || strings.HasSuffix(raw, "/") || strings.HasSuffix(raw, `\`) {
			continue
		}
		want = append(want, n, b)
	}
	if len(want) == 0 {
		return nil // Open reports "no EPL content"
	}
	sizes := map[string]int64{} // best effort, for progress only
	if v, err := runTar("-tvf", archive); err == nil {
		for _, l := range strings.Split(string(v), "\n") {
			if m := tarLine.FindStringSubmatch(strings.TrimRight(l, "\r")); m != nil {
				n, _ := strconv.ParseInt(m[1], 10, 64)
				sizes[strings.ReplaceAll(m[2], `\`, "/")] = n
			}
		}
	}
	var total int64
	args := []string{"-xf", archive, "-C", dest}
	for _, n := range want {
		if strings.HasPrefix(n, "/") || strings.Contains(n, ":") || strings.Contains("/"+n+"/", "/../") {
			return fmt.Errorf("bad path in archive: %s", n)
		}
		total += sizes[n]
		args = append(args, names[n])
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	stop := make(chan struct{})
	if progress != nil {
		go func() {
			t := time.NewTicker(500 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-stop:
					return
				case <-t.C:
					progress(dirBytes(dest), total)
				}
			}
		}()
	}
	_, err = runTar(args...)
	close(stop)
	if err != nil {
		return err
	}
	return os.WriteFile(done, nil, 0o644)
}

func dirBytes(dir string) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

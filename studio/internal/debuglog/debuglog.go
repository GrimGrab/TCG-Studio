// Package debuglog builds the Debug page's report: the game's BepInEx log of the last session plus a short summary
// (Studio/mod versions, game folder, setup state, what the game really loaded) that players copy and send for support.
package debuglog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Paths inside the game folder.
const (
	logRel    = "BepInEx/LogOutput.log"
	modDLLRel = "BepInEx/plugins/TCGCustomCards/TCGCustomCards.dll"
)

// MaxLog is the most log text kept; longer logs keep their start (versions, plugin loading) and their end (latest errors).
const MaxLog = 3 << 20

const keepHead = 256 << 10

// Note is one finding shown above the log. Level: ok, warn, err.
type Note struct {
	Level string `json:"level"`
	Text  string `json:"text"`
}

// Input is what the app knows besides the log.
type Input struct {
	GameDir       string
	StudioVersion string
	ModVersion    string   // mod version this Studio installs ("" = dev build without payload)
	SetupReady    bool     // Setup screen is all green
	SetupProblems []string // Setup items that aren't green, "Title: message"
	GameRunning   bool
	Home          string // user profile folder, masked in the report ("" = no masking)
}

// Report is the Debug page's content.
type Report struct {
	Path          string `json:"path"`
	Found         bool   `json:"found"`
	When          string `json:"when"`
	Size          int64  `json:"size"`
	Truncated     bool   `json:"truncated"`
	BepInEx       string `json:"bepinex"`
	LoadedVersion string `json:"loadedVersion"` // mod version the game loaded last session ("" = not loaded)
	Errors        int    `json:"errors"`        // [Error / [Fatal lines
	Notes         []Note `json:"notes"`
	Header        string `json:"header"` // plain-text summary put above the log when copying / saving
	Log           string `json:"log"`
}

var (
	loadingRe = regexp.MustCompile(`Loading \[TCG Custom Cards ([^\]]+)\]`)
	bepinexRe = regexp.MustCompile(`BepInEx (\d+\.\d+[^\s]*)`)
)

// Build reads the log of the last game session and works out what it says.
func Build(in Input) Report {
	r := Report{Path: filepath.Join(in.GameDir, filepath.FromSlash(logRel))}
	note := func(level, format string, a ...any) { r.Notes = append(r.Notes, Note{level, fmt.Sprintf(format, a...)}) }

	info, err := os.Stat(r.Path)
	if in.GameDir == "" || err != nil {
		note("err", "No BepInEx log in this game folder: the game hasn't been started with the mod loader from here, or Studio is set to a different game folder than the one Steam starts (Settings → Game folder).")
	} else {
		r.Found = true
		r.When = info.ModTime().Format("2006-01-02 15:04")
		r.Size = info.Size()
		b, _ := os.ReadFile(r.Path)
		text := strings.ReplaceAll(string(b), "\r\n", "\n")
		analyse(&r, text, in, info.ModTime(), note)
		if len(text) > MaxLog {
			r.Truncated = true
			text = text[:keepHead] + fmt.Sprintf("\n\n… %d KB cut from the middle …\n\n", (len(text)-MaxLog)>>10) + text[len(text)-(MaxLog-keepHead):]
		}
		r.Log = mask(text, in.Home)
	}
	r.Header = mask(header(r, in), in.Home)
	return r
}

func analyse(r *Report, text string, in Input, logTime time.Time, note func(level, format string, a ...any)) {
	if m := bepinexRe.FindStringSubmatch(firstLines(text, 5)); m != nil {
		r.BepInEx = m[1]
	}
	loads := loadingRe.FindAllStringSubmatch(text, -1)
	if len(loads) > 0 {
		r.LoadedVersion = strings.TrimSpace(loads[0][1])
	}
	for _, line := range strings.Split(text, "\n") {
		if l := strings.TrimSpace(line); strings.HasPrefix(l, "[Error") || strings.HasPrefix(l, "[Fatal") {
			r.Errors++
		}
	}

	if in.GameRunning {
		note("warn", "The game is running: the log covers the current session so far.")
	}
	stale := false
	if dll, err := os.Stat(filepath.Join(in.GameDir, filepath.FromSlash(modDLLRel))); err == nil && logTime.Before(dll.ModTime()) {
		stale = true
		note("warn", "The game hasn't been started since the mod was installed or updated (%s). Start the game, then Refresh, to see the current mod's log.", dll.ModTime().Format("2006-01-02 15:04"))
	}
	switch {
	case len(loads) == 0:
		note("err", "BepInEx ran, but the TCG Custom Cards mod wasn't loaded in that session.")
	case len(loads) > 1:
		note("err", "The mod was loaded %d times (duplicate copies in BepInEx\\plugins). Open Setup → Install / Repair.", len(loads))
	case in.ModVersion != "" && r.LoadedVersion != in.ModVersion && !stale:
		note("err", "The game loaded mod v%s, but this TCG Studio installs v%s. Open Setup → Install / Repair, then restart the game. If Setup is already green, Studio is set to a different game folder than the one Steam starts.", r.LoadedVersion, in.ModVersion)
	case in.ModVersion != "" && r.LoadedVersion == in.ModVersion:
		note("ok", "The game loaded mod v%s, the version this TCG Studio installs.", r.LoadedVersion)
	}
	if r.Errors > 0 {
		note("warn", "%d error line(s) in the log.", r.Errors)
	}
}

func header(r Report, in Input) string {
	var b strings.Builder
	line := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }
	mod := in.ModVersion
	if mod == "" {
		mod = "unknown (dev build)"
	}
	line("TCG Studio %s (installs mod %s)", in.StudioVersion, mod)
	line("Game folder: %s", in.GameDir)
	if in.SetupReady {
		line("Setup: all green")
	} else {
		line("Setup: needs work")
		for _, p := range in.SetupProblems {
			line("  - %s", p)
		}
	}
	if r.Found {
		line("Log: %s, written %s, %d KB%s", r.Path, r.When, r.Size>>10, map[bool]string{true: " (middle cut)", false: ""}[r.Truncated])
		loaded := r.LoadedVersion
		if loaded == "" {
			loaded = "not loaded"
		}
		line("BepInEx %s · mod loaded by the game: %s · %d error line(s)", orDash(r.BepInEx), loaded, r.Errors)
	} else {
		line("Log: not found (%s)", r.Path)
	}
	for _, n := range r.Notes {
		line("[%s] %s", n.Level, n.Text)
	}
	return b.String()
}

// mask replaces the user's profile folder (Windows user name) with %USERPROFILE% so logs can be posted publicly.
func mask(text, home string) string {
	if home == "" {
		return text
	}
	for _, h := range []string{home, filepath.ToSlash(home)} {
		text = replaceFold(text, h, "%USERPROFILE%")
	}
	return text
}

func replaceFold(s, old, repl string) string {
	if old == "" {
		return s
	}
	lower, lowOld := strings.ToLower(s), strings.ToLower(old)
	if len(lower) != len(s) { // case mapping changed byte lengths (rare non-ASCII): exact match only
		return strings.ReplaceAll(s, old, repl)
	}
	var b strings.Builder
	for {
		i := strings.Index(lower, lowOld)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		b.WriteString(repl)
		s, lower = s[i+len(old):], lower[i+len(old):]
	}
}

func firstLines(s string, n int) string {
	for i, c := 0, 0; i < len(s); i++ {
		if s[i] == '\n' {
			if c++; c == n {
				return s[:i]
			}
		}
	}
	return s
}

func orDash(s string) string {
	if s == "" {
		return "?"
	}
	return s
}

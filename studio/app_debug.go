package main

import (
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"tcgstudio/internal/debuglog"
	"tcgstudio/internal/installer"
)

// DebugReport returns the game's BepInEx log of the last session with a summary for the Debug page (copy / save for support).
func (a *App) DebugReport() debuglog.Report {
	p, _ := installer.Embedded()
	st := a.SetupState()
	in := debuglog.Input{
		GameDir:       a.settings.GameDir,
		StudioVersion: Version,
		ModVersion:    p.Version,
		SetupReady:    st.Ready,
		GameRunning:   st.Running,
	}
	for _, it := range st.Items {
		if it.State != installer.OK && it.State != installer.Info {
			in.SetupProblems = append(in.SetupProblems, it.Title+": "+it.Message)
		}
	}
	in.Home, _ = os.UserHomeDir()
	return debuglog.Build(in)
}

// SaveDebugReport writes the summary + log to a text file the player picks (to attach in Discord). Returns the path, "" if cancelled.
func (a *App) SaveDebugReport() (string, error) {
	r := a.DebugReport()
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save debug log",
		DefaultFilename: "TCG Studio debug log " + time.Now().Format("2006-01-02 1504") + ".txt",
		Filters:         []runtime.FileFilter{{DisplayName: "Text file (*.txt)", Pattern: "*.txt"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	text := r.Header + "\n----- BepInEx\\LogOutput.log -----\n" + r.Log
	return path, os.WriteFile(path, []byte(strings.ReplaceAll(text, "\n", "\r\n")), 0o644)
}

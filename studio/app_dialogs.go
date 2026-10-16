package main

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Native dialogs and Explorer. Bound methods call these rather than the Wails runtime directly.

var imageFilters = []runtime.FileFilter{{DisplayName: "Images (*.png;*.jpg;*.jpeg)", Pattern: "*.png;*.jpg;*.jpeg"}}

// pickImageFile is the one image file dialog every "choose an image" button uses. Returns "" when cancelled.
func (a *App) pickImageFile(title string) (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: title, Filters: imageFilters})
}

// saveFileAs asks where to save a file of type ext (".png"); the name the user typed gets ext when it has none.
// Returns "" when cancelled.
func (a *App) saveFileAs(title, defaultName, ext string, filters []runtime.FileFilter) (string, error) {
	file, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: title, Filters: filters, DefaultFilename: defaultName})
	if err != nil || file == "" {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(file), ext) {
		file += ext
	}
	return file, nil
}

// reveal opens a folder in Explorer (nothing for "").
func reveal(path string) {
	if path != "" {
		_ = exec.Command("explorer", path).Start()
	}
}

// Confirm shows a native Yes/No dialog titled "TCG Studio" (the browser confirm() is titled "wails.localhost says").
func (a *App) Confirm(message string) bool {
	res, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type: runtime.QuestionDialog, Title: "TCG Studio", Message: message, DefaultButton: "No",
	})
	return err == nil && res == "Yes"
}

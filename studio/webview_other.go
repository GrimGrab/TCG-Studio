//go:build !windows

package main

// clearWebViewExperiments: only Windows' WebView2 keeps an experiment seed (webview_windows.go).
func clearWebViewExperiments() {}

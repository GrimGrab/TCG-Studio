//go:build !windows

package main

type installedFont struct{ name, path string }

// installedFonts: only Windows registers fonts where Studio looks (see fonts_windows.go).
func installedFonts() []installedFont { return nil }

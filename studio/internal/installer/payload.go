// Package installer sets up everything the mod needs in a game folder — BepInEx 5, ConfigurationManager and the
// TCG Custom Cards plugin — from any starting state (clean, partial, outdated, other loaders), backing up what it replaces.
package installer

import (
	"embed"
	"errors"
	"io/fs"
	"strings"
)

// payload/ is filled by tools\package.ps1 (bepinex.zip, TCGCustomCards.dll, tcgcc_foil, version.txt). Only README.txt is
// committed, so a plain dev build has no payload and the Setup screen says so.
//
//go:embed payload
var embedded embed.FS

// Payload is everything the installer writes. Tests build their own.
type Payload struct {
	BepInExZip []byte // BepInEx 5.4.23.5 x64 + ConfigurationManager, laid out like the game folder
	ModDLL     []byte // TCGCustomCards.dll
	Bundle     []byte // tcgcc_foil shader bundle (optional)
	Bridge     []byte // tcgcc-forge-bridge.jar, MTG mode's headless Forge bridge (optional)
	Version    string
}

var ErrNoPayload = errors.New("this TCG Studio build doesn't include the installer files — build it with tools\\package.ps1")

// Embedded returns the payload compiled into this build.
func Embedded() (Payload, error) {
	var p Payload
	var err error
	if p.BepInExZip, err = fs.ReadFile(embedded, "payload/bepinex.zip"); err != nil {
		return p, ErrNoPayload
	}
	if p.ModDLL, err = fs.ReadFile(embedded, "payload/TCGCustomCards.dll"); err != nil {
		return p, ErrNoPayload
	}
	p.Bundle, _ = fs.ReadFile(embedded, "payload/tcgcc_foil")
	p.Bridge, _ = fs.ReadFile(embedded, "payload/tcgcc-forge-bridge.jar")
	if v, err := fs.ReadFile(embedded, "payload/version.txt"); err == nil {
		p.Version = strings.TrimSpace(string(v))
	}
	return p, nil
}

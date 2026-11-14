package cardshader

import (
	"embed"
	"errors"
)

// template/tcgcc_cardshader: the template bundle (see template/README.txt). Read at run time so that a checkout without
// the built bundle still compiles; Studio then keeps shaders but cannot make them for the game.
//
//go:embed template
var templateFiles embed.FS

// ErrNoTemplate: this build of Studio has no template bundle.
var ErrNoTemplate = errors.New("this build of TCG Studio has no card shader template, so shaders cannot be made for the game")

// Template returns the template bundle.
func Template() ([]byte, error) {
	b, err := templateFiles.ReadFile("template/tcgcc_cardshader")
	if err != nil {
		return nil, ErrNoTemplate
	}
	return b, nil
}

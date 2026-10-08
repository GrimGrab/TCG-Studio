// Package fontname reads the display name of a TrueType/OpenType font file (its 'name' table).
package fontname

import (
	"encoding/binary"
	"errors"
	"os"
	"strings"
	"unicode/utf16"
)

// Exts are the font files the shop sign accepts (the mod loads them through Unity's FreeType).
var Exts = []string{".ttf", ".otf"}

// IsFont reports whether path has one of Exts.
func IsFont(path string) bool {
	p := strings.ToLower(path)
	for _, e := range Exts {
		if strings.HasSuffix(p, e) {
			return true
		}
	}
	return false
}

// Check returns an error when b isn't a single TrueType/OpenType font.
func Check(b []byte) error {
	if len(b) < 12 {
		return errors.New("not a font file")
	}
	switch string(b[:4]) {
	case "\x00\x01\x00\x00", "OTTO", "true":
		return nil
	case "ttcf":
		return errors.New("font collections (.ttc) aren't supported — pick a single .ttf or .otf")
	}
	return errors.New("not a TrueType/OpenType font")
}

// File reads the full name of a font file ("" when it has none).
func File(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return Of(b)
}

// Of returns the font's full name (name ID 4), else its family (ID 1); Windows (UTF-16) records first.
func Of(b []byte) (name string) {
	defer func() {
		if recover() != nil {
			name = ""
		}
	}()
	if Check(b) != nil {
		return ""
	}
	be := binary.BigEndian
	tables := int(be.Uint16(b[4:]))
	for i := range tables {
		rec := b[12+16*i:]
		if string(rec[:4]) != "name" {
			continue
		}
		t := b[be.Uint32(rec[8:]):]
		count, strs := int(be.Uint16(t[2:])), int(be.Uint16(t[4:]))
		best, bestRank := "", 99
		for j := range count {
			r := t[6+12*j:]
			platform, nameID := be.Uint16(r), be.Uint16(r[6:])
			length, off := int(be.Uint16(r[8:])), int(be.Uint16(r[10:]))
			if nameID != 4 && nameID != 1 {
				continue
			}
			raw := t[strs+off : strs+off+length]
			var s string
			switch platform {
			case 0, 3:
				u := make([]uint16, len(raw)/2)
				for k := range u {
					u[k] = be.Uint16(raw[2*k:])
				}
				s = string(utf16.Decode(u))
			case 1:
				s = string(raw) // Mac Roman; names are ASCII in practice
			default:
				continue
			}
			rank := 0
			if nameID == 1 {
				rank += 2
			}
			if platform == 1 {
				rank++
			}
			if s = strings.TrimSpace(s); s != "" && rank < bestRank {
				best, bestRank = s, rank
			}
		}
		return best
	}
	return ""
}

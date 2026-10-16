// Package ids makes the ids of content made in Studio (sets, accessories, furniture, decorations): random, so ids made
// by different players (or in different setups) don't collide, and never derived from the name, which can change. Show
// the name wherever a person reads it. Existing ids are never changed: game saves key everything by id.
package ids

import (
	"crypto/rand"
	"strings"
)

const (
	letters = "abcdefghijklmnopqrstuvwxyz"
	base32  = "abcdefghijklmnopqrstuvwxyz234567"
	// Length of an id: a letter, then 11 base-32 characters (about 59 bits).
	Length = 12
)

// New returns a fresh random id such as "kq3x9amz2c7e" (lowercase letters and digits 2–7, starting with a letter).
func New() string {
	b := make([]byte, Length)
	if _, err := rand.Read(b); err != nil {
		panic("ids: no randomness: " + err.Error()) // crypto/rand doesn't fail on supported platforms
	}
	var s strings.Builder
	s.WriteByte(letters[int(b[0])%len(letters)])
	for _, c := range b[1:] {
		s.WriteByte(base32[c&31])
	}
	return s.String()
}

// Free returns New() ids until taken reports one as free.
func Free(taken func(id string) bool) string {
	for {
		if id := New(); !taken(id) {
			return id
		}
	}
}

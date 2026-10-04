package game

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf16"
)

// libraryCapability is the text the mod carries (SetDef.LibraryCapability) when it reads card art from <plugin>\Library.
const libraryCapability = "tcgcc-capability:shared-card-art-library"

var capCache struct {
	sync.Mutex
	path string
	mod  time.Time
	size int64
	ok   bool
}

// ModHasLibrary reports whether the installed mod reads the shared card-art library (<plugin>\Library). An older mod only
// looks in each set's folder, so Studio then copies card art there as before. The answer is cached per DLL file version.
func ModHasLibrary(gameDir string) bool {
	dll := filepath.Join(PluginDir(gameDir), "TCGCustomCards.dll")
	info, err := os.Stat(dll)
	if err != nil {
		return false
	}
	capCache.Lock()
	defer capCache.Unlock()
	if capCache.path == dll && capCache.mod.Equal(info.ModTime()) && capCache.size == info.Size() {
		return capCache.ok
	}
	b, err := os.ReadFile(dll)
	if err != nil {
		return false
	}
	u := utf16.Encode([]rune(libraryCapability)) // .NET string literals are UTF-16
	marker := make([]byte, 0, len(u)*2)
	for _, c := range u {
		marker = append(marker, byte(c), byte(c>>8))
	}
	ok := bytes.Contains(b, marker)
	capCache.path, capCache.mod, capCache.size, capCache.ok = dll, info.ModTime(), info.Size(), ok
	return ok
}

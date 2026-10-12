package core

import (
	"path/filepath"
	"runtime"
	"strings"
)

// SameDirectory reports whether a and b refer to the same
// filesystem directory. Both paths are cleaned and resolved to
// absolute form before comparison. On Windows the comparison is
// case-insensitive (strings.EqualFold) because Windows paths are
// case-insensitive; on other platforms the comparison is exact.
func SameDirectory(a, b string) bool {
	aClean := filepath.Clean(a)
	bClean := filepath.Clean(b)

	aAbs, err := filepath.Abs(aClean)
	if err != nil {
		return false
	}
	bAbs, err := filepath.Abs(bClean)
	if err != nil {
		return false
	}

	if runtime.GOOS == "windows" {
		return strings.EqualFold(aAbs, bAbs)
	}
	return aAbs == bAbs
}

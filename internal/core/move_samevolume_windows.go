//go:build windows

package core

import "path/filepath"

// sameVolume reports whether paths a and b reside on the same Windows
// volume by comparing their volume name prefixes.
func sameVolume(a, b string) (bool, error) {
	return filepath.VolumeName(a) == filepath.VolumeName(b), nil
}

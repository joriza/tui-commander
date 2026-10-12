//go:build !windows

package core

import "os"

// removePath deletes a file or directory tree. On non-Windows platforms
// the standard library handles permissions correctly.
func removePath(path string) error {
	return os.RemoveAll(path)
}

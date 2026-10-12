//go:build windows

package core

import "os"

// removePath deletes a file or directory tree. On Windows, readonly
// files must have their attribute cleared before removal.
func removePath(path string) error {
	// Clear readonly attribute so the file/directory can be removed.
	os.Chmod(path, 0666)
	return os.RemoveAll(path)
}

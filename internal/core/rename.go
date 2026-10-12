package core

import (
	"errors"
	"os"
	"path/filepath"
)

// ---------------------------------------------------------------------------
// RenamePath
// ---------------------------------------------------------------------------

// ErrDestExists is returned by RenamePath when the destination name
// already exists in the same directory.
var ErrDestExistsRename = errors.New("destination already exists")

// ErrRelativePath is returned by RenamePath when oldPath is not
// absolute.
var ErrRelativePath = errors.New("oldPath must be absolute")

// RenamePath renames a file or directory within the same directory.
// newName is validated with ValidateFileName. The rename is
// restricted to the same directory as oldPath. Returns an error if
// oldPath is relative, if newName is invalid, or if the destination
// already exists.
func RenamePath(oldPath, newName string) error {
	if err := ValidateFileName(newName); err != nil {
		return err
	}

	if !filepath.IsAbs(oldPath) {
		return ErrRelativePath
	}

	dir := filepath.Dir(oldPath)
	dest := filepath.Join(dir, newName)

	if _, err := os.Stat(dest); err == nil {
		return ErrDestExistsRename
	}

	return os.Rename(oldPath, dest)
}

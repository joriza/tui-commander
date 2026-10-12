package core

import (
	"errors"
	"os"
	"path/filepath"
)

// ---------------------------------------------------------------------------
// MakeDir
// ---------------------------------------------------------------------------

// ErrDirExists is returned by MakeDir when the target directory
// already exists.
var ErrDirExists = errors.New("directory already exists")

// ErrParentMissing is returned by MakeDir when the parent directory
// does not exist.
var ErrParentMissing = errors.New("parent directory does not exist")

// MakeDir creates a single new directory under parent with the given
// name. name is validated with ValidateFileName. Mkdir is used
// (not MkdirAll), so the parent must already exist. Returns an error
// if the name is invalid, the parent is missing, or the directory
// already exists.
func MakeDir(parent, name string) error {
	if err := ValidateFileName(name); err != nil {
		return err
	}

	parentInfo, err := os.Stat(parent)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrParentMissing
		}
		return err
	}
	if !parentInfo.IsDir() {
		return errors.New("parent is not a directory")
	}

	dest := filepath.Join(parent, name)

	if _, err := os.Stat(dest); err == nil {
		return ErrDirExists
	}

	return os.Mkdir(dest, 0755)
}

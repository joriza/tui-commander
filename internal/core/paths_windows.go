//go:build windows

package core

import (
	"errors"
	"path/filepath"
	"strings"
)

// ExtendLongPath on Windows prefixes absolute paths longer than 260 chars
// with `\\?\` (or `\\?\UNC\` for UNC paths). Already-extended paths pass
// through unchanged. Relative paths return an error.
func ExtendLongPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}
	// Already extended: pass through unchanged
	if strings.HasPrefix(path, `\\?\`) {
		return path, nil
	}
	// Relative path: error
	if !filepath.IsAbs(path) {
		return "", &PathError{Op: "ExtendLongPath", Path: path, Err: errors.New("relative path not supported")}
	}
	// UNC path: \\server\share\...
	if strings.HasPrefix(path, `\\`) {
		// Strip leading backslashes and re-prefix with \\?\UNC\
		trimmed := strings.TrimLeft(path, `\`)
		result := `\\?\UNC\` + trimmed
		return result, nil
	}
	// Absolute path: C:\...
	if len(path) > 260 {
		result := `\\?\` + path
		return result, nil
	}
	return path, nil
}

// PathError wraps an error with the path that caused it.
type PathError struct {
	Op   string
	Path string
	Err  error
}

func (e *PathError) Error() string {
	return e.Op + ": " + e.Path + ": " + e.Err.Error()
}

func (e *PathError) Unwrap() error { return e.Err }

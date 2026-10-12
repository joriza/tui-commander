//go:build !windows

package core

// ExtendLongPath is a no-op on non-Windows platforms.
// It returns the input path unchanged.
func ExtendLongPath(path string) (string, error) {
	return path, nil
}

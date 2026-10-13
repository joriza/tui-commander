//go:build !windows

package core

import (
	"errors"
	"os"
)

// removePath deletes a file or directory tree. On non-Windows platforms
// the standard library handles permissions correctly.
func removePath(path string) error {
	return os.RemoveAll(path)
}

// newVolumeProber returns a prober that reports VolumeUnknown for every path,
// so the default policy deletes permanently. Native trash on Linux is
// deferred to a later change.
func newVolumeProber() VolumeProber {
	return func(string) VolumeKind { return VolumeUnknown }
}

// trashPath is a stub on non-Windows platforms; it is never reached by the
// default policy because newVolumeProber always reports VolumeUnknown.
func trashPath(string) error {
	return errors.New("recycle bin is not supported on this platform")
}

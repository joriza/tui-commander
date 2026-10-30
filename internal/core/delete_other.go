//go:build !windows

package core

import (
	"errors"
	"os"
)

// removePath deletes a file or directory tree. On non-Windows platforms
// the standard library handles permissions correctly.
//
// A symlink or junction is removed AS A LINK: the traversal policy is to
// never follow a link. Removing the link does not touch the target.
// os.RemoveAll would follow a symlink and delete the target tree, so
// we use Lstat and remove only the link when it is one.
func removePath(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(path)
	}
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

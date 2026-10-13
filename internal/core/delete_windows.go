//go:build windows

package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// removePath deletes a file or directory tree. On Windows, readonly
// files must have their attribute cleared before removal.
func removePath(path string) error {
	// Clear readonly attribute so the file/directory can be removed.
	os.Chmod(path, 0666)
	return os.RemoveAll(path)
}

// newVolumeProber returns a prober backed by GetDriveType. A drive root needs
// a trailing backslash (e.g. "C:\"); remote, CD-ROM, RAM-disk and unknown
// volume types all map to VolumeUnknown so the default policy deletes them
// permanently.
func newVolumeProber() VolumeProber {
	return func(path string) VolumeKind {
		root := filepath.VolumeName(path)
		if root == "" {
			root = path
		}
		// A bare drive letter ("C:") must be turned into a root ("C:\").
		if len(root) == 2 && root[1] == ':' {
			root += `\`
		}

		ptr, err := windows.UTF16PtrFromString(root)
		if err != nil {
			return VolumeUnknown
		}

		switch windows.GetDriveType(ptr) {
		case windows.DRIVE_FIXED:
			return VolumeFixed
		case windows.DRIVE_REMOVABLE:
			return VolumeRemovable
		default:
			return VolumeUnknown
		}
	}
}

// ---------------------------------------------------------------------------
// Recycle bin (SHFileOperationW)
// ---------------------------------------------------------------------------

var (
	shell32              = windows.NewLazySystemDLL("shell32.dll")
	procSHFileOperationW = shell32.NewProc("SHFileOperationW")
)

// shFileOpStruct mirrors the Win32 SHFILEOPSTRUCTW layout. x/sys/windows does
// not wrap SHFileOperationW, so the struct and the proc are declared here.
type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

const (
	foDelete          = 0x0003
	fofSilent         = 0x0004
	fofNoConfirmation = 0x0010
	fofAllowUndo      = 0x0040
	fofNoConfirmMkDir = 0x0200
)

// trashPath moves a path to the Windows recycle bin via SHFileOperationW.
// The pFrom field must be a double-NUL-terminated list; UTF16FromString
// already appends one NUL, so a second one is appended here.
func trashPath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	from, err := windows.UTF16FromString(abs)
	if err != nil {
		return err
	}
	from = append(from, 0)

	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  &from[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent | fofNoConfirmMkDir,
	}

	ret, _, _ := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("SHFileOperationW failed: %d", ret)
	}
	if op.fAnyOperationsAborted != 0 {
		return errors.New("recycle bin operation was aborted")
	}
	return nil
}

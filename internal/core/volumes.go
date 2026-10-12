package core

import (
	"os"
	"runtime"
)

// Volume represents a mounted drive or filesystem root a panel can load.
type Volume struct {
	Name string // short display label, e.g. `C:\` or `/`
	Path string // path to load into the panel
}

// Volumes returns the available filesystem roots, ordered by drive letter.
// On Windows: every existing drive letter `A:\`..`Z:\` (a root that cannot
// be stat-ed, e.g. an empty card reader, is skipped). On other platforms:
// the single root `/`.
//
// The drive letter itself is used as the display label for v0. Reading real
// volume labels would require syscall/Windows APIs, which are deliberately
// avoided here.
func Volumes() []Volume {
	if runtime.GOOS != "windows" {
		return []Volume{{Name: "/", Path: "/"}}
	}

	var volumes []Volume
	for c := 'A'; c <= 'Z'; c++ {
		root := string(c) + ":\\"
		if _, err := os.Stat(root); err != nil {
			continue
		}
		volumes = append(volumes, Volume{Name: root, Path: root})
	}
	return volumes
}

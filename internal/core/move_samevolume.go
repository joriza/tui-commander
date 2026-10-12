//go:build !windows

package core

import (
	"os"
	"syscall"
)

// sameVolume reports whether paths a and b reside on the same filesystem
// volume by comparing their device numbers.
func sameVolume(a, b string) (bool, error) {
	aInfo, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	bInfo, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	aStat := aInfo.Sys().(*syscall.Stat_t)
	bStat := bInfo.Sys().(*syscall.Stat_t)
	return aStat.Dev == bStat.Dev, nil
}

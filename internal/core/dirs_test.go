package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ---------------------------------------------------------------------------
// TestSameDirectory_SamePath
// ---------------------------------------------------------------------------

func TestSameDirectory_SamePath(t *testing.T) {
	dir := t.TempDir()
	if !SameDirectory(dir, dir) {
		t.Errorf("SameDirectory(%q, %q) = false, want true", dir, dir)
	}
}

// ---------------------------------------------------------------------------
// TestSameDirectory_TempDirs
// ---------------------------------------------------------------------------

func TestSameDirectory_TempDirs(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	if SameDirectory(a, b) {
		t.Errorf("SameDirectory(%q, %q) = true, want false (different temp dirs)", a, b)
	}
}

// ---------------------------------------------------------------------------
// TestSameDirectory_RelativeAndAbsolute
// ---------------------------------------------------------------------------

func TestSameDirectory_RelativeAndAbsolute(t *testing.T) {
	abs, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Use a relative path that resolves to the same absolute directory.
	rel := "."
	if !SameDirectory(abs, rel) {
		t.Errorf("SameDirectory(%q, %q) = false, want true (relative vs absolute)", abs, rel)
	}
}

// ---------------------------------------------------------------------------
// TestSameDirectory_DifferentCase
// ---------------------------------------------------------------------------

func TestSameDirectory_DifferentCase(t *testing.T) {
	dir := t.TempDir()

	// On Windows, paths are case-insensitive; on other platforms they are
	// case-sensitive. We test both semantics by constructing a same-path
	// variant with different casing where possible.
	parent := filepath.Dir(dir)
	base := filepath.Base(dir)

	var mixedCase string
	for i, r := range base {
		if i == 0 {
			// Flip the case of the first rune to create a different-case path.
			if r >= 'a' && r <= 'z' {
				mixedCase += string(r - 'a' + 'A')
			} else if r >= 'A' && r <= 'Z' {
				mixedCase += string(r - 'A' + 'a')
			} else {
				mixedCase += string(r)
			}
		} else {
			mixedCase += string(r)
		}
	}
	mixedPath := filepath.Join(parent, mixedCase)

	// On Windows (GOOS=windows), SameDirectory should return true for
	// different-case paths pointing to the same directory.
	// On other platforms, it should return false because the paths differ.
	if runtime.GOOS == "windows" {
		if !SameDirectory(dir, mixedPath) {
			t.Errorf("SameDirectory(%q, %q) = false, want true on Windows", dir, mixedPath)
		}
	} else {
		if SameDirectory(dir, mixedPath) {
			t.Errorf("SameDirectory(%q, %q) = true, want false on non-Windows", dir, mixedPath)
		}
	}
}

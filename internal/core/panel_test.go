package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mkDir creates a temp directory and populates it with a known structure
// for testing. It returns the temp dir path and a cleanup function.
func mkDir(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "panel-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}

	// Create subdirectories
	for _, d := range []string{"alpha", "Beta", "gamma"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatalf("MkdirAll %q: %v", d, err)
		}
	}

	// Create files with known sizes and mod times
	files := map[string]struct {
		content string
		modTime time.Time
	}{
		"a-file.txt":  {"hello", time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)},
		"b-file.txt":  {"world!", time.Date(2025, 2, 20, 14, 0, 0, 0, time.UTC)},
		"c-file.log":  {"log entry", time.Date(2025, 3, 10, 8, 0, 0, 0, time.UTC)},
	}
	for name, info := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(info.content), 0644); err != nil {
			t.Fatalf("WriteFile %q: %v", name, err)
		}
		if err := os.Chtimes(p, info.modTime, info.modTime); err != nil {
			t.Fatalf("Chtimes %q: %v", name, err)
		}
	}

	return dir, func() { os.RemoveAll(dir) }
}

// ---------------------------------------------------------------------------
// Load
// ---------------------------------------------------------------------------

func TestLoad_RealTempDir(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load(%q) unexpected error: %v", dir, err)
	}

	entries := p.Entries()
	if len(entries) == 0 {
		t.Fatal("expected entries after Load, got none")
	}
}

func TestLoad_NonexistentPath(t *testing.T) {
	p := NewPanel()
	err := p.Load("/nonexistent/path/that/does/not/exist/12345")
	if err == nil {
		t.Error("expected error for nonexistent path, got nil")
	}
}

// ---------------------------------------------------------------------------
// Ordering: dirs first, case-insensitive name
// ---------------------------------------------------------------------------

func TestLoad_OrderingDirsFirstThenNameAsc(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load(%q) unexpected error: %v", dir, err)
	}

	entries := p.Entries()

	// Find indices of directories and files
	var dirNames, fileNames []string
	for _, e := range entries {
		if e.IsDir {
			dirNames = append(dirNames, e.Name)
		} else {
			fileNames = append(fileNames, e.Name)
		}
	}

	// All dirs should come before all files
	if len(dirNames) > 0 && len(fileNames) > 0 {
		lastDirIdx := -1
		firstFileIdx := len(entries)
		for i, e := range entries {
			if e.IsDir {
				lastDirIdx = i
			} else if firstFileIdx > i {
				firstFileIdx = i
			}
		}
		if lastDirIdx >= firstFileIdx {
			t.Errorf("directories should come before files: last dir index %d >= first file index %d", lastDirIdx, firstFileIdx)
		}
	}

	// Dir names should be case-insensitive ascending
	for i := 1; i < len(dirNames); i++ {
		prev := dirNames[i-1]
		curr := dirNames[i]
		if lower(prev) > lower(curr) {
			t.Errorf("dirs not sorted case-insensitive ascending: %q before %q", prev, curr)
		}
	}

	// File names should be case-insensitive ascending
	for i := 1; i < len(fileNames); i++ {
		prev := fileNames[i-1]
		curr := fileNames[i]
		if lower(prev) > lower(curr) {
			t.Errorf("files not sorted case-insensitive ascending: %q before %q", prev, curr)
		}
	}
}

func lower(s string) string {
	// simple case-insensitive comparison using lowercase
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// ---------------------------------------------------------------------------
// Cursor bounds
// ---------------------------------------------------------------------------

func TestCursor_InitialZero(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if p.Cursor() != 0 {
		t.Errorf("initial cursor expected 0, got %d", p.Cursor())
	}
}

func TestCursor_SetWithinBounds(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	n := len(p.Entries())
	if n < 2 {
		t.Skip("need at least 2 entries for this test")
	}

	p.SetCursor(n - 1)
	if p.Cursor() != n-1 {
		t.Errorf("cursor expected %d, got %d", n-1, p.Cursor())
	}
}

func TestCursor_SetNegativeClamped(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	p.SetCursor(-5)
	if p.Cursor() != 0 {
		t.Errorf("cursor expected 0 after negative SetCursor, got %d", p.Cursor())
	}
}

func TestCursor_SetBeyondClamped(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	n := len(p.Entries())
	p.SetCursor(n + 100)
	if p.Cursor() != n-1 {
		t.Errorf("cursor expected %d (last index) after beyond-bounds SetCursor, got %d", n-1, p.Cursor())
	}
}

func TestCursor_UpFromZero(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	p.SetCursor(0)
	p.Up()
	if p.Cursor() != 0 {
		t.Errorf("cursor should stay at 0 when Up from top, got %d", p.Cursor())
	}
}

// ---------------------------------------------------------------------------
// Up / Descend
// ---------------------------------------------------------------------------

func TestUp_ReturnsParent(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	parent, err := p.Up()
	if err != nil {
		t.Fatalf("Up() unexpected error: %v", err)
	}

	expectedParent := filepath.Dir(dir)
	if parent != expectedParent {
		t.Errorf("Up() expected parent %q, got %q", expectedParent, parent)
	}
}

func TestUp_DriveRootErrors(t *testing.T) {
	// On Windows, drive root like "C:\" has no parent; on Unix, "/" has no parent.
	// Use the root path which should error on Up.
	p := NewPanel()
	if err := p.Load("/"); err != nil {
		t.Fatalf("Load /: %v", err)
	}

	_, err := p.Up()
	if err == nil {
		t.Error("expected error calling Up() at filesystem root, got nil")
	}
}

func TestDescend_IntoDirectory(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Find a directory entry to descend into
	var dirName string
	for _, e := range p.Entries() {
		if e.IsDir {
			dirName = e.Name
			break
		}
	}
	if dirName == "" {
		t.Skip("no subdirectory found to descend into")
	}

	err := p.Descend(dirName)
	if err != nil {
		t.Fatalf("Descend(%q) unexpected error: %v", dirName, err)
	}

	expectedPath := filepath.Join(dir, dirName)
	if p.currentPath != expectedPath {
		t.Errorf("after Descend, currentPath expected %q, got %q", expectedPath, p.currentPath)
	}
}

func TestDescend_FileError(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Find a file entry to try descending into
	var fileName string
	for _, e := range p.Entries() {
		if !e.IsDir {
			fileName = e.Name
			break
		}
	}
	if fileName == "" {
		t.Skip("no file entry found")
	}

	err := p.Descend(fileName)
	if err == nil {
		t.Errorf("expected error descending into file %q, got nil", fileName)
	}
}

func TestDescend_NonexistentError(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	err := p.Descend("nonexistent-directory-xyz")
	if err == nil {
		t.Error("expected error descending into nonexistent directory, got nil")
	}
}

// ---------------------------------------------------------------------------
// Entries expose correct fields
// ---------------------------------------------------------------------------

func TestEntries_HasRequiredFields(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, e := range p.Entries() {
		if e.Name == "" {
			t.Error("entry Name should not be empty")
		}
		// Size should be non-negative for files
		if !e.IsDir && e.Size < 0 {
			t.Errorf("file entry %q has negative size %d", e.Name, e.Size)
		}
		// ModTime should be zero-value for entries where we can't determine it
		// (we set it for files, so it should be non-zero for our test files)
	}
}

// ---------------------------------------------------------------------------
// Reload on demand
// ---------------------------------------------------------------------------

func TestLoad_ReloadReflectsNewContent(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	initialCount := len(p.Entries())

	// Create a new file in the directory
	newFile := filepath.Join(dir, "new-file.txt")
	if err := os.WriteFile(newFile, []byte("new content"), 0644); err != nil {
		t.Fatalf("WriteFile new file: %v", err)
	}

	// Reload
	if err := p.Load(dir); err != nil {
		t.Fatalf("Reload Load: %v", err)
	}

	newCount := len(p.Entries())
	if newCount <= initialCount {
		t.Errorf("after reload, expected more entries (%d > %d), got %d", newCount, initialCount, newCount)
	}
}

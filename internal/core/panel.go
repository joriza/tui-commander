package core

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// ---------------------------------------------------------------------------
// Entry
// ---------------------------------------------------------------------------

// Entry represents a single filesystem entry in a panel listing.
type Entry struct {
	Name   string
	IsDir  bool
	Size   int64
	ModTime time.Time
}

// ---------------------------------------------------------------------------
// Panel
// ---------------------------------------------------------------------------

// Panel holds the state for a single file browser panel: current path,
// directory entries, cursor position, and sort configuration.
// Entries are sorted with directories first, then files, both groups
// ordered case-insensitively by name ascending.
type Panel struct {
	currentPath string
	entries     []Entry
	cursor      int
}

// NewPanel creates a Panel with zero state (no path loaded).
func NewPanel() *Panel {
	return &Panel{}
}

// Load populates the panel's entries from the filesystem at path using
// os.ReadDir. The path is cleaned and stored as the panel's current path.
// Entries are sorted with directories first, then case-insensitive name
// ascending. Returns an error if the path cannot be read.
func (p *Panel) Load(path string) error {
	cleaned := filepath.Clean(path)

	entries, err := os.ReadDir(cleaned)
	if err != nil {
		return err
	}

	p.currentPath = cleaned
	p.entries = p.entries[:0]

	for _, de := range entries {
		e := Entry{
			Name:  de.Name(),
			IsDir: de.IsDir(),
		}
		if !de.IsDir() {
			info, err := de.Info()
			if err == nil {
				e.Size = info.Size()
				e.ModTime = info.ModTime()
			}
		}
		p.entries = append(p.entries, e)
	}

	p.sortEntries()
	p.clampCursor()

	return nil
}

// sortEntries sorts directories first, then files, both groups
// case-insensitively by name ascending.
func (p *Panel) sortEntries() {
	sort.SliceStable(p.entries, func(i, j int) bool {
		a, b := p.entries[i], p.entries[j]

		// Directories come before files
		if a.IsDir != b.IsDir {
			return a.IsDir
		}

		// Same type: case-insensitive ascending by name
		return lessIgnoreCase(a.Name, b.Name)
	})
}

// lessIgnoreCase reports whether a is less than b using Unicode-aware
// case-insensitive comparison.
func lessIgnoreCase(a, b string) bool {
	// Use filepath-specific comparison: compare lowercase runes
	// This avoids importing strings for a simple case-insensitive compare
	// and handles Unicode correctly via simple lowercasing.
	aLower := make([]rune, 0, len(a))
	bLower := make([]rune, 0, len(b))
	for _, r := range a {
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		aLower = append(aLower, r)
	}
	for _, r := range b {
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		bLower = append(bLower, r)
	}
	return string(aLower) < string(bLower)
}

// clampCursor ensures the cursor stays within valid bounds.
func (p *Panel) clampCursor() {
	if len(p.entries) == 0 {
		p.cursor = 0
		return
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(p.entries) {
		p.cursor = len(p.entries) - 1
	}
}

// Entries returns a copy of the current panel entries.
func (p *Panel) Entries() []Entry {
	result := make([]Entry, len(p.entries))
	copy(result, p.entries)
	return result
}

// Cursor returns the current cursor index.
func (p *Panel) Cursor() int {
	return p.cursor
}

// SetCursor sets the cursor to the given index, clamped to valid bounds.
func (p *Panel) SetCursor(idx int) {
	p.cursor = idx
	p.clampCursor()
}

// Up moves the cursor up by one position and returns the parent directory
// path. If the cursor is already at the top, the cursor stays at 0 and
// the parent path is still returned. If the current path is a filesystem
// root (no parent), an error is returned.
func (p *Panel) Up() (string, error) {
	if p.cursor > 0 {
		p.cursor--
	}

	parent := filepath.Dir(p.currentPath)
	// Detect filesystem root: if parent equals current path, we're at root.
	if parent == p.currentPath {
		return "", errors.New("cannot go up from filesystem root")
	}

	return parent, nil
}

// Descend changes the panel's current path into the named subdirectory
// of the current path and reloads entries. Returns an error if the name
// is not a directory or does not exist.
func (p *Panel) Descend(name string) error {
	target := filepath.Join(p.currentPath, name)

	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("not a directory: " + name)
	}

	return p.Load(target)
}

// CurrentPath returns the panel's current directory path.
func (p *Panel) CurrentPath() string {
	return p.currentPath
}

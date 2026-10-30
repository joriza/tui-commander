package core

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Entry
// ---------------------------------------------------------------------------

// Entry represents a single filesystem entry in a panel listing.
//
// Link holds the resolved target of a symlink or junction when the entry is
// one, and is empty for ordinary entries. It is filled by Lstat, never by
// following the link: the listing marks a link without descending into it.
// The field exists so a renderer can show the `name -> target` marker and so
// the traversal policy can refuse a link without a second stat call.
type Entry struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
	Link    string
}

// ---------------------------------------------------------------------------
// Panel
// ---------------------------------------------------------------------------

// SortBy selects the primary sort key for a panel listing.
type SortBy int

const (
	SortByName SortBy = iota
	SortBySize
	SortByTime
	SortByExt
)

// String returns the human-readable name of the sort key.
func (s SortBy) String() string {
	switch s {
	case SortByName:
		return "Name"
	case SortBySize:
		return "Size"
	case SortByTime:
		return "Time"
	case SortByExt:
		return "Ext"
	default:
		return "Unknown"
	}
}

// SortConfig holds the per-panel sort configuration.
type SortConfig struct {
	By      SortBy
	DirsFirst bool
	Reverse   bool
}

// DefaultSortConfig returns the v0 default: name ascending, dirs first.
func DefaultSortConfig() SortConfig {
	return SortConfig{By: SortByName, DirsFirst: true, Reverse: false}
}

// Panel holds the state for a single file browser panel: current path,
// directory entries, cursor position, and sort configuration.
// Entries are sorted with directories first, then files, both groups
// ordered case-insensitively by name ascending.
type Panel struct {
	currentPath string
	entries     []Entry
	cursor      int
	// selected tracks explicitly selected entries keyed by entry Name.
	selected map[string]bool

	// filter holds the active quick search string. When empty, visible
	// contains all entries. When non-empty, visible contains entries whose
	// name contains the filter (case-insensitively).
	filter string
	// visible is the list Entries()/cursor/selection operate on. When
	// filter is empty, visible is a copy of entries; otherwise it holds
	// matching entries in the same order. visible must never alias the
	// entries backing array.
	visible []Entry

	// sortConfig holds the per-panel sort configuration.
	sortConfig SortConfig
}

// NewPanel creates a Panel with zero state (no path loaded).
func NewPanel() *Panel {
	return &Panel{
		selected:    make(map[string]bool),
		visible:     nil,
		filter:      "",
		sortConfig:  DefaultSortConfig(),
	}
}

// Load populates the panel's entries from the filesystem at path using
// os.ReadDir. The path is cleaned and stored as the panel's current path.
// Entries are sorted with directories first, then case-insensitive name
// ascending. Returns an error if the path cannot be read.
//
// Load clears any existing selection: changing directory or rereading the
// listing drops the selection, matching Total Commander semantics.
func (p *Panel) Load(path string) error {
	cleaned := filepath.Clean(path)

	entries, err := os.ReadDir(cleaned)
	if err != nil {
		return err
	}

	p.currentPath = cleaned
	p.entries = p.entries[:0]
	p.selected = make(map[string]bool)

	for _, de := range entries {
		e := Entry{
			Name:  de.Name(),
			IsDir: de.IsDir(),
		}
		// de.Info() is an Lstat: it reports the link itself, never its
		// target, so a symlink shows up as a link here and the listing does
		// not descend into it.
		if info, err := de.Info(); err == nil {
			e.ModTime = info.ModTime()
			if !de.IsDir() {
				e.Size = info.Size()
			}
			// A symlink or junction is marked with its target so the listing
			// can render `name -> target`. The target is read with Readlink,
			// which does not follow the link, so a dangling link reports its
			// stored target rather than an error.
			if info.Mode()&os.ModeSymlink != 0 {
				if target, rerr := os.Readlink(filepath.Join(cleaned, de.Name())); rerr == nil {
					e.Link = target
				}
			}
		}
		p.entries = append(p.entries, e)
	}

	p.sortEntries()

	// Clear any active filter (changing directory or rereading clears it).
	p.filter = ""
	p.applyFilter()
	p.clampCursor()

	return nil
}

// sortEntries sorts the panel entries according to the panel's
// sortConfig. When DirsFirst is true, directories are grouped before
// files; the primary key is By (Name, Size, Time, Ext); Reverse
// flips the order.
func (p *Panel) sortEntries() {
	sort.SliceStable(p.entries, func(i, j int) bool {
		a, b := p.entries[i], p.entries[j]

		// Directories-first grouping
		if p.sortConfig.DirsFirst && a.IsDir != b.IsDir {
			if a.IsDir {
				return !p.sortConfig.Reverse
			}
			return p.sortConfig.Reverse
		}

		// Primary sort key
		var less bool
		switch p.sortConfig.By {
		case SortByName:
			less = lessIgnoreCase(a.Name, b.Name)
		case SortBySize:
			if a.Size != b.Size {
				less = a.Size < b.Size
			} else {
				less = lessIgnoreCase(a.Name, b.Name)
			}
		case SortByTime:
			if !a.ModTime.Equal(b.ModTime) {
				less = a.ModTime.Before(b.ModTime)
			} else {
				less = lessIgnoreCase(a.Name, b.Name)
			}
		case SortByExt:
			extA := filepath.Ext(a.Name)
			extB := filepath.Ext(b.Name)
			if extA != extB {
				less = extA < extB
			} else {
				less = lessIgnoreCase(a.Name, b.Name)
			}
		}

		if p.sortConfig.Reverse {
			return !less
		}
		return less
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
	if len(p.visible) == 0 {
		p.cursor = 0
		return
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	if p.cursor >= len(p.visible) {
		p.cursor = len(p.visible) - 1
	}
}

// Entries returns a copy of the current panel entries (visible entries).
func (p *Panel) Entries() []Entry {
	if len(p.visible) == 0 {
		return []Entry{}
	}
	result := make([]Entry, len(p.visible))
	copy(result, p.visible)
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

// SortConfig returns a copy of the panel's sort configuration.
func (p *Panel) SortConfig() SortConfig {
	return p.sortConfig
}

// SetSortBy changes the primary sort key and re-sorts the panel.
func (p *Panel) SetSortBy(by SortBy) {
	p.sortConfig.By = by
	p.sortEntries()
}

// SetDirsFirst toggles the directories-first setting and re-sorts.
func (p *Panel) SetDirsFirst(dirsFirst bool) {
	p.sortConfig.DirsFirst = dirsFirst
	p.sortEntries()
}

// SetReverse toggles the reverse-order setting and re-sorts.
func (p *Panel) SetReverse(reverse bool) {
	p.sortConfig.Reverse = reverse
	p.sortEntries()
}

// ToggleDirsFirst flips the directories-first setting and re-sorts.
func (p *Panel) ToggleDirsFirst() {
	p.sortConfig.DirsFirst = !p.sortConfig.DirsFirst
	p.sortEntries()
}

// ToggleReverse flips the reverse-order setting and re-sorts.
func (p *Panel) ToggleReverse() {
	p.sortConfig.Reverse = !p.sortConfig.Reverse
	p.sortEntries()
}

// ---------------------------------------------------------------------------
// Selection
// ---------------------------------------------------------------------------

// ToggleSelectAtCursor toggles the selection state of the entry at the
// cursor and returns the resulting state (true when the entry is now
// selected). It returns false when the panel has no entries.
func (p *Panel) ToggleSelectAtCursor() bool {
	if len(p.visible) == 0 {
		return false
	}
	name := p.visible[p.cursor].Name
	if p.selected[name] {
		delete(p.selected, name)
		return false
	}
	p.selected[name] = true
	return true
}

// IsSelected reports whether the entry with the given name is selected.
func (p *Panel) IsSelected(name string) bool {
	return p.selected[name]
}

// SelectedNames returns the names of the selected entries in the panel's
// current visible entry order.
func (p *Panel) SelectedNames() []string {
	names := make([]string, 0, len(p.selected))
	for _, e := range p.visible {
		if p.selected[e.Name] {
			names = append(names, e.Name)
		}
	}
	return names
}

// SelectedEntries returns a copy of the selected entries in the panel's
// current visible entry order (same copy semantics as Entries).
func (p *Panel) SelectedEntries() []Entry {
	result := make([]Entry, 0, len(p.selected))
	for _, e := range p.visible {
		if p.selected[e.Name] {
			result = append(result, e)
		}
	}
	return result
}

// SelectionCount returns the number of selected entries that are currently
// visible. While a filter is active only visible selections are counted;
// hidden selections are retained internally and reappear when the filter is
// cleared.
func (p *Panel) SelectionCount() int {
	count := 0
	for _, e := range p.visible {
		if p.selected[e.Name] {
			count++
		}
	}
	return count
}

// ClearSelection removes every selected entry.
func (p *Panel) ClearSelection() {
	p.selected = make(map[string]bool)
}

// SelectMask selects every visible entry whose name matches mask and
// returns the number of entries that this call newly selected. Matching
// uses filepath.Match, case-insensitively (both mask and name are
// lowercased first). A malformed mask returns the filepath.Match error
// and leaves the selection unchanged.
func (p *Panel) SelectMask(mask string) (int, error) {
	names, err := p.matchMask(mask)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, name := range names {
		if !p.selected[name] {
			p.selected[name] = true
			count++
		}
	}
	return count, nil
}

// DeselectMask deselects every visible entry whose name matches mask and
// returns the number of entries that this call newly deselected. Matching
// uses the same case-insensitive filepath.Match rules as SelectMask. A
// malformed mask returns the filepath.Match error and leaves the selection
// unchanged.
func (p *Panel) DeselectMask(mask string) (int, error) {
	names, err := p.matchMask(mask)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, name := range names {
		if p.selected[name] {
			delete(p.selected, name)
			count++
		}
	}
	return count, nil
}

// InvertSelection inverts the selection across all visible entries and
// returns the resulting selection count.
func (p *Panel) InvertSelection() int {
	next := make(map[string]bool, len(p.visible))
	for _, e := range p.visible {
		if !p.selected[e.Name] {
			next[e.Name] = true
		}
	}
	p.selected = next
	return len(p.selected)
}

// matchMask returns the names of the visible entries matching mask,
// matched case-insensitively with filepath.Match. The pattern is validated
// even when the panel has no entries, so a malformed mask always reports
// the filepath.Match error.
func (p *Panel) matchMask(mask string) ([]string, error) {
	lowerMask := strings.ToLower(mask)
	if _, err := filepath.Match(lowerMask, ""); err != nil {
		return nil, err
	}
	var names []string
	for _, e := range p.visible {
		ok, err := filepath.Match(lowerMask, strings.ToLower(e.Name))
		if err != nil {
			return nil, err
		}
		if ok {
			names = append(names, e.Name)
		}
	}
	return names, nil
}

// applyFilter rebuilds the visible slice from entries according to filter.
// When filter is empty, visible is a copy of entries. When non-empty,
// visible holds matching entries in the same order. visible must never alias
// entries' backing array.
func (p *Panel) applyFilter() {
	if p.filter == "" {
		if p.visible == nil {
			p.visible = make([]Entry, len(p.entries))
			copy(p.visible, p.entries)
			return
		}
		p.visible = append(p.visible[:0], p.entries...)
		return
	}
	filterLower := strings.ToLower(p.filter)
	if p.visible == nil {
		p.visible = make([]Entry, 0, len(p.entries))
	} else {
		p.visible = p.visible[:0]
	}
	for _, e := range p.entries {
		if strings.Contains(strings.ToLower(e.Name), filterLower) {
			p.visible = append(p.visible, e)
		}
	}
}

// SetFilter stores the filter, rebuilds visible (case-insensitive substring
// match), and clamps the cursor to valid bounds.
func (p *Panel) SetFilter(filter string) {
	p.filter = filter
	p.applyFilter()
	p.clampCursor()
}

// Filter returns the active filter string.
func (p *Panel) Filter() string {
	return p.filter
}

// ClearFilter clears the filter (equivalent to SetFilter("")).
func (p *Panel) ClearFilter() {
	p.SetFilter("")
}

// FirstMatchIndex returns the full-listing index of the first visible entry.
// It returns -1 when the filter is empty or there are no visible entries.
func (p *Panel) FirstMatchIndex() int {
	if p.filter == "" || len(p.visible) == 0 {
		return -1
	}
	// visible preserves entries' order, so the first visible entry is the
	// first entry in entries that matches the filter.
	filterLower := strings.ToLower(p.filter)
	for i, e := range p.entries {
		if strings.Contains(strings.ToLower(e.Name), filterLower) {
			return i
		}
	}
	return -1
}

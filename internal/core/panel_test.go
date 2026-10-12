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

// ---------------------------------------------------------------------------
// Selection
// ---------------------------------------------------------------------------

func TestToggleSelectAtCursor(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	entries := p.Entries()
	if len(entries) == 0 {
		t.Fatal("expected entries after Load")
	}

	p.SetCursor(0)
	name := entries[0].Name

	if got := p.ToggleSelectAtCursor(); !got {
		t.Errorf("first toggle expected true (now selected), got %v", got)
	}
	if !p.IsSelected(name) {
		t.Errorf("expected %q selected after toggle", name)
	}
	if p.SelectionCount() != 1 {
		t.Errorf("SelectionCount expected 1, got %d", p.SelectionCount())
	}

	if got := p.ToggleSelectAtCursor(); got {
		t.Errorf("second toggle expected false (now deselected), got %v", got)
	}
	if p.IsSelected(name) {
		t.Errorf("expected %q deselected after second toggle", name)
	}
	if p.SelectionCount() != 0 {
		t.Errorf("SelectionCount expected 0, got %d", p.SelectionCount())
	}
}

func TestToggleSelectAtCursor_NoEntries(t *testing.T) {
	p := NewPanel()

	if got := p.ToggleSelectAtCursor(); got {
		t.Errorf("toggle with no entries expected false, got %v", got)
	}
	if p.SelectionCount() != 0 {
		t.Errorf("SelectionCount expected 0, got %d", p.SelectionCount())
	}
}

func TestClearSelection(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	p.SetCursor(0)
	p.ToggleSelectAtCursor()
	if p.SelectionCount() != 1 {
		t.Fatalf("precondition: expected 1 selected, got %d", p.SelectionCount())
	}

	p.ClearSelection()
	if p.SelectionCount() != 0 {
		t.Errorf("SelectionCount expected 0 after ClearSelection, got %d", p.SelectionCount())
	}
	if len(p.SelectedEntries()) != 0 {
		t.Errorf("SelectedEntries expected empty after ClearSelection, got %d", len(p.SelectedEntries()))
	}
}

func TestSelectionOrderFollowsVisibleEntries(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	entries := p.Entries()
	if len(entries) < 3 {
		t.Fatalf("need at least 3 entries, got %d", len(entries))
	}

	// Select last, then first, then second: selection order must still
	// follow the visible entry order, not the selection order.
	p.SetCursor(len(entries) - 1)
	p.ToggleSelectAtCursor()
	p.SetCursor(0)
	p.ToggleSelectAtCursor()
	p.SetCursor(1)
	p.ToggleSelectAtCursor()

	expected := []string{entries[0].Name, entries[1].Name, entries[len(entries)-1].Name}

	names := p.SelectedNames()
	if len(names) != len(expected) {
		t.Fatalf("SelectedNames expected %d names, got %d (%v)", len(expected), len(names), names)
	}
	for i, want := range expected {
		if names[i] != want {
			t.Errorf("SelectedNames[%d] expected %q, got %q", i, want, names[i])
		}
	}

	selEntries := p.SelectedEntries()
	if len(selEntries) != len(expected) {
		t.Fatalf("SelectedEntries expected %d, got %d", len(expected), len(selEntries))
	}
	for i, want := range expected {
		if selEntries[i].Name != want {
			t.Errorf("SelectedEntries[%d] expected %q, got %q", i, want, selEntries[i].Name)
		}
	}
}

func TestSelectedEntries_ReturnsCopy(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	p.SetCursor(0)
	p.ToggleSelectAtCursor()

	first := p.SelectedEntries()
	if len(first) != 1 {
		t.Fatalf("expected 1 selected entry, got %d", len(first))
	}
	first[0].Name = "mutated-by-caller"

	second := p.SelectedEntries()
	if second[0].Name == "mutated-by-caller" {
		t.Error("SelectedEntries must return a copy; mutating the result changed panel state")
	}
}

func TestInvertSelection(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	total := len(p.Entries())
	if total == 0 {
		t.Fatal("expected entries after Load")
	}

	// Start with exactly one selected.
	p.SetCursor(0)
	p.ToggleSelectAtCursor()
	first := p.Entries()[0].Name

	got := p.InvertSelection()
	if want := total - 1; got != want {
		t.Errorf("InvertSelection expected %d, got %d", want, got)
	}
	if p.SelectionCount() != total-1 {
		t.Errorf("SelectionCount expected %d after invert, got %d", total-1, p.SelectionCount())
	}
	if p.IsSelected(first) {
		t.Errorf("expected %q deselected after invert", first)
	}

	// Inverting twice restores the original selection.
	got2 := p.InvertSelection()
	if got2 != 1 {
		t.Errorf("second InvertSelection expected 1, got %d", got2)
	}
	if !p.IsSelected(first) {
		t.Errorf("expected %q selected again after double invert", first)
	}
	if p.SelectionCount() != 1 {
		t.Errorf("SelectionCount expected 1 after double invert, got %d", p.SelectionCount())
	}
}

func TestSelectMaskAndDeselectMask(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	n, err := p.SelectMask("*.txt")
	if err != nil {
		t.Fatalf("SelectMask(*.txt) unexpected error: %v", err)
	}
	if n != 2 {
		t.Errorf("SelectMask(*.txt) expected 2, got %d", n)
	}
	if !p.IsSelected("a-file.txt") || !p.IsSelected("b-file.txt") {
		t.Error("expected a-file.txt and b-file.txt selected")
	}
	if p.IsSelected("c-file.log") {
		t.Error("c-file.log should not be selected by *.txt")
	}
	if p.SelectionCount() != 2 {
		t.Errorf("SelectionCount expected 2, got %d", p.SelectionCount())
	}

	m, err := p.DeselectMask("*.txt")
	if err != nil {
		t.Fatalf("DeselectMask(*.txt) unexpected error: %v", err)
	}
	if m != 2 {
		t.Errorf("DeselectMask(*.txt) expected 2, got %d", m)
	}
	if p.SelectionCount() != 0 {
		t.Errorf("SelectionCount expected 0 after deselect, got %d", p.SelectionCount())
	}
}

func TestSelectMask_CaseInsensitive(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Uppercase pattern must match the lowercase file names.
	n, err := p.SelectMask("*.TXT")
	if err != nil {
		t.Fatalf("SelectMask(*.TXT) unexpected error: %v", err)
	}
	if n != 2 {
		t.Errorf("SelectMask(*.TXT) expected 2, got %d", n)
	}
	if !p.IsSelected("a-file.txt") || !p.IsSelected("b-file.txt") {
		t.Error("expected case-insensitive *.TXT to select a-file.txt and b-file.txt")
	}
}

func TestSelectMask_Malformed(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Pre-select one entry so we can assert the selection is untouched.
	p.SetCursor(0)
	p.ToggleSelectAtCursor()
	before := p.SelectionCount()

	n, err := p.SelectMask("[")
	if err == nil {
		t.Error("SelectMask(\"[\") expected error for malformed pattern, got nil")
	}
	if n != 0 {
		t.Errorf("malformed SelectMask should select nothing, got %d", n)
	}
	if p.SelectionCount() != before {
		t.Errorf("selection changed on malformed SelectMask: before %d, after %d", before, p.SelectionCount())
	}
}

func TestDeselectMask_Malformed(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	p.SetCursor(0)
	p.ToggleSelectAtCursor()
	before := p.SelectionCount()

	n, err := p.DeselectMask("[")
	if err == nil {
		t.Error("DeselectMask(\"[\") expected error for malformed pattern, got nil")
	}
	if n != 0 {
		t.Errorf("malformed DeselectMask should deselect nothing, got %d", n)
	}
	if p.SelectionCount() != before {
		t.Errorf("selection changed on malformed DeselectMask: before %d, after %d", before, p.SelectionCount())
	}
}

func TestLoad_ClearsSelection(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	p.SetCursor(0)
	p.ToggleSelectAtCursor()
	if p.SelectionCount() == 0 {
		t.Fatal("precondition: expected a non-empty selection")
	}

	if err := p.Load(dir); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if p.SelectionCount() != 0 {
		t.Errorf("expected selection cleared after Load, got %d", p.SelectionCount())
	}
	if len(p.SelectedEntries()) != 0 {
		t.Errorf("SelectedEntries expected empty after Load, got %d", len(p.SelectedEntries()))
	}
}

// ---------------------------------------------------------------------------
// Incremental filter (quick search)
// ---------------------------------------------------------------------------

// namesOf returns the entry names in order, for compact assertions.
func namesOf(entries []Entry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return names
}

// equalStrings reports whether two string slices are equal in order.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSetFilter_NarrowsEntries(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}
	all := len(p.Entries())
	if all != 6 {
		t.Fatalf("precondition: expected 6 entries, got %d", all)
	}

	p.SetFilter("a-file")
	got := p.Entries()
	if len(got) != 1 || got[0].Name != "a-file.txt" {
		t.Fatalf("SetFilter(%q) expected [a-file.txt], got %v", "a-file", namesOf(got))
	}

	// The full entry set is preserved internally: clearing restores it.
	p.ClearFilter()
	if got := len(p.Entries()); got != all {
		t.Errorf("after ClearFilter expected %d entries, got %d", all, got)
	}
}

func TestSetFilter_CaseInsensitive(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	cases := []struct {
		name   string
		filter string
		want   []string
	}{
		{"uppercase matches lowercase", "ALPHA", []string{"alpha"}},
		{"lowercase matches mixed case", "beta", []string{"Beta"}},
		{"mixed case matches lowercase", "GaMmA", []string{"gamma"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p.SetFilter(tc.filter)
			if got := namesOf(p.Entries()); !equalStrings(got, tc.want) {
				t.Errorf("SetFilter(%q) = %v, want %v", tc.filter, got, tc.want)
			}
		})
	}
}

func TestSetFilter_SubstringMatch(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// "file" appears inside all three file names but no directory name.
	p.SetFilter("file")
	want := []string{"a-file.txt", "b-file.txt", "c-file.log"}
	if got := namesOf(p.Entries()); !equalStrings(got, want) {
		t.Errorf("SetFilter(%q) = %v, want %v", "file", got, want)
	}
}

func TestClearFilter_RestoresFullListing(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}
	all := len(p.Entries())

	p.SetFilter("a-file")
	if p.Filter() != "a-file" {
		t.Fatalf("Filter() expected %q, got %q", "a-file", p.Filter())
	}

	p.ClearFilter()
	if p.Filter() != "" {
		t.Errorf("Filter() expected empty after ClearFilter, got %q", p.Filter())
	}
	if got := len(p.Entries()); got != all {
		t.Errorf("after ClearFilter expected %d entries, got %d", all, got)
	}
}

func TestFilter_DoesNotTruncateFullListing(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}
	all := len(p.Entries())

	// A filter that matches nothing must not corrupt the full listing, and
	// repeated filter changes must always rebuild from the full entry set.
	for _, f := range []string{"a", "file", "no-such-entry-zzz", ""} {
		p.SetFilter(f)
	}
	if got := len(p.Entries()); got != all {
		t.Errorf("after filter cycle expected %d entries, got %d", all, got)
	}
}

func TestSetFilter_CursorClampedToVisible(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Move the cursor to the last full-listing entry, then narrow to a
	// single-match filter: the cursor must clamp into the filtered listing.
	p.SetCursor(len(p.Entries()) - 1)
	p.SetFilter("gamma")
	if p.Cursor() != 0 {
		t.Errorf("cursor expected 0 after narrowing filter, got %d", p.Cursor())
	}

	// SetCursor bounds now use the visible length.
	p.SetCursor(100)
	if p.Cursor() != 0 {
		t.Errorf("cursor expected 0 after SetCursor beyond visible bounds, got %d", p.Cursor())
	}
}

func TestSetFilter_SelectionVisibleScoped(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	n, err := p.SelectMask("*.txt")
	if err != nil {
		t.Fatalf("SelectMask(*.txt): %v", err)
	}
	if n != 2 {
		t.Fatalf("precondition: expected 2 selected, got %d", n)
	}

	// Hide b-file.txt by filtering to a-file.txt only.
	p.SetFilter("a-file")
	if got := p.SelectionCount(); got != 1 {
		t.Errorf("SelectionCount expected 1 (visible only), got %d", got)
	}
	if got := p.SelectedNames(); !equalStrings(got, []string{"a-file.txt"}) {
		t.Errorf("SelectedNames expected [a-file.txt], got %v", got)
	}

	// The hidden selection is retained: clearing the filter brings it back.
	p.ClearFilter()
	if got := p.SelectionCount(); got != 2 {
		t.Errorf("SelectionCount expected 2 after ClearFilter, got %d", got)
	}
}

func TestSelectMask_OnlyAffectsVisible(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	p.SetFilter("a-file")
	n, err := p.SelectMask("*.txt")
	if err != nil {
		t.Fatalf("SelectMask(*.txt): %v", err)
	}
	if n != 1 {
		t.Errorf("SelectMask on filtered listing expected 1, got %d", n)
	}
	if !p.IsSelected("a-file.txt") {
		t.Error("expected a-file.txt selected")
	}
	if p.IsSelected("b-file.txt") {
		t.Error("b-file.txt is hidden and must not be selected")
	}
}

func TestFirstMatchIndex(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Full-listing index of "gamma" (dirs sort first: alpha, Beta, gamma).
	full := p.Entries()
	wantIdx := -1
	for i, e := range full {
		if e.Name == "gamma" {
			wantIdx = i
		}
	}
	if wantIdx != 2 {
		t.Fatalf("precondition: expected gamma at full index 2, got %d", wantIdx)
	}

	if got := p.FirstMatchIndex(); got != -1 {
		t.Errorf("FirstMatchIndex with no filter expected -1, got %d", got)
	}

	p.SetFilter("gamma")
	if got := p.FirstMatchIndex(); got != wantIdx {
		t.Errorf("FirstMatchIndex(%q) expected %d, got %d", "gamma", wantIdx, got)
	}

	p.SetFilter("no-such-entry-zzz")
	if got := p.FirstMatchIndex(); got != -1 {
		t.Errorf("FirstMatchIndex with no matches expected -1, got %d", got)
	}
}

func TestLoad_ClearsFilter(t *testing.T) {
	dir, cleanup := mkDir(t)
	defer cleanup()

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	p.SetFilter("a-file")
	if got := len(p.Entries()); got != 1 {
		t.Fatalf("precondition: expected 1 filtered entry, got %d", got)
	}

	if err := p.Load(dir); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if p.Filter() != "" {
		t.Errorf("expected filter cleared after Load, got %q", p.Filter())
	}
	if got := len(p.Entries()); got != 6 {
		t.Errorf("expected 6 entries after Load (filter cleared), got %d", got)
	}
}

package ui

import (
	"os"
	"path/filepath"
	"testing"

	"tui-commander/internal/core"
)

// --- opLabel ---------------------------------------------------------------

func TestOpLabel(t *testing.T) {
	tests := []struct {
		name string
		verb string
		item string
		n    int
		want string
	}{
		{
			name: "single item uses the name",
			verb: "Copying",
			item: "x.txt",
			n:    1,
			want: "Copying x.txt",
		},
		{
			name: "multi selection uses the count",
			verb: "Copying",
			item: "x.txt",
			n:    3,
			want: "Copying 3 items",
		},
		{
			name: "deleting a single named item",
			verb: "Deleting",
			item: "report.pdf",
			n:    1,
			want: "Deleting report.pdf",
		},
		{
			name: "moving a multi selection",
			verb: "Moving",
			item: "report.pdf",
			n:    2,
			want: "Moving 2 items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := opLabel(tt.verb, tt.item, tt.n)
			if got != tt.want {
				t.Fatalf("opLabel(%q, %q, %d) = %q, want %q", tt.verb, tt.item, tt.n, got, tt.want)
			}
		})
	}
}

// --- targetPaths -----------------------------------------------------------

func TestTargetPathsSingleSourceKeepsFullDest(t *testing.T) {
	const dest = `D:\dst\renamed.txt`

	got := targetPaths(dest, []core.Entry{{Name: "x.txt"}})

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0] != dest {
		t.Fatalf("single-source target = %q, want the full destination path unchanged (%q)", got[0], dest)
	}
}

func TestTargetPathsMultiJoinsEachName(t *testing.T) {
	const dest = `D:\dst`

	got := targetPaths(dest, []core.Entry{{Name: "a.txt"}, {Name: "b.txt"}})
	want := []string{
		filepath.Join(dest, "a.txt"),
		filepath.Join(dest, "b.txt"),
	}

	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("targetPaths[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTargetPathsEmptySources(t *testing.T) {
	got := targetPaths(`D:\dst`, nil)
	if len(got) != 0 {
		t.Fatalf("targetPaths with no sources = %v, want empty", got)
	}
}

// --- overwritePrompt -------------------------------------------------------

func TestOverwritePrompt(t *testing.T) {
	tests := []struct {
		name string
		dest string
		n    int
		want string
	}{
		{
			name: "single item shows the destination path",
			dest: `D:\dst\x`,
			n:    1,
			want: `Overwrite D:\dst\x?`,
		},
		{
			name: "multi selection shows the count",
			dest: `D:\dst`,
			n:    2,
			want: "Overwrite 2 items?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := overwritePrompt(tt.dest, tt.n)
			if got != tt.want {
				t.Fatalf("overwritePrompt(%q, %d) = %q, want %q", tt.dest, tt.n, got, tt.want)
			}
		})
	}
}

// --- selectedEntries -------------------------------------------------------

// newTestPanel loads a real core.Panel from a fresh temp directory holding
// one file per given name.
func newTestPanel(t *testing.T, names ...string) *core.Panel {
	t.Helper()

	dir := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("test setup: cannot create %s: %v", name, err)
		}
	}

	panel := core.NewPanel()
	if err := panel.Load(dir); err != nil {
		t.Fatalf("test setup: cannot load panel: %v", err)
	}
	return panel
}

func entryIndex(entries []core.Entry, name string) int {
	for i, e := range entries {
		if e.Name == name {
			return i
		}
	}
	return -1
}

func entryNames(entries []core.Entry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return names
}

func TestSelectedEntriesEmptyPanelReturnsNil(t *testing.T) {
	left := newTestPanel(t) // empty directory
	right := core.NewPanel()
	app := NewApp(left, right)

	if got := app.selectedEntries(left); got != nil {
		t.Fatalf("selectedEntries on empty panel = %v, want nil", entryNames(got))
	}
}

func TestSelectedEntriesFallsBackToCursor(t *testing.T) {
	left := newTestPanel(t, "a.txt", "b.txt", "c.txt")
	right := core.NewPanel()
	app := NewApp(left, right)

	entries := left.Entries()
	left.SetCursor(1) // cursor on b.txt, nothing selected

	got := app.selectedEntries(left)

	if len(got) != 1 {
		t.Fatalf("selectedEntries with empty selection returned %d entries, want exactly 1", len(got))
	}
	if got[0].Name != entries[1].Name {
		t.Fatalf("selectedEntries = %v, want exactly the cursor entry %q", entryNames(got), entries[1].Name)
	}
}

func TestSelectedEntriesUsesSelectionExcludingCursor(t *testing.T) {
	left := newTestPanel(t, "a.txt", "b.txt", "c.txt")
	right := core.NewPanel()
	app := NewApp(left, right)

	entries := left.Entries()
	idxA := entryIndex(entries, "a.txt")
	idxB := entryIndex(entries, "b.txt")
	if idxA < 0 || idxB < 0 {
		t.Fatalf("test setup: entries %v are missing a.txt/b.txt", entryNames(entries))
	}

	// Select b.txt, then move the cursor onto the unselected a.txt.
	left.SetCursor(idxB)
	if !left.ToggleSelectAtCursor() {
		t.Fatalf("test setup: selecting b.txt failed")
	}
	left.SetCursor(idxA)

	got := app.selectedEntries(left)

	if len(got) != 1 || got[0].Name != "b.txt" {
		t.Fatalf("selectedEntries = %v, want exactly the selected entry b.txt (cursor a.txt excluded)", entryNames(got))
	}
}

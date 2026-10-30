package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"tui-commander/internal/core"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// newTestAppForEditor builds an App with a panel that has a known entry.
func newTestAppForEditor(t *testing.T) *App {
	app := NewApp(core.NewPanel(), core.NewPanel())
	app.buildUI()
	stopped := false
	app.stopApp = func() { stopped = true }
	_ = stopped

	// Populate left panel with a test directory structure
	testDir := t.TempDir()
	// Create a test file
	testFile := filepath.Join(testDir, "testfile.txt")
	_ = os.WriteFile(testFile, []byte("hello"), 0o644)
	// Create a subdirectory
	subDir := filepath.Join(testDir, "subdir")
	_ = os.Mkdir(subDir, 0o755)

	app.leftPanel.Load(testDir)
	app.rightPanel.Load(testDir)
	app.tviewApp.SetFocus(app.leftView)

	return app
}

// deliverKey routes a key through the app's input capture and focus chain.
func deliverKey(app *App, key tcell.Key) {
	ev := tcell.NewEventKey(key, 0, tcell.ModNone)
	if ev2 := app.tviewApp.GetInputCapture()(ev); ev2 != nil {
		if app.pages.HasFocus() {
			app.pages.InputHandler()(ev2, func(p tview.Primitive) { app.tviewApp.SetFocus(p) })
		}
	}
}

// TestEditCursorFile_FolderRejected tests that selecting a folder
// shows a message and does not launch an editor.
func TestEditCursorFile_FolderRejected(t *testing.T) {
	app := newTestAppForEditor(t)

	// Cursor on a directory
	app.leftPanel.Load(app.leftPanel.CurrentPath()) // reload to ensure entries
	entries := app.leftPanel.Entries()
	dirIdx := -1
	for i, e := range entries {
		if e.IsDir {
			dirIdx = i
			break
		}
	}
	if dirIdx == -1 {
		t.Fatal("no directory entry in test setup")
	}
	app.leftPanel.SetCursor(dirIdx)

	oldEditor := os.Getenv("EDITOR")
	_ = os.Setenv("EDITOR", "echo")
	defer func() { _ = os.Setenv("EDITOR", oldEditor) }()

	app.editCursorFile()

	// Should have shown a message dialog
	if len(app.dialogStack) != 1 {
		t.Fatalf("expected 1 dialog, got %d: %v", len(app.dialogStack), app.dialogStack)
	}
	if !strings.HasPrefix(app.dialogStack[0], "Edit") {
		t.Fatalf("expected Edit dialog, got %q", app.dialogStack[0])
	}
}

// TestEditCursorFile_NoEditorConfigured tests that missing EDITOR/VISUAL shows a message.
func TestEditCursorFile_NoEditorConfigured(t *testing.T) {
	app := newTestAppForEditor(t)

	// Select a file (not the directory)
	entries := app.leftPanel.Entries()
	fileIdx := -1
	for i, e := range entries {
		if !e.IsDir && e.Name != ".." {
			fileIdx = i
			break
		}
	}
	if fileIdx == -1 {
		t.Fatal("no file entry in test setup")
	}
	app.leftPanel.SetCursor(fileIdx)

	// Ensure EDITOR/VISUAL are unset
	oldEditor := os.Getenv("EDITOR")
	oldVisual := os.Getenv("VISUAL")
	_ = os.Unsetenv("EDITOR")
	_ = os.Unsetenv("VISUAL")
	defer func() {
		_ = os.Setenv("EDITOR", oldEditor)
		_ = os.Setenv("VISUAL", oldVisual)
	}()

	app.editCursorFile()

	if len(app.dialogStack) != 1 || !strings.HasPrefix(app.dialogStack[0], "Edit") {
		t.Fatalf("expected Edit dialog, got %v", app.dialogStack)
	}
}

// TestEditCursorFile_InvalidEditorValue tests that an EDITOR value
// that parseEditorCommand rejects (quoted empty string) shows an error dialog.
func TestEditCursorFile_InvalidEditorValue(t *testing.T) {
	app := newTestAppForEditor(t)

	// Select a file (not the directory)
	entries := app.leftPanel.Entries()
	fileIdx := -1
	for i, e := range entries {
		if !e.IsDir && e.Name != ".." {
			fileIdx = i
			break
		}
	}
	if fileIdx == -1 {
		t.Fatal("no file entry in test setup")
	}
	app.leftPanel.SetCursor(fileIdx)

	// Quoted empty EDITOR: resolveEditor returns it, but parseEditorCommand fails
	oldEditor := os.Getenv("EDITOR")
	oldVisual := os.Getenv("VISUAL")
	_ = os.Setenv("EDITOR", `""`)
	_ = os.Unsetenv("VISUAL")
	defer func() {
		_ = os.Setenv("EDITOR", oldEditor)
		_ = os.Setenv("VISUAL", oldVisual)
	}()

	// Debug: verify env is set correctly
	fmt.Printf("DEBUG: EDITOR=%q VISUAL=%q\n", os.Getenv("EDITOR"), os.Getenv("VISUAL"))

	app.editCursorFile()

	// Debug: what dialog was shown?
	fmt.Printf("DEBUG: dialogStack=%v\n", app.dialogStack)

	if len(app.dialogStack) != 1 || !strings.HasPrefix(app.dialogStack[0], "Error") {
		t.Fatalf("expected Error dialog, got %v", app.dialogStack)
	}
}

// TestEditCursorFile_SuspendFailure tests that failed Suspend shows error.
// This is hard to test deterministically because Suspend depends on terminal state.
func TestEditCursorFile_SuspendFailure(t *testing.T) {
	t.Skip("Suspend behavior depends on terminal state; skipped in headless CI")
}

// TestEditCursorFile_RereadAfterEditor tests that the panel is reread after editor returns.
// We simulate by setting a dummy editor that succeeds.
// Note: Suspend doesn't work in headless test environment, so this test verifies
// the code path up to the Suspend call, and that the function handles errors gracefully.
func TestEditCursorFile_RereadAfterEditor(t *testing.T) {
	app := newTestAppForEditor(t)

	// Select a file
	entries := app.leftPanel.Entries()
	fileIdx := -1
	for i, e := range entries {
		if !e.IsDir && e.Name != ".." {
			fileIdx = i
			break
		}
	}
	if fileIdx == -1 {
		t.Fatal("no file entry in test setup")
	}
	app.leftPanel.SetCursor(fileIdx)

	// Set an editor that would work if Suspend succeeded.
	// In headless CI, Suspend fails and we get an error dialog,
	// which is the expected behavior for this test environment.
	var editorCmd string
	if runtime.GOOS == "windows" {
		editorCmd = "cmd /c exit 0"
	} else {
		editorCmd = "true"
	}
	oldEditor := os.Getenv("EDITOR")
	_ = os.Setenv("EDITOR", editorCmd)
	defer func() { _ = os.Setenv("EDITOR", oldEditor) }()

	app.editCursorFile()

	// In headless CI, Suspend fails and we get "Cannot suspend the terminal"
	// which is correct behavior. The test just verifies no panic and
	// the error handling path works.
	if len(app.dialogStack) == 0 {
		t.Fatal("expected an error dialog (Suspend fails in headless CI)")
	}
	if !strings.HasPrefix(app.dialogStack[0], "Error") {
		t.Fatalf("expected Error dialog for Suspend failure, got %v", app.dialogStack)
	}

	// The panel should still be usable after the error
	newEntries := app.leftPanel.Entries()
	if len(newEntries) == 0 {
		t.Fatal("panel has no entries after error")
	}
}

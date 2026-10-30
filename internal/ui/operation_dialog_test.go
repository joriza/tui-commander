package ui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// cancelled is a test double for context.CancelFunc: it records whether it
// was invoked, so a test can assert that Esc did or did not abort an
// in-flight operation depending on which dialog was closed.
type cancelFlag struct{ called bool }

func (c *cancelFlag) cancel() { c.called = true }

// openProgressDialog opens a real ShowProgressDialog and binds a test
// cancel func to its page name, mirroring what runDelete/runCopy/runMove
// do after ShowProgressDialog returns.
func openProgressDialog(app *App) (closeProgress func(), title string) {
	update, closeProgress := app.ShowProgressDialog("Progress")
	_ = update
	title = app.dialogStack[len(app.dialogStack)-1]
	return closeProgress, title
}

// bindOperationCancel registers a CancelFunc for the given dialog title,
// exactly as the operation runners do. Returns the flag so the test can
// inspect it.
func bindOperationCancel(app *App, title string) *cancelFlag {
	f := &cancelFlag{}
	app.operationCancel[title] = f.cancel
	return f
}

// TestEscOnQuitDialogDoesNotCancelOperation reproduces the HIGH-2 defect:
// with the quit confirmation stacked over a progress dialog, pressing Esc
// to back out of quitting must close ONLY the quit dialog and must NOT
// abort the in-flight operation the user never asked to cancel.
func TestEscOnQuitDialogDoesNotCancelOperation(t *testing.T) {
	app, _ := newQuitTestApp()

	closeProgress, progTitle := openProgressDialog(app)
	// Bind the cancel BEFORE Esc and keep the SAME instance: the previous
	// version bound a second, freshly-allocated flag after Esc and asserted on
		// it, which is false by construction and made the test pass regardless of
	// whether Esc had cancelled the operation.
	f := bindOperationCancel(app, progTitle)
	app.confirmQuit() // stacks the quit dialog on top of the progress dialog

	app.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	// The quit dialog must be gone, the progress dialog must survive.
	if len(app.dialogStack) != 1 || app.dialogStack[0] != progTitle {
		t.Fatalf("dialog stack = %v, want only the progress dialog left after Esc on the quit prompt", app.dialogStack)
	}
	// The operation must NOT have been cancelled: Esc dismissed the quit
	// prompt, it did not abort the copy/move/delete. f is the flag bound
	// before Esc, so its state is the one Esc would have acted on.
	if f.called {
		t.Fatal("Esc on the quit dialog cancelled the in-flight operation; the user only asked to back out of quitting")
	}
	_ = closeProgress
}

// TestEscOnProgressDialogCancelsOperation asserts the inverse: Esc on the
// operation's own progress dialog is a legitimate cancel, and the cancel
// func bound to that dialog IS invoked.
func TestEscOnProgressDialogCancelsOperation(t *testing.T) {
	app, _ := newQuitTestApp()

	closeProgress, progTitle := openProgressDialog(app)
	f := bindOperationCancel(app, progTitle)

	app.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if len(app.dialogStack) != 0 {
		t.Fatalf("dialog stack = %v, want empty after Esc on the progress dialog", app.dialogStack)
	}
	if !f.called {
		t.Fatal("Esc on the progress dialog did not cancel the in-flight operation")
	}
	_ = closeProgress
}

// TestCloseDialogByNameRemovesBuriedDialog verifies the HIGH-1 fix at the
// primitive level: closeDialog(name) removes the named page and stack entry
// wherever it sits, not just the top. This is what OnDone's closeProgress
// must call instead of closeTopDialog.
func TestCloseDialogByNameRemovesBuriedDialog(t *testing.T) {
	app, _ := newQuitTestApp()

	_, progTitle := openProgressDialog(app)
	app.confirmQuit() // progTitle is buried under quitDialogTitle

	app.closeDialog(progTitle)

	// The progress dialog is gone, the quit dialog survives on top.
	if len(app.dialogStack) != 1 || !strings.HasPrefix(app.dialogStack[0], quitDialogTitle) {
		t.Fatalf("dialog stack = %v, want only the quit dialog after closing the buried progress dialog", app.dialogStack)
	}
	if app.pages.HasPage(progTitle) {
		t.Fatal("progress page still present after closeDialog by name")
	}
}

// TestCloseDialogByNameIsNoOpWhenMissing asserts closeDialog degrades
// gracefully when the name is not on the stack (e.g. a double-close).
func TestCloseDialogByNameIsNoOpWhenMissing(t *testing.T) {
	app, _ := newQuitTestApp()
	app.confirmQuit()

	before := len(app.dialogStack)
	app.closeDialog("does-not-exist")

	if len(app.dialogStack) != before {
		t.Fatalf("dialog stack changed from %d to %d when closing a missing dialog", before, len(app.dialogStack))
	}
}

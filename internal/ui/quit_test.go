package ui

import (
	"testing"

	"tui-commander/internal/core"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// newQuitTestApp builds an App with a full UI whose stop is observable:
// the tests replace stopApp with a flag instead of stopping tview.
func newQuitTestApp() (*App, *bool) {
	app := NewApp(core.NewPanel(), core.NewPanel())
	app.buildUI()
	stopped := false
	app.stopApp = func() { stopped = true }
	return app, &stopped
}

// findForm walks a dialog primitive tree and returns the first *tview.Form.
func findForm(p tview.Primitive) *tview.Form {
	switch v := p.(type) {
	case *tview.Form:
		return v
	case *tview.Flex:
		for i := 0; i < v.GetItemCount(); i++ {
			if f := findForm(v.GetItem(i)); f != nil {
				return f
			}
		}
	}
	return nil
}

func TestF10OpensQuitConfirmationInsteadOfStopping(t *testing.T) {
	app, stopped := newQuitTestApp()

	app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))

	if *stopped {
		t.Fatal("F10 stopped the app; want a confirmation dialog first")
	}
	if n := len(app.dialogStack); n != 1 || app.dialogStack[n-1] != quitDialogTitle {
		t.Fatalf("dialog stack = %v, want exactly the quit confirmation", app.dialogStack)
	}
}

func TestQuitDialogDefaultsToNo(t *testing.T) {
	app, stopped := newQuitTestApp()
	app.confirmQuit()

	form := findForm(app.pages.GetPage(quitDialogTitle))
	if form == nil {
		t.Fatal("quit dialog page contains no form")
	}
	_, focusedButton := form.GetFocusedItemIndex()
	if focusedButton != 1 {
		t.Fatalf("focused button index = %d, want 1 (No must be the default answer)", focusedButton)
	}

	// Enter on the focused No button declines and keeps the app running.
	form.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if *stopped {
		t.Fatal("Enter on the default No answer stopped the app")
	}
	if len(app.dialogStack) != 0 {
		t.Fatalf("dialog stack = %v, want empty after declining", app.dialogStack)
	}
}

func TestQuitDialogYesStopsApp(t *testing.T) {
	app, stopped := newQuitTestApp()
	app.confirmQuit()

	form := findForm(app.pages.GetPage(quitDialogTitle))
	quitButton := form.GetButton(0)
	if got := quitButton.GetLabel(); got != "Quit" {
		t.Fatalf("first button = %q, want %q", got, "Quit")
	}
	quitButton.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})

	if !*stopped {
		t.Fatal("confirming Quit did not stop the app")
	}
	if len(app.dialogStack) != 0 {
		t.Fatalf("dialog stack = %v, want empty after quitting", app.dialogStack)
	}
}

func TestEscOnQuitDialogCancels(t *testing.T) {
	app, stopped := newQuitTestApp()
	app.confirmQuit()

	app.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	if *stopped {
		t.Fatal("Esc on the quit dialog stopped the app")
	}
	if len(app.dialogStack) != 0 {
		t.Fatalf("dialog stack = %v, want empty after cancelling", app.dialogStack)
	}
}

func TestF10WhileQuitDialogOpenDoesNotStackAnother(t *testing.T) {
	app, stopped := newQuitTestApp()

	app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))
	app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))

	if *stopped {
		t.Fatal("repeated F10 stopped the app")
	}
	if n := len(app.dialogStack); n != 1 {
		t.Fatalf("dialog stack has %d entries, want 1", n)
	}
}

func TestF10WhileAnotherDialogOpenAsksConfirmation(t *testing.T) {
	app, stopped := newQuitTestApp()
	app.ShowMessageDialog("TestDialog", "some message", nil)

	app.handleKey(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone))

	if *stopped {
		t.Fatal("F10 with a dialog open stopped the app; want confirmation first")
	}
	if n := len(app.dialogStack); n != 2 || app.dialogStack[n-1] != quitDialogTitle {
		t.Fatalf("dialog stack = %v, want the quit confirmation on top", app.dialogStack)
	}

	// Cancelling the confirmation returns to the previous dialog.
	app.handleKey(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if len(app.dialogStack) != 1 || app.dialogStack[0] != "TestDialog" {
		t.Fatalf("dialog stack = %v, want only TestDialog left", app.dialogStack)
	}
	if *stopped {
		t.Fatal("cancelling the confirmation stopped the app")
	}
}

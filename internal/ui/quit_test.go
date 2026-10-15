package ui

import (
	"strings"
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

// deliver routes a key exactly like the real event loop does: global
// input capture first, then the root primitive handler with the app
// focus delegate. Keys that find no focus chain are dropped, matching
// production behavior.
func deliver(app *App, key tcell.Key) {
	ev := tcell.NewEventKey(key, 0, tcell.ModNone)
	if ev2 := app.tviewApp.GetInputCapture()(ev); ev2 != nil {
		if app.pages.HasFocus() {
			app.pages.InputHandler()(ev2, func(p tview.Primitive) { app.tviewApp.SetFocus(p) })
		}
	}
}

// dumpScreen renders the current simulation screen content as text.
func dumpScreen(s tcell.SimulationScreen) string {
	w, h := s.Size()
	var b strings.Builder
	for y := range h {
		for x := range w {
			r, _, _, _ := s.GetContent(x, y)
			b.WriteRune(r)
		}
		b.WriteByte('\n')
	}
	return b.String()
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

	// Enter on the default No answer declines and keeps the app running.
	deliver(app, tcell.KeyEnter)

	if *stopped {
		t.Fatal("Enter on the default No answer stopped the app")
	}
	if len(app.dialogStack) != 0 {
		t.Fatalf("dialog stack = %v, want empty after declining", app.dialogStack)
	}
}

func TestQuitDialogDeclineReturnsFocusToMainView(t *testing.T) {
	app, stopped := newQuitTestApp()
	app.confirmQuit()

	deliver(app, tcell.KeyEnter) // decline via the default No

	// The application focus must land back on the focused panel view: a
	// stale focus pointing at the removed dialog leaves the keyboard dead
	// for everything the global key capture does not consume.
	if got := app.tviewApp.GetFocus(); got != tview.Primitive(app.leftView) {
		t.Fatalf("focus after decline = %T, want the focused panel view (stale focus = dead keyboard)", got)
	}
	if *stopped {
		t.Fatal("declining stopped the app")
	}
}

func TestQuitDialogRightArrowSelectsQuit(t *testing.T) {
	app, stopped := newQuitTestApp()
	app.confirmQuit()

	deliver(app, tcell.KeyRight) // move the selection to Quit
	deliver(app, tcell.KeyEnter) // confirm

	if !*stopped {
		t.Fatal("Right + Enter did not confirm quitting (arrows must move the selection)")
	}
}

func TestQuitDialogLeftArrowReturnsToNo(t *testing.T) {
	app, stopped := newQuitTestApp()
	app.confirmQuit()

	deliver(app, tcell.KeyRight) // Quit
	deliver(app, tcell.KeyLeft)  // back to No
	deliver(app, tcell.KeyEnter) // decline

	if *stopped {
		t.Fatal("Left returned to No but Enter still stopped the app")
	}
	if len(app.dialogStack) != 0 {
		t.Fatalf("dialog stack = %v, want empty after declining", app.dialogStack)
	}
}

func TestEscOnQuitDialogCancels(t *testing.T) {
	app, stopped := newQuitTestApp()
	app.confirmQuit()

	deliver(app, tcell.KeyEscape)

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

func TestQuitDialogRendersMessageAndButtons(t *testing.T) {
	app, stopped := newQuitTestApp()

	screen := tcell.NewSimulationScreen("UTF-8")
	screen.SetSize(80, 25)
	app.tviewApp.SetScreen(screen)

	app.confirmQuit()
	app.tviewApp.ForceDraw()

	dump := dumpScreen(screen)
	for _, want := range []string{"Quit tui-commander?", "Quit", "No"} {
		if !strings.Contains(dump, want) {
			t.Fatalf("rendered screen does not contain %q:\n%s", want, dump)
		}
	}
	if *stopped {
		t.Fatal("rendering the dialog stopped the app")
	}
}

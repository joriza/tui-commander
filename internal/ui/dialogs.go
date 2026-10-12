package ui

import (
	"fmt"

	"github.com/rivo/tview"
)

// Dialog helpers run on the UI goroutine: tview event handlers and
// OperationRunner's OnDone callback are already marshalled there via
// QueueUpdateDraw. Calling QueueUpdateDraw from these paths would
// deadlock the event loop (it blocks until the next draw, which cannot
// happen while a handler is still running). The only cross-goroutine
// call below is updateProgress, invoked from the operation goroutine.

// ShowFormDialog displays a centered modal dialog with a form containing
// the given items. The dialog has a fixed width of approximately 60
// characters. Enter triggers OK, Esc triggers Cancel. The callback is
// invoked on the UI goroutine with true for OK, false for Cancel.
func (a *App) ShowFormDialog(title string, items []tview.FormItem, callback func(bool)) {
	form := tview.NewForm()
	for _, item := range items {
		form.AddFormItem(item)
	}
	form.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s ", title)).
		SetTitleAlign(tview.AlignCenter)
	form.AddButton("OK", func() {
		a.closeTopDialog()
		callback(true)
	}).AddButton("Cancel", func() {
		a.closeTopDialog()
		callback(false)
	})

	a.pages.AddPage(title, a.wrapDialog(form), true, true)
	a.dialogStack = append(a.dialogStack, title)
}

// ShowMessageDialog displays a modal dialog with a message and an OK button.
func (a *App) ShowMessageDialog(title, message string, callback func()) {
	textView := tview.NewTextView().
		SetText(message).
		SetDynamicColors(true)

	form := tview.NewForm().
		AddFormItem(textView).
		AddButton("OK", func() {
			a.closeTopDialog()
			if callback != nil {
				callback()
			}
		})
	form.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s ", title)).
		SetTitleAlign(tview.AlignCenter)

	a.pages.AddPage(title, a.wrapDialog(form), true, true)
	a.dialogStack = append(a.dialogStack, title)
}

// ShowErrorDialog displays a modal dialog with an error message and an OK
// button. The message is shown in a scrollable TextView, one line per error.
func (a *App) ShowErrorDialog(title, message string, callback func()) {
	textView := tview.NewTextView().
		SetText(message).
		SetDynamicColors(true).
		SetScrollable(true)

	form := tview.NewForm().
		AddFormItem(textView).
		AddButton("OK", func() {
			a.closeTopDialog()
			if callback != nil {
				callback()
			}
		})
	form.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s ", title)).
		SetTitleAlign(tview.AlignCenter)

	a.pages.AddPage(title, a.wrapDialog(form), true, true)
	a.dialogStack = append(a.dialogStack, title)
}

// ShowProgressDialog displays a modal progress dialog and returns two
// functions: both must be called from the UI goroutine (OperationRunner
// already marshals OnProgress and OnDone there). They must never call
// QueueUpdateDraw — nesting it inside a queued update self-deadlocks the
// event loop: the inner send waits for a loop iteration that cannot start
// until the outer update returns.
func (a *App) ShowProgressDialog(title string) (updateProgress func(done, total int), closeDialog func()) {
	textView := tview.NewTextView().
		SetDynamicColors(true)
	textView.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s ", title))
	textView.SetText("Processing...")

	a.pages.AddPage(title, a.wrapDialog(textView), true, true)
	a.dialogStack = append(a.dialogStack, title)

	updateProgress = func(done, total int) {
		textView.SetText(fmt.Sprintf("Processing... %d/%d", done, total))
	}

	closeDialog = func() {
		a.closeTopDialog()
	}

	return updateProgress, closeDialog
}

// wrapDialog wraps a primitive in a centered container with a fixed width
// of approximately 60 characters.
func (a *App) wrapDialog(content tview.Primitive) tview.Primitive {
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(content, 60, 1, true).
			AddItem(nil, 0, 1, false),
			0, 1, true).
		AddItem(nil, 0, 1, false)
}

// closeTopDialog removes the topmost dialog page from the stack.
func (a *App) closeTopDialog() {
	if len(a.dialogStack) == 0 {
		return
	}
	name := a.dialogStack[len(a.dialogStack)-1]
	a.dialogStack = a.dialogStack[:len(a.dialogStack)-1]
	a.pages.RemovePage(name)
}

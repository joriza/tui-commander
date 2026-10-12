package ui

import (
	"fmt"

	"github.com/rivo/tview"
)

// ShowFormDialog displays a centered modal dialog with a form containing
// the given items. The dialog has a fixed width of approximately 60
// characters. Enter triggers OK, Esc triggers Cancel. The callback is
// invoked on the UI goroutine with true for OK, false for Cancel.
func (a *App) ShowFormDialog(title string, items []tview.FormItem, callback func(bool)) {
	form := tview.NewForm()
	for _, item := range items {
		form.AddFormItem(item)
	}
	form.AddButton("OK", func() {
		a.tviewApp.QueueUpdateDraw(func() {
			a.closeTopDialog()
			callback(true)
		})
	}).AddButton("Cancel", func() {
		a.tviewApp.QueueUpdateDraw(func() {
			a.closeTopDialog()
			callback(false)
		})
	})

	dialog := a.wrapDialog(form)

	a.tviewApp.QueueUpdateDraw(func() {
		a.pages.AddPage(title, dialog, true, true)
		a.dialogStack = append(a.dialogStack, title)
	})
}

// ShowMessageDialog displays a modal dialog with a message and an OK button.
func (a *App) ShowMessageDialog(title, message string, callback func()) {
	textView := tview.NewTextView().
		SetText(message).
		SetDynamicColors(true)

	form := tview.NewForm().
		AddFormItem(textView).
		AddButton("OK", func() {
			a.tviewApp.QueueUpdateDraw(func() {
				a.closeTopDialog()
				if callback != nil {
					callback()
				}
			})
		})

	dialog := a.wrapDialog(form)

	a.tviewApp.QueueUpdateDraw(func() {
		a.pages.AddPage(title, dialog, true, true)
		a.dialogStack = append(a.dialogStack, title)
	})
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
			a.tviewApp.QueueUpdateDraw(func() {
				a.closeTopDialog()
				if callback != nil {
					callback()
				}
			})
		})

	dialog := a.wrapDialog(form)

	a.tviewApp.QueueUpdateDraw(func() {
		a.pages.AddPage(title, dialog, true, true)
		a.dialogStack = append(a.dialogStack, title)
	})
}

// ShowProgressDialog displays a modal progress dialog and returns
// functions to update the progress text and close the dialog.
func (a *App) ShowProgressDialog(title string) (updateProgress func(done, total int), closeDialog func()) {
	textView := tview.NewTextView().
		SetDynamicColors(true)
	textView.SetText("Processing...")

	dialog := a.wrapDialog(textView)

	a.tviewApp.QueueUpdateDraw(func() {
		a.pages.AddPage(title, dialog, true, true)
		a.dialogStack = append(a.dialogStack, title)
	})

	updateProgress = func(done, total int) {
		a.tviewApp.QueueUpdateDraw(func() {
			textView.SetText(fmt.Sprintf("Processing... %d/%d", done, total))
		})
	}

	closeDialog = func() {
		a.tviewApp.QueueUpdateDraw(func() {
			a.closeTopDialog()
		})
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

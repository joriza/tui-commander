package ui

import (
	"fmt"
	"path/filepath"
	"strings"

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
// extraRows adds additional rows to the dialog height (useful for
// confirmation dialogs that need more vertical space).
func (a *App) ShowFormDialog(title string, items []tview.FormItem, extraRows int, callback func(bool)) {
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

	a.pages.AddPage(title, a.wrapDialog(form, len(items)+5+extraRows), true, true)
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

	height := strings.Count(message, "\n") + 5
	a.pages.AddPage(title, a.wrapDialog(form, height), true, true)
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

	height := strings.Count(message, "\n") + 5
	if height > 20 {
		height = 20 // scrollable TextView handles the overflow
	}
	a.pages.AddPage(title, a.wrapDialog(form, height), true, true)
	a.dialogStack = append(a.dialogStack, title)
}

// ShowProgressDialog displays a modal progress dialog with dual
// progress bars, file/current-item name, ETA, and throughput.
// It returns two functions: both must be called from the UI goroutine
// (OperationRunner already marshals OnProgress and OnDone there).
// They must never call QueueUpdateDraw — nesting it inside a queued
// update self-deadlocks the event loop.
func (a *App) ShowProgressDialog(title string) (updateProgress func(currentName string, fileBytes, fileSize int64, done, total int, doneBytes, totalBytes int64, eta ProgressETA), closeDialog func()) {
	textView := tview.NewTextView().
		SetDynamicColors(true)
	textView.SetBorder(true).
		SetTitle(fmt.Sprintf(" %s ", title))
	textView.SetText("Processing...")

	a.pages.AddPage(title, a.wrapDialog(textView, 7), true, true)
	a.dialogStack = append(a.dialogStack, title)

	updateProgress = func(currentName string, fileBytes, fileSize int64, done, total int, doneBytes, totalBytes int64, eta ProgressETA) {
		var sb strings.Builder
		// Line 1: current file name (basename, truncated to 30 chars).
		name := currentName
		if name == "" {
			name = "(preparando...)"
		} else if len(name) > 30 {
			name = filepath.Base(name)
			if len(name) > 30 {
				name = "..." + name[len(name)-27:]
			}
		}
		fmt.Fprintf(&sb, "%s\n", name)

		// Line 2: file count + size.
		if done > total {
			done = total
		}
		fmt.Fprintf(&sb, "Archivos: %d/%d  (%s)", done, total, formatSize(fileSize))

		textView.SetText(sb.String())
	}

	closeDialog = func() {
		a.closeTopDialog()
	}

	return updateProgress, closeDialog
}

// progressBar returns a 20-cell progress bar string.
// filled is the number of filled cells (0-20).
func progressBar(filled int) string {
	if filled < 0 {
		filled = 0
	}
	if filled > 20 {
		filled = 20
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)
}

// formatETARate formats a throughput rate in MB/s for display.
func formatETARate(rateMBs float64) string {
	if rateMBs <= 0 {
		return ""
	}
	return fmt.Sprintf("%s/s", formatSize(int64(rateMBs*1024*1024)))
}

// wrapDialog wraps a primitive in a centered container with a fixed width
// of approximately 60 characters and a content-sized height, so modals do
// not stretch over the full screen.
func (a *App) wrapDialog(content tview.Primitive, height int) tview.Primitive {
	return tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(content, 60, 1, true).
			AddItem(nil, 0, 1, false),
			height, 0, true).
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

package ui

import (
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"tui-commander/internal/core"
)

const (
	// pagerPageName is the Pages key of the full-screen built-in pager.
	pagerPageName = "pager"

	// previewMaxBytes bounds how much of a file the pager samples, so even a
	// huge file is never read into memory in full.
	previewMaxBytes = 1 << 20 // 1 MiB
)

// pagerTitle builds the pager border title: the file name, its total size and
// a marker when only the first bytes of the file are shown. Kept free of live
// tview primitives so it is unit-testable.
func pagerTitle(name string, preview core.TextPreview) string {
	title := fmt.Sprintf("%s  %s", name, formatSize(preview.Size))
	if preview.Truncated {
		title += " (truncated)"
	}
	return title
}

// pagerBody returns the text the pager displays. A binary preview shows a short
// notice with the file size instead of dumping raw bytes. Kept free of live
// tview primitives so it is unit-testable.
func pagerBody(preview core.TextPreview) string {
	if preview.Binary {
		return fmt.Sprintf("Binary file (%s): contents not shown.", formatSize(preview.Size))
	}
	return preview.Text
}

// viewCursorFile opens the focused panel's cursor entry in the built-in pager
// (Total Commander F3). Only single files are supported: a directory or the
// parent entry shows a message instead. Reading is bounded by previewMaxBytes.
func (a *App) viewCursorFile() {
	panel := a.focusedPanel()
	entries := panel.Entries()
	if len(entries) == 0 {
		return
	}

	entry := entries[panel.Cursor()]
	if entry.IsDir || entry.Name == ".." {
		a.ShowMessageDialog("View", "Select a file to view. Folders are not supported in v0.", nil)
		return
	}

	path := filepath.Join(panel.CurrentPath(), entry.Name)
	preview, err := core.ReadTextPreview(path, previewMaxBytes)
	if err != nil {
		a.ShowErrorDialog("Error", fmt.Sprintf("Cannot view file: %v", err), nil)
		return
	}

	a.openPager(entry.Name, preview)
}

// openPager shows a full-screen, scrollable page with the preview. It runs on
// the UI goroutine (the key handler), so no QueueUpdateDraw is used. The panels
// page is hidden first so the pager is fully opaque; it is restored on close.
func (a *App) openPager(name string, preview core.TextPreview) {
	view := tview.NewTextView()
	// Literal text: a viewer must not interpret "[...]" sequences in file
	// contents as tview color tags.
	view.SetDynamicColors(false)
	view.SetScrollable(true)
	view.SetBorder(true)
	view.SetBorderColor(tcell.ColorGrey)
	view.SetTitle(fmt.Sprintf(" %s ", pagerTitle(name, preview)))
	view.SetText(pagerBody(preview))

	a.pagerOpen = true
	a.pages.HidePage("main")
	a.pages.AddPage(pagerPageName, view, true, true)
	a.tviewApp.SetFocus(view)
}

// handlePagerKey routes keys while the pager is open. Esc, F3 and F10 close it.
// Every other key (scroll keys included) is returned so the focused TextView
// handles it; pager input never reaches the panels underneath.
func (a *App) handlePagerKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape, tcell.KeyF3, tcell.KeyF10:
		a.closePager()
		return nil
	}
	return event
}

// closePager hides the pager and restores the panels. The panels page must be
// shown before removing the pager so Pages keeps a valid visible page.
func (a *App) closePager() {
	a.pagerOpen = false
	a.pages.ShowPage("main")
	a.pages.RemovePage(pagerPageName)
}

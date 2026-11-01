package ui

import (
	"fmt"
	"path/filepath"
	"strings"

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

// pagerBodyHighlighted returns highlighted lines for syntax highlighting.
// Returns (highlightedLines, usedHighlighting).
func pagerBodyHighlighted(preview core.TextPreview, ext string) ([]string, bool) {
	if preview.Binary || preview.Text == "" {
		return nil, false
	}
	// Normalize line endings before splitting
	normalizedText := normalizeLineEndings(preview.Text)
	highlighted, err := HighlightLines(ext, strings.Split(normalizedText, "\n"), "black")
	if err != nil || len(highlighted) == 0 {
		return nil, false
	}
	return highlighted, true
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
	panel := a.focusedPanel()
	path := filepath.Join(panel.CurrentPath(), name)
	ext := strings.ToLower(filepath.Ext(name))

	// Check for media files
	if mediaInfo, err := core.ReadMediaInfo(path); err == nil && mediaInfo.FileType != "unknown" {
		// PDF: show extracted text content
		if mediaInfo.FileType == "document" && mediaInfo.Format == "PDF" {
			text, err := core.ExtractPDFText(path)
			if err != nil {
				a.openMediaViewer(name, mediaInfo)
				return
			}
			if text != "" {
				view := tview.NewTextView()
				view.SetDynamicColors(false)
				view.SetScrollable(true)
				view.SetBorder(true)
				view.SetBorderColor(tcell.ColorGrey)
				view.SetTitle(fmt.Sprintf(" %s ", name))
				view.SetText(text)
				a.pagerOpen = true
				a.pages.HidePage("main")
				a.pages.AddPage(pagerPageName, view, true, true)
				a.tviewApp.SetFocus(view)
				return
			}
		}
		// Images/videos: show "cannot display" message
		a.openCannotDisplay(name)
		return
	}

	// Try syntax highlighting for code files
	highlighted, usedHighlighting := pagerBodyHighlighted(preview, ext)

	view := tview.NewTextView()
	if usedHighlighting {
		// Use DynamicColors for syntax highlighting tags
		view.SetDynamicColors(true)
		view.SetText(strings.Join(highlighted, "\n"))
	} else {
		// Literal text: a viewer must not interpret "[...]" sequences in file
		// contents as tview color tags.
		view.SetDynamicColors(false)
		view.SetText(pagerBody(preview))
	}

	view.SetScrollable(true)
	view.SetBorder(true)
	view.SetBorderColor(tcell.ColorGrey)
	view.SetTitle(fmt.Sprintf(" %s ", pagerTitle(name, preview)))

	a.pagerOpen = true
	a.pagerHexMode = false
	a.pagerWrapMode = true
	a.pagerSearchMode = false
	a.pagerSearchQuery = ""
	a.pagerOrigText = preview.Text
	a.pagerOrigBinary = preview.Binary
	a.pagerOrigTruncated = preview.Truncated
	a.pagerOrigSize = preview.Size
	a.pages.HidePage("main")
	a.pages.AddPage(pagerPageName, view, true, true)
	a.tviewApp.SetFocus(view)
}

// handlePagerKey routes keys while the pager is open. Esc, F3 and F10 close it.
// Handles hex/wrap/search toggles and search navigation.
func (a *App) handlePagerKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape, tcell.KeyF3, tcell.KeyF10:
		a.closePager()
		return nil
	case tcell.KeyRune:
		switch event.Rune() {
		case 'h', 'H':
			a.togglePagerHexMode()
			return nil
		case 'w', 'W':
			a.togglePagerWrapMode()
			return nil
		case '/':
			a.enterPagerSearchMode()
			return nil
		case 'n':
			a.pagerSearchNext()
			return nil
		case 'N':
			a.pagerSearchPrev()
			return nil
		case 'q', 'Q':
			a.closePager()
			return nil
		}
	}
	return event
}

// openMediaViewer opens a media info viewer for image/video files.
func (a *App) openMediaViewer(name string, mediaInfo core.MediaInfo) {
	view := tview.NewTextView()
	view.SetDynamicColors(false)
	view.SetScrollable(true)
	view.SetBorder(true)
	view.SetBorderColor(tcell.ColorGrey)
	view.SetTitle(fmt.Sprintf(" %s ", name))
	view.SetText(mediaInfo.MediaInfoString())

	a.pagerOpen = true
	a.pages.HidePage("main")
	a.pages.AddPage(pagerPageName, view, true, true)
	a.tviewApp.SetFocus(view)
}

// openCannotDisplay shows a "cannot display" message for unsupported file types.
func (a *App) openCannotDisplay(name string) {
	view := tview.NewTextView()
	view.SetDynamicColors(false)
	view.SetScrollable(true)
	view.SetBorder(true)
	view.SetBorderColor(tcell.ColorGrey)
	view.SetTitle(fmt.Sprintf(" %s ", name))
	view.SetText(fmt.Sprintf("Cannot display %s\n\nThis file type (image/video) cannot be previewed in the terminal.", name))

	a.pagerOpen = true
	a.pages.HidePage("main")
	a.pages.AddPage(pagerPageName, view, true, true)
	a.tviewApp.SetFocus(view)
}

// closePager hides the pager and restores the panels. The panels page must be
// shown before removing the pager so Pages keeps a valid visible page.
func (a *App) closePager() {
	a.pagerOpen = false
	a.pages.ShowPage("main")
	a.pages.RemovePage(pagerPageName)
}

// quickView shows a quick preview of the focused file in the other panel.
// This is similar to Total Commander's Quick View feature (Ctrl+Q).
func (a *App) quickView() {
	panel := a.focusedPanel()
	entries := panel.Entries()
	if len(entries) == 0 {
		return
	}

	entry := entries[panel.Cursor()]
	if entry.IsDir || entry.Name == ".." {
		return
	}

	path := filepath.Join(panel.CurrentPath(), entry.Name)
	preview, err := core.ReadTextPreview(path, previewMaxBytes)
	if err != nil {
		return
	}

	ext := strings.ToLower(filepath.Ext(entry.Name))
	otherView := a.rightView
	if a.focusIndex == 1 {
		otherView = a.leftView
	}

	// Check for media files
	if mediaInfo, err := core.ReadMediaInfo(filepath.Join(panel.CurrentPath(), entry.Name)); err == nil && mediaInfo.FileType != "unknown" {
		otherView.SetDynamicColors(false)
		otherView.SetScrollable(true)
		otherView.SetBorder(true)
		otherView.SetBorderColor(tcell.ColorDarkCyan)
		otherView.SetTitle(fmt.Sprintf(" Quick View: %s ", entry.Name))
		otherView.SetText(mediaInfo.MediaInfoString())
		return
	}

	// Try syntax highlighting for code files
	highlighted, usedHighlighting := pagerBodyHighlighted(preview, ext)
	if usedHighlighting {
		otherView.SetDynamicColors(true)
		otherView.SetText(strings.Join(highlighted, "\n"))
	} else {
		otherView.SetDynamicColors(false)
		otherView.SetText(pagerBody(preview))
	}
	otherView.SetScrollable(true)
	otherView.SetBorder(true)
	otherView.SetBorderColor(tcell.ColorDarkCyan)
	otherView.SetTitle(fmt.Sprintf(" Quick View: %s ", entry.Name))
}

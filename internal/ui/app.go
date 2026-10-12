package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"tui-commander/internal/core"
)

// App is the dual-panel UI application. It consumes two core.Panel
// instances and renders them side by side using tview primitives.
// The UI layer performs no filesystem logic; all panel state comes
// from core.Panel.
type App struct {
	leftPanel    *core.Panel
	rightPanel   *core.Panel
	leftView     *tview.TextView
	rightView    *tview.TextView
	focusIndex   int // 0 = left panel, 1 = right panel
	tviewApp     *tview.Application
	keyBar       *tview.TextView
	pages        *tview.Pages
	dialogStack  []string
	activeCancel context.CancelFunc

	// Quick search window state (TC "letter with search window").
	searchMode   bool
	searchField  *tview.InputField
	searchOrigin int
}

// NewApp creates a new dual-panel App with the given panels.
func NewApp(left, right *core.Panel) *App {
	return &App{
		leftPanel:  left,
		rightPanel: right,
		focusIndex: 0,
		tviewApp:   tview.NewApplication(),
	}
}

// Run starts the tview event loop.
func (a *App) Run() error {
	a.buildUI()
	return a.tviewApp.Run()
}

// Stop halts the application.
func (a *App) Stop() {
	a.tviewApp.Stop()
}

func (a *App) buildUI() {
	// Left panel view
	a.leftView = tview.NewTextView()
	a.leftView.SetDynamicColors(true)
	a.leftView.SetBorder(true)
	a.leftView.SetBorderColor(tcell.ColorGrey)

	// Right panel view
	a.rightView = tview.NewTextView()
	a.rightView.SetDynamicColors(true)
	a.rightView.SetBorder(true)
	a.rightView.SetBorderColor(tcell.ColorGrey)

	// Key bar at the bottom
	a.keyBar = tview.NewTextView()
	a.keyBar.SetText("Tab Panel  Ins/Space Sel  +/-/* Group  Alt+F1/F2 Drive  F2 Reread  F5 Copy  Shift+F5 Dup  F6 Move  Shift+F6 Ren  F7 MkDir  F8 Del  F10 Quit")

	// Two panels side by side
	panelsFlex := tview.NewFlex().
		AddItem(a.leftView, 0, 1, false).
		AddItem(a.rightView, 0, 1, false)

	// Root layout: panels on top, key bar on bottom
	rootFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(panelsFlex, 0, 1, false).
		AddItem(a.keyBar, 1, 0, false)

	// Pages as root; "main" page holds the existing layout.
	a.pages = tview.NewPages()
	a.pages.AddPage("main", rootFlex, true, true)

	// Quick search window: a bottom-pinned input field shown while the
	// user is typing a filter. Added hidden; it is never pushed onto
	// dialogStack because it is not a modal dialog.
	a.searchField = tview.NewInputField()
	a.searchField.SetFieldWidth(45)
	a.searchField.SetBorder(true)
	a.searchField.SetTitle(" Quick search ")

	searchWrap := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(a.searchField, 45, 1, true).
			AddItem(nil, 0, 1, false), 3, 0, true).
		// Fixed 2-row bottom spacer: keeps the window above the key bar
		// (which occupies the last row of the main page).
		AddItem(nil, 2, 0, false)
	a.pages.AddPage("search", searchWrap, true, false)

	a.tviewApp.SetRoot(a.pages, true)

	// Global key capture
	a.tviewApp.SetInputCapture(a.handleKey)

	// Re-render every frame so the cursor bar matches the real panel
	// width (startup and terminal resizes included).
	a.tviewApp.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		a.render()
		return false
	})
}

// render refreshes both panel views from their core.Panel state.
func (a *App) render() {
	a.renderPanel(a.leftPanel, a.leftView, a.focusIndex == 0)
	a.renderPanel(a.rightPanel, a.rightView, a.focusIndex == 1)
}

// renderPanel draws a single panel's entries into its TextView.
// The focused panel gets a green border; the unfocused gets grey.
// The cursor entry is shown in reverse video.
func (a *App) renderPanel(panel *core.Panel, view *tview.TextView, focused bool) {
	view.Clear()

	// Border color based on focus
	if focused {
		view.SetBorderColor(tcell.ColorGreen)
	} else {
		view.SetBorderColor(tcell.ColorGrey)
	}

	// Title shows the current path, plus the selection count when non-empty.
	if panel.SelectionCount() > 0 {
		view.SetTitle(fmt.Sprintf(" %s [%d] ", panel.CurrentPath(), panel.SelectionCount()))
	} else {
		view.SetTitle(fmt.Sprintf(" %s ", panel.CurrentPath()))
	}

	entries := panel.Entries()
	cursor := panel.Cursor()

	_, _, width, _ := view.GetInnerRect()
	if width <= 0 {
		width = 80
	}

	for i, e := range entries {
		var sizeStr string
		if e.IsDir {
			sizeStr = "<DIR>"
		} else {
			sizeStr = formatSize(e.Size)
		}
		mtimeStr := e.ModTime.Format("2006-01-02 15:04")

		nameCol := nameColumnWidth(width)
		line := fmt.Sprintf("%-*s  %8s  %s", nameCol, truncateMiddle(e.Name, nameCol), sizeStr, mtimeStr)
		// The cursor bar must match the real row width: a fixed padding
		// wider than the panel wraps lines and drags the background color
		// into extra visual lines (looked like "everything selected").
		line = padOrTruncate(line, width)

		if i == cursor {
			// Full-row bar: muted green on the focused panel, dim grey on
			// the other (both cursors visible, TC style). Close with
			// [-:-] (fg AND bg reset) — a bare [-] only resets the
			// foreground, so the background would bleed into every row
			// below the cursor.
			if focused {
				fmt.Fprintf(view, "[black:darkgreen]%s[-:-]\n", line)
			} else {
				fmt.Fprintf(view, "[black:gray]%s[-:-]\n", line)
			}
		} else if panel.IsSelected(e.Name) {
			// Selected non-cursor rows are highlighted in yellow.
			fmt.Fprintf(view, "[yellow]%s[-]\n", line)
		} else {
			fmt.Fprintf(view, "%s\n", line)
		}
	}
}

// padOrTruncate makes the line exactly width runes wide so the cursor
// highlight spans the row without wrapping.
func padOrTruncate(s string, width int) string {
	r := []rune(s)
	if len(r) > width {
		return string(r[:max(width-1, 0)]) + "…"
	}
	return s + strings.Repeat(" ", width-len(r))
}

// nameColumnWidth returns the width reserved for an entry name inside a panel
// with the given inner width. The name column fills the space left after the
// fixed size and date columns, so it scales with the panel (and window) width.
// A minimum keeps names readable in a narrow, unmaximized window.
func nameColumnWidth(innerWidth int) int {
	const reserved = 8 /* size */ + 2 /* gap */ + 16 /* date */ + 2 /* gap */
	w := innerWidth - reserved
	if w < 20 {
		w = 20
	}
	return w
}

// truncateMiddle shortens s to at most w runes by inserting a single ellipsis
// in the middle, so the beginning and the end (typically the extension) both
// stay visible. Strings already within w are returned unchanged.
func truncateMiddle(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return "…"
	}
	left := (w - 1) / 2
	right := w - 1 - left
	return string(r[:left]) + "…" + string(r[len(r)-right:])
}

// formatSize returns a human-readable file size string.
// Directories are shown as "<DIR>" by the caller; this function
// handles files only.
func formatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%dB", size)
	}
	div, exp := int64(unit), 0
	for size >= div*unit && exp < 3 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(size)/float64(div), "KMGT"[exp])
}

// handleKey processes global key events for navigation and focus switching.
// When a dialog is open, only Esc (close dialog) and F10 (quit) are handled
// at the app level; all other keys pass through to the focused dialog.
func (a *App) handleKey(event *tcell.EventKey) *tcell.EventKey {
	// Dialog is open: only Esc and F10 are handled here.
	if len(a.dialogStack) > 0 {
		switch {
		case event.Key() == tcell.KeyEscape:
			a.closeTopDialog()
			if a.activeCancel != nil {
				a.activeCancel()
				a.activeCancel = nil
			}
			return nil

		case event.Key() == tcell.KeyF10:
			a.tviewApp.Stop()
			return nil
		}
		// Pass all other keys through to the dialog primitive.
		return event
	}

	// Quick search window is open: route keys to the search handler. A key
	// that cancels the search is processed normally below so it acts on the
	// restored full listing.
	if a.searchMode {
		ev := a.handleSearchKey(event)
		if ev == nil {
			return nil
		}
		event = ev
	}

	// No dialog open: normal key handling.
	switch {
	case event.Key() == tcell.KeyTab:
		a.switchFocus()
		return nil

	case event.Key() == tcell.KeyEnter:
		a.enter()
		return nil

	case event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2:
		a.parentDir()
		return nil

	case event.Key() == tcell.KeyPgUp && event.Modifiers()&tcell.ModCtrl != 0:
		a.moveCursor(-10)
		return nil

	case event.Key() == tcell.KeyPgDn && event.Modifiers()&tcell.ModCtrl != 0:
		a.moveCursor(10)
		return nil

	case event.Key() == tcell.KeyUp:
		a.moveCursor(-1)
		return nil

	case event.Key() == tcell.KeyDown:
		a.moveCursor(1)
		return nil

	case event.Key() == tcell.KeyPgUp:
		a.moveCursor(-10)
		return nil

	case event.Key() == tcell.KeyPgDn:
		a.moveCursor(10)
		return nil

	case event.Key() == tcell.KeyHome:
		a.setCursor(0)
		return nil

	case event.Key() == tcell.KeyEnd:
		a.setCursor(len(a.focusedPanel().Entries()) - 1)
		return nil

	case event.Key() == tcell.KeyF1 && event.Modifiers()&tcell.ModAlt != 0:
		a.driveDialog(a.leftPanel)
		return nil

	case event.Key() == tcell.KeyF2 && event.Modifiers()&tcell.ModAlt != 0:
		a.driveDialog(a.rightPanel)
		return nil

	case event.Key() == tcell.KeyF2:
		a.rereadFocusedPanel()
		return nil

	case event.Key() == tcell.KeyF5 && event.Modifiers()&tcell.ModShift != 0:
		a.duplicateDialog()
		return nil

	case event.Key() == tcell.KeyF5:
		a.copyDialog()
		return nil

	case event.Key() == tcell.KeyF6 && event.Modifiers()&tcell.ModShift != 0:
		a.renameDialog()
		return nil

	case event.Key() == tcell.KeyF6:
		a.moveRenameDialog()
		return nil

	case event.Key() == tcell.KeyF7:
		a.mkdirDialog()
		return nil

	case event.Key() == tcell.KeyF8 || event.Key() == tcell.KeyDelete:
		a.deleteDialog()
		return nil

	case event.Key() == tcell.KeyEscape:
		// Total Commander: Esc unselects everything when there is a
		// selection; with none it does nothing (F10 quits).
		if clearSelectionIfAny(a.focusedPanel()) {
			a.render()
			return nil
		}

	case event.Key() == tcell.KeyF10:
		a.tviewApp.Stop()
		return nil

	case event.Key() == tcell.KeyInsert:
		a.toggleSelectAndAdvance()
		return nil

	case event.Key() == tcell.KeyRune && event.Rune() == ' ' &&
		event.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0:
		a.toggleSelectCurrent()
		return nil

	case event.Key() == tcell.KeyRune && event.Rune() == '+' &&
		event.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0:
		a.selectMaskDialog(true)
		return nil

	case event.Key() == tcell.KeyRune && event.Rune() == '-' &&
		event.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0:
		a.selectMaskDialog(false)
		return nil

	case event.Key() == tcell.KeyRune && event.Rune() == '*' &&
		event.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0:
		a.invertSelection()
		return nil

	case event.Key() == tcell.KeyRune &&
		event.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0 &&
		isSearchRune(event.Rune()):
		a.startSearch(string(event.Rune()))
		return nil
	}

	return event
}

// isSearchRune reports whether a typed rune starts/extends the quick
// search. Control characters and the bound selection keys (space, and the
// +/-/* group keys) are excluded; everything else printable qualifies.
func isSearchRune(r rune) bool {
	if r < ' ' || r == 0x7f {
		return false
	}
	switch r {
	case ' ', '+', '-', '*':
		return false
	}
	return true
}

// startSearch opens the quick search window seeded with the typed rune,
// records the cursor position so Esc can restore it, and narrows the focused
// panel's listing to matching entries.
func (a *App) startSearch(r string) {
	a.searchMode = true
	a.searchOrigin = a.focusedPanel().Cursor()
	a.searchField.SetText(r)
	a.focusedPanel().SetFilter(r)
	a.pages.ShowPage("search")
	a.render()
}

// handleSearchKey processes a key while the quick search window is open.
// It returns nil when the key was consumed, or the original event when the
// search was cancelled by that key so the caller can process it normally.
func (a *App) handleSearchKey(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case event.Key() == tcell.KeyEscape:
		a.cancelSearch()
		return nil

	case event.Key() == tcell.KeyEnter:
		a.commitSearch()
		return nil

	case event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2:
		runes := []rune(a.searchField.GetText())
		if len(runes) > 0 {
			runes = runes[:len(runes)-1]
		}
		a.setSearchText(string(runes))
		return nil

	case event.Key() == tcell.KeyRune &&
		event.Modifiers()&(tcell.ModCtrl|tcell.ModAlt) == 0 &&
		isSearchRune(event.Rune()):
		a.setSearchText(a.searchField.GetText() + string(event.Rune()))
		return nil
	}

	// Any other key cancels the search, then acts normally on the
	// restored listing.
	a.cancelSearch()
	return event
}

// setSearchText updates the search field text and the focused panel's
// filter, then re-renders.
func (a *App) setSearchText(text string) {
	a.searchField.SetText(text)
	a.focusedPanel().SetFilter(text)
	a.render()
}

// cancelSearch clears the filter, restores the cursor to where it was when
// the search started, and hides the search window.
func (a *App) cancelSearch() {
	panel := a.focusedPanel()
	panel.ClearFilter()
	panel.SetCursor(a.searchOrigin)
	a.searchMode = false
	a.pages.HidePage("search")
	a.render()
}

// commitSearch puts the cursor on the first match in the full listing and
// hides the search window.
func (a *App) commitSearch() {
	panel := a.focusedPanel()
	idx := panel.FirstMatchIndex()
	panel.ClearFilter()
	if idx >= 0 {
		panel.SetCursor(idx)
	}
	a.searchMode = false
	a.pages.HidePage("search")
	a.render()
}

// switchFocus toggles keyboard focus between the left and right panels.
func (a *App) switchFocus() {
	a.focusIndex = 1 - a.focusIndex
	a.render()
}

// rereadFocusedPanel reloads the focused panel (TC standard F2).
func (a *App) rereadFocusedPanel() {
	panel := a.focusedPanel()
	if err := panel.Load(panel.CurrentPath()); err != nil {
		a.ShowErrorDialog("Error", fmt.Sprintf("Cannot reload panel: %v", err), nil)
	}
	a.render()
}

// driveDialog shows a volume list and loads the chosen root into the given
// panel (Total Commander Alt+F1 / Alt+F2).
func (a *App) driveDialog(panel *core.Panel) {
	volumes := core.Volumes()
	if len(volumes) == 0 {
		return
	}

	items := make([]string, len(volumes))
	for i, v := range volumes {
		items[i] = v.Name
	}

	a.ShowListDialog("Drive", items, func(index int, _ string) {
		if index < 0 || index >= len(volumes) {
			return
		}
		// Choosing a drive makes that panel the active one (Total
		// Commander behaviour): Alt+F1 targets the left panel, Alt+F2 the
		// right one.
		if panel == a.leftPanel {
			a.focusIndex = 0
		} else {
			a.focusIndex = 1
		}
		if err := panel.Load(volumes[index].Path); err != nil {
			a.ShowErrorDialog("Error", fmt.Sprintf("Cannot load drive: %v", err), nil)
			return
		}
		a.render()
	})
}

// focusedPanel returns the panel that currently has keyboard focus.
func (a *App) focusedPanel() *core.Panel {
	if a.focusIndex == 0 {
		return a.leftPanel
	}
	return a.rightPanel
}

// moveCursor shifts the cursor by delta, clamped to valid bounds.
func (a *App) moveCursor(delta int) {
	panel := a.focusedPanel()
	newCursor := panel.Cursor() + delta
	if newCursor < 0 {
		newCursor = 0
	}
	if len(panel.Entries()) > 0 && newCursor >= len(panel.Entries()) {
		newCursor = len(panel.Entries()) - 1
	}
	panel.SetCursor(newCursor)
	a.render()
}

// setCursor moves the cursor to the given index, clamped to valid bounds.
func (a *App) setCursor(idx int) {
	panel := a.focusedPanel()
	if idx < 0 {
		idx = 0
	}
	if len(panel.Entries()) > 0 && idx >= len(panel.Entries()) {
		idx = len(panel.Entries()) - 1
	}
	panel.SetCursor(idx)
	a.render()
}

// enter descends into the directory under the cursor on the focused panel.
func (a *App) enter() {
	panel := a.focusedPanel()
	entries := panel.Entries()
	if len(entries) == 0 {
		return
	}
	entry := entries[panel.Cursor()]
	if !entry.IsDir {
		return
	}
	if err := panel.Descend(entry.Name); err != nil {
		return
	}
	a.render()
}

// parentDir navigates the focused panel to its parent directory.
func (a *App) parentDir() {
	panel := a.focusedPanel()
	parent, err := panel.Up()
	if err != nil {
		return
	}
	if err := panel.Load(parent); err != nil {
		return
	}
	a.render()
}

// mkdirDialog shows a form dialog to create a new directory under the
// focused panel's current path. On success the panel is reread.
func (a *App) mkdirDialog() {
	panel := a.focusedPanel()

	input := tview.NewInputField().
		SetLabel("Directory name: ").
		SetFieldWidth(30)

	a.ShowFormDialog("New directory", []tview.FormItem{input}, 0, func(ok bool) {
		if !ok {
			return
		}
		name := input.GetText()
		if err := core.MakeDir(panel.CurrentPath(), name); err != nil {
			a.ShowErrorDialog("Error", fmt.Sprintf("Cannot create directory: %v", err), nil)
		} else {
			if err := panel.Load(panel.CurrentPath()); err != nil {
				a.ShowErrorDialog("Error", fmt.Sprintf("Cannot reload panel: %v", err), nil)
			}
			a.render()
		}
	})
}

// deleteDialog shows a confirmation dialog for deleting the focused panel's
// selection set (or the cursor entry when nothing is selected). A
// multi-selection is confirmed by item count only. On confirmation the
// deletion runs in a goroutine with a progress modal; Esc cancels it.
func (a *App) deleteDialog() {
	panel := a.focusedPanel()
	sources := a.selectedEntries(panel)
	if len(sources) == 0 {
		return
	}

	confirm := fmt.Sprintf("Delete %s? (permanent)", sources[0].Name)
	if len(sources) > 1 {
		confirm = fmt.Sprintf("Delete %d items? (permanent)", len(sources))
	}

	a.ShowFormDialog("Delete", []tview.FormItem{
		tview.NewTextView().SetText(confirm),
	}, 2, func(ok bool) {
		if !ok {
			return
		}
		a.runDelete(panel, sources)
	})
}

// runDelete executes a delete queue for the given entries, showing progress
// and error modals as appropriate.
func (a *App) runDelete(panel *core.Panel, entries []core.Entry) {
	if len(entries) == 0 {
		return
	}
	path := panel.CurrentPath()
	items := make([]string, len(entries))
	for i, e := range entries {
		items[i] = filepath.Join(path, e.Name)
	}

	queue := core.NewDeleteQueue(items, core.DeleteOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	a.activeCancel = cancel

	eventCh := queue.Run(ctx)
	runner := NewOperationRunner(eventCh)

	progressTitle := opLabel("Deleting", entries[0].Name, len(entries))
	updateProgress, closeProgress := a.ShowProgressDialog(progressTitle)

	runner.
		OnProgress(func(currentName string, fileBytes, fileSize int64, done, total int, doneBytes, totalBytes int64, eta ProgressETA) {
			updateProgress(currentName, fileBytes, fileSize, done, total, doneBytes, totalBytes, eta)
		}).
		OnDone(func(result OperationResult) {
			a.activeCancel = nil
			closeProgress()
			// Reread the focused panel.
			if err := panel.Load(panel.CurrentPath()); err != nil {
				a.ShowErrorDialog("Error", fmt.Sprintf("Cannot reload panel: %v", err), nil)
			}
			a.render()
			// Show errors if any.
			if len(result.Errors) > 0 {
				var sb strings.Builder
				for _, err := range result.Errors {
					fmt.Fprintf(&sb, "- %v\n", err)
				}
				a.ShowErrorDialog("Delete errors", sb.String(), nil)
			}
		})

	runner.Run(a)
}

// selectedEntries returns the panel's explicitly selected entries. When
// nothing is explicitly selected it falls back to the cursor entry, which
// is how Total Commander treats "no selection" (operate on the cursor).
// Copy (F5), move (F6) and delete (F8/Delete) operate on this set.
func (a *App) selectedEntries(panel *core.Panel) []core.Entry {
	selected := panel.SelectedEntries()
	if len(selected) > 0 {
		return selected
	}
	entries := panel.Entries()
	if len(entries) == 0 {
		return nil
	}
	return []core.Entry{entries[panel.Cursor()]}
}

// toggleSelectAndAdvance toggles the cursor entry's selection and then
// advances the cursor by one (Insert semantics, Total Commander style).
func (a *App) toggleSelectAndAdvance() {
	a.focusedPanel().ToggleSelectAtCursor()
	a.moveCursor(1)
	a.render()
}

// toggleSelectCurrent toggles the cursor entry's selection without moving
// the cursor (Space, Total Commander style).
func (a *App) toggleSelectCurrent() {
	a.focusedPanel().ToggleSelectAtCursor()
	a.render()
}

// clearSelectionIfAny clears the panel's selection when one exists and
// reports whether it did. Total Commander unselects everything on Esc
// (instead of acting on the cursor); with no selection Esc stays a no-op so
// F10 remains the only quit key. Kept render-free so it is unit-testable
// without a running tview application.
func clearSelectionIfAny(panel *core.Panel) bool {
	if panel.SelectionCount() == 0 {
		return false
	}
	panel.ClearSelection()
	return true
}

// selectMaskDialog prompts for a wildcard mask and applies it to the
// focused panel: selecting matching entries when selectMode is true,
// deselecting them otherwise. A malformed mask shows an error dialog.
func (a *App) selectMaskDialog(selectMode bool) {
	panel := a.focusedPanel()

	input := tview.NewInputField().
		SetLabel("Mask: ").
		SetFieldWidth(30).
		SetText("*")

	title := "Select group"
	if !selectMode {
		title = "Deselect group"
	}

	a.ShowFormDialog(title, []tview.FormItem{input}, 0, func(ok bool) {
		if !ok {
			return
		}
		mask := input.GetText()
		var err error
		if selectMode {
			_, err = panel.SelectMask(mask)
		} else {
			_, err = panel.DeselectMask(mask)
		}
		if err != nil {
			a.ShowErrorDialog("Error", fmt.Sprintf("Invalid mask %q: %v", mask, err), nil)
			return
		}
		a.render()
	})
}

// invertSelection inverts the selection of all visible entries in the
// focused panel (Total Commander '*').
func (a *App) invertSelection() {
	a.focusedPanel().InvertSelection()
	a.render()
}

// otherPanel returns the panel that is not the given one.
func (a *App) otherPanel(panel *core.Panel) *core.Panel {
	if panel == a.leftPanel {
		return a.rightPanel
	}
	return a.leftPanel
}

// opLabel returns a progress-dialog title: "<verb> <name>" for a single
// item, or "<verb> <n> items" for a multi-selection. Multi-selection titles
// never list names, only the item count.
func opLabel(verb, name string, n int) string {
	if n > 1 {
		return fmt.Sprintf("%s %d items", verb, n)
	}
	return fmt.Sprintf("%s %s", verb, name)
}

// targetPaths returns one destination per source. For a single source, dest
// is the full target path (rename-on-copy); for many, dest is the target
// directory and each source keeps its own name.
func targetPaths(dest string, sources []core.Entry) []string {
	paths := make([]string, len(sources))
	if len(sources) == 1 {
		paths[0] = dest
		return paths
	}
	for i, e := range sources {
		paths[i] = filepath.Join(dest, e.Name)
	}
	return paths
}

// overwritePrompt returns the overwrite-confirmation text: the destination
// path for one item, or "Overwrite <n> items?" for many. Multi-selection
// prompts never list names, only the item count.
func overwritePrompt(dest string, n int) string {
	if n > 1 {
		return fmt.Sprintf("Overwrite %d items?", n)
	}
	return fmt.Sprintf("Overwrite %s?", dest)
}

// anyTargetExists reports whether at least one destination path for the
// selection already exists on disk.
func anyTargetExists(dest string, sources []core.Entry) bool {
	for _, p := range targetPaths(dest, sources) {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// copyDialog shows a form dialog to copy the focused panel's selection set
// (or the cursor entry when nothing is selected). For a single source the
// destination pre-fills to the full target path (the user may rename while
// copying); for a multi-selection it pre-fills to the other panel's
// directory and each source keeps its name. On OK it confirms overwrite if
// any destination exists, then runs CopyQueue.
func (a *App) copyDialog() {
	panel := a.focusedPanel()
	sources := a.selectedEntries(panel)
	if len(sources) == 0 {
		return
	}

	other := a.otherPanel(panel)
	destPath := other.CurrentPath()
	if len(sources) == 1 {
		destPath = filepath.Join(other.CurrentPath(), sources[0].Name)
	}

	input := tview.NewInputField().
		SetLabel("Destination: ").
		SetFieldWidth(50).
		SetText(destPath)

	a.ShowFormDialog("Copy", []tview.FormItem{input}, 0, func(ok bool) {
		if !ok {
			return
		}
		dest := input.GetText()
		a.confirmOverwriteAndCopy(panel, sources, dest)
	})
}

// duplicateDialog shows a form dialog to duplicate the cursor
// entry in the same directory with a " - Copy" suffix.
func (a *App) duplicateDialog() {
	panel := a.focusedPanel()
	entries := panel.Entries()
	if len(entries) == 0 {
		return
	}
	entry := entries[panel.Cursor()]
	name := entry.Name

	ext := filepath.Ext(name)
	stem := name[:len(name)-len(ext)]
	dupName := stem + " - Copy" + ext

	destPath := filepath.Join(panel.CurrentPath(), dupName)

	input := tview.NewInputField().
		SetLabel("Destination: ").
		SetFieldWidth(50).
		SetText(destPath)

	a.ShowFormDialog("Duplicate", []tview.FormItem{input}, 0, func(ok bool) {
		if !ok {
			return
		}
		dest := input.GetText()
		a.confirmOverwriteAndCopy(panel, []core.Entry{entry}, dest)
	})
}

// confirmOverwriteAndCopy checks whether any destination for the selection
// already exists and, if so, asks for one count-aware overwrite confirmation
// before running the copy operation.
func (a *App) confirmOverwriteAndCopy(sourcePanel *core.Panel, sources []core.Entry, dest string) {
	if anyTargetExists(dest, sources) {
		a.ShowFormDialog("Destination exists. Overwrite?", []tview.FormItem{
			tview.NewTextView().SetText(overwritePrompt(dest, len(sources))),
		}, 2, func(ok bool) {
			if !ok {
				return
			}
			a.runCopy(sourcePanel, sources, dest, true)
		})
		return
	}
	a.runCopy(sourcePanel, sources, dest, false)
}

// runCopy executes a copy queue for the given entries, showing
// progress and error modals as appropriate.
func (a *App) runCopy(sourcePanel *core.Panel, sources []core.Entry, dest string, overwrite bool) {
	if len(sources) == 0 {
		return
	}
	sourcePath := sourcePanel.CurrentPath()
	paths := targetPaths(dest, sources)
	items := make([]core.CopyItem, len(sources))
	for i, e := range sources {
		items[i] = core.CopyItem{
			Source: filepath.Join(sourcePath, e.Name),
			Dest:   paths[i],
		}
	}

	queue := core.NewCopyQueue(items, core.CopyOptions{Overwrite: overwrite})

	ctx, cancel := context.WithCancel(context.Background())
	a.activeCancel = cancel

	eventCh := queue.Run(ctx)
	runner := NewOperationRunner(eventCh)

	progressTitle := opLabel("Copying", sources[0].Name, len(sources))
	updateProgress, closeProgress := a.ShowProgressDialog(progressTitle)

	runner.
		OnProgress(func(currentName string, fileBytes, fileSize int64, done, total int, doneBytes, totalBytes int64, eta ProgressETA) {
			updateProgress(currentName, fileBytes, fileSize, done, total, doneBytes, totalBytes, eta)
		}).
		OnDone(func(result OperationResult) {
			a.activeCancel = nil
			closeProgress()
			// Reread the other panel (destination).
			other := a.otherPanel(sourcePanel)
			if err := other.Load(other.CurrentPath()); err != nil {
				a.ShowErrorDialog("Error", fmt.Sprintf("Cannot reload panel: %v", err), nil)
			}
			a.render()
			// Show errors if any.
			if len(result.Errors) > 0 {
				var sb strings.Builder
				for _, err := range result.Errors {
					fmt.Fprintf(&sb, "- %v\n", err)
				}
				a.ShowErrorDialog("Copy errors", sb.String(), nil)
			}
		})

	runner.Run(a)
}

// moveRenameDialog shows a form dialog to move or rename the focused panel's
// selection set (or the cursor entry when nothing is selected). For a single
// source the destination pre-fills to the full target path (rename-on-move);
// for a multi-selection it pre-fills to the other panel's directory and each
// source keeps its name. On OK it dispatches to the rename or move path.
func (a *App) moveRenameDialog() {
	panel := a.focusedPanel()
	sources := a.selectedEntries(panel)
	if len(sources) == 0 {
		return
	}

	other := a.otherPanel(panel)
	destPath := other.CurrentPath()
	if len(sources) == 1 {
		destPath = filepath.Join(other.CurrentPath(), sources[0].Name)
	}

	input := tview.NewInputField().
		SetLabel("Destination: ").
		SetFieldWidth(50).
		SetText(destPath)

	a.ShowFormDialog("Move/Rename", []tview.FormItem{input}, 0, func(ok bool) {
		if !ok {
			return
		}
		dest := input.GetText()
		a.dispatchMove(panel, sources, dest)
	})
}

// dispatchMove routes a move/rename request for the selection set. A single
// source keeps the same-directory rename fast path and the cross-directory
// overwrite confirmation. A multi-selection always moves every source into
// the target directory; if the target directory is the source panel's own
// directory the request is a no-op (reload and return) so items are never
// moved onto themselves.
func (a *App) dispatchMove(sourcePanel *core.Panel, sources []core.Entry, dest string) {
	if len(sources) == 0 {
		return
	}

	if len(sources) == 1 {
		entry := sources[0]
		sameDir := core.SameDirectory(
			filepath.Dir(filepath.Join(sourcePanel.CurrentPath(), entry.Name)),
			filepath.Dir(dest))
		// Cross-directory moves onto an existing destination must be
		// confirmed; same-dir renames fail with an error instead.
		if !sameDir && anyTargetExists(dest, sources) {
			a.ShowFormDialog("Destination exists. Overwrite?", []tview.FormItem{
				tview.NewTextView().SetText(overwritePrompt(dest, 1)),
			}, 2, func(ok bool) {
				if !ok {
					return
				}
				a.runMoveOrRename(sourcePanel, entry, dest, true)
			})
			return
		}
		a.runMoveOrRename(sourcePanel, entry, dest, false)
		return
	}

	// Multi-selection: dest is the target directory. A move onto the source
	// panel's own directory is a no-op.
	if core.SameDirectory(sourcePanel.CurrentPath(), dest) {
		if err := sourcePanel.Load(sourcePanel.CurrentPath()); err != nil {
			a.ShowErrorDialog("Error", fmt.Sprintf("Cannot reload panel: %v", err), nil)
		}
		a.render()
		return
	}

	if anyTargetExists(dest, sources) {
		a.ShowFormDialog("Destination exists. Overwrite?", []tview.FormItem{
			tview.NewTextView().SetText(overwritePrompt(dest, len(sources))),
		}, 2, func(ok bool) {
			if !ok {
				return
			}
			a.runMoveQueue(sourcePanel, sources, dest, true)
		})
		return
	}
	a.runMoveQueue(sourcePanel, sources, dest, false)
}

// renameDialog shows a form dialog to rename the cursor entry
// in-place. The destination pre-fills to the same directory
// with the same name.
func (a *App) renameDialog() {
	panel := a.focusedPanel()
	entries := panel.Entries()
	if len(entries) == 0 {
		return
	}
	entry := entries[panel.Cursor()]

	destPath := filepath.Join(panel.CurrentPath(), entry.Name)

	input := tview.NewInputField().
		SetLabel("New name: ").
		SetFieldWidth(50).
		SetText(destPath)

	a.ShowFormDialog("Rename", []tview.FormItem{input}, 0, func(ok bool) {
		if !ok {
			return
		}
		dest := input.GetText()
		a.runRename(panel, entry, dest)
	})
}

// runMoveOrRename dispatches a single source to RenamePath for
// same-directory moves or to the move queue for cross-directory moves.
func (a *App) runMoveOrRename(sourcePanel *core.Panel, entry core.Entry, dest string, overwrite bool) {
	sourcePath := filepath.Join(sourcePanel.CurrentPath(), entry.Name)

	if core.SameDirectory(filepath.Dir(sourcePath), filepath.Dir(dest)) {
		a.runRename(sourcePanel, entry, dest)
		return
	}

	a.runMoveQueue(sourcePanel, []core.Entry{entry}, dest, overwrite)
}

// runMoveQueue executes a MoveQueue for the whole selection set, showing
// progress and error modals as appropriate, then rereads both panels.
func (a *App) runMoveQueue(sourcePanel *core.Panel, sources []core.Entry, dest string, overwrite bool) {
	if len(sources) == 0 {
		return
	}
	sourcePath := sourcePanel.CurrentPath()
	paths := targetPaths(dest, sources)
	items := make([]core.MoveItem, len(sources))
	for i, e := range sources {
		items[i] = core.MoveItem{
			Source: filepath.Join(sourcePath, e.Name),
			Dest:   paths[i],
		}
	}

	queue := core.NewMoveQueue(items, core.MoveOptions{Overwrite: overwrite})

	ctx, cancel := context.WithCancel(context.Background())
	a.activeCancel = cancel

	eventCh := queue.Run(ctx)
	runner := NewOperationRunner(eventCh)

	progressTitle := opLabel("Moving", sources[0].Name, len(sources))
	updateProgress, closeProgress := a.ShowProgressDialog(progressTitle)

	runner.
		OnProgress(func(currentName string, fileBytes, fileSize int64, done, total int, doneBytes, totalBytes int64, eta ProgressETA) {
			updateProgress(currentName, fileBytes, fileSize, done, total, doneBytes, totalBytes, eta)
		}).
		OnDone(func(result OperationResult) {
			a.activeCancel = nil
			closeProgress()
			// Reread both panels.
			if err := sourcePanel.Load(sourcePanel.CurrentPath()); err != nil {
				a.ShowErrorDialog("Error", fmt.Sprintf("Cannot reload panel: %v", err), nil)
			}
			other := a.otherPanel(sourcePanel)
			if err := other.Load(other.CurrentPath()); err != nil {
				a.ShowErrorDialog("Error", fmt.Sprintf("Cannot reload panel: %v", err), nil)
			}
			a.render()
			// Show errors if any.
			if len(result.Errors) > 0 {
				var sb strings.Builder
				for _, err := range result.Errors {
					fmt.Fprintf(&sb, "- %v\n", err)
				}
				a.ShowErrorDialog("Move errors", sb.String(), nil)
			}
		})

	runner.Run(a)
}

// runRename calls core.RenamePath for same-directory rename
// (no progress modal). On success both panels are reread.
func (a *App) runRename(sourcePanel *core.Panel, entry core.Entry, dest string) {
	sourcePath := filepath.Join(sourcePanel.CurrentPath(), entry.Name)
	newName := filepath.Base(dest)

	err := core.RenamePath(sourcePath, newName)
	if err != nil {
		a.ShowErrorDialog("Error", fmt.Sprintf("Cannot rename: %v", err), nil)
		return
	}

	// Reread both panels.
	if err := sourcePanel.Load(sourcePanel.CurrentPath()); err != nil {
		a.ShowErrorDialog("Error", fmt.Sprintf("Cannot reload panel: %v", err), nil)
	}
	other := a.otherPanel(sourcePanel)
	if err := other.Load(other.CurrentPath()); err != nil {
		a.ShowErrorDialog("Error", fmt.Sprintf("Cannot reload panel: %v", err), nil)
	}
	a.render()
}

package ui

import (
	"context"
	"fmt"
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
	a.keyBar.SetText("Tab Panel  F5 Copy  F6 Move  F7 MkDir  F8 Del  F10 Quit")

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

	// Title shows the current path
	view.SetTitle(fmt.Sprintf(" %s ", panel.CurrentPath()))

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

		line := fmt.Sprintf("%-30s  %10s  %s", e.Name, sizeStr, mtimeStr)
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

	case event.Key() == tcell.KeyF7:
		a.mkdirDialog()
		return nil

	case event.Key() == tcell.KeyF8 || event.Key() == tcell.KeyDelete:
		a.deleteDialog()
		return nil

	case event.Key() == tcell.KeyF10:
		a.tviewApp.Stop()
		return nil
	}

	return event
}

// switchFocus toggles keyboard focus between the left and right panels.
func (a *App) switchFocus() {
	a.focusIndex = 1 - a.focusIndex
	a.render()
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

	a.ShowFormDialog("New directory", []tview.FormItem{input}, func(ok bool) {
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

// deleteDialog shows a confirmation dialog for deleting the cursor entry
// of the focused panel. On confirmation the deletion runs in a goroutine
// with a progress modal; Esc cancels the operation.
func (a *App) deleteDialog() {
	panel := a.focusedPanel()
	entries := panel.Entries()
	if len(entries) == 0 {
		return
	}
	entry := entries[panel.Cursor()]
	name := entry.Name

	a.ShowFormDialog("Delete", []tview.FormItem{
		tview.NewTextView().SetText(fmt.Sprintf("Delete %s? (permanent)", name)),
	}, func(ok bool) {
		if !ok {
			return
		}
		a.runDelete(panel, entry)
	})
}

// runDelete executes a delete queue for the given entry, showing progress
// and error modals as appropriate.
func (a *App) runDelete(panel *core.Panel, entry core.Entry) {
	path := panel.CurrentPath()
	fullPath := filepath.Join(path, entry.Name)

	queue := core.NewDeleteQueue([]string{fullPath}, core.DeleteOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	a.activeCancel = cancel

	eventCh := queue.Run(ctx)
	runner := NewOperationRunner(eventCh)

	progressTitle := "Deleting " + entry.Name
	updateProgress, closeProgress := a.ShowProgressDialog(progressTitle)

	runner.
		OnProgress(func(done, total int) {
			updateProgress(done, total)
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

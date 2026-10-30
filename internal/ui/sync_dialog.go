package ui

import (
	"context"
	"fmt"

	"github.com/rivo/tview"
	"tui-commander/internal/core"
)

// syncDialog opens the directory comparison/sync dialog (Shift+F2).
func (a *App) syncDialog() {
	left := a.leftPanel
	right := a.rightPanel

	leftPath := left.CurrentPath()
	rightPath := right.CurrentPath()

	// Initial comparison in Real mode
	result, err := core.CompareDirectories(leftPath, rightPath, core.DefaultSyncOptions())
	if err != nil {
		a.ShowErrorDialog("Error", fmt.Sprintf("Cannot compare directories: %v", err), nil)
		return
	}

	a.showSyncResult(result, leftPath, rightPath)
}

// showSyncResult displays the comparison result and lets the user choose mode and actions.
func (a *App) showSyncResult(result *core.CompareResult, leftPath, rightPath string) {
	// Reclassify with current mode
	result.ClassifyEntries(result.Mode)

	form := tview.NewForm()

	// Mode selector
	modeIndex := 0
	if result.Mode == core.SyncMirror {
		modeIndex = 1
	}
	form.AddDropDown("Mode", []string{"Real (bidirectional)", "Mirror (left -> right)"}, modeIndex, func(option string, optionIndex int) {
		if optionIndex == 0 {
			result.ClassifyEntries(core.SyncReal)
		} else {
			result.ClassifyEntries(core.SyncMirror)
		}
		a.showSyncResult(result, leftPath, rightPath) // Refresh
	})

	// Summary
	summary := fmt.Sprintf("Left: %s\nRight: %s\nTotal: %d  Skip: %d  Copy L->R: %d  Copy R->L: %d  Delete: %d  Conflicts: %d",
		leftPath, rightPath, result.TotalItems,
		result.Actions[core.ActionSkip],
		result.Actions[core.ActionCopyLeftToRight],
		result.Actions[core.ActionCopyRightToLeft],
		result.Actions[core.ActionDeleteRight]+result.Actions[core.ActionDeleteLeft],
		result.Actions[core.ActionConflict])
	form.AddFormItem(tview.NewTextView().SetText(summary).SetDynamicColors(true))

	form.AddButton("Execute Sync", func() {
		if result.Actions[core.ActionConflict] > 0 {
			a.ShowConfirmDialog("Conflicts", "There are unresolved conflicts. Proceed anyway?", "Yes", "No", true, func(ok bool) {
				if ok {
					a.executeSync(result, leftPath, rightPath)
				}
			})
			return
		}
		a.executeSync(result, leftPath, rightPath)
	})
	form.AddButton("Cancel", func() { a.closeTopDialog() })
	form.SetBorder(true).SetTitle(" Sync: Compare Directories ").SetTitleAlign(tview.AlignCenter)
	a.pages.AddPage("sync", a.wrapDialog(form, 20), true, true)
	a.dialogStack = append(a.dialogStack, "sync")
	a.tviewApp.SetFocus(form)
}

// executeSync executes the sync plan with a progress dialog.
func (a *App) executeSync(result *core.CompareResult, leftPath, rightPath string) {
	a.closeTopDialog()

	update, closeProgress := a.ShowProgressDialog("Syncing")
	_, cancel := context.WithCancel(context.Background())
	a.operationCancel["sync"] = cancel

	go func() {
		err := core.ExecuteSync(result, func(e core.SyncEntry, current, total int) {
			a.tviewApp.QueueUpdateDraw(func() {
				update(e.RelPath, 0, 0, current+1, total, 0, 0, ProgressETA{})
			})
		})

		a.tviewApp.QueueUpdateDraw(func() {
			closeProgress()
			delete(a.operationCancel, "sync")
			if err != nil {
				a.ShowErrorDialog("Sync Error", fmt.Sprintf("Sync failed: %v", err), nil)
			} else {
				a.ShowMessageDialog("Sync Complete", "Directory synchronization completed.", nil)
			}
			a.leftPanel.Load(a.leftPanel.CurrentPath())
			a.rightPanel.Load(a.rightPanel.CurrentPath())
			a.render()
		})
	}()
}

// syncActionString returns a human-readable string for a sync action.
func syncActionString(a core.SyncAction) string {
	switch a {
	case core.ActionSkip:
		return "=="
	case core.ActionCopyLeftToRight:
		return "L->R"
	case core.ActionCopyRightToLeft:
		return "R->L"
	case core.ActionDeleteRight:
		return "DEL R"
	case core.ActionDeleteLeft:
		return "DEL L"
	case core.ActionConflict:
		return "CONFLICT"
	default:
		return "?"
	}
}
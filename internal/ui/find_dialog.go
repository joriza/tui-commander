package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rivo/tview"
	"tui-commander/internal/core"
)

// findDialog opens the Find Files dialog.
func (a *App) findDialog() {
	panel := a.focusedPanel()
	startPath := panel.CurrentPath()

	form := tview.NewForm()

	// Search root
	rootInput := tview.NewInputField().
		SetLabel("Search in: ").
		SetText(startPath).
		SetFieldWidth(50)
	form.AddFormItem(rootInput)

	// Name pattern
	nameInput := tview.NewInputField().
		SetLabel("Name pattern: ").
		SetText("*").
		SetFieldWidth(30)
	form.AddFormItem(nameInput)

	// Mode dropdown
	form.AddDropDown("Type", []string{"Files & Dirs", "Files only", "Dirs only"}, 0, func(option string, optionIndex int) {
		// Mode is read from the form on submit
	})

	// Size range
	minSizeInput := tview.NewInputField().
		SetLabel("Min size (bytes): ").
		SetText("").
		SetFieldWidth(20)
	maxSizeInput := tview.NewInputField().
		SetLabel("Max size (bytes): ").
		SetText("").
		SetFieldWidth(20)
	form.AddFormItem(minSizeInput)
	form.AddFormItem(maxSizeInput)

	// Date range
	dateAfterInput := tview.NewInputField().
		SetLabel("Modified after (YYYY-MM-DD): ").
		SetText("").
		SetFieldWidth(30)
	dateBeforeInput := tview.NewInputField().
		SetLabel("Modified before (YYYY-MM-DD): ").
		SetText("").
		SetFieldWidth(30)
	form.AddFormItem(dateAfterInput)
	form.AddFormItem(dateBeforeInput)

	// Content search
	contentInput := tview.NewInputField().
		SetLabel("Content contains: ").
		SetText("").
		SetFieldWidth(30)
	caseSensitive := tview.NewCheckbox().
		SetLabel("Case sensitive").
		SetChecked(false)
	form.AddFormItem(contentInput)
	form.AddFormItem(caseSensitive)

	form.AddButton("Find", func() {
		root := strings.TrimSpace(rootInput.GetText())
		if root == "" {
			root = startPath
		}

		opts := core.FindOptions{
			NamePattern:   nameInput.GetText(),
			Content:       contentInput.GetText(),
			CaseSensitive: caseSensitive.IsChecked(),
		}

		// Parse size
		if minStr := strings.TrimSpace(minSizeInput.GetText()); minStr != "" {
			if v, err := strconv.ParseInt(minStr, 10, 64); err == nil {
				opts.MinSize = v
			}
		}
		if maxStr := strings.TrimSpace(maxSizeInput.GetText()); maxStr != "" {
			if v, err := strconv.ParseInt(maxStr, 10, 64); err == nil {
				opts.MaxSize = v
			}
		}

		// Parse dates
		if dateStr := strings.TrimSpace(dateAfterInput.GetText()); dateStr != "" {
			if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				opts.ModTimeAfter = &t
			}
		}
		if dateStr := strings.TrimSpace(dateBeforeInput.GetText()); dateStr != "" {
			if t, err := time.Parse("2006-01-02", dateStr); err == nil {
				opts.ModTimeBefore = &t
			}
		}

		// Determine mode
		modeIndex, _ := form.GetFormItem(2).(*tview.DropDown).GetCurrentOption()
		switch modeIndex {
		case 0:
			opts.Mode = core.FindBoth
		case 1:
			opts.Mode = core.FindFilesOnly
		case 2:
			opts.Mode = core.FindDirs
		}

		a.closeTopDialog()
		a.executeFind(root, opts)
	})
	form.AddButton("Cancel", func() { a.closeTopDialog() })
	form.SetBorder(true).SetTitle(" Find Files ").SetTitleAlign(tview.AlignCenter)
	a.pages.AddPage("find", a.wrapDialog(form, 20), true, true)
	a.dialogStack = append(a.dialogStack, "find")
	a.tviewApp.SetFocus(form)
}

// executeFind runs the search and shows results.
func (a *App) executeFind(root string, opts core.FindOptions) {
	_, closeProgress := a.ShowProgressDialog("Searching")
	_, cancel := context.WithCancel(context.Background())
	a.operationCancel["find"] = cancel

	go func() {
		results, err := core.FindFiles(root, opts)

		a.tviewApp.QueueUpdateDraw(func() {
			closeProgress()
			delete(a.operationCancel, "find")
			if err != nil {
				a.ShowErrorDialog("Find Error", fmt.Sprintf("Search failed: %v", err), nil)
				return
			}
			a.showFindResults(results)
		})
	}()
}

// showFindResults displays the search results in a list dialog.
func (a *App) showFindResults(results *core.FindResults) {
	if len(results.Matches) == 0 {
		a.ShowMessageDialog("Find Results", fmt.Sprintf("No matches found.\nScanned: %d items", results.Scanned), nil)
		return
	}

	list := tview.NewList().ShowSecondaryText(true)
	for _, m := range results.Matches {
		match := m
		icon := "📄"
		if m.IsDir {
			icon = "📁"
		} else if m.IsLink {
			icon = "🔗"
		}
		sizeStr := formatSize(m.Size)
		timeStr := m.ModTime.Format("2006-01-02 15:04")
		mainText := fmt.Sprintf("%s %s", icon, m.RelPath)
		secondaryText := fmt.Sprintf("%s  %s", sizeStr, timeStr)
		if m.ContentMatch {
			secondaryText += "  [content match]"
		}
		list.AddItem(mainText, secondaryText, 0, func() {
			// Navigate to the file's directory
			targetPanel := a.focusedPanel()
			dir := match.Path
			if !match.IsDir {
				dir = filepath.Dir(match.Path)
			}
			if err := targetPanel.Load(dir); err != nil {
				a.ShowErrorDialog("Error", fmt.Sprintf("Cannot navigate: %v", err), nil)
			} else {
				// Position cursor on the file
				entries := targetPanel.Entries()
				for i, e := range entries {
					if e.Name == match.Name {
						targetPanel.SetCursor(i)
						break
					}
				}
				a.closeTopDialog()
				a.render()
			}
		})
	}
	list.SetBorder(true).
		SetTitle(fmt.Sprintf(" Find Results: %d matches (scanned %d) ", len(results.Matches), results.Scanned)).
		SetTitleAlign(tview.AlignCenter)
	list.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		// Already handled by AddItem callback
	})

	a.pages.AddPage("findresults", a.wrapDialog(list, min(20, len(results.Matches)+4)), true, true)
	a.dialogStack = append(a.dialogStack, "findresults")
	a.tviewApp.SetFocus(list)
}
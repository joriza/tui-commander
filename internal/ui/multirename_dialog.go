package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rivo/tview"
	"tui-commander/internal/core"
)

// multirenameDialog opens the Multi-Rename dialog (Ctrl+M).
func (a *App) multirenameDialog() {
	panel := a.focusedPanel()
	sources := a.selectedEntries(panel)
	if len(sources) == 0 {
		// Use cursor entry if nothing selected
		entries := panel.Entries()
		if len(entries) == 0 {
			return
		}
		entry := entries[panel.Cursor()]
		sources = []core.Entry{entry}
	}

	if len(sources) == 0 {
		return
	}

	form := tview.NewForm()

	// Pattern input
	patternInput := tview.NewInputField().
		SetLabel("Pattern: ").
		SetText("[N]_[C:3][E]").
		SetFieldWidth(50)
	form.AddFormItem(patternInput)

	// Search/Replace
	searchInput := tview.NewInputField().
		SetLabel("Search: ").
		SetText("").
		SetFieldWidth(30)
	replaceInput := tview.NewInputField().
		SetLabel("Replace: ").
		SetText("").
		SetFieldWidth(30)
	useRegex := tview.NewCheckbox().
		SetLabel("Regex").
		SetChecked(false)
	form.AddFormItem(searchInput)
	form.AddFormItem(replaceInput)
	form.AddFormItem(useRegex)

	// Counter options
	counterStartInput := tview.NewInputField().
		SetLabel("Counter start: ").
		SetText("1").
		SetFieldWidth(10)
	counterStepInput := tview.NewInputField().
		SetLabel("Counter step: ").
		SetText("1").
		SetFieldWidth(10)
	counterDigitsInput := tview.NewInputField().
		SetLabel("Counter digits: ").
		SetText("3").
		SetFieldWidth(10)
	form.AddFormItem(counterStartInput)
	form.AddFormItem(counterStepInput)
	form.AddFormItem(counterDigitsInput)

	// Case options
	uppercase := tview.NewCheckbox().
		SetLabel("Uppercase").
		SetChecked(false)
	lowercase := tview.NewCheckbox().
		SetLabel("Lowercase").
		SetChecked(false)
	form.AddFormItem(uppercase)
	form.AddFormItem(lowercase)

	// Preview area
	previewText := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true)
	previewText.SetBorder(true).
		SetTitle(" Preview ").
		SetTitleAlign(tview.AlignCenter)

	// Function to update preview
	updatePreview := func() {
		pattern := patternInput.GetText()
		if pattern == "" {
			previewText.SetText("Enter a pattern to see preview...")
			return
		}

		opts := core.MultiRenameOptions{
			Search:    searchInput.GetText(),
			Replace:   replaceInput.GetText(),
			UseRegex:  useRegex.IsChecked(),
			Uppercase: uppercase.IsChecked(),
			Lowercase: lowercase.IsChecked(),
		}

		// Parse counter options
		if v, err := strconv.Atoi(counterStartInput.GetText()); err == nil {
			opts.CounterStart = v
		}
		if v, err := strconv.Atoi(counterStepInput.GetText()); err == nil {
			opts.CounterStep = v
		}
		if v, err := strconv.Atoi(counterDigitsInput.GetText()); err == nil {
			opts.CounterDigits = v
		}

		panel := a.focusedPanel()
		selectedEntries := a.selectedEntries(panel)
		if len(selectedEntries) == 0 {
			entries := panel.Entries()
			if len(entries) > 0 {
				selectedEntries = []core.Entry{entries[panel.Cursor()]}
			}
		}

		results, err := core.MultiRenamePreview(selectedEntries, panel.CurrentPath(), patternInput.GetText(), opts)
		if err != nil {
			previewText.SetText(fmt.Sprintf("[red]Error: %v[-]", err))
			return
		}

		var preview strings.Builder
		preview.WriteString(fmt.Sprintf("Total: %d items\n\n", len(results)))
		maxShow := 20
		for i, r := range results {
			if i >= maxShow {
				preview.WriteString(fmt.Sprintf("\n... and %d more", len(results)-maxShow))
				break
			}
			if r.Error != nil {
				preview.WriteString(fmt.Sprintf("[red]%s -> %s (ERROR: %v)[-]\n", r.OldName, r.NewName, r.Error))
			} else if r.OldName != r.NewName {
				preview.WriteString(fmt.Sprintf("[green]%s -> %s[-]\n", r.OldName, r.NewName))
			} else {
				preview.WriteString(fmt.Sprintf("%s (unchanged)\n", r.OldName))
			}
		}
		previewText.SetText(preview.String())
	}

	// Update preview on changes
	patternInput.SetChangedFunc(func(text string) {
		updatePreview()
	})
	searchInput.SetChangedFunc(func(text string) {
		updatePreview()
	})
	replaceInput.SetChangedFunc(func(text string) {
		updatePreview()
	})
	useRegex.SetChangedFunc(func(checked bool) {
		updatePreview()
	})
	counterStartInput.SetChangedFunc(func(text string) {
		updatePreview()
	})
	counterStepInput.SetChangedFunc(func(text string) {
		updatePreview()
	})
	counterDigitsInput.SetChangedFunc(func(text string) {
		updatePreview()
	})
	uppercase.SetChangedFunc(func(checked bool) {
		updatePreview()
	})
	lowercase.SetChangedFunc(func(checked bool) {
		updatePreview()
	})

	// Layout: form on left, preview on right
	formFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(form, 0, 1, true).
		AddItem(previewText, 0, 1, false)

	mainFlex := tview.NewFlex().
		AddItem(formFlex, 0, 1, true)

	// Buttons
	buttonForm := tview.NewForm()
	buttonForm.AddButton("Execute", func() {
		a.executeMultiRename(patternInput.GetText(), searchInput.GetText(), replaceInput.GetText(),
			useRegex.IsChecked(), uppercase.IsChecked(), lowercase.IsChecked(),
			counterStartInput.GetText(), counterStepInput.GetText(), counterDigitsInput.GetText())
	})
	buttonForm.AddButton("Cancel", func() {
		a.closeTopDialog()
	})
	buttonForm.SetBorder(true).
		SetTitle(" Actions ").
		SetTitleAlign(tview.AlignCenter)

	outerFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(mainFlex, 0, 1, true).
		AddItem(buttonForm, 3, 0, false)

	a.pages.AddPage("multirename", a.wrapDialog(outerFlex, 25), true, true)
	a.dialogStack = append(a.dialogStack, "multirename")
	a.tviewApp.SetFocus(patternInput)

	// Initial preview
	updatePreview()
}

// executeMultiRename performs the multi-rename operation.
func (a *App) executeMultiRename(pattern, search, replace string, useRegex, uppercase, lowercase bool,
	counterStart, counterStep, counterDigits string) {

	opts := core.MultiRenameOptions{
		Search:    search,
		Replace:   replace,
		UseRegex:  useRegex,
		Uppercase: uppercase,
		Lowercase: lowercase,
	}

	if v, err := strconv.Atoi(counterStart); err == nil {
		opts.CounterStart = v
	}
	if v, err := strconv.Atoi(counterStep); err == nil {
		opts.CounterStep = v
	}
	if v, err := strconv.Atoi(counterDigits); err == nil {
		opts.CounterDigits = v
	}

	panel := a.focusedPanel()
	entries := a.selectedEntries(panel)
	if len(entries) == 0 {
		entriesList := panel.Entries()
		if len(entriesList) > 0 {
			entries = []core.Entry{entriesList[panel.Cursor()]}
		}
	}

	results, err := core.MultiRenamePreview(entries, panel.CurrentPath(), pattern, opts)
	if err != nil {
		a.ShowErrorDialog("Error", fmt.Sprintf("Preview failed: %v", err), nil)
		return
	}

	// Check for errors in preview
	hasErrors := false
	for _, r := range results {
		if r.Error != nil {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		a.ShowConfirmDialog("Warnings", "Some items have conflicts (target exists). Continue anyway?", "Yes", "No", true, func(ok bool) {
			if ok {
				a.executeRename(entries, panel.CurrentPath(), pattern, opts)
			}
		})
		return
	}

	a.executeRename(entries, panel.CurrentPath(), pattern, opts)
}

func (a *App) executeRename(entries []core.Entry, parentPath, pattern string, opts core.MultiRenameOptions) {
	results, err := core.MultiRenamePreview(entries, parentPath, pattern, opts)
	if err != nil {
		a.ShowErrorDialog("Error", fmt.Sprintf("Preview failed: %v", err), nil)
		return
	}

	results = core.ExecuteMultiRename(parentPath, results)

	// Check results
	errorCount := 0
	for _, r := range results {
		if r.Error != nil {
			errorCount++
		}
	}

	if errorCount > 0 {
		a.ShowErrorDialog("Multi-Rename Complete", fmt.Sprintf("Completed with %d errors.", errorCount), nil)
	} else {
		a.ShowMessageDialog("Multi-Rename Complete", fmt.Sprintf("Successfully renamed %d items.", len(results)), nil)
	}

	// Refresh panel
	a.focusedPanel().Load(a.focusedPanel().CurrentPath())
	a.render()
	a.closeTopDialog()
}

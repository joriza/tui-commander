package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// parseEditorCommand splits an editor command string into program and args.
// It honours a leading double-quoted program path and collapses extra whitespace.
// Returns (program, args, ok).
func parseEditorCommand(cmd string) (string, []string, bool) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "", nil, false
	}

	// Try to parse a possibly quoted program path.
	if strings.HasPrefix(cmd, `"`) {
		// Find the closing quote.
		end := strings.IndexByte(cmd[1:], '"')
		if end < 0 {
			// Unclosed quote: treat the rest as program.
			prog := strings.TrimSpace(cmd[1:])
			if prog == "" {
				return "", nil, false
			}
			return prog, nil, true
		}
		prog := cmd[1 : 1+end]
		remainder := strings.TrimSpace(cmd[1+end+1:])
		var args []string
		if remainder != "" {
			args = splitArgs(remainder)
		}
		if prog == "" {
			return "", nil, false
		}
		return prog, args, true
	}

	// Unquoted: split on whitespace, first token is program.
	fields := splitArgs(cmd)
	if len(fields) == 0 {
		return "", nil, false
	}
	prog := fields[0]
	var args []string
	if len(fields) > 1 {
		args = fields[1:]
	}
	return prog, args, true
}

// splitArgs splits a string by whitespace, collapsing multiple spaces/tabs.
// It honours double quotes and single quotes, so arguments like
// --open "My Notes.txt" are kept as a single token.
// Empty quoted strings ("" or ”) produce an empty token.
// Unclosed quotes at the end are treated as literal characters.
//
// It operates on raw BYTES and preserves them byte-for-byte: an EDITOR
// command may contain non-ASCII bytes (a path like C:\Users\José\edit.exe or
// an argument like --open "Informe ñ.txt"), and re-encoding each byte as a
// rune would double-encode every byte >= 0x80 into mojibake. Quote matching
// is byte-level for the ASCII quote characters, so byte iteration is correct
// and sufficient — no rune-level iteration is needed.
func splitArgs(s string) []string {
	var fields []string
	var cur strings.Builder
	inSpace := true
	inQuote := byte(0)    // 0 = not in quote, '"' or '\'' = inside quoted string
	quotedEmpty := false  // true when we're in a quote and haven't seen content yet
	for i := 0; i < len(s); i++ {
		c := s[i]
		// Handle quote boundaries
		if c == '"' || c == '\'' {
			if inQuote == c {
				// Closing quote of the same type.
				inQuote = 0
				if quotedEmpty {
					// The quote enclosed nothing. Flush any pending token first so an
					// empty quoted string embedded after content (e.g. --flag="")
					// does not split one argument into two out of source order.
					if cur.Len() > 0 {
						fields = append(fields, cur.String())
						cur.Reset()
						inSpace = true
					}
					fields = append(fields, "")
				}
				quotedEmpty = false
				continue
			}
			if inQuote == 0 {
				// Opening quote
				inQuote = c
				quotedEmpty = true // will emit empty if no content before closing
				continue
			}
			// Quote inside a different quote type -> literal
		}
		if inQuote == 0 && (c == ' ' || c == '\t' || c == '\r' || c == '\n') {
			if !inSpace && cur.Len() > 0 {
				fields = append(fields, cur.String())
				cur.Reset()
				inSpace = true
			}
			continue
		}
		inSpace = false
		cur.WriteByte(c)
		quotedEmpty = false // we saw content inside the quote
	}
	// Handle unclosed quote at end: treat the opening quote as literal.
	if inQuote != 0 {
		if cur.Len() > 0 {
			fields = append(fields, string(inQuote)+cur.String())
		} else {
			fields = append(fields, string(inQuote))
		}
	} else if !inSpace && cur.Len() > 0 {
		fields = append(fields, cur.String())
	}
	return fields
}

// resolveEditor returns the editor command string from environment.
// EDITOR wins over VISUAL. Empty/whitespace -> not set.
func resolveEditor() (string, bool) {
	if v := os.Getenv("EDITOR"); strings.TrimSpace(v) != "" {
		return v, true
	}
	if v := os.Getenv("VISUAL"); strings.TrimSpace(v) != "" {
		return v, true
	}
	return "", false
}

// editCursorFile opens the focused panel's cursor entry in an external editor.
// See behaviour in spec: only single files; dirs/.. show message; no editor -> message.
func (a *App) editCursorFile() {
	panel := a.focusedPanel()
	entries := panel.Entries()
	if len(entries) == 0 {
		return
	}

	entry := entries[panel.Cursor()]
	if entry.IsDir || entry.Name == ".." {
		a.ShowMessageDialog("Edit", "Select a file to edit. Folders are not supported in v0.", nil)
		return
	}

	editorCmd, ok := resolveEditor()
	if !ok {
		a.ShowMessageDialog("Edit", "No editor configured. Set the EDITOR environment variable (%EDITOR% on Windows) to a command that can open files.", nil)
		return
	}

	prog, args, ok := parseEditorCommand(editorCmd)
	if !ok || prog == "" {
		a.ShowErrorDialog("Error", "Invalid EDITOR value.", nil)
		return
	}

	filePath := filepath.Join(panel.CurrentPath(), entry.Name)

	// Append file as extra arg
	fullArgs := make([]string, 0, len(args)+1)
	fullArgs = append(fullArgs, args...)
	fullArgs = append(fullArgs, filePath)

	cmd := exec.Command(prog, fullArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Suspend and run editor
	var runErr error
	suspended := a.tviewApp.Suspend(func() {
		runErr = cmd.Run()
	})
	if !suspended {
		a.ShowErrorDialog("Error", "Cannot suspend the terminal to launch the editor.", nil)
		return
	}

	// The editor may have renamed, created or deleted files: always reread,
	// then surface a launch or exit failure instead of swallowing it.
	a.rereadFocusedPanel()
	if runErr != nil {
		a.ShowErrorDialog("Error", fmt.Sprintf("The editor reported an error: %v", runErr), nil)
	}
}

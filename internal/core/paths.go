package core

import (
	"errors"
	"path/filepath"
	"strings"
)

// windowsReservedNames maps Windows reserved device names (case-insensitive)
// to a sentinel. Names with extensions (e.g., "CON.txt") are also rejected.
var windowsReservedNames = map[string]struct{}{
	"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
	"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {},
	"COM5": {}, "COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
	"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {},
	"LPT5": {}, "LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
}

// illegalFileNameChars are characters illegal inside a filename on Windows.
// Slash and backslash are separators, not part of names.
const illegalFileNameChars = "<>:\"|?*"

// ValidateFileName rejects names that are empty, ".", "..", contain NUL bytes,
// Windows reserved device names (with or without extension), trailing dots or
// spaces, or illegal filename characters.
func ValidateFileName(name string) error {
	if name == "" {
		return errors.New("empty filename")
	}
	if name == "." || name == ".." {
		return errors.New("invalid filename: " + name)
	}
	if containsNul(name) {
		return errors.New("filename contains NUL byte")
	}
	if hasIllegalChars(name) {
		return errors.New("filename contains illegal characters")
	}
	if hasTrailingDotOrSpace(name) {
		return errors.New("filename has trailing dot or space")
	}
	if isWindowsReserved(name) {
		return errors.New("filename is a reserved Windows device name")
	}
	return nil
}

// JoinPath joins path elements using filepath.Join, validating each element
// with ValidateFileName. The base may be a drive root or absolute path.
// Errors name the offending element.
func JoinPath(base string, elems ...string) (string, error) {
	if base == "" {
		return "", &ElemError{Elem: "(base)", Err: errors.New("empty base path")}
	}
	for _, elem := range elems {
		if err := ValidateFileName(elem); err != nil {
			return "", &ElemError{Elem: elem, Err: err}
		}
	}
	return filepath.Join(append([]string{base}, elems...)...), nil
}

// ElemError wraps a ValidateFileName error for a specific path element.
type ElemError struct {
	Elem string
	Err  error
}

func (e *ElemError) Error() string {
	return "element " + e.Elem + ": " + e.Err.Error()
}

func (e *ElemError) Unwrap() error { return e.Err }

func containsNul(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return true
		}
	}
	return false
}

func hasIllegalChars(s string) bool {
	for _, r := range s {
		for _, illegal := range illegalFileNameChars {
			if r == illegal {
				return true
			}
		}
	}
	return false
}

func hasTrailingDotOrSpace(s string) bool {
	if len(s) == 0 {
		return false
	}
	last := s[len(s)-1]
	return last == '.' || last == ' '
}

func isWindowsReserved(name string) bool {
	// Check the base name before any extension
	base := name
	if ext := filepath.Ext(name); ext != "" {
		base = name[:len(name)-len(ext)]
	}
	_, ok := windowsReservedNames[strings.ToUpper(base)]
	return ok
}

//go:build windows

package core

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestExtendLongPath_AbsoluteLongerThan260(t *testing.T) {
	// Build an absolute path longer than 260 chars on Windows
	// C:\ is 3 chars, so we need 258+ more chars in the rest
	longSuffix := strings.Repeat("a", 260)
	absPath := "C:\\" + longSuffix

	result, err := ExtendLongPath(absPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `\\?\C:\` + longSuffix
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestExtendLongPath_UNCPath(t *testing.T) {
	uncPath := `\\server\share\` + strings.Repeat("a", 260)

	result, err := ExtendLongPath(uncPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `\\?\UNC\server\share\` + strings.Repeat("a", 260)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestExtendLongPath_AlreadyExtended(t *testing.T) {
	input := `\\?\C:\some\path`

	result, err := ExtendLongPath(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != input {
		t.Errorf("expected passthrough %q, got %q", input, result)
	}
}

func TestExtendLongPath_RelativePath(t *testing.T) {
	_, err := ExtendLongPath("relative\\path")
	if err == nil {
		t.Error("expected error for relative path, got nil")
	}
}

func TestExtendLongPath_Exactly260(t *testing.T) {
	// Exactly 260 chars should NOT get the prefix
	exactPath := "C:\\" + strings.Repeat("a", 257)
	if len(exactPath) != 260 {
		t.Fatalf("test setup: expected len 260, got %d", len(exactPath))
	}

	result, err := ExtendLongPath(exactPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != exactPath {
		t.Errorf("expected no prefix for exactly 260 chars, got %q", result)
	}
}

func TestExtendLongPath_ShortAbsolute(t *testing.T) {
	shortPath := `C:\short\path`

	result, err := ExtendLongPath(shortPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != shortPath {
		t.Errorf("expected passthrough for short path, got %q", result)
	}
}

func TestExtendLongPath_PreservesOriginalOnError(t *testing.T) {
	relPath := "relative\\path"
	_, err := ExtendLongPath(relPath)
	if err == nil {
		t.Fatal("expected error for relative path")
	}
	if !strings.Contains(err.Error(), relPath) {
		t.Errorf("error %q should contain original input %q", err.Error(), relPath)
	}
}

func TestExtendLongPath_JoinPathWindows(t *testing.T) {
	result, err := JoinPath(`C:\`, "docs", "file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join(`C:\`, "docs", "file.txt")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

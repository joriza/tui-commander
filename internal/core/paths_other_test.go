//go:build !windows

package core

import (
	"strings"
	"testing"
)

func TestExtendLongPath_Identity(t *testing.T) {
	input := "/home/user/docs/file.txt"

	result, err := ExtendLongPath(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != input {
		t.Errorf("expected %q, got %q", input, result)
	}
}

func TestExtendLongPath_Empty(t *testing.T) {
	result, err := ExtendLongPath("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestExtendLongPath_LongPath(t *testing.T) {
	longPath := "/home/user/" + strings.Repeat("a", 300) + "/file.txt"

	result, err := ExtendLongPath(longPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != longPath {
		t.Errorf("expected identity for long path, got %q", result)
	}
}

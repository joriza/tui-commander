package core

import (
	"path/filepath"
	"testing"
)

func TestValidateFileName_Empty(t *testing.T) {
	err := ValidateFileName("")
	if err == nil {
		t.Error("expected error for empty name, got nil")
	}
}

func TestValidateFileName_Dot(t *testing.T) {
	err := ValidateFileName(".")
	if err == nil {
		t.Error("expected error for '.', got nil")
	}
}

func TestValidateFileName_DoubleDot(t *testing.T) {
	err := ValidateFileName("..")
	if err == nil {
		t.Error("expected error for '..', got nil")
	}
}

func TestValidateFileName_NulByte(t *testing.T) {
	err := ValidateFileName("a\x00b")
	if err == nil {
		t.Error("expected error for NUL byte, got nil")
	}
}

func TestValidateFileName_IllegalChars(t *testing.T) {
	illegal := []string{"a<b", `a"b`, "a:b", "a|b", "a?b", "a*b"}
	for _, name := range illegal {
		t.Run(name, func(t *testing.T) {
			if err := ValidateFileName(name); err == nil {
				t.Errorf("expected error for %q, got nil", name)
			}
		})
	}
}

func TestValidateFileName_ReservedDeviceNames(t *testing.T) {
	reserved := []string{
		"CON", "con", "Con", "PRN", "AUX", "NUL",
		"COM1", "COM9", "com1", "com9",
		"LPT1", "LPT9", "lpt1", "lpt9",
	}
	for _, name := range reserved {
		t.Run(name, func(t *testing.T) {
			if err := ValidateFileName(name); err == nil {
				t.Errorf("expected error for reserved name %q, got nil", name)
			}
		})
	}
}

func TestValidateFileName_ReservedWithExtension(t *testing.T) {
	err := ValidateFileName("CON.txt")
	if err == nil {
		t.Error("expected error for CON.txt, got nil")
	}
}

func TestValidateFileName_TrailingDot(t *testing.T) {
	err := ValidateFileName("file.")
	if err == nil {
		t.Error("expected error for trailing dot, got nil")
	}
}

func TestValidateFileName_TrailingSpace(t *testing.T) {
	err := ValidateFileName("file ")
	if err == nil {
		t.Error("expected error for trailing space, got nil")
	}
}

func TestValidateFileName_ValidNames(t *testing.T) {
	valid := []string{"file.txt", "my-file", "a1b2", "foo.bar.baz", "README.md"}
	for _, name := range valid {
		t.Run(name, func(t *testing.T) {
			if err := ValidateFileName(name); err != nil {
				t.Errorf("unexpected error for %q: %v", name, err)
			}
		})
	}
}

func TestJoinPath_Basic(t *testing.T) {
	result, err := JoinPath("/home/user", "docs", "file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join("/home/user", "docs", "file.txt")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestJoinPath_InvalidElem(t *testing.T) {
	_, err := JoinPath("/base", "bad<name")
	if err == nil {
		t.Error("expected error for invalid elem, got nil")
	}
}

func TestJoinPath_EmptyBase(t *testing.T) {
	_, err := JoinPath("", "file.txt")
	if err == nil {
		t.Error("expected error for empty base, got nil")
	}
}

func TestJoinPath_DriveRoot(t *testing.T) {
	result, err := JoinPath("C:\\", "docs", "file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join("C:\\", "docs", "file.txt")
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

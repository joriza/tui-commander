package core

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// TestMakeDir_Valid
// ---------------------------------------------------------------------------

func TestMakeDir_Valid(t *testing.T) {
	dir := t.TempDir()

	err := MakeDir(dir, "newdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := filepath.Join(dir, "newdir")
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected directory, got file")
	}
}

// ---------------------------------------------------------------------------
// TestMakeDir_Exists
// ---------------------------------------------------------------------------

func TestMakeDir_Exists(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing")
	if err := os.Mkdir(existing, 0755); err != nil {
		t.Fatal(err)
	}

	err := MakeDir(dir, "existing")
	if err == nil {
		t.Fatal("expected error when directory already exists")
	}
}

// ---------------------------------------------------------------------------
// TestMakeDir_InvalidName
// ---------------------------------------------------------------------------

func TestMakeDir_InvalidName(t *testing.T) {
	dir := t.TempDir()

	err := MakeDir(dir, "bad<name")
	if err == nil {
		t.Fatal("expected error for invalid name")
	}
}

// ---------------------------------------------------------------------------
// TestMakeDir_MissingParent
// ---------------------------------------------------------------------------

func TestMakeDir_MissingParent(t *testing.T) {
	dir := t.TempDir()
	missingParent := filepath.Join(dir, "does-not-exist")

	err := MakeDir(missingParent, "newdir")
	if err == nil {
		t.Fatal("expected error when parent does not exist")
	}
}

// ---------------------------------------------------------------------------
// TestMakeDir_ReservedName
// ---------------------------------------------------------------------------

func TestMakeDir_ReservedName(t *testing.T) {
	dir := t.TempDir()

	err := MakeDir(dir, "CON")
	if err == nil {
		t.Fatal("expected error for reserved Windows device name")
	}
}

// ---------------------------------------------------------------------------
// TestMakeDir_ParentIsFile
// ---------------------------------------------------------------------------

func TestMakeDir_ParentIsFile(t *testing.T) {
	dir := t.TempDir()
	parentFile := filepath.Join(dir, "afile.txt")
	if err := os.WriteFile(parentFile, []byte("not a dir"), 0644); err != nil {
		t.Fatal(err)
	}

	err := MakeDir(parentFile, "newdir")
	if err == nil {
		t.Fatal("expected error when parent is a file")
	}
}

package core

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// TestRenamePath_Valid
// ---------------------------------------------------------------------------

func TestRenamePath_Valid(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(src, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	err := RenamePath(src, "new.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("old path should not exist after rename")
	}

	newPath := filepath.Join(dir, "new.txt")
	got, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("new path should exist: %v", err)
	}
	if string(got) != "content" {
		t.Errorf("content = %q, want %q", string(got), "content")
	}
}

// ---------------------------------------------------------------------------
// TestRenamePath_Collision
// ---------------------------------------------------------------------------

func TestRenamePath_Collision(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(src, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(dst, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	err := RenamePath(src, "existing.txt")
	if err == nil {
		t.Fatal("expected error when destination exists")
	}

	// Source should still exist (no rename happened).
	if _, err := os.Stat(src); err != nil {
		t.Errorf("source should still exist after failed rename: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestRenamePath_ReservedName
// ---------------------------------------------------------------------------

func TestRenamePath_ReservedName(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(src, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	err := RenamePath(src, "CON")
	if err == nil {
		t.Fatal("expected error for reserved Windows device name")
	}
}

// ---------------------------------------------------------------------------
// TestRenamePath_MissingSource
// ---------------------------------------------------------------------------

func TestRenamePath_MissingSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "nonexistent.txt")

	err := RenamePath(src, "new.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent source")
	}
}

// ---------------------------------------------------------------------------
// TestRenamePath_RelativeOldPath
// ---------------------------------------------------------------------------

func TestRenamePath_RelativeOldPath(t *testing.T) {
	err := RenamePath("relative/path.txt", "new.txt")
	if err == nil {
		t.Fatal("expected error for relative oldPath")
	}
}

// ---------------------------------------------------------------------------
// TestRenamePath_InvalidName
// ---------------------------------------------------------------------------

func TestRenamePath_InvalidName(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(src, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	err := RenamePath(src, "bad<name")
	if err == nil {
		t.Fatal("expected error for invalid newName")
	}
}

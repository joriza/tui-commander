package core

import (
	"strings"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestFindFiles_Basic tests basic file finding functionality.
func TestFindFiles_Basic(t *testing.T) {
	dir := t.TempDir()

	// Create test structure
	files := map[string]string{
		"file1.txt":       "content1",
		"file2.log":       "content2",
		"script.go":       "package main",
		"README.md":       "# Readme",
		"subdir/nested.txt": "nested",
	}
	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	p := NewPanel()
	if err := p.Load(dir); err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Test: find all .txt files
	opts := DefaultFindOptions()
	opts.NamePattern = "*.txt"
	opts.Mode = FindFilesOnly

	results, err := FindFiles(dir, opts)
	if err != nil {
		t.Fatalf("FindFiles: %v", err)
	}

	if results.Total != 2 {
		t.Errorf("expected 2 .txt files, got %d", results.Total)
	}
	for _, m := range results.Matches {
		if m.IsDir {
			t.Errorf("expected file, got dir: %s", m.RelPath)
		}
	}
}

// TestFindFiles_DirsOnly tests finding only directories.
func TestFindFiles_DirsOnly(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "sub1"), 0755)
	os.MkdirAll(filepath.Join(dir, "sub2/nested"), 0755)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644)

	opts := DefaultFindOptions()
	opts.Mode = FindDirs

	results, err := FindFiles(dir, opts)
	if err != nil {
		t.Fatalf("FindFiles: %v", err)
	}

	if results.Total != 3 { // sub1, sub2, sub2/nested
		t.Errorf("expected 3 dirs, got %d: %v", results.Total, results.Matches)
	}
	for _, m := range results.Matches {
		if !m.IsDir {
			t.Errorf("expected dir, got file: %s", m.RelPath)
		}
	}
}

// TestFindFiles_SizeFilter tests size-based filtering.
func TestFindFiles_SizeFilter(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "small.txt"), []byte("small"), 0644)
	os.WriteFile(filepath.Join(dir, "medium.txt"), []byte(strings.Repeat("x", 10000)), 0644)
	os.WriteFile(filepath.Join(dir, "large.txt"), []byte(strings.Repeat("x", 100000)), 0644)

	opts := DefaultFindOptions()
	opts.Mode = FindFilesOnly
	opts.MinSize = 5000
	opts.MaxSize = 50000

	results, err := FindFiles(dir, opts)
	if err != nil {
		t.Fatalf("FindFiles: %v", err)
	}

	if results.Total != 1 {
		t.Errorf("expected 1 file (medium), got %d: %v", results.Total, results.Matches)
	}
	if results.Matches[0].RelPath != "medium.txt" {
		t.Errorf("expected medium.txt, got %s", results.Matches[0].RelPath)
	}
}

// TestFindFiles_TimeFilter tests time-based filtering.
func TestFindFiles_TimeFilter(t *testing.T) {
	dir := t.TempDir()

	oldFile := filepath.Join(dir, "old.txt")
	newFile := filepath.Join(dir, "new.txt")

	os.WriteFile(oldFile, []byte("old"), 0644)
	os.Chtimes(oldFile, time.Now().Add(-24*time.Hour), time.Now().Add(-24*time.Hour))

	os.WriteFile(newFile, []byte("new"), 0644)
	os.Chtimes(newFile, time.Now(), time.Now())

	cutoff := time.Now().Add(-12 * time.Hour)

	opts := DefaultFindOptions()
	opts.Mode = FindFilesOnly
	opts.ModTimeAfter = &cutoff

	results, err := FindFiles(dir, opts)
	if err != nil {
		t.Fatalf("FindFiles: %v", err)
	}

	if results.Total != 1 {
		t.Errorf("expected 1 new file, got %d: %v", results.Total, results.Matches)
	}
	if results.Matches[0].RelPath != "new.txt" {
		t.Errorf("expected new.txt, got %s", results.Matches[0].RelPath)
	}
}

// TestFindFiles_ContentSearch tests content-based searching.
func TestFindFiles_ContentSearch(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "match.go"), []byte("package main\nfunc main() {}"), 0644)
	os.WriteFile(filepath.Join(dir, "nomatch.txt"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(dir, "also_match.go"), []byte("// TODO: implement"), 0644)

	opts := DefaultFindOptions()
	opts.Mode = FindFilesOnly
	opts.NamePattern = "*.go"
	opts.Content = "package"

	results, err := FindFiles(dir, opts)
	if err != nil {
		t.Fatalf("FindFiles: %v", err)
	}

	if results.Total != 1 {
		t.Errorf("expected 1 file with 'package' in *.go, got %d: %v", results.Total, results.Matches)
	}
	if results.Matches[0].RelPath != "match.go" {
		t.Errorf("expected match.go, got %s", results.Matches[0].RelPath)
	}
	if !results.Matches[0].ContentMatch {
		t.Error("expected ContentMatch=true")
	}
}

// TestFindFiles_CaseInsensitiveContent tests case-insensitive content search.
func TestFindFiles_CaseInsensitiveContent(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("Hello World"), 0644)

	opts := DefaultFindOptions()
	opts.Mode = FindFilesOnly
	opts.Content = "hello"
	opts.CaseSensitive = false

	results, err := FindFiles(dir, opts)
	if err != nil {
		t.Fatalf("FindFiles: %v", err)
	}

	if results.Total != 1 {
		t.Errorf("expected 1 match (case-insensitive), got %d", results.Total)
	}

	// Case sensitive should not match
	opts.CaseSensitive = true
	results, err = FindFiles(dir, opts)
	if err != nil {
		t.Fatalf("FindFiles: %v", err)
	}

	if results.Total != 0 {
		t.Errorf("expected 0 matches (case-sensitive), got %d", results.Total)
	}
}

// TestFindFiles_RespectsSymlinkPolicy tests that symlinks are not followed.
func TestFindFiles_RespectsSymlinkPolicy(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()

	os.WriteFile(filepath.Join(target, "target_file.txt"), []byte("target"), 0644)

	linkPath := filepath.Join(dir, "link_to_target")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Skipf("Symlink not supported: %v", err)
	}

	opts := DefaultFindOptions()
	opts.Mode = FindBoth

	results, err := FindFiles(dir, opts)
	if err != nil {
		t.Fatalf("FindFiles: %v", err)
	}

	// Should find the symlink itself, not the target's contents
	foundLink := false
	for _, m := range results.Matches {
		if m.RelPath == "link_to_target" {
			foundLink = true
			if !m.IsLink {
				t.Error("symlink should be marked as link")
			}
		}
	}
	if !foundLink {
		t.Error("expected to find the symlink")
	}

	// Should NOT find target_file.txt (not following symlinks)
	for _, m := range results.Matches {
		if m.RelPath == "link_to_target/target_file.txt" {
			t.Error("should not follow symlink into target")
		}
	}
}
package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestMultiRename_Basic tests basic rename with counter.
func TestMultiRename_Basic(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	files := []string{"file1.txt", "file2.txt", "file3.txt"}
	for _, f := range files {
		os.WriteFile(filepath.Join(dir, f), []byte("content"), 0644)
	}

	entries := []Entry{
		{Name: "file1.txt", IsDir: false},
		{Name: "file2.txt", IsDir: false},
		{Name: "file3.txt", IsDir: false},
	}

	opts := DefaultMultiRenameOptions()
	opts.CounterStart = 1
	opts.CounterStep = 1
	opts.CounterDigits = 3

	results, err := MultiRename(entries, "[N]_[C:3][E]", opts)
	if err != nil {
		t.Fatalf("MultiRename: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	expected := []string{"file1_001.txt", "file2_002.txt", "file3_003.txt"}
	for i, r := range results {
		if r.NewName != expected[i] {
			t.Errorf("result %d: expected %s, got %s", i, expected[i], r.NewName)
		}
		if r.Error != nil {
			t.Errorf("result %d has error: %v", i, r.Error)
		}
	}
}

// TestMultiRename_SearchReplace tests search and replace.
func TestMultiRename_SearchReplace(t *testing.T) {
	entries := []Entry{
		{Name: "document_v1.txt", IsDir: false},
		{Name: "image_v1.png", IsDir: false},
		{Name: "data_v1.csv", IsDir: false},
	}

	opts := DefaultMultiRenameOptions()
	opts.Search = "v1"
	opts.Replace = "v2"

	results, err := MultiRename(entries, "[N][E]", opts)
	if err != nil {
		t.Fatalf("MultiRename: %v", err)
	}

	expected := []string{"document_v2.txt", "image_v2.png", "data_v2.csv"}
	for i, r := range results {
		if r.NewName != expected[i] {
			t.Errorf("result %d: expected %s, got %s", i, expected[i], r.NewName)
		}
	}
}

// TestMultiRename_Regex tests regex search and replace.
func TestMultiRename_Regex(t *testing.T) {
	entries := []Entry{
		{Name: "file_001.txt", IsDir: false},
		{Name: "file_002.txt", IsDir: false},
	}

	opts := DefaultMultiRenameOptions()
	opts.Search = `(\d+)`
	opts.Replace = "num$1"
	opts.UseRegex = true

	results, err := MultiRename(entries, "[N][E]", opts)
	if err != nil {
		t.Fatalf("MultiRename: %v", err)
	}

	expected := []string{"file_num001.txt", "file_num002.txt"}
	for i, r := range results {
		if r.NewName != expected[i] {
			t.Errorf("result %d: expected %s, got %s", i, expected[i], r.NewName)
		}
	}
}

// TestMultiRename_DatePlaceholders tests date/time placeholders.
func TestMultiRename_DatePlaceholders(t *testing.T) {
	entry := Entry{
		Name:    "photo.jpg",
		IsDir:   false,
		ModTime: time.Date(2024, 12, 25, 14, 30, 45, 0, time.UTC),
	}

	opts := DefaultMultiRenameOptions()

	results, err := MultiRename([]Entry{entry}, "[Y]-[M]-[D]_[H]-[I]-[S]_[N][E]", opts)
	if err != nil {
		t.Fatalf("MultiRename: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Check that date placeholders were replaced
	newName := results[0].NewName
	if !containsAll(newName, []string{"2024", "12", "25", "14", "30", "45"}) {
		t.Errorf("date placeholders not replaced correctly: %s", newName)
	}
	if !containsAll(newName, []string{"photo", ".jpg"}) {
		t.Errorf("name/ext not preserved: %s", newName)
	}
}

// TestMultiRename_CaseConversion tests uppercase/lowercase.
func TestMultiRename_CaseConversion(t *testing.T) {
	entries := []Entry{
		{Name: "MyFile.TXT", IsDir: false},
		{Name: "another.PNG", IsDir: false},
	}

	// Test uppercase
	opts := DefaultMultiRenameOptions()
	opts.Uppercase = true

	results, err := MultiRename(entries, "[N][E]", opts)
	if err != nil {
		t.Fatalf("MultiRename: %v", err)
	}

	if results[0].NewName != "MYFILE.TXT" {
		t.Errorf("uppercase failed: got %s", results[0].NewName)
	}

	// Test lowercase
	opts.Uppercase = false
	opts.Lowercase = true

	results, err = MultiRename(entries, "[N][E]", opts)
	if err != nil {
		t.Fatalf("MultiRename: %v", err)
	}

	if results[0].NewName != "myfile.txt" {
		t.Errorf("lowercase failed: got %s", results[0].NewName)
	}
}

// TestMultiRename_ExtensionPreserved tests that extension is preserved.
func TestMultiRename_ExtensionPreserved(t *testing.T) {
	entry := Entry{Name: "document.pdf", IsDir: false}
	opts := DefaultMultiRenameOptions()

	// Pattern without [E] should still preserve extension
	results, err := MultiRename([]Entry{entry}, "[N]_new", opts)
	if err != nil {
		t.Fatalf("MultiRename: %v", err)
	}

	if results[0].NewName != "document_new.pdf" {
		t.Errorf("extension not preserved: got %s", results[0].NewName)
	}
}

// TestMultiRename_CounterFormat tests counter digit formatting.
func TestMultiRename_CounterFormat(t *testing.T) {
	entries := []Entry{
		{Name: "a.txt", IsDir: false},
		{Name: "b.txt", IsDir: false},
		{Name: "c.txt", IsDir: false},
		{Name: "d.txt", IsDir: false},
		{Name: "e.txt", IsDir: false},
		{Name: "f.txt", IsDir: false},
		{Name: "g.txt", IsDir: false},
		{Name: "h.txt", IsDir: false},
		{Name: "i.txt", IsDir: false},
		{Name: "j.txt", IsDir: false}, // 10th file
	}

	opts := DefaultMultiRenameOptions()
	opts.CounterStart = 1
	opts.CounterStep = 1
	opts.CounterDigits = 3

	results, err := MultiRename(entries, "[N]_[C][E]", opts)
	if err != nil {
		t.Fatalf("MultiRename: %v", err)
	}

	// Check that 10th file gets 010
	if results[9].NewName != "j_010.txt" {
		t.Errorf("10th file counter format wrong: got %s", results[9].NewName)
	}
}

// TestMultiRenamePreview_Execute tests the preview and execute flow.
func TestMultiRenamePreview_Execute(t *testing.T) {
	dir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644)

	entries := []Entry{
		{Name: "a.txt", IsDir: false, Size: 1},
		{Name: "b.txt", IsDir: false, Size: 1},
	}

	opts := DefaultMultiRenameOptions()
	opts.CounterStart = 1
	opts.CounterStep = 1
	opts.CounterDigits = 2

	// Preview
	results, err := MultiRenamePreview(entries, dir, "[N]_[C:2][E]", opts)
	if err != nil {
		t.Fatalf("MultiRenamePreview: %v", err)
	}

	if results[0].NewName != "a_01.txt" || results[1].NewName != "b_02.txt" {
		t.Errorf("preview names wrong: %s, %s", results[0].NewName, results[1].NewName)
	}

	// Execute
	results = ExecuteMultiRename(dir, results)

	if results[0].Error != nil || results[1].Error != nil {
		t.Errorf("execute errors: %v, %v", results[0].Error, results[1].Error)
	}

	// Verify files were renamed
	if _, err := os.Stat(filepath.Join(dir, "a_01.txt")); err != nil {
		t.Errorf("a_01.txt not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b_02.txt")); err != nil {
		t.Errorf("b_02.txt not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); err == nil {
		t.Error("old file a.txt still exists")
	}
}

func containsAll(s string, substrs []string) bool {
	for _, sub := range substrs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > len(sub) && (s[:len(sub)] == sub || s[len(s)-len(sub):] == sub || findSubstring(s, sub)))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writeFileMtime(t *testing.T, dir, name string, content []byte, mtime time.Time) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return p
}

// ---------------------------------------------------------------------------
// TestMoveQueue_SameVolume_FastPath
// ---------------------------------------------------------------------------

func TestMoveQueue_SameVolume_FastPath(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	mtime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	srcFile := writeFileMtime(t, srcDir, "hello.txt", []byte("hello world"), mtime)
	dstFile := filepath.Join(dstDir, "hello.txt")

	q := NewMoveQueue([]MoveItem{{Source: srcFile, Dest: dstFile}}, MoveOptions{})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	// Source must be gone.
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Errorf("source should not exist after move, err = %v", err)
	}

	// Dest must have content and mtime.
	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read dest file: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("content mismatch: expected %q, got %q", "hello world", string(got))
	}
	stat, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("failed to stat dest file: %v", err)
	}
	if !stat.ModTime().Equal(mtime) {
		t.Errorf("mtime not preserved: expected %v, got %v", mtime, stat.ModTime())
	}

	// Same-volume fast path: no ItemProgress events.
	progressEvents := findEvent[ItemProgress](events)
	if len(progressEvents) > 0 {
		t.Errorf("expected no ItemProgress events for same-volume fast path, got %d", len(progressEvents))
	}

	started, ok := firstEvent[ItemStarted](events)
	if !ok {
		t.Fatal("expected ItemStarted event")
	}
	if started.Size != int64(len("hello world")) {
		t.Errorf("ItemStarted.Size = %d, want %d", started.Size, int64(len("hello world")))
	}
	var done ItemDone
	if _, ok := firstEvent[ItemDone](events); !ok {
		t.Fatal("expected ItemDone event")
	}
	_ = done

	var qd QueueDone
	if _, ok := firstEvent[QueueDone](events); !ok {
		t.Fatal("expected QueueDone event")
	}
	if qd.Err != nil {
		t.Errorf("QueueDone.Err = %v, want nil", qd.Err)
	}
}

// ---------------------------------------------------------------------------
// TestMoveQueue_FallbackForced
// ---------------------------------------------------------------------------

func TestMoveQueue_FallbackForced(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcSub := filepath.Join(srcDir, "src")
	mkdirAll(t, filepath.Join(srcSub, "sub"))
	writeFileMtime(t, srcSub, "a.txt", []byte("file a"), time.Now())
	writeFileMtime(t, filepath.Join(srcSub, "sub"), "b.txt", []byte("file b"), time.Now())

	dstRoot := filepath.Join(dstDir, "dst")

	q := NewMoveQueue([]MoveItem{{Source: srcSub, Dest: dstRoot}}, MoveOptions{ForceFallback: true})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	// Source tree must be removed after successful copy+delete.
	if _, err := os.Stat(srcSub); !os.IsNotExist(err) {
		t.Errorf("source tree should be removed after fallback move, err = %v", err)
	}

	// Dest must have the content.
	if _, err := os.ReadFile(filepath.Join(dstRoot, "a.txt")); err != nil {
		t.Errorf("missing a.txt in dest: %v", err)
	}
	if _, err := os.ReadFile(filepath.Join(dstRoot, "sub", "b.txt")); err != nil {
		t.Errorf("missing sub/b.txt in dest: %v", err)
	}

	// Fallback path produces progress events (from copy engine).
	progressEvents := findEvent[ItemProgress](events)
	if len(progressEvents) == 0 {
		t.Error("expected ItemProgress events for fallback copy path")
	}

	// ItemStarted.Size for a directory item should equal total tree bytes.
	expectedBytes := int64(len("file a") + len("file b"))
	var started ItemStarted
	for _, e := range events {
		if v, ok := e.(ItemStarted); ok {
			started = v
			break
		}
	}
	if started.Size != expectedBytes {
		t.Errorf("ItemStarted.Size = %d, want %d (total tree bytes)", started.Size, expectedBytes)
	}

	var qd QueueDone
	if _, ok := firstEvent[QueueDone](events); !ok {
		t.Fatal("expected QueueDone event")
	}
	if qd.Err != nil {
		t.Errorf("QueueDone.Err = %v, want nil", qd.Err)
	}
}

// ---------------------------------------------------------------------------
// TestMoveQueue_PerItemErrorContinues
// ---------------------------------------------------------------------------

func TestMoveQueue_PerItemErrorContinues(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	f1 := writeFileMtime(t, srcDir, "f1.txt", []byte("file one"), time.Now())
	// f2 dest already exists and Overwrite is false.
	f2 := writeFileMtime(t, srcDir, "f2.txt", []byte("file two"), time.Now())
	dstCollision := filepath.Join(dstDir, "f2.txt")
	writeFileMtime(t, dstDir, "f2.txt", []byte("existing"), time.Now())

	q := NewMoveQueue([]MoveItem{
		{Source: f1, Dest: filepath.Join(dstDir, "f1.txt")},
		{Source: f2, Dest: dstCollision},
	}, MoveOptions{Overwrite: false})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	var errs []ItemError
	for _, e := range events {
		if v, ok := e.(ItemError); ok {
			errs = append(errs, v)
		}
	}
	if len(errs) == 0 {
		t.Fatal("expected at least one ItemError")
	}

	// First item (f1) should succeed despite second item error.
	var dones []ItemDone
	for _, e := range events {
		if v, ok := e.(ItemDone); ok {
			dones = append(dones, v)
		}
	}
	found := false
	for _, d := range dones {
		if d.Source == f1 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ItemDone for first item despite second item error")
	}

	// f1 should no longer exist at source (moved).
	if _, err := os.Stat(f1); !os.IsNotExist(err) {
		t.Error("f1 source should have been removed after successful move")
	}
}

// ---------------------------------------------------------------------------
// TestMoveQueue_DestInsideSourceGuard
// ---------------------------------------------------------------------------

func TestMoveQueue_DestInsideSourceGuard(t *testing.T) {
	srcDir := t.TempDir()
	srcSub := filepath.Join(srcDir, "src")
	mkdirAll(t, srcSub)
	writeFileMtime(t, srcSub, "a.txt", []byte("content"), time.Now())

	dstInside := filepath.Join(srcSub, "moved")

	q := NewMoveQueue([]MoveItem{{Source: srcSub, Dest: dstInside}}, MoveOptions{})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	var errs []ItemError
	for _, e := range events {
		if v, ok := e.(ItemError); ok {
			errs = append(errs, v)
		}
	}
	if len(errs) == 0 {
		t.Fatal("expected ItemError for dest inside source guard")
	}
}

// ---------------------------------------------------------------------------
// TestMoveQueue_OverwritePolicy
// ---------------------------------------------------------------------------

func TestMoveQueue_OverwriteFalse(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := writeFileMtime(t, srcDir, "a.txt", []byte("new content"), time.Now())
	dstFile := filepath.Join(dstDir, "a.txt")
	writeFileMtime(t, dstDir, "a.txt", []byte("old content"), time.Now())

	q := NewMoveQueue([]MoveItem{{Source: srcFile, Dest: dstFile}}, MoveOptions{Overwrite: false})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	var errs []ItemError
	for _, e := range events {
		if v, ok := e.(ItemError); ok {
			errs = append(errs, v)
		}
	}
	if len(errs) == 0 {
		t.Fatal("expected ItemError for collision with Overwrite=false")
	}

	// Dest should still have old content.
	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("dest file should still exist: %v", err)
	}
	if string(got) != "old content" {
		t.Errorf("dest file was overwritten: got %q, want %q", string(got), "old content")
	}
}

func TestMoveQueue_OverwriteTrue(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := writeFileMtime(t, srcDir, "a.txt", []byte("new content"), time.Now())
	dstFile := filepath.Join(dstDir, "a.txt")
	writeFileMtime(t, dstDir, "a.txt", []byte("old content"), time.Now())

	q := NewMoveQueue([]MoveItem{{Source: srcFile, Dest: dstFile}}, MoveOptions{Overwrite: true})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	var errs []ItemError
	for _, e := range events {
		if v, ok := e.(ItemError); ok {
			errs = append(errs, v)
		}
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected ItemError: %v", errs[0].Cause)
	}

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("dest file should exist: %v", err)
	}
	if string(got) != "new content" {
		t.Errorf("dest file content = %q, want %q", string(got), "new content")
	}

	// Source should be gone.
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Error("source should have been removed after overwrite move")
	}
}

// ---------------------------------------------------------------------------
// TestMoveQueue_NonExistentSource
// ---------------------------------------------------------------------------

func TestMoveQueue_NonExistentSource(t *testing.T) {
	dstDir := t.TempDir()

	q := NewMoveQueue([]MoveItem{{Source: "/nonexistent/path/to/file.txt", Dest: filepath.Join(dstDir, "file.txt")}}, MoveOptions{})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	var errs []ItemError
	for _, e := range events {
		if v, ok := e.(ItemError); ok {
			errs = append(errs, v)
		}
	}
	if len(errs) == 0 {
		t.Fatal("expected ItemError for nonexistent source")
	}
}

// ---------------------------------------------------------------------------
// TestMoveQueue_EmptyQueue
// ---------------------------------------------------------------------------

func TestMoveQueue_EmptyQueue(t *testing.T) {
	q := NewMoveQueue(nil, MoveOptions{})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	var totals QueueTotals
	if _, ok := firstEvent[QueueTotals](events); !ok {
		t.Fatal("expected QueueTotals event")
	}
	if totals.TotalBytes != 0 || totals.TotalItems != 0 {
		t.Errorf("empty queue totals = %+v, want {0, 0}", totals)
	}

	var qd QueueDone
	if _, ok := firstEvent[QueueDone](events); !ok {
		t.Fatal("expected QueueDone event")
	}
	if qd.Err != nil {
		t.Errorf("QueueDone.Err = %v, want nil", qd.Err)
	}
}

// TestMoveQueue_ItemStartedSizePopulated verifies that ItemStarted.Size
// is populated for each move item (file size for files, tree bytes for dirs).
func TestMoveQueue_ItemStartedSizePopulated(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	f1 := writeFileMtime(t, srcDir, "f1.txt", []byte("hello"), time.Now())
	f2 := writeFileMtime(t, srcDir, "f2.txt", []byte("world!!"), time.Now())

	q := NewMoveQueue([]MoveItem{
		{Source: f1, Dest: filepath.Join(dstDir, "f1.txt")},
		{Source: f2, Dest: filepath.Join(dstDir, "f2.txt")},
	}, MoveOptions{ForceFallback: true})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	var starteds []ItemStarted
	for _, e := range events {
		if v, ok := e.(ItemStarted); ok {
			starteds = append(starteds, v)
		}
	}
	if len(starteds) != 2 {
		t.Fatalf("expected 2 ItemStarted events, got %d", len(starteds))
	}
	if starteds[0].Size != int64(len("hello")) {
		t.Errorf("ItemStarted[0].Size = %d, want %d", starteds[0].Size, int64(len("hello")))
	}
	if starteds[1].Size != int64(len("world!!")) {
		t.Errorf("ItemStarted[1].Size = %d, want %d", starteds[1].Size, int64(len("world!!")))
	}
}

// ---------------------------------------------------------------------------
// TestMoveQueue_FallbackForwardsItemFile
// ---------------------------------------------------------------------------

// TestMoveQueue_FallbackForwardsItemFile verifies that the copy-then-delete
// fallback forwards ItemFile events from the copy engine with the individual
// file sources, so a directory move reports per-file progress.
func TestMoveQueue_FallbackForwardsItemFile(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcSub := filepath.Join(srcDir, "src")
	mkdirAll(t, filepath.Join(srcSub, "sub"))
	writeFileMtime(t, srcSub, "a.txt", []byte("file a"), time.Now())
	writeFileMtime(t, filepath.Join(srcSub, "sub"), "b.txt", []byte("file b"), time.Now())

	dstRoot := filepath.Join(dstDir, "dst")

	q := NewMoveQueue([]MoveItem{{Source: srcSub, Dest: dstRoot}}, MoveOptions{ForceFallback: true})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	totals, ok := firstEvent[QueueTotals](events)
	if !ok {
		t.Fatal("expected QueueTotals event")
	}
	if totals.TotalFiles != 2 {
		t.Errorf("QueueTotals.TotalFiles = %d, want 2", totals.TotalFiles)
	}

	files := findEvent[ItemFile](events)
	if len(files) != 2 {
		t.Fatalf("expected 2 ItemFile events, got %d", len(files))
	}
	for _, f := range files {
		if f.Source == srcSub {
			t.Errorf("ItemFile.Source = %q, want an individual file, not the directory", f.Source)
		}
	}
}

// ---------------------------------------------------------------------------
// TestMoveQueue_MoveDirectory
// ---------------------------------------------------------------------------

func TestMoveQueue_MoveDirectory(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcSub := filepath.Join(srcDir, "srcdir")
	mkdirAll(t, filepath.Join(srcSub, "sub", "deep"))
	writeFileMtime(t, srcSub, "a.txt", []byte("file a"), time.Now())
	writeFileMtime(t, filepath.Join(srcSub, "sub"), "b.txt", []byte("file b"), time.Now())
	writeFileMtime(t, filepath.Join(srcSub, "sub", "deep"), "c.txt", []byte("file c"), time.Now())

	dstRoot := filepath.Join(dstDir, "srcdir")

	q := NewMoveQueue([]MoveItem{{Source: srcSub, Dest: dstRoot}}, MoveOptions{})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	// Source tree must be gone.
	if _, err := os.Stat(srcSub); !os.IsNotExist(err) {
		t.Errorf("source directory should be removed after move, err = %v", err)
	}

	// Dest must have all content.
	for _, tc := range []struct {
		relPath string
		content string
	}{
		{"a.txt", "file a"},
		{"sub/b.txt", "file b"},
		{"sub/deep/c.txt", "file c"},
	} {
		p := filepath.Join(dstRoot, tc.relPath)
		got, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("missing file %q: %v", p, err)
			continue
		}
		if string(got) != tc.content {
			t.Errorf("file %q content = %q, want %q", p, string(got), tc.content)
		}
	}

	var qd QueueDone
	if _, ok := firstEvent[QueueDone](events); !ok {
		t.Fatal("expected QueueDone")
	}
	if qd.Err != nil {
		t.Errorf("QueueDone.Err = %v, want nil", qd.Err)
	}
}

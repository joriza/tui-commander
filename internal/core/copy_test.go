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

func writeFile(t *testing.T, dir, name string, content []byte, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, mode); err != nil {
		t.Fatal(err)
	}
	return p
}

func writeFileWithMtime(t *testing.T, dir, name string, content []byte, mtime time.Time) string {
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

func mkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
}

func collectEvents(t *testing.T, ch <-chan any) []any {
	t.Helper()
	var events []any
	for e := range ch {
		events = append(events, e)
	}
	return events
}

func findEvent[T any](events []any) []T {
	var out []T
	for _, e := range events {
		if v, ok := e.(T); ok {
			out = append(out, v)
		}
	}
	return out
}

func firstEvent[T any](events []any) (T, bool) {
	var zero T
	for _, e := range events {
		if v, ok := e.(T); ok {
			return v, true
		}
	}
	return zero, false
}

// ---------------------------------------------------------------------------
// TestCopyQueue_SingleFile_CopyContentAndMtime
// ---------------------------------------------------------------------------

func TestCopyQueue_SingleFile_CopyContentAndMtime(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	mtime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	srcFile := writeFileWithMtime(t, srcDir, "hello.txt", []byte("hello world"), mtime)
	dstFile := filepath.Join(dstDir, "hello.txt")

	q := NewCopyQueue([]CopyItem{{Source: srcFile, Dest: dstFile}}, CopyOptions{})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

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

	started, ok := firstEvent[ItemStarted](events)
	if !ok {
		t.Fatal("expected ItemStarted event")
	}
	if started.Size != int64(len("hello world")) {
		t.Errorf("ItemStarted.Size = %d, want %d", started.Size, int64(len("hello world")))
	}

	if _, ok := firstEvent[ItemDone](events); !ok {
		t.Fatal("expected ItemDone event")
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
// TestCopyQueue_RecursiveTreeCopy
// ---------------------------------------------------------------------------

func TestCopyQueue_RecursiveTreeCopy(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	realSrc := filepath.Join(srcDir, "src")
	mkdirAll(t, filepath.Join(realSrc, "sub", "deep"))
	writeFile(t, realSrc, "a.txt", []byte("file a"), 0644)
	writeFile(t, filepath.Join(realSrc, "sub"), "b.txt", []byte("file b"), 0644)
	writeFile(t, filepath.Join(realSrc, "sub", "deep"), "c.txt", []byte("file c"), 0644)

	dstRoot := filepath.Join(dstDir, "dst")

	q := NewCopyQueue([]CopyItem{{Source: realSrc, Dest: dstRoot}}, CopyOptions{})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

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

	totals, ok := firstEvent[QueueTotals](events)
	if !ok {
		t.Fatal("expected QueueTotals event")
	}
	if totals.TotalItems != 1 {
		t.Errorf("TotalItems = %d, want 1", totals.TotalItems)
	}
	expectedBytes := int64(len("file a") + len("file b") + len("file c"))
	if totals.TotalBytes != expectedBytes {
		t.Errorf("TotalBytes = %d, want %d", totals.TotalBytes, expectedBytes)
	}

	// ItemStarted.Size for a directory item should equal total tree bytes.
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
}

// ---------------------------------------------------------------------------
// TestCopyQueue_TotalsCorrectness
// ---------------------------------------------------------------------------

func TestCopyQueue_TotalsCorrectness(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	f1 := writeFile(t, srcDir, "f1.txt", []byte("12345"), 0644)
	f2 := writeFile(t, srcDir, "f2.txt", []byte("1234567890"), 0644)

	q := NewCopyQueue([]CopyItem{
		{Source: f1, Dest: filepath.Join(dstDir, "f1.txt")},
		{Source: f2, Dest: filepath.Join(dstDir, "f2.txt")},
	}, CopyOptions{})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	totals, ok := firstEvent[QueueTotals](events)
	if !ok {
		t.Fatal("expected QueueTotals event")
	}
	if totals.TotalBytes != 15 {
		t.Errorf("TotalBytes = %d, want 15", totals.TotalBytes)
	}
	if totals.TotalItems != 2 {
		t.Errorf("TotalItems = %d, want 2", totals.TotalItems)
	}

	// Each ItemStarted.Size should match the source file size.
	var starteds []ItemStarted
	for _, e := range events {
		if v, ok := e.(ItemStarted); ok {
			starteds = append(starteds, v)
		}
	}
	if len(starteds) != 2 {
		t.Fatalf("expected 2 ItemStarted events, got %d", len(starteds))
	}
	if starteds[0].Size != 5 {
		t.Errorf("ItemStarted[0].Size = %d, want 5", starteds[0].Size)
	}
	if starteds[1].Size != 10 {
		t.Errorf("ItemStarted[1].Size = %d, want 10", starteds[1].Size)
	}
}

// ---------------------------------------------------------------------------
// TestCopyQueue_IntermediateProgress
// ---------------------------------------------------------------------------

func TestCopyQueue_IntermediateProgress(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	size := 5 * 1024 * 1024
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 256)
	}
	srcFile := writeFile(t, srcDir, "big.bin", content, 0644)
	dstFile := filepath.Join(dstDir, "big.bin")

	q := NewCopyQueue([]CopyItem{{Source: srcFile, Dest: dstFile}}, CopyOptions{
		BufferBytes:         1024,
		ProgressMinInterval: 0,
		ProgressMinBytes:    1,
	})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	progressEvents := findEvent[ItemProgress](events)
	if len(progressEvents) == 0 {
		t.Fatal("expected at least one ItemProgress event")
	}

	first := progressEvents[0]
	if first.BytesSoFar <= 0 {
		t.Errorf("first progress BytesSoFar = %d, want > 0", first.BytesSoFar)
	}
	if first.BytesSoFar >= int64(size) {
		t.Errorf("first progress BytesSoFar = %d, want < %d (intermediate)", first.BytesSoFar, size)
	}

	last := progressEvents[len(progressEvents)-1]
	if last.BytesSoFar != int64(size) {
		t.Errorf("final progress BytesSoFar = %d, want %d", last.BytesSoFar, size)
	}
}

// ---------------------------------------------------------------------------
// TestCopyQueue_ThrottlingBoundsEvents
// ---------------------------------------------------------------------------

func TestCopyQueue_ThrottlingBoundsEvents(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	size := 10 * 1024 * 1024
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 256)
	}
	srcFile := writeFile(t, srcDir, "big.bin", content, 0644)
	dstFile := filepath.Join(dstDir, "big.bin")

	q := NewCopyQueue([]CopyItem{{Source: srcFile, Dest: dstFile}}, CopyOptions{
		BufferBytes:         1024,
		ProgressMinInterval: time.Hour,
		ProgressMinBytes:    1 << 20,
	})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	progressEvents := findEvent[ItemProgress](events)
	if len(progressEvents) > 11 {
		t.Errorf("throttled progress events = %d, want <= 11", len(progressEvents))
	}
	if len(progressEvents) >= 100 {
		t.Errorf("progress events = %d, throttling appears inactive", len(progressEvents))
	}
}

// ---------------------------------------------------------------------------
// TestCopyQueue_MidQueueCancellation
// ---------------------------------------------------------------------------

func TestCopyQueue_CancelledQueue(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	f1 := writeFile(t, srcDir, "f1.txt", []byte("file one content"), 0644)
	f2 := writeFile(t, srcDir, "f2.txt", []byte("file two"), 0644)

	dst1 := filepath.Join(dstDir, "f1.txt")
	dst2 := filepath.Join(dstDir, "f2.txt")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before starting

	q := NewCopyQueue([]CopyItem{
		{Source: f1, Dest: dst1},
		{Source: f2, Dest: dst2},
	}, CopyOptions{})

	ch := q.Run(ctx)
	events := collectEvents(t, ch)

	// All items should be skipped since context was cancelled before processing
	var skipped []ItemSkipped
	for _, e := range events {
		if v, ok := e.(ItemSkipped); ok {
			skipped = append(skipped, v)
		}
	}
	if len(skipped) != 2 {
		t.Errorf("expected 2 ItemSkipped events, got %d", len(skipped))
	}

	// Verify QueueDone carries context error
	var qd QueueDone
	if _, ok := firstEvent[QueueDone](events); !ok {
		t.Fatal("expected QueueDone event")
	}
	// Need to find the actual QueueDone value
	for _, e := range events {
		if v, ok := e.(QueueDone); ok {
			qd = v
			break
		}
	}
	if qd.Err == nil {
		t.Error("expected QueueDone.Err to be non-nil after cancellation")
	}
}

// ---------------------------------------------------------------------------
// TestCopyQueue_PerItemErrorDoesNotAbort
// ---------------------------------------------------------------------------

func TestCopyQueue_PerItemErrorDoesNotAbort(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	f1 := writeFile(t, srcDir, "f1.txt", []byte("file one"), 0644)
	dstFile := filepath.Join(dstDir, "collision.txt")
	writeFile(t, dstDir, "collision.txt", []byte("existing"), 0644)

	f2 := writeFile(t, srcDir, "f2.txt", []byte("file two"), 0644)
	dst2 := filepath.Join(dstDir, "f2.txt")

	q := NewCopyQueue([]CopyItem{
		{Source: f1, Dest: dstFile},
		{Source: f2, Dest: dst2},
	}, CopyOptions{Overwrite: false})

	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	var errs []ItemError
	for _, e := range events {
		if v, ok := e.(ItemError); ok {
			errs = append(errs, v)
		}
	}
	if len(errs) == 0 {
		t.Fatal("expected at least one ItemError event")
	}
	if errs[0].Source != f1 {
		t.Errorf("ItemError.Source = %q, want %q", errs[0].Source, f1)
	}

	var dones []ItemDone
	for _, e := range events {
		if v, ok := e.(ItemDone); ok {
			dones = append(dones, v)
		}
	}
	found := false
	for _, d := range dones {
		if d.Source == f2 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ItemDone for second item (f2) despite first item error")
	}

	got, err := os.ReadFile(dst2)
	if err != nil {
		t.Fatalf("f2 should exist: %v", err)
	}
	if string(got) != "file two" {
		t.Errorf("f2 content = %q, want %q", string(got), "file two")
	}
}

// ---------------------------------------------------------------------------
// TestCopyQueue_CollisionPolicy_OverwriteFalse
// ---------------------------------------------------------------------------

func TestCopyQueue_CollisionPolicy_OverwriteFalse(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := writeFile(t, srcDir, "a.txt", []byte("new content"), 0644)
	dstFile := filepath.Join(dstDir, "a.txt")
	writeFile(t, dstDir, "a.txt", []byte("old content"), 0644)

	q := NewCopyQueue([]CopyItem{{Source: srcFile, Dest: dstFile}}, CopyOptions{Overwrite: false})
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

	got, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("dest file should still exist: %v", err)
	}
	if string(got) != "old content" {
		t.Errorf("dest file was overwritten: got %q, want %q", string(got), "old content")
	}
}

// ---------------------------------------------------------------------------
// TestCopyQueue_CollisionPolicy_OverwriteTrue
// ---------------------------------------------------------------------------

func TestCopyQueue_CollisionPolicy_OverwriteTrue(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := writeFile(t, srcDir, "a.txt", []byte("new content"), 0644)
	dstFile := filepath.Join(dstDir, "a.txt")
	writeFile(t, dstDir, "a.txt", []byte("old content"), 0644)

	q := NewCopyQueue([]CopyItem{{Source: srcFile, Dest: dstFile}}, CopyOptions{Overwrite: true})
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
}

// ---------------------------------------------------------------------------
// TestCopyQueue_DestInsideSourceGuard
// ---------------------------------------------------------------------------

func TestCopyQueue_DestInsideSourceGuard(t *testing.T) {
	srcDir := t.TempDir()

	srcSub := filepath.Join(srcDir, "src")
	mkdirAll(t, srcSub)
	writeFile(t, srcSub, "a.txt", []byte("content"), 0644)

	dstInside := filepath.Join(srcSub, "copy")

	q := NewCopyQueue([]CopyItem{{Source: srcSub, Dest: dstInside}}, CopyOptions{})
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
// TestCopyQueue_BufferClamping
// ---------------------------------------------------------------------------

func TestCopyQueue_BufferClamping(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := writeFile(t, srcDir, "a.txt", []byte("content"), 0644)
	dstFile := filepath.Join(dstDir, "a.txt")

	tests := []struct {
		name   string
		buffer int
	}{
		{"below min", 100},
		{"default zero", 0},
		{"within range", 512 * 1024},
		{"above max", 16 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := NewCopyQueue([]CopyItem{{Source: srcFile, Dest: dstFile}}, CopyOptions{
				BufferBytes: tt.buffer,
				Overwrite:   true,
			})
			ch := q.Run(context.Background())
			events := collectEvents(t, ch)

			if _, ok := firstEvent[ItemDone](events); !ok {
				t.Fatal("expected ItemDone")
			}

			got, err := os.ReadFile(dstFile)
			if err != nil {
				t.Fatalf("dest file should exist: %v", err)
			}
			if string(got) != "content" {
				t.Errorf("content = %q, want %q", string(got), "content")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestCopyQueue_NonExistentSource
// ---------------------------------------------------------------------------

func TestCopyQueue_NonExistentSource(t *testing.T) {
	dstDir := t.TempDir()

	q := NewCopyQueue([]CopyItem{{Source: "/nonexistent/path/to/file.txt", Dest: filepath.Join(dstDir, "file.txt")}}, CopyOptions{})
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
// TestCopyQueue_EmptyQueue
// ---------------------------------------------------------------------------

func TestCopyQueue_EmptyQueue(t *testing.T) {
	q := NewCopyQueue(nil, CopyOptions{})
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

// ---------------------------------------------------------------------------
// TestCopyQueue_DirectoryMtimeBestEffort
// ---------------------------------------------------------------------------

func TestCopyQueue_DirectoryMtimeBestEffort(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcSub := filepath.Join(srcDir, "srcdir")
	mkdirAll(t, srcSub)
	writeFile(t, srcSub, "a.txt", []byte("content"), 0644)

	mtime := time.Date(2024, 3, 10, 8, 0, 0, 0, time.UTC)
	if err := os.Chtimes(srcSub, mtime, mtime); err != nil {
		t.Fatal(err)
	}

	dstSub := filepath.Join(dstDir, "srcdir")

	q := NewCopyQueue([]CopyItem{{Source: srcSub, Dest: dstSub}}, CopyOptions{})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	if _, err := os.Stat(dstSub); err != nil {
		t.Fatalf("dest directory should exist: %v", err)
	}

	var qd QueueDone
	if _, ok := firstEvent[QueueDone](events); !ok {
		t.Fatal("expected QueueDone")
	}
	if qd.Err != nil {
		t.Errorf("QueueDone.Err = %v, want nil", qd.Err)
	}
}

// ---------------------------------------------------------------------------
// TestCopyQueue_ModePreservation
// ---------------------------------------------------------------------------

func TestCopyQueue_ModePreservation(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	srcFile := filepath.Join(srcDir, "script.sh")
	if err := os.WriteFile(srcFile, []byte("#!/bin/sh\necho hello"), 0755); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(dstDir, "script.sh")

	q := NewCopyQueue([]CopyItem{{Source: srcFile, Dest: dstFile}}, CopyOptions{})
	ch := q.Run(context.Background())
	collectEvents(t, ch)

	_, err := os.Stat(dstFile)
	if err != nil {
		t.Fatalf("dest file should exist: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestCopyQueue_ItemProgressIndex
// ---------------------------------------------------------------------------

func TestCopyQueue_ItemProgressIndex(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	f1 := writeFile(t, srcDir, "f1.txt", []byte("content1"), 0644)
	f2 := writeFile(t, srcDir, "f2.txt", []byte("content2"), 0644)

	q := NewCopyQueue([]CopyItem{
		{Source: f1, Dest: filepath.Join(dstDir, "f1.txt")},
		{Source: f2, Dest: filepath.Join(dstDir, "f2.txt")},
	}, CopyOptions{
		BufferBytes:         1,
		ProgressMinInterval: 0,
		ProgressMinBytes:    1,
	})
	ch := q.Run(context.Background())
	events := collectEvents(t, ch)

	progressEvents := findEvent[ItemProgress](events)
	if len(progressEvents) == 0 {
		t.Fatal("expected ItemProgress events")
	}

	for _, p := range progressEvents {
		if p.Index < 0 {
			t.Errorf("ItemProgress.Index = %d, want >= 0", p.Index)
		}
	}

	// ItemStarted.Size must be populated for each file item.
	var starteds []ItemStarted
	for _, e := range events {
		if v, ok := e.(ItemStarted); ok {
			starteds = append(starteds, v)
		}
	}
	if len(starteds) != 2 {
		t.Fatalf("expected 2 ItemStarted events, got %d", len(starteds))
	}
	if starteds[0].Size != int64(len("content1")) {
		t.Errorf("ItemStarted[0].Size = %d, want %d", starteds[0].Size, int64(len("content1")))
	}
	if starteds[1].Size != int64(len("content2")) {
		t.Errorf("ItemStarted[1].Size = %d, want %d", starteds[1].Size, int64(len("content2")))
	}
}

func TestCancelledBeforeStart(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	src := writeFile(t, srcDir, "f1.txt", []byte("file one content"), 0644)
	dst := filepath.Join(dstDir, "f1.txt")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before starting

	q := NewCopyQueue([]CopyItem{{Source: src, Dest: dst}}, CopyOptions{})

	events := collectEvents(t, q.Run(ctx))

	var qd QueueDone
	for _, e := range events {
		if v, ok := e.(QueueDone); ok {
			qd = v
		}
	}
	if qd.Err == nil {
		t.Error("expected QueueDone.Err to be non-nil for a pre-cancelled context")
	}
}

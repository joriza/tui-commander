package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// ctxForTest returns a context that is never cancelled, so the queue runs to
// completion. It is the same shape the production callers build.
func ctxForTest() context.Context {
	return context.Background()
}

// eventError extracts the first ItemError from an event stream, or nil when
// the queue completed without one.
func eventError(events []any) error {
	if e, ok := firstEvent[ItemError](events); ok {
		return e.Cause
	}
	return nil
}

// symlinkTraversalGuard reports whether symlinks can be created on this
// machine. The traversal-policy tests are opt-in behind it: a test suite
// must not litter the filesystem, and on a platform where Symlink fails the
// assertions cannot be seeded at all (the same discipline as TestLongPath_Trash).
func symlinkTraversalGuard() bool {
	return os.Getenv("TUI_COMMANDER_TEST_SYMLINK") == "1"
}

// TestCopyQueue_DirCopyDoesNotTraverseSymlink proves the traversal policy: when
// a directory contains a symlink to another directory, copying the tree copies
// the LINK as a link — it does not descend into the target and duplicate its
// tree. A loop cannot hang the copy, and the target's contents never appear
// under the destination.
func TestCopyQueue_DirCopyDoesNotTraverseSymlink(t *testing.T) {
	src := t.TempDir()
	target := t.TempDir()

	// Target has a tree that must NOT leak into the copy.
	if err := os.Mkdir(filepath.Join(target, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "inner", "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Source contains a symlink to that target.
	linkPath := filepath.Join(src, "link-to-target")
	if err := os.Symlink(target, linkPath); err != nil {
		if symlinkTraversalGuard() {
			t.Fatalf("Symlink required but failed: %v", err)
		}
		t.Skipf("Symlink not supported on this platform: %v", err)
	}

	dst := t.TempDir()

	q := NewCopyQueue([]CopyItem{{Source: src, Dest: dst}}, CopyOptions{})
	events := collectEvents(t, q.Run(ctxForTest()))

	if err := eventError(events); err != nil {
		t.Fatalf("copy failed: %v", err)
	}

	// The link itself must exist in the destination.
	linkDst := filepath.Join(dst, "link-to-target")
	if _, err := os.Lstat(linkDst); err != nil {
		t.Fatalf("symlink not copied to destination: %v", err)
	}

	// The target's tree must NOT have been duplicated under the destination:
	// copying never traversed the link.
	if _, err := os.Stat(filepath.Join(dst, "inner")); err == nil {
		t.Fatal("the copy traversed the symlink: the target's tree leaked into the destination")
	}
	if _, err := os.Stat(filepath.Join(linkDst, "inner")); err == nil {
		t.Fatal("the copy traversed the symlink: the target's tree leaked under the copied link")
	}
}

// TestCopyQueue_DirCopyDoesNotHangOnCircularSymlink proves a loop cannot hang
// the copy: a symlink that points back into the source tree is copied as a
// link, so the recursive walk never re-enters the source.
func TestCopyQueue_DirCopyDoesNotHangOnCircularSymlink(t *testing.T) {
	src := t.TempDir()

	if err := os.Mkdir(filepath.Join(src, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A symlink inside the tree that points back at the source root: a
	// naive Stat-based walk would re-enter src forever.
	linkPath := filepath.Join(src, "child", "loop")
	if err := os.Symlink(src, linkPath); err != nil {
		if symlinkTraversalGuard() {
			t.Fatalf("Symlink required but failed: %v", err)
		}
		t.Skipf("Symlink not supported on this platform: %v", err)
	}

	dst := t.TempDir()

	q := NewCopyQueue([]CopyItem{{Source: src, Dest: dst}}, CopyOptions{})
	events := collectEvents(t, q.Run(ctxForTest()))

	if err := eventError(events); err != nil {
		t.Fatalf("copy failed: %v", err)
	}

	// The loop link is present as a link, and the copy finished (no hang).
	if _, err := os.Lstat(filepath.Join(dst, "child", "loop")); err != nil {
		t.Fatalf("loop symlink not copied: %v", err)
	}
}

// TestMoveQueue_PreWalkUsesLstat proves the byte/total estimate never follows
// a symlink: a directory symlink's target tree is not counted, so a move
// whose source contains a link reports the link's own size, not the target's.
func TestMoveQueue_PreWalkUsesLstat(t *testing.T) {
	src := t.TempDir()
	target := t.TempDir()

	if err := os.WriteFile(filepath.Join(target, "big.txt"), makeBytes(1024*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(src, "link-to-target")
	if err := os.Symlink(target, linkPath); err != nil {
		if symlinkTraversalGuard() {
			t.Fatalf("Symlink required but failed: %v", err)
		}
		t.Skipf("Symlink not supported on this platform: %v", err)
	}

	// Drive the private preWalk through the public queue: NewMoveQueue then
	// Run, capturing QueueTotals. The totals must reflect the link only.
	q := NewMoveQueue([]MoveItem{{Source: src, Dest: t.TempDir()}}, MoveOptions{})
	ch := q.Run(ctxForTest())
	var totalBytes int64
	for ev := range ch {
		if tt, ok := ev.(QueueTotals); ok {
			totalBytes = tt.TotalBytes
		}
	}
	// The target's 1 MiB file must not be counted: preWalk used Lstat and
	// did not follow the link into its target.
	if totalBytes >= 1024*1024 {
		t.Errorf("TotalBytes = %d, want under 1 MiB (preWalk followed the symlink)", totalBytes)
	}
}

func makeBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}